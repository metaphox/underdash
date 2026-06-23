package backend

import (
	"errors"
	"strings"
	"testing"
)

// --- Finding #3: all spec-defined backend types should be accepted ---

func TestNew_AllSpecTypes(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "stdout",
			cfg:     Config{Type: "stdout"},
			wantErr: false,
			errMsg:  "",
		},
		{
			name:    "claude with key",
			cfg:     Config{Type: "claude", APIKey: "sk-test"},
			wantErr: false,
			errMsg:  "",
		},
		{
			name:    "claude without key",
			cfg:     Config{Type: "claude"},
			wantErr: true,
			errMsg:  "requires an API key",
		},
		{
			name:    "openai should not be unknown",
			cfg:     Config{Type: "openai"},
			wantErr: true,
			errMsg:  "not yet implemented", // should be a friendly "not implemented" not "unknown"
		},
		{
			name:    "local should not be unknown",
			cfg:     Config{Type: "local"},
			wantErr: true,
			errMsg:  "not yet implemented",
		},
		{
			name:    "http should not be unknown",
			cfg:     Config{Type: "http"},
			wantErr: true,
			errMsg:  "not yet implemented",
		},
		{
			name:    "truly unknown type",
			cfg:     Config{Type: "foobar"},
			wantErr: true,
			errMsg:  "unknown backend type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("New(%+v) should return error", tt.cfg)
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Fatalf("New(%+v) unexpected error: %v", tt.cfg, err)
				}
			}
		})
	}
}

// --- Claude SSE streaming ---

func TestReadClaudeStream_ConcatenatesTextDeltas(t *testing.T) {
	stream := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start"}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":0}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"{\"type\":"}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"\"command\",\"command\":\"ls\"}"}}`,
		``,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	got, err := readClaudeStream(strings.NewReader(stream), "claude")
	if err != nil {
		t.Fatalf("readClaudeStream: %v", err)
	}
	want := `{"type":"command","command":"ls"}`
	if got != want {
		t.Errorf("readClaudeStream\n got: %q\nwant: %q", got, want)
	}
}

func TestReadClaudeStream_PropagatesErrorEvent(t *testing.T) {
	stream := "data: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"servers busy\"}}\n\n"
	_, err := readClaudeStream(strings.NewReader(stream), "claude")
	if err == nil {
		t.Fatal("expected error from error event")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Type != "overloaded_error" || apiErr.Message != "servers busy" {
		t.Errorf("error should surface event details, got Type=%q Message=%q", apiErr.Type, apiErr.Message)
	}
}

func TestReadClaudeStream_EmptyIsError(t *testing.T) {
	stream := "event: message_start\ndata: {\"type\":\"message_start\"}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	_, err := readClaudeStream(strings.NewReader(stream), "claude")
	if err == nil {
		t.Fatal("expected error when no text deltas were received")
	}
}
