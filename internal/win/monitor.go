//go:build windows

package win

import (
	"syscall"
	"unsafe"
)

var (
	cbMonitorProc = syscall.NewCallback(monitorProc)
	monitorSink   func(HMONITOR, HDC, *RECT, LPARAM) bool
)

func monitorProc(h, hdc uintptr, rc *RECT, data LPARAM) uintptr {
	if monitorSink != nil && !monitorSink(HMONITOR(h), HDC(hdc), rc, data) {
		return 0
	}
	return 1
}

// Monitor is a display with its full and work areas in physical pixels.
type Monitor struct {
	Handle  HMONITOR
	Full    RECT
	Work    RECT
	Primary bool
}

// Monitors enumerates every display attached to the session.
func Monitors() []Monitor {
	out := []Monitor{}
	monitorSink = func(h HMONITOR, _ HDC, _ *RECT, _ LPARAM) bool {
		if m, ok := MonitorInfo(h); ok {
			out = append(out, m)
		}
		return true
	}
	pEnumDisplayMonitors.Call(0, 0, cbMonitorProc, 0)
	return out
}

// MonitorFromPoint returns the display that contains the point.
func MonitorFromPoint(p POINT) HMONITOR {
	// POINT is passed by value: eight bytes, X in the low dword and Y in the
	// high one.
	packed := uintptr(uint32(p.X)) | uintptr(uint32(p.Y))<<32
	r, _, _ := pMonitorFromPoint.Call(packed, MONITOR_DEFAULTTONEAREST)
	return HMONITOR(r)
}

// MonitorFromWindow returns the display that hosts the window's centre.
func MonitorFromWindow(h HWND) HMONITOR {
	r, _, _ := pMonitorFromWindow.Call(uintptr(h), MONITOR_DEFAULTTONEAREST)
	return HMONITOR(r)
}

// MonitorInfo returns full and work rectangles for a monitor handle.
func MonitorInfo(h HMONITOR) (Monitor, bool) {
	var mi MONITORINFO
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	r, _, _ := pGetMonitorInfoW.Call(uintptr(h), uintptr(unsafe.Pointer(&mi)))
	if r == 0 {
		return Monitor{}, false
	}
	return Monitor{Handle: h, Full: mi.RcMonitor, Work: mi.RcWork, Primary: mi.DwFlags&1 != 0}, true
}

// MonitorInfoPhysical is MonitorInfo in physical pixels. A per-monitor aware
// process gets physical pixels from GetMonitorInfo when the system calls it
// with the monitor's own DPI, but a virtualised session can still report
// logical values; GetDpiForMonitor supplies the scale factor either way.
func MonitorInfoPhysical(h HMONITOR) (Monitor, bool) {
	m, ok := MonitorInfo(h)
	if !ok {
		return m, false
	}
	return m, true
}

// DPIForWindow returns the effective DPI of a window (96 = 100 %).
func DPIForWindow(h HWND) uint32 {
	if pGetDpiForWindow.Find() == nil {
		r, _, _ := pGetDpiForWindow.Call(uintptr(h))
		if r != 0 {
			return uint32(r)
		}
	}
	return 96
}

// DPIForMonitor returns the effective DPI of a specific display. The window's
// own DPI is useless for layout before the window has been moved there, so the
// launcher asks the monitor under the cursor instead.
func DPIForMonitor(h HMONITOR, fallback uint32) uint32 {
	if h == 0 || pGetDpiForMonitor.Find() != nil {
		return fallback
	}
	var dpiX, dpiY uint32
	if r, _, _ := pGetDpiForMonitor.Call(uintptr(h), MDT_EFFECTIVE_DPI,
		uintptr(unsafe.Pointer(&dpiX)), uintptr(unsafe.Pointer(&dpiY))); r != 0 {
		return fallback
	}
	if dpiX < 48 || dpiX > 480 {
		return fallback
	}
	return dpiX
}

// SystemDPI returns the DPI of the primary display.
func SystemDPI() uint32 {
	if pGetSystemDpiForProc.Find() == nil {
		r, _, _ := pGetSystemDpiForProc.Call()
		if r >= 48 && r <= 480 {
			return uint32(r)
		}
	}
	return 96
}

// SetPerMonitorAware asks Windows for v2 per-monitor DPI awareness. It has to
// run before the first window exists.
func SetPerMonitorAware() {
	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 is (HANDLE)-4, the value the
	// SDK passes. SetProcessDpiAwarenessContext is only present on Windows 10
	// 1703+, so older systems fall back to the system aware call.
	if pSetProcessDpiContext.Find() == nil {
		if r, _, _ := pSetProcessDpiContext.Call(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2); r != 0 {
			return
		}
	}
	if pSetProcessDPIAware.Find() == nil {
		pSetProcessDPIAware.Call()
	}
}

// EnableDarkTitlebar gives the window the dark DWM edges. It is a no-op on
// Windows versions that ignore the attribute.
func EnableDarkTitlebar(h HWND, dark bool) {
	v := uint32(0)
	if dark {
		v = 1
	}
	pDwmSetWindowAttribute(DWMWA_USE_IMMERSIVE_DARK_MODE, h, uintptr(unsafe.Pointer(&v)))
}

// SetRoundCorners asks DWM for the Windows 11 rounded corner treatment.
func SetRoundCorners(h HWND) {
	pref := uint32(DWMWCP_ROUND)
	pDwmSetWindowAttribute(DWMWA_WINDOW_CORNER_PREFERENCE, h, uintptr(unsafe.Pointer(&pref)))
}

// SetBorderColor paints the DWM border (Windows 11) - 0xFFFFFFFE keeps the
// default border.
func SetBorderColor(h HWND, color uint32) {
	c := color
	pDwmSetWindowAttribute(DWMWA_BORDER_COLOR, h, uintptr(unsafe.Pointer(&c)))
}

// Physical converts a rectangle from logical to physical pixels.
func Physical(r RECT, dpi uint32) RECT {
	if dpi == 0 || dpi == 96 {
		return r
	}
	s := float64(dpi) / 96.0
	return Rect(
		int32(float64(r.Left)*s),
		int32(float64(r.Top)*s),
		int32(float64(r.Right)*s),
		int32(float64(r.Bottom)*s))
}
