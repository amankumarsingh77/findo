package search

import (
	"context"
	"testing"
	"time"

	"findo/internal/store"
)

// fakeIndexStore is a test double for filenameIndexStore.
type fakeIndexStore struct {
	searchResults []store.FilenameMatch
	searchErr     error
	globResults   []store.FileRecord
	globErr       error
	// capture the last glob pattern passed
	lastGlobPattern string
}

func (f *fakeIndexStore) FilenameSearch(query string, limit int) ([]store.FilenameMatch, error) {
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	results := f.searchResults
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (f *fakeIndexStore) FilenameGlob(pattern string, limit int) ([]store.FileRecord, error) {
	f.lastGlobPattern = pattern
	if f.globErr != nil {
		return nil, f.globErr
	}
	results := f.globResults
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// makeFileRecord is a helper to build a minimal FileRecord for testing.
func makeFileRecord(path string) store.FileRecord {
	basename, parent, stem := store.PathParts(path)
	return store.FileRecord{
		ID:         1,
		Path:       path,
		FileType:   "text",
		Extension:  ".py",
		SizeBytes:  100,
		ModifiedAt: time.Now(),
		IndexedAt:  time.Now(),
		Basename:   basename,
		Parent:     parent,
		Stem:       stem,
	}
}

// makeFilenameMatch builds a store.FilenameMatch with sensible defaults.
func makeFilenameMatch(path string, score float64, kind string) store.FilenameMatch {
	f := makeFileRecord(path)
	return store.FilenameMatch{
		File:      f,
		Score:     score,
		MatchKind: kind,
		Highlights: []store.HighlightRange{
			{Start: 0, End: len(f.Basename)},
		},
	}
}

func TestStoreFilenameIndex_EmptyQuery(t *testing.T) {
	idx := NewStoreFilenameIndex(&fakeIndexStore{}, 20)
	ctx := context.Background()

	for _, q := range []string{"", "   ", "\t"} {
		hits, err := idx.Query(ctx, q, 10)
		if err != nil {
			t.Errorf("Query(%q): unexpected error: %v", q, err)
		}
		if len(hits) != 0 {
			t.Errorf("Query(%q): expected 0 hits, got %d", q, len(hits))
		}
	}
}

func TestStoreFilenameIndex_StandardQuery_ReturnsHits(t *testing.T) {
	fake := &fakeIndexStore{
		searchResults: []store.FilenameMatch{
			makeFilenameMatch("/home/user/demo.py", 0.9, "exact"),
			makeFilenameMatch("/home/user/demo_utils.py", 0.7, "substring"),
			makeFilenameMatch("/home/user/unrelated.go", 0.3, "substring"),
		},
	}
	idx := NewStoreFilenameIndex(fake, 20)
	ctx := context.Background()

	hits, err := idx.Query(ctx, "demo", 10)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least one hit")
	}
	// Verify no hit exceeds the returned count from the fake.
	if len(hits) > 3 {
		t.Errorf("got %d hits, want <= 3", len(hits))
	}
}

func TestStoreFilenameIndex_StandardQuery_RespectsLimit(t *testing.T) {
	fake := &fakeIndexStore{
		searchResults: []store.FilenameMatch{
			makeFilenameMatch("/home/user/demo.py", 0.9, "exact"),
			makeFilenameMatch("/home/user/demo2.py", 0.8, "substring"),
			makeFilenameMatch("/home/user/demo3.py", 0.7, "substring"),
			makeFilenameMatch("/home/user/demo4.py", 0.6, "substring"),
		},
	}
	idx := NewStoreFilenameIndex(fake, 20)
	ctx := context.Background()

	const limit = 2
	hits, err := idx.Query(ctx, "demo", limit)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(hits) > limit {
		t.Errorf("got %d hits, want <= %d", len(hits), limit)
	}
}

func TestStoreFilenameIndex_GlobQuery_UsesGlobPath(t *testing.T) {
	fake := &fakeIndexStore{
		globResults: []store.FileRecord{
			makeFileRecord("/home/user/main.py"),
			makeFileRecord("/home/user/test.py"),
		},
	}
	idx := NewStoreFilenameIndex(fake, 20)
	ctx := context.Background()

	hits, err := idx.Query(ctx, "*.py", 10)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(hits) != 2 {
		t.Errorf("got %d hits, want 2", len(hits))
	}
	// Verify the glob path was used (not FTS).
	if fake.lastGlobPattern == "" {
		t.Error("expected FilenameGlob to be called")
	}
	if len(fake.searchResults) != 0 {
		// FTS results were not set, so this indirectly confirms the glob path.
	}
}

func TestStoreFilenameIndex_GlobQuery_DoesNotRunFuzzy(t *testing.T) {
	// Glob queries should use FilenameGlob exclusively; FilenameSearch not called.
	fake := &fakeIndexStore{
		searchResults: []store.FilenameMatch{
			// These should NOT appear in results when glob path is taken.
			makeFilenameMatch("/home/user/demo.py", 0.9, "exact"),
		},
		globResults: []store.FileRecord{
			makeFileRecord("/home/user/test.py"),
		},
	}
	idx := NewStoreFilenameIndex(fake, 20)
	ctx := context.Background()

	hits, err := idx.Query(ctx, "*.py", 10)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	// Only the glob result should be present (1 result from globResults).
	if len(hits) != 1 {
		t.Errorf("got %d hits, want 1 (glob only)", len(hits))
	}
	// Glob hits should have MatchKind "substring" and fixed score 0.8.
	if hits[0].MatchKind != "substring" {
		t.Errorf("glob hit MatchKind = %q, want \"substring\"", hits[0].MatchKind)
	}
	if hits[0].Score != 0.8 {
		t.Errorf("glob hit Score = %v, want 0.8", hits[0].Score)
	}
}

func TestStoreFilenameIndex_SpecialCharsInNonGlobQuery_NoError(t *testing.T) {
	fake := &fakeIndexStore{
		searchResults: nil,
		searchErr:     nil,
	}
	idx := NewStoreFilenameIndex(fake, 20)
	ctx := context.Background()

	// Special characters that are not glob wildcards should not cause errors.
	specialQueries := []string{
		"file[0]",
		"query with spaces",
		"hello-world",
		"path/to/file",
	}
	for _, q := range specialQueries {
		_, err := idx.Query(ctx, q, 10)
		if err != nil {
			t.Errorf("Query(%q): unexpected error: %v", q, err)
		}
	}
}

func TestStoreFilenameIndex_OrderingRespectsMaxScore(t *testing.T) {
	// Provide three matches; fuzzy should score the closest match highest.
	fake := &fakeIndexStore{
		searchResults: []store.FilenameMatch{
			// "demo.py" is an exact match; should score high.
			makeFilenameMatch("/home/user/demo.py", 1.0, "exact"),
			// "demo_helper.py" is a more distant match.
			makeFilenameMatch("/home/user/demo_helper.py", 0.5, "substring"),
			// "unrelated.py" should score low or zero.
			makeFilenameMatch("/home/user/unrelated.py", 0.2, "substring"),
		},
	}
	idx := NewStoreFilenameIndex(fake, 20)
	ctx := context.Background()

	hits, err := idx.Query(ctx, "demo", 10)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(hits) < 2 {
		t.Fatalf("expected at least 2 hits, got %d", len(hits))
	}
	// First hit should have a score >= second hit.
	if hits[0].Score < hits[1].Score {
		t.Errorf("hits not sorted: hits[0].Score=%v < hits[1].Score=%v", hits[0].Score, hits[1].Score)
	}
}

func TestStoreFilenameIndex_HighlightRangesWithinBasename(t *testing.T) {
	fake := &fakeIndexStore{
		searchResults: []store.FilenameMatch{
			makeFilenameMatch("/home/user/demo.py", 0.9, "substring"),
		},
	}
	idx := NewStoreFilenameIndex(fake, 20)
	ctx := context.Background()

	hits, err := idx.Query(ctx, "demo", 10)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least one hit")
	}
	basename := hits[0].File.Basename
	for _, hr := range hits[0].Highlights {
		if hr.Start < 0 || hr.End > len(basename) || hr.Start >= hr.End {
			t.Errorf("highlight [%d, %d) out of basename bounds (len=%d)", hr.Start, hr.End, len(basename))
		}
	}
}

