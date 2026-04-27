package query

import (
	"strings"
	"time"
	"unicode"
)

// recognizedOps is the whitelist of operator keywords that may appear before ':'.
var recognizedOps = map[string]bool{
	"kind":   true,
	"ext":    true,
	"size":   true,
	"before": true,
	"after":  true,
	"since":  true,
	"in":     true,
	"path":   true,
}

// nowFunc returns the reference time used by the grammar's NL date resolution.
// Overridable from tests via setNowForTest. Production always uses time.Now().
var nowFunc = func() time.Time { return time.Now() }

// Parse converts a user query string into a FilterSpec.
// It never panics and returns a best-effort result on malformed input.
func Parse(input string) (spec FilterSpec) {
	defer func() {
		_ = recover()
		spec.Source = SourceGrammar
	}()

	now := nowFunc()
	var semanticParts []string

	if nlSpec, nlSemantic, ok := parseNaturalLanguage(input, now); ok {
		spec = nlSpec
		if nlSemantic != "" {
			spec.SemanticQuery = nlSemantic
		}
		spec.Source = SourceGrammar
		return spec
	}

	runes := []rune(input)
	pos := 0
	n := len(runes)

	for pos < n {
		for pos < n && unicode.IsSpace(runes[pos]) {
			pos++
		}
		if pos >= n {
			break
		}

		if runes[pos] == '"' {
			pos++
			start := pos
			for pos < n && runes[pos] != '"' {
				pos++
			}
			phrase := string(runes[start:pos])
			if pos < n {
				pos++
			}
			semanticParts = append(semanticParts, phrase)
			continue
		}

		if runes[pos] == '-' && pos+1 < n && !unicode.IsSpace(runes[pos+1]) {
			pos++
			start := pos
			tok := readToken(runes, &pos)
			if tok == "" {
				continue
			}
			colonIdx := strings.IndexByte(tok, ':')
			if colonIdx > 0 {
				keyword := strings.ToLower(tok[:colonIdx])
				if recognizedOps[keyword] {
					value := tok[colonIdx+1:]
					if value == "" {
						for pos < n && unicode.IsSpace(runes[pos]) {
							pos++
						}
						value = readOpValue(runes, &pos, now)
					}
					if clauses, _, handled := parseOperator(keyword, value, now); handled {
						spec.MustNot = append(spec.MustNot, clauses...)
						continue
					}
				}
			}
			_ = start
			spec.MustNot = append(spec.MustNot, Clause{
				Field: FieldPath,
				Op:    OpContains,
				Value: tok,
			})
			continue
		}

		start := pos
		tok := readToken(runes, &pos)
		if tok == "" {
			continue
		}

		colonIdx := strings.IndexByte(tok, ':')
		if colonIdx > 0 {
			keyword := strings.ToLower(tok[:colonIdx])
			if recognizedOps[keyword] {
				value := tok[colonIdx+1:]
				// If value is empty or operator value needs multi-word support,
				// collect the full value (may be quoted or multi-word for date ops).
				if value == "" {
					for pos < n && unicode.IsSpace(runes[pos]) {
						pos++
					}
					value = readOpValue(runes, &pos, now)
				}
				if clause, extra, handled := parseOperator(keyword, value, now); handled {
					spec.Must = append(spec.Must, clause...)
					if extra != "" {
						semanticParts = append(semanticParts, extra)
					}
					continue
				}
			}
			semanticParts = append(semanticParts, string(runes[start:pos]))
			continue
		}

		semanticParts = append(semanticParts, tok)
	}

	semanticText := strings.TrimSpace(strings.Join(semanticParts, " "))

	// Post-pass: scan semantic text for an embedded date phrase. If one is
	// found AND the spec doesn't already have a modified_at clause (from an
	// explicit operator like before:/after:), emit it and strip the matched
	// phrase from the semantic text.
	if semanticText != "" && !specHasModifiedAt(spec) {
		if after, before, matched, ok := scanForEmbeddedDate(semanticText, now); ok {
			spec.Must = append(spec.Must,
				Clause{Field: FieldModifiedAt, Op: OpGte, Value: after.Unix()},
				Clause{Field: FieldModifiedAt, Op: OpLte, Value: before.Unix()},
			)
			semanticText = removeMatchedPhrase(semanticText, matched)
		}
	}

	spec.SemanticQuery = semanticText
	return spec
}

// specHasModifiedAt returns true iff spec.Must already contains a modified_at
// clause (from an explicit before:/after: operator or from parseNaturalLanguage).
func specHasModifiedAt(spec FilterSpec) bool {
	for _, c := range spec.Must {
		if c.Field == FieldModifiedAt {
			return true
		}
	}
	return false
}

