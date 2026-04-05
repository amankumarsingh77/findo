package indexer

import "testing"

func TestClassifyFile(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"doc.pdf", "document"},
		{"photo.jpg", "image"},
		{"clip.mp4", "video"},
		{"song.mp3", "audio"},
		{"pic.webp", "image"},
		{"slides.pptx", "document"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := ClassifyFile(tt.path)
			if got != tt.expected {
				t.Errorf("ClassifyFile(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestMimeType(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"photo.jpg", "image/jpeg"},
		{"video.mp4", "video/mp4"},
		{"song.mp3", "audio/mpeg"},
		{"doc.pdf", "application/pdf"},
		{"unknown.xyz", ""},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := MimeType(tt.path)
			if got != tt.expected {
				t.Errorf("MimeType(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}
