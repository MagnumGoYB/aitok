# 2026-05-11 TUI Period And Threads Iteration

This note is a versioned handoff for future AI coding agents. Read it before continuing TUI period display, local thread/session listing, source title extraction, or the release follow-up for PR #13.

## Fast Resume

- Repo: `/Users/sosbs/coding/aitok`
- PR: `https://github.com/MagnumGoYB/aitok/pull/13`
- Feature branch used: `codex/tui-period-threads-list`
- Feature commit: `107b7da1f2d3a4d4207c9c4581778aca1153a45e`
- Merge commit: `97e4cbbd8bcd08e5a2667b415eb35122de434acc`
- Release status: PR #13 and follow-up TUI fixes were released through `v0.1.25`; the current post-release feature/bugfix follow-up is targeting `v0.1.26`
- Primary agent contract: `aitok --no-version-check summary --period today --threads --format json`

## Why This Iteration Happened

The TUI period label was not matching the user's expectation, and the product needed a direct way to inspect the local sessions behind a summary result. The user also required thread selection to behave like a scrollable process list rather than a paged or collapsible panel.

The work therefore shipped two related changes:

- Make the TUI period label factual and less noisy.
- Add a machine-readable and TUI-visible threads list for matching local usage events.

## Product Decisions

- Do not change period semantics in this iteration. The user's initial `this-week` range was treated as a display example, not a request to change query windows.
- `today` in the TUI shows only the local date and timezone. Non-`today` periods show the actual `Window.Start` to `Window.End` range and timezone.
- The date label should not use an emoji.
- `summary --threads` is opt-in so default JSON payloads do not grow or break existing automation.
- TUI threads use a fixed box with a header, highlighted current row, and a scrollbar. There is no pagination and no expand/collapse state.
- Copying a selected thread copies only the thread/session ID with OSC52. It does not copy a resume command.

## Thread Title Rules

The durable priority order is:

1. custom title
2. Codex UI title from `.codex/session_index.jsonl` `thread_name`, or explicit AI title fields/events
3. first real user message
4. cwd basename
5. short ID

Important Codex-specific findings:

- Current Codex Desktop names are indexed in `/Users/sosbs/.codex/session_index.jsonl`.
- Session JSONL files often do not contain the same UI title.
- Do not treat arbitrary assistant messages as AI summary titles.
- Filter injected/non-title messages such as `# Context from my IDE setup:`, `<turn_aborted>`, and placeholder summary values like `none`.

## Shipped Scope

- Added thread metadata fields to `usage.UsageEvent`.
- Added `query.ThreadResult` aggregation with usage, requests, events, cost, source, created time, and last active time.
- Added Codex, Claude, and Gemini local session metadata extraction.
- Added `summary --threads` with JSON/table/markdown reporting support.
- Updated TUI period rendering.
- Added a TUI Threads box with focus toggle, row movement, home/end, scrollbar, and OSC52 copy feedback.
- Updated README and README.zh-CN with `summary --threads` and TUI thread shortcuts.
- Added the implementation plan at `docs/superpowers/plans/2026-05-11-tui-period-threads-list.md`.

## Post-Merge Follow-Ups

After PR #13 merged, several TUI polish releases landed on `main`:

- `v0.1.16`: released the initial TUI period and threads feature after merge.
- `v0.1.17`: refined the Threads layout by placing Threads before Model Usage, adding a Model Usage border, widening the ID/name spacing, limiting thread-name display width, removing the trailing Threads column line, and aligning the selected-row/tab color with `#00B2FF`.
- `v0.1.18`: fixed the Threads alignment policy so `Name`, `Tool`, `Model`, `Provider`, and `Req` are left-aligned while `Events`, `Cost`, and `Tokens` are right-aligned.
- `v0.1.19`: fixed additional TUI layout issues: period range uses ASCII `~`, section gaps are smaller, Threads renders a real scrollbar when overflowing, cursor movement updates the scrollbar offset, and regression coverage confirms TUI Threads respects the selected period window.
- `v0.1.20`: compacted the dashboard so it fits better in a terminal viewport: toolbar is 3 lines, summary cards are 4 lines, Threads is capped at 6 visible rows, and Model Usage caps provider-heavy output to the top rows instead of filling the screen.
- `v0.1.21`: aligned cost columns more consistently across the TUI and standardized Claude-facing docs wording.
- `v0.1.22`: fixed Threads filtering so active tool/search state, cursor movement, copy actions, and scrollbar math operate on the filtered thread list, and aligned `Cost` by its rendered end edge in Model Usage and Threads.
- `v0.1.23`: made Threads default to descending token usage order in both `summary --threads` payloads and the TUI filtered view.
- `v0.1.24`: fixed Model Usage chart ratios for sub-1m token rows and added total `Tokens` to the Model Usage table.
- `v0.1.25`: kept one row per thread ID, replaced thread `model=mixed` with a compact comma-joined model list, merged redundant request/event-heavy default output, and shaded Model Usage bars within one hue family so relative usage is still visually distinct.
- Pending `v0.1.26` feature/bugfix scope: add `--sort tokens|cost` for query outputs, let TUI switch sort metrics with `s`, show metric badges in Threads and Model Usage, localize missed table/search labels, and scale Model Usage chart bars/labels by USD when Cost sorting is active.

