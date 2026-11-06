//go:build windows

package main

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dvygolov/yalauncher/internal/index"
	"github.com/dvygolov/yalauncher/internal/win"
)

// The launcher window is drawn entirely by hand: one borderless top level
// window that paints the search field and the result list with GDI, and takes
// its input from the global hook. A custom window is what makes the frame
// appear in a few milliseconds and look identical on every monitor - no
// toolkit, no layout pass, no font fallback surprises.

// msgRows is posted by the app when the index has been rebuilt.
const (
	msgRows      = win.WM_APP + 50
	msgIconReady = win.WM_APP + 51
)

const (
	uiPadding     = 16 // outer margin inside the window, 96 dpi units
	uiRowHeight   = 60
	uiFieldHeight = 40
	uiIconSize    = 28
	uiMaxRows     = 9
	uiCorner      = 14
)

// Palette: a dark surface with a single accent, the same scheme Flow Launcher
// ships by default but without the chrome.
var (
	colBg     = rgb(0x1E, 0x1F, 0x24)
	colField  = rgb(0x27, 0x29, 0x2F)
	colBorder = rgb(0x36, 0x39, 0x41)
	colText   = rgb(0xF2, 0xF3, 0xF5)
	colDim    = rgb(0x92, 0x97, 0xA1)
	colAccent = rgb(0x4C, 0x8D, 0xFF)
	colSel    = rgb(0x33, 0x38, 0x44)
	colSelBar = rgb(0x4C, 0x8D, 0xFF)
	colError  = rgb(0xFF, 0x8A, 0x8A)
	colCalc   = rgb(0x7E, 0xE0, 0x9B)
)

func rgb(r, g, b uint8) uint32 {
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16
}

// Row is one line in the result list.
type Row struct {
	Title  string
	Sub    string
	Icon   *win.MemDC
	Item   *index.Item
	Action func() // non-program rows (calculation result)
}

// UI owns the window, the query and the result list.
type UI struct {
	app  *App
	hwnd win.HWND

	query   []rune
	queryMu sync.Mutex

	rows   []*Row
	sel    int
	scroll int

	fontTitle win.HFONT
	fontSub   win.HFONT
	fontQuery win.HFONT

	dpi    uint32
	width  int32
	height int32

	icons       map[string]*win.MemDC
	iconMu      sync.Mutex
	iconPending map[string]bool

	calcResult string
	calcOK     bool
	errText    string

	monitor     win.Monitor
	hoverRow    int
	mouseInside bool
	caretOn     bool
}

func newUI(app *App) *UI {
	return &UI{
		app:         app,
		icons:       make(map[string]*win.MemDC),
		iconPending: make(map[string]bool),
		hoverRow:    -1,
		monitor:     win.Monitor{Full: win.Rect(0, 0, 1920, 1080), Work: win.Rect(0, 0, 1920, 1040)},
	}
}

// create makes the window but does not show it.
func (u *UI) create(icon win.HICON) error {
	if err := win.RegisterClass(windowClass, icon); err != nil {
		return err
	}
	h := win.CreateWindow(windowClass, appName,
		win.WS_EX_TOOLWINDOW|win.WS_EX_TOPMOST,
		win.WS_POPUP,
		0, 0, 660, 400, u.wndProc)
	if h == 0 {
		return fmt.Errorf("не удалось создать окно")
	}
	u.hwnd = h
	win.DropFiles(h)
	win.EnableDarkTitlebar(h, true)
	win.SetRoundCorners(h)

	return nil
}

// show places the window in the middle of the display that holds the cursor,
// resets the query and gives the window the focus.
// toggle opens the launcher or closes it when it is already open. The
// decision is taken on the keysToUI flag that the app keeps in step with the
// window, not on IsWindowVisible, because a top level window that has been
// shown once keeps WS_VISIBLE through most of its life.
func (u *UI) toggle() {
	if u.app.keysToUI.Load() {
		u.hide()
		return
	}
	u.app.showLauncher()
}

