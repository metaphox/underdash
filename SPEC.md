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


### File Attachments

A query token of the form `@<path>` attaches a local file to the request so the
model can see its contents (e.g. `_ explain this error @screenshot.png`,
`_ summarize @report.pdf`). Attachment handling is deliberate and guarded:

- **Recognition.** A token is treated as an attachment only when it begins with
  `@` *and* the remainder resolves to an existing readable file. Tokens that look
  like `@`-references but do not point at a file (an `@mention`, an email address)
  are left verbatim in the query, so ordinary prompts are never broken.
- **Type guard.** The file type is detected by extension first, then by content
  sniffing. Only an allowlist is accepted: images (`png`, `jpeg`, `gif`, `webp`),
  PDF documents, and UTF-8 text files. Anything else is rejected with a clear
  error naming the file and detected type.
- **Size guard.** Each file must be within a per-file byte cap (default 5 MiB,
  configurable via `attach.max_bytes`) — kept well under the provider's
  per-request limit. Oversized files are rejected.
- **Encoding.** Binary files (images, PDFs) are MIME/base64-encoded; text files
  are passed through as UTF-8. Validation and encoding happen locally, *before*
  any backend call, so an unreadable, oversized, or unsupported file fails fast
  without spending a request.

Attachments are delivered as provider content blocks rather than inlined into the
prompt text:

- **`claude`** — full support: images and PDFs become base64 `image` / `document`
  content blocks; text files become `text` documents. The encoded attachments are
  placed before the user's instruction in the message content.
- **`openai` / `local` / `http`** — images only, sent as base64 `image_url` data
  URIs; text files are inlined into the message. PDF/document attachments are
  rejected with a clear error, since the Chat Completions protocol cannot carry
  them.
- **`stdout`** — prints an attachment summary (name, kind, media type, size)
  instead of transmitting anything, keeping the debug path informative.

Attachments require a backend model with the matching capability (vision for
images, document support for PDFs); when the configured model lacks it, the
provider returns a clear error.


### Non-LLM Inference layer

Based on a quick skim of user input, Underdash should inspect the system *conditionally*, for:
- current working directory contents
- git repo presence
- environment variables (security risk - keys or secrets in environment variables should NOT to be transmitted to the backend)

### Backend Abstraction

Underdash supports multiple named backends. Each backend is defined in `config.yaml` and one is designated as the default. A backend entry includes:

- `type`: one of `claude`, `openai`, `local`, `http`, `stdout`
- `model`: model identifier (e.g., `claude-sonnet-4-20250514`, `gpt-4o`) — **optional**. When omitted, Underdash discovers an available model from the provider's `/v1/models` on first use (you pick interactively on a TTY, or it auto-selects a sensible default when non-interactive), then writes the choice back to this config so subsequent runs are stable. Model IDs are never hardcoded, since providers retire them; if a saved model is later retired, the next run self-heals by re-discovering and updating the config. If the config location is not writable, Underdash uses the discovered model for that run and warns that it could not persist it.
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

The `openai`, `local`, and `http` backends all speak the OpenAI-compatible Chat Completions protocol and share a single implementation. They differ only in defaults: `openai` defaults to the public OpenAI endpoint and **requires** an API key; `local` and `http` **require** an explicit `endpoint` (e.g. an Ollama server at `http://localhost:11434/v1`) and treat the API key as optional, since many local servers need none. `local` is simply the conventional name for a localhost model server. An `endpoint` may be given as a base URL ending in `/v1` — the `/chat/completions` path is appended automatically — or as a full chat/completions URL.

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
  markdown: true    # render explanation output as Markdown on a TTY (default: true)
