# Specification

## Design Concepts

Intended workflow:

1. parse a short natural-language fragment
2. infer likely command family and local context, *without a model*
3. build structured prompt or execution request
4. send it to a backend
5. present or run the result under a strict policy

Underdash should not only be a prompt-perparer, it asks an agent and can execute the result after examination / confirmation.

Backend abstraction: the prepared prompt can be send to a abstracted backend which can be resolved to:
    - print to stdout
    - call a remote model like Claude / OpenAI
    - call a local model
    - call a custom HTTP endpoint

Underdash is designed to be a shell assistant for one-shot tasks. This means each execution should try to limit the number of requests to a minimum, but not necessarily only one, during one invocation. Invocations are "stateless", so there is no built-in concept of sessions or context management between each invocation.

### Tool Hints
Underdash infers the tools to call from the natural-language input.

#### Implicit: no indicator.

Examples:
- `_ curl endpoint with bearer token $TOKEN`: should generate a command to call `curl`.
- `_ show large files`: it does not specify which tool to use or how many files should be shown, so the response should be an educated guess with reasonable assumptions, calling a tool / a combination of tools / a one-off script that shows the large files under cwd.

#### Prompt-only marker - "these are just prompt":
`--` indicates all following words are just prompt. Examples:
- `_ --curl failed when getting endpoint, try something else`: Explicitly notify Underdash that *no* words after `--` should be treated as any tools to call.
- `_ echo "text" prints "text" followed by a newline -- how to make echo print no new line?`: All words after `--` are treated as a supplementary prompt.


### Non-LLM Inference layer

Based on a quick skim of user input, Underdash should inspect the system *conditionally*, for:
- current working directory contents
- git repo presence
- environment variables (security risk - keys or secrets in environment variables should NOT to be transmitted to the backend)

### Backend Abstraction

Underdash supports multiple named backends. Each backend is defined in `config.yaml` and one is designated as the default. A backend entry includes:

- `type`: one of `claude`, `openai`, `local`, `http`, `stdout`
- `model`: model identifier (e.g., `claude-sonnet-4-20250514`, `gpt-4o`)
- `endpoint`: API base URL (required for `http` and `local`; optional override for `claude`/`openai`)
- `api_key`: API key string (optional — environment variables are preferred)
- `env_key`: name of the environment variable holding the API key (e.g., `ANTHROPIC_API_KEY`)

If both `api_key` and `env_key` are set, the environment variable takes precedence. When `env_key` is not configured, each backend type falls back to a conventional default — `claude` reads `ANTHROPIC_API_KEY`, `openai` reads `OPENAI_API_KEY` — so the env var works without any config file. Underdash should warn at startup if the config file containing an `api_key` is world-readable.

#### .env Loading

At startup, before reading the YAML config, Underdash looks for a `.env` file in the directory of the running binary (resolving symlinks first, so it finds the real install location even when invoked via the `_` alias) and loads any `KEY=VALUE` pairs into the process environment.

- Existing environment variables are **not** overridden — values already present in the shell win.
- A missing or unreadable `.env` is silently ignored; it is not required.
- Parse errors are non-fatal: Underdash continues without the variables it could not load.
- This file is the recommended place to keep API keys (e.g., `ANTHROPIC_API_KEY=...`) so that backends configured with `env_key` resolve without polluting the user's shell profile.

Example config:

```yaml
default_backend: claude

backends:
  claude:
    type: claude
    model: claude-sonnet-4-20250514
    env_key: ANTHROPIC_API_KEY
  openai:
    type: openai
    model: gpt-4o
    env_key: OPENAI_API_KEY
  local:
    type: local
    model: llama3
    endpoint: http://localhost:11434/v1
  stdout:
    type: stdout   # just prints the assembled prompt — useful for debugging
```

The backend can be overridden per invocation with `--backend <name>` (or `-b`).

The `stdout` backend prints the fully assembled prompt to stdout without calling any model — useful for debugging and piping into other tools.

### Execution Policy

When underdash produces a shell command, the following policy determines what happens next:

1. **Risk classification.** Every generated command is checked against a risk model:
   - **safe** — read-only or low-impact commands (e.g., `ls`, `cat`, `grep`, `git status`).
   - **confirm** — commands that modify state (e.g., `mv`, `cp`, `git commit`).
   - **dangerous** — destructive or privileged commands (e.g., `rm -rf`, `sudo`, `chmod 777`, `mkfs`, pipes to `sh`).

2. **Default behavior by risk level:**
   - safe → auto-execute and print output.
   - confirm → show the command and ask `y/n` before running.
   - dangerous → show the command with a warning, ask `y/n`, require explicit confirmation.

3. **Built-in defaults + user overrides.** Underdash ships with a hardcoded set of patterns for each risk level. Users can extend or override these in `config.yaml`:

