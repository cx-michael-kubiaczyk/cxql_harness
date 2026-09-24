package harness

import (
	"strings"

	"github.com/cxpsemea/cxql-harness/internal/llm"
	"github.com/cxpsemea/cxql-harness/internal/tooldef"
)

// notepadTool returns the single edit_notes tool definition for use in
// dedicated note-taking LLM turns (not part of availableTools).
func notepadTool() []llm.ToolDef {
	return []llm.ToolDef{
		{
			Name:        tooldef.ToolReview,
			Description: "Create, update, or delete notes in your scratchpad. Use this to record findings, decisions, and observations for later reference. Write a summary of the result with respect to the purpose of the tool call and indicate if it was useful.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"summary": map[string]any{"type": "string", "description": "Summary of the tool call"},
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
						"type":  "array",
						"items": map[string]any{"type": "string"},
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
				"required": []string{"summary"},
			},
		},
	}
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
					"purpose": map[string]any{"type": "string", "description": "Purpose for this tool call, e.g. Information Gathering"},
					"query":   map[string]any{"type": "string", "description": "The query in the format Language.Group.QueryName"},
				},
				"required": []string{"purpose", "query"},
			},
		},
		{
			Name:        tooldef.ToolSearchQueries,
			Description: "Search the names of all known queries for a substring. Use this to find a query's Language.Group.QueryName path when you only know its short name (e.g. a sanitizer helper referenced by another query), instead of guessing at get_query_info with different group names.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"purpose":   map[string]any{"type": "string", "description": "Purpose for this tool call, e.g. Locate the group for a helper query seen in another query's source"},
					"substring": map[string]any{"type": "string", "description": "Text to search for within query names, e.g. HSTS_Sanitize"},
				},
				"required": []string{"purpose", "substring"},
			},
		},
		{
			Name:        tooldef.ToolRunQuery,
			Description: "Run an existing CxQL query as-is and view its results shown inline with the source code.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"purpose": map[string]any{"type": "string", "description": "Purpose for this tool call, e.g. Information Gathering"},
					"query":   map[string]any{"type": "string", "description": "The query in the format Language.Group.QueryName"},
					"level":   map[string]any{"type": "string", "description": "The query level: Product, Tenant, or Application"},
				},
				"required": []string{"purpose", "query"},
			},
		},
		{
			Name:        tooldef.ToolUpdateQuery,
			Description: "Update the code for an application-level query override.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"purpose": map[string]any{"type": "string", "description": "Purpose for this tool call, e.g. Test behavior when matching additional input XYZ"},
					"query":   map[string]any{"type": "string", "description": "The query in the format Language.Group.QueryName"},
					"code":    map[string]any{"type": "string", "description": "Complete CxQL source code for the query"},
				},
				"required": []string{"purpose", "query", "code"},
			},
		},
		{
			Name:        tooldef.ToolRestoreQuery,
			Description: "Revert an Application-level query override back to the version it had before your first edit in this session, undoing any saved changes.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"purpose": map[string]any{"type": "string", "description": "Purpose for this tool call, e.g. Undo a regression introduced by the last edit"},
					"query":   map[string]any{"type": "string", "description": "The query in the format Language.Group.QueryName"},
				},
				"required": []string{"purpose", "query"},
			},
		},
		{
			Name:        tooldef.ToolSandbox,
			Description: "Test CxQL code without changing existing queries.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"purpose":  map[string]any{"type": "string", "description": "Purpose for this tool call, e.g. Test behavior when matching additional input XYZ"},
					"language": map[string]any{"type": "string", "description": "The language for the query as provided by other tool calls, the first part of the query when written in the format Language.Group.QueryName"},
					"code":     map[string]any{"type": "string", "description": "Complete CxQL source code for the query"},
				},
				"required": []string{"purpose", "language", "code"},
			},
		},
	}
}

// mutatingTools returns only the tools that change query state (update_query,
// restore_query, sandbox), for use once the harness has forced the model past
// an info-gathering loop and it must act rather than keep inspecting.
func mutatingTools() []llm.ToolDef {
	var out []llm.ToolDef
	for _, t := range availableTools() {
		if t.Name == tooldef.ToolUpdateQuery || t.Name == tooldef.ToolRestoreQuery || t.Name == tooldef.ToolSandbox {
			out = append(out, t)
		}
	}
	return out
}

func getToolDef(name string) []llm.ToolDef {
	tools := availableTools()
	for _, t := range tools {
		if strings.EqualFold(t.Name, name) {
			return []llm.ToolDef{t}
		}
	}
	return nil
}
