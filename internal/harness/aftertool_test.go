package harness

import (
	"context"
	"testing"

	"github.com/cxpsemea/cxql-harness/internal/llm"
	"github.com/cxpsemea/cxql-harness/internal/tooldef"
	"github.com/sirupsen/logrus"
)

// scriptedLLM is a minimal LLM stub for exercising specific harness code
// paths not covered by the existing canned TestLLM script.
type scriptedLLM struct {
	responses []llm.Response
	i         int
}

func (s *scriptedLLM) Chat(ctx context.Context, messages []llm.Message, tools []llm.ToolDef) (llm.Response, error) {
	if s.i >= len(s.responses) {
		return llm.Response{}, nil
	}
	r := s.responses[s.i]
	s.i++
	return r, nil
}

// TestAfterToolCapturesMisplacedToolCall reproduces the exact failure seen in
// the field: during the note-taking turn, the model proposes a real action
// (search_queries) instead of tool_review. Before the fix this was silently
// discarded after burning retries; after the fix it should be captured as a
// Task note and the turn should complete without exhausting all 3 retries.
func TestAfterToolCapturesMisplacedToolCall(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel) // keep test output quiet

	h := &Harness{
		logger:         logger,
		mcp:            NewTestMCP(),
		notepad:        NewNotepad(),
		changelog:      NewChangelog(),
		queryOriginals: map[string]string{},
	}

	llmClient := &scriptedLLM{
		responses: []llm.Response{
			{
				Content: "I should check where Find_HSTS_Sanitize lives.",
				ToolCalls: []llm.ToolCall{
					{
						ID:   "misplaced",
						Name: tooldef.ToolSearchQueries,
						Args: map[string]any{
							"purpose":   "Locate the sanitizer helper's group",
							"substring": "Find_HSTS_Sanitize",
						},
					},
				},
			},
			// If the bug were still present, this second scripted response
			// would be consumed by a wasted retry. If the fix works, this
			// entry should never be reached because the first response is
			// captured immediately without retrying.
			{
				ToolCalls: []llm.ToolCall{
					{ID: "should-not-be-reached", Name: tooldef.ToolReview, Args: map[string]any{"summary": "should not get here"}},
				},
			},
		},
	}
	h.llm = llmClient

	messages := NewHistory()
	messages.SetSystem("test system prompt")

	prevCall := llm.ToolCall{
		ID:   "prev",
		Name: tooldef.ToolGetQueryInfo,
		Args: map[string]any{"purpose": "test", "query": "javascript.JavaScript_Medium_Threat.Missing_HSTS_Header"},
	}

	err := h.afterTool(context.Background(), &messages, prevCall)
	if err != nil {
		t.Fatalf("afterTool returned error: %v", err)
	}

	if llmClient.i != 1 {
		t.Fatalf("expected exactly 1 LLM call (misplaced call captured without retry), got %d", llmClient.i)
	}

	notes := h.notepad.All()
	if len(notes) != 1 {
		t.Fatalf("expected 1 note capturing the misplaced call, got %d: %+v", len(notes), notes)
	}
	if notes[0].Type != "Task" {
		t.Fatalf("expected captured note to be a Task, got %q", notes[0].Type)
	}
	t.Logf("captured task note: %s", notes[0].Content)
}
