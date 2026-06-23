package input

import (
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
