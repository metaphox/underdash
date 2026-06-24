package exec

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"metaphox/underdash/display"
	"metaphox/underdash/response"
)

// Outcome reports what Run did, for the audit log.
type Outcome struct {
	Action string // explained | executed | dry-run | denied | declined
	Risk   string // safe | confirm | dangerous | denied, or "" for non-commands
}

// Run executes or displays the model's response according to the execution policy.
// overrides may be nil; when set, deny/auto_run/confirm patterns from config take precedence.
func Run(resp *response.Result, autoYes bool, dryRun bool, overrides *PolicyOverrides) (Outcome, error) {
	switch resp.Type {
	case response.Explanation:
		display.ShowExplanation(resp.Explanation)
		return Outcome{Action: "explained"}, nil

	case response.Command:
		return runCommand(resp.Command, resp.Explanation, autoYes, dryRun, overrides)

	case response.Script:
		return runScript(resp.Script, resp.Explanation, autoYes, dryRun)

	default:
		return Outcome{}, fmt.Errorf("unknown response type: %q", resp.Type)
	}
}

func runCommand(command string, explanation string, autoYes bool, dryRun bool, overrides *PolicyOverrides) (Outcome, error) {
	risk := ClassifyWithOverrides(command, overrides)
	display.ShowCommand(command, explanation)
	out := Outcome{Risk: risk.String()}

	// Denied is unconditional — --yes cannot bypass it.
	if risk == Denied {
		display.ShowError("Command is denied by execution policy.")
		out.Action = "denied"
		return out, fmt.Errorf("denied by policy")
	}

	if dryRun {
		out.Action = "dry-run"
		return out, nil
	}

	if !autoYes {
		switch risk {
		case Confirm:
			if !promptUser("Execute? [y/N] ") {
				fmt.Fprintln(os.Stderr, "Cancelled.")
				out.Action = "declined"
				return out, nil
			}
		case Dangerous:
			display.ShowError("This is a destructive command.")
			if !promptUser("Execute anyway? [y/N] ") {
				fmt.Fprintln(os.Stderr, "Cancelled.")
				out.Action = "declined"
				return out, nil
			}
		}
	}

	out.Action = "executed"
	return out, executeShell(command)
}

func runScript(script string, explanation string, autoYes bool, dryRun bool) (Outcome, error) {
	display.ShowScript(script, explanation)
	out := Outcome{Risk: Dangerous.String()}

	if dryRun {
		out.Action = "dry-run"
		return out, nil
	}

	// Scripts are always treated as dangerous because they hide multiple operations.
	if !autoYes {
		display.ShowError("Review the script above carefully.")
		if !promptUser("Execute? [y/N] ") {
			fmt.Fprintln(os.Stderr, "Cancelled.")
			out.Action = "declined"
			return out, nil
		}
	}

	// Write to temp file and execute.
	out.Action = "executed"
	tmp, err := os.CreateTemp("", "underdash-*.sh")
	if err != nil {
		return out, fmt.Errorf("create temp script: %w", err)
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // best-effort cleanup of temp file

	if _, err := tmp.WriteString(script); err != nil {
		_ = tmp.Close() // close before returning error
		return out, fmt.Errorf("write temp script: %w", err)
	}
	_ = tmp.Close() // done writing; error would surface on re-read

	if err := os.Chmod(tmp.Name(), 0700); err != nil {
		return out, fmt.Errorf("chmod temp script: %w", err)
	}

	return out, executeShell(tmp.Name())
}

func executeShell(command string) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	cmd := exec.Command(shell, "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command exited with error: %w", err)
	}
	return nil
}

func promptUser(prompt string) bool {
	fmt.Fprint(os.Stderr, prompt)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}
