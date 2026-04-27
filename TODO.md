# TODO — Spec Gaps to Resolve

## High Priority (core functionality)

- [x] **Prompt Assembly**: Defined — system prompt + context block (XML-tagged) + user message. JSON output format.
- [x] **Response Parsing**: Defined — JSON schema with `command/explanation/script` types. Smart retry on malformed JSON (max 3), bail on clearly non-JSON.
- [x] **Output Type Detection**: Defined — model returns `type` field in JSON. Underdash dispatches on it.

## Medium Priority (correctness & safety)

- [ ] **Tool Hint Parsing Grammar**: Formalize `:tool` and `--` syntax — mid-sentence `:`, escaping, disambiguation from CLI `--` flags (Cobra interaction), conflicting usage.
- [x] **Variable / Environment Expansion**: Defined — always pass `$VAR` literally, never expand. Model is instructed to preserve variable references.
- [ ] **Execution Policy Edge Cases**: Risk classification for compound commands (`&&`, pipes, `$()`), whether `deny` blocks entirely or still allows override, whether `--yes` bypasses `deny`.
- [ ] **Error Handling**: Backend unreachable / timeout / rate-limit, invalid API key, generated command fails (non-zero exit), Ctrl+C mid-stream behavior, retry policy.
- [ ] **First-Run Privacy Acknowledgment**: When a non-local backend (`claude`, `openai`, `http`) is configured, prompt the user once to acknowledge that local context will be uploaded to a third party. Persist the acknowledgment (e.g. `consent: true` in config, or a marker file under `~/.config/underdash/`) so it never appears again. Skip entirely for `stdout` and `local` backends. The full disclosure must also be printable via `underdash --version` / `-v` so users can re-read it without resetting consent.

  Disclosure should enumerate everything that may be transmitted:
  - OS, architecture, shell name (`$SHELL`)
  - Current working directory **path** (absolute, may reveal username and project names)
  - Top-level directory listing — filenames only, capped at 20
  - Git metadata when inside a repo: branch name, short `git status` (includes paths of modified / untracked files), `origin` remote URL (may include host and username)
  - Detected project type and marker file (`go.mod`, `package.json`, …)
  - Recent commit subjects (planned, per SPEC)
  - Notable tools on `$PATH` (planned, per SPEC)
  - Shell history — opt-in only, never sent unless `context.history: true`
  - Names of environment variables referenced in the prompt (values are never sent)
  - The user's verbatim prompt text, including anything pasted into it (tokens, paths, snippets)

  Open sub-decisions: where to store the consent flag, whether `--yes` should auto-accept on first run, whether to re-prompt when the user switches to a different remote backend, and whether the disclosure text lives in code or a versioned `DISCLOSURE.md`.

## Design Decisions Pending

- [ ] **Context Token Budget**: How much context (dir listing, git log, PATH tools) is too much? Should the prompt template enforce a hard token/character budget, or leave it unbounded? Especially relevant for local models with small context windows.
- [ ] **System Prompt Length vs. Speed**: Long system prompts improve model compliance but cost tokens on every invocation. For a one-shot CLI tool where speed and cost matter, how minimal can the role definition be?

## Lower Priority (cleanup & clarity)

- [ ] **Multi-Request Invocations**: When/why multiple backend requests happen in one invocation, caps, and the feedback loop (run → feed output back → retry).
- [ ] **`local` vs `http` Backend**: Clarify the distinction or merge them. Currently both are "call an HTTP endpoint."
- [ ] **Non-LLM Inference Trigger Logic**: Resolve "always" vs "conditionally" for context signals, define caps (N for directory listing), define how relevance to the query is determined.
