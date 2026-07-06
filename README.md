# Underdash

**Ask an LLM for shell help, one line at a time — without leaving your terminal.**

Underdash is a non-interactive CLI coding agent. You describe what you want in plain
language; it reads your local context, asks an LLM, and gives you back a ready-to-run
command, a script, or a straight answer. It is built to be aliased to `_` (the
underscore), so help is always one keystroke-ish away:

```bash
_ tar the newest directory
_ tell me what this repository does
_ undo my last git commit but keep the changes
```

If pressing **Shift+Minus** is still too much effort, alias it to `u` instead.

---

## Why

Sometimes you fire up a full agent just to fix something tiny. Sometimes you just
forgot which flag `tar` or `curl` wants and you don't feel like leaving the shell to
search for it. `codex exec "..."` or `claude -p "..."` is a lot of typing for that.
Underdash is the short path: one prefix character, a sentence, an answer — and, when
it's a command, the option to run it on the spot.

---

## How it works

For each prompt, Underdash:

1. **Gathers local context** — OS, architecture, shell, a short listing of the current
   directory, git status/branch/recent commits, the detected project type, and notable
   tools on your `$PATH`. (Shell history is included only if you opt in.)
2. **Asks the backend** for a structured answer: a one-line **command**, a multi-line
   **script**, or a plain-language **explanation**.
3. **Classifies the risk** of any command and decides whether to auto-run it, ask you
   first, warn you, or refuse outright.
4. **Runs it** (after confirmation when needed) in your shell, streaming output straight
   through — or just prints the explanation.

Because it sees your working directory, git state, and project type, answers are
tailored to where you actually are. `_ run the tests` does the right thing in a Go repo
and a Node repo without you spelling it out.

---

## Install

Requires Go 1.26+.

```bash
git clone https://github.com/metaphox/underdash
cd underdash/src
go build -o underdash .
```

Put the binary somewhere on your `$PATH`, then alias it:

```bash
# ~/.zshrc or ~/.bashrc
alias _='/path/to/underdash'
```

Set an API key for your backend (Claude is the default):

```bash
export ANTHROPIC_API_KEY=sk-ant-...
# or, for OpenAI:
export OPENAI_API_KEY=sk-...
```

