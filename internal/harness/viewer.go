package harness

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"os"
)

//go:embed viewer.tmpl.html
var viewerTemplate []byte

const (
	viewerPath        = "viewer.html"
	viewerPlaceholder = "__CXQL_HARNESS_EXCHANGES_JSON__"
)

// writeViewer fully regenerates viewer.html from the in-memory exchange
// transcript, so it's always a complete, self-contained, offline artifact —
// never a stale mix of old and new turns.
func (h *Harness) writeViewer() {
	exs := h.exchanges
	if exs == nil {
		exs = []Exchange{}
	}
	// Keep encoding/json's default HTML-escaping (<,>,& -> \uXXXX) — that's what
	// makes it safe to inline this blob inside a <script> tag even if a tool
	// result contains a literal "</script" substring (e.g. from source code).
	data, err := json.Marshal(exs)
	if err != nil {
		h.logger.Warnf("Failed to marshal exchanges for viewer: %s", err)
		return
	}
	out := bytes.Replace(viewerTemplate, []byte(viewerPlaceholder), data, 1)
	if err := os.WriteFile(viewerPath, out, 0o644); err != nil {
		h.logger.Warnf("Failed to write %s: %s", viewerPath, err)
	}
}
