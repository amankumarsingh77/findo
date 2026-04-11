package query

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/genai"
)

const (
	llmModelName   = "gemini-2.5-flash-lite"
	llmTimeout     = 500 * time.Millisecond
)

// LLMParser parses a query using Gemini Flash-Lite with structured output.
type LLMParser struct {
	client  *genai.Client
	limiter interface{ Allow() bool }
	timeout time.Duration
	model   string
}

// NewLLMParser creates an LLMParser with the given client and rate limiter.
func NewLLMParser(client *genai.Client, limiter interface{ Allow() bool }) *LLMParser {
	return &LLMParser{
		client:  client,
		limiter: limiter,
		timeout: llmTimeout,
		model:   llmModelName,
	}
}

// llmClause is the JSON representation of a clause returned by the LLM.
type llmClause struct {
	Field string  `json:"field"`
	Op    string  `json:"op"`
	Value string  `json:"value"`
	Boost float64 `json:"boost"`
}

// llmResponse is the JSON schema the LLM must fill.
type llmResponse struct {
	SemanticQuery string      `json:"semantic_query"`
	Must          []llmClause `json:"must"`
	MustNot       []llmClause `json:"must_not"`
	Should        []llmClause `json:"should"`
}

// fieldEnumValues lists valid field enum strings for the response schema.
var fieldEnumValues = []string{
	string(FieldFileType),
	string(FieldExtension),
	string(FieldSizeBytes),
	string(FieldModifiedAt),
	string(FieldPath),
	string(FieldSemanticContains),
}

// opEnumValues lists valid op enum strings for the response schema.
var opEnumValues = []string{
	string(OpEq), string(OpNeq), string(OpGt), string(OpGte),
	string(OpLt), string(OpLte), string(OpContains), string(OpInSet),
}

// clauseSchema returns the genai.Schema for a single clause object.
func clauseSchema() *genai.Schema {
	return &genai.Schema{
		Type:     genai.TypeObject,
		Required: []string{"field", "op", "value"},
		Properties: map[string]*genai.Schema{
			"field": {
				Type: genai.TypeString,
				Enum: fieldEnumValues,
			},
			"op": {
				Type: genai.TypeString,
				Enum: opEnumValues,
			},
			"value": {Type: genai.TypeString},
			"boost": {Type: genai.TypeNumber},
		},
	}
}

// buildResponseSchema returns the schema for the LLM response.
func buildResponseSchema() *genai.Schema {
	cs := clauseSchema()
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"semantic_query": {Type: genai.TypeString},
			"must":           {Type: genai.TypeArray, Items: cs},
			"must_not":       {Type: genai.TypeArray, Items: cs},
			"should":         {Type: genai.TypeArray, Items: cs},
		},
	}
}

// Parse invokes Gemini with a structured response schema.
// If rate-limited, timed out, or errored, returns grammarSpec unchanged.
// Passes "Today is YYYY-MM-DD" in system prompt.
func (p *LLMParser) Parse(ctx context.Context, query string, grammarSpec FilterSpec) (FilterSpec, error) {
	if !p.limiter.Allow() {
		return grammarSpec, nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	today := time.Now().Format("2006-01-02")
	systemPrompt := fmt.Sprintf(
		`You convert a file search query into a structured filter. Today is %s.
Return JSON matching the schema. Only use the fields listed (file_type, extension,
size_bytes, modified_at, path, semantic_contains). Prefer 'should' with boost over
'must' for ambiguous tokens like language names. Never invent fields.`,
		today,
	)

	config := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemPrompt, genai.RoleUser),
		ResponseMIMEType:  "application/json",
		ResponseSchema:    buildResponseSchema(),
	}

	resp, err := p.client.Models.GenerateContent(
		timeoutCtx,
		p.model,
		genai.Text(query),
		config,
	)
	if err != nil {
		// Timeout or other error: return grammar spec unchanged.
		return grammarSpec, nil
	}

	text := resp.Text()
	if text == "" {
		return grammarSpec, nil
	}

	var llmResp llmResponse
	if err := json.Unmarshal([]byte(text), &llmResp); err != nil {
		return grammarSpec, nil
	}

	llmSpec := FilterSpec{
		SemanticQuery: strings.TrimSpace(llmResp.SemanticQuery),
		Source:        SourceLLM,
	}

	// Convert and validate clauses.
	for _, c := range llmResp.Must {
		if clause, ok := llmClauseToClause(c); ok {
			llmSpec.Must = append(llmSpec.Must, clause)
		}
	}
	for _, c := range llmResp.MustNot {
		if clause, ok := llmClauseToClause(c); ok {
			llmSpec.MustNot = append(llmSpec.MustNot, clause)
		}
	}
	for _, c := range llmResp.Should {
		if clause, ok := llmClauseToClause(c); ok {
			llmSpec.Should = append(llmSpec.Should, clause)
		}
	}

	return llmSpec, nil
}

// llmClauseToClause converts an llmClause to a Clause, validating the field.
// Returns (Clause{}, false) if the field is unknown (NLQ-034).
func llmClauseToClause(lc llmClause) (Clause, bool) {
	field := FieldEnum(lc.Field)
	if !KnownFields[field] {
		return Clause{}, false
	}
	op := Op(lc.Op)
	return Clause{
		Field: field,
		Op:    op,
		Value: lc.Value,
		Boost: float32(lc.Boost),
	}, true
}
