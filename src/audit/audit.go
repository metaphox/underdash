// Package audit records an optional, opt-in JSONL log of each invocation so a
// tool that can execute shell commands stays accountable after the fact.
package audit

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// defaultMaxSize bounds the log when no max_size is configured.
const defaultMaxSize int64 = 10 << 20 // 10 MB

// Config controls audit logging, sourced from the `audit` config section.
type Config struct {
	Enabled bool
	Path    string
	MaxSize int64 // bytes; <= 0 uses defaultMaxSize
}

// Record is one logged invocation. By construction it never carries API keys or
// environment-variable values; Query and Command are the user's verbatim text,
// which is exactly why audit logging is opt-in.
type Record struct {
	TS           string `json:"ts"`
	Query        string `json:"query"`
	Backend      string `json:"backend"`
	Model        string `json:"model,omitempty"`
	ResponseType string `json:"response_type,omitempty"`
	Command      string `json:"command,omitempty"`
	Risk         string `json:"risk,omitempty"`
	Action       string `json:"action"`
	Exit         int    `json:"exit"`
	DurationMs   int64  `json:"duration_ms,omitempty"`
	Error        string `json:"error,omitempty"`
}

// Log appends rec to the audit log as one JSON line. It is a no-op when
// disabled or unconfigured. After appending it front-truncates whole lines so
// the file never exceeds MaxSize. Errors are returned (callers treat them as
// non-fatal).
func Log(cfg Config, rec Record) error {
	if !cfg.Enabled || cfg.Path == "" {
		return nil
	}
	if rec.TS == "" {
		rec.TS = time.Now().UTC().Format(time.RFC3339)
	}

	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(cfg.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	max := cfg.MaxSize
	if max <= 0 {
		max = defaultMaxSize
	}
	return truncateFront(cfg.Path, max)
}

// truncateFront drops whole lines from the start of the file until it fits
// within max bytes, keeping the most recent records.
func truncateFront(path string, max int64) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() <= max {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for int64(len(data)) > max {
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			break // single oversized line; leave it rather than lose the record
		}
		data = data[i+1:]
	}
	return os.WriteFile(path, data, 0o644)
}
