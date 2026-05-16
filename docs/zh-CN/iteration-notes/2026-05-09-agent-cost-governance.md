# 2026-05-09 Agent 成本治理迭代记录

这是一份随仓库版本化的交接记录。后续 AI 编码代理继续处理成本治理、价格覆盖、预算、`doctor`、GitHub 自动化或发布工作前，先读这里。

## 快速恢复

- 仓库：`/Users/sosbs/coding/aitok`
- PR：`https://github.com/MagnumGoYB/aitok/pull/11`
- Release：`https://github.com/MagnumGoYB/aitok/releases/tag/v0.1.15`
- 使用过的功能分支：`codex/aitok-local-cost-governance`
- 发布提交：`918137e75c9412fe831eeef60de82a23427b870e`
- 发布标签：`v0.1.15`
- 主要 Agent 调用契约：`--format json --no-version-check`

## 这轮迭代为什么发生

产品 review 认为最高价值不是再做一张 token 表，而是补本地成本治理能力。用户和 AI Agent 真正需要回答的是：

- 我有没有超预算？
- 哪个 tool、model 或工作目录异常？
- 自动化能否稳定调用，而不是解析人类文本？
- 这些能力能否完全基于本地日志离线运行？

因此实现优先级是性能、离线行为和稳定的机器可读 CLI 输出。

## 时间线

1. 头脑风暴核心价值：预算/阈值检查、趋势/对比、项目维度洞察、更强的 `doctor`、价格覆盖 audit。
2. 在 PR #11 实现第一段治理能力：流式扫描、累加器聚合、价格 audit、预算检查、`doctor` 增强。
3. 排查价格覆盖报告：`known 20231`、`unknown 30`、`unknown_models 1`。
4. 修复 `codex-auto-review / bcb` 缺少默认价格覆盖的问题。
5. 把误导性的价格报告字段从 `unknown_*` 调整为 `unpriced_*` / `priced_events`。
6. 合并 PR #11。
7. 发布 `v0.1.15`，GoReleaser 产出 darwin、linux、windows 包。
8. 增加这份仓库级交接记录，避免后续会话依赖 Codex 全局 memory 索引。
9. 移除常驻 CodeRabbit review，因为持续付费的 PR review 门禁对本仓库不划算。
10. 增加 `make setup` 和 PR range commitlint，让人工提交和 Agent 提交共用同一套提交消息契约。
11. 增加显式 PR 发版判定门禁：工程/流程优化不触发软件发版，feature 和 bugfix 必须标记后续发版或明确延后。

## 产品决策

- `aitok` 仍然是面向人类的 CLI，但 AI Agent 和自动化调用可靠性是一级产品优先级。
- 稳定自动化路径是 `--format json --no-version-check`。
- JSON stdout 必须保持完整、可机器解析。
- 人类可读 warning、版本提示、预算失败解释应进入 stderr 或错误路径。
- `budget check` 在预算超限时可以返回非零退出码，但仍必须在 stdout 输出结构化 JSON。
- 离线优先是硬约束：不上传用量数据、不做远程同步、不自动联网同步价格。
- 成本是离线估算，不是账单对账。

## 已交付范围

- 增加流式 source 扫描：`internal/sources.Scan` 和 `sources.ForEach`。
- 增加 `internal/query.Accumulator`，支持更低内存的聚合路径。
- 增加 `aitok pricing audit`。
- 增加 `aitok budget check --limit-usd`。
- 增强 `aitok doctor --format table|json|markdown`，作为 onboarding/audit 入口。
- 增加治理报告代码：`internal/report/governance.go`。
- 增加查询 benchmark：`internal/query/query_bench_test.go`。
- 更新 README、AGENTS 和计划文档。

## 价格覆盖细节

那次让人困惑的报告不是说所有事件都是 unknown。实际情况是大部分事件已定价，另外 30 个事件来自同一个缺失的 model/provider 组合。

