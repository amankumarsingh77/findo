// Package search provides the semantic and filename search engine, blending
// results from multiple pipelines via Reciprocal Rank Fusion.
package search

import (
	"context"
	"strings"

	"findo/internal/search/fuzzy"
	"findo/internal/store"
)

// FilenameIndex queries the persistent filename index and returns blendable hits.
type FilenameIndex interface {
	// Query returns up to limit FilenameHit results for query q.
	// An empty q (after trimming) returns an empty slice and nil error.
	Query(ctx context.Context, q string, limit int) ([]FilenameHit, error)
}

// filenameIndexStore is the subset of store.Store required by StoreFilenameIndex.
type filenameIndexStore interface {
	FilenameSearch(query string, limit int) ([]store.FilenameMatch, error)
	FilenameGlob(pattern string, limit int) ([]store.FileRecord, error)
}

// StoreFilenameIndex is the production implementation of FilenameIndex, backed
// by a store.Store (or any equivalent interface for testing).
type StoreFilenameIndex struct {
	store     filenameIndexStore
	fuzzyTopN int
}

// NewStoreFilenameIndex constructs a StoreFilenameIndex.
//
// fuzzyTopN controls how many FTS hits are retrieved before fuzzy rescoring.
// A value of 0 or negative is replaced with the default of 50.
func NewStoreFilenameIndex(s filenameIndexStore, fuzzyTopN int) *StoreFilenameIndex {
	if fuzzyTopN <= 0 {
		fuzzyTopN = 50
	}
	return &StoreFilenameIndex{store: s, fuzzyTopN: fuzzyTopN}
}

// Query returns up to limit filename hits for query q.
//
// Behaviour:
//   - Empty q (after trimming) → empty slice, nil error.
//   - Glob query (contains '*' or '?') → LIKE-based path scan; no fuzzy scoring.
//   - Standard query → FTS5 BM25 search followed by fuzzy rescoring; final score
//     is max(ftsScore, fuzzyScore).
func (idx *StoreFilenameIndex) Query(ctx context.Context, q string, limit int) ([]FilenameHit, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}

	if isGlobPattern(q) {
		return idx.queryGlob(q, limit)
	}
	return idx.queryFTS(q, limit)
}

// queryGlob handles queries that contain '*' or '?'.
// It translates the glob to a SQL LIKE pattern and returns hits with a fixed
// score of 0.8 and MatchKind "substring".
func (idx *StoreFilenameIndex) queryGlob(pattern string, limit int) ([]FilenameHit, error) {
	files, err := idx.store.FilenameGlob(pattern, limit)
	if err != nil {
		return nil, err
	}

	hits := make([]FilenameHit, 0, len(files))
	for _, f := range files {
		hits = append(hits, FilenameHit{
			File:       f,
			Score:      0.8,
			MatchKind:  "substring",
			Highlights: nil,
		})
	}
	return hits, nil
}

// queryFTS handles standard (non-glob) queries using FTS5 BM25 search followed
// by fuzzy rescoring.
func (idx *StoreFilenameIndex) queryFTS(q string, limit int) ([]FilenameHit, error) {
	topN := idx.fuzzyTopN
	if topN < limit {
		topN = limit
	}

	matches, err := idx.store.FilenameSearch(q, topN)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, nil
	}

	candidates := make([]fuzzy.Candidate, len(matches))
	for i := range matches {
		candidates[i] = fuzzy.Candidate{
			Text:    matches[i].File.Basename,
			Payload: &matches[i],
		}
	}

	scored := fuzzy.RescoreTopN(q, candidates, limit)

	hits := make([]FilenameHit, 0, len(scored))
	for _, sc := range scored {
		m := sc.Payload.(*store.FilenameMatch)

		// Final score is the maximum of the FTS BM25 score and the fuzzy score.
		finalScore := m.Score
		if sc.Score > finalScore {
			finalScore = sc.Score
		}

		matchKind := m.MatchKind
		if m.MatchKind != "exact" && sc.Score > m.Score {
			matchKind = "fuzzy"
		}

		// Prefer fuzzy byte offsets (sc.Matched) when available; fall back to
		// the FTS highlights from the store.
		highlights := storeHighlightsToSearch(m.Highlights)
		if len(sc.Matched) > 0 && matchKind != "exact" {
			highlights = matchedToHighlights(sc.Matched)
		}

		basenameLen := len(m.File.Basename)
		highlights = clampHighlights(highlights, basenameLen)

		hits = append(hits, FilenameHit{
			File:       m.File,
			Score:      finalScore,
			MatchKind:  matchKind,
			Highlights: highlights,
		})
	}
	return hits, nil
}

// isGlobPattern reports whether q contains any glob wildcard characters.
func isGlobPattern(q string) bool {
	return strings.ContainsAny(q, "*?")
}

// storeHighlightsToSearch converts []store.HighlightRange to []HighlightRange.
func storeHighlightsToSearch(in []store.HighlightRange) []HighlightRange {
	if len(in) == 0 {
		return nil
	}
	out := make([]HighlightRange, len(in))
	for i, h := range in {
		out[i] = HighlightRange{Start: h.Start, End: h.End}
	}
	return out
}

// matchedToHighlights converts a flat slice of byte offsets (as produced by
// fuzzy.Score) into a minimal set of HighlightRange values by merging
// consecutive byte positions into contiguous ranges.
func matchedToHighlights(matched []int) []HighlightRange {
	if len(matched) == 0 {
		return nil
	}
	var ranges []HighlightRange
	start := matched[0]
	end := matched[0] + 1

	for _, off := range matched[1:] {
		if off == end {
			// Extend the current range by one byte.
			end++
		} else {
			ranges = append(ranges, HighlightRange{Start: start, End: end})
			start = off
			end = off + 1
		}
	}
	ranges = append(ranges, HighlightRange{Start: start, End: end})
	return ranges
}

// clampHighlights removes or trims any HighlightRange that falls outside [0, maxLen).
func clampHighlights(in []HighlightRange, maxLen int) []HighlightRange {
	if maxLen <= 0 {
		return nil
	}
	out := make([]HighlightRange, 0, len(in))
	for _, h := range in {
		if h.Start >= maxLen {
			continue
		}
		end := h.End
		if end > maxLen {
			end = maxLen
		}
		if end <= h.Start {
			continue
		}
		out = append(out, HighlightRange{Start: h.Start, End: end})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
