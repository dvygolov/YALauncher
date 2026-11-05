package index

import (
	"os"
	"path/filepath"
	"testing"
)

func fixtureIndex(t *testing.T, names []string) *Index {
	t.Helper()
	ix := New()
	for i, n := range names {
		ix.items = append(ix.items, &Item{Name: n, Path: `C:\Program Files\` + n + `.exe`, Kind: KindExe, Modified: int64(i)})
	}
	ix.byName = ix.items
	return ix
}

func TestSearchOrdersByQuality(t *testing.T) {
	ix := fixtureIndex(t, []string{"Chrome", "Google Chrome Canary", "Chromium", "Notepad"})
	res := ix.Search(Query{Text: "chrome", Limit: 10, Precision: 50})
	if len(res) == 0 {
		t.Fatal("no results")
	}
	if res[0].Name != "Chrome" {
		t.Fatalf("expected Chrome first, got %s", res[0].Name)
	}
	for _, r := range res {
		if r.Name == "Notepad" {
			t.Fatal("Notepad must not match")
		}
	}
}

func TestMostUsedWinsOnShortQuery(t *testing.T) {
	ix := fixtureIndex(t, []string{"Chrome", "Chess", "Chrome Remote Desktop"})
	freq := NewMostFrequent()
	for i := 0; i < 10; i++ {
		freq.Hit(ix.items[0].Key(), int64(i))
		freq.Hit(ix.items[0].Key(), int64(i))
	}
	res := ix.Search(Query{Text: "ch", Limit: 10, Precision: 50, Frequent: freq})
	if len(res) == 0 {
		t.Fatal("no results")
	}
	if res[0].Name != "Chrome" {
		t.Fatalf("expected the often used Chrome first, got %s", res[0].Name)
	}
}

func TestEmptyQueryReturnsRecentPrograms(t *testing.T) {
	ix := fixtureIndex(t, []string{"A", "B", "C"})
	res := ix.Search(Query{Empty: true, Limit: 2})
	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}
	if res[0].Name != "C" {
		t.Fatalf("expected the newest first, got %s", res[0].Name)
	}
}

func TestScanFindsSuffixes(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"App.exe", "Shortcut.lnk", "Site.url", "note.txt", "UnInstall.exe"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ix := Scan(Options{Folders: []string{dir}}, nil, nil, nil)
	names := map[string]bool{}
	for _, it := range ix.Items() {
		names[it.Name] = true
	}
	// itemFromFile is windows-only; on other hosts the scan yields nothing.
	if !names["App"] && !names["Shortcut"] {
		t.Skip("scanner is Windows only")
	}
	if names["note"] {
		t.Fatal("txt must not be indexed")
	}
	if names["UnInstall"] {
		t.Fatal("uninstaller must be dropped")
	}
}

func TestCacheRoundTrip(t *testing.T) {
	ix := fixtureIndex(t, []string{"One", "Two"})
	path := filepath.Join(t.TempDir(), "cache.json")
	if err := ix.Save(path); err != nil {
		t.Fatal(err)
	}
	got := LoadCache(path, 24*3600*1e9)
	if got == nil || got.Len() != 2 {
		t.Fatalf("cache did not round trip: %+v", got)
	}
}
