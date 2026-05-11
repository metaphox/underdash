# Go Style Guide Review: `src/`

Review of Go source code against `golang-style-guide/` (style-guide.md, style-decisions.md, best-practices.md).

---

## Critical

### 1. Package `context` shadows the standard library

**Files:** `src/context/gather.go:1`, `src/cmd/root.go:19`, `src/prompt/prompt.go:8`  
**Rule:** style-decisions.md §Package names; best-practices.md §Shadowing

The package is named `context`, colliding with the standard library's `context` package. This forces every importer to use an alias (`uctx`), which the guide explicitly warns against.

**Current:**
```go
// src/context/gather.go
package context

// src/cmd/root.go
import uctx "metaphox/underdash/context"
```

**Suggested fix:** Rename the package to `sysinfo`, `gather`, or `environ`:
```go
// src/sysinfo/gather.go
package sysinfo

// src/cmd/root.go
import "metaphox/underdash/sysinfo"
```

---

## High — Missing Documentation

### 2. No package comments on any package

**Rule:** style-decisions.md §Package comments

None of the 9 packages have a `// Package <name> ...` comment.

**Suggested fix:** Add a package comment to the primary file in each package:

```go
// Package cmd implements CLI commands for the underdash binary.
package cmd
```

```go
// Package backend provides LLM backend implementations.
package backend
```

```go
// Package sysinfo gathers system context for prompt construction.
package sysinfo
```

```go
// Package display handles terminal output formatting.
package display
```

```go
// Package exec classifies and runs shell commands.
package exec
```

```go
// Package input parses user input from CLI arguments and stdin.
package input
```

```go
// Package prompt builds prompts from system context and user input.
package prompt
```

```go
// Package response parses and classifies LLM responses.
package response
```

For `main.go`, use a binary doc comment:
```go
// Underdash is a non-interactive CLI coding agent that provides
// one-line LLM assistance for terminal use.
package main
```

### 3. Missing doc comments on exported symbols

**Rule:** style-decisions.md §Doc comments

| File:Line | Symbol |
|-----------|--------|
| `backend/claude.go:26` | `func (c *ClaudeBackend) Name()` |
| `backend/claude.go:55` | `func (c *ClaudeBackend) Send(...)` |
| `backend/stdout.go:12` | `func (s *StdoutBackend) Name()` |
| `backend/stdout.go:14` | `func (s *StdoutBackend) Send(...)` |
| `cmd/root.go:45` | `func Execute()` |

**Suggested fix:**
```go
// Name returns the backend identifier used for logging and configuration.
func (c *ClaudeBackend) Name() string { ... }

// Send transmits the prompt to Claude and streams the response via the provided callback.
func (c *ClaudeBackend) Send(...) { ... }

// Execute runs the root command and exits on error.
func Execute() { ... }
```

---

## Medium — Naming Violations

### 4. `response.ResponseType` — redundant package name in type

**File:** `src/response/response.go:10`  
**Rule:** style-decisions.md §Package vs. exported symbol name

**Current:**
```go
type ResponseType string
// Usage: response.ResponseType
```

**Suggested fix:**
```go
type Kind string
// Usage: response.Kind
```

### 5. `response.LLMResponse` — redundant package name in struct

**File:** `src/response/response.go:19`  
**Rule:** style-decisions.md §Package vs. exported symbol name

**Current:**
```go
type LLMResponse struct { ... }
// Usage: response.LLMResponse
```

**Suggested fix:**
```go
type Result struct { ... }
// Usage: response.Result
```

### 6. `CommandStr`, `ScriptStr` — unnecessary type suffix

**File:** `src/response/response.go:21,23`  
**Rule:** style-decisions.md §Variable name vs. type

**Current:**
```go
type LLMResponse struct {
    CommandStr string `json:"command"`
    ScriptStr  string `json:"script"`
}
```

**Suggested fix:**
```go
type Result struct {
    Command string `json:"command"`
    Script  string `json:"script"`
}
```

### 7. `projectTyp` — non-idiomatic abbreviation

**File:** `src/context/gather.go:205`  
**Rule:** style-decisions.md §Variable names

**Current:**
```go
projectTyp string
```

**Suggested fix:**
```go
projectType string
```

---

## Medium — Error Handling

### 8. Discarded errors without explanatory comments

**Rule:** style-decisions.md §Handle errors

**Current (`src/cmd/root.go`):**
```go
outputMode, _ := cmd.Flags().GetString("output")
dryRun, _ := cmd.Flags().GetBool("dry-run")
noExec, _ := cmd.Flags().GetBool("no-exec")
autoYes, _ := cmd.Flags().GetBool("yes")
backendName, _ := cmd.Flags().GetString("backend")
```

