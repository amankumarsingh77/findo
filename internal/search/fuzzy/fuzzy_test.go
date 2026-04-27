package fuzzy

import (
	"math"
	"testing"
)

// TestScore_BasicSubsequenceMatches covers the core scoring scenarios
// described in the Phase 4 spec.
func TestScore_BasicSubsequenceMatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		pattern       string
		candidate     string
		wantMinScore  float64 // inclusive lower bound for the returned score
		wantMaxScore  float64 // inclusive upper bound (use 1.0 for "high")
		wantZero      bool    // expect exactly 0
		wantMatchOffs []int   // expected byte offsets (nil = don't check)
	}{
		{
			name:          "exact prefix demo in demo.py",
			pattern:       "demo",
			candidate:     "demo.py",
			wantMinScore:  0.7,
			wantMaxScore:  1.0,
			wantMatchOffs: []int{0, 1, 2, 3},
		},
		{
			name:         "dem in dem.py — high score consecutive match",
			pattern:      "dem",
			candidate:    "dem.py",
			wantMinScore: 0.7,
			wantMaxScore: 1.0,
		},
		{
			name:         "dmo in demo.py — subsequence with gap",
			pattern:      "dmo",
			candidate:    "demo.py",
			wantMinScore: 0.1,
			wantMaxScore: 1.0,
		},
		{
			name:      "xyz not in demo.py",
			pattern:   "xyz",
			candidate: "demo.py",
			wantZero:  true,
		},
		{
			name:         "py at end of demo.py",
			pattern:      "py",
			candidate:    "demo.py",
			wantMinScore: 0.1,
			wantMaxScore: 1.0,
		},
		{
			name:         "uppercase DEMO matches demo.py without case bonus",
			pattern:      "DEMO",
			candidate:    "demo.py",
			wantMinScore: 0.1,
			wantMaxScore: 0.99, // no exact case bonus, should be slightly lower than lowercase demo
		},
		{
			name:         "demo matches Demo.PY (partial case)",
			pattern:      "demo",
			candidate:    "Demo.PY",
			wantMinScore: 0.1,
			wantMaxScore: 1.0,
		},
		{
			name:      "demo cannot match dem.py — missing o",
			pattern:   "demo",
			candidate: "dem.py",
			wantZero:  true,
		},
		{
			name:      "empty candidate non-empty pattern",
			pattern:   "demo",
			candidate: "",
			wantZero:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, matched := Score(tc.pattern, tc.candidate)

			if tc.wantZero {
				if got != 0 {
					t.Errorf("Score(%q, %q) = %v, want 0", tc.pattern, tc.candidate, got)
				}
				if matched != nil {
					t.Errorf("Score(%q, %q) matched = %v, want nil", tc.pattern, tc.candidate, matched)
				}
				return
			}

			if got < tc.wantMinScore || got > tc.wantMaxScore {
				t.Errorf("Score(%q, %q) = %v, want in [%v, %v]",
					tc.pattern, tc.candidate, got, tc.wantMinScore, tc.wantMaxScore)
			}

			if tc.wantMatchOffs != nil {
				if len(matched) != len(tc.wantMatchOffs) {
					t.Fatalf("Score(%q, %q) matched offsets = %v, want %v",
						tc.pattern, tc.candidate, matched, tc.wantMatchOffs)
				}
				for i, off := range matched {
					if off != tc.wantMatchOffs[i] {
						t.Errorf("Score(%q, %q) matched[%d] = %d, want %d",
							tc.pattern, tc.candidate, i, off, tc.wantMatchOffs[i])
					}
				}
			}
		})
	}
}

// TestScore_EmptyPattern tests the vacuous-match contract.
func TestScore_EmptyPattern(t *testing.T) {
	t.Parallel()
	score, matched := Score("", "demo.py")
	if score != 1.0 {
		t.Errorf("Score(\"\", \"demo.py\") = %v, want 1.0", score)
	}
	if matched == nil {
		t.Errorf("Score(\"\", \"demo.py\") matched = nil, want []int{}")
	}
	if len(matched) != 0 {
		t.Errorf("Score(\"\", \"demo.py\") matched = %v, want []", matched)
	}
}

