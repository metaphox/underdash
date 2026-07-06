package sysinfo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestHistorySource(t *testing.T) {
	tests := []struct {
		name     string
		shell    string
		wantPath string
		line     string
		wantLine string
	}{
		{
			name:     "zsh strips extended history metadata",
			shell:    "/bin/zsh",
			wantPath: ".zsh_history",
			line:     ": 1700000000:0;ls -la",
			wantLine: "ls -la",
		},
		{
			name:     "zsh passes plain lines through",
			shell:    "/usr/local/bin/zsh",
			wantPath: ".zsh_history",
			line:     "echo hi",
			wantLine: "echo hi",
		},
		{
			name:     "bash passes lines through",
			shell:    "/bin/bash",
			wantPath: ".bash_history",
			line:     "make test",
			wantLine: "make test",
		},
		{
			name:     "sh uses bash history",
			shell:    "/bin/sh",
			wantPath: ".bash_history",
			line:     "pwd",
			wantLine: "pwd",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path, parser := historySource(tc.shell)
			if path != tc.wantPath {
				t.Errorf("historySource(%q) path = %q, want %q", tc.shell, path, tc.wantPath)
			}
			if got := parser(tc.line); got != tc.wantLine {
				t.Errorf("parser(%q) = %q, want %q", tc.line, got, tc.wantLine)
			}
		})
	}

	t.Run("unsupported shell returns empty path", func(t *testing.T) {
		path, parser := historySource("/usr/bin/fish")
		if path != "" || parser != nil {
			t.Errorf("historySource(fish) = (%q, non-nil parser: %t), want empty", path, parser != nil)
		}
	})
}

func TestGatherDirListing(t *testing.T) {
	t.Run("classifies entry types and records file sizes", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "sub"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "data.txt"), []byte("hello"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("data.txt", filepath.Join(dir, "link")); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)

		c := &SystemContext{}
		c.gatherDirListing()

		want := map[string]DirEntry{
			"sub":      {Name: "sub", Type: "dir"},
			"data.txt": {Name: "data.txt", Type: "file", Size: 5},
			"link":     {Name: "link", Type: "symlink"},
		}
		if len(c.DirEntries) != len(want) {
			t.Fatalf("got %d entries, want %d: %+v", len(c.DirEntries), len(want), c.DirEntries)
		}
		for _, e := range c.DirEntries {
			if w, ok := want[e.Name]; !ok || e != w {
				t.Errorf("entry %q = %+v, want %+v", e.Name, e, w)
			}
		}
		if c.DirOverflow != 0 {
			t.Errorf("DirOverflow = %d, want 0", c.DirOverflow)
		}
	})

	t.Run("caps listing and reports overflow", func(t *testing.T) {
		dir := t.TempDir()
		total := maxDirEntries + 5
		for i := range total {
			name := filepath.Join(dir, fmt.Sprintf("f%02d", i))
			if err := os.WriteFile(name, nil, 0644); err != nil {
				t.Fatal(err)
			}
		}
		t.Chdir(dir)

		c := &SystemContext{}
		c.gatherDirListing()

		if len(c.DirEntries) != maxDirEntries {
			t.Errorf("got %d entries, want %d", len(c.DirEntries), maxDirEntries)
		}
		if c.DirOverflow != 5 {
			t.Errorf("DirOverflow = %d, want 5", c.DirOverflow)
		}
	})
}

func TestGatherProjectType(t *testing.T) {
	tests := []struct {
		name       string
		files      []string
		wantType   string
		wantMarker string
	}{
		{"go module", []string{"go.mod"}, "go", "go.mod"},
		{"node package", []string{"package.json"}, "node", "package.json"},
		{"earlier marker wins over Makefile", []string{"Makefile", "Cargo.toml"}, "rust", "Cargo.toml"},
		{"no marker", nil, "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range tc.files {
				if err := os.WriteFile(filepath.Join(dir, f), nil, 0644); err != nil {
					t.Fatal(err)
				}
			}
			t.Chdir(dir)

			c := &SystemContext{}
			c.gatherProjectType()
			if c.ProjectType != tc.wantType || c.ProjectFile != tc.wantMarker {
				t.Errorf("got (%q, %q), want (%q, %q)",
					c.ProjectType, c.ProjectFile, tc.wantType, tc.wantMarker)
			}
		})
	}
}

func TestGatherPathTools(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "jq")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	c := &SystemContext{}
	c.gatherPathTools()

	if len(c.PathTools) != 1 || c.PathTools[0] != "jq" {
		t.Errorf("PathTools = %v, want [jq]", c.PathTools)
	}
}

