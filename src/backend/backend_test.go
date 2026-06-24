package backend

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- Finding #3: all spec-defined backend types should be accepted ---

func TestNew_AllSpecTypes(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		wantErr  bool
		errMsg   string
		wantName string
	}{
		{name: "stdout", cfg: Config{Type: "stdout"}, wantName: "stdout"},
		{name: "claude with key", cfg: Config{Type: "claude", APIKey: "sk-test"}, wantName: "claude"},
		{name: "claude without key", cfg: Config{Type: "claude"}, wantErr: true, errMsg: "requires an API key"},
		{name: "openai with key", cfg: Config{Type: "openai", APIKey: "sk-test"}, wantName: "openai"},
		{name: "openai without key", cfg: Config{Type: "openai"}, wantErr: true, errMsg: "requires an API key"},
		{name: "local with endpoint", cfg: Config{Type: "local", Endpoint: "http://localhost:11434/v1"}, wantName: "local"},
		{name: "local without endpoint", cfg: Config{Type: "local"}, wantErr: true, errMsg: "requires an endpoint"},
		{name: "http with endpoint", cfg: Config{Type: "http", Endpoint: "http://localhost:8080/v1"}, wantName: "http"},
		{name: "http without endpoint", cfg: Config{Type: "http"}, wantErr: true, errMsg: "requires an endpoint"},
		{name: "truly unknown type", cfg: Config{Type: "foobar"}, wantErr: true, errMsg: "unknown backend type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			be, err := New(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("New(%+v) should return error", tt.cfg)
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("New(%+v) unexpected error: %v", tt.cfg, err)
			}
			if tt.wantName != "" && be.Name() != tt.wantName {
				t.Errorf("Name() = %q, want %q", be.Name(), tt.wantName)
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

func TestClaudeSend_ContextCancellation(t *testing.T) {
	// A server that hangs until the test ends, so the request can only be
	// terminated by context cancellation — proving Send honors ctx.
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer srv.Close()
	defer close(block)

	be := &ClaudeBackend{APIKey: "x", Model: "m", Endpoint: srv.URL}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before the call

	_, err := be.Send(ctx, Request{UserMessage: "hi", MaxTokens: 16})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}
