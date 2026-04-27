package search

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"findo/internal/store"
)

// BenchmarkFilenameIndex_Query100k seeds an in-memory store with 100 000
// synthetic file paths and benchmarks standard query latency.
//
// Skip in -short mode so CI stays fast.
func BenchmarkFilenameIndex_Query100k(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping 100k benchmark in short mode")
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := store.NewStore(":memory:", logger)
	if err != nil {
		b.Fatalf("NewStore: %v", err)
	}
	defer s.Close()

	exts := []string{".go", ".py", ".ts", ".md", ".txt", ".json", ".yaml", ".sh"}
	const total = 100_000
	for i := 0; i < total; i++ {
		dir := i / 100
		ext := exts[i%len(exts)]
		path := fmt.Sprintf("/home/user/project/dir%d/file%d%s", dir, i, ext)
		_, err := s.UpsertFile(store.FileRecord{
			Path:        path,
			FileType:    "text",
			Extension:   ext,
			SizeBytes:   int64(i + 1),
			ModifiedAt:  time.Now(),
			IndexedAt:   time.Now(),
			ContentHash: fmt.Sprintf("hash%d", i),
		})
		if err != nil {
			b.Fatalf("UpsertFile(%q): %v", path, err)
		}
	}

	idx := NewStoreFilenameIndex(s, 100)
	ctx := context.Background()

	queries := []string{"demo", "main", "file1234", "config", "util"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		q := queries[i%len(queries)]
		hits, err := idx.Query(ctx, q, 20)
		if err != nil {
			b.Fatalf("Query(%q): %v", q, err)
		}
		_ = hits
	}
}

// BenchmarkFilenameIndex_GlobQuery100k benchmarks glob query latency against
// the same 100k corpus.
func BenchmarkFilenameIndex_GlobQuery100k(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping 100k glob benchmark in short mode")
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := store.NewStore(":memory:", logger)
	if err != nil {
		b.Fatalf("NewStore: %v", err)
	}
	defer s.Close()

	exts := []string{".go", ".py", ".ts", ".md", ".txt"}
	const total = 100_000
	for i := 0; i < total; i++ {
		dir := i / 100
		ext := exts[i%len(exts)]
		path := fmt.Sprintf("/home/user/project/dir%d/file%d%s", dir, i, ext)
		_, err := s.UpsertFile(store.FileRecord{
			Path:        path,
			FileType:    "text",
			Extension:   ext,
			SizeBytes:   int64(i + 1),
			ModifiedAt:  time.Now(),
			IndexedAt:   time.Now(),
			ContentHash: fmt.Sprintf("hash%d", i),
		})
		if err != nil {
			b.Fatalf("UpsertFile(%q): %v", path, err)
		}
	}

	idx := NewStoreFilenameIndex(s, 100)
	ctx := context.Background()

	globQueries := []string{"*.go", "*.py", "file1?.go", "dir5*/file*.ts"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		q := globQueries[i%len(globQueries)]
		hits, err := idx.Query(ctx, q, 20)
		if err != nil {
			b.Fatalf("Query(%q): %v", q, err)
		}
		_ = hits
	}
}
