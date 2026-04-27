package app

import (
	"context"

	"findo/internal/query"
)

type llmQueryParser interface {
	Parse(ctx context.Context, raw string, grammarSpec query.FilterSpec) (query.ParseResult, error)
}

type parsedQueryCacheIface interface {
	Get(query string) (*query.FilterSpec, error)
	Set(query string, spec query.FilterSpec) error
}