func TestGatherShellHistory(t *testing.T) {
	writeHistory := func(t *testing.T, name string, lines []string) {
		t.Helper()
		home := t.TempDir()
		t.Setenv("HOME", home)
		content := strings.Join(lines, "\n") + "\n"
		if err := os.WriteFile(filepath.Join(home, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("zsh extended format is stripped and truncated to n", func(t *testing.T) {
		var lines []string
		for i := range 30 {
			lines = append(lines, fmt.Sprintf(": 1700000%03d:0;cmd%d", i, i))
		}
		writeHistory(t, ".zsh_history", lines)
		viper.Set("context.history_lines", 5)
		defer viper.Set("context.history_lines", 0)

		c := &SystemContext{Shell: "/bin/zsh"}
		c.gatherShellHistory()

		if len(c.ShellHistory) != 5 {
			t.Fatalf("got %d lines, want 5: %v", len(c.ShellHistory), c.ShellHistory)
		}
		if c.ShellHistory[4] != "cmd29" {
			t.Errorf("last line = %q, want %q", c.ShellHistory[4], "cmd29")
		}
	})

	t.Run("bash history is read as-is with default limit", func(t *testing.T) {
		writeHistory(t, ".bash_history", []string{"ls", "pwd"})
		viper.Set("context.history_lines", 0)

		c := &SystemContext{Shell: "/bin/bash"}
		c.gatherShellHistory()

		want := []string{"ls", "pwd"}
		if len(c.ShellHistory) != len(want) {
			t.Fatalf("got %v, want %v", c.ShellHistory, want)
		}
		for i, w := range want {
			if c.ShellHistory[i] != w {
				t.Errorf("line %d = %q, want %q", i, c.ShellHistory[i], w)
			}
		}
	})

	t.Run("unsupported shell yields no history", func(t *testing.T) {
		writeHistory(t, ".fish_history", []string{"ls"})

		c := &SystemContext{Shell: "/usr/bin/fish"}
		c.gatherShellHistory()
		if c.ShellHistory != nil {
			t.Errorf("ShellHistory = %v, want nil", c.ShellHistory)
		}
	})

	t.Run("missing history file is silently ignored", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())

		c := &SystemContext{Shell: "/bin/zsh"}
		c.gatherShellHistory()
		if c.ShellHistory != nil {
			t.Errorf("ShellHistory = %v, want nil", c.ShellHistory)
		}
	})
}

// gitEnv returns exec environment overrides that isolate git from the user's
// global and system config.
func gitEnv() []string {
	return append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test",
	)
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = gitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestGatherGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	initRepo := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		runGit(t, dir, "init", "-b", "main")
		if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644); err != nil {
			t.Fatal(err)
		}
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "-m", "first commit")
		return dir
	}

	t.Run("outside a repo nothing is gathered", func(t *testing.T) {
		t.Chdir(t.TempDir())

		c := &SystemContext{}
		c.gatherGit()
		if c.InGitRepo {
			t.Error("InGitRepo = true outside a repo")
		}
	})

	t.Run("clean repo on a branch", func(t *testing.T) {
		dir := initRepo(t)
		t.Chdir(dir)

		c := &SystemContext{}
		c.gatherGit()

		if !c.InGitRepo {
			t.Fatal("InGitRepo = false inside a repo")
		}
		if c.GitBranch != "main" {
			t.Errorf("GitBranch = %q, want main", c.GitBranch)
		}
		if c.GitStatus != "" {
			t.Errorf("GitStatus = %q, want empty for clean tree", c.GitStatus)
		}
		if len(c.GitLog) != 1 || c.GitLog[0] != "first commit" {
			t.Errorf("GitLog = %v, want [first commit]", c.GitLog)
		}
		if c.GitRemote != "" {
			t.Errorf("GitRemote = %q, want empty without origin", c.GitRemote)
		}
	})

	t.Run("dirty tree and remote are reported", func(t *testing.T) {
		dir := initRepo(t)
		runGit(t, dir, "remote", "add", "origin", "https://example.com/repo.git")
		if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)

		c := &SystemContext{}
		c.gatherGit()

		if !strings.Contains(c.GitStatus, "b.txt") {
			t.Errorf("GitStatus = %q, want mention of b.txt", c.GitStatus)
		}
		if c.GitRemote != "https://example.com/repo.git" {
			t.Errorf("GitRemote = %q", c.GitRemote)
		}
	})

	t.Run("detached HEAD falls back to short SHA", func(t *testing.T) {
		dir := initRepo(t)
		sha := runGit(t, dir, "rev-parse", "--short", "HEAD")
		runGit(t, dir, "checkout", "--detach", "HEAD")
		t.Chdir(dir)

		c := &SystemContext{}
		c.gatherGit()

		want := "(detached at " + sha + ")"
		if c.GitBranch != want {
			t.Errorf("GitBranch = %q, want %q", c.GitBranch, want)
		}
	})
}

func TestGather(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("SHELL", "/bin/zsh")
	viper.Set("context.history", false)
	defer viper.Set("context.history", nil)

	ctx := Gather()

	if ctx.OS != runtime.GOOS || ctx.Arch != runtime.GOARCH {
		t.Errorf("OS/Arch = %q/%q, want %q/%q", ctx.OS, ctx.Arch, runtime.GOOS, runtime.GOARCH)
	}
	if ctx.Shell != "/bin/zsh" {
		t.Errorf("Shell = %q, want /bin/zsh", ctx.Shell)
	}
	if ctx.CWD == "" {
		t.Error("CWD is empty")
	}
	if ctx.ProjectType != "go" {
		t.Errorf("ProjectType = %q, want go", ctx.ProjectType)
	}
	if len(ctx.ShellHistory) != 0 {
		t.Errorf("ShellHistory gathered despite context.history=false: %v", ctx.ShellHistory)
	}
}
