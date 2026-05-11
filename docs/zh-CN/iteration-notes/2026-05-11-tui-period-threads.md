# 2026-05-11 TUI 日期与 Threads 迭代记录

这是一份随仓库版本化的交接记录。后续 AI 编码代理继续处理 TUI 日期展示、本地 thread/session 列表、source 标题提取，或 PR #13 合并后的发版跟进前，先读这里。

## 快速恢复

- 仓库：`/Users/sosbs/coding/aitok`
- PR：`https://github.com/MagnumGoYB/aitok/pull/13`
- 使用过的功能分支：`codex/tui-period-threads-list`
- 功能提交：`107b7da1f2d3a4d4207c9c4581778aca1153a45e`
- 合并提交：`97e4cbbd8bcd08e5a2667b415eb35122de434acc`
- 发版状态：合并后需要发版，本记录中尚未完成发版
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

## 验证证据

PR 前本地验证：

- `go test ./internal/sources ./internal/query ./internal/report ./internal/tui ./internal/cli`
- `go run ./cmd/aitok summary --period today --tool codex --threads --format json --no-version-check`
- `make check`
- `make test`
- `make test-harness`
- `make build`
- `git diff --check`

GitHub PR 检查：

- `test`：通过
- `metadata`：通过
- `checklist`：PR 标记 ready 后通过
- linux amd64/arm64、darwin amd64/arm64、windows amd64 的 `build`：通过

## 发版跟进

PR #13 已合并，并且属于 feature + bugfix。下一位 Agent 应继续进入仓库既有发版流程，除非用户明确要求延后。

发版前：

- 确认本地 `main` 已更新到 `origin/main`。
- 跑当前 release flow 要求的仓库发布验证。
- Release notes 中包含 TUI 日期修复和 threads 功能。

## 后续工作

- 增加可选快捷键复制 resume 命令，例如 `codex resume <id>` 或 Claude 对应命令。
- 随 Codex、Claude、Gemini 日志 schema 演进，继续改进标题提取。
- 考虑做 git root 归一化，让深层 `cwd` 在 thread 和 project 视图中更干净。
