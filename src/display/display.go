// Package display handles terminal output formatting.
package display

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/glamour"
	"golang.org/x/term"
)

// outputMode holds the configured output mode ("streaming" or "plain").
var outputMode string

// markdownEnabled controls whether explanation output is rendered as Markdown
// on a TTY. It only takes effect outside plain mode.
var markdownEnabled = true

// SetOutputMode sets the output mode. Use "plain" to force plain output.
func SetOutputMode(mode string) {
	outputMode = mode
}

// SetMarkdown enables or disables Markdown rendering of explanation output.
func SetMarkdown(enabled bool) {
	markdownEnabled = enabled
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
	frames  []string
	stop    chan struct{}
	done    chan struct{}
	active  bool
	cleared atomic.Bool // set once the deadline countdown should stop
}

// NewSpinner creates a new spinner (does not start it).
func NewSpinner() *Spinner {
	return &Spinner{
		frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
}

// Start begins the spinner. The line reads "<frame> <label> <elapsed>s · <detail>",
// with detail omitted when empty. When deadline is non-zero, a "· timeout in <n>s"
// countdown is shown during the final 10 seconds before it — until ClearDeadline
// is called. No-op in plain mode.
func (s *Spinner) Start(label, detail string, deadline time.Time) {
	if IsPlainMode() {
		return
	}
	s.active = true
	s.cleared.Store(false)
	start := time.Now()
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
				var remaining time.Duration
				countdown := !deadline.IsZero() && !s.cleared.Load()
				if countdown {
					remaining = time.Until(deadline)
				}
				line := formatSpinner(s.frames[i%len(s.frames)], label, time.Since(start), remaining, detail, countdown)
				fmt.Fprintf(os.Stderr, "\r\033[K%s", line)
				i++
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
}

// ClearDeadline stops the response-timeout countdown (e.g. once the backend has
// responded). Safe to call concurrently with the render goroutine.
func (s *Spinner) ClearDeadline() {
	s.cleared.Store(true)
}

// formatSpinner builds the spinner line. The countdown segment is appended only
// when countdown is true and fewer than 10 seconds remain.
func formatSpinner(frame, label string, elapsed, remaining time.Duration, detail string, countdown bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s %ds", frame, label, int(elapsed/time.Second))
	if detail != "" {
		b.WriteString(" · ")
		b.WriteString(detail)
	}
	if countdown && remaining < 10*time.Second {
		n := max(int(remaining/time.Second), 0)
		fmt.Fprintf(&b, " · timeout in %ds", n)
	}
	return b.String()
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

// ShowExplanation prints an explanation to stdout. On a TTY (and when Markdown
// rendering is enabled) the text is treated as Markdown and rendered with color
// and layout, like Glow; piped/plain output prints the raw Markdown so it stays
// clean and pipe-friendly.
func ShowExplanation(text string) {
	plain := IsPlainMode() || !markdownEnabled
	out := renderExplanation(text, plain)
	if plain {
		fmt.Println(out)
	} else {
		fmt.Print(out) // glamour already pads the rendered block with newlines
	}
}

// renderExplanation returns the explanation ready to print: the raw text when
// plain, otherwise Glow-style ANSI-rendered Markdown. Any rendering error falls
// back to the raw text — display must never fail the command.
func renderExplanation(text string, plain bool) string {
	if plain {
		return text
	}
	rendered, err := renderMarkdown(text)
	if err != nil {
		return text
	}
	return rendered
}

// renderMarkdown renders Markdown to ANSI using glamour (the library behind Glow),
// auto-styling for the terminal's dark/light background and wrapping to its width.
func renderMarkdown(text string) (string, error) {
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(markdownWidth()),
	)
	if err != nil {
		return "", err
	}
	return r.Render(text)
}

// markdownWidth returns the wrap width for rendered Markdown: the terminal width,
// capped for readability, falling back to 80 when it can't be determined.
func markdownWidth() int {
	const (
		fallback = 80
		maxWidth = 120
	)
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return fallback
	}
	return min(w, maxWidth)
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

// ShowErrorDetails prints a red "error:" summary followed by indented detail
// lines (dimmed in TTY mode), e.g. the provider message, request id, and a hint.
func ShowErrorDetails(summary string, details []string) {
	ShowError(summary)
	for _, d := range details {
		if IsPlainMode() {
			fmt.Fprintf(os.Stderr, "  %s\n", d)
		} else {
			fmt.Fprintf(os.Stderr, "  \033[2m%s\033[0m\n", d)
		}
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
