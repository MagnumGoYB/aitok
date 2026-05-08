# aitok

[English](README.md)

`aitok` 是一个轻量、离线优先的 CLI，用于统计本机 Claude Code、Codex 和 Gemini CLI 的 Token 用量。

它不会上传数据、读取 API Key 或估算费用。所有统计都来自本机工具日志。

## 安装

```bash
go install github.com/sosbs/aitok/cmd/aitok@latest
```

本地开发：

```bash
go install ./cmd/aitok
```

## 使用

```bash
aitok summary --period today
aitok summary --period this-week --group-by tool,model,provider --format markdown
aitok report --period last-week --format json
aitok tui
aitok doctor
aitok setup gemini --dry-run
```

时间范围：

- `today`
- `yesterday`
- `this-week`
- `last-week`
- `this-month`

过滤条件：

- `--tool claude|codex|gemini`
- `--model <name>`
- `--provider <provider-or-auth-type>`
- `--cwd <path-fragment>`

分组：

```bash
--group-by tool,model,provider,day,cwd
```

## 数据源

- Claude Code：`~/.claude/projects/**/*.jsonl`
- Codex：`~/.codex/sessions/**/*.jsonl`
- Gemini CLI：`~/.gemini/settings.json` 中配置的本地 telemetry outfile

Gemini CLI 默认关闭 telemetry。运行：

```bash
aitok setup gemini
```

该命令会配置本地 telemetry 输出，并设置 `logPrompts=false`，避免在 telemetry 中记录 prompt。

## 开发

```bash
make check
make test
make test-harness
make vet
make build
make validate
make validate-pr-body
```

Harness 和 AI agent 约束见 `AGENTS.md`、`AGENTS.zh-CN.md`、`docs/harness-engineering.md` 和 `docs/zh-CN/harness-engineering.md`。

## 开源流程

- 贡献指南：`CONTRIBUTING.md` / `CONTRIBUTING.zh-CN.md`
- 安全策略：`SECURITY.md` / `SECURITY.zh-CN.md`
- GitHub 自动化：`docs/github-automation.md` / `docs/zh-CN/github-automation.md`
- 行为准则：`CODE_OF_CONDUCT.md` / `CODE_OF_CONDUCT.zh-CN.md`
- 支持说明：`SUPPORT.md` / `SUPPORT.zh-CN.md`
- License：MIT