```

**`streaming` mode (default):**
- While waiting for the backend: a single-line spinner showing live status, e.g.
  `⠋ Thinking 12s · claude/opus-4-8 · ctx: git, go, 14 tools, 2 files`. The fields are:
  - **elapsed time** since the request started (counts up; there is no hard total
    timeout, so this is elapsed rather than a countdown);
  - the **backend/model** the request is going to (`<backend>/<model>`);
  - a **`ctx:` summary** of which local context signals were sent (git, project
    type, tool count, history, attachment count) — omitted when nothing was gathered.
- A **`· timeout in <n>s`** countdown is appended only during the final 10 seconds
  before the response (first-byte) timeout, as an early warning that the backend has
  not responded. It disappears the moment the backend's response headers arrive.
- As tokens arrive: streamed to the terminal in real time.
- **`explanation`-type responses are rendered as Markdown** (headings, bold,
  lists, inline code) with color and layout, via the `glamour` library (the engine
  behind [Glow](https://github.com/charmbracelet/glow)), auto-styling for the
  terminal's dark/light background and wrapping to its width. Disable with
  `output.markdown: false` to print raw Markdown even on a TTY.
- Final command output is printed cleanly after execution.

**`plain` mode:**
- No spinner, no ANSI escape codes.
- Waits for the full response, then prints it to stdout.
- Explanation output is the **raw Markdown** (no rendering), so it stays clean and
  pipe-friendly.
- Suitable for piping into other commands (e.g., `_ show large files --output plain | head`).

Both modes automatically detect when stdout is not a TTY and fall back to plain
output (including raw, unrendered explanations).

### Error Handling & Audit

Underdash is a one-shot tool, so failures must be legible at a glance, and any command it runs on the user's behalf must be accountable after the fact. This section defines how errors are surfaced, what verbose mode exposes, and what is recorded to the audit log.

#### Error Surfaces & Exit Codes

Every error is reduced to a short, human-readable line on `stderr` — never a raw stack trace or an unstructured provider payload. Errors fall into a small set of categories with stable exit codes:

| Category                  | Examples                                            | Exit code            |
|---------------------------|-----------------------------------------------------|----------------------|
| Success                   | command ran; explanation printed                    | 0                    |
| Usage error               | unknown flag; invalid `--output` value              | 2                    |
| Config error              | missing API key; unreadable config; unknown backend | 1                    |
| Backend error             | HTTP 4xx/5xx; malformed stream; invalid model       | 1                    |
| Network / timeout         | unreachable endpoint; no response headers in time   | 1                    |
| Response error            | model returned non-JSON after all retries           | 1                    |
| Policy denial             | generated command matched a `deny` pattern          | 1                    |
| Executed command failed   | generated command itself exited non-zero            | the command's code   |
| Cancelled                 | user pressed Ctrl+C (SIGINT/SIGTERM)                | 130                  |

Exit codes are part of the tool's contract and must remain stable so scripts can branch on them.

#### Friendly Error Output

Backend and network errors carry structure rather than a flattened string. The canonical rendering is a summary line plus dimmed detail lines:

```
error: not found (claude, HTTP 404)
  model: claude-sonnet-4-...
  request id: req_011CcLF9...
  hint: the model may be invalid or retired — verify backends.claude.model
```

- The **summary** humanizes the provider's machine error type (e.g. `not_found_error` → `not found`).
- The **hint** offers status-specific remediation (401 → check API key; 404 → model/endpoint may be wrong; 429 → rate limited, retry; 5xx → provider issue, retry).
- The raw provider payload is preserved only as a fallback, shown when the error envelope cannot be parsed.

Timeouts and cancellation get their own concise lines (`backend timed out — …`, `cancelled`) instead of a generic failure. In TTY mode the summary is colorized and details are dimmed; in plain / non-TTY mode all decoration is dropped.

#### Verbose Mode

`--verbose` / `-v` raises diagnostic output on `stderr` without changing what lands on `stdout`, so pipes stay clean. It is additive to any output mode. (`--version` is the long form only; it does not share the `-v` shorthand.)

Verbose output includes:

- Resolved backend, model, and endpoint.
- Which context signals were gathered or skipped (e.g. `git: yes`, `history: off`).
- The fully assembled prompt actually sent (the same content the `stdout` backend prints).
- Request timing and, when the backend reports it, token usage.
- Each retry attempt and the reason it fired.
- Risk classification and the policy decision for any generated command.

Secrets are never emitted, even in verbose mode: API keys, `Authorization` / `x-api-key` headers, and environment-variable *values* are redacted. A deeper, development-only level for protocol tracing (raw SSE frames, HTTP headers) is gated behind `UNDERDASH_DEBUG=1`.

#### Audit Log

Because Underdash can execute shell commands, it can optionally keep an account of what it did. The audit log is **opt-in** — it may capture prompt text and command lines containing sensitive data — and is configured in `config.yaml`:

```yaml
audit:
  enabled: true
  path: ~/.local/state/underdash/audit.jsonl   # default location when enabled
  max_size: 10MB                                # front-truncated when exceeded
```

Each invocation appends one JSON object per line (JSONL):

```json
{
  "ts": "2026-06-24T10:15:30Z",
  "query": "tar the newest directory",
  "backend": "claude",
  "model": "claude-sonnet-4-...",
  "response_type": "command",
  "command": "tar -czf newest.tgz ./latest",
  "risk": "confirm",
  "action": "executed",
  "exit": 0,
  "duration_ms": 1840
}
```

Rules:

- **Complete account.** Errors and policy denials are logged too (`action` is one of `executed`, `dry-run`, `denied`, `cancelled`, `declined`; an `error` field is added on failure), so the log records attempts, not just successes.
- **Redaction.** Environment-variable values and configured API keys are never written. The verbatim query and generated command *are* recorded — these may still contain secrets the user typed, which is exactly why the log is opt-in.
- **Bounded.** The log is append-only and front-truncated past `audit.max_size`, so it cannot grow without limit.
- **Non-fatal.** An unwritable audit path produces a single startup warning and never blocks the primary task.

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

{{if .Attachments}}
<attachments>
{{range .Attachments}}  {{.Filename}} ({{.Kind}}, {{.MediaType}})
{{end}}</attachments>
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
- `<attachments>` names each `@<path>` file (filename, kind, media type) so the model knows what the accompanying content blocks are; the file *contents* travel as provider content blocks, not inside this text.

#### User Message

The user's original natural-language input, verbatim (after stripping the `--` syntax marker that has already been extracted into tool hints, and any `@<path>` tokens that resolved to attachments).

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
