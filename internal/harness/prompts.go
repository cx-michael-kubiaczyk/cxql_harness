package harness

const promptChooseAction = "What would you like to do next? Use one of the available tools. " +
	"Before calling get_query_info or run_query, check the changelog: if you already made this exact call and drew a conclusion from it, do not repeat it. " +
	"Either act on that conclusion with update_query or sandbox, or, if you genuinely need to call it again, state in your purpose what new information you expect this time that the earlier call didn't already give you."
const promptNotesOnResults = "Summarize what you observe in these results and how it relates to the false positive. " +
	"Check the changelog first: if this result only confirms a conclusion you already recorded, say so explicitly and note that no new information was gained, rather than restating it as a new finding."
const promptDebugQuery = "Provide updated code to address the errors."

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
