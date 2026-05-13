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
9. Removed always-on CodeRabbit review because the recurring paid PR review gate was not worth the cost for this repository.
10. Added `make setup` and PR-side latest-commit commitlint so human commits and agent commits share the same commit-message contract.
11. Added an explicit PR release-decision gate so engineering/process-only changes do not trigger software releases, while feature and bugfix work must mark a follow-up release or an explicit deferral.

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

## 2026-05-13 Usage Accuracy Bugfix

Release target: `v0.1.28`.

The user reported that `summary --period yesterday` did not match the locally verified CC-Switch result for `2026-05-12 00:00 ~ 2026-05-13 00:00`. CC-Switch was treated as the reference product contract for this bugfix.

Root causes:

- Codex session logs expose `total_token_usage` as a cumulative counter. `aitok` was treating each `last_token_usage` row as an independent request and only deduplicating exact repeats, so period totals, request counts, cache totals, and costs drifted across all periods.
- Claude session logs can contain multiple rows for the same `message.id`; intermediate rows without `stop_reason` are not final API calls. `aitok` was counting those rows.
- Model names were not normalized consistently for provider prefixes or date suffixes.
- The cost formula still charged cached input at full input price for non-Codex tools and counted Codex `reasoning_output_tokens` as billable output. CC-Switch calculates cost from `(input_tokens - cache_read_tokens)`, `output_tokens`, `cache_read_tokens`, and `cache_creation_tokens`.

Fix:

- Codex now prefers `total_token_usage`, computes a per-file delta, skips zero-delta boundary rows, falls back to `last_token_usage` only when cumulative totals are absent, and scans `~/.codex/archived_sessions`.
- Claude now groups rows by `message.id`, keeps the final row with `stop_reason`, chooses the larger output row when finality is tied, and skips incomplete or zero-output rows.
- Codex and Claude model names are normalized by lowercasing, stripping provider prefixes, and stripping date suffixes.
- Offline cost estimation now follows the CC-Switch formula and does not add reasoning tokens into output cost.

Reference smoke after the fix:

- Period: `yesterday`, window `2026-05-12T00:00:00+08:00` to `2026-05-13T00:00:00+08:00`.
- Requests: `875`.
- Total tokens: `60,718,109`; input `60,398,385`; output `319,724`.
- Cached tokens: `103,962,752`.
- Cost: `$75.8474705`, displayed as `$75.8475`.
- Model totals: `claude-opus-4-7=852,249`, `gpt-5.5=43,968,237`, `gpt-5.4=15,897,623`.

Validation:

- `go test ./internal/pricing ./internal/sources`.
- `make test`.
- `make build`.
- `git diff --check`.
- `go run ./cmd/aitok summary --period yesterday --format json --no-version-check`.
- `go run ./cmd/aitok summary --period this-week --format json --no-version-check`.

## Tooling And Cache Decision

Earlier validation used `/tmp` or `/private/tmp` for Go caches only because the sandbox denied Go default cache writes under `~/Library/Caches`.

The durable project-local direction is:

- keep validation caches under `.cache/aitok`
- use Makefile targets instead of ad-hoc temp commands
- run `make setup` once per checkout to enable `.githooks/commit-msg`
- run commit message validation through `make commitlint COMMIT_MSG_FILE=<commit-msg-file>`
- let PR CI validate the latest PR commit message so contributors who did not run local setup are still covered

Temporary files like `/tmp/aitok-commit-msg` are not project tooling. They were only ephemeral command inputs.

## Process Automation Updates

CodeRabbit:

- CodeRabbit was uninstalled/disabled externally and `.coderabbit.yaml` was removed from the repository.
- Keep required review gates on GitHub-native checklist comments, CODEOWNERS, branch protection, required checks, and scoped Dependabot auto-merge.
- Do not re-add always-on paid AI review automation by default. Use paid or external AI review only as a deliberate one-off decision for high-risk changes.

Commitlint:

- `make setup` now runs `git config core.hooksPath .githooks`.
- `.githooks/commit-msg` runs `make commitlint COMMIT_MSG_FILE="$1"`.
- `.github/workflows/pr.yml` fetches the PR head SHA and validates the latest commit message with `make commitlint`, so local setup is helpful but not the only gate.

Release decision:

- `.github/pull_request_template*.md` now contains a required `Release Decision` section.
- `tools/validate-pr-body` requires that section and rejects feature/bugfix PRs that mark release as not required without an explicit deferral.
- Engineering/process-only changes such as harness, docs, CI, workflow guardrails, or repository automation should mark `Release not required`.
- Feature and bugfix changes should mark `Release required after merge` and continue into the existing release flow after merge unless the user explicitly defers release work.

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

PR metadata now also expects an explicit release decision. For feature and bugfix PRs, `Release not required` is invalid unless the body states an explicit user-approved deferral.

`make validate-pr-body` can be tested locally by setting `PR_BODY` to a realistic PR body. Running it without `PR_BODY` or `GITHUB_EVENT_PATH` intentionally fails with empty-body errors.

## Next Work

Highest value:

- Add trend/compare reports: today vs yesterday, this week vs last week, and month daily breakdown.
- Improve project/repo normalization by folding deep `cwd` paths to git roots.
- Continue deepening `doctor` with data source paths, latest event time, Gemini telemetry safety, pricing coverage, and unpriced models.

Engineering constraints:

- Keep future performance work streaming-first and benchmarked.
- Keep JSON output stable and tested when fields change.
- Do not add network upload or automatic pricing sync.
