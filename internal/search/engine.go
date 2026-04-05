package search

import (
	"log/slog"
	"time"

	"universal-search/internal/store"
	"universal-search/internal/vectorstore"
)

// Engine performs semantic search by combining vector similarity search
// with SQLite metadata lookups.
type Engine struct {
	store  *store.Store
	index  *vectorstore.Index
	logger *slog.Logger
}

// New creates a new search engine backed by the given store and vector index.
func New(s *store.Store, idx *vectorstore.Index, logger *slog.Logger) *Engine {
	return &Engine{store: s, index: idx, logger: logger.WithGroup("search")}
}

// SearchByVector searches the HNSW index for the nearest neighbors to
// queryVec. It over-fetches 5x candidates from the vector index to ensure
// cross-cluster diversity (text, image, video embeddings live in different
// regions), then deduplicates by file path and trims to k results.
func (e *Engine) SearchByVector(queryVec []float32, k int) ([]store.SearchResult, error) {
	start := time.Now()

	// Over-fetch to improve diversity across embedding clusters.
	fetchK := k * 5
	vecResults, err := e.index.Search(queryVec, fetchK)
	if err != nil {
		e.logger.Error("vector search failed", "error", err)
		return nil, err
	}

	if len(vecResults) == 0 {
		e.logger.Debug("no vector results")
		return nil, nil
	}

	vectorIDs := make([]string, len(vecResults))
	for i, vr := range vecResults {
		vectorIDs[i] = vr.ID
	}

	results, err := e.store.GetChunksByVectorIDs(vectorIDs)
	if err != nil {
		return nil, err
	}

	// Build a lookup from vectorID to result for fast access
	resultByVecID := make(map[string]store.SearchResult, len(results))
	for _, r := range results {
		resultByVecID[r.VectorID] = r
	}

	// Re-order by HNSW ranking and deduplicate by file path (keep best rank)
	seen := make(map[string]bool)
	var deduped []store.SearchResult
	for _, id := range vectorIDs {
		if len(deduped) >= k {
			break
		}
		r, ok := resultByVecID[id]
		if !ok {
			continue
		}
		if seen[r.File.Path] {
			continue
		}
		seen[r.File.Path] = true
		deduped = append(deduped, r)
	}

	e.logger.Info("search completed", "results", len(deduped), "candidates", len(vecResults), "duration", time.Since(start))
	return deduped, nil
}
