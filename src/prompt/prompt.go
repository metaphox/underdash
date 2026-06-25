// Package prompt builds prompts from system context and user input.
package prompt

import (
	"fmt"
	"strings"

	"metaphox/underdash/backend"
	"metaphox/underdash/input"
	"metaphox/underdash/sysinfo"
)

const systemPrompt = `You are a shell command assistant. You help users accomplish tasks on their system by producing shell commands, scripts, or explanations.

The user may attach files (images, PDFs, or text), listed in the context's <attachments> section. When a file is attached, use its contents to inform your response; if they are asking about the file rather than requesting an action, respond with type "explanation".

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

// BuildContextBlock assembles the XML-tagged context block from gathered system
// context, the parsed input, and any attachments the user referenced. The
// attachment contents travel as backend content blocks; the <attachments>
// section here names them so the model knows what those blocks are.
func BuildContextBlock(ctx *sysinfo.SystemContext, inp *input.ParsedInput, atts []backend.Attachment) string {
	var b strings.Builder

	b.WriteString("<context>\n")

	// System info — always present.
	b.WriteString("<system>\n")
	fmt.Fprintf(&b, "os: %s\n", ctx.OS)
	fmt.Fprintf(&b, "arch: %s\n", ctx.Arch)
	if ctx.Shell != "" {
		fmt.Fprintf(&b, "shell: %s\n", ctx.Shell)
	}
	if len(ctx.PathTools) > 0 {
		fmt.Fprintf(&b, "tools: %s\n", strings.Join(ctx.PathTools, ", "))
	}
	b.WriteString("</system>\n")

	// CWD listing.
	b.WriteString("\n<cwd>\n")
	fmt.Fprintf(&b, "path: %s\n", ctx.CWD)
	for _, e := range ctx.DirEntries {
		switch e.Type {
		case "dir":
			fmt.Fprintf(&b, "  %s/  dir\n", e.Name)
		case "file":
			fmt.Fprintf(&b, "  %s  file  %s\n", e.Name, formatSize(e.Size))
		default:
			fmt.Fprintf(&b, "  %s  %s\n", e.Name, e.Type)
		}
	}
	if ctx.DirOverflow > 0 {
		fmt.Fprintf(&b, "  ... and %d more\n", ctx.DirOverflow)
	}
	b.WriteString("</cwd>\n")

	// Git — only if in a repo.
	if ctx.InGitRepo {
		b.WriteString("\n<git>\n")
		if ctx.GitBranch != "" {
			fmt.Fprintf(&b, "branch: %s\n", ctx.GitBranch)
		}
		if ctx.GitStatus != "" {
			fmt.Fprintf(&b, "status:\n%s\n", ctx.GitStatus)
		} else {
			b.WriteString("status: clean\n")
		}
		if len(ctx.GitLog) > 0 {
			b.WriteString("recent_commits:\n")
			for _, subject := range ctx.GitLog {
				fmt.Fprintf(&b, "  %s\n", subject)
			}
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

	// Shell history — opt-in.
	if len(ctx.ShellHistory) > 0 {
		b.WriteString("\n<history>\n")
		for _, line := range ctx.ShellHistory {
			fmt.Fprintf(&b, "  %s\n", line)
		}
		b.WriteString("</history>\n")
	}

	// Attachments — names the files whose contents are sent as content blocks.
	if len(atts) > 0 {
		b.WriteString("\n<attachments>\n")
		for _, a := range atts {
			fmt.Fprintf(&b, "  %s (%s, %s)\n", a.Filename, a.Kind, a.MediaType)
		}
		b.WriteString("</attachments>\n")
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
// When only one of query / supplementary text is present (e.g. a prompt that is
// entirely after "--"), it is returned on its own without the framing label.
func BuildUserMessage(inp *input.ParsedInput) string {
	switch {
	case inp.Query == "":
		return inp.SupplementaryPrompt
	case inp.SupplementaryPrompt == "":
		return inp.Query
	default:
		return inp.Query + "\n\nAdditional context: " + inp.SupplementaryPrompt
	}
}

// formatSize renders a byte count in a compact human-readable form.
func formatSize(n int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.1fG", float64(n)/float64(gb))
	case n >= mb:
		return fmt.Sprintf("%.1fM", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%.1fK", float64(n)/float64(kb))
	default:
		return fmt.Sprintf("%dB", n)
	}
}

// BuildRetryMessage creates the retry prompt when the model returned malformed JSON.
func BuildRetryMessage(malformedResponse string) string {
	return fmt.Sprintf(`Your previous response was not valid JSON. Here is what you returned:

---
%s
---

Respond with a single valid JSON object matching the schema. Nothing else.`, malformedResponse)
}