```yaml
execution:
  # Override risk levels with glob/regex patterns
  auto_run:
    - "git log *"
    - "docker ps *"
  confirm:
    - "docker run *"
  deny:
    - "rm -rf /"
```

4. **Global overrides via flags:**
   - `--dry-run` / `-n`: never execute, just print the command.
   - `--yes` / `-y`: skip all confirmations (auto-run everything).
   - `--no-exec`: equivalent to `--dry-run`.

### Non-LLM Inference Layer

Before sending anything to a backend, underdash performs a fast, local inspection pass. The goal is to enrich the prompt with relevant context so the model produces better results, while keeping latency and token cost low.

**Context signals gathered (conditionally):**

| Signal               | When gathered                              | What's collected                                                       |
|----------------------|--------------------------------------------|------------------------------------------------------------------------|
| Directory listing    | Always                                     | Top-level entries in cwd (name, type, size), capped at a reasonable N  |
| Git metadata         | When `.git/` exists in cwd or ancestors    | Branch name, short status, last 3 commit subjects, remote URL          |
| OS / shell info      | Always                                     | `GOOS`/`GOARCH`, `$SHELL`, notable tools on `$PATH` (e.g., docker, python, node) |
| Project type         | When marker files exist                    | Detected from `go.mod`, `package.json`, `Cargo.toml`, `pyproject.toml`, `Makefile`, etc. |
| Shell history        | **Opt-in only** (`context.history: true`)  | Last N commands from `~/.bash_history` / `~/.zsh_history` / etc.       |

**Security rules:**
- Environment variable *values* are never sent to the backend. Only variable *names* are included when they appear relevant to the query.
- Shell history is opt-in because it may contain secrets typed on the command line.
- The context payload is always visible when using the `stdout` backend.

### Output & TUI

Underdash's output mode is configurable via `config.yaml` or the `--output` flag:

```yaml
output:
  mode: streaming   # or "plain"
```

**`streaming` mode (default):**
- While waiting for the backend: a single-line spinner below the cursor showing status (e.g., `⠋ thinking...`).
- As tokens arrive: streamed to the terminal in real time.
- Final command output is printed cleanly after execution.

**`plain` mode:**
- No spinner, no ANSI escape codes.
- Waits for the full response, then prints it to stdout.
- Suitable for piping into other commands (e.g., `_ show large files --output plain | head`).

Both modes automatically detect when stdout is not a TTY and fall back to plain output.

### Prompt Template

The prompt sent to the backend is assembled from fixed and dynamic parts. The full structure is:

```
[System Prompt]
[Context Block]
[User Message]
```

#### System Prompt

```
You are a shell command assistant. You help users accomplish tasks on their system by producing shell commands, scripts, or explanations.

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
- Do not wrap the JSON in a code fence or any other formatting.
```

#### Context Block

Injected as a system-level or user-level preamble depending on backend capabilities. Template:

```
<context>
<system>
os: {{.OS}}
arch: {{.Arch}}
shell: {{.Shell}}
</system>

<cwd>
path: {{.Cwd}}
{{.DirListing}}
</cwd>

{{if .Git}}
<git>
branch: {{.GitBranch}}
status: {{.GitStatus}}
recent_commits:
{{.GitLog}}
remote: {{.GitRemote}}
</git>
{{end}}

{{if .ProjectType}}
<project type="{{.ProjectType}}" marker="{{.ProjectMarker}}" />
{{end}}

{{if .History}}
<history>
{{.RecentHistory}}
</history>
{{end}}

{{if .ToolHints}}
<tool_hints>
{{.ToolHints}}
</tool_hints>
{{end}}
</context>
```

**Notes on context assembly:**
- XML-style tags are used as structured delimiters (not actual XML — no need to escape content).
- Sections are omitted entirely when not available (e.g., no `<git>` block outside a repo).
- `<tool_hints>` is populated from the parser: the `--` separator becomes "Everything after the marker is supplementary context, not a tool request."

#### User Message

The user's original natural-language input, verbatim (after stripping the `--` syntax marker that has already been extracted into tool hints).

```
{{.UserInput}}
```

### Retry Prompt

When the backend returns invalid JSON, underdash retries with the following appended to the conversation:

```
Your previous response was not valid JSON. Here is what you returned:

---
{{.MalformedResponse}}
---

Respond with a single valid JSON object matching the schema. Nothing else.
```

**Retry rules:**
- Maximum 3 retries (4 total attempts).
- If the response does not start with `{` (clearly not JSON), bail immediately with an error message instead of retrying.
- Each retry includes the malformed response so the model can self-correct.
- On final failure, display the raw response to the user as a fallback and exit with a non-zero status.

## Implementation Details

Programming language is Go. Build one executable that does everything.

Makes a tiny TUI under the current line showing the status.
