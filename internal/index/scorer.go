package index

import (
	"strings"

	"github.com/dvygolov/yalauncher/internal/search"
)

func searchTruncate(q string) string { return strings.TrimSpace(search.Truncate(q)) }

func searchIndices(query, s string) []int {
	m, ok := search.Match(query, s, search.Loose)
	if !ok {
		return nil
	}
	return m.Indices
}

// scoreItem matches a program against the query. The name carries the most
// weight, then the name of the target executable (shortcuts are named after
// the product, the exe after the company), then the shortcut comment. The
// usage bonus from MostFrequent is added at the end.
func scoreItem(it *Item, query string, precision int, freq *MostFrequent) (int, bool) {
	if precision <= 0 {
		precision = int(search.Regular)
	}
	best := -1
	if m, ok := search.Match(query, it.Name, search.Precision(precision)); ok {
		best = m.Score
	}
	// Secondary keys use a lower bar: they only break ties between programs
	// whose names are equally good.
	if it.ExecutableName != "" && !strings.EqualFold(it.ExecutableName, it.Name) {
		if m, ok := search.Match(query, it.ExecutableName, search.Precision(precision-15)); ok && m.Score-10 > best {
			best = m.Score - 10
		}
	}
	if it.Description != "" {
		if m, ok := search.Match(query, it.Description, search.Precision(precision)); ok && m.Score-25 > best {
			best = m.Score - 25
		}
	}
	if best < 0 {
		return 0, false
	}
	if freq != nil {
		best += freq.Bonus(it.Key())
	}
	if best > 100 {
		best = 100
	}
	return best, true
}