// TestScore_EmptyBoth tests the degenerate case where both are empty.
func TestScore_EmptyBoth(t *testing.T) {
	t.Parallel()
	score, matched := Score("", "")
	if score != 1.0 {
		t.Errorf("Score(\"\", \"\") = %v, want 1.0", score)
	}
	if matched == nil || len(matched) != 0 {
		t.Errorf("Score(\"\", \"\") matched = %v, want []", matched)
	}
}

// TestScore_WordBoundaryBonus verifies that a match at a word boundary
// receives a higher score than a mid-word match.
func TestScore_WordBoundaryBonus(t *testing.T) {
	t.Parallel()

	// 'g' in "getUserById.go" starts right at the beginning (word boundary),
	// while in "string.go" it appears at index 3 (mid-word).
	// The word-boundary match should score higher.
	scoreGetUser, _ := Score("g", "src/getUserById.go")
	scoreString, _ := Score("g", "string.go")

	if scoreGetUser <= scoreString {
		t.Errorf("word-boundary score (%v for src/getUserById.go) should be > mid-word score (%v for string.go)",
			scoreGetUser, scoreString)
	}
}

// TestScore_OrderingConsistency verifies the ordering requirement:
// demo matching demo.py > dem.py (no match) > xemonp.py (no match).
// So really we just need demo.py to score high and the no-match cases to be 0.
func TestScore_OrderingConsistency(t *testing.T) {
	t.Parallel()

	scoreDemoPy, _ := Score("demo", "demo.py")
	scoreDemPy, _ := Score("demo", "dem.py")     // 'o' missing → 0
	scoreXemonp, _ := Score("demo", "xemonp.py") // subsequence d-e-m-o not present

	if scoreDemPy != 0 {
		t.Errorf("Score(\"demo\", \"dem.py\") = %v, want 0 (no 'o' in candidate)", scoreDemPy)
	}
	if scoreXemonp != 0 {
		t.Errorf("Score(\"demo\", \"xemonp.py\") = %v, want 0 (no 'd' in candidate)", scoreXemonp)
	}
	if scoreDemoPy <= 0 {
		t.Errorf("Score(\"demo\", \"demo.py\") = %v, want > 0", scoreDemoPy)
	}
}

// TestScore_CaseBonus checks that an exact-case match scores at least as high
// as a case-insensitive match for the same text.
func TestScore_CaseBonus(t *testing.T) {
	t.Parallel()

	scoreLower, _ := Score("demo", "demo.py")
	scoreUpper, _ := Score("DEMO", "demo.py")

	if scoreLower < scoreUpper {
		t.Errorf("lowercase match (%v) should score >= uppercase match (%v)", scoreLower, scoreUpper)
	}
}

// TestScore_PropertyStrictlyIncreasingMatchedOffsets asserts that returned
// byte offsets are always strictly increasing.
func TestScore_PropertyStrictlyIncreasingMatchedOffsets(t *testing.T) {
	t.Parallel()

	cases := []struct {
		pattern   string
		candidate string
	}{
		{"demo", "demo.py"},
		{"dmo", "demo.py"},
		{"py", "demo.py"},
		{"g", "src/getUserById.go"},
		{"ab", "abcdef"},
		{"ac", "abcdef"},
		{"ae", "abcdef"},
	}

	for _, c := range cases {
		_, matched := Score(c.pattern, c.candidate)
		if matched == nil {
			continue
		}
		for i := 1; i < len(matched); i++ {
			if matched[i] <= matched[i-1] {
				t.Errorf("Score(%q, %q): matched offsets not strictly increasing: %v",
					c.pattern, c.candidate, matched)
				break
			}
		}
	}
}

