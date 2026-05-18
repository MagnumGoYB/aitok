# 2026-05-18 TUI Dashboard Modernization

This note is a versioned handoff for future AI coding agents. Read it before continuing dashboard layout, Threads/Selected Thread alignment, Model Usage bars, TUI render hardening, or release follow-up from `v0.1.38` onward.

## Fast Resume

- Repo: `/Users/sosbs/coding/aitok`
- Branch used: `feat/tui-modernize`
- Feature commit: `99e2332`
- Release tags: `v0.1.38`, `v0.1.39`, `v0.1.40`, `v0.1.41`
- Release status: `v0.1.41` is the follow-up release for dashboard render hardening, viewport page scrolling, and replacing modal notifications with the top-right header notice pattern.
- Related earlier TUI context: `docs/iteration-notes/2026-05-11-tui-period-threads.md`

## Why This Iteration Happened

The dashboard had become functionally useful but visually unstable under real terminal widths. The user repeatedly found layout problems from screenshots: overflowing right borders, duplicated metric labels, too many Model Usage bars, weak color hierarchy, awkward help placement, and wrapping Threads rows.

The durable lesson is that screenshot-driven TUI polish must become codified render constraints, not just manual terminal inspection.

## Shipped Decisions

- Keep top breathing room before `Usage Dashboard`; the title must not touch the top of the render.
- Remove redundant subtitle clutter and keep compact `? help` near the remaining status text.
- Keep toolbar metadata compact: tabs, sort, model count, thread count, search, and date range wrap into a compact header instead of stretching past the viewport.
- Show `Models: N` and `Threads: N`, not `N/N`.
- Do not repeat metric badges in section titles when the toolbar already shows the active sort metric.
- Threads and Selected Thread are a locked horizontal pair. Their rendered widths plus gap must not exceed the dashboard width.
- Threads list rows must not wrap. Keep only high-signal list columns; complete model/provider/cost detail belongs in Selected Thread.
- Selected Thread shows `Model`, `Provider`, `Last Active`, `Tokens`, and one informative cost row such as `Cost: $306.0250 (toska $299.77 / openai $6.25)`. Do not restore a separate `Split: toska/openai` row.
- Model Usage bars use the Threads highlight hue family. Higher usage is darker and lower usage is lighter.
- Model Usage shows at most 4 chart bars. Extra rows are folded with a hint; the table remains the full inspection surface.
- Do not re-add a pie chart unless a reliable terminal chart implementation exists and clearly improves over the bar/table pairing.
- When the dashboard content exceeds one terminal screen, page scrolling must move the whole dashboard viewport so users can return to content above the fold.
- Do not use modal/dialog overlays for transient TUI notices such as copied thread IDs or help. They proved visually brittle in terminal rendering. Put these notices in the top-right header area instead, and do not add a bottom notification strip.

## Implementation Notes

- The former monolithic `internal/tui/tui.go` was split into `actions`, `copy`, `filters`, `format`, `layout`, `model`, `view`, and widget files. Future TUI edits should stay in the narrow file that owns the behavior.
- Time-sensitive tests must compute expected display values with the same location conversion as production code. Do not hard-code timestamps rendered in a developer-local timezone.
- `v0.1.39` was pushed and its Release workflow failed because a TUI test expected an Asia/Shanghai timestamp while the GitHub runner rendered UTC. Do not move already-pushed release tags; fix forward and bump the next patch version.
- Screenshot issues should become text render assertions where practical: right-border alignment, no wrapping, visible row caps, folded hints, sort labels, and help placement are all testable from ANSI-stripped output.
- The `? help` affordance and copy status are header notices. Tests should assert they stay in the top header area and do not hide the dashboard body.

## Validation Evidence

- `v0.1.40` local validation passed: `make validate`, `make test-packages PKGS="./internal/tui"`, and `GITHUB_REF_NAME=v0.1.40 GITHUB_REF_TYPE=tag go run ./tools/version --check-ref`.
- `v0.1.40` GitHub Actions passed: tag `Release` run `26027319067` and tag `Build` run `26027319217`.
- `v0.1.40` GitHub Release published successfully: `https://github.com/MagnumGoYB/aitok/releases/tag/v0.1.40`.
- `v0.1.41` local validation before tag creation passed: `make validate`, `make test-packages PKGS="./internal/tui ./internal/sources ./internal/query ./internal/report"`, `git diff --check`, and `make run ARGS="--no-version-check tui --period today --render"`.

## Next Work

- Add a lightweight TUI render snapshot/golden harness for fixed terminal widths so screenshot-driven layout bugs become durable regression coverage.
- Keep future dashboard polish in focused widget/layout files; avoid returning to a single large TUI file.
