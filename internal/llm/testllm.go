package llm

import (
	"context"
	"encoding/json"

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
						"language":   "JavaScript",
						"group":      "JavaScript_Medium_Threat",
						"query_name": "Missing_HSTS_Header",
					},
				},
			}},
			{ToolCalls: []ToolCall{
				{
					ID:   "update-notes",
					Name: tooldef.ToolEditNotes,
					Args: map[string]any{},
				},
			}},
		},
	}
}

func (c *testLLM) Chat(ctx context.Context, messages []Message, tools []ToolDef) (Response, error) {
	msg, _ := json.MarshalIndent(messages, "", "  ")
	c.logger.Info("LLM Receives:\nPrompt:\n", string(msg))
	msg, _ = json.MarshalIndent(tools, "", "  ")
	c.logger.Info("Tools:\n", string(msg))
	response := c.nextResponse()
	msg, _ = json.MarshalIndent(response, "", "  ")
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
