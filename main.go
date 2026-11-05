//go:build windows

// Command YALauncher is "yet another launcher": a Flow Launcher style quick
// launcher for Windows, written in Go with no dependencies beyond Win32.
//
// One hidden window owns a tray icon, a keyboard hook and a single borderless
// window that appears in the middle of the display under the mouse pointer
// when the hotkey is pressed. Programs come from the Start Menu, the App Paths
// registry and any folder the user adds; the fuzzy matcher is a port of the
// one Flow Launcher uses.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dvygolov/yalauncher/internal/index"
	"github.com/dvygolov/yalauncher/internal/win"
)

const (
	appName     = "YALauncher"
	windowClass = "YALauncherWnd"
	mutexName   = `Local\YALauncherSingleInstance`
	repoURL     = "https://github.com/dvygolov/YALauncher"

	// window messages sent to the hidden window
	umTray       = win.WM_APP + 1
	umToggle     = win.WM_APP + 3
	umReindex    = win.WM_APP + 4
	umHide       = win.WM_APP + 5
	umKey        = win.WM_APP + 6
	umUpdateOK   = win.WM_APP + 11
	umUpdateFail = win.WM_APP + 12

	// timers
	timerTray    = 1
	timerUpdate  = 2
	timerReindex = 3
	timerFlash   = 4
	timerCaret   = 5

	// tray menu commands
	cmdShow       = 101
	cmdReindex    = 102
	cmdAutostart  = 103
	cmdAddFolder  = 104
	cmdOpenFolder = 105
	cmdPrecision  = 106
	cmdCheck      = 107
	cmdLog        = 108
	cmdAbout      = 109
	cmdExit       = 110
	cmdConfig     = 111
)

// version is injected by the build: -ldflags "-X main.version=1.2.3".
var version = "0.0.0-dev"

// debugHotkey enables the extra diagnostics of --debug-hotkey.
var debugHotkey bool

// App is the whole program: one hidden window, one tray icon, one hook, one
// launcher window and one background index.
type App struct {
	hwnd     win.HWND
	tray     *win.Tray
	ui       *UI
	cfg      Config
	hk       Hotkey
	mods     modifierState
	hotkeyOn bool

	mu       sync.Mutex
	ix       atomic.Pointer[index.Index]
	scanning atomic.Bool
	preload  bool // the first scan has not finished yet
	watch    *index.Watch

	frequent *index.MostFrequent

	instanceMutex win.HANDLE
	firstRun      bool
	taskbarMsg    uint32
	promoteTries  int

	updateBusy    bool
	updateManual  bool
	updateVersion string
	updatePath    string
	updateMsg     string

	pendingSave atomic.Bool
	keysToUI    atomic.Bool
	startedAt   time.Time
}

func main() {
	// The hook and the message loop must live on one OS thread.
	runtime.LockOSThread()

	for _, arg := range os.Args[1:] {
		switch strings.ToLower(arg) {
		case "--version", "-v", "/version":
			win.MessageBox(appName+" "+version, appName, win.MB_OK|win.MB_ICONINFORMATION)
			return
		case "--help", "-h", "/?":
			win.MessageBox(helpText(), appName, win.MB_OK|win.MB_ICONINFORMATION)
			return
		}
	}

	win.PanicLog = logf
	if hasArg(os.Args[1:], "--debug-hotkey") {
		debugHotkey = true
		logf("debug: keyboard trace enabled")
	}
	win.SetPerMonitorAware()

	cfg, firstRun := loadConfig()
	logf("start: %s %s (pid=%d, exe=%s)", appName, version, os.Getpid(), win.ExePath())
	logf("config: hotkey=%s folders=%v startMenu=%v appPaths=%v precision=%s",
		cfg.Hotkey, cfg.Folders, cfg.StartMenu, cfg.AppPaths, cfg.Precision)

	if win.FindWindow(windowClass) != 0 {
		// Already running: wake the existing instance and leave.
		hwnd := win.FindWindow(windowClass)
		win.PostMessage(hwnd, umToggle, 0, 0)
		logf("start: another instance is running, asked it to show")
		return
	}

	mutex, ok := win.CreateInstanceMutex(mutexName)
	if ok {
		logf("start: another instance holds the single instance mutex")
		return
	}

	hk, err := ParseHotkey(cfg.Hotkey)
	if err != nil {
		logf("hotkey: %v, falling back to ctrl+alt+space", err)
		cfg.Hotkey = "ctrl+alt+space"
		hk, _ = ParseHotkey(cfg.Hotkey)
	}

	app := &App{cfg: cfg, hk: hk, firstRun: firstRun, instanceMutex: mutex, startedAt: time.Now()}
	app.frequent = &app.cfg.Frequent
	app.preload = true
	app.run()
	closeLog()
}

