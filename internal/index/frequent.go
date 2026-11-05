package index

import (
	"sort"
	"strings"
)

// MostFrequent learns which programs the user actually starts, so that a
// short query like "ch" puts Chrome above the fourteen other entries that
// contain those letters. The counts are small and cheap to keep in memory;
// the app persists them in the config file.
type MostFrequent struct {
	Count map[string]int   `json:"count"`
	Last  map[string]int64 `json:"last"`
}

func NewMostFrequent() *MostFrequent {
	return &MostFrequent{Count: map[string]int{}, Last: map[string]int64{}}
}

// Hit records one launch.
func (m *MostFrequent) Hit(key string, at int64) {
	if m.Count == nil {
		m.Count = map[string]int{}
	}
	if m.Last == nil {
		m.Last = map[string]int64{}
	}
	m.Count[key]++
	m.Last[key] = at
	if len(m.Count) > 4096 {
		m.prune()
	}
}

func (m *MostFrequent) prune() {
	type kv struct {
		k string
		n int
	}
	list := make([]kv, 0, len(m.Count))
	for k, n := range m.Count {
		list = append(list, kv{k, n})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].n > list[j].n })
	for _, e := range list[1024:] {
		delete(m.Count, e.k)
		delete(m.Last, e.k)
	}
}

// Bonus returns the score added to a program because it is used often. It is
// capped so that a fuzzy quality match still wins over a rarely used one.
func (m *MostFrequent) Bonus(key string) int {
	if m == nil || m.Count == nil {
		return 0
	}
	n := m.Count[key]
	if n <= 0 {
		return 0
	}
	switch {
	case n >= 20:
		return 30
	case n >= 10:
		return 24
	case n >= 5:
		return 18
	case n >= 3:
		return 12
	case n >= 2:
		return 7
	}
	return 3
}

// Normalize strips the noise that would make two different spellings of the
// same query look distinct in the coaches cache.
func Normalize(q string) string {
	return strings.ToLower(strings.Join(strings.Fields(q), " "))
}
