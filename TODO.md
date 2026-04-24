# TODO — Spec Gaps to Resolve

## High Priority (core functionality)

- [ ] **Prompt Assembly**: Define the system prompt template, how tool hints / context signals / user input are composed into the final prompt, and the structured format (XML tags, JSON, plain text).
- [ ] **Response Parsing**: How underdash distinguishes a runnable command from an explanation, handles multi-command / script output, and deals with model refusals or clarifying questions.
- [ ] **Output Type Detection**: Not every query expects a command (`_ what does awk -F do`). Define how underdash determines whether the response is a command to execute, an explanation to display, or a file/script to write.

## Medium Priority (correctness & safety)

- [ ] **Tool Hint Parsing Grammar**: Formalize `:tool` and `--` syntax — mid-sentence `:`, escaping, disambiguation from CLI `--` flags (Cobra interaction), conflicting usage.
- [ ] **Variable / Environment Expansion**: Does underdash expand `$TOKEN` before sending to the model (conflicts with security rule) or pass it literally? Define the policy.
- [ ] **Execution Policy Edge Cases**: Risk classification for compound commands (`&&`, pipes, `$()`), whether `deny` blocks entirely or still allows override, whether `--yes` bypasses `deny`.
- [ ] **Error Handling**: Backend unreachable / timeout / rate-limit, invalid API key, generated command fails (non-zero exit), Ctrl+C mid-stream behavior, retry policy.

## Lower Priority (cleanup & clarity)

- [ ] **Multi-Request Invocations**: When/why multiple backend requests happen in one invocation, caps, and the feedback loop (run → feed output back → retry).
- [ ] **`local` vs `http` Backend**: Clarify the distinction or merge them. Currently both are "call an HTTP endpoint."
- [ ] **Non-LLM Inference Trigger Logic**: Resolve "always" vs "conditionally" for context signals, define caps (N for directory listing), define how relevance to the query is determined.
