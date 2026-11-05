//go:build windows

package win

import "unsafe"

const (
	fileNotifyChangeFileName   = 0x00000001
	fileNotifyChangeDirName    = 0x00000002
	fileNotifyChangeAttributes = 0x00000004
	fileNotifyChangeSize       = 0x00000008
	fileNotifyChangeLastWrite  = 0x00000010
	fileNotifyChangeCreation   = 0x00000040
	waitTimeout                = uintptr(0x102)
	waitFailed                 = uint32(0xFFFFFFFF)
)

// Folder change notifications. One handle per folder; WaitAny blocks until one
// of them reports a change or the timeout expires.

// WatchDirectory opens change notifications for a folder.
func WatchDirectory(path string, recursive bool) []HANDLE {
	filter := uintptr(fileNotifyChangeFileName | fileNotifyChangeDirName |
		fileNotifyChangeSize | fileNotifyChangeLastWrite | fileNotifyChangeCreation)
	_ = recursive
	p, err := syscallUTF16Ptr(path)
	if err != nil {
		return nil
	}
	h, _, _ := pFindFirstChange.Call(uintptr(unsafe.Pointer(p)), 0, filter)
	if h == 0 {
		return nil
	}
	return []HANDLE{HANDLE(h)}
}

// WaitAny waits up to timeoutMS for any handle to be signalled and returns the
// index of the first one, or ok=false on timeout.
func WaitAny(handles []HANDLE, timeoutMS uint32) (int, bool) {
	if len(handles) == 0 {
		Sleep(timeoutMS)
		return 0, false
	}
	if len(handles) > 64 {
		handles = handles[:64]
	}
	r, _, _ := pWaitForMultiple.Call(uintptr(len(handles)), uintptr(unsafe.Pointer(&handles[0])), 0, uintptr(timeoutMS))
	if r == waitTimeout || uint32(r) == waitFailed {
		return 0, false
	}
	return int(r), true
}

// FindNextChange re-arms a handle after it has been signalled.
func FindNextChange(h HANDLE) { pFindNextChange.Call(uintptr(h)) }

// CloseChangeHandle stops a change notification.
func CloseChangeHandle(h HANDLE) { pFindCloseChange.Call(uintptr(h)) }
