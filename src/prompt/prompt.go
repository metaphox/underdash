package prompt

import (
	"fmt"
	"strings"

	uctx "metaphox/underdash/context"
	"metaphox/underdash/input"
)

const systemPrompt = `You are a shell command assistant. You help users accomplish tasks on their system by producing shell commands, scripts, or explanations.

You MUST respond with a single valid JSON object and nothing else. No markdown, no commentary outside the JSON.

JSON schema:

{
  "type": "command | explanation | script",
  "command": "...",
  "explanation": "...",
  "script": "..."
}

Rules:
- "type" is required. It must be one of: "command", "explanation", "script".
- When type is "command": "command" is required (a single shell expression). Use && or || or pipes to chain steps. "explanation" is optional.
- When type is "explanation": "explanation" is required. "command" and "script" are absent.
- When type is "script": "script" is required (a multi-line executable script including shebang). "explanation" is optional.
- Choose "command" for anything expressible as a one-liner or short pipeline.
- Choose "script" only when the task genuinely requires multi-line logic (loops, conditionals, temp files).
- Choose "explanation" when the user is asking a question, not requesting an action.

Constraints:
- Preserve shell variable references literally (e.g., $TOKEN, $HOME). Never substitute their values.
- Prefer single commands over scripts when possible.
- Target the user's detected shell (see context).
- Do not wrap the JSON in a code fence or any other formatting.`

// BuildSystemPrompt returns the fixed system prompt.
func BuildSystemPrompt() string {
	return systemPrompt
}

// BuildContextBlock assembles the XML-tagged context block from gathered system context.
func BuildContextBlock(ctx *uctx.SystemContext, inp *input.ParsedInput) string {
	var b strings.Builder

	b.WriteString("<context>\n")

	// System info — always present.
	b.WriteString("<system>\n")
	fmt.Fprintf(&b, "os: %s\n", ctx.OS)
	fmt.Fprintf(&b, "arch: %s\n", ctx.Arch)
	if ctx.Shell != "" {
		fmt.Fprintf(&b, "shell: %s\n", ctx.Shell)
	}
	b.WriteString("</system>\n")

	// CWD listing.
	b.WriteString("\n<cwd>\n")
	fmt.Fprintf(&b, "path: %s\n", ctx.CWD)
	if len(ctx.DirListing) > 0 {
		for _, entry := range ctx.DirListing {
			fmt.Fprintf(&b, "  %s\n", entry)
		}
	}
	b.WriteString("</cwd>\n")

	// Git — only if in a repo.
	if ctx.GitBranch != "" {
		b.WriteString("\n<git>\n")
		fmt.Fprintf(&b, "branch: %s\n", ctx.GitBranch)
		if ctx.GitStatus != "" {
			fmt.Fprintf(&b, "status:\n%s\n", ctx.GitStatus)
		} else {
			b.WriteString("status: clean\n")
		}
		if ctx.GitRemote != "" {
			fmt.Fprintf(&b, "remote: %s\n", ctx.GitRemote)
		}
		b.WriteString("</git>\n")
	}

	// Project type.
	if ctx.ProjectType != "" {
		fmt.Fprintf(&b, "\n<project type=%q marker=%q />\n", ctx.ProjectType, ctx.ProjectFile)
	}

	// Tool hints.
	if len(inp.ToolHints) > 0 {
		b.WriteString("\n<tool_hints>\n")
		fmt.Fprintf(&b, "The user wants to use: %s\n", strings.Join(inp.ToolHints, ", "))
		b.WriteString("</tool_hints>\n")
	}

	b.WriteString("</context>")
	return b.String()
}

// BuildUserMessage returns the user-facing message to send to the backend.
func BuildUserMessage(inp *input.ParsedInput) string {
	msg := inp.Query
	if inp.SupplementaryPrompt != "" {
		msg += "\n\nAdditional context: " + inp.SupplementaryPrompt
	}
	return msg
}

// BuildRetryMessage creates the retry prompt when the model returned malformed JSON.
func BuildRetryMessage(malformedResponse string) string {
	return fmt.Sprintf(`Your previous response was not valid JSON. Here is what you returned:

---
%s
---

Respond with a single valid JSON object matching the schema. Nothing else.`, malformedResponse)
}
