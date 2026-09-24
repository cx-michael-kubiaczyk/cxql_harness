package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIChatRequestAndResponse(t *testing.T) {
	var gotReq openAIChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing/incorrect Authorization header: %q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("failed to decode request: %s", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{
				"message": {
					"role": "assistant",
					"content": null,
					"tool_calls": [{"id": "call_9", "type": "function", "function": {"name": "get_query_info", "arguments": "{\"query\":\"javascript.Group.Q\"}"}}]
				},
				"finish_reason": "tool_calls"
			}]
		}`))
	}))
	defer srv.Close()

	c := &openAIClient{model: "gpt-4o", baseURL: srv.URL, apiKey: "test-key", httpClient: &http.Client{}}

	messages := []Message{
		{Role: RoleSystem, Content: "SYS"},
		{Role: RoleTool, Content: "CHANGELOG"},
		{Role: RoleUser, Content: "PROMPT"},
	}
	tools := []ToolDef{{Name: "get_query_info", Description: "d", Parameters: map[string]any{"type": "object"}}}

	resp, err := c.Chat(context.Background(), messages, tools)
	if err != nil {
		t.Fatalf("Chat: %s", err)
	}

	// The whole history collapses into exactly one system + one user message
	// — no assistant/tool turns replayed, so there's no alternation or
	// tool_call_id pairing for the API to reject.
	if len(gotReq.Messages) != 2 {
		t.Fatalf("got %d messages, want 2 (system + user): %+v", len(gotReq.Messages), gotReq.Messages)
	}
	if gotReq.Messages[0].Role != "system" || gotReq.Messages[0].Content == nil || *gotReq.Messages[0].Content != "SYS" {
		t.Fatalf("unexpected system message: %+v", gotReq.Messages[0])
	}
	if gotReq.Messages[1].Role != "user" || gotReq.Messages[1].Content == nil {
		t.Fatalf("unexpected user message: %+v", gotReq.Messages[1])
	}
	userContent := *gotReq.Messages[1].Content
	if !strings.Contains(userContent, "CHANGELOG") || !strings.Contains(userContent, "PROMPT") {
		t.Fatalf("user message missing expected content: %q", userContent)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("got %d tool calls, want 1: %+v", len(resp.ToolCalls), resp.ToolCalls)
	}
	tc := resp.ToolCalls[0]
	if tc.ID != "call_9" || tc.Name != "get_query_info" {
		t.Fatalf("unexpected tool call: %+v", tc)
	}
	if tc.Args["query"] != "javascript.Group.Q" {
		t.Fatalf("unexpected tool call args: %+v", tc.Args)
	}
}

func TestOpenAIChatErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": {"message": "invalid api key", "type": "invalid_request_error"}}`))
	}))
	defer srv.Close()

	c := &openAIClient{model: "gpt-4o", baseURL: srv.URL, apiKey: "bad-key", httpClient: &http.Client{}}
	_, err := c.Chat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}
