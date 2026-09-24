package llm

import (
	"encoding/json"
	"fmt"
	"strings"
)

// The harness's own history isn't a real back-and-forth conversation: most
// calls carry no assistant turns at all (system + accumulated state
// summaries + one instruction), and the ones that do exist only to relay one
// cycle's raw tool call/result into a single following reply, never built on
// over multiple rounds (see CallSiteAfterTool, CallSiteDebugQuery in
// harness.go). Replaying that as native assistant/tool-call turns would mean
// satisfying each provider's own strict, incompatible rules about how those
// turns must be paired — and the harness's data doesn't always cleanly fit
// either shape (the debug-fix loop's tool-result note has no real preceding
// tool call to pair it with). flattenHistory sidesteps all of that by
// narrating everything as plain text inside one system + one user turn.
func flattenHistory(messages []Message) (system, prompt string) {
	var body strings.Builder
	write := func(s string) {
		if strings.TrimSpace(s) == "" {
			return
		}
		if body.Len() > 0 {
			body.WriteString("\n\n")
		}
		body.WriteString(s)
	}

	for _, m := range messages {
		switch m.Role {
		case RoleSystem:
			if system != "" {
				system += "\n\n"
			}
			system += m.Content
		case RoleUser, RoleTool:
			write(m.Content)
		case RoleAssistant:
			write(m.Content)
			for _, tc := range m.ToolCalls {
				args, _ := json.Marshal(tc.Args)
				write(fmt.Sprintf("[You called %s with arguments %s]", tc.Name, args))
			}
		}
	}

	return system, strings.TrimSpace(body.String())
}
