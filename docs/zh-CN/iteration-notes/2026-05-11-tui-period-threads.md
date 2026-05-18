# 2026-05-11 TUI 日期与 Threads 迭代记录

这是一份随仓库版本化的交接记录。后续 AI 编码代理继续处理 TUI 日期展示、本地 thread/session 列表、source 标题提取，或 PR #13 合并后的发版跟进前，先读这里。

## 快速恢复

- 仓库：`/Users/sosbs/coding/aitok`
- PR：`https://github.com/MagnumGoYB/aitok/pull/13`
- 使用过的功能分支：`codex/tui-period-threads-list`
- 功能提交：`107b7da1f2d3a4d4207c9c4581778aca1153a45e`
- 合并提交：`97e4cbbd8bcd08e5a2667b415eb35122de434acc`
- 发版状态：PR #13 以及后续 TUI 修复已发布到 `v0.1.26`；当前迭代没有待跟进发版。
- 后续 dashboard 现代化记录在 `docs/zh-CN/iteration-notes/2026-05-18-tui-dashboard-modernization.md`。
- 主要 Agent 调用契约：`aitok --no-version-check summary --period today --threads --format json`

## 这轮迭代为什么发生

TUI period 标签与用户预期不一致，并且产品需要一种直接查看 summary 结果背后本地会话的方式。用户还明确要求 threads 选择逻辑像可滚动的进程列表，而不是分页或展开/收起面板。

因此本轮交付两类相关改动：

- 让 TUI 日期展示更准确、更干净。
- 为匹配的本地用量事件增加机器可读和 TUI 可见的 threads 列表。

## 产品决策

- 本轮不修改 period 语义。用户最早给出的 `this-week` 范围只作为显示问题示例，不作为查询窗口变更请求。
- TUI 中 `today` 只显示本地日期和时区。非 `today` 显示真实 `Window.Start` 到 `Window.End` 范围和时区。
- 日期标签不使用 emoji。
- `summary --threads` 必须显式传入，避免默认 JSON payload 变大或影响已有自动化。
- TUI threads 使用固定 BOX、固定表头、当前行高亮和滚动条。不做分页，也不做展开/收起状态。
- 复制选中 thread 时只复制 thread/session ID，使用 OSC52。不复制 resume 命令。

## Thread 标题规则

长期优先级如下：

1. custom title
2. `.codex/session_index.jsonl` 中的 Codex UI 标题 `thread_name`，或显式 AI 标题字段/事件
3. 首条真实用户消息
4. cwd basename
5. short ID

Codex 相关的重要发现：

- 当前 Codex Desktop 名称索引在 `/Users/sosbs/.codex/session_index.jsonl`。
- session JSONL 文件里经常没有同一个 UI 标题。
- 不要把普通 assistant 消息当成 AI summary title。
- 需要过滤注入/非标题消息，例如 `# Context from my IDE setup:`、`<turn_aborted>`，以及 `none` 这类占位 summary 值。

## 已交付范围

- 给 `usage.UsageEvent` 增加 thread metadata 字段。
- 增加 `query.ThreadResult` 聚合，包含 usage、requests、events、cost、source、created time 和 last active time。
- 增加 Codex、Claude、Gemini 本地 session metadata 提取。
- 增加 `summary --threads`，支持 JSON/table/markdown 报告。
- 修正 TUI period 展示。
- 增加 TUI Threads BOX，支持聚焦切换、逐行移动、home/end、滚动条和 OSC52 复制反馈。
- 更新 README 和 README.zh-CN，补充 `summary --threads` 和 TUI threads 快捷键。
- 增加实施计划：`docs/superpowers/plans/2026-05-11-tui-period-threads-list.md`。

## 2026-05-12 跟进时间线

PR #13 合并后，`main` 上继续落了多轮 TUI polish 发版。2026-05-12 的提交如下：

