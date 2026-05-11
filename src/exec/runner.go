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

// Run executes or displays the model's response according to the execution policy.
// overrides may be nil; when set, deny/auto_run/confirm patterns from config take precedence.
func Run(resp *response.Result, autoYes bool, dryRun bool, overrides *PolicyOverrides) error {
	switch resp.Type {
	case response.Explanation:
		display.ShowExplanation(resp.Explanation)
		return nil

	case response.Command:
		return runCommand(resp.Command, resp.Explanation, autoYes, dryRun, overrides)

	case response.Script:
		return runScript(resp.Script, resp.Explanation, autoYes, dryRun)

	default:
		return fmt.Errorf("unknown response type: %q", resp.Type)
	}
}

func runCommand(command string, explanation string, autoYes bool, dryRun bool, overrides *PolicyOverrides) error {
	risk := ClassifyWithOverrides(command, overrides)
	display.ShowCommand(command, explanation)

	// Denied is unconditional — --yes cannot bypass it.
	if risk == Denied {
		display.ShowError("Command is denied by execution policy.")
		return fmt.Errorf("denied by policy")
	}

	if dryRun {
		return nil
	}

	if !autoYes {
		switch risk {
		case Confirm:
			if !promptUser("Execute? [y/N] ") {
				fmt.Fprintln(os.Stderr, "Cancelled.")
				return nil
			}
		case Dangerous:
			display.ShowError("This is a destructive command.")
			if !promptUser("Execute anyway? [y/N] ") {
				fmt.Fprintln(os.Stderr, "Cancelled.")
				return nil
			}
		}
	}

	return executeShell(command)
}

func runScript(script string, explanation string, autoYes bool, dryRun bool) error {
	display.ShowScript(script, explanation)

	if dryRun {
		return nil
	}

	// Scripts are always treated as dangerous because they hide multiple operations.
	if !autoYes {
		display.ShowError("Review the script above carefully.")
		if !promptUser("Execute? [y/N] ") {
			fmt.Fprintln(os.Stderr, "Cancelled.")
			return nil
		}
	}

	// Write to temp file and execute.
	tmp, err := os.CreateTemp("", "underdash-*.sh")
	if err != nil {
		return fmt.Errorf("create temp script: %w", err)
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // best-effort cleanup of temp file

	if _, err := tmp.WriteString(script); err != nil {
		_ = tmp.Close() // close before returning error
		return fmt.Errorf("write temp script: %w", err)
	}
	_ = tmp.Close() // done writing; error would surface on re-read

	if err := os.Chmod(tmp.Name(), 0700); err != nil {
		return fmt.Errorf("chmod temp script: %w", err)
	}

	return executeShell(tmp.Name())
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
