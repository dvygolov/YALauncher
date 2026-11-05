// Package search is the ranking engine of the launcher: a port of the fuzzy
// matcher Flow Launcher uses, with the same two stages - acronym matching and
// a character-by-character fuzzy pass - and the same score formula, so that
// "vs19" finds "Visual Studio 2019" and "chr" finds "Google Chrome".
package search

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Precision is the minimum score a match has to reach to be offered to the
// user. Flow Launcher exposes the same three presets.
type Precision int

const (
	Loose    Precision = 30
	Regular  Precision = 50
	Strict   Precision = 70
	scoreMax           = 100
)

// Match is one scored result.
type Result struct {
	Score int
	// Indices are the rune offsets in the compared text that the query
	// matched; the UI highlights them.
	Indices []int
}

// Match compares the query against the text. ok is false when the text does
// not contain the query with the required precision.
func Match(query, text string, precision Precision) (Result, bool) {
	query = strings.TrimSpace(query)
	return matchText(query, text, precision)
}

// Fold lowercases a string the way the matcher does: Russian and Latin both
// end up in the low range, so "Яндекс" matches "яндекс".
func Fold(s string) string { return strings.ToLower(s) }

func matchText(query, text string, precision Precision) (Result, bool) {
	if query == "" {
		return Result{Score: scoreMax - 1}, true
	}
	if text == "" {
		return Result{}, false
	}
	q := []rune(strings.ToLower(query))
	c := []rune(strings.ToLower(text))
	// original case matters for the acronym rule (upper case starts a word)
	orig := []rune(text)

	// ---- acronym match -------------------------------------------------
	// A character is an acronym when it is the first character, follows a
	// space or a separator, is upper case in the original text, or is a
	// digit. Contiguous digits form a single group.
	acronymIdx := make([]int, 0, len(q))
	groups := 0
	matched := 0
	for i := 0; i < len(orig); i++ {
		if !isAcronym(orig, i) {
			continue
		}
		if matched < len(q) && c[i] == q[matched] {
			acronymIdx = append(acronymIdx, i)
			matched++
			if !isAcronymGroupStart(orig, i) {
				continue
			}
		}
		if isAcronymGroupStart(orig, i) {
			groups++
		}
	}
	if matched > 0 && matched == len(q) && groups > 0 {
		matchedGroups := distinctGroups(acronymIdx, orig)
		acronymScore := matchedGroups * scoreMax / groups
		if acronymScore >= int(precision) {
			return Result{Score: acronymScore, Indices: dedupe(acronymIdx)}, true
		}
	}

	// ---- fuzzy match ---------------------------------------------------
	// The query is split on spaces; every part must appear in order, and the
	// score rewards early, dense matches.
	parts := strings.Fields(query)
	if len(parts) == 0 {
		return Result{Score: scoreMax - 1}, true
	}
	idxs := make([]int, 0, len(q))
	pi := 0
	part := []rune(strings.ToLower(parts[0]))
	ci := 0
	firstMatch := -1
	lastMatch := 0
	allContained := true
	spaces := 0
	matchedPart := false
	for i := 0; i < len(c); i++ {
		if pi >= len(parts) {
			break
		}
		if part[ci] == c[i] {
			if firstMatch < 0 {
				firstMatch = i
			}
			idxs = append(idxs, i)
			lastMatch = i + 1
			ci++
			matchedPart = true
			if ci == len(part) {
				pi++
				if pi < len(parts) {
					part = []rune(strings.ToLower(parts[pi]))
				}
				ci = 0
			}
		}
	}
	if pi < len(parts) {
		return Result{}, false
	}
	// count spaces before the first match to normalise the position bonus
	for i := 0; i < firstMatch && i < len(c); i++ {
		if c[i] == ' ' {
			spaces++
		}
	}
	_ = matchedPart
	firstWordStart := firstMatch
	for firstWordStart > 0 && c[firstWordStart-1] != ' ' && !isSeparator(c[firstWordStart-1]) {
		firstWordStart--
	}
	prefix := firstMatch - firstWordStart
	matchLen := lastMatch - firstMatch
	score := scoreMax * (len(q) + 1) / (1 + prefix + matchLen + 1)
	if prefix == 0 && !allContained {
		score -= spaces
	}
	diff := len(c) - len(q)
	switch {
	case diff < 5:
		score += 20
	case diff < 10:
		score += 10
	}
	if len(parts) == 1 {
		n := 0
		for _, r := range q {
			if !unicode.IsSpace(r) {
				n++
			}
		}
		if n <= 4 {
			score += n * 10
		} else {
			score += 40 + (n-4)*5
		}
	}
	if score > scoreMax-1 {
		score = scoreMax - 1
	}
	if score < int(precision) {
		return Result{}, false
	}
	return Result{Score: score, Indices: dedupe(idxs)}, true
}

func isAcronym(r []rune, i int) bool {
	if r[i] >= '0' && r[i] <= '9' {
		return true
	}
	if unicode.IsUpper(r[i]) || unicode.IsTitle(r[i]) {
		return true
	}
	if i == 0 {
		return true
	}
	p := r[i-1]
	return p == ' ' || isSeparator(p)
}

func isAcronymGroupStart(r []rune, i int) bool {
	if r[i] >= '0' && r[i] <= '9' {
		return i == 0 || r[i-1] < '0' || r[i-1] > '9'
	}
	return true
}

func isSeparator(r rune) bool {
	switch r {
	case '_', '-', '.', '\\', '/', '(', '[', '+', '&', ':':
		return true
	}
	return false
}

func distinctGroups(idxs []int, text []rune) int {
	seen := make(map[int]bool, len(idxs))
	groups := 0
	for _, i := range idxs {
		if seen[i] {
			continue
		}
		seen[i] = true
		groups++
		if text[i] >= '0' && text[i] <= '9' {
			for j := i + 1; j < len(text) && text[j] >= '0' && text[j] <= '9'; j++ {
				seen[j] = true
			}
		}
	}
	return groups
}

func dedupe(in []int) []int {
	if len(in) < 2 {
		return in
	}
	out := in[:0]
	prev := -1
	for _, v := range in {
		if v != prev {
			out = append(out, v)
			prev = v
		}
	}
	return out
}

// Quantize keeps the best score per distinct text and caps the result list.
const Quantize = 100

// MaxQueryRunes protects the matcher from pathological input.
const MaxQueryRunes = 128

// Truncate shortens the query defensively.
func Truncate(q string) string {
	if utf8.RuneCountInString(q) <= MaxQueryRunes {
		return q
	}
	r := []rune(q)
	return string(r[:MaxQueryRunes])
}
