//go:build windows

package win

import (
	"unsafe"
)

// Tray icon (Shell_NotifyIcon) and the context menu shown from it.

type Tray struct {
	data NOTIFYICONDATAW
	hwnd HWND
	icon HICON
}

// NewTray registers a tray icon whose mouse events are posted to cbMsg.
func NewTray(hwnd HWND, icon HICON, tip string, cbMsg uint32) *Tray {
	t := &Tray{hwnd: hwnd, icon: icon}
	d := &t.data
	d.CbSize = uint32(unsafe.Sizeof(NOTIFYICONDATAW{}))
	d.HWnd = hwnd
	d.UID = 1
	d.UFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP | NIF_SHOWTIP
	d.UCallbackMessage = cbMsg
	d.HIcon = icon
	CopyUTF16(d.SzTip[:], tip)
	t.Add()
	d.UVersion = NOTIFYICON_VERSION_4
	pShellNotifyIconW.Call(NIM_SETVERSION, uintptr(unsafe.Pointer(&t.data)), 0)
	d.UVersion = 0
	return t
}

// Add (re)adds the icon; Explorer drops it on restart, so this is also the
// handler for the TaskbarCreated broadcast.
func (t *Tray) Add() bool {
	r, _, _ := pShellNotifyIconW.Call(NIM_ADD, uintptr(unsafe.Pointer(&t.data)))
	return r != 0
}

// SetIcon swaps the current icon.
func (t *Tray) SetIcon(icon HICON) {
	if icon == 0 || icon == t.icon {
		return
	}
	t.icon = icon
	t.data.HIcon = icon
	t.data.UFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP | NIF_SHOWTIP
	pShellNotifyIconW.Call(NIM_MODIFY, uintptr(unsafe.Pointer(&t.data)))
}

// SetTip updates the hover tooltip.
func (t *Tray) SetTip(tip string) {
	CopyUTF16(t.data.SzTip[:], tip)
	t.data.UFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP | NIF_SHOWTIP
	pShellNotifyIconW.Call(NIM_MODIFY, uintptr(unsafe.Pointer(&t.data)))
}

// Balloon shows a notification bubble.
func (t *Tray) Balloon(title, text string, flags uint32) {
	d := &t.data
	d.UFlags = NIF_INFO
	CopyUTF16(d.SzInfo[:], text)
	CopyUTF16(d.SzInfoTitle[:], title)
	d.DwInfoFlags = flags
	pShellNotifyIconW.Call(NIM_MODIFY, uintptr(unsafe.Pointer(d)))
	d.UFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP | NIF_SHOWTIP
}

func (t *Tray) Remove() {
	pShellNotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(&t.data)))
}

// Menu is a lightweight popup menu wrapper.
type Menu struct{ h HMENU }

func NewPopupMenu() *Menu {
	r, _, _ := pCreatePopupMenu.Call()
	return &Menu{h: HMENU(r)}
}

func (m *Menu) Add(id uint32, text string, checked bool) {
	flags := uint32(MF_STRING)
	if checked {
		flags |= MF_CHECKED
	}
	pAppendMenuW.Call(uintptr(m.h), uintptr(flags), uintptr(id), uintptr(unsafe.Pointer(UTF16Ptr(text))))
}

// AddInfo adds a disabled line used for headers.
func (m *Menu) AddInfo(text string) {
	pAppendMenuW.Call(uintptr(m.h), MF_STRING|MF_GRAYED|MF_DISABLED, 0, uintptr(unsafe.Pointer(UTF16Ptr(text))))
}

func (m *Menu) Separator() {
	pAppendMenuW.Call(uintptr(m.h), MF_SEPARATOR, 0, 0)
}

func (m *Menu) Default(id uint32) { pSetMenuDefault.Call(uintptr(m.h), uintptr(id), 0) }

func (m *Menu) Destroy() {
	if m.h != 0 {
		pDestroyMenu.Call(uintptr(m.h))
		m.h = 0
	}
}

// Track runs the menu at the cursor and returns the chosen command id (0 when
// dismissed).
func (m *Menu) Track(owner HWND) uint32 {
	pt := CursorPos()
	SetForegroundWindow(owner)
	r, _, _ := pTrackPopupMenu.Call(uintptr(m.h),
		TPM_RETURNCMD|TPM_RIGHTBUTTON|TPM_NONOTIFY,
		uintptr(pt.X), uintptr(pt.Y), 0, uintptr(owner), 0)
	PostMessage(owner, WM_NULL, 0, 0)
	return uint32(r)
}

// SmallIconSize is the size Windows wants for tray icons.
func SmallIconSize() int {
	n := GetSystemMetrics(SM_CXSMICON)
	if n <= 0 {
		return 16
	}
	return int(n)
}
