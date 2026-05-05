// Package sysinfo gathers local system context for prompt construction.
package sysinfo

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/viper"
)

const (
	maxDirEntries     = 20
	defaultHistoryN   = 20
	gitRecentCommitsN = 3
)

// DirEntry is a single entry in the cwd listing with type and size metadata.
type DirEntry struct {
	Name string
	Type string // "dir", "file", "symlink", "other"
	Size int64  // bytes; 0 for non-regular entries
}

// SystemContext holds local system information gathered before prompting.
type SystemContext struct {
	OS           string
	Arch         string
	Shell        string
	CWD          string
	DirEntries   []DirEntry
	DirOverflow  int      // count of entries beyond maxDirEntries, or 0
	PathTools    []string // notable tools available on $PATH
	InGitRepo    bool
	GitBranch    string
	GitStatus    string
	GitRemote    string
	GitLog       []string // last N commit subjects (newest first)
	ProjectType  string
	ProjectFile  string // marker file that identified the project type
	ShellHistory []string
}

// notablePathTools is the candidate list probed via exec.LookPath.
// Hardcoded for now; entries present on $PATH are surfaced to the model.
var notablePathTools = []string{
	"docker", "kubectl", "git", "make", "jq",
	"python", "python3", "node", "npm", "pnpm", "yarn",
	"cargo", "rustc", "go",
	"rg", "fd", "fzf",
}

// Gather collects system context. Best-effort: errors are silently ignored
// so that a missing git or unreadable directory never blocks the pipeline.
func Gather() *SystemContext {
	ctx := &SystemContext{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	ctx.Shell = os.Getenv("SHELL")
	ctx.CWD, _ = os.Getwd()

	ctx.gatherDirListing()
	ctx.gatherPathTools()
	ctx.gatherGit()
	ctx.gatherProjectType()
	if viper.GetBool("context.history") {
		ctx.gatherShellHistory()
	}

	return ctx
}

func (c *SystemContext) gatherDirListing() {
	entries, err := os.ReadDir(".")
	if err != nil {
		return
	}
	for i, e := range entries {
		if i >= maxDirEntries {
			c.DirOverflow = len(entries) - maxDirEntries
			break
		}
		entry := DirEntry{Name: e.Name()}
		switch {
		case e.IsDir():
			entry.Type = "dir"
		case e.Type()&os.ModeSymlink != 0:
			entry.Type = "symlink"
		case e.Type().IsRegular():
			entry.Type = "file"
		default:
			entry.Type = "other"
		}
		if info, err := e.Info(); err == nil && entry.Type == "file" {
			entry.Size = info.Size()
		}
		c.DirEntries = append(c.DirEntries, entry)
	}
}

func (c *SystemContext) gatherPathTools() {
	for _, tool := range notablePathTools {
		if _, err := exec.LookPath(tool); err == nil {
			c.PathTools = append(c.PathTools, tool)
		}
	}
}

func (c *SystemContext) gatherGit() {
	// Check if we're in a git repo.
	if err := exec.Command("git", "rev-parse", "--is-inside-work-tree").Run(); err != nil {
		return
	}
	c.InGitRepo = true

	if out, err := exec.Command("git", "branch", "--show-current").Output(); err == nil {
		c.GitBranch = strings.TrimSpace(string(out))
	}
	// Detached HEAD: --show-current returns empty; fall back to short SHA.
	if c.GitBranch == "" {
		if out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil {
			if sha := strings.TrimSpace(string(out)); sha != "" {
				c.GitBranch = "(detached at " + sha + ")"
			}
		}
	}
	if out, err := exec.Command("git", "status", "-s").Output(); err == nil {
		c.GitStatus = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("git", "remote", "get-url", "origin").Output(); err == nil {
		c.GitRemote = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("git", "log", fmt.Sprintf("-%d", gitRecentCommitsN), "--format=%s").Output(); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line != "" {
				c.GitLog = append(c.GitLog, line)
			}
		}
	}
}

func (c *SystemContext) gatherShellHistory() {
	n := viper.GetInt("context.history_lines")
	if n <= 0 {
		n = defaultHistoryN
	}

	path, parser := historySource(c.Shell)
	if path == "" {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	f, err := os.Open(filepath.Join(home, path))
	if err != nil {
		return
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		if entry := parser(scanner.Text()); entry != "" {
			lines = append(lines, entry)
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	c.ShellHistory = lines
}

// historySource returns the relative path under $HOME and a per-line parser
// for the user's shell, or "" if the shell is unsupported.
func historySource(shell string) (string, func(string) string) {
	base := filepath.Base(shell)
	switch base {
	case "zsh":
		// zsh extended history lines look like: ": 1700000000:0;ls -la"
		// Strip the metadata prefix; otherwise pass through.
		return ".zsh_history", func(line string) string {
			if strings.HasPrefix(line, ": ") {
				if idx := strings.Index(line, ";"); idx >= 0 {
					return line[idx+1:]
				}
			}
			return line
		}
	case "bash", "sh":
		return ".bash_history", func(line string) string { return line }
	}
	return "", nil
}

var projectMarkers = []struct {
	file        string
	projectType string
}{
	{"go.mod", "go"},
	{"package.json", "node"},
	{"Cargo.toml", "rust"},
	{"pyproject.toml", "python"},
	{"requirements.txt", "python"},
	{"Gemfile", "ruby"},
	{"Makefile", "make"},
}

func (c *SystemContext) gatherProjectType() {
	for _, m := range projectMarkers {
		p := filepath.Join(".", m.file)
		if _, err := os.Stat(p); err == nil {
			c.ProjectType = m.projectType
			c.ProjectFile = m.file
			return
		}
	}
}
