//go:build windows

package win

import (
	"syscall"
)

type HWND uintptr
type HICON uintptr
type HMENU uintptr
type HINSTANCE uintptr
type HANDLE uintptr
type HKEY uintptr
type HDC uintptr
type HBITMAP uintptr
type HGDIOBJ uintptr
type HBRUSH uintptr
type HPEN uintptr
type HFONT uintptr
type HMONITOR uintptr
type HRESULT int32
type LPARAM uintptr
type WPARAM uintptr
type LRESULT uintptr
type ATOM uint16

type POINT struct{ X, Y int32 }

func (p POINT) Pack() uintptr { return uintptr(uint32(p.X)) | uintptr(uint32(p.Y))<<16 }

type RECT struct{ Left, Top, Right, Bottom int32 }

func (r RECT) Width() int32  { return r.Right - r.Left }
func (r RECT) Height() int32 { return r.Bottom - r.Top }
func (r RECT) Contains(p POINT) bool {
	return p.X >= r.Left && p.X < r.Right && p.Y >= r.Top && p.Y < r.Bottom
}

type MSG struct {
	HWnd    HWND
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
	Private uint32
}

type WNDCLASSEXW struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   HINSTANCE
	Icon       HICON
	HCursor    uintptr
	Background HBRUSH
	MenuName   *uint16
	ClassName  *uint16
	IconSm     HICON
}

type PAINTSTRUCT struct {
	Hdc         HDC
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

type MONITORINFO struct {
	CbSize    uint32
	RcMonitor RECT
	RcWork    RECT
	DwFlags   uint32
}

const MONITOR_DEFAULTTONEAREST = 2

type NOTIFYICONDATAW struct {
	CbSize           uint32
	HWnd             HWND
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            HICON
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         [16]byte
	HBalloonIcon     HICON
}

type KBDLLHOOKSTRUCT struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type SHFILEINFOW struct {
	HIcon         HICON
	IIcon         int32
	DwAttributes  uint32
	SzDisplayName [MAX_PATH]uint16
	SzTypeName    [80]uint16
}

type KEYBDINPUT struct {
	Vk    uint16
	Scan  uint16
	Flags uint32
	Time  uint32
	Extra uintptr
}

type MOUSEINPUT struct {
	Dx, Dy    int32
	MouseData uint32
	Flags     uint32
	Time      uint32
	Extra     uintptr
}

type HARDWAREINPUT struct {
	Msg    uint32
	ParamL uint16
	ParamH uint16
}

type INPUT struct {
	Type uint32
	_    uint32
	Ki   KEYBDINPUT
	_    [8]byte
}

const INPUT_KEYBOARD = 1

type BITMAPINFOHEADER struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type BITMAPINFO struct {
	Header BITMAPINFOHEADER
	Colors [1]uint32
}

type BLENDFUNCTION struct {
	BlendOp             byte
	BlendFlags          byte
	SourceConstantAlpha byte
	AlphaFormat         byte
}

func (b BLENDFUNCTION) Pack() uint32 {
	return uint32(b.BlendOp) | uint32(b.BlendFlags)<<8 | uint32(b.SourceConstantAlpha)<<16 | uint32(b.AlphaFormat)<<24
}

// UTF16Ptr converts a Go string to a NUL terminated UTF-16 pointer. The result
// is intentionally not freed: call sites pass it straight to an API call, and
// the garbage collector owns the backing array for as long as it is alive.
func UTF16Ptr(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		// Embedded NUL: fall back to a truncated copy.
		return syscall.StringToUTF16Ptr(string([]rune(s)))
	}
	return p
}

// CopyUTF16 copies s into a fixed-size UTF-16 buffer, always terminating it.
func CopyUTF16(dst []uint16, s string) {
	if len(dst) == 0 {
		return
	}
	n := len(dst) - 1
	i := 0
	for _, r := range s {
		if i >= n {
			break
		}
		if r > 0xFFFF {
			r = '?'
		}
		dst[i] = uint16(r)
		i++
	}
	dst[i] = 0
}

func Rect(l, t, rr, b int32) RECT { return RECT{Left: l, Top: t, Right: rr, Bottom: b} }

// TEXTMETRICW is the GDI text metrics record; only Ascent is read.
type TEXTMETRICW struct {
	Height           int32
	Ascent           int32
	Descent          int32
	InternalLeading  int32
	ExternalLeading  int32
	AveCharWidth     int32
	MaxCharWidth     int32
	Weight           int32
	Overhang         int32
	DigitizedAspectX int32
	DigitizedAspectY int32
	FirstChar        uint16
	LastChar         uint16
	DefaultChar      uint16
	BreakChar        uint16
	Italic           byte
	Underlined       byte
	StruckOut        byte
	PitchAndFamily   byte
	CharSet          byte
}