func (u *UI) show() {
	if u.hwnd == 0 {
		return
	}

	pt := win.CursorPos()
	winDpi := win.DPIForWindow(u.hwnd)
	if winDpi == 0 {
		winDpi = win.SystemDPI()
	}
	if m, ok := win.MonitorInfo(win.MonitorFromPoint(pt)); ok {
		u.monitor = m
		// The window's DPI still describes the monitor it was left on, but the
		// launcher is about to move to the one under the cursor: the scaling of
		// that monitor is what the sizes and the position have to use.
		u.dpi = win.DPIForMonitor(m.Handle, winDpi)
	} else {
		u.dpi = winDpi
	}
	u.rebuildFonts()
	work := u.monitor.Work
	u.width = u.scale(520)
	if work.Width() < u.width+u.scale(40) {
		u.width = work.Width() - u.scale(40)
	}
	if u.width < u.scale(360) {
		u.width = u.scale(360)
	}
	u.setQuery("")
	u.refreshRows(true)
	u.resizeToContent()
	u.place()
	win.EnsureForeground(u.hwnd)
	win.InvalidateRect(u.hwnd, nil, false)
	win.UpdateWindow(u.hwnd)
	u.caretOn = true
	win.SetTimer(u.hwnd, timerBlink, 530)
}

func (u *UI) hide() {
	if u.hwnd != 0 {
		win.KillTimer(u.hwnd, timerBlink)
		win.ShowWindow(u.hwnd, win.SW_HIDE)
		u.app.keysToUI.Store(false)
	}
}

// scale converts a 96 dpi design unit to the current monitor.
func (u *UI) scale(v int32) int32 {
	if u.dpi == 0 {
		return v
	}
	return int32(math.Round(float64(v) * float64(u.dpi) / 96.0))
}

// contentHeight is the height the window wants for the current row count,
// capped by the work area so the list never spills over the taskbar.
func (u *UI) contentHeight() int32 {
	// With no matches the field stands alone; a row is never invented just to
	// fill the window.
	h := u.scale(uiFieldHeight + uiPadding)
	n := len(u.rows)
	if n > uiMaxRows {
		n = uiMaxRows
	}
	h += int32(n) * u.scale(uiRowHeight)
	if foot := u.footerHeight(); foot > 0 {
		h += foot
	}
	if maxH := u.monitor.Work.Height() - u.scale(40); h > maxH {
		h = maxH
	}
	return h
}

func (u *UI) footerHeight() int32 {
	if u.calcOK || u.errText != "" {
		return u.scale(32)
	}
	return 0
}

func (u *UI) rebuildFonts() {
	for _, f := range []win.HFONT{u.fontTitle, u.fontSub, u.fontQuery} {
		if f != 0 {
			win.DeleteObject(win.HGDIOBJ(f))
		}
	}
	const family = "Segoe UI"
	u.fontTitle = win.Font{Name: family, Height: -u.scale(15), Weight: win.FW_NORMAL}.HFONT()
	u.fontSub = win.Font{Name: family, Height: -u.scale(12), Weight: win.FW_NORMAL}.HFONT()
	u.fontQuery = win.Font{Name: family, Height: -u.scale(22), Weight: win.FW_NORMAL}.HFONT()
}

func (u *UI) resizeToContent() {
	h := u.contentHeight()
	if h == u.height && u.width == u.scale(520) {
		return
	}
	u.height = h
	u.place()
}

func (u *UI) place() {
	// A per-monitor aware process gets physical pixels from GetMonitorInfo and
	// passes physical pixels to SetWindowPos, so the work area is used as is;
	// the monitor's DPI only scales the design sizes (u.width, u.height).
	work := u.monitor.Work
	x := work.Left + (work.Width()-u.width)/2
	y := work.Top + (work.Height()-u.height)/2
	if y < work.Top+u.scale(10) {
		y = work.Top + u.scale(10)
	}
	if y+u.height > work.Bottom-u.scale(10) {
		y = work.Bottom - u.height - u.scale(10)
	}
	// The window is moved and shown again in one call. SWP_NOMOVE keeps the
	// position this function just computed; the size is the full one, so the
	// first show and every later resize take the same path.
	win.SetWindowPos(u.hwnd, win.HWND(HWND_TOPMOST), x, y, u.width, u.height,
		win.SWP_SHOWWINDOW|win.SWP_NOACTIVATE)
}

// ---- query handling --------------------------------------------------------

