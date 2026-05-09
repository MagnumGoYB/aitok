# AGENTS

[English](AGENTS.md)

此文件是本仓库中 AI 编码代理的执行指南。面向用户的过程更新和交付说明默认使用 zh-CN；代码、命令名、测试名、上游文档和引用原文保持 as-is。

## 1) 项目使命

`aitok` 是一个离线优先的 Go CLI，用于统计本机 Claude Code、Codex、Gemini CLI 的 Token 用量。
它可以基于本地 Token 统计、离线价格表和用户本地覆盖配置预估 USD 成本。
虽然它是面向人类用户的命令行工具，但产品和工程决策要优先保证 AI Agent 与自动化流程可以可靠调用。

当前代码库形态：

- Go 1.26.3+
- 单二进制 CLI：`cmd/aitok`
- 适配器：`internal/sources`
- 查询聚合：`internal/query`
- 价格表和成本估算：`internal/pricing`
- 报告输出：`internal/report`
- Gemini 本地 telemetry 设置：`internal/setup`
- TUI：`internal/tui`
- Harness 传感器：`harness`

除非明确要求，否则不在当前范围内：

- 网络上传、远程同步或云端统计
- 读取、保存、打印、哈希或指纹化真实 API Key
- 账单对账或声称与 provider 账单完全一致
- 把 TUI/Web 仪表盘做成后台常驻服务

## 2) 开发命令

- 格式和静态检查：`make check`
- 全量测试：`make test`
- Harness-only 测试：`make test-harness`
- Vet：`make vet`
- 构建：`make build`
- 完整本地验证：`make validate`
- PR 元数据校验：`make validate-pr-body`
- 提交消息检查：`make commitlint COMMIT_MSG_FILE=<commit-msg-file>`
- 直接运行 CLI：`go run ./cmd/aitok summary --period today`

## 3) 迭代前自我约束流程

每次产品或 Harness 迭代在编辑文件前，先做简短内部 Review：

1. 需求分类
- 将请求分类为 feature、bugfix、refactor、harness/tooling 或 analysis-only。
- 写明用户可见结果、目标平台和影响区域。
- 如果需求有歧义，先写出最安全的具体假设；只有错误假设会造成明显数据或产品风险时才提问。

2. 技术栈匹配
- 优先使用 Go 标准库和现有包边界。
- 新依赖必须有明确收益，并说明对单二进制体积、离线行为和供应链风险的影响。
- 解析本地日志时必须使用流式扫描，不允许一次性把大型 JSONL 文件全部载入内存。

3. 验收和回归计划
- 实现前写出可观察的验收标准。
- 至少覆盖一个失败或边界场景：空目录、损坏 JSONL、缺字段、重复事件、未开启 Gemini telemetry、未知 provider。
- 每个验收标准都要映射到单元测试、Harness 测试、CLI smoke、手动验证或明确的不适用理由。

4. 范围保护
- 明确预期修改的文件或目录。
- 不要混入无关重构、格式化 churn、依赖升级或发布流程改造。
- Harness、CI、文档、生产代码职责分离。

5. 交付就绪
- 迭代时运行最小目标检查。
- 交付前根据变更范围运行第 6 节验证矩阵。
- 总结残余风险和无法运行的验证。

## 4) 编码规则

- 所有 source adapter 输出统一的 `usage.UsageEvent`。
- 不读取、不保存、不展示真实 API Key。provider 维度只能来自 CLI 日志中的 provider/auth_type 或 `unknown`。
- 不新增用量数据网络传输能力。唯一允许的命令启动网络行为是低频 GitHub release 元数据版本检查；它不得读取本地日志或上传用量数据，并且必须可通过 `--no-version-check` 或 `AITOK_NO_VERSION_CHECK=1` 跳过。
- 成本估算必须默认保持离线。默认价格可基于公开 provider 价格更新，但自动网络同步必须明确请求且显式 opt-in。
- Gemini CLI 历史数据以已有 local telemetry outfile 为准；未开启时必须如实报告没有可解析历史数据。
- CLI 输出必须稳定，JSON 字段变更需有测试覆盖。
- AI Agent 应将 `--format json` 加 `--no-version-check` 作为主要自动化调用契约。
- JSON 命令的 stdout 必须保持完整、可机器解析的 JSON payload。面向人类的 warning、预算失败摘要和版本提示应进入 stderr 或返回错误路径。
- 退出码属于调用契约：`budget check` 预算超限时返回非零状态，但仍需把结构化 payload 写入 stdout。
- Markdown/table 报告要保持可读且可脚本化。
- TUI 不得替代 CLI/JSON 输出；自动化场景必须能绕过 TUI。
- 生产代码不要依赖 `harness/` 或 `tools/`。

## 5) Harness 维护规则

- 当重复问题来自缺失上下文，更新 `AGENTS.md` / `AGENTS.zh-CN.md` 或 docs。
- 当问题可确定性检测，新增或更新 `harness` 测试。
- 当修改 Harness、CI、PR workflow 或验证脚本，必须同步更新：
  - `docs/harness-engineering.md`
  - `docs/zh-CN/harness-engineering.md`
  - `.github/pull_request_template.md`（如 PR 约束变化）
- Harness 测试只检测仓库结构、流程约束和文档/脚本一致性，不测试产品业务逻辑。

## 6) 回归验证矩阵

- 文档或 Harness only：`make check`、`make test-harness`，PR 规则变化时加 `make validate-pr-body`。
- Source adapter：`make test`，并覆盖正常、空目录、损坏 JSONL、缺字段、重复事件。
- Query/report：`make test`，并覆盖时间窗口、过滤、分组、JSON/Markdown 稳定输出。
- CLI/TUI：`make test`、`make build`，必要时跑 `go run ./cmd/aitok doctor` 或 `summary` smoke。
- 依赖、CI、发布配置：`make validate`，并说明体积/离线/供应链影响。

## 7) CI 和提交流程

- CI 必须运行 `make validate`、`make test-harness`。
- PR 必须包含 Summary、Requirement Classification、Acceptance Criteria、Changed Areas、TDD / Test Evidence、Validation、Risk and Rollback。
- 提交消息必须符合仓库内 Go commitlint 格式：`{emoji} {type}{scope}: {subject}`。可运行 `make commitlint COMMIT_MSG_FILE=<commit-msg-file>`，或通过 `git config core.hooksPath .githooks` 启用 `.githooks/commit-msg`。
- 不要绕过本地验证后声称完成。
- Stage/commit 只包含当前迭代文件；不要纳入无关 dirty files。

## 8) 开源文档和 GitHub 自动化

- 每个公开指南或策略文档都需要 zh-CN 对照，例如 `README.md` + `README.zh-CN.md`、`CONTRIBUTING.md` + `CONTRIBUTING.zh-CN.md`。
- GitHub PR、review、bugfix、build、release workflow 变化必须同步记录到 `docs/github-automation.md` 和 `docs/zh-CN/github-automation.md`。
- 修改 release 自动化时，运行 `make validate`，并确认 `.goreleaser.yml` 与 `.github/workflows/release.yml` 保持一致。
