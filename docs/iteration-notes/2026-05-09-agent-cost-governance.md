# 2026-05-09 Agent Cost Governance Iteration

This note preserves project context that future AI coding agents should read when continuing work after PR #11 and release `v0.1.15`.

## Outcome

- PR: `https://github.com/MagnumGoYB/aitok/pull/11`
- Release: `https://github.com/MagnumGoYB/aitok/releases/tag/v0.1.15`
- Feature branch used: `codex/aitok-local-cost-governance`
- Release commit: `918137e75c9412fe831eeef60de82a23427b870e`
- Release tag: `v0.1.15`

## Product Decisions

- `aitok` remains a human CLI, but AI Agent and automation reliability are a first-class product priority.
- The primary agent contract is `--format json --no-version-check`.
- JSON stdout must remain a complete machine-readable payload.
- Human-readable warnings, version prompts, and budget failure explanations belong on stderr or in the returned error path.
- `budget check` can return non-zero when a budget is exceeded, but it must still emit structured JSON on stdout.
- Offline-first remains mandatory: no usage-data upload, no remote sync, and no automatic pricing network sync.

## Shipped Scope

- Added streaming source scanning with `internal/sources.Scan` and `sources.ForEach`.
- Added `internal/query.Accumulator` for lower-memory aggregation.
- Added `aitok pricing audit`.
- Added `aitok budget check --limit-usd`.
- Enhanced `aitok doctor --format table|json|markdown` as an onboarding/audit surface.
- Added governance report code in `internal/report/governance.go`.
- Added query benchmark coverage in `internal/query/query_bench_test.go`.
- Updated README, AGENTS, and planning documentation.

## Pricing Coverage Fix

The observed report was:

- `known 20231`
- `unknown 30`
- `unknown_models 1`

The root cause was missing bundled pricing coverage for `codex-auto-review / bcb`.

The fix added default pricing coverage for that model/provider pair and changed misleading language from `unknown_*` to clearer `unpriced_*` / `priced_events`. Future pricing work should preserve the distinction between parseable events and unpriced events.

## Tooling And Cache Decision

Earlier validation used `/tmp` or `/private/tmp` for Go caches only because the sandbox denied Go default cache writes under `~/Library/Caches`.

The durable project-local direction is:

- keep validation caches under `.cache/aitok`
- use Makefile targets instead of ad-hoc temp commands
- run commit message validation through `make commitlint COMMIT_MSG_FILE=<commit-msg-file>`

Temporary files like `/tmp/aitok-commit-msg` are not project tooling. They were only ephemeral command inputs.

## Validation Evidence

Feature PR checks run locally:

- `go test ./internal/cli`
- `go test ./internal/pricing ./internal/report ./internal/cli`
- `go test ./internal/query -bench . -benchmem`
- `make check`
- `make test`
- `make build`

Release validation:

- `make validate` passed locally before tagging.
- GitHub Actions release run succeeded: `https://github.com/MagnumGoYB/aitok/actions/runs/25598864490`
- GoReleaser uploaded darwin, linux, windows artifacts and `checksums.txt`.

## GitHub Automation Notes

`gh pr edit` may fail in this repository because GitHub Projects classic GraphQL fields are deprecated. If PR body edits hit that issue, use the REST API:

```sh
jq -Rs '{body:.}' <body.md> > <body.json>
gh api repos/MagnumGoYB/aitok/pulls/<number> -X PATCH --input <body.json>
```

PR metadata validation expects explicit coverage for requirement classification, acceptance criteria evidence, unit tests, failure/edge coverage, skipped validation, and residual risk.

## Suggested Next Work

- Add trend/compare reports: today vs yesterday, this week vs last week, and month daily breakdown.
- Improve project/repo normalization by folding deep `cwd` paths to git roots.
- Continue deepening `doctor` with data source paths, latest event time, Gemini telemetry safety, pricing coverage, and unpriced models.
- Keep future performance work streaming-first and benchmarked.
