# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Purpose

`cxql_harness` is a Go CLI tool that drives an LLM-powered agentic loop to remediate false-positive findings from Checkmarx One (CxOne) SAST scans. It connects to CxOne, inspects the CxQL (Checkmarx Query Language) query responsible for a finding, and iteratively instructs an LLM to modify the query until the finding is suppressed.

## Commands

```bash
# Build
go build ./...

# Run tests (no tests exist yet)
go test ./...

# Run with a real CxOne finding URL
go run main.go -link "https://<cx1-url>/sast-results/<projectId>/<scanId>?resultId=..." -backend anthropic

# Run in stub mode (no credentials or LLM needed)
go run main.go -test
```

**CLI flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `-backend` | `ollama` | LLM backend: `ollama`, `openai`, or `anthropic` |
| `-model` | backend-specific | Override the default model name |
| `-max-iter` | `20` | Max agentic loop iterations |
| `-link` | (required) | Full URL of the CxOne SAST finding to remediate |
| `-prompt` | `` | Optional guidance injected into the LLM system prompt |
| `-test` | `false` | Use in-memory stubs instead of live services |

**CxOne credentials** are read from environment variables by `Cx1ClientGo.NewClient` (e.g. `CX_TENANT`, `CX_CLIENT_ID`, `CX_CLIENT_SECRET`, `CX_URL`).

## Sibling Repositories

This module uses `replace` directives in `go.mod` pointing to two sibling repos that must be checked out at the same directory level:

- `../Cx1ClientGo` — Go SDK for the CxOne REST API. Has its own `CLAUDE.md`.
- `../cxqlmcp` — The MCP server backend used by this harness. Has its own `CLAUDE.md`. The `cxqlmcp` module is a two-module repo: a root binary module and a sub-library at `/mcp/` — this harness imports `cxqlmcp/mcp`.

## Architecture

### Data Flow

```
main.go
  ├── Cx1ClientGo.Cx1Client  (authenticates to CxOne)
  ├── llm.LLM                (ollama / openai / anthropic)
  └── harness.Harness.Run()
        ├── mcp.CreateSessionFromURL()    → resolves finding, opens audit session
        ├── mcp.GetFindingDetails()       → finding description injected as context
        └── loop (up to maxIter):
              ├── mcp.GetCodeSnippets()   → annotated source files
              ├── llm.Chat(system + history + tools)
              └── dispatch tool call:
                    ├── get_query_info  → mcp.GetQueryInfo()
                    ├── run_query       → mcp.RunQuery()
                    └── test_query      → mcp.TestQuery()  [auto-saves on compile success]
```

### Key Interfaces

- **`harness.mcpi`** (`internal/harness/mcp.go`) — Decouples the harness from the real MCP implementation. This is the contract that both `mcp.MCP` (real) and `TestMCP` (stub) must satisfy.
- **`llm.LLM`** (`internal/llm/llm.go`) — Single method: `Chat(ctx, []Message, []ToolDef) (Response, error)`. Pluggable backends; all three real backends (Anthropic, Ollama, OpenAI) are stubs returning "not yet implemented".
- **`harness.MessageHistory`** (`internal/harness/MessageHistory.go`) — Maintains conversation turns with roles: system, user, assistant, tool-result. The system message is prepended dynamically on each call.

### CxQL Query Hierarchy

Queries exist at four levels: **Product** (built-in, read-only) → **Tenant** → **Application** → **Project**. `closestQuery()` picks the most-specific non-nil override. `TestQuery` automatically promotes to a project-level override if none exists.

### Implementation Status (as of initial commit)

The agentic loop scaffold is in place but several pieces are stubs:

- `handleGetQueryInfo`, `handleRunQuery`, `handleTestQuery` in `harness.go` — not yet wired to the `mcpi` methods.
- `TestsPassed()` always returns `false`; `getLastQueryInfo()` returns `""`.
- All three real LLM backends return errors.
- `TestMCP` and `TestLLM` are functional and used by `-test` mode.

When implementing: wire `harness.go` tool handlers to `h.mcp.*` calls, then implement at least one real LLM backend. The `cxqlmcp` backend (sibling repo) is already fully functional.
