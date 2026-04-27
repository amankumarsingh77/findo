package app

import (
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"findo/internal/config"
	"findo/internal/indexer"
	"findo/internal/query"
	"findo/internal/search"
	"findo/internal/store"
	"findo/internal/vectorstore"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	s, err := store.NewStore(":memory:", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return &App{cfg: config.DefaultConfig(), store: s, logger: slog.Default()}
}

func TestSeedDefaultIgnorePatterns_SeedsOnEmptyDB(t *testing.T) {
	a := newTestApp(t)
	a.seedDefaultIgnorePatterns()

	patterns, err := a.GetIgnoredFolders()
	if err != nil {
		t.Fatalf("GetIgnoredFolders returned error: %v", err)
	}
	if len(patterns) != len(defaultIgnorePatterns) {
		t.Errorf("expected %d patterns, got %d", len(defaultIgnorePatterns), len(patterns))
	}
}

func TestSeedDefaultIgnorePatterns_SkipsIfPatternsExist(t *testing.T) {
	a := newTestApp(t)

	if err := a.store.AddExcludedPattern("mypattern"); err != nil {
		t.Fatal(err)
	}

	a.seedDefaultIgnorePatterns()

	patterns, err := a.GetIgnoredFolders()
	if err != nil {
		t.Fatalf("GetIgnoredFolders returned error: %v", err)
	}
	if len(patterns) != 1 {
		t.Errorf("expected 1 pattern (seeding should be skipped), got %d", len(patterns))
	}
}

func TestAddIgnoredFolder_EmptyPattern_ReturnsError(t *testing.T) {
	a := newTestApp(t)

	if err := a.AddIgnoredFolder(""); err == nil {
		t.Error("expected error for empty pattern, got nil")
	}

	if err := a.AddIgnoredFolder("  "); err == nil {
		t.Error("expected error for whitespace-only pattern, got nil")
	}
}

func TestAddIgnoredFolder_DuplicatePattern_Succeeds(t *testing.T) {
	a := newTestApp(t)

	if err := a.AddIgnoredFolder("node_modules"); err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	if err := a.AddIgnoredFolder("node_modules"); err != nil {
		t.Fatalf("second add (duplicate) failed: %v", err)
	}

	patterns, err := a.GetIgnoredFolders()
	if err != nil {
		t.Fatalf("GetIgnoredFolders returned error: %v", err)
	}
	if len(patterns) != 1 {
		t.Errorf("expected 1 pattern after duplicate add, got %d", len(patterns))
	}
}

func TestRemoveIgnoredFolder_NonExistent_ReturnsNil(t *testing.T) {
	a := newTestApp(t)

	if err := a.RemoveIgnoredFolder("nonexistent"); err != nil {
		t.Errorf("expected nil for removing nonexistent pattern, got: %v", err)
	}
}

func TestGetFilePreview_ReturnsTextContent(t *testing.T) {
	a := newTestApp(t)

	f, err := os.CreateTemp(t.TempDir(), "preview-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	content := "hello world\nthis is a test file"
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()

	got, err := a.GetFilePreview(f.Name())
	if err != nil {
		t.Fatalf("GetFilePreview returned error: %v", err)
	}
	if got != content {
		t.Fatalf("expected %q, got %q", content, got)
	}
}

func TestGetFilePreview_TruncatesAtMaxBytes(t *testing.T) {
	a := newTestApp(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "large.txt")

	data := make([]byte, 10000)
	for i := range data {
		data[i] = 'a'
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	got, err := a.GetFilePreview(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) > 8192 {
		t.Fatalf("expected at most 8192 bytes, got %d", len(got))
	}
}

func TestGetFilePreview_RejectsBinaryFile(t *testing.T) {
	a := newTestApp(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "binary.bin")

	data := []byte{'h', 'e', 'l', 'l', 'o', 0, 'w', 'o', 'r', 'l', 'd'}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := a.GetFilePreview(path)
	if err == nil {
		t.Fatal("expected error for binary file, got nil")
	}
}

func TestGetFilePreview_RejectsNonExistentFile(t *testing.T) {
	a := newTestApp(t)

	_, err := a.GetFilePreview("/nonexistent/path/file.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestGetFilePreview_RejectsEmptyFile(t *testing.T) {
	a := newTestApp(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")

	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := a.GetFilePreview(path)
	if err == nil {
		t.Fatal("expected error for empty file, got nil")
	}
}

func TestGetFilePreview_RejectsInvalidUTF8(t *testing.T) {
	a := newTestApp(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.txt")

	data := []byte{0xff, 0xfe, 'h', 'e', 'l', 'l', 'o'}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := a.GetFilePreview(path)
	if err == nil {
		t.Fatal("expected error for invalid UTF-8, got nil")
	}
}

func TestSearchResultDTO_HasScoreField(t *testing.T) {
	dto := SearchResultDTO{
		Score: 0.95,
	}
	if dto.Score != 0.95 {
		t.Fatalf("expected Score 0.95, got %v", dto.Score)
	}
}

func newTestPipeline(t *testing.T, s *store.Store) *indexer.Pipeline {
	t.Helper()
	idx := vectorstore.NewDefaultIndex(slog.Default())
	p := indexer.NewPipeline(s, idx, nil, t.TempDir(), slog.Default(), nil, indexer.DefaultPipelineConfig())
	t.Cleanup(func() { p.Stop() })
	return p
}

func TestGetHotkeyString_ReturnsNonEmpty(t *testing.T) {
	a := newTestApp(t)
	got := a.GetHotkeyString()
	if got == "" {
		t.Fatal("GetHotkeyString returned empty string")
	}
}

func TestGetHotkeyString_CustomHotkey(t *testing.T) {
	a := newTestApp(t)
	if err := a.store.SetSetting("global_hotkey", "ctrl+shift+space"); err != nil {
		t.Fatal(err)
	}
	got := a.GetHotkeyString()
	expected := "Ctrl+Shift+Space"
	if runtime.GOOS == "darwin" {
		expected = "⌃⇧Space"
	}
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestReindexFolder_NilStore(t *testing.T) {
	a := &App{store: nil, pipeline: nil, logger: slog.Default()}
	a.ReindexFolder("/some/path")
}

func TestReindexFolder_NilPipeline(t *testing.T) {
	s, err := store.NewStore(":memory:", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	a := &App{store: s, pipeline: nil, logger: slog.Default()}
	a.ReindexFolder("/some/path")
}

func TestReindexFolder_Success(t *testing.T) {
	s, err := store.NewStore(":memory:", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	for _, p := range []string{"node_modules", ".git"} {
		if err := s.AddExcludedPattern(p); err != nil {
			t.Fatal(err)
		}
	}

	p := newTestPipeline(t, s)
	a := &App{store: s, pipeline: p, logger: slog.Default()}
	a.ReindexFolder("/any/folder")
}

func TestReindexFolder_StoreError_DoesNotPanic(t *testing.T) {
	s, err := store.NewStore(":memory:", slog.Default())
	if err != nil {
		t.Fatal(err)
	}

	p := newTestPipeline(t, s)
	a := &App{store: s, pipeline: p, logger: slog.Default()}

	s.Close()
	a.ReindexFolder("/any/folder")
}

func TestReindexFolder_NonExistentPath(t *testing.T) {
	s, err := store.NewStore(":memory:", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	p := newTestPipeline(t, s)
	a := &App{store: s, pipeline: p, logger: slog.Default()}
	a.ReindexFolder("/does/not/exist/at/all")
}

func TestDetectMissingVectorBlobs_False(t *testing.T) {
	a := newTestApp(t)

	result := a.DetectMissingVectorBlobs()
	if result {
		t.Fatal("expected DetectMissingVectorBlobs to return false on empty store")
	}
}

func newTestAppWithCache(t *testing.T) *App {
	t.Helper()
	s, err := store.NewStore(":memory:", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	a := &App{
		cfg:              config.DefaultConfig(),
		store:            s,
		logger:           slog.Default(),
		parsedQueryCache: query.NewParsedQueryCache(s),
	}
	return a
}

func TestParseQuery_GrammarOnly(t *testing.T) {
	a := newTestAppWithCache(t)

	result, err := a.ParseQuery("kind:image beach")
	if err != nil {
		t.Fatalf("ParseQuery returned error: %v", err)
	}
	if result.SemanticQuery != "beach" {
		t.Errorf("expected SemanticQuery=beach, got %q", result.SemanticQuery)
	}
	if !result.HasFilters {
		t.Error("expected HasFilters=true for kind:image query")
	}
	if len(result.Chips) == 0 {
		t.Error("expected at least one chip for kind:image filter")
	}
	var found bool
	for _, c := range result.Chips {
		if c.Field == "file_type" {
			found = true
			if c.Value != "image" {
				t.Errorf("expected chip value=image, got %q", c.Value)
			}
		}
	}
	if !found {
		t.Error("expected a chip with field=file_type")
	}
}

func TestParseQuery_Disabled(t *testing.T) {
	a := newTestAppWithCache(t)
	if err := a.store.SetSetting("nl_query_enabled", "false"); err != nil {
		t.Fatal(err)
	}

	result, err := a.ParseQuery("kind:image beach")
	if err != nil {
		t.Fatalf("ParseQuery returned error: %v", err)
	}
	if len(result.Chips) != 0 {
		t.Errorf("expected no chips when disabled, got %d", len(result.Chips))
	}
	if result.HasFilters {
		t.Error("expected HasFilters=false when disabled")
	}
}

func TestSearchWithFilters_NoSpec(t *testing.T) {
	a := newTestAppWithCache(t)
	_, err := a.SearchWithFilters("", "", nil)
	_ = err // we just confirm it doesn't panic
}

func TestBuildChipDTOs_FileType(t *testing.T) {
	spec := query.FilterSpec{
		Must: []query.Clause{
			{Field: query.FieldFileType, Op: query.OpEq, Value: "image"},
		},
	}
	chips := buildChipDTOs(spec)
	if len(chips) == 0 {
		t.Fatal("expected at least one chip")
	}
	c := chips[0]
	if c.Label != "Images" {
		t.Errorf("expected label=Images, got %q", c.Label)
	}
	if c.Field != "file_type" {
		t.Errorf("expected field=file_type, got %q", c.Field)
	}
	if c.Op != "eq" {
		t.Errorf("expected op=eq, got %q", c.Op)
	}
	if c.Value != "image" {
		t.Errorf("expected value=image, got %q", c.Value)
	}
	if c.ClauseKey != "file_type|eq|image" {
		t.Errorf("expected clauseKey=file_type|eq|image, got %q", c.ClauseKey)
	}
}

func TestBuildChipDTOs_ModifiedAt(t *testing.T) {
	date := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	spec := query.FilterSpec{
		Must: []query.Clause{
			{Field: query.FieldModifiedAt, Op: query.OpGte, Value: date},
		},
	}
	chips := buildChipDTOs(spec)
	if len(chips) == 0 {
		t.Fatal("expected at least one chip")
	}
	if !strings.HasPrefix(chips[0].Label, "Since") {
		t.Errorf("expected label starting with Since, got %q", chips[0].Label)
	}
}

func TestBuildChipDTOs_Extension(t *testing.T) {
	spec := query.FilterSpec{
		Must: []query.Clause{
			{Field: query.FieldExtension, Op: query.OpInSet, Value: []string{".py", ".go"}},
		},
	}
	chips := buildChipDTOs(spec)
	if len(chips) == 0 {
		t.Fatal("expected at least one chip")
	}
	if !strings.Contains(chips[0].Label, ".py") || !strings.Contains(chips[0].Label, ".go") {
		t.Errorf("expected label to contain .py and .go, got %q", chips[0].Label)
	}
}

func TestBuildChipDTOs_SizeBytes(t *testing.T) {
	spec := query.FilterSpec{
		Must: []query.Clause{
			{Field: query.FieldSizeBytes, Op: query.OpGt, Value: int64(10485760)},
		},
	}
	chips := buildChipDTOs(spec)
	if len(chips) == 0 {
		t.Fatal("expected at least one chip")
	}
	if !strings.Contains(chips[0].Label, "10 MB") {
		t.Errorf("expected label to contain '10 MB', got %q", chips[0].Label)
	}
}

func TestBuildChipDTOs_MustNotPrefixesNot(t *testing.T) {
	spec := query.FilterSpec{
		MustNot: []query.Clause{
			{Field: query.FieldFileType, Op: query.OpEq, Value: "video"},
		},
	}
	chips := buildChipDTOs(spec)
	if len(chips) == 0 {
		t.Fatal("expected at least one chip")
	}
	if !strings.HasPrefix(chips[0].Label, "Not ") {
		t.Errorf("expected label to start with 'Not ', got %q", chips[0].Label)
	}
}

func TestParseQuery_CacheHit(t *testing.T) {
	a := newTestAppWithCache(t)

	spec := query.FilterSpec{
		SemanticQuery: "beach",
		Must: []query.Clause{
			{Field: query.FieldFileType, Op: query.OpEq, Value: "image"},
		},
		Source: query.SourceGrammar,
	}
	if err := a.parsedQueryCache.Set("kind:image beach", spec); err != nil {
		t.Fatal(err)
	}

	result, err := a.ParseQuery("kind:image beach")
	if err != nil {
		t.Fatalf("ParseQuery returned error: %v", err)
	}
	if !result.CacheHit {
		t.Error("expected CacheHit=true on second call")
	}
	if result.SemanticQuery != "beach" {
		t.Errorf("expected SemanticQuery=beach from cache, got %q", result.SemanticQuery)
	}
}

func TestOfflineMode_SkipsLLM(t *testing.T) {
	a := newTestAppWithCache(t)
	result, err := a.ParseQuery("kind:image beach")
	if err != nil {
		t.Fatalf("ParseQuery returned error in offline mode: %v", err)
	}
	if !result.IsOffline {
		t.Error("expected IsOffline=true when embedder is nil")
	}
	if len(result.Chips) == 0 {
		t.Error("expected chips from grammar parse even in offline mode")
	}
}

func TestOfflineMode_SearchReturnsFilenameResults(t *testing.T) {
	a := newTestAppWithCache(t)
	_, err := a.store.UpsertFile(store.FileRecord{
		Path:       "/home/user/documents/report.pdf",
		FileType:   "document",
		Extension:  ".pdf",
		SizeBytes:  1024,
		ModifiedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := a.SearchWithFilters("report", "", nil)
	if err != nil {
		t.Fatalf("SearchWithFilters returned error in offline mode: %v", err)
	}
	found := false
	for _, r := range result.Results {
		if strings.Contains(r.FilePath, "report") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected filename-based result for 'report', got %+v", result.Results)
	}
}

func TestNLQueryEnabled_DefaultTrue(t *testing.T) {
	a := newTestAppWithCache(t)
	if !a.GetNLQueryEnabled() {
		t.Error("expected GetNLQueryEnabled()=true when no setting stored")
	}
}

func TestNLQueryEnabled_CanBeDisabled(t *testing.T) {
	a := newTestAppWithCache(t)
	if err := a.SetNLQueryEnabled(false); err != nil {
		t.Fatalf("SetNLQueryEnabled(false) returned error: %v", err)
	}
	if a.GetNLQueryEnabled() {
		t.Error("expected GetNLQueryEnabled()=false after SetNLQueryEnabled(false)")
	}
	result, err := a.ParseQuery("kind:image beach")
	if err != nil {
		t.Fatalf("ParseQuery returned error: %v", err)
	}
	if len(result.Chips) != 0 {
		t.Errorf("expected no chips when NL query disabled, got %d", len(result.Chips))
	}
}

func TestGetNLQueryEnabled_CanBeEnabled(t *testing.T) {
	a := newTestAppWithCache(t)
	if err := a.SetNLQueryEnabled(false); err != nil {
		t.Fatal(err)
	}
	if err := a.SetNLQueryEnabled(true); err != nil {
		t.Fatalf("SetNLQueryEnabled(true) returned error: %v", err)
	}
	if !a.GetNLQueryEnabled() {
		t.Error("expected GetNLQueryEnabled()=true after SetNLQueryEnabled(true)")
	}
}

func TestGetDebugStats_ReturnsMap(t *testing.T) {
	a := newTestApp(t)
	a.queryStats = &QueryStats{}

	stats := a.GetDebugStats()
	if stats == nil {
		t.Fatal("expected non-nil map from GetDebugStats")
	}
	for _, key := range []string{"llm_call_count", "llm_avg_ms", "cache_hits", "cache_misses"} {
		if _, ok := stats[key]; !ok {
			t.Errorf("expected key %q in GetDebugStats result", key)
		}
	}
}

func TestGetDebugStats_ZeroInitial(t *testing.T) {
	a := newTestApp(t)
	a.queryStats = &QueryStats{}

	stats := a.GetDebugStats()
	for _, key := range []string{"llm_call_count", "llm_avg_ms", "cache_hits", "cache_misses"} {
		val, ok := stats[key]
		if !ok {
			t.Fatalf("missing key %q", key)
		}
		if val.(int64) != 0 {
			t.Errorf("expected %q=0 initially, got %v", key, val)
		}
	}
}

func TestBruteForceThreshold_DefaultWhenMissing(t *testing.T) {
	a := newTestApp(t)

	got := a.getBruteForceThreshold()
	if got != search.DefaultBruteForceThreshold {
		t.Errorf("expected %d, got %d", search.DefaultBruteForceThreshold, got)
	}
}

func TestBruteForceThreshold_FromSettings(t *testing.T) {
	a := newTestApp(t)
	if err := a.store.SetSetting("brute_force_threshold", "100"); err != nil {
		t.Fatal(err)
	}

	got := a.getBruteForceThreshold()
	if got != 100 {
		t.Errorf("expected 100, got %d", got)
	}
}

func TestNeedsReindex_FalseWhenEmpty(t *testing.T) {
	a := newTestApp(t)

	if a.NeedsReindex() {
		t.Fatal("expected NeedsReindex()=false on empty store")
	}
}
