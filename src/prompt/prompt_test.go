package prompt

import (
	"testing"

	"metaphox/underdash/input"
)

func TestBuildUserMessage(t *testing.T) {
	tests := []struct {
		name string
		inp  *input.ParsedInput
		want string
	}{
		{
			name: "query only",
			inp:  &input.ParsedInput{Query: "list files"},
			want: "list files",
		},
		{
			name: "supplementary only (whole prompt after --)",
			inp:  &input.ParsedInput{SupplementaryPrompt: "how do I suppress the newline"},
			want: "how do I suppress the newline",
		},
		{
			name: "both are framed",
			inp:  &input.ParsedInput{Query: "echo hi", SupplementaryPrompt: "no newline"},
			want: "echo hi\n\nAdditional context: no newline",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := BuildUserMessage(tc.inp); got != tc.want {
				t.Errorf("BuildUserMessage() = %q, want %q", got, tc.want)
			}
		})
	}
}
