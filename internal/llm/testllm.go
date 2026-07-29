package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cxpsemea/cxql-harness/internal/tooldef"
	"github.com/sirupsen/logrus"
)

type testLLM struct {
	reqCount  int
	responses []Response
	logger    *logrus.Logger
}

func NewTestLLM(logger *logrus.Logger) LLM {
	return &testLLM{
		logger:   logger,
		reqCount: 0,
		responses: []Response{
			{ToolCalls: []ToolCall{
				{
					ID:   "call-query-info-xss",
					Name: tooldef.ToolGetQueryInfo,
					Args: map[string]any{
						"purpose":    "Gather information on the query generating the false-positive.",
						"language":   "JavaScript",
						"group":      "JavaScript_Medium_Threat",
						"query_name": "Missing_HSTS_Header",
					},
				},
			}},
			{ToolCalls: []ToolCall{
				{
					ID:   "update-notes",
					Name: tooldef.ToolReview,
					Args: map[string]any{
						"summary":         "Well that was pointless.",
						"notes_to_create": []any{map[string]any{"type": "Finding", "content": "This Missing_HSTS query is broken"}},
					},
				},
			}},
			{ToolCalls: []ToolCall{
				{
					ID:   "call-run-query",
					Name: tooldef.ToolRunQuery,
					Args: map[string]any{
						"purpose":    "Observe the output of the dependent query Find_HSTS_Sanitize.",
						"language":   "JavaScript",
						"group":      "General",
						"query_name": "Find_HSTS_Sanitize",
					},
				},
			}},
			{ToolCalls: []ToolCall{
				{
					ID:   "update-notes",
					Name: tooldef.ToolReview,
					Args: map[string]any{
						"summary":         "Well that was useful.",
						"notes_to_delete": []any{"note-0001"},
						"notes_to_create": []any{map[string]any{"type": "Finding", "content": "This Find_HSTS_Sanitize query is broken"}},
					},
				},
			}},
		},
	}
}

func (c *testLLM) Chat(ctx context.Context, messages []Message, tools []ToolDef) (Response, error) {
	str := strings.Builder{}
	for _, msg := range messages {
		min := 50
		max := 100
		if min > len(msg.Content) || max > len(msg.Content) || min+max > len(msg.Content) {
			fmt.Fprintf(&str, "%s: %s\n", msg.Role, msg.Content)
		} else {
			fmt.Fprintf(&str, "%s: %s..\n..%s\n", msg.Role, msg.Content[:min], msg.Content[len(msg.Content)-max:])
		}
	}
	c.logger.Info("LLM Receives messages:\n", str.String())

	str.Reset()
	for _, tool := range tools {
		str.WriteString(fmt.Sprintf("%s: %s\n", tool.Name, tool.Description))
	}
	c.logger.Info("Tools:\n", str.String())

	response := c.nextResponse()
	msg, _ := json.MarshalIndent(response, "", "  ")
	c.logger.Info("LLM Responds:\n", string(msg))
	return response, nil
}

func (c *testLLM) nextResponse() Response {
	if c.reqCount >= len(c.responses) {
		return Response{}
	}
	res := c.responses[c.reqCount]
	c.reqCount++
	return res
}
