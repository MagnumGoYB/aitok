# CLAUDE

[中文](CLAUDE.zh-CN.md)

@AGENTS.md

# Claude Code entrypoint

Use `AGENTS.md` as the main project instruction source.

Additional Claude Code notes for this repo:
- Prefer zh-CN for user-facing progress updates unless the user asks otherwise.
- Before continuing pricing governance, budget, doctor, release, or GitHub automation work, read `docs/iteration-notes/2026-05-09-agent-cost-governance.md`.
- Before continuing TUI period display, local thread/session listing, or source title extraction work, read `docs/iteration-notes/2026-05-11-tui-period-threads.md`.
- Use `make check`, `go test ./...`, and `go build ./cmd/aitok` for the common local validation path.
- Use `make validate` before handoff when the change is broader than a small localized edit.
- Keep automation-friendly behavior stable: prefer `--format json` with `--no-version-check` for scripted or agent usage.
