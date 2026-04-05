package chunker

import (
	"fmt"
	"os"
)

func ChunkBinary(filePath, mimeType string) ([]Chunk, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read binary file: %w", err)
	}
	return []Chunk{{
		Content:  data,
		MimeType: mimeType,
		Index:    0,
	}}, nil
}
