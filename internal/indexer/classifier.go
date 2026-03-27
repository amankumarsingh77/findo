package indexer

import (
	"path/filepath"
	"strings"
)

var fileTypes = map[string]string{
	// Text
	".pdf": "text", ".md": "text", ".txt": "text", ".docx": "text",
	".go": "text", ".py": "text", ".js": "text", ".ts": "text",
	".java": "text", ".rs": "text", ".c": "text", ".cpp": "text",
	".h": "text", ".json": "text", ".yaml": "text", ".yml": "text",
	".toml": "text", ".csv": "text", ".html": "text", ".css": "text",
	".sh": "text", ".bash": "text", ".rb": "text", ".php": "text",
	".swift": "text", ".kt": "text", ".scala": "text", ".xml": "text",
	// Images
	".jpg": "image", ".jpeg": "image", ".png": "image", ".webp": "image",
	".gif": "image", ".bmp": "image", ".tiff": "image", ".svg": "image",
	// Video
	".mp4": "video", ".mkv": "video", ".avi": "video", ".mov": "video",
	".webm": "video", ".flv": "video",
	// Audio
	".mp3": "audio", ".wav": "audio", ".flac": "audio", ".aac": "audio",
	".ogg": "audio", ".m4a": "audio",
}

// ClassifyFile returns the file category based on extension: "text", "image", "video", "audio", or "" for unknown.
func ClassifyFile(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	return fileTypes[ext]
}

// MimeType returns the MIME type string for the given file path based on extension.
func MimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	mimeTypes := map[string]string{
		".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png",
		".webp": "image/webp", ".gif": "image/gif", ".bmp": "image/bmp",
		".mp4": "video/mp4", ".mkv": "video/x-matroska", ".avi": "video/x-msvideo",
		".mov": "video/quicktime", ".webm": "video/webm",
		".mp3": "audio/mpeg", ".wav": "audio/wav", ".flac": "audio/flac",
		".aac": "audio/aac", ".ogg": "audio/ogg", ".m4a": "audio/mp4",
		".pdf": "application/pdf",
	}
	return mimeTypes[ext]
}
