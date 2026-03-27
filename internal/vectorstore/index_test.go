package vectorstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVectorIndex_AddAndSearch(t *testing.T) {
	idx := NewIndex()

	vec1 := make([]float32, 768)
	vec1[0] = 1.0
	vec2 := make([]float32, 768)
	vec2[1] = 1.0
	vec3 := make([]float32, 768)
	vec3[0] = 0.9
	vec3[1] = 0.1

	err := idx.Add("id-1", vec1)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	idx.Add("id-2", vec2)
	idx.Add("id-3", vec3)

	query := make([]float32, 768)
	query[0] = 1.0
	results, err := idx.Search(query, 2)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != "id-1" {
		t.Fatalf("expected id-1 as top result, got %s", results[0].ID)
	}
}

func TestVectorIndex_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.hnsw")

	idx := NewIndex()
	vec := make([]float32, 768)
	vec[0] = 1.0
	idx.Add("id-1", vec)

	err := idx.Save(path)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if _, err := os.Stat(path + ".graph"); os.IsNotExist(err) {
		t.Fatal("graph file not created")
	}

	idx2, err := LoadIndex(path)
	if err != nil {
		t.Fatalf("LoadIndex failed: %v", err)
	}

	query := make([]float32, 768)
	query[0] = 1.0
	results, err := idx2.Search(query, 1)
	if err != nil {
		t.Fatalf("Search on loaded index failed: %v", err)
	}
	if results[0].ID != "id-1" {
		t.Fatalf("expected id-1, got %s", results[0].ID)
	}
}

func TestVectorIndex_Delete(t *testing.T) {
	idx := NewIndex()
	vec := make([]float32, 768)
	vec[0] = 1.0
	idx.Add("id-1", vec)

	deleted := idx.Delete("id-1")
	if !deleted {
		t.Fatal("expected deletion to succeed")
	}

	deleted = idx.Delete("nonexistent")
	if deleted {
		t.Fatal("expected deletion of nonexistent to fail")
	}
}
