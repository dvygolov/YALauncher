//go:build windows

package main

import (
	"syscall"
	"unsafe"

	"github.com/dvygolov/yalauncher/internal/win"
)

var (
	dllShell32     = syscall.NewLazyDLL("shell32.dll")
	pSHBrowse      = dllShell32.NewProc("SHBrowseForFolderW")
	pSHGetPath     = dllShell32.NewProc("SHGetPathFromIDListW")
	pCoTaskMemFree = syscall.NewLazyDLL("ole32.dll").NewProc("CoTaskMemFree")
	pSetCurrentDir = syscall.NewLazyDLL("kernel32.dll").NewProc("SetCurrentDirectoryW")
)

const (
	bifReturnOnlyFSDirs = 0x0001
	bifNewDialogStyle   = 0x0040
	bifEditBox          = 0x0010
)

type browseInfoW struct {
	Owner        uintptr
	Root         uintptr
	DisplayName  *uint16
	Title        *uint16
	Flags        uint32
	Callback     uintptr
	CallbackData uintptr
	Image        int32
}

// browseFolder shows the shell folder picker and returns the chosen path.
func browseFolder(title string) string {
	var bi browseInfoW
	bi.Title = win.UTF16Ptr(title)
	bi.Flags = bifReturnOnlyFSDirs | bifNewDialogStyle | bifEditBox
	pidl, _, _ := pSHBrowse.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return ""
	}
	defer pCoTaskMemFree.Call(pidl)
	buf := make([]uint16, win.MAX_PATH)
	r, _, _ := pSHGetPath.Call(pidl, uintptr(unsafe.Pointer(&buf[0])))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}
