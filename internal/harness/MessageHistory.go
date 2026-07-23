package harness

import "github.com/cxpsemea/cxql-harness/internal/llm"

type MessageHistory struct {
	messages []llm.Message
}

func (h *MessageHistory) EnsureUserTurn() {
	if len(h.messages) == 0 || h.messages[len(h.messages)-1].Role == llm.RoleAssistant {
		h.AppendUser(promptChooseAction)
	}
}

func (h *MessageHistory) History(system string) []llm.Message {
	return append([]llm.Message{{
		Role:    llm.RoleSystem,
		Content: system,
	}}, h.messages...)
}

func (h *MessageHistory) AppendAssistant(resp llm.Response) {
	h.messages = append(h.messages, llm.Message{
		Role:      llm.RoleAssistant,
		Content:   resp.Content,
		ToolCalls: resp.ToolCalls,
	})
}

func (h *MessageHistory) AppendToolResult(toolCallID, content string) {
	h.messages = append(h.messages, llm.Message{
		Role:       llm.RoleTool,
		Content:    content,
		ToolCallID: toolCallID,
	})
}

func (h *MessageHistory) AppendUser(content string) {
	h.messages = append(h.messages, llm.Message{
		Role:    llm.RoleUser,
		Content: content,
	})
}
