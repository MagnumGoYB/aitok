# aitok

[中文](README.zh-CN.md)

`aitok` is a lightweight offline CLI for summarizing local token usage from Claude Code, Codex, and Gemini CLI.

It does not upload data, read API keys, or estimate cost. All summaries are built from local tool logs.

## Install

Homebrew:

```bash
brew tap MagnumGoYB/aitok
brew install --cask aitok
```

Go:

```bash
go install github.com/MagnumGoYB/aitok/cmd/aitok@latest
```

For local development:

```bash
go install ./cmd/aitok
```

## Usage

```bash
aitok summary --period today
aitok summary --period this-week --group-by tool,model,provider --format markdown
aitok report --period last-week --format json
aitok tui
aitok doctor
aitok setup gemini --dry-run
```

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
