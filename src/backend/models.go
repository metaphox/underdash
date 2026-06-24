package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// ModelLister is implemented by backends that can enumerate the models their
// endpoint currently offers, used to pick a default without hardcoding IDs.
type ModelLister interface {
	ListModels(ctx context.Context) ([]string, error)
}

// modelListResponse is the shared shape of /models responses (Anthropic and
// OpenAI both return {"data":[{"id":...}]}).
type modelListResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// ListModels returns the model IDs Anthropic currently offers (newest-first).
func (c *ClaudeBackend) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, claudeModelsURL(c.Endpoint), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	return doListModels(req, c.Name())
}

// ListModels returns the model IDs an OpenAI-compatible endpoint offers.
func (o *OpenAIBackend) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resolveModelsEndpoint(o.Endpoint), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if o.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.APIKey)
	}
	return doListModels(req, o.name)
}

func doListModels(req *http.Request, backendName string) ([]string, error) {
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort cleanup
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(backendName, resp.StatusCode, body)
	}

	var lr modelListResponse
	if err := json.Unmarshal(body, &lr); err != nil {
		return nil, fmt.Errorf("parse models: %w", err)
	}
	ids := make([]string, 0, len(lr.Data))
	for _, m := range lr.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
}

// claudeModelsURL derives the /models URL from the messages endpoint.
func claudeModelsURL(messagesEndpoint string) string {
	e := messagesEndpoint
	if e == "" {
		e = defaultClaudeEndpoint
	}
	if strings.HasSuffix(e, "/messages") {
		return strings.TrimSuffix(e, "/messages") + "/models"
	}
	return e
}

// resolveModelsEndpoint derives the /models URL from an OpenAI-compatible base
// or chat/completions URL.
func resolveModelsEndpoint(endpoint string) string {
	e := strings.TrimRight(endpoint, "/")
	if e == "" {
		e = defaultOpenAIEndpoint
	}
	e = strings.TrimRight(strings.TrimSuffix(e, "/chat/completions"), "/")
	return e + "/models"
}

// openAINonChat are substrings marking non-chat model IDs to exclude.
var openAINonChat = []string{
	"embedding", "whisper", "dall-e", "tts", "audio",
	"realtime", "transcribe", "search", "moderation", "image", "instruct",
}

// RankModels filters and preference-orders a model list so ranked[0] is the
// best default. The ordering uses durable family keywords, never pinned dates.
func RankModels(backendType string, ids []string) []string {
	switch backendType {
	case "claude":
		return rankClaude(ids)
	case "openai", "local", "http":
		return rankOpenAI(ids)
	}
	return ids
}

func rankClaude(ids []string) []string {
	var sonnet, rest []string
	for _, id := range ids {
		if !strings.HasPrefix(id, "claude-") {
			continue
		}
		if strings.Contains(id, "sonnet") {
			sonnet = append(sonnet, id)
		} else {
			rest = append(rest, id)
		}
	}
	return append(sonnet, rest...)
}

func rankOpenAI(ids []string) []string {
	families := []string{"gpt-4o", "gpt-4.1", "gpt-4", "gpt-"}

	type scored struct {
		id   string
		rank int
	}
	var list []scored
	for _, id := range ids {
		if !strings.HasPrefix(id, "gpt-") || isNonChat(id) {
			continue
		}
		fam := len(families) // least preferred
		for i, f := range families {
			if strings.HasPrefix(id, f) {
				fam = i
				break
			}
		}
		size := 0 // prefer base over mini/nano within a family
		switch {
		case strings.Contains(id, "mini"):
			size = 1
		case strings.Contains(id, "nano"):
			size = 2
		}
		list = append(list, scored{id, fam*10 + size})
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].rank < list[j].rank })

	out := make([]string, len(list))
	for i, s := range list {
		out[i] = s.id
	}
	return out
}

func isNonChat(id string) bool {
	for _, bad := range openAINonChat {
		if strings.Contains(id, bad) {
			return true
		}
	}
	return false
}
