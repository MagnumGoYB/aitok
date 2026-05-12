# CLAUDE

[English](CLAUDE.md)

@AGENTS.zh-CN.md

# Claude Code 入口

将 `AGENTS.zh-CN.md` 作为本仓库的主要项目规范来源。

补充的 Claude Code 约定：
- 除非用户明确要求其他语言，面向用户的进度更新默认使用 zh-CN。
- 继续处理 pricing governance、budget、doctor、release 或 GitHub automation 相关工作前，先阅读 `docs/zh-CN/iteration-notes/2026-05-09-agent-cost-governance.md`。
- 继续处理 TUI period display、本地 thread/session 列表或 source title extraction 相关工作前，先阅读 `docs/zh-CN/iteration-notes/2026-05-11-tui-period-threads.md`。
- 常用本地验证路径：`make check`、`go test ./...`、`go build ./cmd/aitok`。
- 当改动超过小范围局部修改时，交付前运行 `make validate`。
- 保持自动化调用契约稳定：脚本或 Agent 调用优先使用 `--format json` 和 `--no-version-check`。
