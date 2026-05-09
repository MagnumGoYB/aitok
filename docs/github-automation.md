# GitHub Automation

[中文](zh-CN/github-automation.md)

This repository uses GitHub-native automation for pull requests, CodeRabbit reviews, review prompts, Dependabot auto-merge, bug reports, pricing-watch alerts, cross-platform builds, and releases.

## Pull Request Flow

- `.github/pull_request_template.md` requires requirement classification, acceptance criteria, test evidence, validation, rollback, and residual risk.
- `.github/workflows/pr.yml` validates the real pull request body with `make validate-pr-body`.
- `.github/workflows/ci.yml` runs local validation and harness gates on pushes and pull requests.

## Review Flow

- `.coderabbit.yaml` configures CodeRabbit automatic reviews for PRs targeting `main`.
- CodeRabbit reviews use zh-CN comments, an assertive profile, request-changes workflow, and path-specific instructions for Go code, GitHub workflows, docs, and harness files.
- CodeRabbit must still be installed as the GitHub App for repository PRs; the YAML file only defines repository-specific behavior.
- `.github/workflows/pr-review.yml` posts a checklist comment on new or updated pull requests.
- The checklist workflow runs with `issues: write` and `pull-requests: write` so `actions/github-script` can create or update the PR issue comment under branch protection.
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
- GitHub Actions workflows use Node 24 action majors such as `actions/checkout@v6`, `actions/setup-go@v6`, `actions/upload-artifact@v6`, and `actions/github-script@v8`.

## Pricing Watch Flow

- `.github/workflows/pricing-watch.yml` runs daily and on `workflow_dispatch`.
- The workflow runs `go run ./tools/pricing-watch`, which fetches machine-readable official pricing pages listed in `docs/pricing-sources.json`, verifies required pricing text such as model names and cache sections, and compares normalized SHA256 values only for sources stable enough to hash.
- Official pages that block automated fetches, such as the OpenAI pricing page, are marked `manual_review` in `docs/pricing-sources.json`; maintainers review them during pricing updates instead of letting CI fail on anti-bot responses.
- When an official pricing source changes, the workflow opens or updates one issue labeled `pricing-watch` instead of changing code automatically.
- Maintainers must review the official pricing page, update `internal/pricing/pricing.go`, adjust tests/docs when needed, and then update `docs/pricing-sources.json` with any new required text or reviewed SHA256 value.
- This monitoring workflow is the only pricing path that performs network access. The `aitok` CLI remains offline by default and never syncs prices automatically.

## Release Flow

- `.github/workflows/release.yml` runs only when code is merged to `main` or a `v*` tag is pushed.
- The release job reads `VERSION` through `tools/version`; tag releases must match `VERSION` as `v<version>`.
- On `main`, the release workflow validates the project but does not publish a GitHub Release.
- On matching `v*` tags, the release job runs `make validate`, then uses GoReleaser with `.goreleaser.yml` to publish checksums, platform archives, and the Homebrew cask.
- The Homebrew cask is published to the `MagnumGoYB/homebrew-aitok` tap and installs with `brew tap MagnumGoYB/aitok` followed by `brew install --cask aitok`; docs intentionally avoid the fully qualified cask name because it repeats `aitok`.
- The cask is generated from the macOS archive set only. Linux and Windows archives are still published as GitHub Release assets, but they are not included in the Homebrew cask DSL.
- The generated cask runs a post-install macOS `xattr` hook for the installed `aitok` binary so unsigned CLI archives do not remain quarantined after Homebrew installation.
- GitHub Releases use `GITHUB_TOKEN`; publishing the tap requires the repository secret `HOMEBREW_TAP_GITHUB_TOKEN`, because the default workflow token cannot write to the separate `homebrew-aitok` repository.
- The release workflow pins GoReleaser v2 instead of using `latest`.
- Release automation does not add external network telemetry or usage upload.

## Dependabot

- `.github/dependabot.yml` checks GitHub Actions and Go module updates weekly.
- `.github/workflows/dependabot-auto-merge.yml` uses `dependabot/fetch-metadata@v3` and enables GitHub auto-merge only for non-draft Dependabot PRs that are not semantic major version updates.
- Dependabot PRs skip the human PR body validator because their generated bodies cannot fill the repository review template.
- Major dependency updates stay manual because they need explicit binary-size, offline behavior, and supply-chain impact review.
- Repository auto-merge and delete-branch-on-merge are enabled in GitHub settings for this workflow.
- The `main` branch protection requires the `metadata`, `test`, and platform build checks to pass before GitHub auto-merge can land a PR.
