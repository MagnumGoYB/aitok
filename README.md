# aitok

[中文](README.zh-CN.md)

![aitok cover](README.en.jpg)

`aitok` is a lightweight offline CLI for summarizing local token usage from Claude Code, Codex, and Gemini CLI.

It does not upload data or read API keys. Usage and USD cost summaries are built from local tool logs.

## Install

Homebrew:

```bash
brew tap MagnumGoYB/aitok
brew install --cask aitok
```

The tap step keeps the install command short and avoids the less readable fully qualified cask name.

Go:

```bash
go install github.com/MagnumGoYB/aitok/cmd/aitok@latest
```

For local development:

```bash
go install ./cmd/aitok
```

`aitok` checks GitHub release metadata at most once every 24 hours before a command runs. If a newer version exists, it prints an upgrade prompt to stderr based on the detected install method. The check does not upload usage data, does not read logs, and can be skipped with `--no-version-check` or `AITOK_NO_VERSION_CHECK=1`.

## Usage

```bash
aitok summary --period today
aitok summary --period this-week --group-by tool,model,provider --format markdown
aitok report --period last-week --format json
aitok tui
aitok tui --lang zh-CN
aitok doctor
aitok setup gemini --dry-run
```

The TUI uses English by default. Pass `--lang zh-CN` to start in Chinese, or press `l` inside the TUI to switch languages.

Periods:

- `today`
- `yesterday`
- `this-week`
- `last-week`
- `this-month`

Filters:

- `--tool claude|codex|gemini`
- `--model <name>`
- `--provider <provider-or-auth-type>`
- `--cwd <path-fragment>`

Grouping:

```bash
--group-by tool,model,provider,day,cwd
```

Reports include request count, token totals, cache tokens, and estimated USD cost. Cost uses an offline default model price catalog based on an official public pricing snapshot and can be overridden locally:

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

Save this as `~/.aitok/pricing.json`, or pass a file explicitly:

```bash
aitok summary --pricing ./pricing.json --format json
```

Prices are USD per 1M tokens. Reasoning tokens are charged as output tokens. `multiplier` defaults to `1`.

## Data Sources

- Claude Code: `~/.claude/projects/**/*.jsonl`
- Codex: `~/.codex/sessions/**/*.jsonl`
- Gemini CLI: local telemetry outfile configured in `~/.gemini/settings.json`

Gemini CLI telemetry is disabled by default. Run:

```bash
aitok setup gemini
```

This configures local telemetry output and sets `logPrompts=false` so prompts are not recorded in telemetry.

## Development

```bash
make check
make test
make test-harness
make vet
make build
make validate
make validate-pr-body
```

Harness and AI agent constraints live in `AGENTS.md`, `AGENTS.zh-CN.md`, and `docs/harness-engineering.md`.

## Open Source Flow

- Contributing guide: `CONTRIBUTING.md` / `CONTRIBUTING.zh-CN.md`
- Security policy: `SECURITY.md` / `SECURITY.zh-CN.md`
- GitHub automation: `docs/github-automation.md` / `docs/zh-CN/github-automation.md`
- Code of Conduct: `CODE_OF_CONDUCT.md` / `CODE_OF_CONDUCT.zh-CN.md`
- Support: `SUPPORT.md` / `SUPPORT.zh-CN.md`
- License: MIT
