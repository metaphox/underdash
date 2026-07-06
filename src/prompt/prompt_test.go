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

func TestBuildSystemPrompt(t *testing.T) {
	got := BuildSystemPrompt()
	for _, want := range []string{
		"JSON schema",
		`"type": "command | explanation | script"`,
		"shell command assistant",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("system prompt missing %q", want)
		}
	}
}

func TestBuildRetryMessage(t *testing.T) {
	got := BuildRetryMessage("not json at all")
	if !strings.Contains(got, "not json at all") {
		t.Errorf("retry message does not echo the malformed response:\n%s", got)
	}
	if !strings.Contains(got, "valid JSON") {
		t.Errorf("retry message does not restate the JSON requirement:\n%s", got)
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0B"},
		{512, "512B"},
		{1024, "1.0K"},
		{1536, "1.5K"},
		{2 * 1024 * 1024, "2.0M"},
		{3 * 1024 * 1024 * 1024, "3.0G"},
	}
	for _, tc := range tests {
		if got := formatSize(tc.n); got != tc.want {
			t.Errorf("formatSize(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestBuildContextBlock_Sections(t *testing.T) {
	inp := &input.ParsedInput{Query: "do things"}

	t.Run("full context renders every section", func(t *testing.T) {
		ctx := &sysinfo.SystemContext{
			OS:    "darwin",
			Arch:  "arm64",
			Shell: "/bin/zsh",
			CWD:   "/work",
			DirEntries: []sysinfo.DirEntry{
				{Name: "src", Type: "dir"},
				{Name: "main.go", Type: "file", Size: 2048},
				{Name: "link", Type: "symlink"},
			},
			DirOverflow:  7,
			PathTools:    []string{"git", "go"},
			InGitRepo:    true,
			GitBranch:    "main",
			GitStatus:    " M main.go",
			GitRemote:    "git@example.com:x.git",
			GitLog:       []string{"second", "first"},
			ProjectType:  "go",
			ProjectFile:  "go.mod",
			ShellHistory: []string{"make test"},
		}
		got := BuildContextBlock(ctx, &input.ParsedInput{Query: "q", ToolHints: []string{"jq"}}, nil)

		for _, want := range []string{
			"os: darwin", "arch: arm64", "shell: /bin/zsh",
			"tools: git, go",
			"path: /work",
			"src/  dir",
			"main.go  file  2.0K",
			"link  symlink",
			"... and 7 more",
			"branch: main",
			"status:\n M main.go",
			"recent_commits:",
			"second",
			"remote: git@example.com:x.git",
			`<project type="go" marker="go.mod" />`,
			"<history>",
			"make test",
			"<tool_hints>",
			"The user wants to use: jq",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("context block missing %q; got:\n%s", want, got)
			}
		}
	})

	t.Run("clean repo reports clean status", func(t *testing.T) {
		ctx := &sysinfo.SystemContext{OS: "linux", Arch: "amd64", InGitRepo: true, GitBranch: "main"}
		got := BuildContextBlock(ctx, inp, nil)
		if !strings.Contains(got, "status: clean") {
			t.Errorf("expected clean status; got:\n%s", got)
		}
	})

	t.Run("optional sections omitted when empty", func(t *testing.T) {
		ctx := &sysinfo.SystemContext{OS: "linux", Arch: "amd64"}
		got := BuildContextBlock(ctx, inp, nil)
		for _, absent := range []string{"<git>", "<project", "<history>", "<tool_hints>", "shell:", "tools:"} {
			if strings.Contains(got, absent) {
				t.Errorf("context block should omit %q; got:\n%s", absent, got)
			}
		}
	})
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
