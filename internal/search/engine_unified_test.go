package search

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"findo/internal/query"
	"findo/internal/store"
	"findo/internal/vectorstore"
)

type stubFilenameIndex struct {
	hits   []FilenameHit
	called bool
	err    error
}

func (s *stubFilenameIndex) Query(_ context.Context, _ string, _ int) ([]FilenameHit, error) {
	s.called = true
	return s.hits, s.err
}

func makeUnifiedEngine(t *testing.T, fidx FilenameIndex) (*Engine, *store.Store, *vectorstore.Index) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := store.NewStore(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	idx := vectorstore.NewDefaultIndex(logger)

	fileID, err := db.UpsertFile(store.FileRecord{
		Path: "/tmp/engine_unified.txt", FileType: "text", Extension: ".txt", SizeBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.InsertChunk(store.ChunkRecord{FileID: fileID, VectorID: "vu-1", ChunkIndex: 0}); err != nil {
		t.Fatal(err)
	}
	vec := make([]float32, 768)
	vec[0] = 1.0
	if err := idx.Add("vu-1", vec); err != nil {
		t.Fatal(err)
	}

	planner := NewPlanner(db, idx, DefaultPlannerConfig())
	eng := NewWithPlanner(db, idx, logger, planner)
	eng.filenameIdx = fidx
	eng.blendCfg = DefaultBlendConfig()
	return eng, db, idx
}

// TestSearchUnified_KindContent_SkipsFilenameIndex verifies that a pure content
// query does not call the filename index.
func TestSearchUnified_KindContent_SkipsFilenameIndex(t *testing.T) {
	stub := &stubFilenameIndex{hits: []FilenameHit{
		{File: store.FileRecord{Path: "/tmp/stub.txt"}, Score: 1.0, MatchKind: "exact"},
	}}
	eng, _, _ := makeUnifiedEngine(t, stub)

	raw := "find the document about finance"
	queryVec := make([]float32, 768)
	queryVec[0] = 1.0

	res, err := eng.SearchUnified(context.Background(), raw, queryVec, query.FilterSpec{}, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.called {
		t.Error("filenameIdx.Query was called for KindContent query — should have been skipped")
	}
	if res.Kind != query.KindContent {
		t.Errorf("expected KindContent, got %v", res.Kind)
	}
	for _, r := range res.Results {
		if r.MatchKind != "content" {
			t.Errorf("result MatchKind = %q, want \"content\"", r.MatchKind)
		}
	}
}

// TestSearchUnified_KindFilename_SkipsPlanner verifies that a pure filename
// query does not run the semantic planner (queryVec may be nil).
func TestSearchUnified_KindFilename_SkipsSemanticPipeline(t *testing.T) {
	stub := &stubFilenameIndex{hits: []FilenameHit{
		{File: store.FileRecord{Path: "/tmp/demo.py", Basename: "demo.py"}, Score: 0.9, MatchKind: "exact"},
	}}
	eng, _, _ := makeUnifiedEngine(t, stub)

	raw := "demo.py"

	res, err := eng.SearchUnified(context.Background(), raw, nil, query.FilterSpec{}, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !stub.called {
		t.Error("filenameIdx.Query was not called for KindFilename query")
	}
	if res.Kind != query.KindFilename {
		t.Errorf("expected KindFilename, got %v", res.Kind)
	}
	if len(res.Results) == 0 {
		t.Fatal("expected at least 1 blended result")
	}
	if res.Results[0].File.Path != "/tmp/demo.py" {
		t.Errorf("expected /tmp/demo.py, got %s", res.Results[0].File.Path)
	}
	if res.Results[0].MatchKind != "filename" {
		t.Errorf("MatchKind = %q, want \"filename\"", res.Results[0].MatchKind)
	}
}

// TestSearchUnified_KindHybrid_CallsBoth verifies that a hybrid query calls
// both the semantic planner and the filename index.
func TestSearchUnified_KindHybrid_CallsBoth(t *testing.T) {
	stub := &stubFilenameIndex{hits: []FilenameHit{
		{File: store.FileRecord{Path: "/tmp/engine_unified.txt", Basename: "engine_unified.txt"}, Score: 0.85, MatchKind: "substring"},
	}}
	eng, _, _ := makeUnifiedEngine(t, stub)

	raw := "engine_unified"
	queryVec := make([]float32, 768)
	queryVec[0] = 1.0

	res, err := eng.SearchUnified(context.Background(), raw, queryVec, query.FilterSpec{}, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !stub.called {
		t.Error("filenameIdx.Query was not called for KindHybrid query")
	}
	if res.Kind != query.KindHybrid {
		t.Errorf("expected KindHybrid, got %v", res.Kind)
	}
	if len(res.Results) == 0 {
		t.Fatal("expected blended results")
	}
	if res.Results[0].MatchKind != "both" {
		t.Errorf("MatchKind = %q, want \"both\"", res.Results[0].MatchKind)
	}
}

// TestSearchUnified_NilFilenameIdx_ForcesContent verifies that when the engine
// has no filename index, any query is routed as KindContent.
func TestSearchUnified_NilFilenameIdx_ForcesContent(t *testing.T) {
	eng, _, _ := makeUnifiedEngine(t, nil) // no filenameIdx
	eng.filenameIdx = nil

	raw := "demo.py"
	queryVec := make([]float32, 768)
	queryVec[0] = 1.0

	res, err := eng.SearchUnified(context.Background(), raw, queryVec, query.FilterSpec{}, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Kind != query.KindContent {
		t.Errorf("expected KindContent when filenameIdx is nil, got %v", res.Kind)
	}
}
