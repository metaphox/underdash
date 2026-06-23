# Underdash Roadmap to 1.0

This file tracks the path from the current state to a `v1.0` release. It supersedes the
old `TODO.md` (spec-gap checklist). Milestones are ordered; M1 items are release blockers.

## Already done

- [x] **Prompt assembly** — system prompt + XML-tagged context block + user message (`prompt/`).
- [x] **Response parsing** — JSON `command/explanation/script` schema, smart retry on malformed
  JSON (max 3), bail on clearly non-JSON (`response/`).
- [x] **Output type dispatch** — model returns `type`; runner dispatches on it (`exec/runner.go`).
- [x] **Variable safety** — `$VAR` always passed literally; model instructed never to expand.
- [x] **Claude backend** — Anthropic Messages API over SSE (`backend/claude.go`).
- [x] **Local context gathering** — cwd listing, git metadata, project type, `$PATH` tools,
  opt-in shell history (`sysinfo/`).
- [x] **Execution policy core** — risk classification + config overrides; `deny` is
  unconditional and `--yes` cannot bypass it (`exec/classify.go`, `exec/runner.go`).
- [x] **Config** — Viper YAML + `UNDERDASH_` env + flags; `.env` loading; world-readable
  api_key warning (`cmd/`).
- [x] **Flag/prompt `--` parsing (pflag layer)** — `--dry-run` parses as a flag, `--`
  terminates flag parsing so the rest is literal prompt, `SetInterspersed(false)` confines
  flags to before the prompt, leading `-<digit>` treated as prompt, structured backend error
  display. **Note:** this is the *flag-parsing* behavior only — the *semantic* `--` split is
  still open; see M1.

---

## M1 — Correctness & safety (release blockers)

- [x] **Reconcile the `--` semantic split.** SPEC `§Tool Hints` defines `--` as "everything
  after is supplementary prompt, **not** a tool request." Done: `runRoot` now passes
  `cmd.ArgsLenAtDash()` into `input.Parse`, which honors both a parser-consumed front `--`
  and a literal `--` left in args after the first bareword (needed because flag interspersing
  is disabled). `BuildUserMessage` collapses to a single block when only one side is present.
  Covered by `input/parser_test.go` and `prompt/prompt_test.go`, verified end-to-end via the
  `stdout` backend.

- [ ] **Backend timeout + cancellation.** `backend/claude.go` uses `http.DefaultClient` (no
  timeout) and `main.go` has no signal handling, so a hung backend hangs forever and Ctrl+C
  can't cancel a stream cleanly. Add an `http.Client` timeout and wire
  `signal.NotifyContext(SIGINT)` into the root context.

- [ ] **Harden the classifier against substitution.** `splitSegments` (`exec/classify.go`)
  isn't shell-aware: `echo $(rm -rf ~)`, backticks, `eval`, and heredocs classify by the
  outer safe command. Scan the full command for `$(`, backticks, `eval`, and sensitive
  redirects before trusting per-segment safety. Also resolve the policy edge cases noted
  previously: compound-command (`&&`, `|`, `;`) risk aggregation and whether `--yes` bypasses
  `deny` (currently: it does not — keep that, document it).

- [ ] **Tests for the above** — `--` split, timeout/cancellation, classifier substitution
  cases; backfill missing unit tests for `input/`, `prompt/`, `exec/runner.go`.

## M2 — Backend completeness

- [ ] **Implement `openai`** — reuse the OpenAI-shaped `backend.APIError` envelope already
  added in `backend/error.go`.
- [ ] **Implement a generic OpenAI-compatible `http`/`local` backend** (e.g. Ollama `/v1`),
  and **resolve `local` vs `http`** — currently both are "call an HTTP endpoint"; either
  merge them or define a crisp distinction.
- [ ] **`--backend` integration tests** across the resolved backend set.

## M3 — Privacy & version

- [ ] **`--version` / `-v`** — currently absent. Print version plus the full data-disclosure
  text so users can re-read what may be transmitted without resetting consent.
- [ ] **First-run privacy acknowledgment** — when a non-local backend (`claude`/`openai`/
  `http`) is configured, prompt once to acknowledge that local context is uploaded to a third
  party; persist consent (config flag or marker file under `~/.config/underdash/`); skip for
  `stdout` and `local`.

  Disclosure must enumerate everything transmittable: OS/arch/shell; cwd **path**; top-level
  listing (names only, capped 20); git branch + short status (includes modified/untracked
  paths) + `origin` URL; project type + marker; recent commit subjects; notable `$PATH`
  tools; shell history (opt-in only); **names** of env vars referenced (values never sent);
  and the verbatim prompt text. *Note: env-var-name inclusion is specced but not yet wired —
  `sysinfo`/`prompt` currently include no env vars at all, so the privacy guarantee holds by
  omission today.*

  Open sub-decisions: where to store consent; whether `--yes` auto-accepts on first run;
  whether switching to a different remote backend re-prompts; disclosure text in code vs a
  versioned `DISCLOSURE.md`.

## M4 — Streaming UX (decide in or out for 1.0)

- [ ] **Real streaming display** — SSE is wired but output buffers until the full JSON parses.
  Add `SendStream(ctx, req, onText func(string))` to the backend interface plus a JSON-string
  extractor that emits only the `command`/`explanation`/`script` field bytes as they arrive;
  stop the spinner on first emitted byte.

  Open sub-decisions: double-render (streamed output replaces vs duplicates the post-parse
  `display.Show*`); retry UX (ANSI-wipe streamed garbage vs append `(retrying…)`); plain/
  non-TTY behavior (buffer vs line-buffered stream); field ordering (rely on `type` first vs
  detect any known field key).

## M5 — Release polish

- [ ] **Docs** — flesh out `README.md` (install, config example, security note, usage
  examples); ensure `SPEC.md` matches final `--`/backend decisions.
- [ ] **Repo hygiene** — remove the empty `code-review.md`, add `.DS_Store` to `.gitignore`.
- [ ] **CI** — confirm build + `go test ./...` + `golangci-lint` run as gates.
- [ ] **Tag `v1.0`.**

## Design decisions still pending

- [ ] **Context token budget** — cap context size (dir listing, git log, PATH tools) or leave
  unbounded? Matters for small-context local models.
- [ ] **System prompt length vs. speed** — how minimal can the role definition be for a
  one-shot tool where every invocation pays the token cost?
- [ ] **Multi-request invocations** — when/why multiple backend calls happen in one
  invocation, caps, and the run → feed-output-back → retry loop.
- [ ] **Tool-hint grammar** — formalize `:tool` and `--`: mid-sentence `:`, escaping,
  disambiguation, conflicting usage.
- [ ] **Non-LLM inference triggers** — resolve "always" vs "conditionally" per signal and how
  query relevance is determined.
