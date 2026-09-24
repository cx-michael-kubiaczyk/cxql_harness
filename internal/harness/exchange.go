package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cxpsemea/cxql-harness/internal/llm"
)

const (
	CallSiteChooseAction = "choose_action"
	CallSiteAfterTool    = "after_tool"
	CallSiteDebugQuery   = "debug_query"
)

// Exchange is one structurally-sectioned LLM call/response, dumped to
// messages/{Seq}.json and rendered as a "turn" in viewer.html.
type Exchange struct {
	Seq              int
	CallSite         string
	Timestamp        time.Time
	System           string
	Changelog        string
	ChangelogEntries []ToolCallEntry
	Notes            string
	NoteEntries      []Note
	Prompt           string
	Messages         []llm.Message
	Tools            []llm.ToolDef
	Response         llm.Response
	Error            string
}

// parseJSONAfterPrefixLine strips the fixed lead-in sentence (up to and
// including the first newline) that getChangelog()/getNotes() prepend, and
// best-effort unmarshals the remainder as []T. Returns nil on empty input or
// a parse failure — callers keep the raw string as a verbatim fallback the
// viewer can render when this is nil. Do NOT swap this for pulling structured
// data straight from h.changelog/h.notepad: handleQueryError's hist can have
// blank changelog/notes content even while the filter says to include them,
// and the log must reflect exactly what was sent, not the harness's real
// accumulated state.
func parseJSONAfterPrefixLine[T any](raw string) []T {
	if raw == "" {
		return nil
	}
	idx := strings.IndexByte(raw, '\n')
	if idx < 0 {
		return nil
	}
	var out []T
	if err := json.Unmarshal([]byte(raw[idx+1:]), &out); err != nil {
		return nil
	}
	return out
}

// recordExchange appends ex to the in-memory transcript, writes it as its own
// messages/{Seq}.json file, and regenerates viewer.html from the full transcript.
func (h *Harness) recordExchange(ex Exchange) {
	h.exchanges = append(h.exchanges, ex)

	data, err := json.MarshalIndent(ex, "", "  ")
	if err != nil {
		h.logger.Warnf("Failed to marshal exchange %d: %s", ex.Seq, err)
	} else {
		path := filepath.Join(messagesDir, fmt.Sprintf("%d.json", ex.Seq))
		if err := os.WriteFile(path, data, 0o644); err != nil {
			h.logger.Warnf("Failed to write %s: %s", path, err)
		}
	}

	h.writeViewer()
}
