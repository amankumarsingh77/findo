package embedder

import (
	"context"
	"fmt"
	"os"
	"time"

	"google.golang.org/genai"
)

const (
	// DefaultModel is the Gemini embedding model.
	DefaultModel = "gemini-embedding-exp-03-07"

	// TaskRetrieval is used for embedding queries at search time.
	TaskRetrieval = "RETRIEVAL_QUERY"

	// TaskDocument is used for embedding documents at index time.
	TaskDocument = "RETRIEVAL_DOCUMENT"

	// defaultRateLimit is ~55 requests per minute to stay within free tier.
	defaultRateLimit = 55
	defaultRateWindow = time.Minute
)

// Embedder wraps the Google Gemini API to produce vector embeddings
// from text, images, video, and audio.
type Embedder struct {
	client  *genai.Client
	model   string
	dims    int32
	limiter *RateLimiter
}

// NewEmbedder creates an Embedder with the given API key and output dimensionality.
func NewEmbedder(apiKey string, dims int32) (*Embedder, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("embedder: create client: %w", err)
	}
	return &Embedder{
		client:  client,
		model:   DefaultModel,
		dims:    dims,
		limiter: NewRateLimiter(defaultRateLimit, defaultRateWindow),
	}, nil
}

// NewEmbedderFromEnv creates an Embedder using the GEMINI_API_KEY environment variable.
func NewEmbedderFromEnv(dims int32) (*Embedder, error) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		key = os.Getenv("GOOGLE_API_KEY")
	}
	if key == "" {
		return nil, fmt.Errorf("embedder: GEMINI_API_KEY or GOOGLE_API_KEY must be set")
	}
	return NewEmbedder(key, dims)
}

// EmbedText produces an embedding for a single text string.
// taskType should be TaskRetrieval or TaskDocument.
func (e *Embedder) EmbedText(ctx context.Context, text string, taskType string) ([]float32, error) {
	e.limiter.Wait()

	content := genai.NewContentFromText(text, genai.RoleUser)
	resp, err := e.client.Models.EmbedContent(ctx, e.model, []*genai.Content{content}, &genai.EmbedContentConfig{
		TaskType:             taskType,
		OutputDimensionality: genai.Ptr(e.dims),
	})
	if err != nil {
		return nil, fmt.Errorf("embedder: embed text: %w", err)
	}
	if len(resp.Embeddings) == 0 {
		return nil, fmt.Errorf("embedder: no embeddings returned")
	}
	return resp.Embeddings[0].Values, nil
}

// EmbedTexts produces embeddings for multiple texts in a single API call.
// Each text is embedded as a separate Content entry.
func (e *Embedder) EmbedTexts(ctx context.Context, texts []string, taskType string) ([][]float32, error) {
	e.limiter.Wait()

	contents := make([]*genai.Content, len(texts))
	for i, t := range texts {
		contents[i] = genai.NewContentFromText(t, genai.RoleUser)
	}

	resp, err := e.client.Models.EmbedContent(ctx, e.model, contents, &genai.EmbedContentConfig{
		TaskType:             taskType,
		OutputDimensionality: genai.Ptr(e.dims),
	})
	if err != nil {
		return nil, fmt.Errorf("embedder: embed texts: %w", err)
	}
	if len(resp.Embeddings) != len(texts) {
		return nil, fmt.Errorf("embedder: expected %d embeddings, got %d", len(texts), len(resp.Embeddings))
	}

	result := make([][]float32, len(resp.Embeddings))
	for i, emb := range resp.Embeddings {
		result[i] = emb.Values
	}
	return result, nil
}

// EmbedBytes produces an embedding for binary content (images, video, audio).
// mimeType should be the IANA MIME type (e.g. "image/png", "video/mp4", "audio/wav").
func (e *Embedder) EmbedBytes(ctx context.Context, data []byte, mimeType string) ([]float32, error) {
	e.limiter.Wait()

	content := genai.NewContentFromBytes(data, mimeType, genai.RoleUser)
	resp, err := e.client.Models.EmbedContent(ctx, e.model, []*genai.Content{content}, &genai.EmbedContentConfig{
		OutputDimensionality: genai.Ptr(e.dims),
	})
	if err != nil {
		return nil, fmt.Errorf("embedder: embed bytes: %w", err)
	}
	if len(resp.Embeddings) == 0 {
		return nil, fmt.Errorf("embedder: no embeddings returned")
	}
	return resp.Embeddings[0].Values, nil
}

// EmbedQuery is a convenience wrapper that embeds text with RETRIEVAL_QUERY task type.
func (e *Embedder) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	return e.EmbedText(ctx, query, TaskRetrieval)
}

// EmbedDocument is a convenience wrapper that embeds text with RETRIEVAL_DOCUMENT task type.
func (e *Embedder) EmbedDocument(ctx context.Context, text string) ([]float32, error) {
	return e.EmbedText(ctx, text, TaskDocument)
}
