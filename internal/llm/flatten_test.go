package llm

import (
	"strings"
	"testing"
)

func TestFlattenHistory(t *testing.T) {
	messages := []Message{
		{Role: RoleSystem, Content: "SYS"},
		{Role: RoleTool, Content: "CHANGELOG"},
		{Role: RoleTool, Content: "NOTES"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "call_1", Name: "get_query_info", Args: map[string]any{"query": "x"}}}},
		{Role: RoleTool, Content: "The call to get_query_info returned the following:\nRESULT", ToolCallID: "call_1"},
		{Role: RoleUser, Content: "PROMPT"},
	}

	system, prompt := flattenHistory(messages)
	if system != "SYS" {
		t.Fatalf("system = %q, want SYS", system)
	}

	positions := map[string]int{}
	for _, want := range []string{"CHANGELOG", "NOTES", "get_query_info", `"query":"x"`, "RESULT", "PROMPT"} {
		idx := strings.Index(prompt, want)
		if idx < 0 {
			t.Fatalf("prompt missing %q: %s", want, prompt)
		}
		positions[want] = idx
	}
	if !(positions["CHANGELOG"] < positions["NOTES"] &&
		positions["NOTES"] < positions["get_query_info"] &&
		positions["get_query_info"] < positions["RESULT"] &&
		positions["RESULT"] < positions["PROMPT"]) {
		t.Fatalf("prompt sections out of order: %+v\nprompt: %s", positions, prompt)
	}
}
