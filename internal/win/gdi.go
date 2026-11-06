//go:build windows

package win

import (
	"runtime"
	"syscall"
	"unsafe"
)

// GDI drawing helpers. The launcher paints its whole surface itself, so these
// are deliberately low level: a device context, pens, brushes, a handful of
// drawing verbs and three fonts.

// SolidBrush creates a brush from a COLORREF (0x00BBGGRR).
func SolidBrush(color uint32) HBRUSH {
	r, _, _ := pCreateSolidBr.Call(uintptr(color))
	return HBRUSH(r)
}

// Pen creates a cosmetic pen of the given width.
func Pen(color uint32, width int) HPEN {
	r, _, _ := pCreatePen.Call(PS_SOLID, uintptr(width), uintptr(color))
	return HPEN(r)
}

func DeleteObject(o HGDIOBJ) {
	if o != 0 {
		pDeleteObject.Call(uintptr(o))
	}
}

func SelectObject(dc HDC, o HGDIOBJ) HGDIOBJ {
	if r, _, _ := pSelectObject.Call(uintptr(dc), uintptr(o)); r != 0 {
		return HGDIOBJ(r)
	}
	return HGDIOBJ(o)
}

// FillRect fills a rectangle with a brush.
func FillRect(dc HDC, r RECT, brush HBRUSH) {
	pFillRect.Call(uintptr(dc), uintptr(unsafe.Pointer(&r)), uintptr(brush))
}

// SolidRect is the common case: one fill, no brush handle to manage.
func SolidRect(dc HDC, r RECT, color uint32) {
	b := SolidBrush(color)
	if b != 0 {
		FillRect(dc, r, b)
		DeleteObject(HGDIOBJ(b))
	}
}

// RoundRect draws a filled rounded rectangle with a pen and a brush.
func RoundRect(dc HDC, r RECT, radius int, pen HPEN, brush HBRUSH) {
	op := SelectObject(dc, HGDIOBJ(pen))
	ob := SelectObject(dc, HGDIOBJ(brush))
	pRoundRect.Call(uintptr(dc), uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom),
		uintptr(radius*2), uintptr(radius*2))
	SelectObject(dc, op)
	SelectObject(dc, ob)
}

// Rect is a filled rectangle with a 1px border.
func RectOutlined(dc HDC, r RECT, radius int, pen HPEN, brush HBRUSH) {
	RoundRect(dc, r, radius, pen, brush)
}

// Line draws a single pixel line.
func Line(dc HDC, x1, y1, x2, y2 int32, pen HPEN) {
	op := SelectObject(dc, HGDIOBJ(pen))
	pMoveToEx.Call(uintptr(dc), uintptr(x1), uintptr(y1), 0)
	pLineTo.Call(uintptr(dc), uintptr(x2), uintptr(y2))
	SelectObject(dc, op)
}

// Font describes a GDI font request.
type Font struct {
	Name   string
	Height int32 // negative = character height
	Weight int32
	Italic bool
}

// HFONT creates the font handle.
func (f Font) HFONT() HFONT {
	italic := uintptr(0)
	if f.Italic {
		italic = 1
	}
	// CreateFontW takes fourteen parameters: height, width, escapement,
	// orientation, weight, italic, underline, strike out, charset, output
	// precision, clip precision, quality, pitch and family, face name.
	r, _, _ := pCreateFontW.Call(
		uintptr(f.Height), 0, 0, 0,
		uintptr(f.Weight), uintptr(italic), 0, 0,
		DEFAULT_CHARSET, OUT_DEFAULT_PRECIS, CLIP_DEFAULT_PRECIS, CLEARTYPE_QUALITY,
		DEFAULT_PITCH|FF_DONTCARE, uintptr(unsafe.Pointer(UTF16Ptr(f.Name))),
	)
	return HFONT(r)
}

