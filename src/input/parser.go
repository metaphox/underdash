package input

import "strings"

// ParsedInput is the result of parsing the user's raw CLI arguments.
type ParsedInput struct {
	// Query is the natural-language request after stripping tool hints and separators.
	Query string
	// ToolHints are explicitly requested tools (extracted from :tool tokens).
	ToolHints []string
	// SupplementaryPrompt is text after the -- separator, treated as pure prompt.
	SupplementaryPrompt string
}

// Parse splits raw CLI args into a structured request.
//
// Syntax:
//   - :word  → tool hint (the word after colon is a tool name)
//   - --     → everything after this token is supplementary prompt
//   - all other tokens form the natural-language query
func Parse(args []string) *ParsedInput {
	result := &ParsedInput{}

	var queryParts []string
	separatorSeen := false

	for _, arg := range args {
		if separatorSeen {
			// Everything after -- is supplementary prompt.
			if result.SupplementaryPrompt != "" {
				result.SupplementaryPrompt += " "
			}
			result.SupplementaryPrompt += arg
			continue
		}

		if arg == "--" {
			separatorSeen = true
			continue
		}

		if strings.HasPrefix(arg, ":") && len(arg) > 1 {
			result.ToolHints = append(result.ToolHints, arg[1:])
			continue
		}

		queryParts = append(queryParts, arg)
	}

	result.Query = strings.Join(queryParts, " ")
	return result
}