func (u *UI) setQuery(q string) {
	u.queryMu.Lock()
	u.query = []rune(q)
	u.queryMu.Unlock()
	u.refreshRows(true)
}

func (u *UI) text() string {
	u.queryMu.Lock()
	defer u.queryMu.Unlock()
	return string(u.query)
}

// refreshRows recomputes the list for the current query. It is a linear pass
// over a few thousand programs plus an icon cache lookup, so it stays under a
// millisecond for a short query.
func (u *UI) refreshRows(resetSelection bool) {
	q := strings.TrimSpace(u.text())
	u.calcOK = false
	u.calcResult = ""
	u.errText = ""

	if q == "" {
		// An empty query is just the input line: no list, no history preview.
		// The history still ranks the results once the user types.
		u.rows = nil
		if resetSelection {
			u.sel = 0
			u.scroll = 0
		}
		return
	}

	// Numbers and arithmetic get a calculator row above the program matches,
	// the trick Flow Launcher's calculator plugin does.
	if val, ok := evaluate(q); ok {
		u.calcOK = true
		u.calcResult = val
		u.rows = []*Row{{
			Title: val,
			Sub:   "Скопировать результат · Enter",
			Action: func() {
				u.app.copyText(val)
				u.flash("Результат скопирован")
			},
		}}
		u.sel = 0
		u.scroll = 0
		return
	}

	ix := u.app.snapshot()
	hits := ix.Search(index.Query{
		Text:      q,
		Limit:     uiMaxRows * 4,
		Precision: int(u.app.cfg.precision()),
		Frequent:  u.app.frequent,
	})
	rows := make([]*Row, 0, len(hits)+1)
	for _, it := range hits {
		rows = append(rows, u.rowForItem(it))
	}
	if len(rows) == 0 {
		u.errText = "Ничего не найдено"
	}
	u.rows = rows
	if resetSelection || u.sel >= len(rows) {
		u.sel = 0
		u.scroll = 0
	}
}

func (u *UI) rowForItem(it *index.Item) *Row {
	sub := shortenPath(it.Path)
	if it.Description != "" {
		sub = it.Description + "  ·  " + sub
	}
	return &Row{Title: it.Name, Sub: sub, Item: it, Icon: u.cachedIcon(it)}
}

// glyphIcon renders a short text glyph into an icon slot.
func (u *UI) glyphIcon(text string, color uint32) *win.MemDC {
	return drawGlyph(u.scale(uiIconSize), text, color, u.dpi)
}

// ---- window procedure ------------------------------------------------------

func (u *UI) wndProc(h win.HWND, msg uint32, wp, lp uintptr) uintptr {
	switch msg {
	case win.WM_PAINT:
		u.paint(h)
		return 0
	case win.WM_ERASEBKGND:
		return 1
	case win.WM_KEYDOWN, win.WM_SYSKEYDOWN, win.WM_CHAR:
		// Keystrokes arrive through the global hook (see App.onHotkey), which
		// is the only path that also works when the window cannot take the
		// real focus. Handling them here as well would double the input.
		return 0
	case win.WM_MOUSEWHEEL:
		if int16(uint16(wp>>16)) > 0 {
			u.moveSelection(-1)
		} else {
			u.moveSelection(1)
		}
		return 0
	case win.WM_MOUSEMOVE:
		u.onMouseMove(lp)
		return 0
	case win.WM_LBUTTONDOWN:
		u.onClick(lp)
		return 0
	case win.WM_MOUSELEAVE:
		u.hoverRow = -1
		u.mouseInside = false
		win.InvalidateRect(h, nil, false)
		return 0
	case win.WM_DROPFILES:
		u.onDrop(wp)
		return 0
	case msgIconReady:
		u.applyIcons()
		win.InvalidateRect(h, nil, false)
		return 0
	case msgRows:
		u.refreshRows(false)
		u.resizeToContent()
		win.InvalidateRect(h, nil, false)
		return 0
	case win.WM_TIMER:
		if wp == timerBlink {
			u.caretOn = !u.caretOn
			win.InvalidateRect(h, nil, false)
			return 0
		}
		if wp == timerFlash {
			win.KillTimer(h, timerFlash)
			u.errText = ""
			win.InvalidateRect(h, nil, false)
		}
		return 0
	case win.WM_KILLFOCUS:
		// The launcher is driven by the global hook, not by the window focus,
		// so losing the focus is normal (it happens the moment a remote session
		// or a full screen app keeps the foreground). Closing is bound to the
		// hotkey and Esc instead.
		return 0
	case win.WM_DPICHANGED:
		// Windows moved the window to a monitor with another scale. The sizes
		// are rebuilt for the new DPI and the window is re-centred there.
		u.dpi = uint32(uint16(wp))
		u.rebuildFonts()
		if m, ok := win.MonitorInfo(win.MonitorFromWindow(h)); ok {
			u.monitor = m
		}
		work := u.monitor.Work
		u.width = u.scale(520)
		if work.Width() < u.width+u.scale(40) {
			u.width = work.Width() - u.scale(40)
		}
		if u.width < u.scale(380) {
			u.width = u.scale(380)
		}
		u.resizeToContent()
		return 0
	}
	return win.DefWindowProc(h, msg, wp, lp)
}

