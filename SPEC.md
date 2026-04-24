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
Underdash should support both implicit and explict tool hints.

#### Implicit: no indicator.

Examples:
- `_ curl endpoint with bearer token $TOKEN`: should generate a command to call `curl`.
- `_ show large files`: it does not specify which tool to use or how many files should be shown, so the response should be an educated guess with reasonable assumptions, calling a tool / a combination of tools / a one-off script that shows the large files under cwd.


#### Explicit hints - "this is important":
`:` marks the tool(s) to be called. Examples:
- `_ :curl endpoint url with bearer token $TOKEN`: `curl` is the tool to call.
- `_ :curl endpoint url with bearer token $TOKEN then :sort the result`: `curl` and `sort` are the tools to call.

#### Explicit hints - "these are just prompt":
`--` indicates all following words are just prompt. Examples:
- `_ --curl failed when getting endpoint, try something else`: Explicitly notify Underdash that *no* words after `--` should be treated as any tools to call.
- `_ :echo "text" prints "text" followed by a newline -- how to make echo print no new line?`: Explicitly notify Underdash that `echo` is the tool to call, and all words come after `--` should be treated as a supplementary prompt.


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

If both `api_key` and `env_key` are set, the environment variable takes precedence. Underdash should warn at startup if the config file containing an `api_key` is world-readable.

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

## Implementation Details

Programming language is Go. Build one executable that does everything.

Makes a tiny TUI under the current line showing the status.
