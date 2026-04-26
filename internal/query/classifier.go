// Package query provides natural-language query parsing, classification, and
// caching utilities for the Universal Search pipeline.
package query

import (
	"regexp"
	"strings"
)

// QueryKind describes how a query should be routed through the search pipeline.
type QueryKind int

const (
	// KindContent routes the query to the semantic (embedding-based) pipeline only.
	KindContent QueryKind = iota
	// KindFilename routes the query to the filename FTS pipeline only.
	KindFilename
	// KindHybrid runs both pipelines and blends results via RRF.
	KindHybrid
)

// String returns a human-readable label for the kind, useful in logs and tests.
func (k QueryKind) String() string {
	switch k {
	case KindContent:
		return "content"
	case KindFilename:
		return "filename"
	case KindHybrid:
		return "hybrid"
	default:
		return "unknown"
	}
}

// reExtension matches a file extension: a dot followed by 1–4 alphanumeric chars
// at the end of the string.
var reExtension = regexp.MustCompile(`\.[a-zA-Z0-9]{1,4}$`)

// reIdentifier matches a bare identifier token: starts with a letter, followed by
// letters, digits, underscores, or hyphens — covers camelCase, snake_case, kebab-case.
var reIdentifier = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

// stopwords is the set of common English function words whose presence strongly
// suggests a natural-language content query rather than a filename query.
var stopwords = map[string]struct{}{
	"the": {}, "a": {}, "an": {}, "and": {}, "or": {}, "of": {},
	"in": {}, "on": {}, "at": {}, "with": {}, "about": {}, "for": {},
	"to": {}, "from": {}, "by": {}, "is": {}, "are": {}, "was": {},
	"were": {}, "be": {}, "been": {}, "this": {}, "that": {}, "these": {},
	"those": {}, "what": {}, "where": {}, "why": {}, "when": {}, "who": {},
	"how": {},
}

// Classify returns the routing kind and the (possibly prefix-stripped) query
// to feed downstream pipelines. The function is pure: same input → same output,
// no I/O, no globals.
//
// The heuristic ladder (top wins, ordered by confidence):
//  1. Trim whitespace. If trimmed string starts with "f:" (case-insensitive),
//     strip the prefix and return KindFilename with the remainder. (REQ-5)
//  2. Contains '*' or '?' (glob char) → KindFilename with the post-trim string.
//  3. Single token, ends with a dot followed by 1–4 alnum chars → KindFilename.
//  4. Single token, length ≤ 30, looks like an identifier → KindHybrid.
//  5. Has a common English stopword AND has ≥ 2 words → KindContent.
//  6. ≥ 4 words → KindContent.
//  7. Else → KindHybrid.
func Classify(raw string) (kind QueryKind, stripped string) {
	q := strings.TrimSpace(raw)

	// Rule 1: explicit "f:" prefix (case-insensitive).
	lower := strings.ToLower(q)
	if strings.HasPrefix(lower, "f:") {
		remainder := q[2:] // strip "f:" (always 2 bytes)
		return KindFilename, remainder
	}

	// Rule 2: glob characters.
	if strings.ContainsAny(q, "*?") {
		return KindFilename, q
	}

	words := strings.Fields(q)
	wordCount := len(words)

	// Empty / whitespace-only input.
	if wordCount == 0 {
		return KindContent, ""
	}

	// Rules 3 and 4 apply only to single-token queries.
	if wordCount == 1 {
		token := words[0]

		// Rule 3: has a file extension suffix.
		if reExtension.MatchString(token) {
			return KindFilename, q
		}

		// Rule 4: looks like a code identifier and is short enough.
		if len(token) <= 30 && reIdentifier.MatchString(token) {
			return KindHybrid, q
		}
	}

	// Rule 5: contains a stopword and has ≥ 2 words.
	if wordCount >= 2 {
		for _, w := range words {
			if _, ok := stopwords[strings.ToLower(w)]; ok {
				return KindContent, q
			}
		}
	}

	// Rule 6: ≥ 4 words (no stopword found at this point).
	if wordCount >= 4 {
		return KindContent, q
	}

	// Rule 7: default.
	return KindHybrid, q
}
