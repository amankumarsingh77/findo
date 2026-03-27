package indexer

import (
	"testing"
)

func TestBuildChunkArgs(t *testing.T) {
	args := buildChunkArgs("/input.mp4", "/out/chunk_0.mp4", 0, 30)
	expected := []string{
		"-ss", "0", "-i", "/input.mp4", "-t", "30",
		"-c", "copy", "-y", "/out/chunk_0.mp4",
	}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("arg[%d] = %q, want %q", i, a, expected[i])
		}
	}
}

func TestCalculateChunks(t *testing.T) {
	chunks := calculateChunks(65.0, 30, 5)
	// 0-30, 25-55, 50-65
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if chunks[0].Start != 0 || chunks[0].End != 30 {
		t.Errorf("chunk 0: got %v", chunks[0])
	}
	if chunks[1].Start != 25 || chunks[1].End != 55 {
		t.Errorf("chunk 1: got %v", chunks[1])
	}
	if chunks[2].Start != 50 || chunks[2].End != 65 {
		t.Errorf("chunk 2: got %v", chunks[2])
	}
}

func TestCalculateChunks_ShortVideo(t *testing.T) {
	chunks := calculateChunks(10.0, 30, 5)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Start != 0 || chunks[0].End != 10 {
		t.Errorf("chunk 0: got %v", chunks[0])
	}
}

func TestCalculateChunks_ExactFit(t *testing.T) {
	chunks := calculateChunks(30.0, 30, 5)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Start != 0 || chunks[0].End != 30 {
		t.Errorf("chunk 0: got %v", chunks[0])
	}
}
