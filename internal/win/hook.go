//go:build windows

package win

import (
	"syscall"
	"unsafe"
)

// KeyEvent is a normalised keyboard event from the low level hook.
type KeyEvent struct {
	VK       uint32
	Scan     uint32
	Flags    uint32
	Down     bool
	Injected bool
	// Ours marks input synthesised by this very process.
	Ours bool
}

// KeyHandler observes every key. It must return fast: it runs inside the input
// path of the whole system.
type KeyHandler func(KeyEvent)

var (
	keyHandler KeyHandler
	keyHookCB  = syscall.NewCallback(keyboardProc)
	hookHandle HANDLE

	// Hotkey is a WH_KEYBOARD_LL filter that can swallow a key. Returning
	// true consumes the event: the launcher uses it so Ctrl+Alt+Space never
	// reaches the application below.
	Hotkey func(KeyEvent) bool
)

func keyboardProc(nCode int32, wparam, lparam uintptr) uintptr {
	if nCode == HC_ACTION {
		kb := (*KBDLLHOOKSTRUCT)(unsafe.Pointer(lparam))
		ev := KeyEvent{
			VK:       kb.VkCode,
			Scan:     kb.ScanCode,
			Flags:    kb.Flags,
			Down:     wparam == WM_KEYDOWN || wparam == WM_SYSKEYDOWN,
			Injected: kb.Flags&LLKHF_INJECTED != 0,
			Ours:     kb.DwExtraInfo == MagicInjectedInput,
		}
		if Hotkey != nil && Hotkey(ev) {
			return 1
		}
		if keyHandler != nil {
			keyHandler(ev)
		}
	}
	r, _, _ := pCallNextHookEx.Call(uintptr(hookHandle), uintptr(nCode), wparam, lparam)
	return r
}

// InstallKeyboardHook installs a global WH_KEYBOARD_LL observer. It must be
// called from the thread that runs the message loop. handler may be nil when
// only the Hotkey filter is needed.
func InstallKeyboardHook(handler KeyHandler) error {
	keyHandler = handler
	h, _, e := pSetWindowsHookExW.Call(WH_KEYBOARD_LL, keyHookCB, uintptr(ModuleHandle()), 0)
	if h == 0 {
		if errno, ok := e.(syscall.Errno); ok && errno != 0 {
			return errno
		}
		return syscall.EINVAL
	}
	hookHandle = HANDLE(h)
	return nil
}

// RemoveHotkey drops the filter but keeps the observer.
func RemoveHotkey() { Hotkey = nil }

func RemoveKeyboardHook() {
	if hookHandle != 0 {
		pUnhookWindowsHookEx.Call(uintptr(hookHandle))
		hookHandle = 0
	}
	keyHandler = nil
}

// KeyDown reports whether the key is currently held, as seen by the input
// queue (modifiers included).
func KeyDown(vk uintptr) bool {
	r, _, _ := pGetAsyncKeyState.Call(vk)
	return uint16(r)&0x8000 != 0
}

// SendInputKey synthesises one key press or release.
func SendInputKey(vk uint16, scan uint16, up bool) bool {
	flags := uint32(0)
	if up {
		flags = KEYEVENTF_KEYUP
	}
	in := INPUT{Type: INPUT_KEYBOARD, Ki: KEYBDINPUT{
		Vk:    vk,
		Scan:  scan,
		Flags: flags,
		Extra: MagicInjectedInput,
	}}
	r, _, _ := pSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
	return r == 1
}

// SendUnicodeText types text into the focused window as WM_CHAR-equivalent
// input. It is only used when selection actions assign a hotkey to a keystroke
// that cannot be represented by a virtual key.
func SendUnicodeText(s string) bool {
	units := syscall.StringToUTF16(s)
	if len(units) > 1 {
		units = units[:len(units)-1]
	}
	var seq []INPUT
	for _, u := range units {
		for _, up := range []bool{false, true} {
			flags := uint32(KEYEVENTF_UNICODE)
			if up {
				flags |= KEYEVENTF_KEYUP
			}
			seq = append(seq, INPUT{Type: INPUT_KEYBOARD, Ki: KEYBDINPUT{Scan: u, Flags: flags, Extra: MagicInjectedInput}})
		}
	}
	if len(seq) == 0 {
		return true
	}
	r, _, _ := pSendInput.Call(uintptr(len(seq)), uintptr(unsafe.Pointer(&seq[0])), unsafe.Sizeof(seq[0]))
	return int(r) == len(seq)
}

// KeyboardLayouts lists the keyboard layouts installed for the current user.
func KeyboardLayouts() []uintptr {
	r, _, _ := pGetKeyboardLayouts.Call(0, 0)
	n := int32(r)
	if n <= 0 || n > 64 {
		return nil
	}
	list := make([]uintptr, n)
	pGetKeyboardLayouts.Call(uintptr(n), uintptr(unsafe.Pointer(&list[0])))
	return list
}

// PrimaryLanguage maps a keyboard layout handle to its primary language id
// (0x09 English, 0x19 Russian, ...).
func PrimaryLanguage(hkl uintptr) uint32 {
	return uint32(hkl) & 0x3FF
}

// ---- mouse hook ------------------------------------------------------------
//
// A low level mouse hook watches for the click that dismisses the launcher:
// the window never has to own the focus for that, which is exactly the
// situation the keyboard input is designed around as well.

const WH_MOUSE_LL = 14

// MouseEvent is a normalised mouse event from the hook.
type MouseEvent struct {
	X, Y int32
	Down bool
}

var (
	mouseHandler  func(MouseEvent)
	mouseHookCB   = syscall.NewCallback(mouseProc)
	mouseHookInst HANDLE
)

type MSLLHOOKSTRUCT struct {
	X, Y  int32
	_     uint32 // mouse data (hi/lo words depend on the message)
	Flags uint32
	Time  uint32
	Extra uintptr
}

func mouseProc(nCode int32, wparam, lparam uintptr) uintptr {
	if nCode == HC_ACTION && mouseHandler != nil {
		const wmLDown, wmRDown, wmMDown = 0x201, 0x204, 0x207
		down := wparam == wmLDown || wparam == wmRDown || wparam == wmMDown
		if down {
			m := (*MSLLHOOKSTRUCT)(unsafe.Pointer(lparam))
			mouseHandler(MouseEvent{X: m.X, Y: m.Y, Down: true})
		}
	}
	r, _, _ := pCallNextHookEx.Call(uintptr(mouseHookInst), uintptr(nCode), wparam, lparam)
	return r
}

// InstallMouseHook installs a global WH_MOUSE_LL observer. It must be called
// from the thread that runs the message loop.
func InstallMouseHook(handler func(MouseEvent)) error {
	mouseHandler = handler
	h, _, e := pSetWindowsHookExW.Call(WH_MOUSE_LL, mouseHookCB, uintptr(ModuleHandle()), 0)
	if h == 0 {
		if errno, ok := e.(syscall.Errno); ok && errno != 0 {
			return errno
		}
		return syscall.EINVAL
	}
	mouseHookInst = HANDLE(h)
	return nil
}

func RemoveMouseHook() {
	if mouseHookInst != 0 {
		pUnhookWindowsHookEx.Call(uintptr(mouseHookInst))
		mouseHookInst = 0
	}
	mouseHandler = nil
}
