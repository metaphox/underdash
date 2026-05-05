// Package display handles terminal output formatting.
package display

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/term"
)

// outputMode holds the configured output mode ("streaming" or "plain").
var outputMode string

// SetOutputMode sets the output mode. Use "plain" to force plain output.
func SetOutputMode(mode string) {
	outputMode = mode
}

// IsTTY returns true if stdout is a terminal.
func IsTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// IsPlainMode returns true if output should be plain (no spinner, no ANSI).
// True when: --output=plain is set, OR stdout is not a TTY.
func IsPlainMode() bool {
	if outputMode == "plain" {
		return true
	}
	return !IsTTY()
}

// --- Spinner ---

// Spinner shows a single-line progress indicator on stderr.
type Spinner struct {
	frames []string
	stop   chan struct{}
	done   chan struct{}
	active bool
}

// NewSpinner creates a new spinner (does not start it).
func NewSpinner() *Spinner {
	return &Spinner{
		frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
}

// Start begins the spinner with the given message. No-op in plain mode.
func (s *Spinner) Start(msg string) {
	if IsPlainMode() {
		return
	}
	s.active = true
	go func() {
		defer close(s.done)
		i := 0
		for {
			select {
			case <-s.stop:
				// Clear the spinner line.
				fmt.Fprintf(os.Stderr, "\r\033[K")
				return
			default:
				fmt.Fprintf(os.Stderr, "\r%s %s", s.frames[i%len(s.frames)], msg)
				i++
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
}

// Stop halts the spinner and clears the line.
func (s *Spinner) Stop() {
	if !s.active {
		return
	}
	close(s.stop)
	<-s.done
	s.active = false
}

// --- Output helpers ---

// ShowCommand prints a command and optional explanation to stderr.
func ShowCommand(command string, explanation string) {
	if IsPlainMode() {
		fmt.Fprintf(os.Stderr, "> %s\n", command)
	} else {
		fmt.Fprintf(os.Stderr, "\033[1;32m❯\033[0m %s\n", command)
	}
	if explanation != "" {
		fmt.Fprintf(os.Stderr, "  %s\n", explanation)
	}
}

// ShowExplanation prints an explanation to stdout.
func ShowExplanation(text string) {
	fmt.Println(text)
}

// ShowScript prints a script and optional explanation.
func ShowScript(script string, explanation string) {
	if explanation != "" {
		fmt.Fprintf(os.Stderr, "  %s\n", explanation)
	}
	fmt.Fprintf(os.Stderr, "---\n%s\n---\n", script)
}

// ShowCommandOutput prints the stdout/stderr of an executed command.
func ShowCommandOutput(output string) {
	if output != "" {
		fmt.Print(output)
	}
}

// ShowError prints an error message to stderr.
func ShowError(msg string) {
	if IsPlainMode() {
		fmt.Fprintf(os.Stderr, "error: %s\n", msg)
	} else {
		fmt.Fprintf(os.Stderr, "\033[1;31merror:\033[0m %s\n", msg)
	}
}

// ShowDryRun prints the generated command/explanation/script without executing.
func ShowDryRun(systemPrompt string, userMessage string) {
	fmt.Println("=== DRY RUN ===")
	fmt.Println()
	fmt.Println("--- System Prompt ---")
	fmt.Println(systemPrompt)
	fmt.Println()
	fmt.Println("--- User Message ---")
	fmt.Println(userMessage)
}
