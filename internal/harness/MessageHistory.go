package harness

import "github.com/cxpsemea/cxql-harness/internal/llm"

type MessageHistory struct {
	system    llm.Message
	changelog llm.Message
	notes     llm.Message
	messages  []llm.Message
}

func NewHistory() MessageHistory {
	return MessageHistory{
		system:    llm.Message{Role: llm.RoleSystem},
		changelog: llm.Message{Role: llm.RoleTool},
		notes:     llm.Message{Role: llm.RoleTool},
	}
}

func (h *MessageHistory) History(prompt string) []llm.Message {
	return append(
		[]llm.Message{h.system, h.changelog, h.notes},
		append(h.messages, llm.Message{Role: llm.RoleUser, Content: prompt})...,
	)
}

func (h *MessageHistory) SetSystem(system string) {
	h.system.Content = system
}
func (h *MessageHistory) SetChangelog(changelog string) {
	h.changelog.Content = changelog
}
func (h *MessageHistory) SetNotes(notes string) {
	h.notes.Content = notes
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
