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

Underdash warns you if your config file contains API keys and is world-readable
(`chmod 600` it).

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