func TestMatchedToHighlights(t *testing.T) {
	tests := []struct {
		name    string
		matched []int
		want    []HighlightRange
	}{
		{
			name:    "empty",
			matched: nil,
			want:    nil,
		},
		{
			name:    "single",
			matched: []int{0},
			want:    []HighlightRange{{0, 1}},
		},
		{
			name:    "consecutive",
			matched: []int{0, 1, 2},
			want:    []HighlightRange{{0, 3}},
		},
		{
			name:    "gap in middle",
			matched: []int{0, 1, 4, 5},
			want:    []HighlightRange{{0, 2}, {4, 6}},
		},
		{
			name:    "all separate",
			matched: []int{1, 3, 5},
			want:    []HighlightRange{{1, 2}, {3, 4}, {5, 6}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchedToHighlights(tt.matched)
			if len(got) != len(tt.want) {
				t.Fatalf("matchedToHighlights: got %v, want %v", got, tt.want)
			}
			for i, hr := range got {
				if hr != tt.want[i] {
					t.Errorf("range[%d]: got %v, want %v", i, hr, tt.want[i])
				}
			}
		})
	}
}

func TestClampHighlights(t *testing.T) {
	tests := []struct {
		name   string
		in     []HighlightRange
		maxLen int
		want   []HighlightRange
	}{
		{
			name:   "all within bounds",
			in:     []HighlightRange{{0, 5}},
			maxLen: 10,
			want:   []HighlightRange{{0, 5}},
		},
		{
			name:   "trim end",
			in:     []HighlightRange{{0, 15}},
			maxLen: 10,
			want:   []HighlightRange{{0, 10}},
		},
		{
			name:   "entirely outside",
			in:     []HighlightRange{{12, 15}},
			maxLen: 10,
			want:   nil,
		},
		{
			name:   "zero maxLen",
			in:     []HighlightRange{{0, 5}},
			maxLen: 0,
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampHighlights(tt.in, tt.maxLen)
			if len(got) != len(tt.want) {
				t.Fatalf("clampHighlights: got %v, want %v", got, tt.want)
			}
			for i, hr := range got {
				if hr != tt.want[i] {
					t.Errorf("range[%d]: got %v, want %v", i, hr, tt.want[i])
				}
			}
		})
	}
}

func TestStoreFilenameIndex_IsGlobPattern(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{"star", "*.py", true},
		{"question", "file?.go", true},
		{"no glob", "demo.py", false},
		{"empty", "", false},
		{"path no glob", "src/main.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGlobPattern(tt.query)
			if got != tt.want {
				t.Errorf("isGlobPattern(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}