// Text draws one line of text. x,y is the top-left of the line box: the text
// is placed inside [y, y+TextHeight]. Vertical centring across a full row is
// the caller's business, because the caller knows the row height.
func Text(dc HDC, s string, x, top int32, font HFONT, color uint32, align uint32, width int32) int32 {
	if font == 0 || dc == 0 {
		return 0
	}
	text := s
	if max := int(width / 8); width > 0 && len(text) > max {
		text = text[:max]
	}
	units := syscall.StringToUTF16(text)
	if len(units) > 0 && units[len(units)-1] == 0 {
		units = units[:len(units)-1]
	}
	if len(units) == 0 {
		return x
	}
	old := SelectObject(dc, HGDIOBJ(font))
	oldColor, _, _ := pSetTextColor.Call(uintptr(dc), uintptr(color))
	pSetBkMode.Call(uintptr(dc), TRANSPARENT)

	// TextOutW is used rather than DrawTextW: on the GDI path this app draws
	// on, DrawTextW accepts the call and returns a glyph count while nothing
	// reaches the surface, whereas the primitive TextOutW renders normally.
	// TextOutW positions by the baseline, and the baseline sits one internal
	// leading plus one ascent below the top of the visible glyphs.
	ty := top + TextAscent(dc, font) - TextInternalLeading(dc, font)
	pTextOutW.Call(uintptr(dc), uintptr(x), uintptr(ty),
		uintptr(unsafe.Pointer(&units[0])), uintptr(len(units)))
	runtime.KeepAlive(units)
	pSetTextColor.Call(uintptr(dc), oldColor)
	SelectObject(dc, old)
	return x + TextWidth(dc, text, font)
}

// TextWidth measures a string with the given font.
func TextWidth(dc HDC, s string, font HFONT) int32 {
	if dc == 0 || font == 0 || s == "" {
		return 0
	}
	units := syscall.StringToUTF16(s)
	if len(units) > 0 && units[len(units)-1] == 0 {
		units = units[:len(units)-1]
	}
	if len(units) == 0 {
		return 0
	}
	old := SelectObject(dc, HGDIOBJ(font))
	var sz struct{ X, Y int32 }
	pGetTextExtentP.Call(uintptr(dc), uintptr(unsafe.Pointer(&units[0])), uintptr(len(units)), uintptr(unsafe.Pointer(&sz)))
	SelectObject(dc, old)
	return sz.X
}

// TextAscent returns the distance from the top of the line box to the text
// baseline. TextOutW draws from the baseline, the rest of the launcher places
// text by the top of the line.
func TextAscent(dc HDC, font HFONT) int32 {
	if dc == 0 || font == 0 {
		return 0
	}
	old := SelectObject(dc, HGDIOBJ(font))
	defer SelectObject(dc, old)
	var tm TEXTMETRICW
	r, _, _ := pGetTextMetricsW.Call(uintptr(dc), uintptr(unsafe.Pointer(&tm)))
	if r == 0 {
		return 0
	}
	// TextOutW draws from the baseline, and the caller places by the top of
	// the text: the baseline sits one ascent below that top.
	return tm.Ascent
}

// TextInternalLeading returns the space GDI reserves above the ascent.
func TextInternalLeading(dc HDC, font HFONT) int32 {
	if dc == 0 || font == 0 {
		return 0
	}
	old := SelectObject(dc, HGDIOBJ(font))
	defer SelectObject(dc, old)
	var tm TEXTMETRICW
	r, _, _ := pGetTextMetricsW.Call(uintptr(dc), uintptr(unsafe.Pointer(&tm)))
	if r == 0 {
		return 0
	}
	return tm.InternalLeading
}

// TextMetrics returns the ascent, descent and internal leading of a font on a
// DC. These are the numbers needed to centre a line by its baseline.
type TextMetrics struct{ Ascent, Descent, InternalLeading int32 }

func GetTextMetrics(dc HDC, font HFONT) TextMetrics {
	var m TextMetrics
	if dc == 0 || font == 0 {
		return m
	}
	old := SelectObject(dc, HGDIOBJ(font))
	defer SelectObject(dc, old)
	var tm TEXTMETRICW
	if r, _, _ := pGetTextMetricsW.Call(uintptr(dc), uintptr(unsafe.Pointer(&tm))); r != 0 {
		m.Ascent, m.Descent, m.InternalLeading = tm.Ascent, tm.Descent, tm.InternalLeading
	}
	return m
}