func hasArg(args []string, name string) bool {
	for _, a := range args {
		if strings.EqualFold(a, name) {
			return true
		}
	}
	return false
}

func (a *App) run() {
	icon := win.LoadIconFromICO(iconData, win.SmallIconSize())
	if err := win.RegisterClass(windowClass, icon); err != nil {
		logf("window: RegisterClass failed: %v", err)
	}
	a.hwnd = win.CreateWindow(windowClass, appName, 0, 0, 0, 0, 0, 0, a.wndProc)
	if a.hwnd == 0 {
		logf("window: CreateWindowEx failed")
		win.MessageBox("Не удалось создать окно приложения.\nПодробности: "+logPath(),
			appName, win.MB_OK|win.MB_ICONWARNING)
		return
	}
	a.taskbarMsg = win.RegisterWindowMessage("TaskbarCreated")

	a.ui = newUI(a)
	if err := a.ui.create(icon); err != nil {
		logf("launcher window: %v", err)
	} else {
		logf("launcher window: created")
	}

	a.tray = win.NewTray(a.hwnd, icon, a.trayTip(), umTray)
	a.installHotkey()
	a.syncAutostart()
	cleanupOldBinary()

	// Index: a cache from the last run is shown immediately, the scan then
	// replaces it. This is how the launcher gets a list on screen in its
	// first second and stays responsive while thousands of folders are read.
	if cached := index.LoadCache(cachePath(), time.Duration(24*60)*time.Minute); cached != nil {
		a.ix.Store(cached)
		a.preload = false
		logf("index: cache loaded (%d programs, built %s)", cached.Len(),
			time.Unix(cached.BuiltAt, 0).Format("2006-01-02 15:04"))
	} else {
		logf("index: no usable cache, scanning")
	}
	go a.reindex(true)

	win.SetTimer(a.hwnd, timerTray, 1500)
	if a.cfg.AutoUpdate {
		win.SetTimer(a.hwnd, timerUpdate, 10_000)
	}
	win.SetTimer(a.hwnd, timerReindex, uint32(time.Duration(a.cfg.ReindexMinutes)*time.Minute/time.Millisecond))

	if a.firstRun {
		a.balloon("YALauncher запущен",
			fmt.Sprintf("Нажмите %s, чтобы открыть поиск. Настройки и лог: %s", a.hk.Name, dataDir()),
			win.NIIF_INFO)
	}

	win.RunMessageLoop()
}

// ---- messages --------------------------------------------------------------

