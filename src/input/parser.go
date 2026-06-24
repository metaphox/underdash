// Package input parses user input from CLI arguments and stdin.
package input

import (
	"os"
	"strings"
)

// ParsedInput is the result of parsing the user's raw CLI arguments.
type ParsedInput struct {
	// Query is the natural-language request after stripping tool hints and separators.
	Query string
	// ToolHints are explicitly requested tools (extracted from :tool tokens).
	ToolHints []string
	// SupplementaryPrompt is text after the -- separator, treated as pure prompt.
	SupplementaryPrompt string
	// Attachments are local file paths referenced with @path tokens that
	// resolved to existing files.
	Attachments []string
}

// Parse splits raw CLI args into a structured request.
//
// Syntax:
//   - :word  → tool hint (the word after colon is a tool name)
//   - @path  → file attachment, but only when path resolves to a real file
//   - --     → everything after this token is supplementary prompt
//   - all other tokens form the natural-language query
//
// dashPos is cobra's ArgsLenAtDash(): the index of a "--" that the flag parser
// already consumed (>= 0), or -1 when none was consumed. Because flag
// interspersing is disabled, a "--" that follows the first bareword is NOT
// consumed by the parser and instead survives as a literal token in args — so
// both signals are honored here.
func Parse(args []string, dashPos int) *ParsedInput {
	result := &ParsedInput{}

	queryArgs := args
	var suppArgs []string

	switch {
	case dashPos >= 0 && dashPos <= len(args):
		// The parser consumed a leading "--" and reported its position.
		queryArgs = args[:dashPos]
		suppArgs = args[dashPos:]
	default:
		// The parser left a literal "--" in args (separator after the first
		// bareword). Split on the first one and drop it.
		for i, a := range args {
			if a == "--" {
				queryArgs = args[:i]
				suppArgs = args[i+1:]
				break
			}
		}
	}

	var queryParts []string
	for _, arg := range queryArgs {
		// Tool hints are only extracted from the query portion; text after the
		// separator is pure prompt, not a tool request.
		if strings.HasPrefix(arg, ":") && len(arg) > 1 {
			result.ToolHints = append(result.ToolHints, arg[1:])
			continue
		}
		// An @path token is an attachment only when it points at a real file;
		// otherwise (an @mention, an email) it stays verbatim in the query.
		if strings.HasPrefix(arg, "@") && len(arg) > 1 {
			if path := arg[1:]; isFile(path) {
				result.Attachments = append(result.Attachments, path)
				continue
			}
		}
		queryParts = append(queryParts, arg)
	}

	result.Query = strings.Join(queryParts, " ")
	result.SupplementaryPrompt = strings.Join(suppArgs, " ")
	return result
}

// isFile reports whether path refers to an existing regular file (not a
// directory). Used to decide whether an @path token is an attachment.
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
