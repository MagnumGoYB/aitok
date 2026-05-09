# 2026-05-09 Agent Cost Governance Iteration

This note is a versioned handoff for future AI coding agents. Read it before continuing cost governance, pricing coverage, budget enforcement, `doctor`, GitHub automation, or release work.

## Fast Resume

- Repo: `/Users/sosbs/coding/aitok`
- PR: `https://github.com/MagnumGoYB/aitok/pull/11`
- Release: `https://github.com/MagnumGoYB/aitok/releases/tag/v0.1.15`
- Feature branch used: `codex/aitok-local-cost-governance`
- Release commit: `918137e75c9412fe831eeef60de82a23427b870e`
- Release tag: `v0.1.15`
- Primary agent contract: `--format json --no-version-check`

## Why This Iteration Happened

The product review identified the highest-value next step as local cost governance rather than another token table. Users and AI agents need to answer:

- Am I over budget?
- Which tool, model, or working directory is unusual?
- Can automation call this safely without parsing human text?
- Can all of this work offline from local logs?

The implementation therefore prioritized performance, offline behavior, and stable machine-readable CLI output.

## Timeline

1. Brainstormed the core value areas: budget/threshold checks, trend/compare reports, project-level insights, a stronger `doctor`, and pricing coverage audit.
2. Implemented the first governance slice in PR #11: streaming scan, accumulator aggregation, pricing audit, budget check, and `doctor` improvements.
3. Investigated a pricing coverage report showing `known 20231`, `unknown 30`, and `unknown_models 1`.
4. Fixed the missing bundled price coverage for `codex-auto-review / bcb`.
5. Renamed misleading pricing report fields from `unknown_*` to `unpriced_*` / `priced_events`.
6. Merged PR #11.
7. Released `v0.1.15` with GoReleaser artifacts for darwin, linux, and windows.
8. Added this repository-level handoff note so future sessions do not depend on Codex global memory indexing.

## Product Decisions

- `aitok` remains a human CLI, but AI Agent and automation reliability are a first-class product priority.
- The stable automation path is `--format json --no-version-check`.
- JSON stdout must remain a complete machine-readable payload.
- Human-readable warnings, version prompts, and budget failure explanations belong on stderr or in the returned error path.
- `budget check` can return non-zero when a budget is exceeded, but it must still emit structured JSON on stdout.
- Offline-first remains mandatory: no usage-data upload, no remote sync, and no automatic pricing network sync.
- Cost estimates are offline estimates, not billing reconciliation.

## Shipped Scope

- Added streaming source scanning with `internal/sources.Scan` and `sources.ForEach`.
- Added `internal/query.Accumulator` for lower-memory aggregation.
- Added `aitok pricing audit`.
- Added `aitok budget check --limit-usd`.
- Enhanced `aitok doctor --format table|json|markdown` as an onboarding/audit surface.
- Added governance report code in `internal/report/governance.go`.
- Added query benchmark coverage in `internal/query/query_bench_test.go`.
- Updated README, AGENTS, and planning documentation.

## Pricing Coverage Details

The confusing report was not saying all events were unknown. It showed many priced events plus 30 unpriced events from one missing model/provider pair.

Root cause:

- Missing bundled pricing coverage for `codex-auto-review / bcb`.

Fix:

- Added default pricing coverage for that model/provider pair.
- Changed report language from `unknown_*` to `unpriced_*` / `priced_events`.

Future pricing work should preserve the distinction between:

- parseable events
- priced events
- unpriced events
- unrecognized model/provider combinations

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

CodeRabbit is intentionally not part of the required PR gate because always-on paid review is not worth the recurring cost for this repository. Keep review enforcement on GitHub-native checklist comments, CODEOWNERS, branch protection, required checks, and scoped Dependabot auto-merge. Use paid or external AI review only as an explicit one-off decision for high-risk changes.

## Next Work

Highest value:

- Add trend/compare reports: today vs yesterday, this week vs last week, and month daily breakdown.
- Improve project/repo normalization by folding deep `cwd` paths to git roots.
- Continue deepening `doctor` with data source paths, latest event time, Gemini telemetry safety, pricing coverage, and unpriced models.

Engineering constraints:

- Keep future performance work streaming-first and benchmarked.
- Keep JSON output stable and tested when fields change.
- Do not add network upload or automatic pricing sync.