**Suggested fix:**
```go
outputMode, _ := cmd.Flags().GetString("output") // err is nil; flag registered in init()
dryRun, _ := cmd.Flags().GetBool("dry-run")      // err is nil; flag registered in init()
noExec, _ := cmd.Flags().GetBool("no-exec")      // err is nil; flag registered in init()
autoYes, _ := cmd.Flags().GetBool("yes")          // err is nil; flag registered in init()
backendName, _ := cmd.Flags().GetString("backend") // err is nil; flag registered in init()
```

**Current (`src/cmd/dotenv.go:39`):**
```go
_ = os.Setenv(key, value)
```

**Suggested fix:**
```go
_ = os.Setenv(key, value) // only fails on empty key, validated by split logic above
```

### 9. Else-clause in error flow

**File:** `src/cmd/root.go:270-281`  
**Rule:** style-decisions.md §Indent error flow

**Current:**
```go
if err := viper.ReadInConfig(); err != nil {
    var configFileNotFoundError viper.ConfigFileNotFoundError
    if !errors.As(err, &configFileNotFoundError) {
        return err
    }
} else {
    cfgPath := viper.ConfigFileUsed()
    // ... check permissions ...
}
```

**Suggested fix:**
```go
err := viper.ReadInConfig()
if err != nil {
    var configFileNotFoundError viper.ConfigFileNotFoundError
    if !errors.As(err, &configFileNotFoundError) {
        return err
    }
    // Config file not found — skip permissions check.
    return viper.BindPFlags(cmd.Flags())
}

// Config file loaded — check permissions.
cfgPath := viper.ConfigFileUsed()
// ... check permissions ...
```

---

## Low

### 10. Reinventing `strings.Contains` in tests

**File:** `src/backend/backend_test.go:73-84`  
**Rule:** style-guide.md §Least mechanism

**Current:**
```go
func containsStr(slice []string, s string) bool { ... }
func containsSubstr(s, substr string) bool { ... }
```

**Suggested fix:** Delete these helpers and use `strings.Contains` / `slices.Contains` directly:
```go
if !strings.Contains(err.Error(), tt.errMsg) {
    t.Errorf(...)
}
```

### 11. Dead code branch in `runScript`

**File:** `src/exec/runner.go:73-88`  
**Rule:** style-guide.md §Simplicity

`risk` is always set to `Dangerous`, so the `case Confirm:` branch is unreachable.

**Suggested fix:** Remove the dead branch:
```go
// risk is always Dangerous here
if !autoYes {
    // prompt user for confirmation
}
```

Or, if `Confirm` is intended for future use, add a `// TODO` comment explaining this.

### 12. Unfilled copyright template

**File:** `src/main.go:1-4`

**Current:**
```go
/* Copyright © 2026 NAME HERE <EMAIL ADDRESS> */
```

**Suggested fix:** Fill in real info or remove:
```go
// Copyright 2026 Tao Wu. All rights reserved.
```

### 13. Test struct literals without field names

**File:** `src/backend/backend_test.go:17-51`  
**Rule:** style-decisions.md §Field names

**Current:**
```go
{"stdout", Config{Type: "stdout"}, false, ""},
```

**Suggested fix:**
```go
{
    name:    "stdout",
    cfg:     Config{Type: "stdout"},
    wantErr: false,
    errMsg:  "",
},
```

### 14. Inconsistent test env cleanup

**File:** `src/cmd/dotenv_test.go`  
**Rule:** best-practices.md (idiomatic test patterns)

**Current:** Mixes `t.Setenv` and manual `os.Unsetenv`.

**Suggested fix:** Use `t.Setenv` exclusively — it registers automatic cleanup via `t.Cleanup`:
```go
t.Setenv("UNDERDASH_BACKEND", "claude")
// no manual Unsetenv needed
```

---

## Summary

| Severity | Count | Category |
|----------|-------|----------|
| Critical | 1 | Package name shadows stdlib (`context`) |
| High | 14 | Missing package/symbol doc comments |
| Medium | 11 | Naming redundancy + error handling |
| Low | 5 | Test style, dead code, template |

### Recommended fix order

1. Rename `context` package (critical — ripple effect across codebase)
2. Add package comments to all packages
3. Fix naming in `response` package (`ResponseType` -> `Kind`, `LLMResponse` -> `Result`)
4. Add error-discard comments in `cmd/root.go`
5. Refactor else-clause error flow in config loading
6. Clean up test helpers and struct literals