根因：

- 默认价格表缺少 `codex-auto-review / bcb`。

修复：

- 增加该 model/provider 的默认价格覆盖。
- 把报告语言从 `unknown_*` 改成 `unpriced_*` / `priced_events`。

后续价格相关工作要继续区分：

- 可解析事件
- 已定价事件
- 未定价事件
- 未识别的 model/provider 组合

## 2026-05-13 用量准确性 BUG 修复

发版目标：`v0.1.28`。

用户报告 `summary --period yesterday` 与本地已验证的 CC-Switch 结果不一致，对应窗口是 `2026-05-12 00:00 ~ 2026-05-13 00:00`。本次 BUG 修复把 CC-Switch 作为参考产品口径。

根因：

- Codex session 日志里的 `total_token_usage` 是累计计数器。`aitok` 之前把每条 `last_token_usage` 当作独立请求，只去掉完全重复行，导致所有 period 下的 token、请求数、缓存 token 和成本都会漂移。
- Claude session 日志里同一个 `message.id` 可能出现多行；没有 `stop_reason` 的中间行不是最终 API 调用。`aitok` 之前把这些中间行也计入统计。
- 模型名没有稳定归一化 provider 前缀和日期后缀。
- 成本公式仍会让非 Codex 工具的 cached input 走完整 input 价格，并把 Codex `reasoning_output_tokens` 当作 output 成本计费。CC-Switch 的成本公式基于 `(input_tokens - cache_read_tokens)`、`output_tokens`、`cache_read_tokens` 和 `cache_creation_tokens`。

修复：

- Codex 优先读取 `total_token_usage`，按文件内累计值计算 delta，跳过零增量边界行；只有缺少累计值时才回退到 `last_token_usage`，并补充扫描 `~/.codex/archived_sessions`。
- Claude 按 `message.id` 聚合，只保留带 `stop_reason` 的最终行；最终性相同时保留 output 更大的行，并跳过未完成或零 output 行。
- Codex 和 Claude 模型名统一转小写、去 provider 前缀、去日期后缀。
- 离线成本估算改为 CC-Switch 公式，并且不再把 reasoning token 叠加进 output 成本。

修复后的参考 smoke：

- Period：`yesterday`，窗口 `2026-05-12T00:00:00+08:00` 到 `2026-05-13T00:00:00+08:00`。
- 请求数：`875`。
- 总 token：`60,718,109`；input `60,398,385`；output `319,724`。
- 缓存 token：`103,962,752`。
- 成本：`$75.8474705`，展示为 `$75.8475`。
- 模型总量：`claude-opus-4-7=852,249`、`gpt-5.5=43,968,237`、`gpt-5.4=15,897,623`。

验证：

- `go test ./internal/pricing ./internal/sources`。
- `make test`。
- `make build`。
- `git diff --check`。
- `go run ./cmd/aitok summary --period yesterday --format json --no-version-check`。
- `go run ./cmd/aitok summary --period this-week --format json --no-version-check`。

## 工具和缓存决策

## 2026-05-14 交互式价格配置

发布目标：`v0.1.29`。

后续功能：现在可以通过 `aitok pricing configure` 在终端里创建本地价格覆盖。

关键合同：

- 命令写入 `~/.aitok/pricing.json`，文件权限为 `0600`，同时支持终端编号 Q/A 问答和可脚本化的纯 flag 配置方式。
- 匹配维度仍然是 `model match + provider/auth label`；不能输入、存储、展示、哈希或指纹化真实 API Key。
- 用户想为不同账号或 API-key 路由设置不同价格时，只能使用本地工具日志中已有的 provider/auth 标签，例如 Codex `model_provider` 或 Gemini `auth_type`。
- JSON 模式下交互提示走 stderr，stdout 只输出最终机器可读 payload。

本切片验证：

