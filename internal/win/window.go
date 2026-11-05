//go:build windows

package win

import (
	"errors"
	"syscall"
	"unsafe"
)

// WindowProc receives the messages of the windows created through this
// package.
type WindowProc func(hwnd HWND, msg uint32, wp, lp uintptr) uintptr

// PanicLog receives the value of a panic caught inside a window procedure.
// Without it the only symptom of a broken paint handler is a blank window.
var PanicLog func(format string, args ...any)

// callbackSlot keeps a Go closure addressable for the lifetime of the process.
// syscall.NewCallback returns an integer that can only be created for funcs of
// known signatures; one generic signature shared by all windows is enough.
var (
	cbProc  = syscall.NewCallback(dispatchProc)
	cbSlot  map[HWND]WindowProc
	cbOrder []HWND
)

func dispatchProc(hwnd HWND, msg uint32, wp, lp uintptr) (ret uintptr) {
	defer func() {
		if r := recover(); r != nil {
			if PanicLog != nil {
				PanicLog("window proc panic: %v", r)
			}
			ret = DefWindowProc(hwnd, msg, wp, lp)
		}
	}()
	if fn, ok := cbSlot[hwnd]; ok && fn != nil {
		return fn(hwnd, msg, wp, lp)
	}
	return DefWindowProc(hwnd, msg, wp, lp)
}

func setWindowProc(h HWND, fn WindowProc) {
	if cbSlot == nil {
		cbSlot = make(map[HWND]WindowProc)
	}
	cbSlot[h] = fn
	cbOrder = append(cbOrder, h)
	if len(cbOrder) > 32 {
		// keep the map bounded: windows live for the whole session
		cbOrder = cbOrder[len(cbOrder)-32:]
	}
}

func clearWindowProc(h HWND) {
	delete(cbSlot, h)
}

// ModuleHandle returns the HINSTANCE of the running exe.
func ModuleHandle() HINSTANCE {
	r, _, _ := pGetModuleHandleW.Call(0)
	return HINSTANCE(r)
}

// ClassName is the extra atom-and-name registration result.
type ClassName struct{ Name string }

// RegisterClass registers a window class with the shared window procedure.
func RegisterClass(name string, icon HICON) error {
	inst := ModuleHandle()
	cursor, _, _ := pLoadCursorW.Call(0, IDC_ARROW)
	wc := WNDCLASSEXW{
		Size:      uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		WndProc:   cbProc,
		Instance:  inst,
		Icon:      icon,
		HCursor:   cursor,
		ClassName: UTF16Ptr(name),
		IconSm:    icon,
	}
	r, _, e := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if r == 0 {
		if errno, ok := e.(syscall.Errno); ok {
			if errno == 1410 { // ERROR_CLASS_ALREADY_EXISTS
				return nil
			}
			if errno != 0 {
				return errno
			}
		}
		return errors.New("RegisterClassExW failed")
	}
	return nil
}

// CreateWindow creates a top level window. exStyle/style are the raw Win32
// flags; the window is not shown.
func CreateWindow(class, title string, exStyle, style uint32, x, y, w, h int32, proc WindowProc) HWND {
	r, _, _ := pCreateWindowExW.Call(
		uintptr(exStyle),
		uintptr(unsafe.Pointer(UTF16Ptr(class))),
		uintptr(unsafe.Pointer(UTF16Ptr(title))),
		uintptr(style),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		0, 0, uintptr(ModuleHandle()), 0,
	)
	if r == 0 {
		return 0
	}
	setWindowProc(HWND(r), proc)
	return HWND(r)
}

func DestroyWindow(h HWND) {
	if h != 0 {
		pDestroyWindow.Call(uintptr(h))
		clearWindowProc(h)
	}
}

func IsWindow(h HWND) bool {
	if h == 0 {
		return false
	}
	r, _, _ := pIsWindow.Call(uintptr(h))
	return r != 0
}

