package indexer

import (
	"path/filepath"
	"strings"

	"universal-search/internal/chunker"
)

func ClassifyFile(path string) string {
	return string(chunker.Classify(path))
}

func MimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	mimeTypes := map[string]string{
		".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png",
		".webp": "image/webp", ".heic": "image/heic", ".heif": "image/heif",
		".mp4": "video/mp4", ".mpeg": "video/mpeg", ".mpg": "video/mpeg",
		".mov": "video/quicktime", ".avi": "video/x-msvideo",
		".webm": "video/webm", ".flv": "video/x-flv",
		".wmv": "video/x-ms-wmv", ".3gp": "video/3gpp",
		".mp3": "audio/mpeg", ".wav": "audio/wav", ".flac": "audio/flac",
		".aac": "audio/aac", ".ogg": "audio/ogg", ".aiff": "audio/aiff",
		".pdf": "application/pdf",
	}
	return mimeTypes[ext]
}
