package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

const defaultOllamaAddress = "http://localhost:11434"

type ollamaClient struct {
	model      string
	baseURL    string
	httpClient *http.Client
}

// NewOllama creates an Ollama-backed LLM client talking to the native
// /api/chat endpoint. address is the base URL of the Ollama server (e.g.
// "http://localhost:11434" or "http://my-gpu-box:11434"); if empty, it
// defaults to http://localhost:11434.
func NewOllama(model string, address string) (LLM, error) {
	if address == "" {
		address = defaultOllamaAddress
	}
	address = strings.TrimRight(address, "/")

	return &ollamaClient{
		model:      model,
		baseURL:    address,
		httpClient: &http.Client{},
	}, nil
}

type ollamaFunctionCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type ollamaToolCall struct {
	Function ollamaFunctionCall `json:"function"`
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type ollamaTool struct {
	Type     string             `json:"type"`
	Function ollamaToolFunction `json:"function"`
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Tools    []ollamaTool    `json:"tools,omitempty"`
	Stream   bool            `json:"stream"`
}

type ollamaChatResponse struct {
	Message ollamaMessage `json:"message"`
	Done    bool          `json:"done"`
	Error   string        `json:"error,omitempty"`
}

func toOllamaRole(r Role) string {
	switch r {
	case RoleSystem:
		return "system"
	case RoleUser:
		return "user"
	case RoleAssistant:
		return "assistant"
	case RoleTool:
		return "tool"
	default:
		return string(r)
	}
}

func (c *ollamaClient) Chat(ctx context.Context, messages []Message, tools []ToolDef) (Response, error) {
	req := ollamaChatRequest{
		Model:  c.model,
		Stream: false,
	}

	for _, m := range messages {
		om := ollamaMessage{
			Role:    toOllamaRole(m.Role),
			Content: m.Content,
		}
		for _, tc := range m.ToolCalls {
			om.ToolCalls = append(om.ToolCalls, ollamaToolCall{
				Function: ollamaFunctionCall{
					Name:      tc.Name,
					Arguments: tc.Args,
				},
			})
		}
		req.Messages = append(req.Messages, om)
	}

	for _, t := range tools {
		req.Tools = append(req.Tools, ollamaTool{
			Type: "function",
			Function: ollamaToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	body, err := json.Marshal(req)
	if err != nil {
		return Response{}, fmt.Errorf("ollama: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("ollama: failed to build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("ollama: request to %s failed: %w", c.baseURL, err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("ollama: failed to read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("ollama: server at %s returned %s: %s", c.baseURL, httpResp.Status, strings.TrimSpace(string(respBody)))
	}

	var chatResp ollamaChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return Response{}, fmt.Errorf("ollama: failed to parse response: %w", err)
	}
	if chatResp.Error != "" {
		return Response{}, fmt.Errorf("ollama: %s", chatResp.Error)
	}

	resp := Response{Content: chatResp.Message.Content}
	for i, tc := range chatResp.Message.ToolCalls {
		resp.ToolCalls = append(resp.ToolCalls, ToolCall{
			ID:   fmt.Sprintf("call_%d", i),
			Name: tc.Function.Name,
			Args: tc.Function.Arguments,
		})
	}

	// Some models (e.g. qwen3-coder) don't reliably populate Ollama's native
	// tool_calls field and instead emit "<function=name><parameter=k>v</parameter></function>"
	// markup inline in content. Fall back to scraping it out of the text.
	if len(resp.ToolCalls) == 0 {
		if remaining, calls := parseTextToolCalls(resp.Content); len(calls) > 0 {
			resp.Content = remaining
			resp.ToolCalls = calls
		}
	}

	return resp, nil
}

var (
	functionBlockRe = regexp.MustCompile(`(?s)<function=([\w.\-]+)>(.*?)</function>`)
	parameterRe     = regexp.MustCompile(`(?s)<parameter=([\w.\-]+)>(.*?)</parameter>`)
	toolCallTagRe   = regexp.MustCompile(`(?s)</?tool_call>`)
)

// parseTextToolCalls scrapes "<function=name><parameter=k>v</parameter>...</function>"
// blocks out of raw assistant text, returning the remaining prose (with the
// blocks and any stray <tool_call> wrapper tags stripped) and the tool calls found.
func parseTextToolCalls(content string) (string, []ToolCall) {
	matches := functionBlockRe.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return content, nil
	}

	var calls []ToolCall
	for i, m := range matches {
		args := map[string]any{}
		for _, p := range parameterRe.FindAllStringSubmatch(m[2], -1) {
			args[p[1]] = strings.TrimSpace(p[2])
		}
		calls = append(calls, ToolCall{
			ID:   fmt.Sprintf("call_%d", i),
			Name: strings.TrimSpace(m[1]),
			Args: args,
		})
	}

	remaining := functionBlockRe.ReplaceAllString(content, "")
	remaining = toolCallTagRe.ReplaceAllString(remaining, "")
	remaining = strings.TrimSpace(remaining)

	return remaining, calls
}