func (a *App) wndProc(h win.HWND, msg uint32, wp, lp uintptr) uintptr {
	switch msg {
	case a.taskbarMsg:
		if a.tray != nil {
			a.tray.Add()
			a.tray.SetTip(a.trayTip())
		}
		return 0

	case umTray:
		// NOTIFYICON_VERSION_4 packs the event into the low word of lparam
		ev := uint32(lp) & 0xFFFF
		switch ev {
		case win.WM_LBUTTONUP, win.WM_LBUTTONDBLCLK, win.WM_RBUTTONUP, win.WM_CONTEXTMENU:
			a.showMenu()
		}
		return 0

	case umToggle:
		if a.ui != nil {
			a.ui.toggle()
		}
		return 0
	case umReindex:
		go a.reindex(false)
		return 0
	case umHide:
		if a.ui != nil {
			a.ui.hide()
		}
		return 0
	case umKey:
		if a.ui != nil {
			a.ui.onKeyMessage(uint32(wp), lp&1 != 0, lp&4 != 0, lp&2 != 0)
		}
		return 0

	case win.WM_TIMER:
		switch wp {
		case timerTray:
			a.promoteTrayIcon()
		case timerUpdate:
			win.KillTimer(h, timerUpdate)
			go a.checkUpdates(false)
		case timerReindex:
			// Fallback rescan for folders the file system watcher cannot see
			// (network drives, Yandex Disk): Flow Launcher reindexes on a
			// timer for the same reason.
			go a.reindex(false)
			win.SetTimer(h, timerReindex, uint32(time.Duration(a.cfg.ReindexMinutes)*time.Minute/time.Millisecond))
		}
		return 0

	case umUpdateFail:
		a.mu.Lock()
		msg, manual := a.updateMsg, a.updateManual
		a.mu.Unlock()
		if manual && msg != "" {
			a.balloon("Обновления", msg, win.NIIF_INFO)
		}
		a.rearmUpdateTimer()
		return 0

	case umUpdateOK:
		a.mu.Lock()
		ver := a.updateVersion
		a.mu.Unlock()
		logf("update: %s downloaded, installing", ver)
		if err := a.applyUpdate(); err != nil {
			logf("update: apply failed: %v", err)
			a.balloon("Обновление "+ver, err.Error(), win.NIIF_WARNING)
			a.rearmUpdateTimer()
		}
		return 0

	case win.WM_CLOSE:
		win.DestroyWindow(h)
		return 0

	case win.WM_DESTROY:
		win.KillTimer(h, timerTray)
		win.KillTimer(h, timerUpdate)
		win.KillTimer(h, timerReindex)
		win.RemoveKeyboardHook()
		if a.watch != nil {
			a.watch.Close()
		}
		if a.tray != nil {
			a.tray.Remove()
		}
		if a.pendingSave.Load() {
			a.saveConfig()
		}
		if a.instanceMutex != 0 {
			win.CloseHandle(a.instanceMutex)
			a.instanceMutex = 0
		}
		win.PostQuitMessage(0)
		return 0
	}
	return win.DefWindowProc(h, msg, wp, lp)
}

func (a *App) post(msg uint32, wp, lp uintptr) {
	if a.hwnd != 0 {
		win.PostMessage(a.hwnd, msg, wp, lp)
	}
}

// ---- launcher window -------------------------------------------------------

func (a *App) showLauncher() {
	if a.ui == nil || !win.IsWindow(a.ui.hwnd) {
		return
	}
	a.preload = false
	a.ui.show()
	a.keysToUI.Store(true)
}

// ---- hotkey ----------------------------------------------------------------

// installHotkey (re)registers the keyboard filter for the configured hotkey.
func (a *App) installHotkey() {
	win.Hotkey = a.onHotkey
	if err := win.InstallKeyboardHook(nil); err != nil {
		logf("hook: WH_KEYBOARD_LL failed: %v", err)
		a.balloon("Горячая клавиша недоступна",
			"Система отклонила установку хука клавиатуры. Попробуйте запустить программу от администратора.",
			win.NIIF_WARNING)
		return
	}
	a.hotkeyOn = true
	logf("hook: installed, hotkey=%s", a.hk.Name)
}

