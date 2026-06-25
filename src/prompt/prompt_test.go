package prompt

import (
	"strings"
	"testing"

	"metaphox/underdash/backend"
	"metaphox/underdash/input"
	"metaphox/underdash/sysinfo"
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

func TestBuildContextBlock_Attachments(t *testing.T) {
	ctx := &sysinfo.SystemContext{OS: "darwin", Arch: "arm64"}
	inp := &input.ParsedInput{Query: "describe"}

	t.Run("section omitted when there are no attachments", func(t *testing.T) {
		if got := BuildContextBlock(ctx, inp, nil); strings.Contains(got, "<attachments>") {
			t.Errorf("expected no <attachments> section, got:\n%s", got)
		}
	})

	t.Run("section lists each attachment", func(t *testing.T) {
		atts := []backend.Attachment{
			{Filename: "diagram.png", Kind: "image", MediaType: "image/png"},
			{Filename: "report.pdf", Kind: "document", MediaType: "application/pdf"},
		}
		got := BuildContextBlock(ctx, inp, atts)
		for _, want := range []string{
			"<attachments>",
			"diagram.png (image, image/png)",
			"report.pdf (document, application/pdf)",
			"</attachments>",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("context block missing %q; got:\n%s", want, got)
			}
		}
	})
}
