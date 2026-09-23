package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cxpsemea/cxql-harness/internal/llm"
	"github.com/cxpsemea/cxql-harness/internal/tooldef"
	"github.com/sirupsen/logrus"
)

// messagesDir is where per-call request/response dumps are written, see chat().
const messagesDir = "messages"

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
	msgCount       int
	infoStreak     int // consecutive get_query_info/run_query calls with no mutating tool call in between
}

// Info-gathering tool calls (get_query_info, run_query) don't change state; a weak
// or under-guided model can loop on them indefinitely, restating the same
// conclusion instead of acting on it. After infoStreakNudge consecutive
// info-only cycles, the prompt is strengthened to push toward action; after
// infoStreakForce, the info-gathering tools are removed from the choice
// entirely so the model must call update_query, sandbox, or restore_query.
const (
	infoStreakNudge = 3
	infoStreakForce = 5
)

func New(logger *logrus.Logger, mcpClient mcpi, llmClient llm.LLM, maxIter int, verbose bool) *Harness {
	if err := resetMessagesDir(messagesDir); err != nil {
		logger.Warnf("Failed to reset messages dir %s: %s", messagesDir, err)
	}
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

// resetMessagesDir ensures dir exists and is empty, so each run starts fresh.
func resetMessagesDir(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return os.MkdirAll(dir, 0o755)
}

const (
	QUERY_LEVEL_PROJECT     = "Project"
	QUERY_LEVEL_APPLICATION = "Application"
	QUERY_LEVEL_TENANT      = "Tenant"
	QUERY_LEVEL_PRODUCT     = "Product"
)

// Run initialises a session for the given finding URL and drives the reasoning
// loop until the finding is resolved or maxIter is reached.
func (h *Harness) Run(ctx context.Context, findingURL, userPrompt string, TPList, TNList []string) error {
	if err := h.initSession(ctx, findingURL, TPList, TNList); err != nil {
		return fmt.Errorf("init session: %s", err)
	}
	return h.runLoop(ctx, userPrompt)
}

func (h *Harness) initSession(_ context.Context, findingURL string, TPList, TNList []string) error {
	h.logger.Infof("Starting session with target %s (TPs: %+s, TNs: %+s)", findingURL, TPList, TNList)
	if response := strings.TrimSpace(h.mcp.CreateSessionFromURL(findingURL, TPList, TNList)); response != "The session was created successfully and the finding is present." {
		return fmt.Errorf("CreateSessionFromURL: %s", response)
	}
	return nil
}

// runLoop is the top-level state machine. Each iteration is one LLM call that
// produces either a tool invocation or a final text answer.
func (h *Harness) runLoop(ctx context.Context, userPrompt string) error {
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
Queries exist at four levels, from least to most specific: Product (built-in) -> Tenant -> Application -> Project. Most queries can be 'overridden' at the Tenant, Application, or Project level to change their behavior.
Query execution can be chained across levels, so an Application-level override of Reflected_XSS can call the next level up via: result = base.Reflected_XSS();
When addressing false-positive results in a finding, the process follows these steps:
1. Examine the source code involved in the dataflow for the false-positive result.
2. Examine the target query generating the false-positive result to see the query source code and any dependencies on other queries.
3. Examine any other queries and their overrides if they are part of the target query's call chains. You may run and inspect Product-level and Tenant-level queries to understand their behavior, but you must never create, edit, or save Tenant-level overrides.
4. Run any queries involved in the target query's call chain to identify points of improvement.
5. Create new Application-level overrides, or update existing Application-level overrides, to address the false positive.
6. Test the updated queries to evaluate the result.
7. Repeat the process as needed until the false positive is removed.

Query changes are restricted to the Application level for compliance reasons: you must never create, edit, or save Project-level query overrides, and Product-level queries cannot be modified at all.
`,
	)
	if strings.TrimSpace(userPrompt) != "" {
		system += "\nAdditional guidance from the operator for this session:\n" + userPrompt + "\n"
	}
	messages.SetSystem(system)

	for i := 0; i < h.maxIter; i++ {
		messages.SetChangelog(h.getChangelog())
		messages.SetNotes(h.getNotes())
		err := h.loopStep(ctx, &messages)
		if err != nil {
			return fmt.Errorf("failed step: %s", err)
		}
		passed, err := h.TestsPassed()
		if err != nil {
			return fmt.Errorf("check tests passed: %w", err)
		}
		if passed {
			return nil
		}
	}
	return fmt.Errorf("reached max iterations (%d) without resolving the finding", h.maxIter)
}

// loopStep handles each iteration through the process
// each iteration will be a tool call until the process finishes
// the tool calls can be: get query info, run query, and test query
// the tool call updates the back-end state
func (h *Harness) loopStep(ctx context.Context, messages *MessageHistory) error {
	prompt := promptChooseAction
	tools := availableTools()
	switch {
	case h.infoStreak >= infoStreakForce:
		h.logger.Warnf("%d consecutive info-gathering calls with no action; forcing a mutating tool choice", h.infoStreak)
		prompt = promptForceAction
		tools = mutatingTools()
	case h.infoStreak >= infoStreakNudge:
		h.logger.Warnf("%d consecutive info-gathering calls with no action; nudging toward action", h.infoStreak)
		prompt = promptNudgeAction
	}

	// The LLM only sees the changelog + notepad summaries here, not the raw
	// tool traffic from earlier cycles — that scratch buffer is local to a
	// single cycle and is cleared in handleNotes once it's been summarized.
	resp, err := h.chat(ctx, messages.History(prompt, FilterAll.Except(HistoryFilter{Messages: true})), tools)
	if err != nil {
		return fmt.Errorf("LLM call: %w", err)
	}

	if len(resp.ToolCalls) == 0 {
		// LLM produced a plain text response — it may be an error
		return fmt.Errorf("No tool call generated, response was: %s", resp.Content)
	}
	messages.AppendAssistant(resp)

	call := resp.ToolCalls[0]
	switch call.Name {
	case tooldef.ToolGetQueryInfo, tooldef.ToolRunQuery:
		h.infoStreak++
	default:
		h.infoStreak = 0
	}

	switch call.Name {
	case tooldef.ToolGetQueryInfo:
		return h.handleGetQueryInfo(ctx, messages, call)
	case tooldef.ToolRunQuery:
		return h.handleRunQuery(ctx, messages, call)
	case tooldef.ToolUpdateQuery:
		return h.handleUpdateQuery(ctx, messages, call)
	case tooldef.ToolRestoreQuery:
		return h.handleRestoreQuery(ctx, messages, call)
	case tooldef.ToolSandbox:
		return h.handleSandboxQuery(ctx, messages, call)
	default:
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
	messages.AppendToolResult(call.ID, fmt.Sprintf("The call to %s returned the following:\n", tooldef.ToolGetQueryInfo)+toolResult)

	// once done, we call the after-tool function
	return h.afterTool(ctx, messages, call)
}

// handleRunQuery runs an existing query and pages the results through the LLM.
func (h *Harness) handleRunQuery(ctx context.Context, messages *MessageHistory, call llm.ToolCall) error {
	lang, group, name, err := queryFromStr(strArg(call.Args, "query"))
	if err != nil {
		return err
	}
	level := strArg(call.Args, "level")

	h.logger.Infof("Harness handling call to run query: %s.%s.%s", lang, group, name)
	toolResult := h.mcp.RunQuery(level, lang, group, name)
	messages.AppendToolResult(call.ID, fmt.Sprintf("The call to %s returned the following:\n", tooldef.ToolRunQuery)+toolResult)

	// once done, we call the after-tool function
	return h.afterTool(ctx, messages, call)
}

// handleRestoreQuery reverts an Application-level query override to the
// version it had before the first save_query call touched it this session.
func (h *Harness) handleRestoreQuery(ctx context.Context, messages *MessageHistory, call llm.ToolCall) error {
	lang, group, name, err := queryFromStr(strArg(call.Args, "query"))
	if err != nil {
		return err
	}

	h.logger.Infof("Harness handling call to restore query: %s.%s.%s", lang, group, name)
	toolResult := h.mcp.RestoreQuery(QUERY_LEVEL_APPLICATION, lang, group, name)
	messages.AppendToolResult(call.ID, fmt.Sprintf("The call to %s returned the following:\n", tooldef.ToolRestoreQuery)+toolResult)

	// once done, we call the after-tool function
	return h.afterTool(ctx, messages, call)
}

// handleSandboxQuery runs an existing query and pages the results through the LLM.
func (h *Harness) handleSandboxQuery(ctx context.Context, messages *MessageHistory, call llm.ToolCall) error {
	code := strArg(call.Args, "code")
	lang := strArg(call.Args, "language")

	group := "CxDefaultQueryGroup"
	name := "CxDefaultQuery"

	h.logger.Infof("Harness handling call to test cxql: %s.%s.%s", lang, group, name)
	toolResult := h.mcp.TestQuery(QUERY_LEVEL_APPLICATION, lang, group, name, code)
	if strings.HasPrefix(toolResult, "Error:") {
		h.logger.Errorf("Failure running query:\n%s", toolResult)
		// handleQueryError identifies the query being fixed via call.Args["query"];
		// the sandbox tool has no such field, so synthesize one from its fixed group/name.
		call.Args["query"] = lang + "." + group + "." + name
		last_call, result, err := h.handleQueryError(ctx, messages, call, toolResult)
		if err != nil {
			return err
		}
		toolResult = result
		call = last_call
	}
	messages.AppendToolResult(call.ID, fmt.Sprintf("The call to %s returned the following:\n", tooldef.ToolSandbox)+toolResult)

	// once done, we call the after-tool function
	return h.afterTool(ctx, messages, call)
}

// handleUpdateQuery tests a modified query. On compile success it auto-saves and
// checks whether the original finding is still present.
func (h *Harness) handleUpdateQuery(ctx context.Context, messages *MessageHistory, call llm.ToolCall) error {
	code := strArg(call.Args, "code")
	lang, group, name, err := queryFromStr(strArg(call.Args, "query"))
	if err != nil {
		return err
	}

	original_code := h.mcp.GetQueryCode(QUERY_LEVEL_APPLICATION, lang, group, name)
	if strings.HasPrefix(original_code, "Error:") {
		h.logger.Errorf("Failure getting query:\n%s", original_code)
	}

	h.logger.Infof("Harness handling call to run query: %s.%s.%s", lang, group, name)
	toolResult := h.mcp.TestQuery(QUERY_LEVEL_APPLICATION, lang, group, name, code)
	if strings.HasPrefix(toolResult, "Error:") {
		h.logger.Errorf("Failure running query:\n%s", toolResult)
		last_call, result, err := h.handleQueryError(ctx, messages, call, toolResult)
		if err != nil {
			return err
		}
		toolResult = result
		call = last_call
	}
	messages.AppendToolResult(call.ID, fmt.Sprintf("The call to %s returned the following:\n", tooldef.ToolUpdateQuery)+toolResult)

	code = strArg(call.Args, "code") // may have changed during error-handling
	save := h.mcp.SaveQuery(QUERY_LEVEL_APPLICATION, lang, group, name, code)
	if strings.HasPrefix(save, "Error:") {
		return fmt.Errorf("Failed to save query: %s", save)
	}

	// check control projects
	control := h.mcp.CheckControlProjects()
	if strings.HasPrefix(control, "Regression:") {
		h.logger.Debugf("Control check failure after updating app-level query %s.%s.%s with code:\n%s\n\n---------\n%s", lang, group, name, code, control)
		messages.AppendToolResult("check_control_projects", "Running the true-positive and true-negative control projects with this query change resulted in: "+control)
		/*if original_code != "Query doesn't exist" {
			original_code = strings.TrimSuffix(strings.TrimPrefix(original_code, "```csharp\n"), "\n```\n")
			save := h.mcp.SaveQuery(QUERY_LEVEL_APPLICATION, lang, group, name, original_code)
			if strings.HasPrefix(save, "Error:") {
				return fmt.Errorf("Failed to revert query: %s", save)
			}
		}*/
	}

	// once done, we call the after-tool function
	return h.afterTool(ctx, messages, call)
}

func (h *Harness) handleQueryError(ctx context.Context, messages *MessageHistory, call llm.ToolCall, toolResult string) (last_call llm.ToolCall, last_result string, err error) {
	var hist MessageHistory
	var resp llm.Response
	query := strArg(call.Args, "query")
	lang, group, name, err := queryFromStr(query)
	if err != nil {
		return call, toolResult, err
	}

	lastToolDef := getToolDef(call.Name)
	if len(lastToolDef) == 0 {
		return call, toolResult, fmt.Errorf("expected a tool named %s but found none", call.Name)
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
		hist.AppendToolResult(call.Name, fmt.Sprintf(`The call to %s returned the following:
`+"```"+`
%s
`+"```"+`

The source code for %s is:
`+"```"+`
%s
`+"```"+`
`, call.Name, toolResult, query, code))

		err = h.withRetries("Generate fixed query "+query, 3, func() error {
			resp, err = h.chat(ctx, hist.History(promptDebugQuery, FilterAll.Except(HistoryFilter{Changelog: false, Notes: false})), lastToolDef)
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
		//messages.AppendAssistant(resp)

		last_call.Args["purpose"] = strArg(call.Args, "purpose")
		// last_call carries the LLM's fixed code — test that, not the code that
		// just failed, and carry it forward so the next retry (if any) reports
		// on this attempt rather than re-describing the original failure.
		code = strArg(last_call.Args, "code")
		call = last_call

		h.logger.Infof("Harness handling call to run query: %s.%s.%s", lang, group, name)
		last_result = h.mcp.TestQuery(QUERY_LEVEL_APPLICATION, lang, group, name, code)
		if strings.HasPrefix(last_result, "Error:") {
			toolResult = last_result
			return fmt.Errorf("Query has error")
		}
		return nil
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

	// The cycle is now fully captured in the changelog + notepad; drop the raw
	// tool traffic so the next cycle's "choose action" turn starts clean.
	messages.ClearMessages()
	return nil
}

func (h *Harness) getNotes() string {
	str, _ := json.Marshal(h.notepad.All())
	return "These are the notes in your notepad:\n" + string(str)
}

func (h *Harness) getChangelog() string {
	return "This is the immutable changelog listing actions executed thus far:\n" + h.changelog.GetChangelog()
}

func (h *Harness) TestsPassed() (bool, error) {
	control := h.mcp.CheckControlProjects()
	if strings.HasPrefix(control, "Error:") {
		return false, errors.New(control)
	}
	if strings.HasPrefix(control, "Regression:") {
		return false, nil
	}

	status := h.mcp.CheckOriginalFinding()
	if strings.HasPrefix(status, "Error:") {
		return false, errors.New(status)
	}
	if status == "Finding is present" { // expecting to remove the FP
		return false, nil
	}
	return true, nil
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
	h.msgCount++
	h.logger.Infof("LLM Receives #%d:\n------------\n%s\n==============\n\n", h.msgCount, str.String())

	h.dumpMessageFile(h.msgCount, "in", struct {
		Messages []llm.Message
		Tools    []llm.ToolDef
	}{messages, tools})

	response, err := h.llm.Chat(ctx, messages, tools)
	if err != nil {
		h.logger.Errorf("Chat to LLM failed: %s", err)
	}

	msg, _ := json.MarshalIndent(response, "", "  ")
	h.logger.Infof("LLM Responds #%d:\n------------\n%s\n==============\n\n", h.msgCount, msg)

	h.dumpMessageFile(h.msgCount, "out", response)

	return response, err
}

// dumpMessageFile writes v as indented JSON to messages/{num}_{suffix}.txt for
// later review of exactly what was sent to and received from the LLM.
func (h *Harness) dumpMessageFile(num int, suffix string, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		h.logger.Warnf("Failed to marshal %s for message %d: %s", suffix, num, err)
		return
	}
	path := filepath.Join(messagesDir, fmt.Sprintf("%d_%s.txt", num, suffix))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		h.logger.Warnf("Failed to write %s: %s", path, err)
	}
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
