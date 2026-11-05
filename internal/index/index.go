// Package index scans the folders Flow Launcher scans by default - the two
// Start Menu trees, the HKCU/HKLM "App Paths" registry keys, optional user
// folders - and builds the searchable list of programs.
//
// A folder scan looks for .exe, .lnk, .url and .appref-ms files, exactly the
// suffixes Flow Launcher's Program plugin indexes out of the box. Shortcuts
// are resolved so that a start menu entry is shown with the name of the
// shortcut and matches the name of its target, and uninstallers are dropped
// the way Flow Launcher drops them.
package index

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Kind tells the launcher how to start the entry.
type Kind uint8

const (
	KindExe Kind = iota
	KindShortcut
	KindURL
	KindAppRef
	KindUWP
)

// Item is one program the launcher can start.
type Item struct {
	Name string // what the user searches for
	Path string // the file that gets started (.exe, .lnk, .url ...)
	Args string // arguments baked into the entry (shortcut arguments)
	Kind Kind

	// ExecutableName is the file name of the target of a shortcut; it is a
	// secondary search key because start menu shortcuts are often named after
	// the product while the exe is named after the company.
	ExecutableName string
	// Description is the shortcut comment, used by some installers to say
	// what the program is.
	Description string

	Source   string // folder or "registry" / "apps"
	Modified int64  // unix seconds, used to break ties
	// Score is the match score of the current query.
	Score int
}

// Key is the identity of an item: the file it starts plus its arguments.
func (i *Item) Key() string {
	return strings.ToLower(i.Path + "\x00" + i.Args)
}

// Suffixes is the list of indexed extensions, by default what Flow Launcher
// indexes for programs.
var Suffixes = []string{".exe", ".lnk", ".url", ".appref-ms"}

type registrySource struct {
	key  string // flat list of files discovered in App Paths
	root uintptr
}

// Progress is reported while a scan is running.
type Progress struct {
	Done  int
	Total int
	Phase string
}

// Index is an immutable snapshot of the programs plus the query cache.
type Index struct {
	items    []*Item
	byName   []*Item
	seen     map[string]bool
	BuiltAt  int64
	ScannedN int

	// byKey is built lazily on the first history lookup.
	muByKey sync.Mutex
	byKey   map[string]*Item
}

// New returns an empty index.
func New() *Index {
	return &Index{seen: make(map[string]bool)}
}

// Items exposes the snapshot.
func (ix *Index) Items() []*Item { return ix.items }

// Len returns the number of programs.
func (ix *Index) Len() int { return len(ix.items) }

// Options configures a scan.
type Options struct {
	// Folders are user configured roots, scanned recursively.
	Folders []string
	// StartMenu enables the two Start Menu trees and the Program Files scan
	// of shortcuts (Flow Launcher's "Start Menu source").
	StartMenu bool
	// AppPaths enables the registry source.
	AppPaths bool
	// EnvPath enables the folders from the PATH variable (non recursive).
	EnvPath bool
	// Recursive enables descending into sub folders of Folders.
	Recursive bool
	// IncludeHidden keeps hidden and system files.
	IncludeHidden bool
}

// Scan builds a new index. Progress is called from the scanning goroutine;
// pass nil to skip. Rejected entries are those whose Key is in skip.
func Scan(opts Options, skip map[string]bool, progress func(Progress), cancel func() bool) *Index {
	roots := collectRoots(opts)
	var (
		mu    sync.Mutex
		found []*Item
		seen  = make(map[string]*Item)
	)

	add := func(it *Item) {
		if it == nil || it.Name == "" {
			return
		}
		key := it.Key()
		mu.Lock()
		if old, ok := seen[key]; ok {
			// Prefer the more informative duplicate: a shortcut carries a
			// comment and its target name, so it wins over a bare exe.
			if it.Kind == KindShortcut && old.Kind != KindShortcut {
				*old = *it
			}
			mu.Unlock()
			return
		}
		if skip != nil && skip[key] {
			mu.Unlock()
			return
		}
		seen[key] = it
		found = append(found, it)
		mu.Unlock()
	}

	total := len(roots)
	for i, root := range roots {
		if cancel != nil && cancel() {
			break
		}
		if progress != nil {
			progress(Progress{Done: i, Total: total, Phase: root})
		}
		scanRoot(root, opts.Recursive, opts.IncludeHidden, add, cancel)
	}

	if opts.AppPaths {
		if progress != nil {
			progress(Progress{Done: total, Total: total + 1, Phase: "реестр: App Paths"})
		}
		for _, it := range scanAppPaths() {
			add(it)
		}
	}

	ix := New()
	ix.items = dedupe(found)
	ix.ScannedN = len(roots)
	ix.BuiltAt = nowUnix()
	ix.byName = ix.items
	return ix
}

// collectRoots builds the list of folders to walk. The start menu is walked
// first: those are the entries the user expects to find.
func collectRoots(opts Options) []string {
	var out []string
	seen := map[string]bool{}
	push := func(dir string) {
		if dir == "" {
			return
		}
		clean := filepath.Clean(dir)
		key := strings.ToLower(clean)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, clean)
	}
	if opts.StartMenu {
		push(knownFolder(csidlPrograms))
		push(knownFolder(csidlCommonPrograms))
	}
	for _, f := range opts.Folders {
		push(stripQuotes(f))
	}
	if opts.EnvPath {
		for _, p := range filepath.SplitList(os.Getenv("PATH")) {
			push(stripQuotes(p))
		}
	}
	return out
}

func stripQuotes(s string) string {
	return strings.Trim(strings.TrimSpace(s), `"'`)
}

func scanRoot(root string, recursive, hidden bool, add func(*Item), cancel func() bool) {
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		return
	}
	walk(root, recursive, hidden, add, cancel)
}

func walk(dir string, recursive, hidden bool, add func(*Item), cancel func() bool) {
	entries, err := os.ReadDir(dir) // sorted by name
	if err != nil {
		return
	}
	for _, e := range entries {
		if cancel != nil && cancel() {
			return
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(dir, name)
		if e.IsDir() {
			if recursive {
				walk(full, recursive, hidden, add, cancel)
			}
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if !hidden && info.Mode()&(os.ModeSymlink) != 0 {
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		if !suffixAllowed(ext) {
			continue
		}
		it := itemFromFile(full, info.ModTime().Unix(), dir)
		add(it)
	}
}

func suffixAllowed(ext string) bool {
	for _, s := range Suffixes {
		if ext == s {
			return true
		}
	}
	return false
}

// Root returns the source label for a file: the folder it was found in.
func RootOf(path string) string { return filepath.Dir(path) }

func dedupe(in []*Item) []*Item {
	sort.SliceStable(in, func(i, j int) bool {
		return strings.ToLower(in[i].Name) < strings.ToLower(in[j].Name)
	})
	return in
}
