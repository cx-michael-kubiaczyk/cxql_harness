package harness

import (
	"context"
	"fmt"
	"strings"

	"github.com/cxpsemea/cxql-harness/internal/llm"
	"github.com/cxpsemea/cxql-harness/internal/tooldef"
	"github.com/sirupsen/logrus"
)

// Harness drives the false-positive resolution loop.
type Harness struct {
	logger         *logrus.Logger
	mcp            mcpi
	llm            llm.LLM
	maxIter        int
	queryOriginals map[string]string // queryKey -> original CxQL, saved before first edit
}

func New(logger *logrus.Logger, mcpClient mcpi, llmClient llm.LLM, maxIter int) *Harness {
	return &Harness{
		logger:         logger,
		mcp:            mcpClient,
		llm:            llmClient,
		maxIter:        maxIter,
		queryOriginals: make(map[string]string),
	}
}

// Run initialises a session for the given finding URL and drives the reasoning
// loop until the finding is resolved or maxIter is reached.
func (h *Harness) Run(ctx context.Context, findingURL, userPrompt string) error {
	if err := h.initSession(ctx, findingURL); err != nil {
		return fmt.Errorf("init session: %s", err)
	}
	return h.runLoop(ctx)
}

func (h *Harness) initSession(_ context.Context, findingURL string) error {
	if response := strings.TrimSpace(h.mcp.CreateSessionFromURL(findingURL)); response != "The session was created successfully and the finding is present." {
		return fmt.Errorf("CreateSessionFromURL: %s", response)
	}
	return nil
}

// runLoop is the top-level state machine. Each iteration is one LLM call that
// produces either a tool invocation or a final text answer.
func (h *Harness) runLoop(ctx context.Context) error {
	/*
		The MCP server manages the state of an application/environment
		Some properties of the environment do not change:
		- High Level Description (via mcp.GetHLD) does not change and describes the overall process
		- Finding Details do not change, they represent the original false-positive reported by the CxSAST tool

		However there are other properties that will change:
		- GetCodeSnippets returns the original source code scanned by the CxSAST tool augmented with comments
			which show the results of running different queries (comments denoting dataflow, sources, sinks etc).
			The comments will have only the last-run-query
		- The harness will track the last query that was run, and automatically add information about that query
			to the messages sent to the llm
		- The harness will track if the FP is still present
		- The harness will track if the FP/TP test cases have passed

	*/
	findingDetails := h.mcp.GetFindingDetails()
	if strings.HasPrefix(findingDetails, "Error:") {
		return fmt.Errorf("Error while getting finding details. %s", findingDetails)
	}
	messages := MessageHistory{}

	system := fmt.Sprintf("You are an agent in charge of updating C# code which is used to evaluate source code and discover vulnerabilities.\n%s\n%s\n",
		findingDetails,
		h.mcp.GetHLD(),
	)

	for i := 0; i < h.maxIter; i++ {
		err := h.loopStep(ctx, system, messages)
		if err != nil {
			return fmt.Errorf("failed step: %s", err)
		}
		if h.TestsPassed() {
			return nil
		}
	}
	return fmt.Errorf("reached max iterations (%d) without resolving the finding", h.maxIter)
}

// loopStep handles each iteration through the process
// each iteration will be a tool call until the process finishes
// the tool calls can be: get query info, run query, and test query
// the tool call updates the back-end state
func (h *Harness) loopStep(ctx context.Context, system string, messages MessageHistory) error {
	system += h.getLastQueryInfo() + "\n"
	system += h.mcp.GetCodeSnippets()
	// add system message to message history object?

	messages.EnsureUserTurn()
	resp, err := h.llm.Chat(ctx, messages.History(system), availableTools())
	if err != nil {
		return fmt.Errorf("LLM call: %w", err)
	}

	if len(resp.ToolCalls) == 0 {
		// LLM produced a plain text response — it may be an error
		return fmt.Errorf("No tool call generated, response was: %s", resp.Content)
	}

	call := resp.ToolCalls[0]
	switch call.Name {
	case tooldef.ToolGetQueryInfo:
		return h.handleGetQueryInfo(ctx, &messages, call)
	case tooldef.ToolRunQuery:
		return h.handleRunQuery(ctx, &messages, call)
	case tooldef.ToolTestQuery:
		return h.handleTestQuery(ctx, &messages, call)
	default:
		//messages.AppendToolResult(call.ID, fmt.Sprintf("unknown tool: %s", call.Name))
		return fmt.Errorf("unknown tool: %s", call.Name)
	}
}

// handleGetQueryInfo fetches CxQL source and hierarchy for a query.
// No sub-loop needed — the LLM reads the info and picks its next action.
func (h *Harness) handleGetQueryInfo(ctx context.Context, messages *MessageHistory, call llm.ToolCall) error {
	lang, group, name := strArg(call.Args, "language"), strArg(call.Args, "group"), strArg(call.Args, "query_name")
	h.logger.Infof("Call to get query info: %s.%s.%s", lang, group, name)

	//info := h.mcp.GetQueryInfo(lang, group, name)

	return nil
}

// handleRunQuery runs an existing query and pages the results through the LLM.
func (h *Harness) handleRunQuery(ctx context.Context, messages *MessageHistory, call llm.ToolCall) error {
	//lang, group, name := strArg(call.Args, "language"), strArg(call.Args, "group"), strArg(call.Args, "query_name")
	//results := h.mcp.RunQuery(lang, group, name)

	return nil
}

// handleTestQuery tests a modified query. On compile success it auto-saves and
// checks whether the original finding is still present.
func (h *Harness) handleTestQuery(ctx context.Context, messages *MessageHistory, call llm.ToolCall) error {
	//lang, group, name, code := strArg(call.Args, "language"), strArg(call.Args, "group"), strArg(call.Args, "query_name"), strArg(call.Args, "code")

	//results := h.mcp.TestQuery(lang, group, name, code)

	// Finding persists — show the status so the LLM can continue reasoning.
	//h.appendUser(fmt.Sprintf("Finding still present after save: %s\nContinue exploring.", findingStatus))
	return nil
}

func (h *Harness) getLastQueryInfo() string {
	return ""
}

func (h *Harness) TestsPassed() bool {
	return false
}

func strArg(args map[string]any, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
