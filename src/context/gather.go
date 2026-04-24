package context

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const maxDirEntries = 20

// SystemContext holds local system information gathered before prompting.
type SystemContext struct {
	OS          string
	Arch        string
	Shell       string
	CWD         string
	DirListing  []string // "name/" for dirs, "name" for files
	GitBranch   string
	GitStatus   string
	GitRemote   string
	ProjectType string
	ProjectFile string // marker file that identified the project type
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
	ctx.gatherGit()
	ctx.gatherProjectType()

	return ctx
}

func (c *SystemContext) gatherDirListing() {
	entries, err := os.ReadDir(".")
	if err != nil {
		return
	}
	for i, e := range entries {
		if i >= maxDirEntries {
			c.DirListing = append(c.DirListing, fmt.Sprintf("... and %d more", len(entries)-maxDirEntries))
			break
		}
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		c.DirListing = append(c.DirListing, name)
	}
}

func (c *SystemContext) gatherGit() {
	// Check if we're in a git repo.
	if err := exec.Command("git", "rev-parse", "--is-inside-work-tree").Run(); err != nil {
		return
	}

	if out, err := exec.Command("git", "branch", "--show-current").Output(); err == nil {
		c.GitBranch = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("git", "status", "-s").Output(); err == nil {
		c.GitStatus = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("git", "remote", "get-url", "origin").Output(); err == nil {
		c.GitRemote = strings.TrimSpace(string(out))
	}
}

var projectMarkers = []struct {
	file       string
	projectTyp string
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
			c.ProjectType = m.projectTyp
			c.ProjectFile = m.file
			return
		}
	}
}
