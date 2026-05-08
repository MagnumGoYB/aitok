# GitHub Automation

[English](../github-automation.md)

本仓库使用 GitHub 原生自动化覆盖 PR、review 提示、bug report、跨平台构建和 release。

## Pull Request 流程

- `.github/pull_request_template.md` 要求填写需求分类、验收标准、测试证据、验证、回滚和残余风险。
- `.github/workflows/pr.yml` 使用 `make validate-pr-body` 校验真实 PR body。
- `.github/workflows/ci.yml` 在 push 和 pull request 上运行本地验证和 Harness 门禁。

## Review 流程

- `.github/workflows/pr-review.yml` 会在新建或更新 PR 时发布 checklist 评论。
- Checklist 提醒 reviewer 检查离线/隐私边界、source adapter 流式扫描、fixture 覆盖、CLI 输出稳定性和 release 影响。
- `.github/CODEOWNERS` 为 adapter、query/report、harness 和 GitHub workflows 等核心区域请求 review。

## Bugfix 流程

- `.github/ISSUE_TEMPLATE/bug_report.yml` 收集工具、时间范围、命令、预期行为、实际行为、脱敏日志和验证证据。
- Bugfix PR 在可行时必须先添加失败测试、fixture、Harness 传感器或明确的手动复现，再修复。
- PR 模板要求失败/边界覆盖和回滚说明。

## Build 流程

- `.github/workflows/build.yml` 在 PR 和 push 上构建 Linux、macOS 和 Windows 产物。
- 构建产物会上传供检查，但不会发布 release。
- `make build` 仍是本地单平台构建门禁。

## Release 流程

- `.github/workflows/release.yml` 在 `v*` tag 上触发。
- Release job 先运行 `make validate`，再使用 GoReleaser 和 `.goreleaser.yml` 发布校验和与多平台 archive。
- Release 仅需要 `GITHUB_TOKEN`；release 自动化不新增外部 telemetry 或 usage 上传。

## Dependabot

- `.github/dependabot.yml` 每周检查 GitHub Actions 和 Go module 更新。
- 依赖更新仍必须通过 `make validate`，并在相关时说明体积、离线行为和供应链影响。
