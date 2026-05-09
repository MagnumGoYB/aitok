# Harness Engineering

[中文](zh-CN/harness-engineering.md)

A harness combines feedforward guides that steer an agent before it edits with feedback sensors that catch drift after it edits.

For `aitok`, the harness is intentionally lightweight: Go tests, a Makefile, PR metadata validation, CI gates, and concise agent guides. It protects the offline token-accounting contract without adding background services.

## Feedforward Guides

- `AGENTS.md` and `AGENTS.zh-CN.md`: repository mission, coding constraints, validation matrix, privacy boundaries, and handoff rules.
- `README.md`: user-facing CLI usage and install path.
- `CONTRIBUTING.md`: contributor validation and offline-first rules.
- `Makefile`: canonical local commands: `make check`, `make test`, `make test-harness`, `make vet`, `make build`, `make validate`, `make validate-pr-body`, and `make commitlint`.
- `tools/commitlint` and `.githooks/commit-msg`: repository-native Go commit-message validation for `{emoji} {type}{scope}: {subject}` without Node/npm tooling.
- `.github/pull_request_template.md`: repeatable PR checklist for requirement classification, acceptance criteria, test evidence, validation, rollback, and residual risk.
- `.github/workflows/ci.yml`: hosted validation matching the local gates.

## Feedback Sensors

- `go test ./...`: unit and integration coverage for source adapters, period windows, aggregation, reports, CLI, setup, and TUI smoke rendering.
- `go test ./harness`: repository-structure sensors for agent docs, Makefile commands, CI gates, PR template, and offline/privacy constraints.
- `go vet ./...`: static analysis.
- `go build ./cmd/aitok`: single-binary build check.
- `go run ./tools/validate-pr-body`: executable PR body metadata gate.
- `make commitlint COMMIT_MSG_FILE=<commit-msg-file>`: executable commit-message gate, optionally wired through `.githooks/commit-msg`.
- `.cache/aitok/`: repository-local, git-ignored Go build/module cache used by Makefile targets so agent verification stays bound to this checkout instead of ad hoc `/tmp` paths.

## Agent Workflow Contract

- Classify each request before editing: feature, bugfix, refactor, harness/tooling, or analysis-only.
- Lock observable acceptance criteria before implementation.
- Add a failing test, harness sensor, or explicit manual verification checklist before changing behavior when practical.
- Keep edits scoped to the named files/directories.
- Map every acceptance criterion to evidence before handoff.
- Report skipped validation and residual risk.

## aitok-Specific Guardrails

- Do not add network upload, sync, telemetry, or remote reporting by default.
- Do not read, store, print, hash, or fingerprint raw API keys.
- Keep source adapters streaming and fixture-backed.
- Preserve stable CLI output for automation.
- Keep TUI optional; JSON and Markdown reports remain first-class.
- Gemini CLI support depends on local telemetry being configured. `setup gemini` must keep `logPrompts=false`.

## Updating Harness

When changing harness, CI, PR workflow, or validation scripts:

1. Update the executable script/test/workflow.
2. Update `AGENTS.md` and `AGENTS.zh-CN.md` if agent behavior changes.
3. Update this document and `docs/zh-CN/harness-engineering.md`.
4. Run `make check`, `make test-harness`, and `make validate-pr-body` when PR metadata rules changed.

When changing commit workflow rules, update `tools/commitlint`, `.githooks/commit-msg`, `AGENTS.md`, `AGENTS.zh-CN.md`, this document, and `docs/zh-CN/harness-engineering.md`.

- `CODE_OF_CONDUCT.md` / `CODE_OF_CONDUCT.zh-CN.md` and `SUPPORT.md` / `SUPPORT.zh-CN.md` keep open-source community guidance bilingual.

## GitHub Automation Coverage

- `docs/github-automation.md` and `docs/zh-CN/github-automation.md` document PR review prompts, bugfix, build, release, and Dependabot auto-merge flows.
- `.github/workflows/pr.yml` validates real PR metadata.
- `.github/workflows/pr-review.yml` posts the review checklist with `issues: write` and `pull-requests: write` permissions so it can create or update the PR issue comment.
- `.github/CODEOWNERS` routes high-risk areas to maintainers without requiring paid AI review automation.
- `.github/workflows/dependabot-auto-merge.yml` enables GitHub auto-merge only for non-major Dependabot updates.
- `.github/workflows/build.yml` uploads cross-platform build artifacts.
- `.github/workflows/release.yml` publishes tag releases through GoReleaser.
- `.github/ISSUE_TEMPLATE/bug_report.yml`, `.github/CODEOWNERS`, and `.github/dependabot.yml` keep open-source maintenance paths explicit.
