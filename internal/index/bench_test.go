package index

import (
	"fmt"
	"testing"
	"time"
)

// BenchmarkSearch checks that a full fuzzy pass over a big index stays cheap.
// This is the part of a keystroke that must not stall the message loop; the
// slow part used to be icon extraction, which is now off the UI thread.
func BenchmarkSearch(b *testing.B) {
	ix := New()
	for i := 0; i < 5000; i++ {
		ix.items = append(ix.items, &Item{
			Name:           fmt.Sprintf("Application %d Name With CamelCase %d", i, i%7),
			Path:           fmt.Sprintf(`C:\Program Files\App %d\App.exe`, i),
			Kind:           KindExe,
			ExecutableName: fmt.Sprintf("App %d.exe", i),
			Modified:       int64(i),
		})
	}
	ix.byName = ix.items
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ix.Search(Query{Text: "app name camel", Limit: 40, Precision: 50})
	}
}

func TestSearchIsFast(t *testing.T) {
	ix := New()
	for i := 0; i < 8000; i++ {
		ix.items = append(ix.items, &Item{
			Name: fmt.Sprintf("Program %d Tool", i),
			Path: fmt.Sprintf(`C:\Tools\P%d.exe`, i),
			Kind: KindExe,
		})
	}
	ix.byName = ix.items
	start := time.Now()
	for _, q := range []string{"a", "to", "prog", "tool"} {
		ix.Search(Query{Text: q, Limit: 40, Precision: 50})
	}
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Fatalf("four searches took %v; the UI must stay in the millisecond range", elapsed)
	}
	t.Logf("four searches over 8000 items: %v", elapsed)
}
