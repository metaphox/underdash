# Code Review Against `SPEC.md`

## Findings

1. **Critical**: pipe-to-shell payloads are classified too low, so a spec-defined dangerous command can slip through with only a normal confirmation.
   - Spec reference: `SPEC.md` marks `curl | sh` / `wget | bash` style commands as **dangerous**.
   - Code reference: `src/exec/classify.go:33-34` lists those patterns as dangerous, but `src/exec/classify.go:45` and `src/exec/classify.go:92-98` split the command on `|` before matching. By the time `src/exec/classify.go:57-60` checks `strings.Contains(seg, p)`, no segment still contains `curl | sh`, so the dangerous pattern is never matched.
   - Impact: a generated `curl https://example | sh` command will be downgraded from the spec's dangerous path to `Confirm`, which weakens the execution policy on one of the exact high-risk cases the spec calls out.

2. **High**: `--dry-run` implements the wrong behavior, and `--no-exec` is missing entirely.
   - Spec reference: `SPEC.md` defines `--dry-run` / `--no-exec` as "never execute, just print the command".
   - Code reference: `src/cmd/root.go:55` defines `--dry-run`, but `src/cmd/root.go:77-82` exits before contacting any backend and only prints the assembled prompts via `display.ShowDryRun`. `src/cmd/root.go:52-58` never defines `--no-exec`.
   - Impact: users cannot use the spec's safe inspection flow to see the generated command/script without executing it. Instead they only see prompt internals, which is a materially different interface.

3. **High**: the backend abstraction does not implement most backend types required by the spec.
   - Spec reference: `SPEC.md` requires named backends with `type` in `{claude, openai, local, http, stdout}`.
   - Code reference: `src/backend/backend.go:31-48` only supports `stdout` and `claude`; every other spec-defined type fails as `unknown backend type`. `src/cmd/root.go:192-208` does load generic backend config, but there is no implementation behind `openai`, `local`, or `http`.
   - Impact: a valid spec-compliant config cannot be used. This is a core feature gap in the abstraction layer rather than an optional enhancement.

4. **Medium**: output mode handling from the spec is effectively unimplemented, and non-TTY fallback is checked against the wrong stream.
   - Spec reference: `SPEC.md` requires configurable `streaming` vs `plain` output, with automatic fallback to `plain` whenever **stdout** is not a TTY.
   - Code reference: `src/cmd/root.go:57` defines `--output`, but nothing reads it. `src/cmd/root.go:106-116` always starts the spinner path. `src/display/display.go:11-13` bases TTY detection on `stderr`, not `stdout`.
   - Impact: piping output can still emit spinner/status UI to `stderr` even though the spec says the program should switch to plain mode when stdout is not interactive. The documented `--output plain` behavior also does not exist.

5. **Medium**: execution policy overrides from `config.yaml` are ignored, including the spec's deny list.
   - Spec reference: `SPEC.md` says built-in risk defaults must be extendable/overridable via `execution.auto_run`, `execution.confirm`, and `execution.deny`.
   - Code reference: `src/exec/classify.go:17-38` hardcodes the classifier tables, and there is no code in `src/cmd/root.go` or `src/exec/runner.go` that reads `execution.*` from Viper or enforces a deny path.
   - Impact: user policy cannot tighten or relax execution behavior as specified. In particular, there is no way to make a command unconditionally denied through config.

6. **Medium**: the context enrichment layer omits several required signals, so the backend is operating with materially less context than the spec promises.
   - Spec reference: `SPEC.md` requires directory entries to include name/type/size, git context to include the last 3 commit subjects, OS/shell info to include notable tools on `$PATH`, and optional shell history support.
   - Code reference: `src/context/gather.go:15-25` has no fields for PATH tools, recent commits, or history. `src/context/gather.go:46-61` records only entry names, not size/type metadata. `src/context/gather.go:70-78` collects branch/status/remote only. `src/prompt/prompt.go:59-87` therefore cannot emit the missing sections.
   - Impact: prompt assembly is not just incomplete; it diverges from the spec's non-LLM inference contract that is meant to improve one-shot command generation.

7. **Medium**: the startup security warning for plaintext API keys in a world-readable config file is missing.
   - Spec reference: `SPEC.md` says Underdash should warn at startup if the config file contains an `api_key` and is world-readable.
   - Code reference: `src/cmd/root.go:211-234` reads config but never inspects file permissions or config contents after load.
   - Impact: one of the spec's explicit safeguards around secret storage is absent, so insecure local configuration goes unnoticed.

8. **Low**: retry behavior is looser than the strict JSON contract defined in the spec.
   - Spec reference: `SPEC.md` says to bail immediately when the response does not start with `{`.
   - Code reference: `src/response/response.go:36-44` accepts prose-wrapped JSON by extracting the first `{...}` substring, and `src/response/response.go:79-82` treats any output containing `{` as retryable. `src/cmd/root.go:133-139` therefore retries some responses the spec says should fail immediately.
   - Impact: this makes the parser more forgiving, but it weakens the "single valid JSON object and nothing else" contract and can hide backend formatting regressions.

## Notes

- `go test ./...` from `src/` reached all packages successfully enough to report `[no test files]`, but the command still exited non-zero in this sandbox because Go could not trim its cache under `/Users/taowu/Library/Caches/go-build/trim.txt`.
- I focused this review on mismatches between the current `src/` implementation and `SPEC.md`, not on general style or future enhancements.