Current TUI layout constraints to preserve:

- Threads filtering must stay in sync with the active tool tabs and search term. Any cursor movement, Home/End jump, copy action, and scrollbar math should operate on the filtered thread slice instead of the unfiltered payload.
- Query output supports two descending sort metrics: `tokens` by default and `cost` when `--sort cost` is passed. TUI can switch the active metric with `s` and must show the current metric badge in Threads and Model Usage.
- The old `t threads` focus shortcut has been removed from user-facing help; `j/k/home/end/c` operate directly on the filtered Threads list.
- `Cost` should be treated like the other numeric columns: right-aligned by its rendered end edge in both Model Usage and Threads, even when values include `$`.

- Toolbar should stay compact with no vertical padding.
- Summary cards should stay compact and avoid decorative vertical whitespace.
- Threads should show at most 6 rows and rely on the scrollbar plus `j/k/home/end` for navigation.
- Model Usage should handle many provider/model groups by limiting chart rows and table rows; do not let provider-heavy data push the footer off-screen.
- Model Usage chart bars should not collapse all small non-zero rows to the same visual width. Preserve sub-cell ratios with partial block rendering while keeping non-zero rows visible.
- Model Usage table should show total `Tokens` alongside `Input`, `Output`, and `Cached` so users can compare the chart and table without doing mental addition.
- The date range separator is ASCII `~`, not full-width `～`.
- `this-week` still means the current natural week window from `query.WindowFor`; it was not changed to a rolling 7-day period.

## Validation Evidence

Local validation before PR:

- `go test ./internal/sources ./internal/query ./internal/report ./internal/tui ./internal/cli`
- `go run ./cmd/aitok summary --period today --tool codex --threads --format json --no-version-check`
- `make check`
- `make test`
- `make test-harness`
- `make build`
- `git diff --check`

Follow-up validation used during the v0.1.17-v0.1.20 polish releases:

- `go test ./internal/tui`
- `go test ./internal/tui ./internal/cli`
- `go test ./internal/query ./internal/report ./internal/sources ./internal/tui ./internal/cli`
- `make check`
- `make test`
- `make build`
- `make validate` before release bumps
- `GITHUB_REF_NAME=vX.Y.Z GITHUB_REF_TYPE=tag go run ./tools/version --check-ref`
- GitHub Release workflows for `v0.1.16`, `v0.1.17`, `v0.1.18`, `v0.1.19`, `v0.1.20`, `v0.1.21`, `v0.1.22`, and `v0.1.23` completed successfully.
- Current `v0.1.26` validation target: `go test ./internal/query ./internal/report ./internal/cli ./internal/tui`, `make test`, `make build`, `make validate`, `GITHUB_REF_NAME=v0.1.26 GITHUB_REF_TYPE=tag go run ./tools/version --check-ref`, and `git diff --check` before tagging.

GitHub PR checks:

- `test`: passed
- `metadata`: passed
- `checklist`: passed after PR was marked ready
- `build` for linux amd64/arm64, darwin amd64/arm64, and windows amd64: passed

## Release Follow-Up

No pending release follow-up remains for the original PR #13 scope as of `v0.1.25`. The current query-sort and Cost-chart follow-up should ship as `v0.1.26` unless the user explicitly defers release work.

## Future Work

- Add an optional shortcut to copy a resume command, such as `codex resume <id>` or the equivalent Claude command.
- Improve title extraction as Codex, Claude, or Gemini log schemas evolve.
- Consider root-normalized project grouping so deep `cwd` values produce cleaner thread and project views.
