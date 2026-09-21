package harness

import (
	"context"
	"encoding/json"
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
	notepad        *Notepad
	changelog      *Changelog
	verbose        bool
}

func New(logger *logrus.Logger, mcpClient mcpi, llmClient llm.LLM, maxIter int, verbose bool) *Harness {
	return &Harness{
		logger:         logger,
		mcp:            mcpClient,
		llm:            llmClient,
		maxIter:        maxIter,
		queryOriginals: make(map[string]string),
		notepad:        NewNotepad(),
		changelog:      NewChangelog(),
		verbose:        verbose,
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
	messages := NewHistory()

	system := fmt.Sprintf("You are an agent in charge of updating C# code which is used to evaluate source code and discover vulnerabilities.\n%s\n%s\n",
		findingDetails,
		`When a CxSAST scan runs, various "CxQL queries" (written as C# code modules) are run against an AST (abstract syntax tree) representation of a codebase.
Each query returns a list of items representing nodes or dataflow paths through the AST.
The CxSAST product includes a variety of queries covering a range of security vulnerabilities, such as Reflected XSS or SQL Injection.
Queries that represent security vulnerabilities can call other queries to assemble the dataflows from the 'source node' to the 'sink node' in the AST.
Most queries can be 'overridden' allowing users to change the behavior of a specific query.
Query execution can also be chained, so a Tenant-wide override of Reflected_XSS can call the product default version via: result = base.Reflected_XSS();
When addressing false-positive results in a finding, the process follows these steps:
1. Examine the source code involved in the dataflow for the false-positive result.
2. Examine the target query generating the false-positive result to see the query source code and any dependencies on other queries.
3. Examine any other queries and their overrides if they are part of the target query's call chains.
4. Run any queries involved in the target query's call chain to identify points of improvement.
5. Create new application-level overrides, or update existing application-level overrides.
6. Test the updated queries to evaluate the result.
7. Repeat the process as needed until the false positive is removed.

This process enforces only application-level query changes for compliance reasons.
`,
	)
	messages.SetSystem(system)

	for i := 0; i < h.maxIter; i++ {
		messages.SetChangelog(h.getChangelog())
		messages.SetNotes(h.getNotes())
		err := h.loopStep(ctx, messages)
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
func (h *Harness) loopStep(ctx context.Context, messages MessageHistory) error {
	//system += h.getLastQueryInfo() + "\n"
	//system += h.mcp.GetCodeSnippets()
	// add system message to message history object?

	resp, err := h.chat(ctx, messages.History(promptChooseAction, FilterAll), availableTools())
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
	case tooldef.ToolUpdateQuery:
		return h.handleUpdateQuery(ctx, &messages, call)
	case tooldef.ToolSandbox:
		return h.handleSandboxQuery(ctx, &messages, call)
	default:
		//messages.AppendToolResult(call.ID, fmt.Sprintf("unknown tool: %s", call.Name))
		return fmt.Errorf("unknown tool: %s", call.Name)
	}
}

// handleGetQueryInfo fetches CxQL source and hierarchy for a query.
// No sub-loop needed — the LLM reads the info and picks its next action.
func (h *Harness) handleGetQueryInfo(ctx context.Context, messages *MessageHistory, call llm.ToolCall) error {
	lang, group, name, err := queryFromStr(strArg(call.Args, "query"))
	if err != nil {
		return err
	}
	h.logger.Infof("Harness handling call to get query info: %s.%s.%s", lang, group, name)

	// loop over querygetinfo, if it starts with "Error:" then retry (with error in history)
	toolResult := h.mcp.GetQueryInfoFiltered(lang, group, name, []bool{true, true, true, false}, []bool{false, false, true, false})
	messages.AppendToolResult(tooldef.ToolGetQueryInfo, fmt.Sprintf("The call to %s returned the following:\n", tooldef.ToolGetQueryInfo)+toolResult)

	// once done, we call the after-tool function
	h.afterTool(ctx, messages, call)
	return nil
}

// handleRunQuery runs an existing query and pages the results through the LLM.
func (h *Harness) handleRunQuery(ctx context.Context, messages *MessageHistory, call llm.ToolCall) error {
	lang, group, name, err := queryFromStr(strArg(call.Args, "query"))
	if err != nil {
		return err
	}

	h.logger.Infof("Harness handling call to run query: %s.%s.%s", lang, group, name)
	toolResult := h.mcp.RunQuery(lang, group, name)
	messages.AppendToolResult(tooldef.ToolRunQuery, fmt.Sprintf("The call to %s returned the following:\n", tooldef.ToolRunQuery)+toolResult)

	// once done, we call the after-tool function
	h.afterTool(ctx, messages, call)
	return nil
}

// handleSandboxQuery runs an existing query and pages the results through the LLM.
func (h *Harness) handleSandboxQuery(ctx context.Context, messages *MessageHistory, call llm.ToolCall) error {
	code := strArg(call.Args, "code")
	lang := strArg(call.Args, "language")

	group := "CxDefaultQueryGroup"
	name := "CxDefaultQuery"

	h.logger.Infof("Harness handling call to test cxql: %s.%s.%s", lang, group, name)
	toolResult := h.mcp.TestQuery(lang, group, name, code)
	if strings.HasPrefix(toolResult, "Error:") {
		h.logger.Errorf("Failure running query:\n%s", toolResult)
		last_call, result, err := h.handleQueryError(ctx, messages, call, toolResult)
		if err != nil {
			return nil
		}
		toolResult = result
		call = last_call
	}
	messages.AppendToolResult(tooldef.ToolUpdateQuery, fmt.Sprintf("The call to %s returned the following:\n", tooldef.ToolUpdateQuery)+toolResult)

	// once done, we call the after-tool function
	h.afterTool(ctx, messages, call)
	return nil
}

// handleUpdateQuery tests a modified query. On compile success it auto-saves and
// checks whether the original finding is still present.
func (h *Harness) handleUpdateQuery(ctx context.Context, messages *MessageHistory, call llm.ToolCall) error {
	code := strArg(call.Args, "code")
	lang, group, name, err := queryFromStr(strArg(call.Args, "query"))
	if err != nil {
		return err
	}

	h.logger.Infof("Harness handling call to run query: %s.%s.%s", lang, group, name)
	toolResult := h.mcp.TestQuery(lang, group, name, code)
	if strings.HasPrefix(toolResult, "Error:") {
		h.logger.Errorf("Failure running query:\n%s", toolResult)
		last_call, result, err := h.handleQueryError(ctx, messages, call, toolResult)
		if err != nil {
			return err
		}
		toolResult = result
		call = last_call
	}
	messages.AppendToolResult(tooldef.ToolUpdateQuery, fmt.Sprintf("The call to %s returned the following:\n", tooldef.ToolUpdateQuery)+toolResult)

	// decide to save or not
	// for now, just save
	code = strArg(call.Args, "code") // may have changed during error-handling
	save := h.mcp.SaveQuery(lang, group, name, code)
	if strings.HasPrefix(save, "Error:") {
		return fmt.Errorf("Failed to save query: %s", save)
	}

	// once done, we call the after-tool function
	h.afterTool(ctx, messages, call)
	return nil
}

func (h *Harness) handleQueryError(ctx context.Context, messages *MessageHistory, call llm.ToolCall, toolResult string) (last_call llm.ToolCall, last_result string, err error) {
	var hist MessageHistory
	var resp llm.Response
	query := strArg(call.Args, "query")
	lang, group, name, err := queryFromStr(strArg(call.Args, "query"))
	if err != nil {
		return call, toolResult, err
	}

	lastToolDef := getToolDef(call.Name)
	if len(lastToolDef) == 0 {
		err = fmt.Errorf("expected a tool named %s but found none", call.Name)
	}

	code := strArg(call.Args, "code")

	err = h.withRetries(fmt.Sprintf("Fix query %s error", query), 3, func() error {
		fullHist := false
		if fullHist {
			hist = messages.CloneHistory()
			hist.SetSystem("Review the history of events that lead to the error(s) in this query. Update the code to address ")
		} else {
			hist.SetSystem("Update the code to address any errors.")
		}
		messages.AppendToolResult(call.Name, fmt.Sprintf(`The call to %s returned the following:
`+"```"+`
%s
`+"```"+`

The source code for %s is: 
`+"```"+`
%s
`+"```"+`
`, call.Name, toolResult, query, code))

		err = h.withRetries("Generate fixed query "+query, 3, func() error {
			resp, err = h.chat(ctx, messages.History(promptDebugQuery, FilterAll.Except(HistoryFilter{Changelog: false, Notes: false})), lastToolDef)
			if err != nil {
				return fmt.Errorf("LLM call: %w", err)
			}

			if len(resp.ToolCalls) == 0 {
				// LLM produced a plain text response — it may be an error
				return fmt.Errorf("No tool call generated, response was: %s", resp.Content)
			}

			last_call = resp.ToolCalls[0]
			if last_call.Name != call.Name {
				return fmt.Errorf("Received tool call %s but expected %s", last_call.Name, call.Name)
			}
			return nil
		})

		if err != nil {
			return err
		}

		last_call.Args["purpose"] = strArg(call.Args, "purpose")
		h.logger.Infof("Harness handling call to run query: %s.%s.%s", lang, group, name)
		last_result = h.mcp.TestQuery(lang, group, name, code)
		if strings.HasPrefix(last_result, "Error:") {
			toolResult = last_result
			return fmt.Errorf("Query has error")
		} else {
			return nil
		}
	})

	return
}

func (h *Harness) afterTool(ctx context.Context, messages *MessageHistory, prevCall llm.ToolCall) error {
	resp, err := h.chat(ctx, messages.History(promptNotesOnResults, FilterAll.Except(HistoryFilter{Changelog: false, Notes: false})), notepadTool())
	if err != nil {
		return fmt.Errorf("LLM call: %w", err)
	}

	if len(resp.ToolCalls) == 0 {
		// LLM produced a plain text response — it may be an error
		return fmt.Errorf("No tool call generated, response was: %s", resp.Content)
	}

	call := resp.ToolCalls[0]

	if call.Name == tooldef.ToolReview {
		return h.handleNotes(ctx, messages, call, prevCall)
	} else {
		return fmt.Errorf("unknown tool: %s", call.Name)
	}
}

func (h *Harness) handleNotes(ctx context.Context, messages *MessageHistory, call llm.ToolCall, prevCall llm.ToolCall) error {
	summary := strArg(call.Args, "summary")
	prevPurpose := strArg(prevCall.Args, "purpose")
	prevCallString := prevCall.Name + "(\"" + strArg(prevCall.Args, "query") + "\")"
	h.changelog.AddToolCall(prevPurpose, summary, prevCallString)

	if items, ok := call.Args["notes_to_create"].([]any); ok {
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				h.notepad.Create(strArg(m, "type"), strArg(m, "content"))
			}
		}
	}

	if items, ok := call.Args["notes_to_delete"].([]any); ok {
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				h.notepad.Delete(strArg(m, "id"))
			}
		}
	}

	if items, ok := call.Args["notes_to_update"].([]any); ok {
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				h.notepad.Update(strArg(m, "id"), strArg(m, "content"))
			}
		}
	}

	messages.AppendToolResult(tooldef.ToolReview, "Notes updated.")
	return nil
}