A `.env` next to the binary or a key in the config file work too — see
[API keys](#api-keys) for exactly where each one goes.

On first use of a remote backend, Underdash prints exactly what it transmits and asks
you to acknowledge it once. See [Privacy](#privacy).

---

## Usage

```
_ [flags] <your request in plain words>
```

Everything that isn't a flag becomes your prompt. There are **no subcommands** — `_ help`
is a question you're asking the model, not a built-in command. For the real flag list,
use `_ --help`.

### Examples

```bash
# Get and (optionally) run a command
_ find the 10 largest files under this directory

# Ask a question — answered as prose, nothing is executed
_ what does the -z flag do in tar

# Preview without running
_ --dry-run delete every node_modules folder under here

# Skip confirmations for a safe, scripted run
_ -y compress all the logs in ./var

# Prompts that start with a dash: put them after --
_ -- --version means what for ffmpeg
```

### Response types

| Type | What you get | Default behavior |
|------|--------------|------------------|
| **command** | a single shell one-liner | run after a risk check (auto / confirm / warn / deny) |
| **script** | a multi-line script with a shebang | always shown and confirmed before running |
| **explanation** | a plain-language answer | printed (rendered as Markdown), nothing runs |

---

## Special prompt syntax

| Token | Meaning |
|-------|---------|
| `@path/to/file` | **Attach a file.** Images (PNG/JPEG/GIF/WebP), PDFs, and text are sent to the model. Only applied when the path is a real file — otherwise `@something` stays literal text. |
| `:toolname` | **Tool hint.** Nudges the model toward a specific tool, e.g. `_ :jq pull the names out of data.json`. Only recognized before a `--`; after the separator a `:word` stays literal text. |
| `--` | **Literal separator.** Everything after it is treated as prompt text, so you can write prompts that start with a dash. |

```bash
_ explain what this diagram shows @architecture.png
_ :rg find every TODO that mentions auth
```

---

## Flags

| Flag | Description |
|------|-------------|
| `-b, --backend <name>` | Backend to use (overrides the config default). |
| `-n, --dry-run` | Show the generated command without executing it. |
| `--no-exec` | Alias for `--dry-run`. |
| `-y, --yes` | Skip all confirmations (cannot bypass a **denied** command). |
| `--output <mode>` | Output mode: `streaming` (default) or `plain`. |
| `-v, --verbose` | Print diagnostics (context, prompt, timing) to stderr. |
| `--version` | Print the version and the full data-disclosure notice. |
| `--sysinfo` | Print the gathered local context and exit, without calling any backend. |
| `--init` | Write a default, annotated config file and exit (prompts before overwriting; `--yes` forces). |
| `--config <path>` | Use an alternate config file. |

`--sysinfo` is the honest way to see exactly what context would be sent before you send
anything.

---

## Backends

Underdash is backend-agnostic. Configure one or more under `backends.<name>` and pick a
default.

| Type | Use |
|------|-----|
| `claude` | Anthropic's API (the default). Needs `ANTHROPIC_API_KEY`. |
| `openai` | OpenAI's API. Needs `OPENAI_API_KEY`. |
| `local` / `http` | Any OpenAI-compatible HTTP endpoint — Ollama, vLLM, LM Studio, a gateway. Needs an `endpoint`; API key optional. |
| `stdout` | Debug backend: prints the assembled prompt instead of calling a model. |

**Model auto-discovery.** If you don't pin a model, Underdash queries the provider's
`/models` endpoint, ranks the results by durable family keywords (never hardcoded,
soon-to-be-retired IDs), picks the best default, and saves it to your config. If a
pinned model later 404s mid-request, Underdash re-discovers a replacement and retries —
no manual config surgery.

---

## Configuration

Config lives at `~/.config/underdash/config.yaml`. Precedence is
**CLI flags > environment variables (`UNDERDASH_*`) > config file**.

```yaml
default_backend: claude

backends:
  claude:
    type: claude
    # model: omit to auto-discover and pin
    env_key: ANTHROPIC_API_KEY   # which env var holds the key
    # api_key_file: ~/.secrets/llm-keys   # or read the key from a file
    # api_key: sk-ant-...        # or paste the key here (chmod 600 the file)
  local:
    type: local
    endpoint: http://localhost:11434/v1   # e.g. Ollama
    model: llama3

# Execution policy — glob patterns override the built-in risk rules.
execution:
  auto_run:               # always run without asking
    - "docker ps *"
    - "kubectl get *"
  confirm:                # always ask first
    - "git push *"
  deny:                   # never run, even with -y
    - "rm -rf /*"

# Output
output:
  mode: streaming         # or plain
  markdown: true          # render explanations as Markdown (default true)

# Extra context (off by default)
context:
  history: false          # include recent shell history in the prompt
  history_lines: 20

# File attachments
attach:
  max_bytes: 5242880      # per-file cap (5 MiB default)

# Audit log (off by default)
audit:
  enabled: false
  path: ~/.local/state/underdash/audit.jsonl
  max_size: 10MB
```

### Environment variables

Every config key has an environment-variable equivalent: prefix `UNDERDASH_`, uppercase,
and replace each `.` (and `-`) with `_`. So `output.mode` becomes `UNDERDASH_OUTPUT_MODE`
and `backends.claude.model` becomes `UNDERDASH_BACKENDS_CLAUDE_MODEL`. Environment
variables sit between CLI flags and the config file in precedence.

| Variable | Overrides | Example |
|----------|-----------|---------|
| `UNDERDASH_DEFAULT_BACKEND` | `default_backend` | `claude` |
| `UNDERDASH_BACKENDS_<NAME>_TYPE` | `backends.<name>.type` | `claude`, `openai`, `local`, `http`, `stdout` |
| `UNDERDASH_BACKENDS_<NAME>_MODEL` | `backends.<name>.model` | `claude-opus-4-8` |
| `UNDERDASH_BACKENDS_<NAME>_ENDPOINT` | `backends.<name>.endpoint` | `http://localhost:11434/v1` |
| `UNDERDASH_BACKENDS_<NAME>_API_KEY` | `backends.<name>.api_key` | `sk-ant-...` |
| `UNDERDASH_BACKENDS_<NAME>_API_KEY_FILE` | `backends.<name>.api_key_file` | `~/.secrets/llm-keys` |
| `UNDERDASH_BACKENDS_<NAME>_ENV_KEY` | `backends.<name>.env_key` | `ANTHROPIC_API_KEY` |
| `UNDERDASH_OUTPUT_MODE` | `output.mode` | `streaming`, `plain` |
| `UNDERDASH_OUTPUT_MARKDOWN` | `output.markdown` | `true`, `false` |
| `UNDERDASH_EXECUTION_AUTO_RUN` | `execution.auto_run` | `"docker ps *"` (see caveat) |
| `UNDERDASH_EXECUTION_CONFIRM` | `execution.confirm` | `"git push *"` (see caveat) |
| `UNDERDASH_EXECUTION_DENY` | `execution.deny` | `"rm -rf /*"` (see caveat) |
| `UNDERDASH_CONTEXT_HISTORY` | `context.history` | `true`, `false` |
| `UNDERDASH_CONTEXT_HISTORY_LINES` | `context.history_lines` | `20` |
| `UNDERDASH_AUDIT_ENABLED` | `audit.enabled` | `true`, `false` |
| `UNDERDASH_AUDIT_PATH` | `audit.path` | `~/.local/state/underdash/audit.jsonl` |
| `UNDERDASH_AUDIT_MAX_SIZE` | `audit.max_size` | `10MB` |
| `UNDERDASH_ATTACH_MAX_BYTES` | `attach.max_bytes` | `5242880` |

`<NAME>` is the backend's config name (e.g. `CLAUDE`, `OPENAI`, `LOCAL`), so multiple
backends each get their own set.

> **Caveat — list values.** The `execution.*` keys are lists, and a value set via an
> environment variable is split on whitespace. That shreds patterns containing spaces
> (`UNDERDASH_EXECUTION_DENY="rm -rf /*"` becomes three entries), so set glob patterns
> that contain spaces in the config file instead.

Beyond these, `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` (or any name you point `env_key` at)
supply the API key without the `UNDERDASH_` prefix — see [API keys](#api-keys).

### API keys

For a given backend, Underdash resolves the key from these sources, in order (the first
one that yields a value wins):

1. **Environment variable** (recommended). Each backend type has a default var —
   `ANTHROPIC_API_KEY` for `claude`, `OPENAI_API_KEY` for `openai`. Set it in your
   shell profile so it's exported for every session:

   ```bash
   # ~/.zshrc or ~/.bashrc
   export ANTHROPIC_API_KEY=sk-ant-...
   ```

   To read from a differently-named variable, point `env_key` at it
   (e.g. `env_key: MY_TEAM_KEY` under `backends.<name>`). A `.env` next to the binary
   feeds this same source — see [.env loading](#env-loading) below.

2. **A key file** via `backends.<name>.api_key_file`, a path (a leading `~/` is
   expanded). Keeps the secret out of the config file itself. The file is either:

   - **just the key** on its own:

     ```
     sk-ant-...
     ```

   - or **`NAME=value` pairs** (dotenv style, `export` and quotes allowed), so a single
     file can hold keys for several backends. The entry named by the backend's
     `env_key` is used:

     ```
     ANTHROPIC_API_KEY=sk-ant-...
     OPENAI_API_KEY=sk-...
     ```

   ```yaml
   backends:
     claude:
       type: claude
       api_key_file: ~/.secrets/llm-keys   # chmod 600 it
   ```

   A configured `api_key_file` that can't be read, or that has no entry matching
   `env_key`, is a hard error — a broken path fails loudly rather than silently
   falling through.

3. **Inline in the config file** via `backends.<name>.api_key`. Least safe, since the
   key sits in the config in plaintext — `chmod 600 ~/.config/underdash/config.yaml`.
   Underdash warns you if that file contains API keys and is world-readable.

`local` / `http` backends usually need no key; supply one the same way only if your
endpoint requires it.

#### .env loading

At startup, before reading the config, Underdash loads a `.env` file from the directory
of the real binary (symlinks are resolved, so the `_` alias still finds it). Each
`KEY=VALUE` line becomes an environment variable, feeding source 1 above — a handy place
to keep keys without exporting them from your shell profile:

```bash
# /path/to/.env  (same directory as the underdash binary)
ANTHROPIC_API_KEY=sk-ant-...
```

Variables already set in your shell are **not** overridden, and a missing or malformed
`.env` is ignored — it's a convenience, not a requirement.

---

## Safety

Every generated command is classified before it runs:

- **Safe** (read-only / low impact — `ls`, `cat`, `git status`, …) runs automatically.
- **Confirm** (anything not known-safe) prompts `y/N` first.
- **Dangerous** (`rm -rf`, `sudo`, `dd`, `curl | sh`, fork bombs, writes to `/etc`, …)
  prints a warning and requires explicit confirmation.
- **Denied** (matched by your `execution.deny` list) never runs — not even with `-y`.

Risk classification looks inside pipes, `&&`/`||`/`;` chains, command substitutions, and
backticks, so a dangerous command can't hide behind a safe-looking wrapper. **Scripts
are always treated as dangerous** and shown in full before execution.

---

## Privacy

When the backend is remote (`claude` / `openai` / `http`), Underdash transmits your
prompt plus the local context described in [How it works](#how-it-works). It **never**
sends API keys or the *values* of environment variables — only the *names* of env vars
your prompt references.

- On first use of a remote backend, the full disclosure is printed and you must
  acknowledge it once (stored at `~/.config/underdash/consent`).
- `_ --version` reprints that disclosure any time.
- `local` and `stdout` backends transmit nothing off your machine.

Audit logging is opt-in. When enabled, each invocation appends one JSON line (query,
backend, model, command, risk, action, exit code, timing) to a size-capped log — useful
precisely because the tool can run shell commands on your behalf.

---

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success. |
| `1` | Error (backend, parsing, denied command, …). |
| `2` | Usage error (bad flag). |
| `130` | Cancelled (Ctrl-C). |
| *other* | The exit code of the command Underdash ran on your behalf. |

---

## Caveats

In **bash**, an unmatched wildcard like `?` or `*` is passed through unchanged, so a
prompt that ends in `?` just works. In **zsh**, the same prompt errors out:

```bash
_ what is the answer to life, the universe, and everything?
zsh: no matches found: everything?
```

This is zsh's safest default. Quote the prompt, drop the trailing `?`, or run
`setopt no_nomatch` to get bash's behavior.

---

## Development

```bash
cd src
go build -o underdash .   # build
go test ./...             # run all tests
go test ./cmd/...         # one package
```
