// Package fuzzy provides a small fzf-style subsequence scorer for filename
// matching. It returns a normalized score in [0, 1] and the byte offsets of
// matched runes in the candidate.
package fuzzy

import (
	"sort"
	"unicode"
	"unicode/utf8"
)

// Candidate is a scored item carrying an opaque payload alongside its text.
type Candidate struct {
	Text    string
	Payload any
}

// Scored is the result of scoring a Candidate against a pattern.
type Scored struct {
	Score   float64
	Matched []int
	Payload any
}

// Score scores how well pattern fuzzy-matches candidate. Returns 0 when no
// subsequence match exists. Higher = better match.
//
// The algorithm is a greedy left-to-right subsequence scan with weighted
// bonuses:
//   - Proportional base score for the fraction of pattern chars matched.
//   - +0.15 for each consecutive run of matched indices.
//   - +0.5 for a match at a word-boundary (start of string, after '/', '_',
//     '-', '.', ' ', or at a lowercase→uppercase camelCase transition).
//   - +0.05 if the original (non-lowercased) candidate char equals the pattern
//     char (case bonus).
//   - −0.1 per gap character between matches, capped at −0.5 per match event.
//
// If pattern is empty Score returns (1.0, []int{}) — a vacuous match.
// If candidate is empty and pattern is non-empty Score returns (0, nil).
// Final score is clamped to [0, 1].
func Score(pattern, candidate string) (score float64, matched []int) {
	if len(pattern) == 0 {
		return 1.0, []int{}
	}
	if len(candidate) == 0 {
		return 0, nil
	}

	patRunes := []rune(pattern)
	lowerPat := toLowerRunes(patRunes)

	// Collect candidate rune info: rune value, lowercase rune, byte offset,
	// and whether it sits at a word boundary.
	type runeInfo struct {
		r        rune
		lower    rune
		byteOff  int
		wordBnd  bool
		prevLower bool // previous rune was lowercase (for camelCase detection)
	}

	infos := make([]runeInfo, 0, len(candidate))
	byteOff := 0
	var prevR rune
	for i, r := range candidate {
		_ = i
		wb := isWordBoundary(prevR, r)
		infos = append(infos, runeInfo{
			r:         r,
			lower:     unicode.ToLower(r),
			byteOff:   byteOff,
			wordBnd:   wb,
			prevLower: prevR != 0 && unicode.IsLower(prevR),
		})
		byteOff += utf8.RuneLen(r)
		prevR = r
	}

	// Greedy left-to-right subsequence match.
	patIdx := 0
	matched = make([]int, 0, len(lowerPat))
	ci := 0
	for ci < len(infos) && patIdx < len(lowerPat) {
		if infos[ci].lower == lowerPat[patIdx] {
			matched = append(matched, infos[ci].byteOff)
			patIdx++
		}
		ci++
	}

	if patIdx < len(lowerPat) {
		// Pattern could not be fully matched.
		return 0, nil
	}

	// Now compute score from the matched positions.
	// matched holds the byte offsets; we need the runeInfo indices for scoring.
	// Rebuild: map byteOff → runeInfo index.
	offToIdx := make(map[int]int, len(infos))
	for i, ri := range infos {
		offToIdx[ri.byteOff] = i
	}

	const (
		consecutiveBonus = 0.15
		wordBoundBonus   = 0.5
		caseBonus        = 0.05
		gapPenalty       = 0.1
		maxGapPenalty    = 0.5
	)

	raw := float64(len(matched)) / float64(len(patRunes)) // base proportional

	prevCandIdx := -2
	for pi, byteOffset := range matched {
		candIdx := offToIdx[byteOffset]
		ri := infos[candIdx]

		// Consecutive bonus.
		if candIdx == prevCandIdx+1 {
			raw += consecutiveBonus
		}

		// Word boundary bonus.
		if ri.wordBnd {
			raw += wordBoundBonus
		}

		// Case bonus: original candidate rune == original pattern rune.
		if ri.r == patRunes[pi] {
			raw += caseBonus
		}

		// Gap penalty: number of candidate chars skipped since last match.
		gap := 0
		if prevCandIdx >= 0 {
			gap = candIdx - prevCandIdx - 1
		}
		if gap > 0 {
			penalty := float64(gap) * gapPenalty
			if penalty > maxGapPenalty {
				penalty = maxGapPenalty
			}
			raw -= penalty
		}

		prevCandIdx = candIdx
	}

	// Normalize: target is the theoretical maximum raw score for a perfectly
	// consecutive, word-boundary-starting, case-matching pattern of this length:
	//   base(1.0) + (n-1)*consecutiveBonus + wordBoundBonus + n*caseBonus
	// = 1.5 + n*0.20 - 0.15
	// This ensures a perfect match on a same-length prefix scores exactly 1.0,
	// while partial/gapped matches score proportionally lower.
	n := float64(len(patRunes))
	target := 1.5 + n*0.20 - 0.15
	normalized := raw / target

	// Clamp to [0, 1].
	if normalized < 0 {
		normalized = 0
	}
	if normalized > 1 {
		normalized = 1
	}

	return normalized, matched
}

// RescoreTopN scores each candidate in candidates against pattern and returns
// the top-n Scored results ordered by score descending. If n <= 0 or n >=
// len(candidates) all matching candidates are returned.
func RescoreTopN(pattern string, candidates []Candidate, n int) []Scored {
	results := make([]Scored, 0, len(candidates))
	for _, c := range candidates {
		s, m := Score(pattern, c.Text)
		if len(pattern) > 0 && s == 0 {
			continue
		}
		results = append(results, Scored{
			Score:   s,
			Matched: m,
			Payload: c.Payload,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if n > 0 && n < len(results) {
		return results[:n]
	}
	return results
}

// toLowerRunes returns a slice of lower-cased runes from rs.
func toLowerRunes(rs []rune) []rune {
	out := make([]rune, len(rs))
	for i, r := range rs {
		out[i] = unicode.ToLower(r)
	}
	return out
}

// isWordBoundary reports whether position p (with previous rune prev) is a
// word boundary: start of string, after '/', '_', '-', '.', ' ', or at a
// lowercase→uppercase camelCase transition.
func isWordBoundary(prev, cur rune) bool {
	if prev == 0 {
		return true // start of string
	}
	switch prev {
	case '/', '_', '-', '.', ' ':
		return true
	}
	// camelCase: previous was lowercase, current is uppercase.
	if unicode.IsLower(prev) && unicode.IsUpper(cur) {
		return true
	}
	return false
}
