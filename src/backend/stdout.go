package backend

import (
	"context"
	"fmt"
)

// StdoutBackend prints the assembled prompt to stdout for debugging.
// It does not call any model and returns an empty string.
type StdoutBackend struct{}

func (s *StdoutBackend) Name() string { return "stdout" }

func (s *StdoutBackend) Send(_ context.Context, req Request) (string, error) {
	fmt.Println("=== SYSTEM PROMPT ===")
	fmt.Println(req.SystemPrompt)
	fmt.Println()
	fmt.Println("=== USER MESSAGE ===")
	fmt.Println(req.UserMessage)
	return "", nil
}
