package llm

import (
	"context"
	"fmt"
)

type openAIClient struct {
	model string
}

// NewOpenAI creates an OpenAI-backed LLM client.
// TODO: implement using github.com/sashabaranov/go-openai.
// Requires OPENAI_API_KEY environment variable.
func NewOpenAI(model string) (LLM, error) {
	return &openAIClient{model: model}, nil
}

func (c *openAIClient) Chat(ctx context.Context, messages []Message, tools []ToolDef) (Response, error) {
	return Response{}, fmt.Errorf("openai: not yet implemented")
}
