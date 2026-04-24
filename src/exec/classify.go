package exec

import (
	"strings"
)

// RiskLevel indicates how dangerous a command is.
type RiskLevel int

const (
	Safe      RiskLevel = iota // auto-run
	Confirm                    // prompt y/n
	Dangerous                  // warn + prompt y/n
)

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
	"curl | sh", "curl |sh", "wget | sh", "wget |sh",
	"curl | bash", "curl |bash", "wget | bash", "wget |bash",
	"shutdown", "reboot", "init 0", "init 6",
	"mkfs.", "wipefs",
	"> /etc/", ">> /etc/",
}

// Classify returns the risk level of a command string.
// It checks each segment of a pipeline/chain independently and returns
// the highest risk found. Default is Confirm.
func Classify(command string) RiskLevel {
	// Split on pipes and logical operators to check each segment.
	segments := splitSegments(command)

	highest := Safe
	foundSafe := false

	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}

		// Check dangerous first (highest priority).
		for _, p := range dangerousPatterns {
			if strings.Contains(seg, p) {
				return Dangerous
			}
		}

		// Check safe.
		isSafe := false
		for _, s := range safeCommands {
			// Match if the segment starts with the safe command.
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

func splitSegments(cmd string) []string {
	// Replace multi-char separators first, then split on single chars.
	// Not shell-accurate but good enough for risk classification.
	s := strings.ReplaceAll(cmd, "&&", "|")
	s = strings.ReplaceAll(s, "||", "|")
	s = strings.ReplaceAll(s, ";", "|")
	return strings.Split(s, "|")
}
