package indexer

import (
	"os"
	"testing"
)

func TestHashFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.txt"
	os.WriteFile(path, []byte("hello world"), 0o644)

	hash1, err := hashFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if hash1 == "" {
		t.Fatal("hash should not be empty")
	}

	// Same content = same hash
	hash2, err := hashFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if hash1 != hash2 {
		t.Fatal("same content should produce same hash")
	}

	// Different content = different hash
	os.WriteFile(path, []byte("different content"), 0o644)
	hash3, err := hashFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if hash1 == hash3 {
		t.Fatal("different content should produce different hash")
	}
}

func TestHashFile_NonexistentFile(t *testing.T) {
	_, err := hashFile("/nonexistent/file.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}