func (u *UI) onDrop(wp uintptr) {
	files := win.DroppedFiles(wp)
	if len(files) == 0 {
		return
	}
	u.setQuery(files[0])
	u.resizeToContent()
	win.SetForegroundWindow(u.hwnd)
}

func (u *UI) onChar(r rune) {
	if r < 0x20 {
		return
	}
	u.queryMu.Lock()
	if len(u.query) > 512 {
		u.queryMu.Unlock()
		return
	}
	u.query = append(u.query, r)
	u.queryMu.Unlock()
	u.refreshRows(true)
	u.resizeToContent()
	win.InvalidateRect(u.hwnd, nil, false)
}

func (u *UI) backspace() {
	u.queryMu.Lock()
	if len(u.query) > 0 {
		u.query = u.query[:len(u.query)-1]
	}
	u.queryMu.Unlock()
	u.refreshRows(true)
	u.resizeToContent()
	win.InvalidateRect(u.hwnd, nil, false)
}

func (u *UI) moveSelection(delta int) {
	if len(u.rows) == 0 {
		return
	}
	u.sel += delta
	if u.sel < 0 {
		u.sel = len(u.rows) - 1
	}
	if u.sel >= len(u.rows) {
		u.sel = 0
	}
	u.scrollToSel()
	win.InvalidateRect(u.hwnd, nil, false)
}

func (u *UI) scrollToSel() {
	if u.sel < u.scroll {
		u.scroll = u.sel
	}
	if u.sel >= u.scroll+uiMaxRows {
		u.scroll = u.sel - uiMaxRows + 1
	}
}

func (u *UI) activate(ctrl, shift bool) {
	if u.sel < 0 || u.sel >= len(u.rows) {
		return
	}
	r := u.rows[u.sel]
	if r.Action != nil {
		r.Action()
		return
	}
	if r.Item == nil {
		return
	}
	u.app.launch(r.Item, ctrl, shift)
	if !ctrl && !shift {
		u.hide()
	}
}

func (u *UI) onMouseMove(lp uintptr) {
	if !u.mouseInside {
		u.mouseInside = true
		win.TrackMouseLeave(u.hwnd)
	}
	y := int32(int16(uint32(lp >> 16)))
	row := u.rowAt(y)
	if row != u.hoverRow {
		u.hoverRow = row
		// The selection only changes on click or arrow keys, not on hover:
		// otherwise scrolling with the wheel would drag the selection back to
		// wherever the cursor happens to be.
		win.InvalidateRect(u.hwnd, nil, false)
	}
}

func (u *UI) onClick(lp uintptr) {
	y := int32(int16(uint32(lp >> 16)))
	row := u.rowAt(y)
	if row < 0 || row >= len(u.rows) {
		return
	}
	u.sel = row
	u.activate(false, false)
}

func (u *UI) rowAt(y int32) int {
	top := u.scale(uiFieldHeight + uiPadding)
	if y < top {
		return -1
	}
	idx := int((y - top) / u.scale(uiRowHeight))
	if idx < 0 {
		return -1
	}
	idx += u.scroll
	if idx >= len(u.rows) {
		return -1
	}
	return idx
}

// ---- painting --------------------------------------------------------------

