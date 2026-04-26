package app

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"findo/internal/config"
	"findo/internal/embedder"
	"findo/internal/query"
	"findo/internal/search"
	"findo/internal/store"
	"findo/internal/vectorstore"
)

// fakeCallTracker implements embedder.Embedder and records how many times
// EmbedQuery was called. EmbedQuery returns a zero vector so callers can
// proceed without errors.
type fakeCallTracker struct {
	embedQueryCalls int
}

func (f *fakeCallTracker) ModelID() string        { return "fake-tracker" }
func (f *fakeCallTracker) Dimensions() int        { return 768 }
func (f *fakeCallTracker) PausedUntil() time.Time { return time.Time{} }
func (f *fakeCallTracker) EmbedQuery(_ context.Context, _ string) ([]float32, error) {
	f.embedQueryCalls++
	return make([]float32, 768), nil
}
func (f *fakeCallTracker) EmbedBatch(_ context.Context, _ []embedder.ChunkInput) ([][]float32, error) {
	return nil, nil
}

// TestSearch_KindFilename_SkipsEmbedder verifies that when the query classifier
// returns KindFilename, the embedder is NOT called (saving a Gemini round-trip).
func TestSearch_KindFilename_SkipsEmbedder(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := store.NewStore(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	idx := vectorstore.NewDefaultIndex(logger)
	planner := search.NewPlanner(s, idx, search.DefaultPlannerConfig())
	eng := search.NewWithPlanner(s, idx, logger, planner)

	tracker := &fakeCallTracker{}
	a := &App{
		cfg:      config.DefaultConfig(),
		store:    s,
		logger:   logger,
		embedder: tracker,
		engine:   eng,
		ctx:      context.Background(),
	}

	// "demo.py" → KindFilename → no embedding call.
	_, _ = a.Search("demo.py")

	if tracker.embedQueryCalls > 0 {
		t.Errorf("EmbedQuery called %d time(s) for KindFilename query — expected 0", tracker.embedQueryCalls)
	}
}

// TestSearch_KindHybrid_CallsEmbedder verifies that a hybrid query does call
// the embedder.
func TestSearch_KindHybrid_CallsEmbedder(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := store.NewStore(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	idx := vectorstore.NewDefaultIndex(logger)
	planner := search.NewPlanner(s, idx, search.DefaultPlannerConfig())
	eng := search.NewWithPlanner(s, idx, logger, planner)

	tracker := &fakeCallTracker{}
	a := &App{
		cfg:      config.DefaultConfig(),
		store:    s,
		logger:   logger,
		embedder: tracker,
		engine:   eng,
		ctx:      context.Background(),
	}

	// "engine_startup" → KindHybrid (identifier, no extension) → embedding expected.
	_, _ = a.Search("engine_startup")

	if tracker.embedQueryCalls == 0 {
		t.Error("EmbedQuery was not called for KindHybrid query — expected a call")
	}
}

// TestBlendedToSearchResultDTO_MapsMatchKindAndHighlights verifies that
// blendedToSearchResultDTO correctly propagates MatchKind and Highlights.
func TestBlendedToSearchResultDTO_MapsMatchKindAndHighlights(t *testing.T) {
	r := search.BlendedResult{
		File: store.FileRecord{
			Path:      "/tmp/report.txt",
			FileType:  "text",
			Extension: ".txt",
			SizeBytes: 128,
		},
		Score:     0.75,
		MatchKind: "both",
		Highlights: []search.HighlightRange{
			{Start: 0, End: 6},
			{Start: 8, End: 14},
		},
	}

	dto := blendedToSearchResultDTO(r)

	if dto.MatchKind != "both" {
		t.Errorf("MatchKind = %q, want \"both\"", dto.MatchKind)
	}
	if len(dto.Highlights) != 2 {
		t.Fatalf("len(Highlights) = %d, want 2", len(dto.Highlights))
	}
	if dto.Highlights[0].Start != 0 || dto.Highlights[0].End != 6 {
		t.Errorf("Highlights[0] = %+v, want {0,6}", dto.Highlights[0])
	}
	if dto.Highlights[1].Start != 8 || dto.Highlights[1].End != 14 {
		t.Errorf("Highlights[1] = %+v, want {8,14}", dto.Highlights[1])
	}
}

// TestBlendedToSearchResultDTO_NoHighlights_EmptySlice verifies that a result
// with no highlights produces nil Highlights (no JSON field emitted).
func TestBlendedToSearchResultDTO_NoHighlights_EmptySlice(t *testing.T) {
	r := search.BlendedResult{
		File:      store.FileRecord{Path: "/tmp/doc.txt"},
		MatchKind: "content",
	}
	dto := blendedToSearchResultDTO(r)
	if len(dto.Highlights) != 0 {
		t.Errorf("expected empty Highlights, got %v", dto.Highlights)
	}
}

// TestClassifyAndEmbed_FilenameKind_NilVec verifies that classifyAndEmbed
// returns kind=KindFilename and nil vec for a filename query, without calling
// the embedder.
func TestClassifyAndEmbed_FilenameKind_NilVec(t *testing.T) {
	tracker := &fakeCallTracker{}
	a := &App{
		cfg:      config.DefaultConfig(),
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		embedder: tracker,
		ctx:      context.Background(),
	}

	kind, vec, err := a.classifyAndEmbed("demo.py")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != query.KindFilename {
		t.Errorf("kind = %v, want KindFilename", kind)
	}
	if vec != nil {
		t.Errorf("vec = %v, want nil for KindFilename", vec)
	}
	if tracker.embedQueryCalls != 0 {
		t.Errorf("EmbedQuery called %d times, want 0", tracker.embedQueryCalls)
	}
}
