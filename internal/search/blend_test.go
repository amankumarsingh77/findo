package search

import (
	"testing"

	"findo/internal/query"
	"findo/internal/store"
)

// makeSemanticResult is a test helper that builds a store.SearchResult with
// a given path and final score.
func makeSemanticResult(path string, finalScore float32) store.SearchResult {
	return store.SearchResult{
		File:       store.FileRecord{Path: path},
		FinalScore: finalScore,
		Distance:   1 - finalScore,
	}
}

// makeFilenameHit is a test helper that builds a FilenameHit.
func makeFilenameHit(path string, score float64, matchKind string) FilenameHit {
	return FilenameHit{
		File:      store.FileRecord{Path: path},
		Score:     score,
		MatchKind: matchKind,
	}
}

// makeFilenameHitWithHighlights is a test helper that builds a FilenameHit with highlights.
func makeFilenameHitWithHighlights(path string, score float64, matchKind string, highlights []HighlightRange) FilenameHit {
	return FilenameHit{
		File:       store.FileRecord{Path: path},
		Score:      score,
		MatchKind:  matchKind,
		Highlights: highlights,
	}
}

func TestBlend_KindContent_passesThrough(t *testing.T) {
	semantic := []store.SearchResult{
		makeSemanticResult("/a/file1.txt", 0.9),
		makeSemanticResult("/a/file2.txt", 0.8),
		makeSemanticResult("/a/file3.txt", 0.7),
		makeSemanticResult("/a/file4.txt", 0.6),
		makeSemanticResult("/a/file5.txt", 0.5),
	}
	cfg := DefaultBlendConfig()

	got := Blend(semantic, nil, query.KindContent, cfg, 10)

	if len(got) != 5 {
		t.Fatalf("expected 5 results, got %d", len(got))
	}
	for i, r := range got {
		if r.MatchKind != "content" {
			t.Errorf("result[%d].MatchKind = %q, want %q", i, r.MatchKind, "content")
		}
		if r.File.Path != semantic[i].File.Path {
			t.Errorf("result[%d].File.Path = %q, want %q", i, r.File.Path, semantic[i].File.Path)
		}
		if len(r.Highlights) != 0 {
			t.Errorf("result[%d].Highlights should be empty, got %v", i, r.Highlights)
		}
	}
}

func TestBlend_KindContent_respectsKCap(t *testing.T) {
	semantic := []store.SearchResult{
		makeSemanticResult("/a/1.txt", 0.9),
		makeSemanticResult("/a/2.txt", 0.8),
		makeSemanticResult("/a/3.txt", 0.7),
	}
	cfg := DefaultBlendConfig()

	got := Blend(semantic, nil, query.KindContent, cfg, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 results (k=2), got %d", len(got))
	}
}

func TestBlend_KindFilename_passesThrough(t *testing.T) {
	filename := []FilenameHit{
		makeFilenameHit("/f/alpha.go", 0.9, "exact"),
		makeFilenameHit("/f/beta.go", 0.7, "substring"),
		makeFilenameHit("/f/gamma.go", 0.5, "fuzzy"),
	}
	cfg := DefaultBlendConfig()

	got := Blend(nil, filename, query.KindFilename, cfg, 10)

	if len(got) != 3 {
		t.Fatalf("expected 3 results, got %d", len(got))
	}
	for i, r := range got {
		if r.MatchKind != "filename" {
			t.Errorf("result[%d].MatchKind = %q, want %q", i, r.MatchKind, "filename")
		}
		if r.File.Path != filename[i].File.Path {
			t.Errorf("result[%d].File.Path = %q, want %q", i, r.File.Path, filename[i].File.Path)
		}
		if r.Distance != 0 {
			t.Errorf("result[%d].Distance should be 0 (filename-only), got %v", i, r.Distance)
		}
	}
}

func TestBlend_KindFilename_respectsKCap(t *testing.T) {
	filename := []FilenameHit{
		makeFilenameHit("/f/a.go", 0.9, "exact"),
		makeFilenameHit("/f/b.go", 0.7, "substring"),
		makeFilenameHit("/f/c.go", 0.5, "fuzzy"),
	}
	cfg := DefaultBlendConfig()

	got := Blend(nil, filename, query.KindFilename, cfg, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 results (k=2), got %d", len(got))
	}
}