func (u *UI) paint(h win.HWND) {
	dc, ps := win.BeginPaint(h)
	defer win.EndPaint(h, &ps)
	rect := win.GetClientRect(h)

	win.SolidRect(dc, rect, colBg)
	pen := win.Pen(colBorder, 1)
	win.Line(dc, rect.Left, rect.Top, rect.Right-1, rect.Top, pen)
	win.Line(dc, rect.Right-1, rect.Top, rect.Right-1, rect.Bottom-1, pen)
	win.Line(dc, rect.Right-1, rect.Bottom-1, rect.Left, rect.Bottom-1, pen)
	win.Line(dc, rect.Left, rect.Bottom-1, rect.Left, rect.Top, pen)
	win.DeleteObject(win.HGDIOBJ(pen))

	pad := u.scale(uiPadding)
	fieldH := u.scale(uiFieldHeight)
	field := win.Rect(pad, pad, int32(rect.Right)-pad, pad+fieldH-u.scale(8))
	win.SolidRect(dc, field, colField)
	u.paintQuery(dc, field)
	top := field.Bottom + u.scale(4)
	u.paintRows(dc, top, int32(rect.Right), int32(rect.Bottom))
	u.paintTooltip(dc, int32(rect.Right), int32(rect.Bottom))
	u.paintScrollIndicator(dc, int32(rect.Right), int32(rect.Bottom))
	u.paintFooter(dc, int32(rect.Right), int32(rect.Bottom))
}

func (u *UI) paintQuery(dc win.HDC, r win.RECT) {
	text := u.text()
	x := r.Left + u.scale(16)
	width := r.Right - r.Left - u.scale(28)
	if text == "" {
		if u.caretOn {
			win.SolidRect(dc, win.Rect(x+u.scale(2), r.Top+u.scale(8), x+u.scale(4), r.Bottom-u.scale(8)), colDim)
		}
		return
	}
	w := win.TextCentered(dc, text, x, r.Top, u.fontQuery, colText, width, r.Bottom-r.Top) - x
	if w > width {
		w = width
	}
	if u.caretOn {
		win.SolidRect(dc, win.Rect(x+w+u.scale(2), r.Top+u.scale(8), x+w+u.scale(4), r.Bottom-u.scale(8)), colDim)
	}
}

func (u *UI) paintRows(dc win.HDC, top, right, bottom int32) {
	rowH := u.scale(uiRowHeight)
	pad := u.scale(uiPadding)
	visible := uiMaxRows
	for i := 0; i < visible; i++ {
		idx := u.scroll + i
		if idx >= len(u.rows) {
			break
		}
		y := top + int32(i)*rowH
		row := u.rows[idx]
		if idx == u.sel {
			win.SolidRect(dc, win.Rect(pad, y, right-pad, y+rowH), colSel)
			win.SolidRect(dc, win.Rect(pad, y, pad+u.scale(3), y+rowH), colSelBar)
		}
		iconSize := u.scale(uiIconSize)
		iconX := pad + u.scale(14)
		iconY := y + (rowH-iconSize)/2
		if row.Icon != nil {
			win.AlphaBlit(dc, iconX, iconY, row.Icon, 255)
		} else if row.Item != nil {
			win.SolidRect(dc, win.Rect(iconX, iconY, iconX+iconSize, iconY+iconSize), colBorder)
		}
		textX := iconX + iconSize + u.scale(14)
		width := right - pad - u.scale(14) - textX
		titleH := win.TextHeight(dc, u.fontTitle)
		subH := win.TextHeight(dc, u.fontSub)
		gap := u.scale(6)
		// The two lines are measured with the real GDI metrics and placed
		// together: title on top, the path under it with a visible gap, so the
		// descenders of the title never touch the ascenders of the path.
		block := titleH + gap + subH
		if block > rowH {
			// Never let the text escape the row: shrink the block rather than
			// overlap into the next row.
			block = rowH
		}
		titleTop := y + (rowH-block)/2
		win.Text(dc, row.Title, textX, titleTop, u.fontTitle, colText, win.DT_LEFT, width)
		if row.Sub != "" {
			win.Text(dc, row.Sub, textX, titleTop+titleH+gap, u.fontSub, colDim, win.DT_LEFT, width)
		}
	}
}

