package backend

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRankModels_Claude(t *testing.T) {
	// API order is newest-first; sonnet should be floated to the top, non-claude dropped.
	ids := []string{"claude-opus-4-1", "claude-haiku-4", "claude-sonnet-4-5", "gpt-4o", "claude-sonnet-3-7"}
	got := RankModels("claude", ids)

	if len(got) == 0 || !strings.Contains(got[0], "sonnet") {
		t.Fatalf("ranked[0] = %v, want a sonnet model first", got)
	}
	for _, id := range got {
		if !strings.HasPrefix(id, "claude-") {
			t.Errorf("non-claude id leaked into ranking: %q", id)
		}
	}
}

func TestRankModels_OpenAI(t *testing.T) {
	ids := []string{
		"text-embedding-3-small", "gpt-4o-mini", "gpt-3.5-turbo", "whisper-1",
		"gpt-4.1", "gpt-4o", "dall-e-3", "gpt-4o-realtime-preview", "o1-mini",
	}
	got := RankModels("openai", ids)

	if len(got) == 0 {
		t.Fatal("expected some ranked openai models")
	}
	if got[0] != "gpt-4o" {
		t.Errorf("ranked[0] = %q, want gpt-4o (family preference)", got[0])
	}
	for _, id := range got {
		if !strings.HasPrefix(id, "gpt-") {
			t.Errorf("non-chat id leaked into ranking: %q", id)
		}
		for _, bad := range []string{"embedding", "whisper", "dall-e", "realtime", "audio"} {
			if strings.Contains(id, bad) {
				t.Errorf("non-chat id %q should be filtered", id)
			}
		}
	}
}

func TestClaudeListModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("path = %q, want /v1/models", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "sk-test" {
			t.Errorf("x-api-key = %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("missing anthropic-version header")
		}
		w.Write([]byte(`{"data":[{"id":"claude-sonnet-4-5"},{"id":"claude-opus-4-1"}]}`))
	}))
	defer srv.Close()

	be := &ClaudeBackend{APIKey: "sk-test", Endpoint: srv.URL + "/v1/messages"}
	ids, err := be.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(ids) != 2 || ids[0] != "claude-sonnet-4-5" {
		t.Errorf("ids = %v", ids)
	}
}

func TestOpenAIListModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("path = %q, want /v1/models", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.Write([]byte(`{"data":[{"id":"gpt-4o"},{"id":"gpt-4o-mini"}]}`))
	}))
	defer srv.Close()

	be := &OpenAIBackend{name: "openai", APIKey: "sk-test", Endpoint: srv.URL + "/v1"}
	ids, err := be.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(ids) != 2 || ids[0] != "gpt-4o" {
		t.Errorf("ids = %v", ids)
	}
}

func TestOpenAIListModels_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"type":"invalid_api_key","message":"bad key"}}`))
	}))
	defer srv.Close()

	be := &OpenAIBackend{name: "openai", APIKey: "x", Endpoint: srv.URL + "/v1"}
	_, err := be.ListModels(context.Background())
	if err == nil {
		t.Fatal("expected error from 401")
	}
}

func TestIsModelNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  *APIError
		want bool
	}{
		{"anthropic not found", &APIError{StatusCode: 404, Type: "not_found_error", Message: "model: claude-x"}, true},
		{"openai model not found", &APIError{StatusCode: 404, Type: "model_not_found", Message: "no such model"}, true},
		{"404 mentioning model", &APIError{StatusCode: 404, Message: "the model foo does not exist"}, true},
		{"404 unrelated", &APIError{StatusCode: 404, Type: "not_found_error", Message: "endpoint missing"}, false},
		{"429", &APIError{StatusCode: 429, Type: "rate_limit_error"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.IsModelNotFound(); got != tt.want {
				t.Errorf("IsModelNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}
