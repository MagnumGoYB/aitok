# aitok 日期展示与 Threads 交互计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修正 TUI 日期展示，并新增可交互 threads 窗口与 `summary --threads` 机器输出。

**Architecture:** 在 source 解析阶段补充会话元信息，在 query 层按 thread 聚合，report/CLI 只在显式开启 `--threads` 时输出 threads。TUI 复用同一 payload 渲染固定表头、焦点驱动滚动条和 OSC52 复制 ID，不改变现有 period 查询语义。

**Tech Stack:** Go 1.26.3+、Cobra CLI、Bubble Tea、Lip Gloss、标准库 JSON/文件扫描。

---

## Summary

本次按 feature + bugfix 处理：修正 TUI 日期展示，并新增可交互 threads 窗口。参考 `cc-switch` 的会话发现方式，标题优先级固定为：custom title > AI 总结标题字段或尾部标题事件 > 首条真实用户消息 > cwd basename > short id。用户最开始给出的 `this-week` 日期范围仅作为展示问题示例，本次不按该例子改 period 语义。

## Key Changes

- TUI toolbar 去掉日历 emoji；`today` 只显示本地日期和时区，非 `today` 显示真实 `Window.Start ～ Window.End` 和时区。
- 扩展 `usage.UsageEvent` 会话元信息：`thread_id`、`thread_name`、`thread_source`、`thread_created_at`、`thread_last_active_at`。
- Codex/Claude 用 head/tail 少量读取提取 session 元信息；跳过 AGENTS/environment 注入、slash command caveat、subagent/agent 会话。
- Gemini 增加会话文件扫描能力，优先从 `~/.gemini/tmp/<project>/chats/session-*.json` 与 `.project_root` 读取 `sessionId/startTime/lastUpdated/messages`。
- `summary --threads` 输出 `threads` 数组，继承 `period/tool/model/provider/cwd/pricing/format`；默认 summary 不带 threads。
- TUI 新增 Threads BOX：固定表头、当前行高亮、右侧滚动条；不做翻页，不做展开/收起。
- 支持 `t` 聚焦/退出 threads 窗口，`↑/↓` 或 `j/k` 移动，`home/end` 跳转首尾，`c` 快速复制选中 thread ID。

## Implementation Changes

- 新增或扩展 `internal/query.ThreadResult` 和线程聚合器，字段包含 `id/name/model/provider/tool/usage/requests/events/cost_usd/source/created_at/last_active_at`。
- `internal/sources` 增加会话元信息提取辅助函数，保持 JSONL 逐行读取；Codex/Claude 不读取全文，只读取 head/tail；Gemini session JSON 可按文件读取，但不上传、不联网。
- `report.Payload` 新增 `period` 和可选 `threads,omitempty`；table/markdown 在 `--threads` 时追加 Threads 表。
- `internal/tui` 增加 `focusedPane`、`threadCursor`、`threadOffset`、`copyStatus` 状态，并按 BOX 高度裁剪 threads 行；实现焦点驱动滚动条。
- 更新 `README.md` / `README.zh-CN.md`，补充 `summary --threads --format json --no-version-check` 和 TUI threads 快捷键。

## Test Plan

- `go test ./internal/sources ./internal/query ./internal/report ./internal/tui ./internal/cli`
- `make check`
- `make test`
- `make build`
- Source tests：custom title 优先于 AI 总结标题；AI 总结标题优先于首条用户消息；首条用户消息优先于 cwd basename；无可用标题时回退 short id。
- Query/report tests：同一 thread 多事件聚合 token、requests、events、cost；过滤后 threads 与 summary 窗口一致；未传 `--threads` 时 JSON 不含 `threads`。
- TUI tests：日期无 emoji；today 不显示范围；非 today 显示真实窗口；threads box 渲染表头、高亮行、滚动条；`j/k/home/end/c` 更新状态正确；无 `pgup/pgdn/enter` 交互。

## Assumptions

- “AI 总结标题”实现为读取日志中的显式标题/总结字段或尾部标题事件，不调用模型生成标题，不联网。
- `this-week` 不做特殊语义调整；展示始终以实际查询窗口为准。
- Threads 交互优先在 TUI 交付，CLI 的 `summary --threads` 作为机器可解析入口。
- “快速复制 ID”默认复制 thread/session ID，不复制 resume 命令；后续可再加 `r` 复制 `codex resume <id>` / `claude --resume <id>`。
