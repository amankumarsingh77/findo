package query

import (
	"testing"
)

func TestClassify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        string
		wantKind     QueryKind
		wantStripped string
	}{
		// Rule 1: f: prefix (case-insensitive)
		{
			name:         "f:demo.py_returns_filename",
			input:        "f:demo.py",
			wantKind:     KindFilename,
			wantStripped: "demo.py",
		},
		{
			name:         "F:Report_returns_filename_case_insensitive",
			input:        "F:Report",
			wantKind:     KindFilename,
			wantStripped: "Report",
		},
		{
			name:         "f:_with_leading_whitespace_returns_filename",
			input:        "  f:readme.md",
			wantKind:     KindFilename,
			wantStripped: "readme.md",
		},
		// Rule 1 edge: empty remainder after f: prefix (EDGE-1)
		{
			name:         "f:_empty_remainder_returns_filename_empty",
			input:        "f:",
			wantKind:     KindFilename,
			wantStripped: "",
		},

		// Rule 2: glob characters
		{
			name:         "star_dot_py_returns_filename",
			input:        "*.py",
			wantKind:     KindFilename,
			wantStripped: "*.py",
		},
		{
			name:         "src_double_star_returns_filename",
			input:        "src/**",
			wantKind:     KindFilename,
			wantStripped: "src/**",
		},
		{
			name:         "foo_question_bar_returns_filename",
			input:        "foo?bar",
			wantKind:     KindFilename,
			wantStripped: "foo?bar",
		},
		// Rule 2 edge: bare star alone (EDGE-7)
		{
			name:         "bare_star_returns_filename",
			input:        "*",
			wantKind:     KindFilename,
			wantStripped: "*",
		},

		// Rule 3: single token with file extension
		{
			name:         "demo.py_returns_filename",
			input:        "demo.py",
			wantKind:     KindFilename,
			wantStripped: "demo.py",
		},
		{
			name:         "index.tsx_returns_filename",
			input:        "index.tsx",
			wantKind:     KindFilename,
			wantStripped: "index.tsx",
		},
		{
			name:         "report.docx_returns_filename",
			input:        "report.docx",
			wantKind:     KindFilename,
			wantStripped: "report.docx",
		},
		// Rule 3 edge: extension-only token (EDGE-8)
		{
			name:         "dot_py_extension_only_returns_filename",
			input:        ".py",
			wantKind:     KindFilename,
			wantStripped: ".py",
		},

		// Rule 4: single identifier token → hybrid
		{
			name:         "getUserById_returns_hybrid",
			input:        "getUserById",
			wantKind:     KindHybrid,
			wantStripped: "getUserById",
		},
		{
			name:         "report_q3_returns_hybrid",
			input:        "report_q3",
			wantKind:     KindHybrid,
			wantStripped: "report_q3",
		},
		{
			name:         "helpers_returns_hybrid",
			input:        "helpers",
			wantKind:     KindHybrid,
			wantStripped: "helpers",
		},

		// Rule 5: stopword present + ≥2 words → content
		{
			name:         "the_report_about_q3_sales_returns_content",
			input:        "the report about q3 sales",
			wantKind:     KindContent,
			wantStripped: "the report about q3 sales",
		},
		{
			name:         "find_a_movie_about_cats_returns_content",
			input:        "find a movie about cats",
			wantKind:     KindContent,
			wantStripped: "find a movie about cats",
		},
		{
			name:         "two_words_with_stopword_returns_content",
			input:        "the report",
			wantKind:     KindContent,
			wantStripped: "the report",
		},

		// Rule 6: ≥4 words, no stopword → content
		{
			name:         "four_word_query_without_stopwords_returns_content",
			input:        "four word query without stopwords",
			wantKind:     KindContent,
			wantStripped: "four word query without stopwords",
		},

		// Rule 7: default (2-3 words, no stopword) → hybrid
		{
			name:         "two_words_returns_hybrid",
			input:        "two words",
			wantKind:     KindHybrid,
			wantStripped: "two words",
		},

		// Edge cases: empty and whitespace-only (EDGE-1 / default content branch)
		{
			name:         "empty_string_returns_content",
			input:        "",
			wantKind:     KindContent,
			wantStripped: "",
		},
		{
			name:         "whitespace_only_returns_content",
			input:        "   ",
			wantKind:     KindContent,
			wantStripped: "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotKind, gotStripped := Classify(tc.input)
			if gotKind != tc.wantKind {
				t.Errorf("Classify(%q): kind = %s, want %s", tc.input, gotKind, tc.wantKind)
			}
			if gotStripped != tc.wantStripped {
				t.Errorf("Classify(%q): stripped = %q, want %q", tc.input, gotStripped, tc.wantStripped)
			}
		})
	}
}

func TestQueryKind_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind QueryKind
		want string
	}{
		{KindContent, "content"},
		{KindFilename, "filename"},
		{KindHybrid, "hybrid"},
		{QueryKind(99), "unknown"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			if got := tc.kind.String(); got != tc.want {
				t.Errorf("QueryKind(%d).String() = %q, want %q", tc.kind, got, tc.want)
			}
		})
	}
}