- `c636a71` / `v0.1.21`：统一 TUI cost 列视觉对齐，并标准化面向 Claude 的文档措辞。
- `b239c6a` + `10f1d0d` / `v0.1.22`：修复 Threads 过滤逻辑，让 active tool/search 状态、光标移动、复制动作和滚动条计算都基于过滤后的 thread 列表，并完成发版。
- `5e36baf` / `v0.1.23`：让 Threads 在 `summary --threads` payload 与 TUI 过滤视图中默认按 token 消耗降序排列。
- `8ca2d23` / `v0.1.24`：修复 Model Usage 图表在 100 万以下 token 行之间的比例显示，并在 Model Usage 表格中增加总 `Tokens` 列。
- `7492313` / `v0.1.25`：保持同一 thread ID 只出一行，把 thread `model=mixed` 改成逗号拼接的模型列表，压缩默认输出中重复的 request/event 展示，并让 Model Usage bar 在同一色系内按用量深浅区分。
- `f004bcb` / `v0.1.26`：为查询输出增加 `--sort tokens|cost`，TUI 支持用 `s` 切换排序指标，在 Threads 和 Model Usage 中显示指标标识，补齐遗漏的表头/Search 本地化，并在 Cost 排序时让 Model Usage chart 的 bar 和标签按 USD 金额展示。

本迭代完整版本序列如下：

- `v0.1.16`：合并后发布初版 TUI 日期与 threads 功能。
- `v0.1.17`：优化 Threads 布局，包括 Threads 放在 Model Usage 前、给 Model Usage 加边框、加大 ID/Name 间距、限制 thread name 展示宽度、去掉 Threads 末尾竖线，并让选中行/tab 高亮统一到 `#00B2FF` 色系。
- `v0.1.18`：修正 Threads 对齐策略，`Name`、`Tool`、`Model`、`Provider`、`Req` 左对齐，`Events`、`Cost`、`Tokens` 右对齐。
- `v0.1.19`：继续修复 TUI 布局问题：period 范围使用 ASCII `~`，缩小区域间距，Threads 溢出时渲染真实滚动条，光标移动会更新滚动条 offset，并用回归测试确认 TUI Threads 遵守选定 period window。
- `v0.1.20`：压缩 dashboard，让它更容易放进一个终端视口：toolbar 压到 3 行，summary cards 压到 4 行，Threads 最多展示 6 行，Model Usage 在 provider/model 很多时只展示 top rows，避免撑爆一屏。
- `v0.1.21`：继续统一 TUI 中 cost 列的视觉对齐，并标准化面向 Claude 的文档措辞。
- `v0.1.22`：修复 Threads 过滤逻辑，让 active tool/search 状态、光标移动、复制动作和滚动条计算都基于过滤后的 thread 列表，并让 `Model Usage` 与 `Threads` 里的 `Cost` 按渲染末端对齐。
- `v0.1.23`：让 Threads 在 `summary --threads` payload 与 TUI 过滤视图中默认按 token 消耗降序排列。
- `v0.1.24`：已修复 Model Usage 图表在 100 万以下 token 行之间的比例显示，并在 Model Usage 表格中增加总 `Tokens` 列。
- `v0.1.25`：保持同一 thread ID 只出一行，把 thread `model=mixed` 改成逗号拼接的模型列表，压缩默认输出中重复的 request/event 展示，并让 Model Usage bar 在同一色系内按用量深浅区分。
- `v0.1.26`：为查询输出增加 `--sort tokens|cost`，TUI 支持用 `s` 切换排序指标，在 Threads 和 Model Usage 中显示指标标识，补齐遗漏的表头/Search 本地化，并在 Cost 排序时让 Model Usage chart 的 bar 和标签按 USD 金额展示。
当前 TUI 布局约束需要继续保留：

