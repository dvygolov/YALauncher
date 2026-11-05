//go:build windows

package win

import (
	"os"
	"syscall"
	"unsafe"
)

// Shell integration: launching programs, icons of arbitrary files and drag
// and drop file names.

// ShellExecute opens a file, folder or URL through the shell.
func ShellExecute(verb, file, params, dir string, show int32) bool {
	var verbPtr, paramsPtr, dirPtr uintptr
	if verb != "" {
		verbPtr = uintptr(unsafe.Pointer(UTF16Ptr(verb)))
	}
	if params != "" {
		paramsPtr = uintptr(unsafe.Pointer(UTF16Ptr(params)))
	}
	if dir != "" {
		dirPtr = uintptr(unsafe.Pointer(UTF16Ptr(dir)))
	}
	r, _, _ := pShellExecuteW.Call(0, verbPtr,
		uintptr(unsafe.Pointer(UTF16Ptr(file))), paramsPtr, dirPtr, uintptr(show))
	return r > 32
}

// Open runs the default action of a shell item.
func Open(path, params, dir string) bool {
	return ShellExecute("open", path, params, dir, SW_SHOWNORMAL)
}

// OpenAsAdmin asks the shell to relaunch an item elevated.
func OpenAsAdmin(path, params, dir string) bool {
	return ShellExecute("runas", path, params, dir, SW_SHOWNORMAL)
}

// ShowInFolder selects the file inside Explorer.
func ShowInFolder(path string) bool {
	return ShellExecute("open", "explorer.exe", "/select,\""+path+"\"", "", SW_SHOWNORMAL)
}

// FileIcon loads the shell icon of a file into a 32 bpp DIB of the requested
// size. The icon is rendered by the shell exactly as Explorer does, which is
// why .lnk files, UWP stubs and apps with embedded icons all look right.
func FileIcon(path string, size int32, wantLarge bool) *MemDC {
	if path == "" || size <= 0 {
		return nil
	}
	var info SHFILEINFOW
	flags := uintptr(SHGFI_ICON)
	if wantLarge {
		flags = uintptr(SHGFI_ICON | SHGFI_LARGEICON)
	} else {
		flags = uintptr(SHGFI_ICON | SHGFI_SMALLICON)
	}
	if _, err := os.Stat(path); err != nil {
		// The file does not exist: let the shell pick by extension instead of
		// failing with a blank icon.
		flags |= SHGFI_USEFILEATTRIBUTES
	}
	r, _, _ := pSHGetFileInfoW.Call(uintptr(unsafe.Pointer(UTF16Ptr(path))), FILE_ATTRIBUTE_NORMAL,
		uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info), flags)
	if r == 0 || info.HIcon == 0 {
		return nil
	}
	icon := info.HIcon
	defer pDestroyIcon.Call(uintptr(icon))

	dc := NewMemDC(size, size)
	if dc == nil {
		return nil
	}
	// DrawIconEx expands the icon to the DIB. Icons without alpha (old 16x16
	// resources) come out with the classic mask applied by the shell itself.
	ok := DrawIconEx(dc.DC, 0, 0, icon, size, size)
	if !ok {
		// Fall back to the size the shell chose.
		dc.Close()
		return nil
	}
	return dc
}

// FileIconFromICO renders an icon from the bytes of an .ico resource.
func FileIconFromICO(data []byte, size int32) *MemDC {
	h := LoadIconFromICO(data, int(size))
	if h == 0 {
		return nil
	}
	defer pDestroyIcon.Call(uintptr(h))
	dc := NewMemDC(size, size)
	if dc == nil {
		return nil
	}
	if !DrawIconEx(dc.DC, 0, 0, h, size, size) {
		dc.Close()
		return nil
	}
	return dc
}

// DrawIconEx paints an icon stretched into the given box.
func DrawIconEx(dc HDC, x, y int32, icon HICON, w, h int32) bool {
	r, _, _ := pDrawIconEx.Call(uintptr(dc), uintptr(x), uintptr(y), uintptr(icon),
		uintptr(w), uintptr(h), 0, 0, DI_NORMAL)
	return r != 0
}

// LoadIconFromICO picks the image closest to want pixels from an .ico file and
// returns a GDI icon handle.
func LoadIconFromICO(data []byte, want int) HICON {
	if len(data) < 6 || data[2] != 1 || data[3] != 0 {
		return 0
	}
	count := int(uint16(data[4]) | uint16(data[5])<<8)
	best, bestScore := 0, 1<<30
	for i := 0; i < count; i++ {
		off := 6 + 16*i
		if off+16 > len(data) {
			break
		}
		w := int(data[off])
		if w == 0 {
			w = 256
		}
		if w > 128 {
			continue // PNG payloads are not needed for the sizes we draw
		}
		score := w - want
		if score < 0 {
			score = -score
		}
		if score < bestScore {
			bestScore, best = score, i
		}
	}
	if bestScore == 1<<30 {
		best = 0
	}
	off := 6 + 16*best
	if off+16 > len(data) {
		return 0
	}
	size := int(uint32(data[off+8]) | uint32(data[off+9])<<8 | uint32(data[off+10])<<16 | uint32(data[off+11])<<24)
	imgOff := int(uint32(data[off+12]) | uint32(data[off+13])<<8 | uint32(data[off+14])<<16 | uint32(data[off+15])<<24)
	if size <= 0 || imgOff <= 0 || imgOff+size > len(data) {
		return 0
	}
	h, _, _ := pCreateIconFromRes.Call(
		uintptr(unsafe.Pointer(&data[imgOff])), uintptr(size), 1, CREATE_ICON_VERSION, 0, 0, 0)
	return HICON(h)
}

// DropFiles enables WM_DROPFILES and returns the names that were dropped.
func DropFiles(h HWND) {
	pDragAcceptFiles.Call(uintptr(h), 1)
}

// DroppedFiles expands a WM_DROPFILES payload.
func DroppedFiles(wp uintptr) []string {
	if wp == 0 {
		return nil
	}
	n, _, _ := pDragQueryFileW.Call(wp, ^uintptr(0), 0, 0)
	out := make([]string, 0, int(n))
	for i := uintptr(0); i < n; i++ {
		buf := make([]uint16, MAX_PATH*2)
		_, _, _ = pDragQueryFileW.Call(wp, i, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		out = append(out, syscall.UTF16ToString(buf))
	}
	pDragFinish.Call(wp)
	return out
}

// KnownFolder returns one of the shell folder paths (CSIDL_*).
func KnownFolder(csidl int) string {
	buf := make([]uint16, MAX_PATH*2)
	r, _, _ := pSHGetFolderPathW.Call(0, uintptr(csidl), 0, 0, uintptr(unsafe.Pointer(&buf[0])))
	if r != 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}
