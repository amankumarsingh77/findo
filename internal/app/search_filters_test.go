package app

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"findo/internal/apperr"
	"findo/internal/config"
	"findo/internal/embedder"
	"findo/internal/store"
)

type errorEmbedder struct {
	queryErr    error
	pausedUntil time.Time
}

func (e *errorEmbedder) ModelID() string        { return "fake" }
func (e *errorEmbedder) Dimensions() int        { return 768 }
func (e *errorEmbedder) PausedUntil() time.Time { return e.pausedUntil }
func (e *errorEmbedder) Stats() embedder.Stats  { return embedder.Stats{} }
func (e *errorEmbedder) EmbedQuery(_ context.Context, _ string) ([]float32, error) {
	return nil, e.queryErr
}
func (e *errorEmbedder) EmbedBatch(_ context.Context, _ []embedder.ChunkInput) ([][]float32, error) {
	return nil, nil
}

func newSearchFiltersTestApp(t *testing.T) *App {
	t.Helper()
	s, err := store.NewStore(":memory:", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return &App{
		cfg:    config.DefaultConfig(),
		ctx:    context.Background(),
		logger: slog.Default(),
		store:  s,
	}
}

func TestSearchWithFilters_EmbedRateLimited_PopulatesErrorCode(t *testing.T) {
	a := newSearchFiltersTestApp(t)

	_, err := a.store.UpsertFile(store.FileRecord{
		Path:       "/home/user/budget.pdf",
		FileType:   "document",
		Extension:  ".pdf",
		SizeBytes:  1024,
		ModifiedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	rateLimitErr := apperr.Wrap(apperr.ErrRateLimited.Code, "rate limited by provider", nil)
	a.embedder = &errorEmbedder{queryErr: rateLimitErr}

	result, err := a.SearchWithFilters("budget", "budget", nil)
	if err != nil {
		t.Fatalf("SearchWithFilters returned unexpected error: %v", err)
	}
	if result.ErrorCode != apperr.ErrRateLimited.Code {
		t.Errorf("expected ErrorCode=%q, got %q", apperr.ErrRateLimited.Code, result.ErrorCode)
	}
	if len(result.Results) != 0 {
		t.Errorf("expected empty Results on rate-limit, got %d results", len(result.Results))
	}
}

func TestSearchWithFilters_EmbedGenericError_PopulatesErrorCode(t *testing.T) {
	a := newSearchFiltersTestApp(t)

	_, err := a.store.UpsertFile(store.FileRecord{
		Path:       "/home/user/crash_report.txt",
		FileType:   "document",
		Extension:  ".txt",
		SizeBytes:  512,
		ModifiedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	a.embedder = &errorEmbedder{queryErr: fmt.Errorf("boom")}

	result, err := a.SearchWithFilters("crash report", "crash report", nil)
	if err != nil {
		t.Fatalf("SearchWithFilters returned unexpected error: %v", err)
	}
	if result.ErrorCode != apperr.ErrEmbedFailed.Code {
		t.Errorf("expected ErrorCode=%q, got %q", apperr.ErrEmbedFailed.Code, result.ErrorCode)
	}
	if len(result.Results) != 0 {
		t.Errorf("expected empty Results on generic embed error, got %d results", len(result.Results))
	}
}

func TestSearchWithFilters_EmbedRateLimited_PopulatesRetryAfterMs(t *testing.T) {
	a := newSearchFiltersTestApp(t)

	rateLimitErr := apperr.Wrap(apperr.ErrRateLimited.Code, "rate limited by provider", nil)
	pauseUntil := time.Now().Add(5 * time.Second)
	a.embedder = &errorEmbedder{queryErr: rateLimitErr, pausedUntil: pauseUntil}

	result, err := a.SearchWithFilters("query", "query", nil)
	if err != nil {
		t.Fatalf("SearchWithFilters returned unexpected error: %v", err)
	}
	if result.ErrorCode != apperr.ErrRateLimited.Code {
		t.Errorf("expected ErrorCode=%q, got %q", apperr.ErrRateLimited.Code, result.ErrorCode)
	}
	if result.RetryAfterMs < 4000 || result.RetryAfterMs > 5000 {
		t.Errorf("expected RetryAfterMs in [4000,5000], got %d", result.RetryAfterMs)
	}
}

func TestSearchWithFilters_EmbedRateLimited_ZeroPausedUntil_RetryAfterMsIsZero(t *testing.T) {
	a := newSearchFiltersTestApp(t)

	rateLimitErr := apperr.Wrap(apperr.ErrRateLimited.Code, "rate limited by provider", nil)
	a.embedder = &errorEmbedder{queryErr: rateLimitErr} // pausedUntil zero

	result, err := a.SearchWithFilters("query", "query", nil)
	if err != nil {
		t.Fatalf("SearchWithFilters returned unexpected error: %v", err)
	}
	if result.ErrorCode != apperr.ErrRateLimited.Code {
		t.Errorf("expected ErrorCode=%q, got %q", apperr.ErrRateLimited.Code, result.ErrorCode)
	}
	if result.RetryAfterMs != 0 {
		t.Errorf("expected RetryAfterMs=0 when PausedUntil is zero, got %d", result.RetryAfterMs)
	}
}

func TestSearchWithFilters_OfflineFilenameOnly_MatchKind(t *testing.T) {
	a := newSearchFiltersTestApp(t)

	_, err := a.store.UpsertFile(store.FileRecord{
		Path:       "/home/user/budget.txt",
		FileType:   "document",
		Extension:  ".txt",
		SizeBytes:  256,
		ModifiedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := a.SearchWithFilters("budget", "", nil)
	if err != nil {
		t.Fatalf("SearchWithFilters returned unexpected error: %v", err)
	}
	if len(result.Results) == 0 {
		t.Fatal("expected at least one result from offline filename search")
	}
	for _, dto := range result.Results {
		if dto.MatchKind != "filename" {
			t.Errorf("offline path: MatchKind = %q, want %q", dto.MatchKind, "filename")
		}
	}
}

func TestSearchWithFilters_OfflineUnchanged(t *testing.T) {
	a := newSearchFiltersTestApp(t)

	_, err := a.store.UpsertFile(store.FileRecord{
		Path:       "/home/user/notes.txt",
		FileType:   "document",
		Extension:  ".txt",
		SizeBytes:  256,
		ModifiedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := a.SearchWithFilters("notes", "", nil)
	if err != nil {
		t.Fatalf("SearchWithFilters returned unexpected error in offline mode: %v", err)
	}
	if result.ErrorCode != "" {
		t.Errorf("expected empty ErrorCode in offline mode, got %q", result.ErrorCode)
	}
	found := false
	for _, r := range result.Results {
		if r.FileName == "notes.txt" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected filename-fallback to find notes.txt, got results: %+v", result.Results)
	}
}
