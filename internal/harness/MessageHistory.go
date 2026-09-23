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

func (h *MessageHistory) History(prompt string, opts HistoryFilter) []llm.Message {
	var messages []llm.Message

	if opts.System {
		messages = append(messages, h.system)
	}
	if opts.Changelog {
		messages = append(messages, h.changelog)
	}
	if opts.Notes {
		messages = append(messages, h.notes)
	}
	if opts.Messages {
		messages = append(messages, h.messages...)
	}
	messages = append(messages, llm.Message{Role: llm.RoleUser, Content: prompt})

	return messages
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
func (h *MessageHistory) CloneHistory() MessageHistory {
	return MessageHistory{
		messages: h.messages,
	}
}

// ClearMessages drops the raw tool-call scratch accumulated during a cycle.
// Call this once a cycle's outcome has been folded into the changelog/notepad.
func (h *MessageHistory) ClearMessages() {
	h.messages = nil
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
