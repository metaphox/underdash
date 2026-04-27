package exec

import (
	"path/filepath"
	"strings"
)

// RiskLevel indicates how dangerous a command is.
type RiskLevel int

const (
	Safe      RiskLevel = iota // auto-run
	Confirm                    // prompt y/n
	Dangerous                  // warn + prompt y/n
	Denied                     // unconditionally blocked, even --yes cannot bypass
)

// PolicyOverrides holds user-configured execution policy from config.yaml.
type PolicyOverrides struct {
	AutoRun []string // glob patterns → Safe
	Confirm []string // glob patterns → Confirm
	Deny    []string // glob patterns → Denied
}

// safeCommands are read-only or low-impact commands that can auto-run.
var safeCommands = []string{
	"ls", "cat", "head", "tail", "wc", "echo", "pwd", "which", "file",
	"date", "whoami", "hostname", "uname",
	"grep", "rg", "ag", "find", "tree", "du", "df", "stat", "realpath",
	"git log", "git diff", "git status", "git branch", "git show", "git remote",
	"man", "help", "type", "printenv",
}

// dangerousPatterns are destructive or privileged patterns.
var dangerousPatterns = []string{
	"rm -rf", "rm -r", "rm -f",
	"sudo ", "doas ",
	"dd ", "mkfs", "fdisk", "parted",
	"chmod 777", "chmod -R",
	"> /dev/",
	":(){ :|:& };:",
	"shutdown", "reboot", "init 0", "init 6",
	"mkfs.", "wipefs",
	"> /etc/", ">> /etc/",
}

// pipePatterns are dangerous patterns that span across pipe boundaries.
// These must be checked against the full command string before splitting.
// Format: left command prefix piped to right command.
var pipeToShellLeft = []string{"curl", "wget"}
var pipeToShellRight = []string{"sh", "bash", "zsh", "/bin/sh", "/bin/bash"}

// Classify returns the risk level of a command string using only hardcoded rules.
func Classify(command string) RiskLevel {
	return ClassifyWithOverrides(command, nil)
}

// ClassifyWithOverrides returns the risk level of a command, checking user-configured
// overrides first, then falling back to hardcoded rules.
func ClassifyWithOverrides(command string, overrides *PolicyOverrides) RiskLevel {
	// Check user overrides first (highest priority).
	if overrides != nil {
		// Deny takes precedence over everything.
		for _, pattern := range overrides.Deny {
			if matchGlob(pattern, command) {
				return Denied
			}
		}
		// Then auto_run.
		for _, pattern := range overrides.AutoRun {
			if matchGlob(pattern, command) {
				return Safe
			}
		}
		// Then confirm.
		for _, pattern := range overrides.Confirm {
			if matchGlob(pattern, command) {
				return Confirm
			}
		}
	}

	// Check pipe-to-shell patterns against the FULL command first.
	if isPipeToShell(command) {
		return Dangerous
	}

	// Now split into segments and classify each.
	segments := splitSegments(command)

	highest := Safe
	foundSafe := false

	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}

		// Check dangerous patterns per segment.
		for _, p := range dangerousPatterns {
			if strings.Contains(seg, p) {
				return Dangerous
			}
		}

		// Check safe.
		isSafe := false
		for _, s := range safeCommands {
			if seg == s || strings.HasPrefix(seg, s+" ") || strings.HasPrefix(seg, s+"\t") {
				isSafe = true
				break
			}
		}

		if isSafe {
			foundSafe = true
		} else {
			if highest < Confirm {
				highest = Confirm
			}
		}
	}

	if highest == Safe && foundSafe {
		return Safe
	}
	if highest == Safe {
		// No segments matched anything — default to confirm.
		return Confirm
	}
	return highest
}

// matchGlob checks if a command matches a glob-style pattern.
// Uses filepath.Match with a fallback to prefix matching for trailing *.
func matchGlob(pattern, command string) bool {
	// filepath.Match doesn't handle spaces well for our use case.
	// Use simple prefix matching: "docker ps *" matches "docker ps -a".
	if strings.HasSuffix(pattern, " *") {
		prefix := strings.TrimSuffix(pattern, " *")
		return command == prefix || strings.HasPrefix(command, prefix+" ")
	}
	// Exact match or filepath glob.
	if matched, err := filepath.Match(pattern, command); err == nil && matched {
		return true
	}
	return command == pattern
}

// isPipeToShell checks if a command pipes download tool output into a shell interpreter.
// e.g., "curl https://example.com/setup | bash"
func isPipeToShell(command string) bool {
	// Split on pipe only (not && or ;) to find piped pairs.
	parts := strings.Split(command, "|")
	if len(parts) < 2 {
		return false
	}
	for i := 0; i < len(parts)-1; i++ {
		left := strings.TrimSpace(parts[i])
		right := strings.TrimSpace(parts[i+1])
		// Check if left starts with a download command.
		leftIsDL := false
		for _, dl := range pipeToShellLeft {
			if left == dl || strings.HasPrefix(left, dl+" ") {
				leftIsDL = true
				break
			}
		}
		if !leftIsDL {
			continue
		}
		// Check if right is a shell interpreter.
		for _, sh := range pipeToShellRight {
			if right == sh || strings.HasPrefix(right, sh+" ") {
				return true
			}
		}
	}
	return false
}

func splitSegments(cmd string) []string {
	// Replace multi-char separators first, then split on single chars.
	// Not shell-accurate but good enough for risk classification.
	s := strings.ReplaceAll(cmd, "&&", "|")
	s = strings.ReplaceAll(s, "||", "|")
	s = strings.ReplaceAll(s, ";", "|")
	return strings.Split(s, "|")
}