- Threads 过滤逻辑必须与 active tool tabs 和 search term 保持同步。任何光标移动、Home/End 跳转、复制动作、滚动条 offset 计算，都必须基于过滤后的 thread slice，而不是原始未过滤 payload。
- 查询输出支持两种降序排序指标：默认 `tokens`，传入 `--sort cost` 后按成本排序。TUI 可用 `s` 切换当前指标，并且 Threads 与 Model Usage 必须显示当前排序指标标识。
- Model Usage 图表必须与当前指标一致：Tokens 模式按 token count 计算 bar 和右侧标签；Cost 模式按 USD cost 计算 bar 和右侧标签。
- 旧的 `t threads` 聚焦快捷键已从用户可见帮助中移除；`j/k/home/end/c` 直接作用于过滤后的 Threads 列表。
- `Cost` 要和其他数值列保持同一策略：在 Model Usage 与 Threads 中都按渲染后的末端右对齐，即使值里包含 `$`。

- Toolbar 保持紧凑，不要恢复纵向 padding。
- Summary cards 保持紧凑，避免装饰性纵向空白。
- Threads 最多展示 6 行，更多内容依赖滚动条和 `j/k/home/end` 导航。
- Model Usage 需要能处理很多 provider/model 分组，通过限制 chart 行数和 table 行数避免把 footer 挤出屏幕。
- Model Usage 图表不能把所有非零小用量行都压成同样宽度。需要用分段 block 保留更细比例，同时继续保证非零行可见。
- Model Usage 表格需要在 `Input`、`Output`、`Cached` 之外展示总 `Tokens`，方便用户直接对照图表和表格，不用手动相加。
- 日期范围分隔符使用 ASCII `~`，不是全角 `～`。
- `this-week` 仍然是 `query.WindowFor` 里的当前自然周窗口，没有改成滚动 7 天。

## 验证证据

PR 前本地验证：

- `go test ./internal/sources ./internal/query ./internal/report ./internal/tui ./internal/cli`
- `go run ./cmd/aitok summary --period today --tool codex --threads --format json --no-version-check`
- `make check`
- `make test`
- `make test-harness`
- `make build`
- `git diff --check`

`v0.1.17` 到 `v0.1.20` 后续 polish 发版期间使用过的验证：

- `go test ./internal/tui`
- `go test ./internal/tui ./internal/cli`
- `go test ./internal/query ./internal/report ./internal/sources ./internal/tui ./internal/cli`
- `make check`
- `make test`
- `make build`
- `make validate`，在 release bump 前执行
- `GITHUB_REF_NAME=vX.Y.Z GITHUB_REF_TYPE=tag go run ./tools/version --check-ref`
- `v0.1.16`、`v0.1.17`、`v0.1.18`、`v0.1.19`、`v0.1.20`、`v0.1.21`、`v0.1.22`、`v0.1.23` 的 GitHub Release workflows 均已成功。
- `v0.1.26` 本地验证已通过：`go test ./internal/query ./internal/report ./internal/cli ./internal/tui`、`make validate`、`GITHUB_REF_NAME=v0.1.26 GITHUB_REF_TYPE=tag go run ./tools/version --check-ref` 与 `git diff --check`。
- `v0.1.26` GitHub Actions 已通过：tag `v0.1.26` 的 `Release` 和 `Build`，以及 `main` 上的 `CI`、`Build`、`Release`。
- `v0.1.26` GitHub Release 已成功发布，包含 darwin、linux、windows archives 和 `checksums.txt`：`https://github.com/MagnumGoYB/aitok/releases/tag/v0.1.26`。

GitHub PR 检查：

- `test`：通过
- `metadata`：通过
- `checklist`：PR 标记 ready 后通过
- linux amd64/arm64、darwin amd64/arm64、windows amd64 的 `build`：通过

## 发版跟进

截至 `v0.1.26`，原始 PR #13 范围以及 2026-05-12 query sort / Cost chart 跟进都没有待跟进发版。

## 后续工作

- 增加可选快捷键复制 resume 命令，例如 `codex resume <id>` 或 Claude 对应命令。
- 随 Codex、Claude、Gemini 日志 schema 演进，继续改进标题提取。
- 考虑做 git root 归一化，让深层 `cwd` 在 thread 和 project 视图中更干净。