func TestBlend_KindHybrid_disjointPaths_interleavesByRRF(t *testing.T) {
	// Semantic has 3 unique paths, filename has 3 unique paths.
	// RRF rank is 1-based inside our loops (rank+1 in the formula).
	// semantic rank 0 → 1/(60+1) ≈ 0.01639
	// filename rank 0 → 1/(60+1) ≈ 0.01639
	// Both ranks equal at position 0; semantic and filename alternate.
	semantic := []store.SearchResult{
		makeSemanticResult("/s/1.txt", 0.9),
		makeSemanticResult("/s/2.txt", 0.8),
		makeSemanticResult("/s/3.txt", 0.7),
	}
	filename := []FilenameHit{
		makeFilenameHit("/f/a.go", 0.9, "substring"),
		makeFilenameHit("/f/b.go", 0.8, "substring"),
		makeFilenameHit("/f/c.go", 0.7, "substring"),
	}
	cfg := DefaultBlendConfig()

	got := Blend(semantic, filename, query.KindHybrid, cfg, 10)

	if len(got) != 6 {
		t.Fatalf("expected 6 results (3+3 disjoint), got %d", len(got))
	}

	// Verify scores are in descending order.
	for i := 1; i < len(got); i++ {
		if got[i].Score > got[i-1].Score {
			t.Errorf("results not sorted: got[%d].Score=%v > got[%d].Score=%v",
				i, got[i].Score, i-1, got[i-1].Score)
		}
	}
}

func TestBlend_KindHybrid_overlappingPath_deduped(t *testing.T) {
	// The same path appears in both pipelines — should become one "both" result
	// with a score equal to the sum of the two RRF terms.
	sharedPath := "/shared/file.go"

	semantic := []store.SearchResult{
		makeSemanticResult(sharedPath, 0.9),
	}
	filename := []FilenameHit{
		makeFilenameHit(sharedPath, 0.85, "substring"),
	}
	cfg := DefaultBlendConfig()

	got := Blend(semantic, filename, query.KindHybrid, cfg, 10)

	if len(got) != 1 {
		t.Fatalf("expected 1 deduped result, got %d", len(got))
	}
	if got[0].MatchKind != "both" {
		t.Errorf("MatchKind = %q, want %q", got[0].MatchKind, "both")
	}
	if got[0].File.Path != sharedPath {
		t.Errorf("File.Path = %q, want %q", got[0].File.Path, sharedPath)
	}

	// Score must be sum of both RRF terms.
	k := float64(cfg.RrfK)
	expectedScore := 1.0/(k+1) + 1.0/(k+1)
	if got[0].Score < expectedScore-1e-9 || got[0].Score > expectedScore+1e-9 {
		t.Errorf("Score = %v, want ~%v", got[0].Score, expectedScore)
	}
}

func TestBlend_KindHybrid_exactBoostOutranks(t *testing.T) {
	// The exact-match filename hit should outscore a non-exact competing entry.
	exactPath := "/proj/main.go"
	otherPath := "/proj/util.go"

	semantic := []store.SearchResult{
		makeSemanticResult(otherPath, 0.95), // rank 0 in semantic
		makeSemanticResult(exactPath, 0.85), // rank 1 in semantic
	}
	filename := []FilenameHit{
		makeFilenameHit(exactPath, 1.0, "exact"),    // rank 0 in filename + bonus
		makeFilenameHit(otherPath, 0.7, "substring"), // rank 1 in filename
	}
	cfg := DefaultBlendConfig()

	got := Blend(semantic, filename, query.KindHybrid, cfg, 10)

	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	// exactPath should be first: it gets semantic rank-1 RRF + filename rank-0 RRF + ExactBonus.
	// otherPath gets semantic rank-0 RRF + filename rank-1 RRF (no bonus).
	if got[0].File.Path != exactPath {
		t.Errorf("top result = %q, want %q (exact-match boosted)", got[0].File.Path, exactPath)
	}
	if got[0].MatchKind != "both" {
		t.Errorf("top result MatchKind = %q, want %q", got[0].MatchKind, "both")
	}
}

