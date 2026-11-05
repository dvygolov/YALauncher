//go:build windows

package win

// EnterForeground takes the focus the way a quick launcher has to. Windows
// only honours SetForegroundWindow for the process that owns the foreground
// window; the launcher has just been called from a keyboard hook of the
// session, so attaching to the foreground thread for the duration of the call
// is what makes the window appear focused instead of just on top.
func EnterForeground(h HWND) {
	if h == 0 {
		return
	}
	fg := GetForegroundWindow()
	if fg == h {
		return
	}
	tid := GetWindowThreadProcessId(fg)
	me := GetCurrentThreadId()
	attached := false
	if tid != 0 && tid != me {
		r, _, _ := pAttachThreadInput.Call(uintptr(me), uintptr(tid), 1)
		attached = r != 0
	}
	BringWindowToTop(h)
	SetForegroundWindow(h)
	SetFocus(h)
	if attached {
		pAttachThreadInput.Call(uintptr(me), uintptr(tid), 0)
	}
}
