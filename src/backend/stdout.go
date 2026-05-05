package backend

import (
	"context"
	"fmt"
)

// StdoutBackend prints the assembled prompt to stdout for debugging.
// It does not call any model and returns an empty string.
type StdoutBackend struct{}

// Name returns the backend identifier used for configuration and selection.
func (s *StdoutBackend) Name() string { return "stdout" }

// Send prints the assembled prompt and returns no model response.
func (s *StdoutBackend) Send(_ context.Context, req Request) (string, error) {
	fmt.Println("=== SYSTEM PROMPT ===")
	fmt.Println(req.SystemPrompt)
	fmt.Println()
	fmt.Println("=== USER MESSAGE ===")
	fmt.Println(req.UserMessage)
	return "", nil
}
