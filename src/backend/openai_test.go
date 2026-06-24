package backend

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveChatEndpoint(t *testing.T) {
	tests := []struct{ in, want string }{
		{"https://api.openai.com/v1", "https://api.openai.com/v1/chat/completions"},
		{"http://localhost:11434/v1/", "http://localhost:11434/v1/chat/completions"},
		{"http://host/v1/chat/completions", "http://host/v1/chat/completions"},
	}
	for _, tt := range tests {
		if got := resolveChatEndpoint(tt.in); got != tt.want {
			t.Errorf("resolveChatEndpoint(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestReadOpenAIStream_ConcatenatesContent(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"choices":[{"delta":{"role":"assistant","content":""}}]}`,
		``,
		`data: {"choices":[{"delta":{"content":"foo"}}]}`,
		``,
		`data: {"choices":[{"delta":{"content":"bar"}}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	got, err := readOpenAIStream(strings.NewReader(stream), "openai")
	if err != nil {
		t.Fatalf("readOpenAIStream: %v", err)
	}
	if got != "foobar" {
		t.Errorf("got %q, want %q", got, "foobar")
	}
}

func TestReadOpenAIStream_PropagatesErrorEvent(t *testing.T) {
	stream := "data: {\"error\":{\"type\":\"server_error\",\"message\":\"boom\"}}\n\n"
	_, err := readOpenAIStream(strings.NewReader(stream), "openai")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %T: %v", err, err)
	}
	if apiErr.Type != "server_error" || apiErr.Message != "boom" {
		t.Errorf("apiErr = Type %q Message %q", apiErr.Type, apiErr.Message)
	}
}

func TestReadOpenAIStream_EmptyIsError(t *testing.T) {
	_, err := readOpenAIStream(strings.NewReader("data: [DONE]\n\n"), "openai")
	if err == nil {
		t.Fatal("expected error when no content deltas were received")
	}
}

func TestOpenAISend_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q, want /v1/chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("Authorization = %q, want Bearer sk-test", got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello \"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"world\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	be := &OpenAIBackend{name: "openai", APIKey: "sk-test", Model: "gpt-4o", Endpoint: srv.URL + "/v1"}
	got, err := be.Send(context.Background(), Request{SystemPrompt: "sys", UserMessage: "hi", MaxTokens: 16})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestOpenAISend_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":{"type":"model_not_found","message":"no such model"}}`)
	}))
	defer srv.Close()

	be := &OpenAIBackend{name: "openai", APIKey: "sk-test", Model: "x", Endpoint: srv.URL + "/v1"}
	_, err := be.Send(context.Background(), Request{UserMessage: "hi"})

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 404 || apiErr.Type != "model_not_found" || apiErr.Message != "no such model" {
		t.Errorf("apiErr = %+v", apiErr)
	}
	if apiErr.Backend != "openai" {
		t.Errorf("Backend = %q, want openai", apiErr.Backend)
	}
}

func TestOpenAISend_ContextCancellation(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer srv.Close()
	defer close(block)

	be := &OpenAIBackend{name: "openai", APIKey: "x", Model: "m", Endpoint: srv.URL + "/v1"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := be.Send(ctx, Request{UserMessage: "hi", MaxTokens: 16})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestOpenAISend_NoAuthHeaderWhenKeyless(t *testing.T) {
	// local/http backends often have no API key; no Authorization header should be sent.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Header["Authorization"]; ok {
			t.Errorf("unexpected Authorization header for keyless backend")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	be := &OpenAIBackend{name: "local", Model: "llama3", Endpoint: srv.URL + "/v1"}
	got, err := be.Send(context.Background(), Request{UserMessage: "hi"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got != "ok" {
		t.Errorf("got %q, want ok", got)
	}
}