// onHotkey runs inside the keyboard hook: it has to be allocation free on the
// hot path and return in microseconds.
func (a *App) onHotkey(ev win.KeyEvent) bool {
	if ev.Ours {
		return false
	}
	a.mods = modifierStateFrom(ev.VK, ev.Down, a.mods)
	if !ev.Down {
		return false
	}
	if debugHotkey {
		logf("debug: hook vk=%#x ctrl=%v alt=%v shift=%v win=%v",
			ev.VK, a.mods.ctrl, a.mods.alt, a.mods.shift, a.mods.win)
	}
	if a.hk.matches(ev, a.mods) {
		a.post(umToggle, 0, 0)
		return true
	}
	// While the launcher is open the hook feeds it directly. A global hook sees
	// every keystroke of the session, so typing works even when a remote or
	// elevated foreground window keeps the real focus and the launcher window
	// itself never becomes the active one - normal on RDP sessions and in front
	// of full screen applications.
	if !a.keysToUI.Load() {
		return false
	}
	switch ev.VK {
	case win.VK_CONTROL, win.VK_MENU, win.VK_SHIFT, win.VK_LWIN, win.VK_RWIN,
		win.VK_LSHIFT, win.VK_RSHIFT, win.VK_LCONTROL, win.VK_RCONTROL,
		win.VK_LMENU, win.VK_RMENU, win.VK_CAPITAL:
		return false // modifiers are read from the tracked state
	}
	// The trigger key without the launcher owned keystrokes: swallow it too,
	// otherwise the chord would leak into the window below.
	a.post(umKey, uintptr(ev.VK), keyMods(a.mods))
	return true
}

// keyMods packs the modifier state for PostMessage: bit 0 ctrl, bit 1 alt,
// bit 2 shift.
func keyMods(m modifierState) uintptr {
	var v uintptr
	if m.ctrl {
		v |= 1
	}
	if m.alt {
		v |= 2
	}
	if m.shift {
		v |= 4
	}
	return v
}

// boolWord packs two booleans into one word for PostMessage.
func boolWord(any, shift bool) uintptr {
	var v uintptr
	if any {
		v |= 1
	}
	if shift {
		v |= 2
	}
	return v
}

// ---- index -----------------------------------------------------------------

func (a *App) snapshot() *index.Index {
	ix := a.ix.Load()
	if ix == nil {
		return index.New()
	}
	return ix
}

// reindex rescans the configured roots. Scans are serialised: a scan triggered
// by a folder watcher while another one runs simply misses, which is fine
// because the next notification will come.
func (a *App) reindex(first bool) {
	if !a.scanning.CompareAndSwap(false, true) {
		return
	}
	defer a.scanning.Store(false)
	start := time.Now()
	skip := map[string]bool{}
	for _, k := range a.cfg.Rejected {
		skip[k] = true
	}
	ix := index.Scan(a.cfg.options(), skip, nil, nil)
	if ix.Len() == 0 && !first {
		logf("index: scan found nothing, keeping the previous list")
		return
	}
	a.ix.Store(ix)
	a.preload = false
	logf("index: %d programs in %d ms", ix.Len(), time.Since(start).Milliseconds())
	if err := ix.Save(cachePath()); err != nil {
		logf("index: cache save failed: %v", err)
	}
	// repaint the list if the window is open while the scan was running
	if a.ui != nil {
		a.ui.postRowsChanged()
	}
}

func cachePath() string { return filepath.Join(dataDir(), "index.json") }

// ---- launching -------------------------------------------------------------

// launch starts a program. Ctrl opens the containing folder instead, Ctrl+
// Shift runs it elevated - the two shortcuts Flow Launcher's program plugin
// provides.
func (a *App) launch(it *index.Item, ctrl, shift bool) {
	if it == nil {
		return
	}
	switch {
	case ctrl && shift:
		if !win.OpenAsAdmin(it.Path, it.Args, filepath.Dir(it.Path)) {
			win.ShowInFolder(it.Path)
		}
		logf("launch: %s (admin)", it.Path)
	case ctrl:
		win.ShowInFolder(it.Path)
		logf("launch: %s (show in folder)", it.Path)
	default:
		ok := win.Open(it.Path, it.Args, filepath.Dir(it.Path))
		if !ok {
			// A stale shortcut target or a removed drive: report instead of
			// doing nothing, which is what a silent launcher feels like.
			if a.ui != nil {
				a.ui.flash("Не удалось запустить: " + it.Path)
			}
			logf("launch: failed for %s", it.Path)
			return
		}
		logf("launch: %s", it.Path)
	}
	a.recordLaunch(it)
}

