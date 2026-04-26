// Package search provides the semantic and filename search engine, blending
// results from multiple pipelines via Reciprocal Rank Fusion.
package search

import (
	"sort"

	"findo/internal/query"
	"findo/internal/store"
)

// HighlightRange marks a byte range [Start, End) within a filename string
// that should be highlighted in the UI.
type HighlightRange struct {
	Start int
	End   int
}

// FilenameHit is the search-package view of a filename match, decoupled from
// the store layer so Phase 6 compiles independently of Phase 2.
type FilenameHit struct {
	File       store.FileRecord
	Score      float64
	MatchKind  string           // "exact" | "substring" | "fuzzy"
	Highlights []HighlightRange
}

// BlendedResult is a single row returned by Blend, carrying provenance
// information from both the semantic and filename pipelines.
type BlendedResult struct {
	File           store.FileRecord
	Score          float64
	MatchKind      string           // "filename" | "content" | "both"
	Highlights     []HighlightRange // from filename pipeline when present
	Distance       float32          // pass-through from semantic result
	StartTime      float64
	EndTime        float64
	VectorID       string
	ChunkID        int64
	EmbeddingModel string
}

// BlendConfig holds tuning parameters for the RRF blender.
type BlendConfig struct {
	// RrfK is the rank-smoothing constant in the RRF formula (default 60).
	RrfK int
	// ExactBonus is added to the RRF score of a result when the filename
	// pipeline classified its match as "exact" (default 0.05).
	ExactBonus float64
}

// DefaultBlendConfig returns the recommended BlendConfig defaults.
func DefaultBlendConfig() BlendConfig {
	return BlendConfig{
		RrfK:       60,
		ExactBonus: 0.05,
	}
}

// rrfK returns the effective RRF constant, guarding against a zero or negative
// value supplied by callers.
func (c BlendConfig) rrfK() int {
	if c.RrfK <= 0 {
		return 60
	}
	return c.RrfK
}

// blendEntry is the accumulator used while computing merged scores.
type blendEntry struct {
	result BlendedResult
	score  float64
	inSemantic bool
	inFilename bool
}

// Blend merges semantic and filename results according to kind:
//
//   - KindContent  — semantic only; filename slice is ignored.
//   - KindFilename — filename only; semantic slice is ignored.
//   - KindHybrid   — Reciprocal Rank Fusion across both pipelines, exact-bonus
//     applied when a filename hit has MatchKind == "exact", then
//     deduplicated by File.Path and stable-sorted by score descending.
//
// k caps the number of returned results. If k <= 0, all results are returned.
func Blend(
	semantic []store.SearchResult,
	filename []FilenameHit,
	kind query.QueryKind,
	cfg BlendConfig,
	k int,
) []BlendedResult {
	switch kind {
	case query.KindContent:
		return blendContentOnly(semantic, k)
	case query.KindFilename:
		return blendFilenameOnly(filename, k)
	default: // KindHybrid (and any future kind)
		return blendHybrid(semantic, filename, cfg, k)
	}
}

// blendContentOnly passes semantic results through, capping at k.
func blendContentOnly(semantic []store.SearchResult, k int) []BlendedResult {
	out := make([]BlendedResult, 0, min(len(semantic), capOrAll(k, len(semantic))))
	for i, r := range semantic {
		if k > 0 && i >= k {
			break
		}
		out = append(out, blendedFromSemantic(r, "content", nil))
	}
	return out
}

// blendFilenameOnly passes filename hits through, capping at k.
func blendFilenameOnly(filename []FilenameHit, k int) []BlendedResult {
	out := make([]BlendedResult, 0, min(len(filename), capOrAll(k, len(filename))))
	for i, h := range filename {
		if k > 0 && i >= k {
			break
		}
		out = append(out, blendedFromFilename(h))
	}
	return out
}

// blendHybrid fuses both pipelines using Reciprocal Rank Fusion.
func blendHybrid(
	semantic []store.SearchResult,
	filename []FilenameHit,
	cfg BlendConfig,
	k int,
) []BlendedResult {
	rrfK := cfg.rrfK()

	// Accumulate per-path entries.
	entries := make(map[string]*blendEntry, len(semantic)+len(filename))

	// Process semantic pipeline (rank starts at 1).
	for rank, r := range semantic {
		rrfScore := 1.0 / float64(rrfK+rank+1)
		path := r.File.Path
		e, ok := entries[path]
		if !ok {
			br := blendedFromSemantic(r, "content", nil)
			entries[path] = &blendEntry{result: br, score: rrfScore, inSemantic: true}
		} else {
			e.score += rrfScore
			e.inSemantic = true
			// Overwrite semantic-side fields in case this entry was
			// created by the filename pass first.
			e.result.Distance = r.Distance
			e.result.StartTime = r.StartTime
			e.result.EndTime = r.EndTime
			e.result.VectorID = r.VectorID
			e.result.ChunkID = r.ChunkID
			e.result.EmbeddingModel = r.EmbeddingModel
		}
	}

	// Process filename pipeline (rank starts at 1).
	for rank, h := range filename {
		rrfScore := 1.0 / float64(rrfK+rank+1)
		if h.MatchKind == "exact" {
			rrfScore += cfg.ExactBonus
		}
		path := h.File.Path
		e, ok := entries[path]
		if !ok {
			br := blendedFromFilename(h)
			entries[path] = &blendEntry{result: br, score: rrfScore, inFilename: true}
		} else {
			e.score += rrfScore
			e.inFilename = true
			// Filename pipeline owns highlights.
			e.result.Highlights = h.Highlights
		}
	}

	// Resolve MatchKind now that we know which pipelines saw each path.
	out := make([]BlendedResult, 0, len(entries))
	for _, e := range entries {
		switch {
		case e.inSemantic && e.inFilename:
			e.result.MatchKind = "both"
		case e.inFilename:
			e.result.MatchKind = "filename"
		default:
			e.result.MatchKind = "content"
		}
		e.result.Score = e.score
		out = append(out, e.result)
	}

	// Stable sort: descending score; ties broken deterministically by path
	// (insertion order from a map is non-deterministic, so an explicit
	// tiebreaker is required for reproducible output).
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].File.Path < out[j].File.Path
	})

	if k > 0 && len(out) > k {
		out = out[:k]
	}
	return out
}

// blendedFromSemantic constructs a BlendedResult from a store.SearchResult.
func blendedFromSemantic(r store.SearchResult, matchKind string, highlights []HighlightRange) BlendedResult {
	return BlendedResult{
		File:           r.File,
		Score:          float64(r.FinalScore),
		MatchKind:      matchKind,
		Highlights:     highlights,
		Distance:       r.Distance,
		StartTime:      r.StartTime,
		EndTime:        r.EndTime,
		VectorID:       r.VectorID,
		ChunkID:        r.ChunkID,
		EmbeddingModel: r.EmbeddingModel,
	}
}

// blendedFromFilename constructs a BlendedResult from a FilenameHit.
func blendedFromFilename(h FilenameHit) BlendedResult {
	return BlendedResult{
		File:       h.File,
		Score:      h.Score,
		MatchKind:  "filename",
		Highlights: h.Highlights,
	}
}

// capOrAll returns cap if cap > 0, otherwise returns fallback.
func capOrAll(cap, fallback int) int {
	if cap > 0 {
		return cap
	}
	return fallback
}

// min returns the smaller of a and b.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
