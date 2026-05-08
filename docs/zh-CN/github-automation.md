# GitHub Automation

[English](../github-automation.md)

本仓库使用 GitHub 原生自动化覆盖 PR、CodeRabbit review、review 提示、Dependabot 自动合并、bug report、pricing-watch 提醒、跨平台构建和 release。

## Pull Request 流程

- `.github/pull_request_template.md` 要求填写需求分类、验收标准、测试证据、验证、回滚和残余风险。
- `.github/workflows/pr.yml` 使用 `make validate-pr-body` 校验真实 PR body。
- `.github/workflows/ci.yml` 在 push 和 pull request 上运行本地验证和 Harness 门禁。

## Review 流程

- `.coderabbit.yaml` 配置针对 `main` 目标分支 PR 的 CodeRabbit 自动 review。
- CodeRabbit review 使用 zh-CN 评论、assertive profile、request-changes workflow，并对 Go 代码、GitHub workflows、docs 和 harness 文件设置路径级审查指令。
- 仓库 PR 仍需要安装 CodeRabbit GitHub App；YAML 文件只定义仓库级行为。
- `.github/workflows/pr-review.yml` 会在新建或更新 PR 时发布 checklist 评论。
- Checklist workflow 使用 `issues: write` 和 `pull-requests: write`，确保 `actions/github-script` 能在 branch protection 下创建或更新 PR issue comment。
- Checklist 提醒 reviewer 检查离线/隐私边界、source adapter 流式扫描、fixture 覆盖、CLI 输出稳定性和 release 影响。
- `.github/CODEOWNERS` 为 adapter、query/report、harness 和 GitHub workflows 等核心区域请求 review。

## Bugfix 流程

- `.github/ISSUE_TEMPLATE/bug_report.yml` 收集工具、时间范围、命令、预期行为、实际行为、脱敏日志和验证证据。
- Bugfix PR 在可行时必须先添加失败测试、fixture、Harness 传感器或明确的手动复现，再修复。
- PR 模板要求失败/边界覆盖和回滚说明。

## Build 流程

- `.github/workflows/build.yml` 只在代码合并到 `main` 或推送 `v*` tag 时构建 Linux、macOS 和 Windows 产物。
- Build workflow 会通过 `tools/version` 读取仓库内 `VERSION` 文件后再产出构建物。
- 构建产物会上传供检查，但不会发布 release。
- `make build` 仍是本地单平台构建门禁。
- GitHub Actions workflows 使用 Node 24 action major，例如 `actions/checkout@v6`、`actions/setup-go@v6`、`actions/upload-artifact@v6` 和 `actions/github-script@v8`。

## Pricing Watch 流程

- `.github/workflows/pricing-watch.yml` 每天定时运行，也支持 `workflow_dispatch` 手动触发。
- Workflow 运行 `go run ./tools/pricing-watch`，抓取 `docs/pricing-sources.json` 中记录的可机器读取官方价格页，检查 model 名称、缓存章节等 required pricing text，并且只对足够稳定的源比对标准化 SHA256。
- 对阻止自动抓取的官方页面，例如 OpenAI pricing page，在 `docs/pricing-sources.json` 中标记为 `manual_review`；maintainer 在价格更新时人工核对，避免 CI 因反爬响应持续失败。
- 当官方价格源发生变化时，workflow 会创建或更新一个带 `pricing-watch` label 的 issue，不会自动修改代码。
- Maintainer 需要人工核对官方价格页，更新 `internal/pricing/pricing.go`，必要时同步测试/文档，然后把新的 required text 或已审核 SHA256 写回 `docs/pricing-sources.json`。
- 该监控 workflow 是唯一会访问网络的价格相关路径。`aitok` CLI 默认仍保持离线，不会自动同步价格。

## Release 流程

- `.github/workflows/release.yml` 只在代码合并到 `main` 或推送 `v*` tag 时触发。
- Release job 通过 `tools/version` 读取 `VERSION`；tag release 必须与 `VERSION` 匹配为 `v<version>`。
- 在 `main` 上，release workflow 只验证项目，不发布 GitHub Release。
- 在匹配的 `v*` tag 上，release job 先运行 `make validate`，再使用 GoReleaser 和 `.goreleaser.yml` 发布校验和、多平台 archive 和 Homebrew cask。
- Homebrew cask 发布到 `MagnumGoYB/homebrew-aitok` tap，并通过 `brew tap MagnumGoYB/aitok` 后执行 `brew install --cask aitok` 安装；文档刻意不使用完整 cask 名称，避免重复出现 `aitok`。
- Cask 只从 macOS archive 集合生成。Linux 和 Windows archive 仍会作为 GitHub Release 产物发布，但不会进入 Homebrew cask DSL。
- 生成的 cask 会在 macOS post-install 阶段对已安装的 `aitok` 二进制运行 `xattr` hook，避免未签名 CLI archive 在 Homebrew 安装后仍保留 quarantine 状态。
- GitHub Release 使用 `GITHUB_TOKEN`；发布 tap 需要仓库 secret `HOMEBREW_TAP_GITHUB_TOKEN`，因为默认 workflow token 不能写入独立的 `homebrew-aitok` 仓库。
- Release workflow 固定使用 GoReleaser v2，不使用 `latest`。
- Release 自动化不新增外部 telemetry 或 usage 上传。

## Dependabot

- `.github/dependabot.yml` 每周检查 GitHub Actions 和 Go module 更新。
- `.github/workflows/dependabot-auto-merge.yml` 只为非 draft 且不是 semantic major version 的 Dependabot PR 启用 GitHub auto-merge。
- Major 依赖更新保持人工处理，因为需要明确 review 二进制体积、离线行为和供应链影响。
- GitHub 仓库设置已启用 auto-merge 和 delete-branch-on-merge，供该 workflow 使用。
- `main` branch protection 要求 `metadata`、`test` 和各平台 build checks 通过后，GitHub auto-merge 才能落地 PR。
