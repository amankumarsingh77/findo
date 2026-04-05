package chunker

type Chunk struct {
	Content   []byte
	Text      string
	MimeType  string
	StartTime float64
	EndTime   float64
	PageStart int
	PageEnd   int
	Index     int
}

type Strategy func(filePath string) ([]Chunk, error)
