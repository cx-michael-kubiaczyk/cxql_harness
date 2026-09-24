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
	defaultOpenAIAddress = "https://api.openai.com"
	openAIAPIKeyEnv      = "OPENAI_API_KEY"
)

type openAIClient struct {
	model      string
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewOpenAI creates an OpenAI-backed LLM client talking to the /v1/chat/completions
// endpoint. Requires the OPENAI_API_KEY environment variable.
func NewOpenAI(model string) (LLM, error) {
	apiKey := os.Getenv(openAIAPIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("openai: %s environment variable is not set", openAIAPIKeyEnv)
	}
	return &openAIClient{
		model:      model,
		baseURL:    defaultOpenAIAddress,
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}, nil
}

type openAIFunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type openAITool struct {
	Type     string            `json:"type"`
	Function openAIFunctionDef `json:"function"`
}

type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIFunctionCall `json:"function"`
}

type openAIMessage struct {
	Role      string           `json:"role"`
	Content   *string          `json:"content"`
	ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
}

type openAIChatRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	Tools    []openAITool    `json:"tools,omitempty"`
}

type openAIChoice struct {
	Message      openAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type openAIChatResponse struct {
	Choices []openAIChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func strPtr(s string) *string { return &s }

// Chat sends the harness's history as a single system + user turn — see
// flattenHistory for why the history isn't replayed as native assistant/tool
// turns.
func (c *openAIClient) Chat(ctx context.Context, messages []Message, tools []ToolDef) (Response, error) {
	system, prompt := flattenHistory(messages)

	req := openAIChatRequest{Model: c.model}
	if system != "" {
		req.Messages = append(req.Messages, openAIMessage{Role: "system", Content: strPtr(system)})
	}
	req.Messages = append(req.Messages, openAIMessage{Role: "user", Content: strPtr(prompt)})

	for _, t := range tools {
		req.Tools = append(req.Tools, openAITool{
			Type: "function",
			Function: openAIFunctionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	body, err := json.Marshal(req)
	if err != nil {
		return Response{}, fmt.Errorf("openai: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("openai: failed to build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("openai: request failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("openai: failed to read response: %w", err)
	}

	var chatResp openAIChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return Response{}, fmt.Errorf("openai: failed to parse response (status %s): %w: %s", httpResp.Status, err, strings.TrimSpace(string(respBody)))
	}

	if chatResp.Error != nil {
		return Response{}, fmt.Errorf("openai: %s", chatResp.Error.Message)
	}
	if httpResp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("openai: server returned %s: %s", httpResp.Status, strings.TrimSpace(string(respBody)))
	}
	if len(chatResp.Choices) == 0 {
		return Response{}, fmt.Errorf("openai: response contained no choices")
	}

	msg := chatResp.Choices[0].Message
	resp := Response{}
	if msg.Content != nil {
		resp.Content = *msg.Content
	}
	for _, tc := range msg.ToolCalls {
		var args map[string]any
		if tc.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				return Response{}, fmt.Errorf("openai: failed to parse tool call arguments for %s: %w", tc.Function.Name, err)
			}
		}
		resp.ToolCalls = append(resp.ToolCalls, ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: args,
		})
	}

	return resp, nil
}
