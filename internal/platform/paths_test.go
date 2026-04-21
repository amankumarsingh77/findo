package platform

import (
	"strings"
	"testing"
)

func TestDataDir_ReturnsNonEmptyPath(t *testing.T) {
	dir, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir failed: %v", err)
	}
	if dir == "" {
		t.Fatal("DataDir returned empty string")
	}
	if !strings.Contains(dir, "findo") {
		t.Fatalf("DataDir should contain 'findo', got: %s", dir)
	}
}

func TestDBPath_EndsWithMetadataDB(t *testing.T) {
	path, err := DBPath()
	if err != nil {
		t.Fatalf("DBPath failed: %v", err)
	}
	if !strings.HasSuffix(path, "metadata.db") {
		t.Fatalf("DBPath should end with metadata.db, got: %s", path)
	}
}

func TestIndexPath_EndsWithVectorsHnsw(t *testing.T) {
	path, err := IndexPath()
	if err != nil {
		t.Fatalf("IndexPath failed: %v", err)
	}
	if !strings.HasSuffix(path, "vectors.hnsw") {
		t.Fatalf("IndexPath should end with vectors.hnsw, got: %s", path)
	}
}