- `make check`
- `make test`
- `make test-harness`
- `make build`
- 使用 `make run ARGS="--home /private/tmp/aitok-pricing-smoke --no-version-check pricing configure --format json"` 做 CLI smoke
- 本地 `toska` provider 价格验证：summary 成本与配置的 input/output/cache-hit 费率手算一致，且 `pricing audit --provider toska` 在该周期内没有未定价行。

早期验证使用 `/tmp` 或 `/private/tmp` 存 Go cache，只是因为沙箱拒绝写入 Go 默认的 `~/Library/Caches`。

长期项目内约定是：

- 验证缓存放在 `.cache/aitok`
- 优先使用 Makefile 目标，不使用临时拼接命令作为项目工具
- checkout 内 CLI smoke 使用 `make run ARGS="..."`，不要直接裸跑 `go run`
- 每个 checkout 运行一次 `make setup`，启用 `.githooks/commit-msg`
- 提交信息校验使用 `make commitlint COMMIT_MSG_FILE=<commit-msg-file>`
- PR CI 校验 PR range 内每个提交消息，所以没有运行本地 setup 的贡献者也会被覆盖

类似 `/tmp/aitok-commit-msg` 的文件不是项目工具，只是一次性命令输入。

## 2026-05-14 Codex Provider 切换成本归属

Bugfix 目标：`v0.1.29` 之后的下一个 patch release。

用户反馈：同一个 Codex 会话里切换 provider 后，最终估算成本可能仍归到创建 session 时的 provider，而不是每次请求实际使用的 provider。

根因：

- Codex 解析把 `state.provider` 保持为 `session_meta.model_provider`。
- 后续 turn 已经带有 `team-b/gpt-5.4` 这类 provider-qualified model 字符串，但 `aitok` 只在模型归一化时剥掉前缀，没有把该前缀同步成事件 provider。
- 部分 Codex turn 只有 `gpt-5.5` 这类裸模型名；这类行只有在本地 Codex 日志里存在请求 URL，并且 URL host 能唯一匹配到某个已配置 provider 时，才能恢复单次请求 provider。
- provider-specific pricing 因此可能把后续请求继续合并到 session 初始 provider 的 bucket 里。

修复：

- Codex model 解析现在同时返回归一化模型名和可选 provider 前缀。
- `turn_context.payload.model`、token-count `info.model`、token-count `info.model_name`、token-count `payload.model` 在模型带 provider 前缀时都会更新当前事件 provider。
- 对裸 Codex 模型名，aitok 也会读取本地 Codex 请求日志证据，并在请求 host 能唯一映射到 `~/.codex/config.toml` 的 `[model_providers.*].base_url` 时归属 provider。同一个 turn 的请求证据会作用于该 turn 的全部 token-count 行，包括早于 request-completed 日志的 token-count。两个 provider 锚点之间缺少直接证据的裸模型 turn 会保守沿用前一个 provider 段，直到后续切换证据出现。
- 没有 provider-qualified model 的旧日志继续 fallback 到 `session_meta.model_provider`，保持兼容。
- 未知 host、多个 provider 共享同一 host、缺少请求 URL 证据，以及 provider URL 变化后当前本地 config 不再包含旧 host 的情况，都不会猜测归属。
- 查询分组只会为了 Model Usage 展示重分配被 exact request 证据夹住的短 inferred 桥接段；较长的 inferred provider 段仍按事件自身携带的 provider 汇总。

验收：

- 单个 Codex session 中先出现 `team-a/gpt-5.4`、后出现 `team-b/gpt-5.4` 时，会输出独立的 provider bucket。
- 单个 Codex session 中裸模型名也可以在同一 turn 有唯一请求 host 证据时拆分 provider。
- 同 provider 请求 URL 变化时，不会把前一个 host 归属泄漏到后续 turn；未知的新 URL 会回退到 session/model 元数据。
- provider-specific pricing 会分别使用 `team-a` 与 `team-b` 的价格，不再把两次请求都算到初始 provider。
- 行为仍然是 offline-only，不读取 API Key。