// applyIcons swaps the icons of the visible rows for the freshly loaded ones.
// Called when msgIconReady arrives after a background shell extraction.
func (u *UI) applyIcons() {
	for _, r := range u.rows {
		if r.Item == nil {
			continue
		}
		key := strings.ToLower(r.Item.Path)
		u.iconMu.Lock()
		if m, ok := u.icons[key]; ok && m != nil {
			r.Icon = m
		}
		u.iconMu.Unlock()
	}
}

func (u *UI) paintFooter(dc win.HDC, right, bottom int32) {
	foot := u.footerHeight()
	if foot == 0 {
		return
	}
	pad := u.scale(uiPadding)
	y := bottom - foot
	if u.calcOK {
		h := win.TextHeight(dc, u.fontTitle)
		win.Text(dc, u.calcResult, pad+u.scale(17), y+(foot-h)/2, u.fontTitle, colCalc, win.DT_LEFT, right-pad)
	}
	if u.errText != "" {
		h := win.TextHeight(dc, u.fontSub)
		win.Text(dc, u.errText, pad+u.scale(17), y+(foot-h)/2, u.fontSub, colError, win.DT_LEFT, right-pad)
	}
}

// HWND_TOPMOST is the documented (HWND)-1 pseudo handle.
var HWND_TOPMOST = ^uintptr(0)

// postRowsChanged asks the window to re-read the list, e.g. after a rescan.
func (u *UI) postRowsChanged() {
	if u.hwnd != 0 {
		win.PostMessage(u.hwnd, msgRows, 0, 0)
	}
}

// flash shows a short status line in the footer instead of a modal box.
func (u *UI) flash(msg string) {
	u.errText = msg
	win.InvalidateRect(u.hwnd, nil, false)
	win.SetTimer(u.hwnd, timerFlash, 1600)
}

// ---- icon cache ------------------------------------------------------------

// cachedIcon returns the icon only when it is already in the cache. Missing
// icons are extracted in the background and the window is repainted once they
// arrive, so typing never blocks on the shell's icon loader.
func (u *UI) cachedIcon(it *index.Item) *win.MemDC {
	key := strings.ToLower(it.Path)
	u.iconMu.Lock()
	m, ok := u.icons[key]
	u.iconMu.Unlock()
	if ok {
		return m
	}
	u.queueIcon(it)
	return nil
}

// queueIcon schedules the shell icon extraction of a program on a background
// goroutine. The pending map dedupes work when the same path is shown again
// on the next keystroke.
func (u *UI) queueIcon(it *index.Item) {
	key := strings.ToLower(it.Path)
	u.iconMu.Lock()
	if u.iconPending[key] {
		u.iconMu.Unlock()
		return
	}
	u.iconPending[key] = true
	u.iconMu.Unlock()
	go u.loadIcon(it, key)
}

// loadIcon extracts the icon off the UI thread and posts msgIconReady so the
// window is repainted with it.
func (u *UI) loadIcon(it *index.Item, key string) {
	size := u.scale(uiIconSize)
	m := win.FileIcon(it.Path, size, false)
	if m == nil && it.Kind == index.KindShortcut {
		target := it.ExecutableName
		if target != "" && !strings.HasPrefix(strings.ToLower(target), "http") {
			m = win.FileIcon(target, size, false)
		}
	}
	if m == nil {
		m = u.glyphIcon(strings.ToUpper(firstLetter(it.Name)), colAccent)
	}
	u.iconMu.Lock()
	delete(u.iconPending, key)
	u.icons[key] = m
	u.iconMu.Unlock()
	if u.app.keysToUI.Load() {
		win.PostMessage(u.hwnd, msgIconReady, 0, 0)
	}
}

func firstLetter(s string) string {
	for _, r := range s {
		return string(r)
	}
	return "?"
}

