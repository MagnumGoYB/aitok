# Harness Engineering

[English](../harness-engineering.md)

Harness 是一组前馈指南和反馈传感器：前馈指南在代理编辑前约束方向，反馈传感器在代理编辑后捕获漂移。

对 `aitok` 来说，Harness 必须轻量：Go 测试、Makefile、PR 元数据校验、CI 门禁和简洁的代理指南。它保护离线 Token 统计契约，同时不引入后台服务。

## 前馈指南

- `AGENTS.md` 和 `AGENTS.zh-CN.md`：仓库使命、编码约束、验证矩阵、隐私边界和交付规则。
- `README.md`：面向用户的 CLI 使用和安装路径。
- `CONTRIBUTING.md`：贡献者验证和离线优先规则。
- `Makefile`：标准本地命令：`make setup`、`make check`、`make test`、`make test-harness`、`make vet`、`make build`、`make validate`、`make validate-pr-body` 和 `make commitlint`。
- `tools/commitlint` 和 `.githooks/commit-msg`：仓库内 Go 提交消息校验，约束 `{emoji} {type}{scope}: {subject}`，不引入 Node/npm 工具链。`make setup` 会为本地提交启用该 hook。
- `.github/pull_request_template.md`：可重复的 PR 检查清单，覆盖需求分类、验收标准、测试证据、验证、回滚和残余风险。
- `.github/workflows/ci.yml`：与本地门禁一致的托管验证。

## 反馈传感器

- `go test ./...`：覆盖 source adapter、时间窗口、聚合、报告、CLI、setup 和 TUI smoke。
- `go test ./harness`：仓库结构传感器，检查 agent docs、Makefile 命令、CI 门禁、PR 模板和离线/隐私约束。
- `go vet ./...`：静态分析。
- `go build ./cmd/aitok`：单二进制构建检查。
- `go run ./tools/validate-pr-body`：可执行 PR body 元数据门禁。
- `make setup`：一次性本地设置，执行 `git config core.hooksPath .githooks`。
- `make commitlint COMMIT_MSG_FILE=<commit-msg-file>`：可执行提交消息门禁，setup 后通过 `.githooks/commit-msg` 接入，并由 PR CI 校验 PR 最新提交。
- `.cache/aitok/`：仓库内、git 忽略的 Go build/module cache，供 Makefile 目标使用，让 agent 校验绑定当前 checkout，而不是临时拼接 `/tmp` 路径。

## 代理工作流契约

- 编辑前先分类请求：feature、bugfix、refactor、harness/tooling 或 analysis-only。
- 实现前锁定可观察的验收标准。
- 可行时，先添加失败测试、Harness 传感器或明确的手动验证清单，再改变行为。
- 编辑范围限制在声明的文件/目录内。
- 交付前把每条验收标准映射到证据。
- 说明跳过的验证和残余风险。

## aitok 专属约束

- 默认不新增网络上传、同步、telemetry 或远程报告。
- 不读取、保存、打印、哈希或指纹化真实 API Key。
- Source adapter 必须保持流式扫描并有 fixture 测试。
- CLI 输出保持稳定，便于自动化。
- TUI 是可选交互界面；JSON 和 Markdown 报告仍是一等能力。
- Gemini CLI 支持依赖本地 telemetry 配置。`setup gemini` 必须保持 `logPrompts=false`。

## 更新 Harness

当修改 Harness、CI、PR workflow 或验证脚本时：

1. 更新可执行脚本、测试或 workflow。
2. 如果代理行为变化，同步更新 `AGENTS.md` 和 `AGENTS.zh-CN.md`。
3. 更新本文档和 `docs/harness-engineering.md`。
4. PR 元数据规则变化时运行 `make check`、`make test-harness` 和 `make validate-pr-body`。

当修改提交规范时，同步更新 `tools/commitlint`、`.githooks/commit-msg`、`Makefile`、`.github/workflows/pr.yml`、`AGENTS.md`、`AGENTS.zh-CN.md`、本文档和 `docs/harness-engineering.md`。

- `CODE_OF_CONDUCT.md` / `CODE_OF_CONDUCT.zh-CN.md` 和 `SUPPORT.md` / `SUPPORT.zh-CN.md` 保持开源社区指南双语。

## GitHub 自动化覆盖

- `docs/github-automation.md` 和 `docs/zh-CN/github-automation.md` 记录 PR review 提示、bugfix、build、release 和 Dependabot 自动合并流程。
- `.github/workflows/pr.yml` 校验真实 PR 元数据。
- `.github/workflows/pr-review.yml` 使用 `issues: write` 和 `pull-requests: write` 权限发布 review checklist，确保可以创建或更新 PR issue comment。
- `.github/CODEOWNERS` 将高风险区域路由给维护者，不强制依赖付费 AI review 自动化。
- `.github/workflows/dependabot-auto-merge.yml` 只为非 major Dependabot 更新启用 GitHub auto-merge。
- `.github/workflows/build.yml` 上传跨平台构建产物。
- `.github/workflows/release.yml` 通过 GoReleaser 发布 tag release。
- `.github/ISSUE_TEMPLATE/bug_report.yml`、`.github/CODEOWNERS` 和 `.github/dependabot.yml` 明确开源维护路径。
