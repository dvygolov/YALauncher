package index

import (
	"sort"
)

// KindName is the human readable kind shown in the result list.
func (k Kind) String() string {
	switch k {
	case KindExe:
		return "exe"
	case KindShortcut:
		return "lnk"
	case KindURL:
		return "url"
	case KindAppRef:
		return "clickonce"
	case KindUWP:
		return "uwp"
	}
	return "?"
}

// Query restricts the search.
type Query struct {
	Text string
	// Empty is the "show something useful" mode: the most used programs.
	Empty bool
	// Limit caps the number of results (the visible list plus a little).
	Limit int
	// Precision is the minimum fuzzy score.
	Precision int
	// Frequent supplies the usage bonus and the empty-query ranking.
	Frequent *MostFrequent
}

// Search scores every program and returns the best ones. Programs are few
// (a couple of thousand), so a linear pass over pre-folded names is faster
// than any index structure and stays in the low milliseconds.
func (ix *Index) Search(q Query) []*Item {
	if ix == nil {
		return nil
	}
	if q.Limit <= 0 {
		q.Limit = 50
	}
	if q.Empty {
		return ix.mostUsed(q)
	}
	query := searchTruncate(q.Text)
	if query == "" {
		return ix.mostUsed(q)
	}
	type scored struct {
		it    *Item
		score int
	}
	results := make([]scored, 0, 64)
	for _, it := range ix.items {
		score, ok := scoreItem(it, query, q.Precision, q.Frequent)
		if !ok {
			continue
		}
		results = append(results, scored{it, score})
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].score != results[j].score {
			return results[i].score > results[j].score
		}
		// Same score: the shorter name is usually the intended one, and the
		// more recently installed file is a decent second tie breaker.
		li, lj := len(results[i].it.Name), len(results[j].it.Name)
		if li != lj {
			return li < lj
		}
		return results[i].it.Modified > results[j].it.Modified
	})
	if len(results) > q.Limit {
		results = results[:q.Limit]
	}
	out := make([]*Item, 0, len(results))
	for _, r := range results {
		it := *r.it
		it.Score = r.score
		out = append(out, &it)
	}
	return out
}

func (ix *Index) mostUsed(q Query) []*Item {
	all := make([]*Item, 0, len(ix.items))
	for _, it := range ix.items {
		all = append(all, it)
	}
	sort.SliceStable(all, func(i, j int) bool {
		bi, bj := 0, 0
		if q.Frequent != nil {
			bi, bj = q.Frequent.Bonus(all[i].Key()), q.Frequent.Bonus(all[j].Key())
		}
		if bi != bj {
			return bi > bj
		}
		if all[i].Modified != all[j].Modified {
			return all[i].Modified > all[j].Modified
		}
		return all[i].Name < all[j].Name
	})
	if len(all) > q.Limit {
		all = all[:q.Limit]
	}
	out := make([]*Item, 0, len(all))
	for _, it := range all {
		cp := *it
		out = append(out, &cp)
	}
	return out
}

// MatchIndices returns the positions in s that the query matched, for the
// highlight. It is only computed for the dozen visible rows.
func MatchIndices(query, s string) []int {
	return searchIndices(query, s)
}

// ByKey returns the program with the given identity (see Item.Key).
func (ix *Index) ByKey(key string) *Item {
	if ix == nil {
		return nil
	}
	ix.muByKey.Lock()
	defer ix.muByKey.Unlock()
	if ix.byKey == nil {
		ix.byKey = make(map[string]*Item, len(ix.items))
		for _, it := range ix.items {
			ix.byKey[it.Key()] = it
		}
	}
	return ix.byKey[key]
}