// TestScore_PropertyNeverNaNNeverNegative asserts the score is always a finite
// non-negative number.
func TestScore_PropertyNeverNaNNeverNegative(t *testing.T) {
	t.Parallel()

	cases := []struct {
		pattern   string
		candidate string
	}{
		{"demo", "demo.py"},
		{"dmo", "demo.py"},
		{"xyz", "demo.py"},
		{"", "demo.py"},
		{"demo", ""},
		{"", ""},
		{"DEMO", "demo.py"},
		{"py", "demo.py"},
		{"g", "src/getUserById.go"},
		{"g", "string.go"},
		{"longpatternwithmanychars", "x"},
	}

	for _, c := range cases {
		score, _ := Score(c.pattern, c.candidate)
		if math.IsNaN(score) {
			t.Errorf("Score(%q, %q) = NaN", c.pattern, c.candidate)
		}
		if score < 0 {
			t.Errorf("Score(%q, %q) = %v < 0", c.pattern, c.candidate, score)
		}
		if score > 1.0+1e-9 {
			t.Errorf("Score(%q, %q) = %v > 1.0", c.pattern, c.candidate, score)
		}
	}
}

// TestScore_NonASCII verifies that non-ASCII runes produce correct byte offsets.
func TestScore_NonASCII(t *testing.T) {
	t.Parallel()

	// 'é' is a two-byte UTF-8 sequence (0xC3 0xA9).
	// candidate: "résumé.pdf" — let's find "rm".
	score, matched := Score("rm", "résumé.pdf")
	if score == 0 {
		t.Fatalf("Score(\"rm\", \"résumé.pdf\") = 0, expected non-zero match")
	}
	// 'r' is at byte 0, 'm' is at byte 4 (r=0, é=1..2, s=3, u=4... wait let's compute)
	// résumé: r(0) é(1,2) s(3) u(4) m(5) é(6,7) . (8) p(9) d(10) f(11)
	// So 'r' → offset 0, 'm' → offset 5.
	if len(matched) != 2 {
		t.Fatalf("Score(\"rm\", \"résumé.pdf\") matched = %v, want 2 offsets", matched)
	}
	if matched[0] != 0 {
		t.Errorf("matched[0] = %d, want 0 (byte offset of 'r')", matched[0])
	}
	if matched[1] != 5 {
		t.Errorf("matched[1] = %d, want 5 (byte offset of 'm')", matched[1])
	}
}

// TestRescoreTopN validates the top-n selection helper.
func TestRescoreTopN(t *testing.T) {
	t.Parallel()

	candidates := []Candidate{
		{Text: "demo.py", Payload: "p1"},
		{Text: "demolition.go", Payload: "p2"},
		{Text: "dem.txt", Payload: "p3"},   // 'demo' has no 'o' in dem.txt → 0
		{Text: "xyzabc.rs", Payload: "p4"}, // no match
		{Text: "demo_test.go", Payload: "p5"},
		{Text: "demography.md", Payload: "p6"},
		{Text: "unrelated.csv", Payload: "p7"},
		{Text: "demo_utils.ts", Payload: "p8"},
		{Text: "readme.md", Payload: "p9"},
		{Text: "demo_main.go", Payload: "p10"},
	}

	top3 := RescoreTopN("demo", candidates, 3)

	if len(top3) != 3 {
		t.Fatalf("RescoreTopN returned %d results, want 3", len(top3))
	}

	for i := 1; i < len(top3); i++ {
		if top3[i].Score > top3[i-1].Score {
			t.Errorf("RescoreTopN results not sorted: top3[%d].Score=%v > top3[%d].Score=%v",
				i, top3[i].Score, i-1, top3[i-1].Score)
		}
	}

	if top3[0].Matched == nil {
		t.Errorf("top3[0].Matched = nil, want non-nil slice")
	}

	for i, s := range top3 {
		if s.Score <= 0 {
			t.Errorf("top3[%d].Score = %v, want > 0", i, s.Score)
		}
	}
}