func TestBlend_KindHybrid_EDGE9_emptyFilename_behavesLikeContent(t *testing.T) {
	// EDGE-9: empty filename slice → results identical to KindContent path.
	semantic := []store.SearchResult{
		makeSemanticResult("/a/1.txt", 0.9),
		makeSemanticResult("/a/2.txt", 0.8),
	}
	cfg := DefaultBlendConfig()

	hybridGot := Blend(semantic, nil, query.KindHybrid, cfg, 10)
	contentGot := Blend(semantic, nil, query.KindContent, cfg, 10)

	if len(hybridGot) != len(contentGot) {
		t.Fatalf("hybrid len=%d, content len=%d — should be equal", len(hybridGot), len(contentGot))
	}
	for i := range hybridGot {
		if hybridGot[i].File.Path != contentGot[i].File.Path {
			t.Errorf("result[%d]: hybrid path=%q, content path=%q",
				i, hybridGot[i].File.Path, contentGot[i].File.Path)
		}
		// MatchKind will be "content" in both cases when filename is empty.
		if hybridGot[i].MatchKind != "content" {
			t.Errorf("result[%d].MatchKind = %q, want %q", i, hybridGot[i].MatchKind, "content")
		}
	}
}

func TestBlend_KindHybrid_EDGE9_emptySemantic_behavesLikeFilename(t *testing.T) {
	// EDGE-9: empty semantic slice → results follow filename order.
	filename := []FilenameHit{
		makeFilenameHit("/f/x.go", 0.9, "exact"),
		makeFilenameHit("/f/y.go", 0.7, "substring"),
	}
	cfg := DefaultBlendConfig()

	hybridGot := Blend(nil, filename, query.KindHybrid, cfg, 10)

	if len(hybridGot) != 2 {
		t.Fatalf("expected 2 results, got %d", len(hybridGot))
	}
	for i, r := range hybridGot {
		if r.MatchKind != "filename" {
			t.Errorf("result[%d].MatchKind = %q, want %q", i, r.MatchKind, "filename")
		}
	}
}

func TestBlend_KindHybrid_EDGE10_sharedPathMatchKindBoth(t *testing.T) {
	// EDGE-10: a path shared across both pipelines must surface MatchKind="both"
	// so the UI can render stacked icons.
	sharedPath := "/shared/doc.txt"

	semantic := []store.SearchResult{makeSemanticResult(sharedPath, 0.8)}
	filename := []FilenameHit{makeFilenameHit(sharedPath, 0.75, "substring")}
	cfg := DefaultBlendConfig()

	got := Blend(semantic, filename, query.KindHybrid, cfg, 10)

	if len(got) != 1 {
		t.Fatalf("expected 1 deduped result, got %d", len(got))
	}
	if got[0].MatchKind != "both" {
		t.Errorf("MatchKind = %q, want %q (EDGE-10 stacked icon)", got[0].MatchKind, "both")
	}
}

func TestBlend_KindHybrid_highlightsFromFilenameWhenPresent(t *testing.T) {
	sharedPath := "/shared/README.md"
	hl := []HighlightRange{{Start: 0, End: 4}}

	semantic := []store.SearchResult{makeSemanticResult(sharedPath, 0.8)}
	filename := []FilenameHit{makeFilenameHitWithHighlights(sharedPath, 0.9, "substring", hl)}
	cfg := DefaultBlendConfig()

	got := Blend(semantic, filename, query.KindHybrid, cfg, 10)

	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if len(got[0].Highlights) != 1 || got[0].Highlights[0] != hl[0] {
		t.Errorf("Highlights = %v, want %v", got[0].Highlights, hl)
	}
}

func TestBlend_KindHybrid_semanticFieldsPreserved(t *testing.T) {
	// Semantic-side fields (Distance, StartTime, EndTime, VectorID, ChunkID,
	// EmbeddingModel) should be preserved in a blended "both" result.
	path := "/video/clip.mp4"

	semantic := []store.SearchResult{{
		File:           store.FileRecord{Path: path},
		Distance:       0.12,
		StartTime:      10.5,
		EndTime:        40.5,
		VectorID:       "vec-123",
		ChunkID:        42,
		EmbeddingModel: "gemini-embedding-2-preview",
		FinalScore:     0.88,
	}}
	filename := []FilenameHit{makeFilenameHit(path, 0.9, "exact")}
	cfg := DefaultBlendConfig()

	got := Blend(semantic, filename, query.KindHybrid, cfg, 10)

	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	r := got[0]
	if r.Distance != 0.12 {
		t.Errorf("Distance = %v, want 0.12", r.Distance)
	}
	if r.StartTime != 10.5 {
		t.Errorf("StartTime = %v, want 10.5", r.StartTime)
	}
	if r.EndTime != 40.5 {
		t.Errorf("EndTime = %v, want 40.5", r.EndTime)
	}
	if r.VectorID != "vec-123" {
		t.Errorf("VectorID = %q, want %q", r.VectorID, "vec-123")
	}
	if r.ChunkID != 42 {
		t.Errorf("ChunkID = %v, want 42", r.ChunkID)
	}
	if r.EmbeddingModel != "gemini-embedding-2-preview" {
		t.Errorf("EmbeddingModel = %q, want %q", r.EmbeddingModel, "gemini-embedding-2-preview")
	}
}

