package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	defaultAnthropicAddress = "https://api.anthropic.com"
	anthropicAPIKeyEnv      = "ANTHROPIC_API_KEY"
	anthropicVersion        = "2023-06-01"
	anthropicMaxTokens      = 8192
)

type anthropicClient struct {
	model      string
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewAnthropic creates an Anthropic-backed LLM client talking to the
// /v1/messages endpoint. Requires the ANTHROPIC_API_KEY environment variable.
func NewAnthropic(model string) (LLM, error) {
	apiKey := os.Getenv(anthropicAPIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("anthropic: %s environment variable is not set", anthropicAPIKeyEnv)
	}
	return &anthropicClient{
		model:      model,
		baseURL:    defaultAnthropicAddress,
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}, nil
}

type anthropicContentBlock struct {
	Type string `json:"type"`

	// type "text"
	Text string `json:"text,omitempty"`

	// type "tool_use"
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

type anthropicMessage struct {
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
}

type anthropicResponse struct {
	Content    []anthropicContentBlock `json:"content"`
	StopReason string                  `json:"stop_reason"`
	Error      *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Chat sends the harness's history as a single system + user turn — see
// flattenHistory for why the history isn't replayed as native turns.
func (c *anthropicClient) Chat(ctx context.Context, messages []Message, tools []ToolDef) (Response, error) {
	system, prompt := flattenHistory(messages)

	req := anthropicRequest{
		Model:     c.model,
		MaxTokens: anthropicMaxTokens,
		System:    system,
		Messages: []anthropicMessage{
			{Role: "user", Content: []anthropicContentBlock{{Type: "text", Text: prompt}}},
		},
	}
	for _, t := range tools {
		req.Tools = append(req.Tools, anthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Parameters,
		})
	}

	body, err := json.Marshal(req)
	if err != nil {
		return Response{}, fmt.Errorf("anthropic: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("anthropic: failed to build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("anthropic: request failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("anthropic: failed to read response: %w", err)
	}

	var chatResp anthropicResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return Response{}, fmt.Errorf("anthropic: failed to parse response (status %s): %w: %s", httpResp.Status, err, strings.TrimSpace(string(respBody)))
	}

	if chatResp.Error != nil {
		return Response{}, fmt.Errorf("anthropic: %s: %s", chatResp.Error.Type, chatResp.Error.Message)
	}
	if httpResp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("anthropic: server returned %s: %s", httpResp.Status, strings.TrimSpace(string(respBody)))
	}

	resp := Response{}
	var textBuf strings.Builder
	for _, block := range chatResp.Content {
		switch block.Type {
		case "text":
			if textBuf.Len() > 0 {
				textBuf.WriteString("\n")
			}
			textBuf.WriteString(block.Text)
		case "tool_use":
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{
				ID:   block.ID,
				Name: block.Name,
				Args: block.Input,
			})
		}
	}
	resp.Content = textBuf.String()

	return resp, nil
}
