# REASONIX.md — aitok

Auto-pinned working knowledge for this project.

## Stack

- **Go 1.26.3** — single-binary CLI, no external API runtime dependencies
- **`github.com/spf13/cobra`** — CLI framework (subcommands, flags, help)
- **`github.com/charmbracelet/bubbletea`** + **`lipgloss`** — TUI dashboard
- **Module:** `github.com/MagnumGoYB/aitok`

## Layout

| Path | Purpose |
|------|---------|
| `cmd/aitok/main.go` | Entrypoint — wires `cli.App` with `updatecheck`, calls `cmd.Execute()` |
| `internal/cli/cli.go` | All subcommands: `summary`, `report`, `pricing configure/audit`, `budget check`, `tui`, `doctor`, `version`, `update`, `setup gemini` |
| `internal/usage/usage.go` | Core types: `Tool` (claude/codex/gemini/reasonix), `TokenUsage`, `UsageEvent`, `ProviderAttribution` |
| `internal/sources/source.go` | `Source` interface (`Name()`, `Read()`, `Scan()`) + `Options` |
| `internal/sources/claude.go` | Reads `~/.claude/projects/` — walks per-project JSON logs |
| `internal/sources/codex.go` | Reads Codex session files via `~/.codex/projects/` + provider timeline cache |
| `internal/sources/gemini.go` | Reads Gemini telemetry log (`~/.gemini/telemetry.log`) + session meta |
| `internal/sources/reasonix.go` | Reads Reasonix usage logs from `~/.reasonix/usage.jsonl` + session metadata |
| `internal/sources/collect.go` | `ForEachConcurrent()` — goroutine-per-source, collects all events then replays serially |
| `internal/sources/jsonlines.go` | Shared NDJSON line-reader for Claude / Gemini log files |
| `internal/sources/session_meta.go` | Thread-ID / thread-name mapping (Claude & Gemini) |
| `internal/sources/provider_timeline_cache.go` | Codex provider-attribution timeline cache |
| `internal/query/period.go` | `Period` (today/yesterday/this-week/last-week/this-month) + `Window` + `WindowFor()` |
| `internal/query/query.go` | `Accumulator` / `ThreadAccumulator` — group-by aggregation engine with sort + filter |
| `internal/query/query.go` | `Result`, `ThreadResult`, `ThreadTurn`, `Cost`, `Price`, `rebalanceMixedProviderThread` |
| `internal/pricing/config.go` | User config file loader (`~/.aitok/pricing.json`) + `SaveUserPrice` |
| `internal/pricing/pricing.go` | `Catalog` with ~20 built-in model prices + substring matching + `CostFor(event)` |
| `internal/report/report.go` | `Write()` supporting table / json / markdown — `WriteTable`, `WriteThreadsTable`, `WriteMarkdown` |
| `internal/report/governance.go` | `WritePricingAudit()`, `WriteBudget()`, `WriteDoctor()` — governance sub-commands |
| `internal/tui/model.go` | Bubble tea model: panes (tool filter, results, threads), search, sort, copy, resume thread |
| `internal/tui/view.go` | View rendering for the TUI — scroll, layout, focused-pane highlighting |
| `internal/tui/widgets_*.go` | Card, model-usage, and thread-list widget helpers |
| `internal/tui/styles.go` | Lipgloss style definitions, color scheme |
| `internal/tui/actions.go` | Copy-to-clipboard, keyboard mapping, language toggle |
| `internal/tui/layout.go` | Pane layout / resize logic |
| `internal/tui/filters.go` | Per-tool filtering + search filtering in the TUI |
| `internal/tui/copy.go` | i18n strings (en / zh-CN) for TUI copy + resume labels |
| `internal/tui/format.go` | TUI-specific formatting helpers |
| `internal/buildinfo/buildinfo.go` | Hardcoded `Version = "0.2.0"` |
| `internal/updatecheck/updatecheck.go` | GitHub Release version check + homebrew/go auto-upgrade |
| `internal/setup/gemini.go` | Configures Gemini `telemetry.json` to enable local log output |
| `tools/commitlint/` | Commit message validator — enforces `<emoji> <type>(<scope>): <subject>` |
| `tools/pricing-watch/` | GitHub Action — monitors upstream pricing source JSON |
| `tools/validate-pr-body/` | PR description validator (CI gate) |
| `tools/version/` | Bumps version across VERSION + buildinfo.go |
| `harness/architecture_test.go` | Architecture integration tests (package layout, import rules) |

## Commands

