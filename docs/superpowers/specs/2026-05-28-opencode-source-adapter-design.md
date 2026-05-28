# 2026-05-28 OpenCode Source Adapter Design

## Purpose

Add OpenCode CLI as a first-class tool source in aitok, enabling local token usage detection and aggregation for OpenCode sessions.

## Architecture

New source adapter `OpenCode` in `internal/sources/opencode.go`, following the existing `Source` interface pattern (Claude, Codex, Gemini, Reasonix).

### Data Source

OpenCode stores session data in `~/.local/share/opencode/opencode.db` (SQLite).

Key tables:
- `session`: aggregated token counts per session. Fields: `id`, `project_id`, `slug`, `directory`, `title`, `version`, `agent`, `model` (JSON with `providerID`, `modelID`), `cost`, `tokens_input`, `tokens_output`, `tokens_reasoning`, `tokens_cache_read`, `tokens_cache_write`, `time_created`, `time_updated`.
- `message`: individual messages. Fields: `id`, `session_id`, `time_created` (Unix ms), `data` (JSON). Assistant messages have `role: "assistant"`, `data.providerID`, `data.modelID`, `data.tokens.{input,output,reasoning}`, `data.tokens.cache.{read,write}`, `data.cost`, `data.time.created`, `data.path.{cwd,root}`, `data.finish`.

### Event Model

Each assistant `message` row produces one `UsageEvent`:

| UsageEvent field | Source |
|---|---|
| ID | `message.id` |
| Timestamp | `data.time.created` (Unix ms) |
| Tool | `"opencode"` |
| Model | `data.modelID` |
| Provider | `data.providerID` |
| CWD | `data.path.cwd` |
| Source | database path string |
| ThreadID | `session_id` |
| ThreadName | session `title` (fallback: slug, short ID) |
| ThreadSource | session `directory` |
| ThreadCreatedAt | session `time_created` |
| ThreadLastActiveAt | session `time_updated` |
| ThreadCompletedAt* (new) | `data.time.completed` |
| Usage.Input | `data.tokens.input` |
| Usage.Output | `data.tokens.output` |
| Usage.Reasoning | `data.tokens.reasoning` |
| Usage.CachedInput | `data.tokens.cache.read` |
| Usage.CacheCreation | `data.tokens.cache.write` |
| Usage.Total | `data.tokens.total` |

Skip messages with:
- `role != "assistant"`
- zero token total (`data.tokens.total == 0` or all token fields zero)
- malformed JSON `data`

### SQLite Dependency

Add `modernc.org/sqlite` (pure Go, no CGO) for reading the database with `database/sql`. Binary size impact: ~2MB.

### Registration

Added to `sources.Defaults()` in `internal/sources/collect.go` so default `summary` includes OpenCode.

### TUI

- New tab: `OpenCode` in `internal/tui/view.go`, mapped to `ToolOpenCode`.
- Resume: no stable OpenCode resume CLI yet; `resumeCommandForThread` returns nil for OpenCode threads (same as Gemini).

### Changes

| File | Change |
|---|---|
| `internal/usage/usage.go` | Add `ToolOpenCode = "opencode"` |
| `internal/sources/opencode.go` | New adapter |
| `internal/sources/collect.go` | Register `NewOpenCode` in `Defaults()` |
| `internal/tui/view.go` | Add OpenCode tab |
| `internal/tui/model.go` | Add `"opencode"` case in tab shortcut mapping; no resume command |
| `go.mod` | Add `modernc.org/sqlite` |
| `internal/sources/sources_test.go` | Add OpenCode adapter tests |

## Acceptance Criteria

1. `aitok --no-version-check summary --format json` includes OpenCode events
2. `aitok --no-version-check summary --tool opencode --format json` returns only OpenCode events
3. `aitok --no-version-check summary --threads --format json` includes OpenCode threads
4. Malformed messages are skipped, not fatal
5. Missing database returns empty events, no error
6. TUI shows OpenCode tab with correct usage, model breakdown, threads
7. No raw keys read or displayed
8. `make test` passes all packages
