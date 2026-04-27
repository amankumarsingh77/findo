package embedder

import (
	"context"
	"time"
)

// Stats is a snapshot of embedder activity, surfaced to the UI.
type Stats struct {
	// RequestsToday is the number of successful embedding API requests issued
	// since local-midnight rollover. Resets on app restart.
	RequestsToday int
	// CurrentRPM is the number of tokens currently consumed in the rate
	// limiter's sliding window.
	CurrentRPM int
	// MaxRPM is the configured per-minute ceiling.
	MaxRPM int
	// LastEmbedAt is the wall-clock time of the most recent successful embed.
	// Zero value means no embeds have happened yet.
	LastEmbedAt time.Time
}

// Embedder is the narrow interface used by indexing and search code to produce
// vector embeddings. Concrete implementations include GeminiEmbedder (live
// API) and FakeEmbedder (deterministic test double).
type Embedder interface {
	ModelID() string
	Dimensions() int
	EmbedBatch(ctx context.Context, inputs []ChunkInput) ([][]float32, error)
	EmbedQuery(ctx context.Context, text string) ([]float32, error)
	// PausedUntil returns the time until which the embedder's rate limiter is
	// paused. Returns the zero time if no pause is active.
	PausedUntil() time.Time
	// Stats returns a snapshot of embedder activity for UI surfacing.
	Stats() Stats
}
