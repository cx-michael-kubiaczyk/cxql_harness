package llm

import (
	"context"
	"fmt"
)

type anthropicClient struct {
	model string
}

// NewAnthropic creates an Anthropic-backed LLM client.
// TODO: implement using github.com/anthropics/anthropic-sdk-go.
// Requires ANTHROPIC_API_KEY environment variable.
func NewAnthropic(model string) (LLM, error) {
	return &anthropicClient{model: model}, nil
}

func (c *anthropicClient) Chat(ctx context.Context, messages []Message, tools []ToolDef) (Response, error) {
	return Response{}, fmt.Errorf("anthropic: not yet implemented")
}
