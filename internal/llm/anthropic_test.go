package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropicChatRequestAndResponse(t *testing.T) {
	var gotReq anthropicRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("missing/incorrect x-api-key header: %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Fatalf("missing anthropic-version header")
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("failed to decode request: %s", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"content": [
				{"type": "text", "text": "here goes"},
				{"type": "tool_use", "id": "toolu_1", "name": "get_query_info", "input": {"query": "javascript.Group.Q"}}
			],
			"stop_reason": "tool_use"
		}`))
	}))
	defer srv.Close()

	c := &anthropicClient{model: "claude-sonnet-4-5", baseURL: srv.URL, apiKey: "test-key", httpClient: &http.Client{}}

	messages := []Message{
		{Role: RoleSystem, Content: "SYS"},
		{Role: RoleTool, Content: "CHANGELOG"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "call_1", Name: "get_query_info", Args: map[string]any{"query": "x"}}}},
		{Role: RoleTool, Content: "RESULT", ToolCallID: "call_1"},
		{Role: RoleUser, Content: "PROMPT"},
	}
	tools := []ToolDef{{Name: "get_query_info", Description: "d", Parameters: map[string]any{"type": "object"}}}

	resp, err := c.Chat(context.Background(), messages, tools)
	if err != nil {
		t.Fatalf("Chat: %s", err)
	}

	if gotReq.System != "SYS" {
		t.Fatalf("system = %q, want SYS", gotReq.System)
	}
	// The whole history collapses into exactly one user message with a
	// single text block — no tool_use/tool_result blocks needing a
	// preceding, matching turn to be valid.
	if len(gotReq.Messages) != 1 || gotReq.Messages[0].Role != "user" {
		t.Fatalf("expected a single user message, got %+v", gotReq.Messages)
	}
	if len(gotReq.Messages[0].Content) != 1 || gotReq.Messages[0].Content[0].Type != "text" {
		t.Fatalf("expected a single text block, got %+v", gotReq.Messages[0].Content)
	}
	text := gotReq.Messages[0].Content[0].Text
	for _, want := range []string{"CHANGELOG", "get_query_info", "RESULT", "PROMPT"} {
		if !strings.Contains(text, want) {
			t.Fatalf("prompt missing %q: %s", want, text)
		}
	}

	if resp.Content != "here goes" {
		t.Fatalf("Content = %q", resp.Content)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].ID != "toolu_1" || resp.ToolCalls[0].Name != "get_query_info" {
		t.Fatalf("unexpected tool calls: %+v", resp.ToolCalls)
	}
	if resp.ToolCalls[0].Args["query"] != "javascript.Group.Q" {
		t.Fatalf("unexpected tool call args: %+v", resp.ToolCalls[0].Args)
	}
}

func TestAnthropicChatErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"type": "error", "error": {"type": "authentication_error", "message": "invalid x-api-key"}}`))
	}))
	defer srv.Close()

	c := &anthropicClient{model: "claude-sonnet-4-5", baseURL: srv.URL, apiKey: "bad-key", httpClient: &http.Client{}}
	_, err := c.Chat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}