func (a *App) recordLaunch(it *index.Item) {
	key := it.Key()
	a.frequent.Hit(key, time.Now().Unix())
	a.cfg.Frequent = *a.frequent
	a.cfg.History = append([]string{key}, a.cfg.History...)
	if len(a.cfg.History) > 40 {
		a.cfg.History = a.cfg.History[:40]
	}
	a.pendingSave.Store(true)
	go a.saveConfig()
}

func (a *App) recentKeys(limit int) []string {
	out := make([]string, 0, limit)
	seen := map[string]bool{}
	for _, k := range a.cfg.History {
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (a *App) copyText(text string) { copyText(text) }

// ---- config ----------------------------------------------------------------

func (a *App) saveConfig() {
	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()
	if err := cfg.save(); err != nil {
		logf("config: save failed: %v", err)
	}
	a.pendingSave.Store(false)
}

func (a *App) reloadConfig() {
	cfg, _ := loadConfig()
	a.cfg = cfg
	a.frequent = &a.cfg.Frequent
	hk, err := ParseHotkey(cfg.Hotkey)
	if err == nil {
		a.hk = hk
	}
}

// ---- tray ------------------------------------------------------------------

func (a *App) trayTip() string {
	return fmt.Sprintf("%s %s\n%s — поиск программ", appName, version, a.hk.Name)
}

func (a *App) balloon(title, text string, flags uint32) {
	if a.tray != nil {
		a.tray.Balloon(title, text, flags)
	}
}

func (a *App) promoteTrayIcon() {
	if a.promoteTries >= 5 {
		return
	}
	a.promoteTries++
	if win.PromoteTrayIcon(win.ExePath()) {
		a.promoteTries = 5
		if a.tray != nil {
			a.tray.Add()
		}
		logf("tray: icon promoted to the taskbar")
	}
}

func (a *App) showMenu() {
	m := win.NewPopupMenu()
	defer m.Destroy()
	m.AddInfo(fmt.Sprintf("%s %s", appName, version))
	m.AddInfo(fmt.Sprintf("Программ в индексе: %d", a.snapshot().Len()))
	m.Add(cmdShow, "Открыть поиск ("+prettifyHotkey(a.hk.Name)+")", false)
	m.Separator()
	m.Add(cmdReindex, "Переиндексировать сейчас", false)
	m.Add(cmdAddFolder, "Добавить папку в индекс…", false)
	m.Add(cmdOpenFolder, "Открыть папку индекса", false)
	m.Add(cmdPrecision, "Точность поиска: "+a.cfg.Precision, false)
	m.Separator()
	m.Add(cmdAutostart, "Запускать при входе в Windows", a.cfg.Autostart)
	a.mu.Lock()
	ready, ver := a.updatePath, a.updateVersion
	a.mu.Unlock()
	if ready != "" {
		m.Add(cmdCheck, "Установить обновление "+ver, false)
	} else {
		m.Add(cmdCheck, "Проверить обновления", false)
	}
	m.Separator()
	m.Add(cmdConfig, "Открыть конфиг", false)
	m.Add(cmdLog, "Открыть лог", false)
	m.Add(cmdAbout, "О программе", false)
	m.Separator()
	m.Default(cmdShow)
	m.Add(cmdExit, "Выход", false)
	a.handleCommand(m.Track(a.hwnd))
}

func prettifyHotkey(s string) string {
	parts := strings.Split(s, "+")
	for i, p := range parts {
		switch p {
		case "ctrl":
			parts[i] = "Ctrl"
		case "alt":
			parts[i] = "Alt"
		case "shift":
			parts[i] = "Shift"
		case "win":
			parts[i] = "Win"
		case "space":
			parts[i] = "Space"
		default:
			if len(p) > 0 {
				parts[i] = strings.ToUpper(p[:1]) + p[1:]
			}
		}
	}
	return strings.Join(parts, "+")
}

func (a *App) handleCommand(id uint32) {
	switch id {
	case cmdShow:
		a.showLauncher()
	case cmdReindex:
		go func() {
			a.reindex(false)
			a.balloon("Индекс обновлён",
				fmt.Sprintf("Найдено программ: %d", a.snapshot().Len()), win.NIIF_INFO)
		}()
	case cmdAddFolder:
		a.addFolderDialog()
	case cmdOpenFolder:
		win.Open(dataDir(), "", "")
	case cmdPrecision:
		switch a.cfg.Precision {
		case "regular":
			a.cfg.Precision = "loose"
		case "loose":
			a.cfg.Precision = "strict"
		default:
			a.cfg.Precision = "regular"
		}
		a.saveConfig()
		a.balloon("Точность поиска", a.cfg.Precision, win.NIIF_INFO)
	case cmdAutostart:
		a.cfg.Autostart = !a.cfg.Autostart
		a.saveConfig()
		a.syncAutostart()
		if a.cfg.Autostart {
			a.balloon("Автозапуск включён", "Программа будет стартовать при входе в Windows.", win.NIIF_INFO)
		} else {
			a.balloon("Автозапуск выключен", "", win.NIIF_INFO)
		}
	case cmdCheck:
		a.mu.Lock()
		ready := a.updatePath
		a.mu.Unlock()
		if ready != "" {
			if err := a.applyUpdate(); err != nil {
				a.balloon("Обновление", err.Error(), win.NIIF_WARNING)
			}
			return
		}
		a.checkUpdates(true)
	case cmdConfig:
		win.Open(configPath(), "", "")
	case cmdLog:
		win.Open(logPath(), "", "")
	case cmdAbout:
		win.MessageBox(a.aboutText(), appName+" "+version, win.MB_OK|win.MB_ICONINFORMATION)
	case cmdExit:
		win.PostMessage(a.hwnd, win.WM_CLOSE, 0, 0)
	}
}

func (a *App) aboutText() string {
	dirs := "Стартовое меню"
	if len(a.cfg.Folders) > 0 {
		dirs = fmt.Sprintf("Меню + %d папк(и)", len(a.cfg.Folders))
	}
	return fmt.Sprintf(`%s %s
Yet another launcher

Хоткей: %s
Источники: %s
Программ в индексе: %d
Точность поиска: %s
Автозапуск: %s
Обновления: %s

Управление:
  Enter            запустить
  Ctrl+Enter       показать в папке
  Ctrl+Shift+Enter запустить от администратора
  ↑ ↓              выбор
  Esc              очистить / закрыть

Конфиг: %s
Лог:    %s

%s`,
		appName, version, prettifyHotkey(a.hk.Name), dirs, a.snapshot().Len(),
		a.cfg.Precision, autostartText(a.cfg), autoUpdateText(a.cfg),
		configPath(), logPath(), repoURL)
}

func autostartText(cfg Config) string {
	if cfg.Autostart {
		return "включён"
	}
	return "выключен"
}

func autoUpdateText(cfg Config) string {
	if !cfg.AutoUpdate {
		return "выключены"
	}
	return fmt.Sprintf("проверка каждые %d ч", cfg.UpdateCheckHours)
}

func helpText() string {
	return appName + ` ` + version + `
Yet another launcher: поиск и запуск программ по горячей клавише.

По умолчанию Ctrl+Alt+Space открывает окно поиска на том мониторе, где
находится курсор. Индексируются Стартовое меню, ключи App Paths в реестре и
папки, добавленные в конфиг.

Флаги:
  --version   показать версию
  --help      это сообщение

Конфиг: ` + configPath() + `
Лог:    ` + logPath()
}
