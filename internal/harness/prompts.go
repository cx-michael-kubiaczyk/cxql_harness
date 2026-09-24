package harness

const promptChooseAction = "What would you like to do next? Use one of the available tools. " +
	"Before calling get_query_info or run_query, check the changelog: if you already made this exact call and drew a conclusion from it, do not repeat it. " +
	"Either act on that conclusion with update_query or sandbox, or, if you genuinely need to call it again, state in your purpose what new information you expect this time that the earlier call didn't already give you."
const promptNotesOnResults = "Summarize what you observe in these results and how it relates to the false positive. " +
	"Check the changelog first: if this result only confirms a conclusion you already recorded, say so explicitly and note that no new information was gained, rather than restating it as a new finding. " +
	resultsSemanticsReminder
const promptDebugQuery = "Provide updated code to address the errors."

// resultsSemanticsReminder corrects a specific, observed misreading: a run
// where the model wrote "Final verification confirms... successfully
// resolved the false positive" after seeing the flagged code appear in
// run_query's output — inverting the actual meaning (appearing in a
// vulnerability query's results means it is still being flagged) — while the
// harness's own authoritative check (CheckOriginalFinding) still reported the
// finding present. The model's own narration is not verification; only the
// harness's pass/fail check is.
const resultsSemanticsReminder = "Remember what a query's results mean: for a vulnerability-detection query like Missing_HSTS_Header, a code location appearing in the results means it IS currently being flagged as a problem — that is the finding, not evidence that it has been fixed. " +
	"Success looks like the opposite: after your change, the specific false-positive location no longer appears in the query's results at all, while genuine vulnerabilities in the true-positive control project still do. " +
	"Seeing the false-positive's code (e.g. valid helmet middleware config) show up in a query's output is not a good sign by itself — it means that code is still being matched/flagged. Do not declare the false positive resolved based on your own reading of a result; only the harness's own pass/fail check after your change is authoritative."

// cxqlSyntaxReminder is prepended to the system message inside the error-fix
// retry loop (handleQueryError), which otherwise only tells the model "update
// the code to address any errors" with no reminder of what valid CxQL looks
// like. Without this, a compiler error near the top of a C# file (e.g. "using
// System; ... Identifier expected") has been observed to make the model
// conclude the query must be rewritten in JavaScript, because the query name's
// language prefix (e.g. "javascript.Group.Query") refers to the scanned
// source language, not the language the query itself is written in.
const cxqlSyntaxReminder = "CxQL query code is always written in C#, regardless of the language prefix in the query's name (e.g. \"javascript.\" means the query inspects JavaScript source code — it does not mean the query itself is written in JavaScript). " +
	"A query body is typically a single expression assigned to `result`, built from calls to other queries and CxList methods, e.g.: result = Common_Medium_Threat.Missing_HSTS_Header().SanitizeCxList(Find_HSTS_Sanitize()); It is not a full class, namespace, or function/method declaration. " +
	"Never switch to JavaScript or another language to work around a compiler error. " +
	"Only call functions and methods you have actually seen returned by get_query_info or run_query for this or a related query (e.g. SanitizeCxList, Find_HSTS_Sanitize) — never invent a plausible-sounding function name (e.g. Find_Http_Response, GetMembers) that you have not directly observed in a tool result; if you are not sure a helper exists, look it up first instead of guessing. " +
	"If get_query_info reports a query as not found under one language, that does not mean the query doesn't exist — shared queries such as Common_* groups are often only registered under the special language \"Common\" rather than the scanned source language; try that before concluding a base query is missing or unfixable."

// promptNudgeAction is used once infoStreakNudge consecutive cycles have
// called only get_query_info/run_query, to push the model toward acting on
// whatever conclusion it has already reached instead of re-gathering it.
const promptNudgeAction = "You have spent several cycles in a row only gathering information, without attempting a fix. " +
	"Review your notes: if you already have a working theory of the false positive's cause, write and submit an update_query (or test it first with sandbox) now instead of re-confirming what you already know. " +
	"Only continue gathering information if there is a specific, new fact you are missing that you cannot get from your existing notes."

// promptForceAction is used once infoStreakForce consecutive cycles have
// called only get_query_info/run_query; at that point get_query_info and
// run_query are removed from the tool choices entirely so the model must act.
const promptForceAction = "You have spent too many cycles only gathering information without attempting a fix, so get_query_info and run_query are no longer available this turn. " +
	"Use update_query to submit a fix based on what you already know, sandbox to test an idea first, or restore_query to undo a change, then continue from there."
