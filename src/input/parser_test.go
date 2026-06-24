package input

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		dashPos   int
		wantQuery string
		wantSupp  string
		wantHints []string
	}{
		{
			name:      "no separator",
			args:      []string{"list", "large", "files"},
			dashPos:   -1,
			wantQuery: "list large files",
		},
		{
			name:      "literal -- left in args (mid-prompt separator)",
			args:      []string{"echo", "hi", "--", "no", "newline"},
			dashPos:   -1,
			wantQuery: "echo hi",
			wantSupp:  "no newline",
		},
		{
			name:     "consumed -- at front (whole thing is prompt)",
			args:     []string{"whole", "thing", "prompt"},
			dashPos:  0,
			wantSupp: "whole thing prompt",
		},
		{
			name:     "negative-number injection path: empty query, all supplementary",
			args:     []string{"-1", "times", "-1"},
			dashPos:  0,
			wantSupp: "-1 times -1",
		},
		{
			name:      "tool hints extracted from query only",
			args:      []string{":git", "commit", "staged", "--", ":not", "a", "hint"},
			dashPos:   -1,
			wantQuery: "commit staged",
			wantSupp:  ":not a hint",
			wantHints: []string{"git"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Parse(tc.args, tc.dashPos)
			if got.Query != tc.wantQuery {
				t.Errorf("Query = %q, want %q", got.Query, tc.wantQuery)
			}
			if got.SupplementaryPrompt != tc.wantSupp {
				t.Errorf("SupplementaryPrompt = %q, want %q", got.SupplementaryPrompt, tc.wantSupp)
			}
			if strings.Join(got.ToolHints, ",") != strings.Join(tc.wantHints, ",") {
				t.Errorf("ToolHints = %v, want %v", got.ToolHints, tc.wantHints)
			}
		})
	}
}

func TestParse_Attachments(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "diagram.png")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("existing @file becomes an attachment and leaves the query", func(t *testing.T) {
		got := Parse([]string{"explain", "@" + file, "please"}, -1)
		if got.Query != "explain please" {
			t.Errorf("Query = %q, want %q", got.Query, "explain please")
		}
		if len(got.Attachments) != 1 || got.Attachments[0] != file {
			t.Errorf("Attachments = %v, want [%s]", got.Attachments, file)
		}
	})

	t.Run("non-file @token stays verbatim in the query", func(t *testing.T) {
		got := Parse([]string{"email", "@nobody.example", "now"}, -1)
		if got.Query != "email @nobody.example now" {
			t.Errorf("Query = %q, want %q", got.Query, "email @nobody.example now")
		}
		if len(got.Attachments) != 0 {
			t.Errorf("Attachments = %v, want none", got.Attachments)
		}
	})

	t.Run("@tokens after -- are pure prompt, not attachments", func(t *testing.T) {
		got := Parse([]string{"run", "--", "@" + file}, -1)
		if got.SupplementaryPrompt != "@"+file {
			t.Errorf("SupplementaryPrompt = %q, want %q", got.SupplementaryPrompt, "@"+file)
		}
		if len(got.Attachments) != 0 {
			t.Errorf("Attachments = %v, want none", got.Attachments)
		}
	})
}