// TextBaseline draws one line whose baseline is exactly at the given y. This
// is the correct way to centre a line in a field: the caller places the
// baseline, not the top of a line box.
func TextBaseline(dc HDC, s string, x, baseline int32, font HFONT, color uint32, width int32) int32 {
	if font == 0 || dc == 0 {
		return x
	}
	text := s
	if max := int(width / 8); width > 0 && len(text) > max {
		text = text[:max]
	}
	units := syscall.StringToUTF16(text)
	if len(units) > 0 && units[len(units)-1] == 0 {
		units = units[:len(units)-1]
	}
	if len(units) == 0 {
		return x
	}
	old := SelectObject(dc, HGDIOBJ(font))
	oldColor, _, _ := pSetTextColor.Call(uintptr(dc), uintptr(color))
	pSetBkMode.Call(uintptr(dc), TRANSPARENT)
	pTextOutW.Call(uintptr(dc), uintptr(x), uintptr(baseline),
		uintptr(unsafe.Pointer(&units[0])), uintptr(len(units)))
	runtime.KeepAlive(units)
	pSetTextColor.Call(uintptr(dc), oldColor)
	SelectObject(dc, old)
	return x + TextWidth(dc, text, font)
}

// TextCentered draws one line vertically centred in the given rect using
// DrawTextW with DT_VCENTER. DrawTextW lives in user32.dll, not gdi32, which
// is why the original binding silently failed until the DLL was corrected.
func TextCentered(dc HDC, s string, x, top int32, font HFONT, color uint32, width, height int32) int32 {
	if font == 0 || dc == 0 {
		return x
	}
	units := syscall.StringToUTF16(s)
	if len(units) > 0 && units[len(units)-1] == 0 {
		units = units[:len(units)-1]
	}
	if len(units) == 0 {
		return x
	}
	old := SelectObject(dc, HGDIOBJ(font))
	oldColor, _, _ := pSetTextColor.Call(uintptr(dc), uintptr(color))
	pSetBkMode.Call(uintptr(dc), TRANSPARENT)
	r := RECT{Left: x, Top: top, Right: x + width, Bottom: top + height}
	pDrawTextW.Call(uintptr(dc), uintptr(unsafe.Pointer(&units[0])), ^uintptr(0),
		uintptr(unsafe.Pointer(&r)), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)

	runtime.KeepAlive(units)
	pSetTextColor.Call(uintptr(dc), oldColor)
	SelectObject(dc, old)
	return x + TextWidth(dc, s, font)
}

// TextHeight returns the cell height of the font.
func TextHeight(dc HDC, font HFONT) int32 {
	if dc == 0 || font == 0 {
		return 0
	}
	old := SelectObject(dc, HGDIOBJ(font))
	var sz struct{ X, Y int32 }
	one := syscall.StringToUTF16("Ag")
	one = one[:2]
	pGetTextExtentP.Call(uintptr(dc), uintptr(unsafe.Pointer(&one[0])), 2, uintptr(unsafe.Pointer(&sz)))
	SelectObject(dc, old)
	return sz.Y
}

// WindowDC returns a device context for the whole window.
func WindowDC(h HWND) HDC {
	r, _, _ := pGetDC.Call(uintptr(h))
	return HDC(r)
}

func ReleaseDC(h HWND, dc HDC) { pReleaseDC.Call(uintptr(h), uintptr(dc)) }

// BeginPaint/EndPaint wrap a WM_PAINT handler.
func BeginPaint(h HWND) (HDC, PAINTSTRUCT) {
	var ps PAINTSTRUCT
	r, _, _ := pBeginPaint.Call(uintptr(h), uintptr(unsafe.Pointer(&ps)))
	return HDC(r), ps
}

func EndPaint(h HWND, ps *PAINTSTRUCT) {
	pEndPaint.Call(uintptr(h), uintptr(unsafe.Pointer(ps)))
}

// MemDC is an off-screen 32 bpp surface with alpha, used for icon extraction
// and for drawing anti-aliased rounded corners.
type MemDC struct {
	DC     HDC
	Bitmap HBITMAP
	Old    HGDIOBJ
	Pixels uintptr
	Width  int32
	Height int32
}