// RunMessageLoop pumps messages until WM_QUIT or WM_QUIT is posted.
func RunMessageLoop() {
	var m MSG
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// PumpOnce drains every pending message without blocking. It is used by the
// background worker thread, which needs a message queue only so that posted
// notifications reach the main thread in order.
func PumpOnce(hwnd HWND) {
	var m MSG
	for i := 0; i < 64; i++ {
		r, _, _ := pPeekMessageW.Call(uintptr(unsafe.Pointer(&m)), uintptr(hwnd), 0, 0, PM_REMOVE)
		if r == 0 {
			return
		}
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func PostQuitMessage(code int32) { pPostQuitMessage.Call(uintptr(code)) }

func DefWindowProc(h HWND, msg uint32, wp, lp uintptr) uintptr {
	r, _, _ := pDefWindowProcW.Call(uintptr(h), uintptr(msg), wp, lp)
	return r
}

func PostMessage(h HWND, msg uint32, wp, lp uintptr) bool {
	r, _, _ := pPostMessageW.Call(uintptr(h), uintptr(msg), wp, lp)
	return r != 0
}

func SendMessage(h HWND, msg uint32, wp, lp uintptr) uintptr {
	r, _, _ := pSendMessageW.Call(uintptr(h), uintptr(msg), wp, lp)
	return r
}

func FindWindow(class string) HWND {
	r, _, _ := pFindWindowW.Call(uintptr(unsafe.Pointer(UTF16Ptr(class))), 0)
	return HWND(r)
}

// RegisterWindowMessage registers a broadcast message such as TaskbarCreated.
func RegisterWindowMessage(name string) uint32 {
	r, _, _ := pRegisterWindowMsg.Call(uintptr(unsafe.Pointer(UTF16Ptr(name))))
	return uint32(r)
}

func SetTimer(h HWND, id uintptr, ms uint32) { pSetTimer.Call(uintptr(h), id, uintptr(ms), 0) }
func KillTimer(h HWND, id uintptr)           { pKillTimer.Call(uintptr(h), id) }

func MessageBox(text, title string, flags uint32) int {
	r, _, _ := pMessageBoxW.Call(0,
		uintptr(unsafe.Pointer(UTF16Ptr(text))),
		uintptr(unsafe.Pointer(UTF16Ptr(title))),
		uintptr(flags))
	return int(r)
}

func GetSystemMetrics(index int) int32 {
	r, _, _ := pGetSystemMetrics.Call(uintptr(index))
	return int32(r)
}

func CursorPos() POINT {
	var p POINT
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return p
}

func SetForegroundWindow(h HWND) bool {
	r, _, _ := pSetForegroundW.Call(uintptr(h))
	return r != 0
}

func GetForegroundWindow() HWND {
	r, _, _ := pGetForeground.Call()
	return HWND(r)
}

func SetFocus(h HWND) { pSetFocus.Call(uintptr(h)) }

// IsWindowVisible reports whether the window is shown.
func IsWindowVisible(h HWND) bool {
	if h == 0 {
		return false
	}
	r, _, _ := pIsWindowVisible.Call(uintptr(h))
	return r != 0
}
func GetFocus() HWND {
	r, _, _ := pGetFocus.Call()
	return HWND(r)
}

func ShowWindow(h HWND, cmd int32) { pShowWindow.Call(uintptr(h), uintptr(cmd)) }

func SetWindowPos(h HWND, after HWND, x, y, cx, cy int32, flags uint32) {
	// Coordinates are signed; converting a negative int32 straight to uintptr
	// sign extends it to a huge unsigned value and the window is moved to a
	// position Windows cannot show. Going through int keeps the two's
	// complement bit pattern the API expects.
	pSetWindowPos.Call(uintptr(h), uintptr(after),
		uintptr(int(x)), uintptr(int(y)), uintptr(int(cx)), uintptr(int(cy)), uintptr(flags))
}

func UpdateWindow(h HWND) { pUpdateWindow.Call(uintptr(h)) }

func InvalidateRect(h HWND, r *RECT, erase bool) {
	var p uintptr
	if r != nil {
		p = uintptr(unsafe.Pointer(r))
	}
	e := uintptr(0)
	if erase {
		e = 1
	}
	pInvalidateRect.Call(uintptr(h), p, e)
}

func GetClientRect(h HWND) RECT {
	var r RECT
	pGetClientRect.Call(uintptr(h), uintptr(unsafe.Pointer(&r)))
	return r
}

func Sleep(ms uint32) { pSleep.Call(uintptr(ms)) }

// TickCount is the number of milliseconds since boot.
func TickCount() uint64 {
	r, _, _ := pGetTickCount.Call()
	return uint64(r)
}

// SetTimerSafe starts a timer and reports whether it could be created.
func SetTimerSafe(h HWND, id uintptr, ms uint32) bool {
	r, _, _ := pSetTimer.Call(uintptr(h), id, uintptr(ms), 0)
	return r != 0
}

// CreateInstanceMutex creates (or opens) the named single instance mutex. The
// second return value reports that another instance already created it.
func CreateInstanceMutex(name string) (HANDLE, bool) {
	r, _, e := pCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(UTF16Ptr(name))))
	errno, _ := e.(syscall.Errno)
	if r == 0 {
		return 0, true
	}
	return HANDLE(r), errno == ERROR_ALREADY_EXISTS
}

func CloseHandle(h HANDLE) {
	if h != 0 {
		pCloseHandle.Call(uintptr(h))
	}
}

// CurrentProcess returns the pseudo handle of the current process.
func CurrentProcess() HANDLE {
	r, _, _ := pGetCurrentProcess.Call()
	return HANDLE(r)
}

// ExePath returns the full path of the running executable.
func ExePath() string {
	buf := make([]uint16, 32768)
	n := uint32(len(buf))
	r, _, _ := pQueryFullProcessImageW.Call(uintptr(CurrentProcess()), 0,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
	if r == 0 || n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

// ModuleFileName returns the path of the loaded module (the exe for nil).
func ModuleFileName() string {
	buf := make([]uint16, MAX_PATH)
	n, _, _ := pGetModuleFileNameW.Call(uintptr(ModuleHandle()),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf[:n])
}

// IsElevated reports whether the process runs with an elevated token.
func IsElevated() bool {
	var tok HANDLE
	r, _, _ := pOpenProcessToken.Call(uintptr(CurrentProcess()), TOKEN_QUERY,
		uintptr(unsafe.Pointer(&tok)))
	if r == 0 {
		return false
	}
	defer pCloseHandle.Call(uintptr(tok))
	var elevated, retLen uint32
	r, _, _ = pGetTokenInformation.Call(uintptr(tok), TokenElevation,
		uintptr(unsafe.Pointer(&elevated)), 4, uintptr(unsafe.Pointer(&retLen)))
	return r != 0 && elevated != 0
}

// LocalFree releases memory returned by the shell (e.g. the file name list of
// a drag and drop operation).
func LocalFree(p uintptr) {
	if p != 0 {
		pLocalFree.Call(p)
	}
}

// AllowSetForeground drops the foreground lock for this process. A process
// that handles an input event may call SetForegroundWindow; this makes the
// following calls reliable in sessions where the lock is held elsewhere.
func AllowSetForeground() {
	pAllowSetForeground.Call(uintptr(ASFW_ANY))
}

// EnsureForeground moves a window to the top of the z order and asks for the
// keyboard focus. A global hotkey handler is allowed to do this even when it
// is not the foreground process, which is why the launcher calls it from the
// hook thread through a posted message rather than from the UI thread.
func EnsureForeground(h HWND) {
	if h == 0 {
		return
	}
	BringWindowToTop(h)
	SetForegroundWindow(h)
	pSetWindowPos.Call(uintptr(h), uintptr(HWND_TOPMOST), 0, 0, 0, 0,
		uintptr(SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE))
}

// BringWindowToTop is a thin wrapper used by EnsureForeground.
func BringWindowToTop(h HWND) {
	pBringWindowToTop.Call(uintptr(h))
}

// ActivateAppThread unlocks SetForegroundWindow by attaching the input queue
// to the foreground thread before the launcher tries to take the focus.
func ActivateAppThread() {
	tid := GetWindowThreadProcessId(GetForegroundWindow())
	me := GetCurrentThreadId()
	if tid != 0 && tid != me {
		pAttachThreadInput.Call(uintptr(me), uintptr(tid), 1)
		defer pAttachThreadInput.Call(uintptr(me), uintptr(tid), 0)
	}
}

// GetWindowThreadProcessId returns the thread that owns a window.
func GetWindowThreadProcessId(h HWND) uint32 {
	r, _, _ := pGetWindowThreadPID.Call(uintptr(h), 0)
	return uint32(r)
}

// GetCurrentThreadId returns the calling thread id.
func GetCurrentThreadId() uint32 {
	r, _, _ := pGetCurrentThreadID.Call()
	return uint32(r)
}

// TrackMouseLeave asks for a WM_MOUSELEAVE once the cursor leaves the window.
func TrackMouseLeave(h HWND) {
	type trackMouseEvent struct {
		Size      uint32
		Flags     uint32
		HWndTrack HWND
		HoverTime uint32
	}
	ev := trackMouseEvent{Size: uint32(unsafe.Sizeof(trackMouseEvent{})), Flags: 2 /* TME_LEAVE */, HWndTrack: h}
	pTrackMouseEvent.Call(uintptr(unsafe.Pointer(&ev)))
}