// removeMatchedPhrase strips the first occurrence of the matched date phrase
// from the semantic text and also strips nearby filler connectives (from, on,
// at, in, of, during, dated, created, modified) that precede it, along with
// possessive "'s" suffixes.
func removeMatchedPhrase(text, phrase string) string {
	lowerText := strings.ToLower(text)
	idx := strings.Index(lowerText, phrase)
	if idx < 0 {
		return strings.TrimSpace(text)
	}
	before := text[:idx]
	after := text[idx+len(phrase):]

	trimmedBefore := strings.TrimRight(before, " ")
	multiWord := []string{
		"created in the", "created in", "created on", "uploaded in", "uploaded on",
		"modified in the", "modified in", "modified on",
		"in the past", "in the last", "within the last", "within the past",
		"within the", "in the", "within",
	}
	singleWord := []string{
		"from", "on", "at", "during", "dated", "in", "of", "for", "about",
		"around", "just", "created", "modified", "uploaded",
	}
	for _, f := range multiWord {
		if strings.HasSuffix(strings.ToLower(trimmedBefore), " "+f) ||
			strings.EqualFold(trimmedBefore, f) {
			trimmedBefore = trimmedBefore[:len(trimmedBefore)-len(f)]
			trimmedBefore = strings.TrimRight(trimmedBefore, " ")
			break
		}
	}
	for i := 0; i < 3; i++ {
		stripped := false
		for _, filler := range singleWord {
			if strings.HasSuffix(strings.ToLower(trimmedBefore), " "+filler) ||
				strings.EqualFold(trimmedBefore, filler) {
				trimmedBefore = trimmedBefore[:len(trimmedBefore)-len(filler)]
				trimmedBefore = strings.TrimRight(trimmedBefore, " ")
				stripped = true
				break
			}
		}
		if !stripped {
			break
		}
	}

	after = strings.TrimLeft(after, " ")
	if strings.HasPrefix(after, "'s ") || after == "'s" {
		after = strings.TrimPrefix(after, "'s")
		after = strings.TrimLeft(after, " ")
	}

	combined := strings.TrimSpace(trimmedBefore + " " + after)
	for strings.Contains(combined, "  ") {
		combined = strings.ReplaceAll(combined, "  ", " ")
	}
	return combined
}

// readToken reads runes until whitespace or end-of-input.
func readToken(runes []rune, pos *int) string {
	start := *pos
	for *pos < len(runes) && !unicode.IsSpace(runes[*pos]) {
		(*pos)++
	}
	return string(runes[start:*pos])
}

// twoWordDatePrefixes are the first words of recognized two-word relative date phrases.
var twoWordDatePrefixes = map[string]bool{
	"last": true,
	"past": true,
	"this": true,
	"next": true,
}

// readOpValue reads an operator value, supporting:
//   - Quoted strings: "last week"
//   - Two-word relative date phrases: last week, last month, past 3 days
//   - Single words otherwise
func readOpValue(runes []rune, pos *int, now time.Time) string {
	n := len(runes)
	if *pos >= n {
		return ""
	}

	if runes[*pos] == '"' {
		(*pos)++
		start := *pos
		for *pos < n && runes[*pos] != '"' {
			(*pos)++
		}
		val := string(runes[start:*pos])
		if *pos < n {
			(*pos)++
		}
		return val
	}

	first := readToken(runes, pos)
	if first == "" {
		return ""
	}

	lower := strings.ToLower(first)
	if twoWordDatePrefixes[lower] {
		savedPos := *pos
		for *pos < n && unicode.IsSpace(runes[*pos]) {
			(*pos)++
		}
		second := readToken(runes, pos)
		if second != "" {
			candidate := first + " " + second
			if _, _, ok := NormalizeDate(candidate, now); ok {
				return candidate
			}
		}
		*pos = savedPos
	}

	return first
}

// parseOperator processes a recognized keyword + value pair.
// Returns clauses, any residual semantic text, and whether it was handled.
func parseOperator(keyword, value string, now time.Time) (clauses []Clause, residual string, handled bool) {
	switch keyword {
	case "kind":
		return parseKind(value)
	case "ext":
		return parseExt(value)
	case "size":
		return parseSize(value)
	case "before":
		return parseDateOp("before", value, now)
	case "after", "since":
		return parseDateOp("after", value, now)
	case "path", "in":
		return parsePath(value)
	}
	return nil, "", false
}

func parseKind(value string) ([]Clause, string, bool) {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return nil, "", false
	}
	if canonical, ok := KnownKindValues[lower]; ok {
		return []Clause{{Field: FieldFileType, Op: OpEq, Value: canonical}}, "", true
	}
	if canonical, ok := CorrectKind(lower); ok {
		return []Clause{{Field: FieldFileType, Op: OpEq, Value: canonical}}, "", true
	}
	return nil, "kind:" + value, false
}

func parseExt(value string) ([]Clause, string, bool) {
	if value == "" {
		return nil, "", false
	}
	parts := strings.Split(value, ",")
	exts := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		bare := strings.TrimPrefix(p, ".")
		if corrected, ok := CorrectExtension(bare); ok {
			exts = append(exts, "."+corrected)
		} else {
			if !strings.HasPrefix(p, ".") {
				p = "." + p
			}
			exts = append(exts, p)
		}
	}
	if len(exts) == 0 {
		return nil, "", false
	}
	return []Clause{{Field: FieldExtension, Op: OpInSet, Value: exts}}, "", true
}

func parseSize(value string) ([]Clause, string, bool) {
	op, bytes, ok := ParseSize(value)
	if !ok {
		return nil, "size:" + value, false
	}
	return []Clause{{Field: FieldSizeBytes, Op: op, Value: bytes}}, "", true
}

func parseDateOp(direction, value string, now time.Time) ([]Clause, string, bool) {
	if value == "" {
		return nil, "", false
	}
	afterT, _, ok := NormalizeDate(value, now)
	if !ok {
		return nil, direction + ":" + value, false
	}

	var clauses []Clause
	switch direction {
	case "before":
		clauses = append(clauses, Clause{Field: FieldModifiedAt, Op: OpLt, Value: afterT})
	case "after":
		clauses = append(clauses, Clause{Field: FieldModifiedAt, Op: OpGte, Value: afterT})
	}
	return clauses, "", true
}

func parsePath(value string) ([]Clause, string, bool) {
	if value == "" {
		return nil, "", false
	}
	return []Clause{{Field: FieldPath, Op: OpContains, Value: value}}, "", true
}
