# aitok

[English](README.md)

![aitok 封面图](README.jpg)

`aitok` 是一个轻量、离线优先的 CLI，用于统计本机 Claude Code、Codex 和 Gemini CLI 的 Token 用量。

它不会上传数据或读取 API Key。用量和 USD 成本统计都来自本机工具日志。

## 安装

Homebrew：

```bash
brew tap MagnumGoYB/aitok
brew install --cask aitok
```

先执行 tap 可以保持安装命令简洁，避免使用较不规范的完整 cask 名称。

Go：

```bash
go install github.com/MagnumGoYB/aitok/cmd/aitok@latest
```

本地开发：

```bash
go install ./cmd/aitok
```

`aitok` 会在命令执行前最多每 24 小时检查一次 GitHub release 元数据。如果发现新版本，会根据检测到的安装方式把升级提示输出到 stderr。该检查不会上传用量数据，不会读取日志，也可以通过 `--no-version-check` 或 `AITOK_NO_VERSION_CHECK=1` 跳过。

## 使用

```bash
aitok summary --period today
aitok summary --period this-week --group-by tool,model,provider --format markdown
aitok report --period last-week --format json
aitok tui
aitok tui --lang zh-CN
aitok doctor
aitok setup gemini --dry-run
```

TUI 默认使用英文文案。传入 `--lang zh-CN` 可默认显示中文，也可以在 TUI 中按 `l` 切换语言。

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

报告会返回请求数量、Token 总量、缓存 Token 和预估 USD 成本。成本默认使用基于官方公开价格快照的离线 model 价格表，也支持本地覆盖：

```json
{
  "models": [
    {
      "match": "gpt-5.4",
      "input_usd_per_mtok": 1.25,
      "output_usd_per_mtok": 10,
      "cache_hit_usd_per_mtok": 0.125,
      "cache_make_usd_per_mtok": 1.25,
      "multiplier": 1
    }
  ]
}
```

保存为 `~/.aitok/pricing.json`，或通过参数显式指定：

```bash
aitok summary --pricing ./pricing.json --format json
```

价格单位是 USD / 1M tokens。Reasoning tokens 按 output tokens 计费。`multiplier` 默认是 `1`。

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
