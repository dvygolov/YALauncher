//go:build windows

package index

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dvygolov/yalauncher/internal/win"
)

// Watch keeps the index fresh while the launcher runs. File system
// notifications are the fastest path, but the session may span hours and
// installers do not always raise them (network drives, Yandex Disk folders),
// so a low frequency rescan runs as well. Flow Launcher does the same pair:
// a watcher plus a reindex every 30 hours.
type Watch struct {
	mu      sync.Mutex
	roots   []string
	changes chan struct{}
	stop    chan struct{}
	once    sync.Once
	watched []win.HANDLE

	// OnChange is called (from the timer goroutine) when the index is stale.
	OnChange func()

	// IdleScan is how often a full rescan runs even without notifications.
	IdleScan time.Duration
	// Debounce coalesces bursts of file events into one rescan.
	Debounce time.Duration
}

// NewWatch starts watching the folders of an options set.
func NewWatch(opts Options, onChange func()) *Watch {
	w := &Watch{
		changes:  make(chan struct{}, 1),
		stop:     make(chan struct{}),
		OnChange: onChange,
		IdleScan: 30 * time.Minute,
		Debounce: 1200 * time.Millisecond,
	}
	w.roots = collectRoots(opts)
	go w.run()
	go w.wait()
	return w
}

func (w *Watch) run() {
	win.SetThreadDesktopReady()
	if err := win.CoInitialize(); err == nil {
		defer win.CoUninitialize()
	}
	for _, root := range w.roots {
		if _, err := statDir(root); err != nil {
			continue
		}
		w.watchRoot(root)
	}
	debounce := time.NewTimer(0)
	if !debounce.Stop() {
		<-debounce.C
	}
	idle := time.NewTicker(w.IdleScan)
	defer idle.Stop()
	defer debounce.Stop()
	for {
		select {
		case <-w.stop:
			return
		case <-w.changes:
			debounce.Reset(w.Debounce)
		case <-debounce.C:
			w.notify()
			debounce.Stop()
			<-debounce.C
		case <-idle.C:
			w.notify()
		}
	}
}

// wait blocks on every handle of every watched folder at once.
func (w *Watch) wait() {
	for {
		select {
		case <-w.stop:
			w.mu.Lock()
			for _, h := range w.watched {
				win.CloseChangeHandle(h)
			}
			w.watched = nil
			w.mu.Unlock()
			return
		case <-time.After(400 * time.Millisecond):
		}
		w.mu.Lock()
		handles := append([]win.HANDLE(nil), w.watched...)
		w.mu.Unlock()
		if len(handles) == 0 {
			continue
		}
		if idx, ok := win.WaitAny(handles, 300); ok && idx < len(handles) {
			win.FindNextChange(handles[idx])
			w.Touch()
		}
	}
}

func (w *Watch) notify() {
	if w.OnChange != nil {
		w.OnChange()
	}
}

// Touch reports a change from outside (the app calls it after the user
// rejects a program, for instance).
func (w *Watch) Touch() {
	select {
	case w.changes <- struct{}{}:
	default:
	}
}

// Close stops the watcher and every pending read.
func (w *Watch) Close() {
	w.once.Do(func() { close(w.stop) })
}

// Roots returns the watched folders, for the log and the settings UI.
func (w *Watch) Roots() []string { return append([]string(nil), w.roots...) }

func statDir(path string) (bool, error) {
	st, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return st.IsDir(), nil
}

func readDirNames(dir string) ([]os.DirEntry, error) { return os.ReadDir(dir) }

// watchRoot opens one notification handle per folder. watchOne is supplied by
// the platform file because the Win32 call needs a completion routine whose
// lifetime is tied to this object.
func (w *Watch) watchRoot(root string) {
	w.watchPath(root, true)
}

// ---- Win32 FindFirstChangeNotification based watcher ------------------------
//
// A completion routine based ReadDirectoryChangesW would need a callback per
// handle; FindFirstChangeNotification + WaitForMultipleObjects is simpler and
// raises the same change classes (file name, directory name, size, write).

const maxWatchers = 64

func (w *Watch) watchPath(root string, recursive bool) {
	// FindFirstChangeNotification does not take a recursion flag, and a folder
	// tree of a start menu is only two levels deep, so every sub folder gets
	// its own handle; the cap keeps a huge user folder from eating handles.
	w.mu.Lock()
	room := maxWatchers - len(w.watched)
	w.mu.Unlock()
	if room <= 0 {
		return
	}
	paths := []string{root}
	if recursive {
		paths = append(paths, subDirs(root, 3, room)...)
	}
	for _, p := range paths {
		h := win.WatchDirectory(p, recursive)
		if len(h) == 0 {
			continue
		}
		w.mu.Lock()
		w.watched = append(w.watched, h[0])
		w.mu.Unlock()
	}
}

func subDirs(root string, depth, limit int) []string {
	var out []string
	var walk func(dir string, level int)
	walk = func(dir string, level int) {
		if level > depth || len(out) >= limit {
			return
		}
		entries, err := readDirNames(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			out = append(out, filepath.Join(dir, e.Name()))
			if len(out) >= limit {
				return
			}
			walk(filepath.Join(dir, e.Name()), level+1)
		}
	}
	walk(root, 1)
	return out
}
