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

## Design Decisions Pending

- [ ] **Context Token Budget**: How much context (dir listing, git log, PATH tools) is too much? Should the prompt template enforce a hard token/character budget, or leave it unbounded? Especially relevant for local models with small context windows.
- [ ] **System Prompt Length vs. Speed**: Long system prompts improve model compliance but cost tokens on every invocation. For a one-shot CLI tool where speed and cost matter, how minimal can the role definition be?

## Lower Priority (cleanup & clarity)

- [ ] **Multi-Request Invocations**: When/why multiple backend requests happen in one invocation, caps, and the feedback loop (run → feed output back → retry).
- [ ] **`local` vs `http` Backend**: Clarify the distinction or merge them. Currently both are "call an HTTP endpoint."
- [ ] **Non-LLM Inference Trigger Logic**: Resolve "always" vs "conditionally" for context signals, define caps (N for directory listing), define how relevance to the query is determined.