func (h *Harness) getNotes() string {
	str, _ := json.Marshal(h.notepad.All())
	return "These are the notes in your notepad:\n" + string(str)
}

func (h *Harness) getChangelog() string {
	return "This is the immutable changelog listing actions executed thus far:\n" + h.changelog.GetChangelog()
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

func (h *Harness) chat(ctx context.Context, messages []llm.Message, tools []llm.ToolDef) (llm.Response, error) {
	str := strings.Builder{}
	for _, msg := range messages {
		min := 50
		max := 100
		if h.verbose || min > len(msg.Content) || max > len(msg.Content) || min+max > len(msg.Content) {
			fmt.Fprintf(&str, "%s: %s\n", msg.Role, msg.Content)
		} else {
			fmt.Fprintf(&str, "%s: %s..\n..%s\n", msg.Role, msg.Content[:min], msg.Content[len(msg.Content)-max:])
		}
	}
	h.logger.Info("LLM Receives messages:\n", str.String())

	response, err := h.llm.Chat(ctx, messages, tools)
	if err != nil {
		h.logger.Errorf("Chat to LLM failed: %s", err)
	}

	msg, _ := json.MarshalIndent(response, "", "  ")
	h.logger.Info("LLM Responds:\n", string(msg))

	return response, err
}

func queryFromStr(query string) (string, string, string, error) {
	parts := strings.Split(query, ".")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("Incorrect number of parts, got '%s' but expected Language.Group.QueryName", query)
	}

	return parts[0], parts[1], parts[2], nil
}

func (h *Harness) withRetries(process string, num_retries int, f func() error) error {
	var err error
	for i := 1; i <= num_retries; i++ {
		h.logger.Debugf("Attempt #%d/%d: %s", i, num_retries, process)
		err = f()
		if err == nil {
			return nil
		}
	}
	return err
}