| `make` target | Action |
|---------------|--------|
| `check` | `gofmt` + `go vet` |
| `test` | `go test ./...` |
| `build` | `go build ./cmd/aitok` |
| `run ARGS="..."` | `go run ./cmd/aitok -- $(ARGS)` |
| `validate` | `check` → `test` → `build` (full pre-PR gate) |
| `test-packages PKGS="..."` | Test specific packages |
| `test-harness` | Test `./harness` |
| `commitlint` | Validate staged commit message via `tools/commitlint` |
| `commitlint-range COMMIT_RANGE="..."` | Validate commit range |
| `validate-pr-body` | Validate PR body via `tools/validate-pr-body` |
| `setup` | Install `.githooks/` as `core.hooksPath` |

## CLI subcommands

| `aitok <sub>` | Description |
|---------------|-------------|
| `summary` | Print token usage summary (table/json/markdown) with `--period`, `--group-by`, `--sort`, `--threads`, `--full` |
| `report` | Same as summary but for pipeline use — no `--threads` default |
| `pricing configure` | Interactive/flag-driven pricing override writer to `~/.aitok/pricing.json` |
| `pricing audit` | List events with no matching price, outputs a skeleton JSON for easy filling |
| `budget check --limit-usd N` | Exit code 1 if estimated cost exceeds `--limit-usd` |
| `tui` | Interactive terminal dashboard (AltScreen), refresh every 5s |
| `tui --render` | Render TUI once to stdout without starting interactive mode |
| `doctor` | Inspects local data sources (Claude/Codex/Gemini files exist?), pricing coverage |
| `version` | Print current version |
| `update` | Check GitHub Releases and run auto-upgrade |
| `setup gemini` | Enable Gemini CLI local telemetry (writes `~/.gemini/settings.json`) |

Common flags across query subcommands: `--period`, `--format` (table/json/markdown), `--group-by`, `--sort`, `--tool`, `--model`, `--provider`, `--cwd`, `--home`, `--pricing`, `--no-version-check`.

## Data flow

1. `sources.Source.Scan()` → emits `usage.UsageEvent` (one per API call / log event)
2. **Four Source implementations** read local logs from different paths, all produce the same `UsageEvent` type
3. `query.Accumulator.Add(event)` — groups events by `groupBy` keys (tool/model/provider/day/cwd), supports `Filters`
4. `query.ThreadAccumulator` — same but groups into threads with turn-level detail, supports `rebalanceMixedProviderThread`
5. `report.Write(w, format, payload)` — formats aggregated results as table / JSON / markdown
6. Output: `Payload{GeneratedAt, Window, GroupBy, SortBy, Results, Threads}`

## Conventions

- **Commit format:** `<emoji> <type>(<scope>): <subject>` (max 64 chars). Types: `feat|fix|docs|ci|style|refactor|release|perf|test|chore|build`. Scopes: `cli|sources|query|report|setup|tui|usage|harness|docs|github|config|deps|build|tests|release`. Enforced by `tools/commitlint/`.
- **Tests colocated** — `*_test.go` lives next to the source file in the same package.
- **`io.Writer` injection** — all output goes through injected writers (not `os.Stdout`), enabling test capture.
- **`pricing.Catalog.match()`** — model name substring matching, case-insensitive, longer match patterns win.
- **`ForEachConcurrent`** — allocates per-source slices first, replays serially to the caller — guarantees ordering.
- **TUI** — bubbletea alt-screen, 5s auto-refresh, supports `/` search, `c` copy thread ID, `Enter` resume thread.
- **User-facing progress** — prefer zh-CN (`AGENTS.md`).

## Watch out for

- **`buildinfo.Version`** is auto-generated from the root `VERSION` file by `tools/buildinfo-gen` (via `go generate ./internal/buildinfo/...`). Run `make validate` to regenerate; never edit `internal/buildinfo/buildinfo.go` manually.
- **Pricing catalog** (`internal/pricing/pricing.go:DefaultCatalog()`) has ~20 hardcoded model prices. Upstream pricing changes need a code update + release — `tools/pricing-watch` monitors for this.
- **sinks/cache dir** — `.cache/` holds Go build cache. `AITOK_CACHE_DIR` env var controls location.
- **User pricing override** at `~/.aitok/pricing.json` — `Catalog.upsert()` prepends user entries before the `DefaultCatalog()` entries for priority.
- **Codex provider rebalancing** — `rebalanceMixedProviderThread()` in `query.go` has non-trivial heuristics for splitting inferred-timeline turns across providers. Modify with care.
- **TUI** won't start gracefully without a real terminal — `--render` flag outputs one-shot to stdout as an escape.
- **`tools/`** are standalone `package main` — each with its own `main_test.go`. They are NOT part of the main binary.
- **`VERSION`** at root — the single source of truth for the project version. `tools/buildinfo-gen` reads it to produce `internal/buildinfo/buildinfo.go`. Update only `VERSION` for version bumps.
- **`.goreleaser.yml`** — homebrew tap + GitHub release automation; sensitive to tag naming.
