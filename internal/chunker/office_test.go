package chunker

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createTestDocx(t *testing.T, dir string, text string) string {
	t.Helper()
	path := filepath.Join(dir, "test.docx")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)

	doc, err := w.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>` + text + `</w:t></w:r></w:p>
  </w:body>
</w:document>`
	doc.Write([]byte(xml))
	w.Close()
	f.Close()
	return path
}

func createTestPptx(t *testing.T, dir string, slides []string) string {
	t.Helper()
	path := filepath.Join(dir, "test.pptx")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)

	for i, text := range slides {
		name := "ppt/slides/slide" + string(rune('1'+i)) + ".xml"
		slide, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		xml := `<?xml version="1.0" encoding="UTF-8"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree><p:sp><p:txBody>
    <a:p><a:r><a:t>` + text + `</a:t></a:r></a:p>
  </p:txBody></p:sp></p:spTree></p:cSld>
</p:sld>`
		slide.Write([]byte(xml))
	}
	w.Close()
	f.Close()
	return path
}

func TestExtractDocxText(t *testing.T) {
	dir := t.TempDir()
	path := createTestDocx(t, dir, "Hello from Word document")

	text, err := ExtractDocxText(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "Hello from Word document") {
		t.Errorf("expected docx text, got: %q", text)
	}
}

func TestExtractPptxText(t *testing.T) {
	dir := t.TempDir()
	path := createTestPptx(t, dir, []string{"Slide One Title", "Slide Two Content"})

	text, err := ExtractPptxText(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "Slide One Title") {
		t.Errorf("missing slide 1 text, got: %q", text)
	}
	if !strings.Contains(text, "Slide Two Content") {
		t.Errorf("missing slide 2 text, got: %q", text)
	}
}

func TestExtractDocxText_InvalidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.docx")
	os.WriteFile(path, []byte("not a zip"), 0o644)

	_, err := ExtractDocxText(path)
	if err == nil {
		t.Error("expected error for invalid docx")
	}
}

func TestChunkDocument_ModernOfficeWithoutLibreOffice(t *testing.T) {
	if HasLibreOffice() {
		t.Skip("LibreOffice is installed, skipping fallback test")
	}

	dir := t.TempDir()
	longText := strings.Repeat("This is test content for chunking the document into searchable pieces. ", 5)
	path := createTestDocx(t, dir, longText)

	chunks, err := ChunkDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least one text chunk")
	}

	hasText := false
	for _, c := range chunks {
		if strings.Contains(c.Text, "test content") {
			hasText = true
			break
		}
	}
	if !hasText {
		t.Error("expected text chunk containing 'test content'")
	}
}

func TestChunkDocument_LegacyOfficeWithoutLibreOffice(t *testing.T) {
	if HasLibreOffice() {
		t.Skip("LibreOffice is installed, skipping fallback test")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "old.doc")
	os.WriteFile(path, []byte("fake doc"), 0o644)

	_, err := ChunkDocument(path)
	if err == nil {
		t.Error("expected error for legacy format without LibreOffice")
	}
	if !strings.Contains(err.Error(), "libreoffice required") {
		t.Errorf("expected LibreOffice error, got: %v", err)
	}
}