func TestBlend_KindHybrid_kCapRespected(t *testing.T) {
	semantic := []store.SearchResult{
		makeSemanticResult("/s/1.txt", 0.9),
		makeSemanticResult("/s/2.txt", 0.8),
		makeSemanticResult("/s/3.txt", 0.7),
	}
	filename := []FilenameHit{
		makeFilenameHit("/f/a.go", 0.9, "exact"),
		makeFilenameHit("/f/b.go", 0.8, "substring"),
		makeFilenameHit("/f/c.go", 0.7, "fuzzy"),
	}
	cfg := DefaultBlendConfig()

	got := Blend(semantic, filename, query.KindHybrid, cfg, 4)

	if len(got) != 4 {
		t.Fatalf("expected 4 results (k=4), got %d", len(got))
	}
}

func TestBlend_KindHybrid_determinism(t *testing.T) {
	// Same input must always produce the same output (stable sort).
	semantic := []store.SearchResult{
		makeSemanticResult("/s/a.txt", 0.9),
		makeSemanticResult("/s/b.txt", 0.7),
		makeSemanticResult("/s/c.txt", 0.5),
	}
	filename := []FilenameHit{
		makeFilenameHit("/f/x.go", 0.9, "exact"),
		makeFilenameHit("/s/a.txt", 0.8, "substring"), // overlaps with semantic
		makeFilenameHit("/f/z.go", 0.6, "fuzzy"),
	}
	cfg := DefaultBlendConfig()

	first := Blend(semantic, filename, query.KindHybrid, cfg, 10)
	second := Blend(semantic, filename, query.KindHybrid, cfg, 10)

	if len(first) != len(second) {
		t.Fatalf("non-deterministic length: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].File.Path != second[i].File.Path {
			t.Errorf("result[%d] differs: %q vs %q", i, first[i].File.Path, second[i].File.Path)
		}
		if first[i].Score != second[i].Score {
			t.Errorf("result[%d] score differs: %v vs %v", i, first[i].Score, second[i].Score)
		}
	}
}

func TestBlend_KindHybrid_exactBoostAddsBonus(t *testing.T) {
	// Verify exact bonus is numerically applied.
	exactPath := "/dir/exact.go"
	cfg := DefaultBlendConfig()

	semantic := []store.SearchResult{makeSemanticResult(exactPath, 0.9)}
	filenameExact := []FilenameHit{makeFilenameHit(exactPath, 1.0, "exact")}
	filenameSubstr := []FilenameHit{makeFilenameHit(exactPath, 1.0, "substring")}

	gotExact := Blend(semantic, filenameExact, query.KindHybrid, cfg, 10)
	gotSubstr := Blend(semantic, filenameSubstr, query.KindHybrid, cfg, 10)

	if len(gotExact) != 1 || len(gotSubstr) != 1 {
		t.Fatal("expected 1 result each")
	}
	diff := gotExact[0].Score - gotSubstr[0].Score
	if diff < cfg.ExactBonus-1e-9 || diff > cfg.ExactBonus+1e-9 {
		t.Errorf("exact bonus diff = %v, want ~%v", diff, cfg.ExactBonus)
	}
}

func TestDefaultBlendConfig(t *testing.T) {
	cfg := DefaultBlendConfig()
	if cfg.RrfK != 60 {
		t.Errorf("RrfK = %d, want 60", cfg.RrfK)
	}
	if cfg.ExactBonus != 0.05 {
		t.Errorf("ExactBonus = %v, want 0.05", cfg.ExactBonus)
	}
}