// TestRescoreTopN_NoCandidates returns empty on empty input.
func TestRescoreTopN_NoCandidates(t *testing.T) {
	t.Parallel()
	got := RescoreTopN("demo", nil, 3)
	if len(got) != 0 {
		t.Errorf("RescoreTopN with nil candidates = %v, want []", got)
	}
}

// TestRescoreTopN_EmptyPattern returns all candidates (vacuous match).
func TestRescoreTopN_EmptyPattern(t *testing.T) {
	t.Parallel()
	candidates := []Candidate{
		{Text: "a.go", Payload: 1},
		{Text: "b.go", Payload: 2},
		{Text: "c.go", Payload: 3},
	}
	got := RescoreTopN("", candidates, 2)
	if len(got) != 2 {
		t.Errorf("RescoreTopN(\"\", ..., 2) = %d results, want 2", len(got))
	}
	for _, s := range got {
		if s.Score != 1.0 {
			t.Errorf("vacuous match score = %v, want 1.0", s.Score)
		}
	}
}

// TestRescoreTopN_NLessThanCandidates returns exactly n results.
func TestRescoreTopN_NLessThanCandidates(t *testing.T) {
	t.Parallel()
	candidates := []Candidate{
		{Text: "demo.go"},
		{Text: "demo_test.go"},
		{Text: "demo_utils.go"},
		{Text: "demography.go"},
		{Text: "demolition.go"},
	}
	got := RescoreTopN("demo", candidates, 3)
	if len(got) != 3 {
		t.Errorf("RescoreTopN returned %d, want 3", len(got))
	}
}

// TestRescoreTopN_NZeroReturnsAll validates that n<=0 returns all matching.
func TestRescoreTopN_NZeroReturnsAll(t *testing.T) {
	t.Parallel()
	candidates := []Candidate{
		{Text: "demo.go"},
		{Text: "demo_test.go"},
		{Text: "xyz.go"}, // no match for "demo"
	}
	got := RescoreTopN("demo", candidates, 0)
	if len(got) != 2 {
		t.Errorf("RescoreTopN with n=0 returned %d, want 2 matching candidates", len(got))
	}
}

// TestRescoreTopN_PayloadPreserved verifies payloads survive the sort.
func TestRescoreTopN_PayloadPreserved(t *testing.T) {
	t.Parallel()
	candidates := []Candidate{
		{Text: "demo.go", Payload: 42},
		{Text: "demo_test.go", Payload: 99},
	}
	got := RescoreTopN("demo", candidates, 5)
	payloads := make(map[any]bool)
	for _, s := range got {
		payloads[s.Payload] = true
	}
	if !payloads[42] || !payloads[99] {
		t.Errorf("RescoreTopN did not preserve payloads: got %v", payloads)
	}
}

// TestScore_AlreadySortedByRescoreTopN checks that the returned slice is
// actually sorted in descending score order (property test over diverse inputs).
func TestScore_AlreadySortedByRescoreTopN(t *testing.T) {
	t.Parallel()

	candidates := []Candidate{
		{Text: "alpha.go"},
		{Text: "beta_test.go"},
		{Text: "alphabet.go"},
		{Text: "al.go"},
		{Text: "gamma.go"},
		{Text: "allocate.go"},
	}
	got := RescoreTopN("al", candidates, 10)
	for i := 1; i < len(got); i++ {
		if got[i].Score > got[i-1].Score {
			scores := make([]float64, len(got))
			for j, r := range got {
				scores[j] = r.Score
			}
			t.Errorf("RescoreTopN result not sorted: got[%d].Score=%v > got[%d].Score=%v, all scores: %v",
				i, got[i].Score, i-1, got[i-1].Score, scores)
			break
		}
	}
}
