package harness

import "encoding/json"

type ToolCallEntry struct {
	Purpose string
	Summary string
	Call    string
}

type Changelog struct {
	entries []ToolCallEntry
}

func NewChangelog() *Changelog {
	return &Changelog{
		entries: []ToolCallEntry{},
	}
}

func (c *Changelog) AddToolCall(purpose, summary, call string) {
	c.entries = append(c.entries, ToolCallEntry{Purpose: purpose, Summary: summary, Call: call})
}

func (c *Changelog) GetChangelog() string {
	str, _ := json.Marshal(c.entries)
	return string(str)
}
