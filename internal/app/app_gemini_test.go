package app

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"findo/internal/config"
	"findo/internal/indexer"
	"findo/internal/store"
	"findo/internal/vectorstore"
)

func newTestAppFull(t *testing.T) *App {
	t.Helper()
	s, err := store.NewStore(":memory:", slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	idx := vectorstore.NewDefaultIndex(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	p := indexer.NewPipeline(s, idx, nil, t.TempDir(), logger, nil, indexer.DefaultPipelineConfig())
	t.Cleanup(func() { p.Stop() })

	return &App{
		ctx:      context.Background(),
		cfg:      config.DefaultConfig(),
		logger:   logger,
		store:    s,
		pipeline: p,
	}
}

func TestSetGeminiAPIKey_EmptyKey(t *testing.T) {
	a := newTestAppFull(t)

	err := a.SetGeminiAPIKey("")
	if err == nil {
		t.Fatal("expected error for empty key, got nil")
	}

	err = a.SetGeminiAPIKey("   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only key, got nil")
	}
}

func TestSetGeminiAPIKey_PipelinePauseRestore(t *testing.T) {
	t.Run("pipeline_was_running_restored_to_running", func(t *testing.T) {
		a := newTestAppFull(t)

		if a.pipeline.Status().Paused {
			t.Fatal("expected pipeline to start unpaused")
		}

		_ = a.SetGeminiAPIKey("invalid-key-that-will-fail-validation")

		if a.pipeline.Status().Paused {
			t.Error("expected pipeline to be unpaused after failed SetGeminiAPIKey")
		}
	})

	t.Run("pipeline_was_paused_stays_paused", func(t *testing.T) {
		a := newTestAppFull(t)

		a.pipeline.Pause()
		if !a.pipeline.Status().Paused {
			t.Fatal("expected pipeline to be paused")
		}

		_ = a.SetGeminiAPIKey("invalid-key-that-will-fail-validation")

		if !a.pipeline.Status().Paused {
			t.Error("expected pipeline to remain paused after failed SetGeminiAPIKey")
		}
	})
}

func TestGetHasGeminiKey_NoEmbedder(t *testing.T) {
	a := newTestAppFull(t)
	if a.GetHasGeminiKey() {
		t.Error("expected GetHasGeminiKey to return false when embedder is nil")
	}
}
