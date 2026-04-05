package chunker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsTextFile(t *testing.T) {
	dir := t.TempDir()

	textPath := filepath.Join(dir, "test.txt")
	os.WriteFile(textPath, []byte("Hello world"), 0o644)
	isText, err := IsTextFile(textPath)
	if err != nil {
		t.Fatal(err)
	}
	if !isText {
		t.Error("expected text file to be detected as text")
	}

	binPath := filepath.Join(dir, "test.bin")
	os.WriteFile(binPath, []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x00}, 0o644)
	isText, err = IsTextFile(binPath)
	if err != nil {
		t.Fatal(err)
	}
	if isText {
		t.Error("expected binary file to not be detected as text")
	}
}

func TestChunkText_SmallFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.txt")
	os.WriteFile(path, []byte("Hello world"), 0o644)

	chunks, err := ChunkText(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Text != "Hello world" {
		t.Errorf("unexpected content: %q", chunks[0].Text)
	}
}

func TestChunkText_LargeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.txt")
	data := make([]byte, textChunkSize*3)
	for i := range data {
		data[i] = 'A'
	}
	os.WriteFile(path, data, 0o644)

	chunks, err := ChunkText(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if len(c.Text) > textChunkSize {
			t.Errorf("chunk %d exceeds max size: %d", c.Index, len(c.Text))
		}
	}
}

func TestChunkText_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	os.WriteFile(path, []byte{}, 0o644)

	chunks, err := ChunkText(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 0 {
		t.Fatalf("expected 0 chunks for empty file, got %d", len(chunks))
	}
}
