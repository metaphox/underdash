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

- [x] **Backend timeout + cancellation.** Done: shared `httpClient` in `backend/client.go`
  with dial + TLS + response-header timeouts (streaming-safe — total stream duration is not
  capped); `claude.go` uses it. `Execute` wires `signal.NotifyContext(SIGINT/SIGTERM)` into
  the root context via `ExecuteContext`, so an in-flight request/stream aborts on Ctrl+C
  (printing `cancelled`, exit 130; second signal force-quits). `renderError` shows a friendly
  message for `net.Error` timeouts. Cancellation propagation covered by
  `TestClaudeSend_ContextCancellation`.

- [x] **Harden the classifier against substitution.** Done: `classifyBuiltin`
  (`exec/classify.go`) now recursively classifies `$(...)` and backtick substitution bodies
  and floors the overall risk by them, so an opaque or dangerous inner command can't auto-run
  behind a safe outer command (`echo $(rm -rf ~)` → Dangerous; `echo $(mysterious)` →
  Confirm; `echo $(date)` and `$((1+2))` arithmetic stay Safe). `eval` is now a dangerous
  pattern. Compound `&&`/`||`/`;`/`|` risk aggregation was already handled by `classifySegments`
  (highest-wins); `--yes` does **not** bypass `deny` (`Denied` tier, documented in
  `classify.go`/`runner.go`).

- [x] **Tests for the above** — done: `--` split (`input/parser_test.go`), `BuildUserMessage`
  (`prompt/prompt_test.go`), context cancellation (`backend` `TestClaudeSend_ContextCancellation`),
  and substitution/eval (`exec/classify_test.go` `TestClassify_Substitution`). Full suite: 105
  passing. (Note: `exec/runner.go` `executeShell`/`promptUser` still lack direct unit tests —
  they shell out and read stdin; deferred as non-blocking.)

## M2 — Backend completeness

- [x] **Implement `openai`** — done: `OpenAIBackend` (`backend/openai.go`) calls the Chat
  Completions API over SSE, reusing the `parseAPIError` envelope. `New` requires a key and
  defaults endpoint/model. Covered by `backend/openai_test.go` (stream concat, error event,
  empty-stream, success via httptest, API error, cancellation).
- [x] **Generic OpenAI-compatible `local`/`http` + resolve `local` vs `http`** — done:
  resolved by **merging** — `local`/`http`/`openai` share `OpenAIBackend`; `local`/`http`
  require an explicit endpoint and make the key optional (no `Authorization` header when
  keyless, e.g. Ollama). `resolveChatEndpoint` appends `/chat/completions` to a `/v1` base.
  Decision documented in SPEC `§Backend Abstraction`. Tested incl. the keyless path.
- [x] **`--backend` integration tests** — done: `cmd/resolve_test.go` `TestResolveBackend`
  exercises `--backend` / `default_backend` resolution across stdout/claude/openai/local/http,
  including the missing-key and missing-endpoint error cases.
- [x] **Dynamic model resolution (no hardcoded model IDs)** — done: removed the hardcoded
  `claude-sonnet-4-...`/`gpt-4o` defaults from `backend.New`. `backend/models.go` adds a
  `ModelLister` (Claude + OpenAI `/v1/models`) and `RankModels` (durable family keywords).
  When no model is configured, `resolveBackend` discovers one — interactive pick on a TTY,
  auto-pick otherwise — and persists it to the config (`cmd/model.go` `persistModel`,
  YAML round-trip preserving existing keys; warn-and-continue if unwritable). A retired model
  self-heals on a model-not-found 404 (`APIError.IsModelNotFound` + `maybeSelfHeal`: re-discover,
  rewrite config, retry once). Covered by `backend/models_test.go` and `cmd/{model,resolve}_test.go`,
  verified end-to-end against a mock `/v1/models` + `/v1/chat/completions`.

## M3 — Privacy, observability & version

See SPEC `§Error Handling & Audit` for the full design of the items below.

- [x] **`--version`** (long form only) — done: `cmd/version.go` `versionInfo()` prints the
  version (`-ldflags`-overridable `version` var) plus the full disclosure text; handled at the
  top of `runRoot` so it works with no prompt. `-v` is the `--verbose` shorthand, not version.
- [x] **`--verbose` / `-v`** — done: `display.Verbosef` (gated, writes to stderr) wired into
  `runRoot` to log resolved backend/model, context-signal summary, the assembled prompt, and
  request timing. `display.RedactKey` masks key-shaped values. (`UNDERDASH_DEBUG=1` raw
  protocol tracing and token-usage reporting deferred — not blocking.)
- [x] **Audit log** — done: new `audit` package (`audit.Log`, JSONL, append + front-truncate
  past `max_size`, non-fatal). Config via `auditConfigFrom` (`audit.enabled`/`path`/`max_size`,
  `parseSize` handles `10MB`, default `~/.local/state/underdash/audit.jsonl`). `recordAudit`
  in `runRoot` logs the execution outcome (and backend errors); records carry no api_key/env
  values by construction. Verified end-to-end via env config.
- [x] **Stable exit codes** — done: `cmd/exitcode.go` `exitCodeFor` (0/1/2/130 + command
  passthrough via `*exec.ExitError`); `Execute` renders then exits with it. `exec.Run` now
  returns an `Outcome{Action,Risk}` feeding both the audit record and the exit logic.
- [x] **First-run privacy acknowledgment** — done: `cmd/consent.go` `ensureConsent` gates
  remote backends (`claude`/`openai`/`http`; skips `local`/`stdout`) on a one-time marker at
  `~/.config/underdash/consent`, prompting once with the disclosure. **Decisions made:** `--yes`
  auto-accepts on first run; a single acknowledgment covers all remote backends (no re-prompt
  on backend switch); disclosure text lives in code (`disclosureText`).

  *Note still open:* env-var-name inclusion remains unwired — `sysinfo`/`prompt` include no env
  vars at all, so the privacy guarantee holds by omission today (the disclosure lists it as a
  forward-looking item).

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
- [x] **Repo hygiene** — remove the empty `code-review.md`, add `.DS_Store` to `.gitignore`.
- [x] **CI** — confirm build + `go test ./...` + `golangci-lint` run as gates.
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
