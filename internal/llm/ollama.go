package llm

import (
	"context"
	"fmt"
)

type ollamaClient struct {
	model string
}

// NewOllama creates an Ollama-backed LLM client.
// TODO: implement using github.com/ollama/ollama/api (or the openai-compat endpoint).
// Requires Ollama running locally (default: http://localhost:11434).
func NewOllama(model string) (LLM, error) {
	return &ollamaClient{model: model}, nil
}

func (c *ollamaClient) Chat(ctx context.Context, messages []Message, tools []ToolDef) (Response, error) {
	return Response{}, fmt.Errorf("ollama: not yet implemented")
}
