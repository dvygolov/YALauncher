//go:build windows

package win

import (
	"strings"
	"syscall"
	"unsafe"
)

// Small registry helpers: autostart (HKCU\...\Run), reading string values and
// the Windows 11 tray promotion tweak.

const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

func regCreate(root uintptr, path string) HKEY {
	var h HKEY
	var disp uint32
	r, _, _ := pRegCreateKeyExW.Call(
		root,
		uintptr(unsafe.Pointer(UTF16Ptr(path))),
		0, 0, 0, KEY_WRITE,
		0, uintptr(unsafe.Pointer(&h)), uintptr(unsafe.Pointer(&disp)))
	if r != 0 {
		return 0
	}
	return h
}

func RegOpen(root uintptr, path string) HKEY {
	return regOpen(root, path, KEY_READ)
}

func regOpen(root uintptr, path string, access uintptr) HKEY {
	var h HKEY
	r, _, _ := pRegOpenKeyExW.Call(
		root,
		uintptr(unsafe.Pointer(UTF16Ptr(path))),
		0, access, uintptr(unsafe.Pointer(&h)))
	if r != 0 {
		return 0
	}
	return h
}

func RegClose(h HKEY) { regClose(h) }

func regClose(h HKEY) {
	if h != 0 {
		pRegCloseKey.Call(uintptr(h))
	}
}

func regSetString(h HKEY, name, value string) bool {
	v := syscall.StringToUTF16(value)
	r, _, _ := pRegSetValueExW.Call(uintptr(h),
		uintptr(unsafe.Pointer(UTF16Ptr(name))), 0, REG_SZ,
		uintptr(unsafe.Pointer(&v[0])), uintptr(len(v)*2))
	return r == 0
}

func regSetDword(h HKEY, name string, value uint32) bool {
	r, _, _ := pRegSetValueExW.Call(uintptr(h),
		uintptr(unsafe.Pointer(UTF16Ptr(name))), 0, REG_DWORD,
		uintptr(unsafe.Pointer(&value)), 4)
	return r == 0
}

// RegString reads a REG_SZ / REG_EXPAND_SZ value from HKCU.
func RegString(root uintptr, path, name string) string {
	var buf [1024]uint16
	size := uint32(len(buf) * 2)
	r, _, _ := pRegGetValueW.Call(root,
		uintptr(unsafe.Pointer(UTF16Ptr(path))),
		uintptr(unsafe.Pointer(UTF16Ptr(name))),
		RRF_RT_REG_SZ|RRF_RT_REG_EXPAND_SZ, 0,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r != 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:])
}

// SubKeys lists the direct children of a key.
func SubKeys(root uintptr, path string) []string {
	h := regOpen(root, path, KEY_READ)
	if h == 0 {
		return nil
	}
	defer regClose(h)
	var out []string
	buf := make([]uint16, 256)
	for i := uint32(0); ; i++ {
		n := uint32(len(buf))
		rr, _, _ := pRegEnumKeyExW.Call(uintptr(h), uintptr(i),
			uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)), 0, 0, 0, 0)
		if rr != 0 {
			break
		}
		out = append(out, syscall.UTF16ToString(buf[:n]))
	}
	return out
}

// SetRunValue writes an autostart entry for the current user.
func SetRunValue(name, exePath, args string) bool {
	h := regCreate(HKEY_CURRENT_USER, runKeyPath)
	if h == 0 {
		return false
	}
	defer regClose(h)
	value := `"` + exePath + `"`
	if args != "" {
		value += " " + args
	}
	return regSetString(h, name, value)
}

func DeleteRunValue(name string) {
	h := regOpen(HKEY_CURRENT_USER, runKeyPath, KEY_WRITE)
	if h == 0 {
		return
	}
	defer regClose(h)
	pRegDeleteValueW.Call(uintptr(h), uintptr(unsafe.Pointer(UTF16Ptr(name))))
}

func RunValue(name string) string {
	return RegString(HKEY_CURRENT_USER, runKeyPath, name)
}

// DiskLabels returns the volume label of every fixed and removable drive,
// keyed by "C:".
func DiskLabels() map[string]string {
	out := make(map[string]string, 4)
	drives, _, _ := pGetLogicalDrives.Call()
	for i := 0; i < 26; i++ {
		if drives&(1<<uint(i)) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		var name [261]uint16
		var serial, maxLen, flags uint32
		var fs [261]uint16
		if r, _, _ := pGetVolumeInfo.Call(uintptr(unsafe.Pointer(UTF16Ptr(root))),
			uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)),
			uintptr(unsafe.Pointer(&serial)), uintptr(unsafe.Pointer(&maxLen)),
			uintptr(unsafe.Pointer(&flags)), uintptr(unsafe.Pointer(&fs[0])), uintptr(len(fs))); r == 0 {
			continue
		}
		out[string(rune('A'+i))+":"] = syscall.UTF16ToString(name[:])
	}
	return out
}

// Browsers register themselves as the http(s) handler under this path; the
// launcher reads DefaultIcon from there to show a real browser icon.
const UrlAssocPath = `Software\Microsoft\Windows\Shell\Associations\UrlAssociations\http\UserChoice`

// PromoteTrayIcon marks the icon as "always show" in Windows 11's tray.
func PromoteTrayIcon(exePath string) bool {
	if exePath == "" {
		return false
	}
	const root = `Control Panel\NotifyIconSettings`
	hkey := regOpen(HKEY_CURRENT_USER, root, KEY_READ)
	if hkey == 0 {
		return false
	}
	defer regClose(hkey)

	done := false
	for _, sub := range SubKeys(HKEY_CURRENT_USER, root) {
		key := root + `\` + sub
		if !strings.EqualFold(RegString(HKEY_CURRENT_USER, key, "ExecutablePath"), exePath) {
			continue
		}
		kh := regOpen(HKEY_CURRENT_USER, key, KEY_WRITE)
		if kh == 0 {
			continue
		}
		if regSetDword(kh, "IsPromoted", 1) {
			done = true
		}
		regClose(kh)
	}
	return done
}
