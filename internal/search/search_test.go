package search

import "testing"

func TestSubstringAndPrefix(t *testing.T) {
	cases := []struct {
		query, text string
		want        bool
	}{
		{"chrome", "Google Chrome", true},
		{"chro", "Google Chrome", true},
		{"gc", "Google Chrome", true},
		{"vs19", "Visual Studio 2019", true},
		{"vs", "Visual Studio Code", true},
		{"code", "Visual Studio Code", true},
		{"chrome", "Firefox", false},
		{"яндекс", "Яндекс Браузер", true},
		{"yb", "Яндекс Браузер", false},
		{"яб", "Яндекс Браузер", true},
		{"яндекс браузер", "Яндекс Браузер", true},
		{"яндекс", "Google Chrome", false},
	}
	for _, c := range cases {
		_, ok := Match(c.query, c.text, Regular)
		if ok != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.query, c.text, ok, c.want)
		}
	}
}

func TestRankingPrefersTighterMatches(t *testing.T) {
	exact, ok1 := Match("chrome", "Chrome", Regular)
	loose, ok2 := Match("chrome", "Google Chrome Canary", Regular)
	if !ok1 || !ok2 {
		t.Fatal("both must match")
	}
	if exact.Score < loose.Score {
		t.Fatalf("exact score %d should beat loose %d", exact.Score, loose.Score)
	}
}

func TestAcronymBeatsUnrelatedFuzzy(t *testing.T) {
	acro, ok := Match("gch", "Google Chrome", Regular)
	if !ok {
		t.Fatal("acronym must match")
	}
	if acro.Score < int(Regular) {
		t.Fatalf("score %d below precision", acro.Score)
	}
}

func TestEmptyQueryMatchesEverything(t *testing.T) {
	if m, ok := Match("", "Anything", Regular); !ok || m.Score < scoreMax-1 {
		t.Fatalf("empty query must match with the top score, got %+v %v", m, ok)
	}
}

func TestIndicesAreInOrder(t *testing.T) {
	m, ok := Match("chr", "Google Chrome", Regular)
	if !ok {
		t.Fatal("must match")
	}
	for i := 1; i < len(m.Indices); i++ {
		if m.Indices[i] <= m.Indices[i-1] {
			t.Fatalf("indices not increasing: %v", m.Indices)
		}
	}
}
