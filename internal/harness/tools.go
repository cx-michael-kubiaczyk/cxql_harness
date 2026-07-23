package harness

import (
	"github.com/cxpsemea/cxql-harness/internal/llm"
	"github.com/cxpsemea/cxql-harness/internal/tooldef"
)

// notepadTool returns the single edit_notes tool definition for use in
// dedicated note-taking LLM turns (not part of availableTools).
func notepadTool() []llm.ToolDef {
	return []llm.ToolDef{{
		Name:        tooldef.ToolEditNotes,
		Description: "Create, update, or delete notes in your scratchpad. Use this to record findings, decisions, and observations for later reference.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"notes_to_create": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"type":    map[string]any{"type": "string", "enum": []string{"Task", "Finding"}, "description": "Category of the note"},
							"content": map[string]any{"type": "string"},
						},
						"required": []string{"type", "content"},
					},
				},
				"notes_to_delete": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id": map[string]any{"type": "string"},
						},
						"required": []string{"id"},
					},
				},
				"notes_to_update": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id":      map[string]any{"type": "string"},
							"content": map[string]any{"type": "string", "description": "New replacement content"},
						},
						"required": []string{"id", "content"},
					},
				},
			},
		},
	}}
}

// availableTools returns the tool definitions exposed to the LLM at each
// CHOOSE_ACTION turn. SaveQuery and CheckOriginalFinding are intentionally
// omitted — the harness calls them automatically after a successful TestQuery.
func availableTools() []llm.ToolDef {
	return []llm.ToolDef{
		{
			Name:        tooldef.ToolGetQueryInfo,
			Description: "Retrieve the CxQL source code and call hierarchy for a specific query. Use this to understand how a query works before modifying it.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"language":   map[string]any{"type": "string", "description": "Programming language, e.g. Java, CSharp"},
					"group":      map[string]any{"type": "string", "description": "Query group/category, e.g. CxDefaultQueryJava"},
					"query_name": map[string]any{"type": "string", "description": "Query name, e.g. Reflected_XSS"},
				},
				"required": []string{"language", "group", "query_name"},
			},
		},
		{
			Name:        tooldef.ToolRunQuery,
			Description: "Run an existing CxQL query as-is and view its results shown inline with the source code.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"language":   map[string]any{"type": "string"},
					"group":      map[string]any{"type": "string"},
					"query_name": map[string]any{"type": "string"},
				},
				"required": []string{"language", "group", "query_name"},
			},
		},
		{
			Name:        tooldef.ToolTestQuery,
			Description: "Test a modified version of a CxQL query without permanently saving it. On compile success the result is saved automatically and the original finding is re-checked. On compile error the error is returned so you can fix it.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"language":   map[string]any{"type": "string"},
					"group":      map[string]any{"type": "string"},
					"query_name": map[string]any{"type": "string"},
					"code":       map[string]any{"type": "string", "description": "Complete CxQL source code for the query"},
				},
				"required": []string{"language", "group", "query_name", "code"},
			},
		},
	}
}
