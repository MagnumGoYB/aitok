# AGENTS

[中文](AGENTS.zh-CN.md)

This file is the execution guide for AI coding agents working in this repository. User-facing progress and handoff updates should default to zh-CN unless the user asks otherwise. Code, command names, test names, upstream docs, and quoted source text stay as-is.

## 1) Project Mission

`aitok` is an offline-first Go CLI for summarizing local token usage from Claude Code, Codex, and Gemini CLI.
It may estimate USD costs from local token counts using an offline price catalog and local user overrides.
Although it is a command-line tool for humans, product and engineering decisions should prioritize reliable invocation by AI agents and automation.

Current codebase shape:

- Go 1.26.3+
- Single-binary CLI: `cmd/aitok`
- Source adapters: `internal/sources`
- Query aggregation: `internal/query`
- Price catalog and cost estimation: `internal/pricing`
- Report output: `internal/report`
- Gemini local telemetry setup: `internal/setup`
- TUI: `internal/tui`
- Harness sensors: `harness`

Iteration notes:

- Versioned iteration handoff notes live in `docs/iteration-notes/` with zh-CN mirrors in `docs/zh-CN/iteration-notes/`.
- Before continuing agent-cost-governance, pricing coverage, budget, doctor, release, or GitHub automation work, read `docs/iteration-notes/2026-05-09-agent-cost-governance.md`.

Out of scope unless explicitly requested:

- network upload, remote sync, or cloud reporting
- reading, storing, printing, hashing, or fingerprinting raw API keys
- billing reconciliation or claims of exact provider billing
- turning the TUI or future Web dashboard into a persistent background service

## 2) Dev Commands

- Format and static checks: `make check`
- Enable local git hooks: `make setup`
- Full tests: `make test`
- Harness-only tests: `make test-harness`
- Vet: `make vet`
- Build: `make build`
- Full local validation: `make validate`
- PR metadata check: `make validate-pr-body`
- Commit message check: `make commitlint COMMIT_MSG_FILE=<commit-msg-file>`
- Run CLI directly: `go run ./cmd/aitok summary --period today`

## 3) Iteration Self-Constraint Protocol

Before every product or harness iteration, do a short internal review:

1. Requirement classification
- Classify the request as feature, bugfix, refactor, harness/tooling, or analysis-only.
- State the user-visible outcome, target platform, and affected area.
- State the release decision: engineering/process-only optimization does not require a software release; feature and bugfix work must prompt for or continue into the release flow unless the user explicitly defers it.
- If the request is ambiguous, write the safest concrete assumption before editing. Ask only when a wrong assumption would create meaningful data or product risk.

2. Stack fit
- Prefer the Go standard library and existing package boundaries.
- New dependencies need a concrete reason and must explain impact on binary size, offline behavior, and supply-chain risk.
- Local log parsing must stay streaming; do not load large JSONL logs fully into memory.

3. Acceptance and regression plan
- Write observable acceptance criteria before implementation.
- Cover at least one failure or edge case: empty directories, malformed JSONL, missing fields, duplicate events, Gemini telemetry not configured, or unknown provider.
- Map each criterion to unit tests, harness tests, CLI smoke, manual evidence, or explicit not-applicable rationale.

4. Scope guard
- Name the files or directories expected to change.
- Avoid unrelated refactors, formatting churn, dependency upgrades, or release workflow changes.
- Keep harness, CI, docs, and production code responsibilities separated.

5. Handoff readiness
- Run the smallest targeted check while iterating.
- Run the validation matrix in section 6 before handoff based on changed area.
- Summarize residual risks and any validation that could not be run.

## 4) Coding Rules

- Every source adapter emits `usage.UsageEvent`.
- Do not read, store, or display raw API keys. Provider grouping may only use provider/auth_type metadata already present in CLI logs, or `unknown`.
- Do not add usage-data network transmission. The only command-start network behavior is the low-frequency GitHub release metadata version check, which must never read local logs or upload usage data and must remain skippable with `--no-version-check` or `AITOK_NO_VERSION_CHECK=1`.
- Cost estimation must remain offline by default. Default prices may be updated from public provider pricing, but automatic network sync must be explicitly requested and opt-in.
- Gemini CLI historical data depends on an existing local telemetry outfile. When it is not configured, report no parseable historical data.
- CLI output must stay stable; JSON field changes need tests.
- AI agents should treat `--format json` plus `--no-version-check` as the primary automation contract.
- For JSON commands, stdout must remain a complete machine-readable JSON payload. Human-readable warnings, budget failures, and version prompts belong on stderr or in the returned error path.
- Exit codes are part of the contract: `budget check` returns non-zero when the budget is exceeded while still writing the structured payload to stdout.
- Markdown/table reports should remain readable and scriptable.
- TUI must not replace CLI/JSON output; automation must be able to bypass TUI.
- Production packages must not import `harness/` or `tools/`.

## 5) Harness Maintenance Rules

- When a repeated issue comes from missing context, update `AGENTS.md` / `AGENTS.zh-CN.md` or docs.
- When an issue can be detected deterministically, add or refine a `harness` test.
- When changing harness, CI, PR workflow, or validation scripts, update:
  - `docs/harness-engineering.md`
  - `docs/zh-CN/harness-engineering.md`
  - `.github/pull_request_template.md` when PR rules change
- Harness tests should check repository structure, workflow constraints, and docs/script alignment. They should not test product business logic.

## 6) Regression Verification Matrix

- Documentation or harness only: `make check`, `make test-harness`, and `make validate-pr-body` when PR rules changed.
- Source adapter: `make test`, including normal, empty directory, malformed JSONL, missing field, and duplicate event cases.
- Query/report: `make test`, covering period windows, filters, grouping, and stable JSON/Markdown output.
- CLI/TUI: `make test`, `make build`, and when useful `go run ./cmd/aitok doctor` or `summary` smoke.
- Dependencies, CI, or release config: `make validate`, with binary-size/offline/supply-chain impact noted.

## 7) CI and Submission Protocol

- CI must run `make validate` and `make test-harness`.
- PRs must include Summary, Requirement Classification, Acceptance Criteria, Changed Areas, Release Decision, TDD / Test Evidence, Validation, and Risk and Rollback.
- Feature and bugfix PRs must mark release required after merge or identify the explicit user-approved deferral. Harness/tooling, docs, CI, and other engineering-process-only PRs should mark release not required.
- Commit messages must match the repository Go commitlint format: `{emoji} {type}{scope}: {subject}`. Run `make setup` once to enable `.githooks/commit-msg`, or run `make commitlint COMMIT_MSG_FILE=<commit-msg-file>` directly. PR CI also validates the latest PR commit message.
- Do not claim completion after skipping local validation.
- Stage/commit only files that belong to the current iteration; do not include unrelated dirty files.

## 8) Open Source Documentation and GitHub Automation

- Every public guide or policy document needs a zh-CN counterpart, for example `README.md` + `README.zh-CN.md` and `CONTRIBUTING.md` + `CONTRIBUTING.zh-CN.md`.
- Keep GitHub PR, review, bugfix, build, and release workflow changes documented in `docs/github-automation.md` and `docs/zh-CN/github-automation.md`.
- When changing release automation, run `make validate` and verify `.goreleaser.yml` remains aligned with `.github/workflows/release.yml`.
