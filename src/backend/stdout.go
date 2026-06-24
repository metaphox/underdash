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
	if len(req.Attachments) > 0 {
		fmt.Println()
		fmt.Println("=== ATTACHMENTS ===")
		for _, a := range req.Attachments {
			fmt.Printf("%s  [%s, %s, %d bytes encoded]\n", a.Filename, a.Kind, a.MediaType, len(a.Data))
		}
	}
	return "", nil
}
