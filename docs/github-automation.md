# GitHub Automation

[中文](zh-CN/github-automation.md)

This repository uses GitHub-native automation for pull requests, review prompts, bug reports, cross-platform builds, and releases.

## Pull Request Flow

- `.github/pull_request_template.md` requires requirement classification, acceptance criteria, test evidence, validation, rollback, and residual risk.
- `.github/workflows/pr.yml` validates the real pull request body with `make validate-pr-body`.
- `.github/workflows/ci.yml` runs local validation and harness gates on pushes and pull requests.

## Review Flow

- `.github/workflows/pr-review.yml` posts a checklist comment on new or updated pull requests.
- The checklist reminds reviewers to inspect offline/privacy boundaries, source adapter streaming behavior, fixture coverage, CLI output stability, and release impact.
- `.github/CODEOWNERS` requests review for core areas such as adapters, query/report code, harness, and GitHub workflows.

## Bugfix Flow

- `.github/ISSUE_TEMPLATE/bug_report.yml` captures tool, period, command, expected behavior, actual behavior, sanitized logs, and validation evidence.
- Bugfix PRs must include a failing test, fixture, harness sensor, or explicit manual reproduction before the fix when practical.
- The PR template requires failure or edge coverage and rollback notes.

## Build Flow

- `.github/workflows/build.yml` builds Linux, macOS, and Windows artifacts only when code is merged to `main` or a `v*` tag is pushed.
- The build workflow reads the repository `VERSION` file through `tools/version` before producing artifacts.
- Build artifacts are uploaded for inspection without publishing a release.
- `make build` remains the local single-platform build gate.

## Release Flow

- `.github/workflows/release.yml` runs only when code is merged to `main` or a `v*` tag is pushed.
- The release job reads `VERSION` through `tools/version`; tag releases must match `VERSION` as `v<version>`.
- On `main`, the release workflow validates the project but does not publish a GitHub Release.
- On matching `v*` tags, the release job runs `make validate`, then uses GoReleaser with `.goreleaser.yml` to publish checksums and platform archives.
- Releases require `GITHUB_TOKEN` only; no external network telemetry or usage upload is added by release automation.

## Dependabot

- `.github/dependabot.yml` checks GitHub Actions and Go module updates weekly.
- Dependency updates must still pass `make validate` and explain binary-size/offline/supply-chain impact when relevant.