// shortenPath keeps the tail of a path: users recognise
// "Programs\Chrome\chrome.exe", not the whole C:\... prefix.
func shortenPath(p string) string {
	if p == "" {
		return ""
	}
	vol := filepath.VolumeName(p)
	dir := filepath.Dir(strings.TrimPrefix(p, vol))
	base := filepath.Base(p)
	parts := strings.Split(strings.Trim(dir, `\`), `\`)
	if len(parts) > 2 {
		parts = parts[len(parts)-2:]
	}
	out := strings.Join(parts, `\`)
	if out != "" {
		out += `\`
	}
	out += base
	if vol != "" {
		return vol + `\…\` + out
	}
	return out
}

// onKeyMessage handles one key delivered from the global hook. With a single
// hook every keystroke of the session can be routed here while the launcher is
// open, which is how typing works in remote sessions where the window cannot
// take the real focus.
func (u *UI) onKeyMessage(vk uint32, ctrl, shift, _ bool) {
	if vk == win.VK_ESCAPE {
		// Esc closes the launcher outright; clearing the query is what
		// Backspace is for.
		u.app.post(umHide, 0, 0)
		return
	}
	if vk == win.VK_BACK {
		u.backspace()
		return
	}
	if vk == win.VK_UP {
		u.moveSelection(-1)
		return
	}
	if vk == win.VK_DOWN {
		u.moveSelection(1)
		return
	}
	if vk == win.VK_RETURN {
		u.activate(ctrl, shift)
		return
	}
	if vk == win.VK_TAB {
		u.moveSelection(1)
		return
	}
	// Printable characters come from the keyboard layout of the user.
	if r, ok := keyToRune(vk, shift); ok {
		u.onChar(r)
	}
}

// keyToRune maps a virtual key to a lowercase ASCII rune for the common keys
// of the two layouts the launcher targets. Non-ASCII input (Cyrillic, IME)
// reaches the window through WM_CHAR when the window really is focused.
func keyToRune(vk uint32, shift bool) (rune, bool) {
	switch {
	case vk >= 'A' && vk <= 'Z':
		r := rune('a' + vk - 'A')
		if shift {
			r = rune('A' + vk - 'A')
		}
		return r, true
	case vk >= '0' && vk <= '9':
		digits := "0123456789"
		shifted := ")!@#$%^&*("
		if shift {
			return rune(shifted[vk-'0']), true
		}
		return rune(digits[vk-'0']), true
	case vk == win.VK_SPACE:
		return ' ', true
	case vk == win.VK_OEM_MINUS:
		if shift {
			return '_', true
		}
		return '-', true
	case vk == win.VK_OEM_PLUS:
		if shift {
			return '+', true
		}
		return '=', true
	case vk == win.VK_OEM_COMMA:
		return ',', true
	case vk == win.VK_OEM_PERIOD:
		return '.', true
	case vk == win.VK_OEM_2:
		if shift {
			return '?', true
		}
		return '/', true
	}
	return 0, false
}

// itoa formats a small integer without fmt in the paint path.
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [12]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// paintTooltip draws the full path of the program under the mouse cursor at
// the bottom of the window, above the footer. It appears only while a result
// is hovered, so the list stays uncluttered otherwise.
func (u *UI) paintTooltip(dc win.HDC, right, bottom int32) {
	if u.hoverRow < 0 || u.hoverRow >= len(u.rows) {
		return
	}
	r := u.rows[u.hoverRow]
	if r.Item == nil || r.Item.Path == "" {
		return
	}
	pad := u.scale(uiPadding)
	h := u.scale(24)
	y := bottom - h - u.footerHeight()
	win.SolidRect(dc, win.Rect(pad, y, right-pad, y+h), colField)
	text := r.Item.Path
	// Show the whole path; the subtitle font is small enough for the usual
	// Program Files / Start Menu locations.
	win.Text(dc, text, pad+u.scale(8), y+u.scale(3), u.fontSub, colDim, win.DT_LEFT,
		right-pad*2-u.scale(16))
}

// paintScrollIndicator draws "current/total" at the bottom-right corner of the
// result list, so the user knows where they are and how much is left to scroll.
func (u *UI) paintScrollIndicator(dc win.HDC, right, bottom int32) {
	if len(u.rows) == 0 {
		return
	}
	total := len(u.rows)
	current := u.sel + 1
	if current < 1 {
		current = 1
	}
	text := itoa(current) + "/" + itoa(total)
	pad := u.scale(uiPadding)
	h := u.scale(22)
	w := u.scale(44)
	x := right - pad - w
	y := bottom - h - u.footerHeight()
	win.Text(dc, text, x, y+u.scale(3), u.fontSub, colDim, win.DT_RIGHT, w)
}
