package query

import "fmt"

// ClauseKey identifies a clause for denylist matching.
type ClauseKey struct {
	Field FieldEnum
	Op    Op
	Value string
}

// clauseValueString converts a clause value to a string for denylist matching.
func clauseValueString(v any) string {
	return fmt.Sprintf("%v", v)
}

// Merge combines grammar and LLM FilterSpecs with grammar-wins policy.
//   - For each field in LLM Must: if grammar also has a Must on the same field, the LLM clause is dropped.
//   - Non-conflicting clauses are unioned.
//   - ChipDenyList entries are dropped from the merged result.
//   - SemanticQuery comes from grammar (grammar wins).
//   - Source is set to SourceMerged.
func Merge(grammar, llm FilterSpec, chipDenyList []ClauseKey) FilterSpec {
	// Build set of fields grammar has claimed in Must.
	grammarMustFields := make(map[FieldEnum]bool)
	for _, c := range grammar.Must {
		grammarMustFields[c.Field] = true
	}

	// Build denylist lookup.
	denySet := make(map[ClauseKey]bool, len(chipDenyList))
	for _, k := range chipDenyList {
		denySet[k] = true
	}

	isDenied := func(c Clause) bool {
		k := ClauseKey{Field: c.Field, Op: c.Op, Value: clauseValueString(c.Value)}
		return denySet[k]
	}

	// Start with grammar Must (filtered by denylist).
	must := make([]Clause, 0, len(grammar.Must)+len(llm.Must))
	for _, c := range grammar.Must {
		if !isDenied(c) {
			must = append(must, c)
		}
	}
	// Add non-conflicting LLM Must clauses (field not already claimed by grammar).
	for _, c := range llm.Must {
		if grammarMustFields[c.Field] {
			continue // grammar wins
		}
		if !isDenied(c) {
			must = append(must, c)
		}
	}

	// MustNot: union of both (denylist applied).
	mustNot := make([]Clause, 0, len(grammar.MustNot)+len(llm.MustNot))
	for _, c := range grammar.MustNot {
		if !isDenied(c) {
			mustNot = append(mustNot, c)
		}
	}
	for _, c := range llm.MustNot {
		if !isDenied(c) {
			mustNot = append(mustNot, c)
		}
	}

	// Should: union of both (denylist applied).
	should := make([]Clause, 0, len(grammar.Should)+len(llm.Should))
	for _, c := range grammar.Should {
		if !isDenied(c) {
			should = append(should, c)
		}
	}
	for _, c := range llm.Should {
		if !isDenied(c) {
			should = append(should, c)
		}
	}

	// SemanticQuery: prefer grammar's if non-empty, else LLM's.
	semantic := grammar.SemanticQuery
	if semantic == "" {
		semantic = llm.SemanticQuery
	}

	return FilterSpec{
		SemanticQuery: semantic,
		Must:          must,
		MustNot:       mustNot,
		Should:        should,
		Source:        SourceMerged,
	}
}
