# cxql-harness

A Go CLI that drives an LLM-powered agentic loop to remediate false-positive
findings from Checkmarx One (CxOne) SAST scans by editing the CxQL query
responsible for the finding.

Given a link to a specific finding, the harness opens an audit session
against that finding, hands the LLM a set of tools to inspect and modify the
relevant CxQL query, and iterates until the finding is suppressed — without
breaking the true-positive and true-negative cases you tell it to protect.

## How it works

1. **Session setup.** The harness resolves the finding URL (and any
   true-positive/true-negative reference findings) into a CxOne audit
   session via the `cxqlmcp` MCP server.
2. **Agentic loop.** Each iteration, the LLM is given the finding details, a
   running changelog of actions taken, and a notepad of its own working
   notes, then picks one tool call:
   - `get_query_info` — view a CxQL query's source and its hierarchy overrides
   - `search_queries` — find a query's full `Language.Group.QueryName` path from a short name
   - `run_query` — execute an existing query and see its dataflow results
   - `update_query` — test and (on successful compile) save a new
     Application-level override of a query
   - `restore_query` — revert an Application-level override back to its
     pre-session original
   - `sandbox` — compile/run throwaway CxQL without touching a real query
3. **Verification.** After each `update_query`, the harness reruns the
   scan against the project (to confirm the target finding is gone) and
   against any control projects from the config's TP/TN lists (to confirm
   nothing else regressed). The loop ends successfully once both checks
   pass, or fails after `-max-iter` iterations.

Query overrides exist at four levels: **Product** (built-in, read-only) →
**Tenant** → **Application** → **Project**. For compliance reasons, the
harness only ever creates or edits **Application**-level overrides — it will
read but never write Tenant/Product queries, and never touches
Project-level or `Common`-language queries.

### Observability

Every run produces:

- `log.txt` — full run log (LLM prompts/responses, tool calls, errors)
- `messages/{n}.json` — one structured record per LLM exchange (system
  prompt, changelog, notes, tools offered, response)
- `viewer.html` — a self-contained, offline HTML viewer (generated fresh
  after each exchange) for browsing the exchange transcript turn-by-turn

## Prerequisites

- Go (see `go.mod` for the required version)
- Two sibling repositories checked out next to this one (referenced via
  `replace` directives in `go.mod`):
  - `../Cx1ClientGo` — Go SDK for the CxOne REST API
  - `../cxqlmcp` — the MCP server backend that implements the actual CxOne
    query/session operations (this harness imports `cxqlmcp/mcp`)
- Access to a CxOne tenant, and credentials for it (see below)
- One of the LLM backends below

## Build

```bash
go build ./...
```

## Configuration

Runs are driven by a JSON config file (default `conf.json`, override with
`-config`). See `example-conf.json` for the shape:

```json
{ "finding": "URL to FP", "TPList": ["URLs to TPs"], "TNList": ["ScanIDs with TNs"] }
```

| Field | Description |
|-------|-------------|
| `finding` | Full CxOne SAST result URL for the false-positive finding to remediate |
| `TPList` | CxOne SAST result URLs for true-positive findings that must still be flagged after the fix |
| `TNList` | Scan IDs of "control" projects whose results must not regress after the fix |

`conf.json` is git-ignored since it typically contains tenant-specific URLs;
copy `example-conf.json` to `conf.json` and fill it in.

## CxOne credentials

Credentials are supplied as command-line flags (registered by `Cx1ClientGo`
and parsed alongside the harness's own flags — run `go run main.go -h` to
see the full combined list). Either:

- `-apikey <key>`, or
- `-client <id> -secret <secret>`

plus the platform location:

- `-cx1 <CxOne base URL>` `-iam <IAM URL>` `-tenant <tenant name>`

## Usage

```bash
go run main.go -backend anthropic -config conf.json
```

**CLI flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `-backend` | `ollama` | LLM backend: `ollama`, `openai`, or `anthropic` |
| `-model` | backend-specific | Override the default model name |
| `-address` | `http://localhost:11434` | LLM server address (used by `ollama`) |
| `-max-iter` | `20` | Max agentic loop iterations |
| `-prompt` | `` | Optional guidance injected into the LLM system prompt |
| `-config` | `conf.json` | Path to the JSON config describing the target finding + TP/TN list |

## LLM backends

| Backend | Status |
|---------|--------|
| `ollama` | Talks to a local Ollama server's native `/api/chat` endpoint (default model `qwen3-coder:30b`) |
| `openai` | Talks to the OpenAI `/v1/chat/completions` endpoint (default model `gpt-4o`). Requires `OPENAI_API_KEY` |
| `anthropic` | Talks to the Anthropic `/v1/messages` endpoint (default model `claude-sonnet-4-5`). Requires `ANTHROPIC_API_KEY` |

## Project layout

```
main.go                     entry point: flags, config loading, wiring
internal/harness/           the agentic loop, tool dispatch, changelog/notepad, viewer
internal/llm/               LLM backend implementations (ollama/openai/anthropic) + the LLM interface
internal/tooldef/           shared tool name constants
internal/logging/           log.txt setup
```

## Sibling repositories

- `../Cx1ClientGo` — Go SDK for the CxOne REST API. Has its own `CLAUDE.md`.
- `../cxqlmcp` — The MCP server backend used by this harness. Has its own
  `CLAUDE.md`. It's a two-module repo: a root binary module and a
  sub-library at `/mcp/`, which this harness imports as `cxqlmcp/mcp`.
