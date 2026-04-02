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

## Implementation Details

Programming language is Go. Build one executable that does everything.

Makes a tiny TUI under the current line showing the status.