本切片验证：

- `make test-packages PKGS="./internal/sources ./internal/cli"`

## 流程自动化更新

CodeRabbit：

- CodeRabbit 已在仓库外卸载/禁用，仓库内 `.coderabbit.yaml` 已删除。
- 必需 review 门禁保留在 GitHub 原生 checklist comment、CODEOWNERS、branch protection、required checks 和受限 Dependabot 自动合并上。
- 默认不要重新引入常驻付费 AI review 自动化。只有高风险变更明确值得成本时，才按一次性决策使用付费或外部 AI review。

Commitlint：

- `make setup` 会执行 `git config core.hooksPath .githooks`。
- `.githooks/commit-msg` 执行 `make commitlint COMMIT_MSG_FILE="$1"`。
- `.github/workflows/pr.yml` 会用 `make commitlint-range COMMIT_RANGE="<base>..<head>"` 校验 PR range 内每个提交消息；本地 setup 有帮助，但不是唯一门禁。
- 提交 emoji 与 type 强绑定，且每个 type 只允许一个值：`✨ feat`、`🐛 fix`、`📝 docs`、`👷 ci`、`💄 style`、`♻️ refactor`、`🔖 release`、`⚡️ perf`、`✅ test`、`🔧 chore`、`🏗️ build`。

发版判定：

- `.github/pull_request_template*.md` 现在包含必填的 `Release Decision` 区块。
- `tools/validate-pr-body` 要求该区块存在，并拒绝 feature/bugfix PR 写“无需发版”但没有明确延后的情况。
- Harness、docs、CI、workflow guardrails 或仓库自动化等工程/流程优化应标记 `Release not required`。
- Feature 和 bugfix 应标记 `Release required after merge`，并在合并后继续进入既有发版流程，除非用户明确延后。

## 验证证据

功能 PR 本地跑过：

- `go test ./internal/cli`
- `go test ./internal/pricing ./internal/report ./internal/cli`
- `go test ./internal/query -bench . -benchmem`
- `make check`
- `make test`
- `make build`

发布验证：

- 打 tag 前本地 `make validate` 通过。
- GitHub Actions release run 成功：`https://github.com/MagnumGoYB/aitok/actions/runs/25598864490`
- GoReleaser 已上传 darwin、linux、windows 产物和 `checksums.txt`。

## GitHub 自动化注意事项

本仓库里 `gh pr edit` 可能因为 GitHub Projects classic GraphQL 字段废弃而失败。如果 PR body 编辑遇到这个问题，改用 REST API：

```sh
jq -Rs '{body:.}' <body.md> > <body.json>
gh api repos/MagnumGoYB/aitok/pulls/<number> -X PATCH --input <body.json>
```

PR metadata 校验需要明确覆盖需求分类、验收标准证据、单元测试、失败/边界覆盖、跳过验证说明和残余风险。

PR metadata 现在还必须包含明确的发版判定。对 feature 和 bugfix PR 来说，除非正文写明用户批准延后，否则 `Release not required` 是无效的。

本地可以通过设置 `PR_BODY` 为真实 PR body 来测试 `make validate-pr-body`。如果不提供 `PR_BODY` 或 `GITHUB_EVENT_PATH`，该命令按设计会因空 PR body 失败。

## 后续工作

最高价值：

- 增加 trend/compare：today vs yesterday、this week vs last week、month daily breakdown。
- 改进 project/repo 归一化，把深层 `cwd` 折叠到 git root。
- 继续深化 `doctor`：数据源路径、最近事件时间、Gemini telemetry 安全、价格覆盖率、未定价模型。

工程约束：

- 后续性能工作保持 streaming-first，并用 benchmark 验证。
- JSON 输出字段变更要保持稳定并补测试。
- 不增加用量上传或自动价格联网同步。
