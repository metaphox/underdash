# CLAUDE.md

This file provides guidance to Coding Agents when working with code in this repository.

## Project Overview
Underdash is a non-interactive CLI coding agent written in Go. It provides one-line LLM assistance, designed to be aliased as `_` (underscore) for quick terminal use (e.g., `_ tar the newest directory`).

## Tech Stack
- Go language

## Architecture

- **Go module:** `metaphox/underdash` (Go 1.26.1)
- **Configuration:** Viper (`github.com/spf13/viper`) — YAML config at `~/.config/underdash/config.yaml`, env vars with `UNDERDASH_` prefix, CLI flags
- **Entry point:** `src/main.go`
- **Root command:** `src/cmd/root.go` — defines the root Cobra command with config initialization via `PersistentPreRunE`
- **Config precedence:** CLI flags > environment variables > config file

## Configuration

- Config file: `$HOME/.config/underdash/config.yaml`
- Override with `--config /path/to/config.yaml`
- Environment variables: `UNDERDASH_*` prefix (`.` and `-` replaced with `*` in key names)

## Build & Run

All Go source lives in the `src/` directory. Commands must be run from there:

```bash
cd src
go build -o underdash .
./underdash --help
```

## Run Tests

```bash
cd src
go test ./...            # run all tests
go test ./cmd/...        # run tests in a specific package
go test -run TestName ./cmd/...  # run a single test
```

## Conventions
Version control is managed by the user, never commit on your own or ask the user if you should commit.
