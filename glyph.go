//go:build windows

package main

import (
	"github.com/dvygolov/yalauncher/internal/win"
)

// drawGlyph renders one or two characters into a square DIB. It is the icon
// shown for entries whose file has no shell icon (a calculator result, a
// program whose exe vanished from under its shortcut).
func drawGlyph(size int32, text string, color uint32, dpi uint32) *win.MemDC {
	if size <= 0 {
		return nil
	}
	dc := win.NewMemDC(size, size)
	if dc == nil {
		return nil
	}
	scale := float64(dpi) / 96
	if dpi == 0 {
		scale = 1
	}
	// rounded tile
	brush := win.SolidBrush(mix(color, 0.35))
	pen := win.Pen(mix(color, 0.1), 1)
	win.RoundRect(dc.DC, win.Rect(0, 0, size, size), int(size/4), pen, brush)
	win.DeleteObject(win.HGDIOBJ(brush))
	win.DeleteObject(win.HGDIOBJ(pen))
	font := win.Font{Name: "Segoe UI", Height: -int32(14 * scale), Weight: win.FW_SEMIBOLD}.HFONT()
	if font != 0 {
		win.Text(dc.DC, text, 0, size/2, font, 0xFFFFFF, win.DT_CENTER|win.DT_VCENTER, size)
		win.DeleteObject(win.HGDIOBJ(font))
	}
	return dc
}

// mix darkens a COLORREF by the given fraction.
func mix(c uint32, factor float64) uint32 {
	r := float64(c & 0xFF)
	g := float64((c >> 8) & 0xFF)
	b := float64((c >> 16) & 0xFF)
	r *= 1 - factor
	g *= 1 - factor
	b *= 1 - factor
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16
}