// NewMemDC creates a DIB section of the requested size whose pixels are
// accessible as premultiplied BGRA through Pixels.
func NewMemDC(w, h int32) *MemDC {
	dc, _, _ := pCreateCompatibleDC.Call(0)
	if dc == 0 {
		return nil
	}
	var bi BITMAPINFO
	bi.Header.Size = uint32(unsafe.Sizeof(BITMAPINFOHEADER{}))
	bi.Header.Width = w
	bi.Header.Height = -h // top-down
	bi.Header.Planes = 1
	bi.Header.BitCount = 32
	bi.Header.Compression = BI_RGB
	var bits unsafe.Pointer
	bmp, _, _ := pCreateDIBSection.Call(dc, uintptr(unsafe.Pointer(&bi)), DIB_RGB_COLORS,
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 {
		pDeleteDC.Call(dc)
		return nil
	}
	old, _, _ := pSelectObject.Call(dc, bmp)
	return &MemDC{DC: HDC(dc), Bitmap: HBITMAP(bmp), Old: HGDIOBJ(old), Pixels: uintptr(bits), Width: w, Height: h}
}

func (m *MemDC) Close() {
	if m == nil || m.DC == 0 {
		return
	}
	pSelectObject.Call(uintptr(m.DC), uintptr(m.Old))
	pDeleteObject.Call(uintptr(m.Bitmap))
	pDeleteDC.Call(uintptr(m.DC))
	m.DC = 0
}

// Blit copies a memory surface onto the window.
func Blit(dst HDC, x, y int32, src *MemDC) {
	if src == nil {
		return
	}
	pBitBlt.Call(uintptr(dst), uintptr(x), uintptr(y), uintptr(src.Width), uintptr(src.Height),
		uintptr(src.DC), 0, 0, SRCCOPY)
}

// AlphaBlit composites a memory surface with per-pixel alpha.
func AlphaBlit(dst HDC, x, y int32, src *MemDC, alpha byte) {
	if src == nil {
		return
	}
	bf := BLENDFUNCTION{BlendOp: AC_SRC_OVER, AlphaFormat: AC_SRC_ALPHA, SourceConstantAlpha: alpha}
	pAlphaBlend.Call(uintptr(dst), uintptr(x), uintptr(y), uintptr(src.Width), uintptr(src.Height),
		uintptr(src.DC), 0, 0, uintptr(src.Width), uintptr(src.Height), uintptr(bf.Pack()))
}

// StretchTo scales a surface into a memory surface of the target size.
func StretchTo(src *MemDC, w, h int32) *MemDC {
	dst := NewMemDC(w, h)
	if dst == nil || src == nil {
		return dst
	}
	pSetStretchBltMode.Call(uintptr(dst.DC), COLORONCOLOR)
	pStretchBlt.Call(uintptr(dst.DC), 0, 0, uintptr(w), uintptr(h),
		uintptr(src.DC), 0, 0, uintptr(src.Width), uintptr(src.Height), SRCCOPY)
	return dst
}

// LoadImageIcon loads the first icon resource of a file (an exe or a dll)
// directly: it skips the shell association lookup and returns the icon the
// program ships, which is what users expect for a browser.
func LoadImageIcon(path string, size int32) *MemDC {
	if path == "" || size <= 0 {
		return nil
	}
	for _, want := range []uintptr{uintptr(size), uintptr(2 * size), uintptr(GetSystemMetrics(SM_CXICON))} {
		h, _, _ := pLoadImageW.Call(0, uintptr(unsafe.Pointer(UTF16Ptr(path))),
			IMAGE_ICON, want, want, LR_LOADFROMFILE)
		if h == 0 {
			continue
		}
		dc := NewMemDC(size, size)
		if dc == nil {
			pDestroyIcon.Call(h)
			return nil
		}
		ok := DrawIconEx(dc.DC, 0, 0, HICON(h), size, size)
		pDestroyIcon.Call(h)
		if !ok {
			dc.Close()
			return nil
		}
		return dc
	}
	return nil
}

// Cursor returns a stock cursor handle.
func Cursor(id uintptr) uintptr {
	r, _, _ := pLoadCursorW.Call(0, id)
	return r
}
