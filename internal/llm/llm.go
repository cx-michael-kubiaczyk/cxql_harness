package llm

import "context"

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is a single turn in the conversation. ToolCallID is set on RoleTool
// messages to link a result back to the assistant's ToolCall. ToolCalls is set
// on RoleAssistant messages when the model requested tool invocations.
type Message struct {
	Role       Role
	Content    string
	ToolCallID string     `json:"ToolCallID,omitempty"`
	ToolCalls  []ToolCall `json:"ToolCalls,omitempty"`
}

// ToolDef describes a tool the LLM can invoke. Parameters is a JSON Schema
// object (type:"object", properties, required) compatible with OpenAI, Anthropic,
// and Ollama tool-calling formats.
type ToolDef struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// ToolCall is a single tool invocation requested by the model.
type ToolCall struct {
	ID   string
	Name string
	Args map[string]any
}

// Response is the model's reply to a Chat call. Both Content and ToolCalls may
// be populated simultaneously (some models include reasoning text alongside
// tool calls).
type Response struct {
	Content   string
	ToolCalls []ToolCall
}

// LLM is the backend-agnostic interface for language model calls.
// Passing nil or an empty slice for tools disables tool calling for that call
// (useful for note-generation turns where only text output is wanted).
type LLM interface {
	Chat(ctx context.Context, messages []Message, tools []ToolDef) (Response, error)
}
