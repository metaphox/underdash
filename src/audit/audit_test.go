package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLog_DisabledIsNoOp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	if err := Log(Config{Enabled: false, Path: path}, Record{Query: "x"}); err != nil {
		t.Fatalf("Log: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("disabled audit should not create a file")
	}
}

func TestLog_AppendsJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	cfg := Config{Enabled: true, Path: path}

	if err := Log(cfg, Record{Query: "first", Backend: "claude", Action: "executed", Exit: 0}); err != nil {
		t.Fatal(err)
	}
	if err := Log(cfg, Record{Query: "second", Backend: "openai", Action: "denied", Exit: 1}); err != nil {
		t.Fatal(err)
	}

	lines := readLines(t, path)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}

	var rec Record
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("line 0 not valid JSON: %v", err)
	}
	if rec.Query != "first" || rec.Backend != "claude" || rec.Action != "executed" {
		t.Errorf("record 0 = %+v", rec)
	}
	if rec.TS == "" {
		t.Errorf("timestamp should be auto-filled")
	}
}

func TestLog_FrontTruncatesPastMaxSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	// Tiny cap so the log can hold only a couple of records.
	cfg := Config{Enabled: true, Path: path, MaxSize: 120}

	for i := 0; i < 20; i++ {
		if err := Log(cfg, Record{Query: strings.Repeat("q", 10), Backend: "claude", Action: "executed"}); err != nil {
			t.Fatal(err)
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > cfg.MaxSize {
		t.Errorf("file size %d exceeds cap %d", info.Size(), cfg.MaxSize)
	}

	// Whatever remains must still be valid, newest-kept JSONL.
	for _, ln := range readLines(t, path) {
		var rec Record
		if err := json.Unmarshal([]byte(ln), &rec); err != nil {
			t.Errorf("remaining line not valid JSON: %q", ln)
		}
	}
}

func TestLog_UnwritablePathErrors(t *testing.T) {
	dir := t.TempDir()
	// Create a regular file, then try to use it as a parent directory.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(blocker, "audit.jsonl")

	if err := Log(Config{Enabled: true, Path: path}, Record{Query: "x"}); err == nil {
		t.Error("expected error for unwritable audit path")
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) != "" {
			out = append(out, sc.Text())
		}
	}
	return out
}
