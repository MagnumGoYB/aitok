# REASONIX.zh-CN.md — aitok

每次启动自动注入系统提示的项目工作知识。

## 技术栈

- **Go 1.26.3** — 单二进制 CLI，无外部 API 运行时依赖
- **`github.com/spf13/cobra`** — CLI 框架（子命令、标志、帮助）
- **`github.com/charmbracelet/bubbletea`** + **`lipgloss`** — TUI 仪表盘
- **模块路径:** `github.com/MagnumGoYB/aitok`

## 目录布局

| 路径 | 用途 |
|------|------|
| `cmd/aitok/main.go` | 入口 — 组装 `cli.App` 与 `updatecheck`，调用 `cmd.Execute()` |
| `internal/cli/cli.go` | 所有子命令注册：`summary`、`report`、`pricing configure/audit`、`budget check`、`tui`、`doctor`、`version`、`update`、`setup gemini` |
| `internal/usage/usage.go` | 核心类型：`Tool`（claude/codex/gemini/reasonix）、`TokenUsage`、`UsageEvent`、`ProviderAttribution` |
| `internal/sources/source.go` | `Source` 接口（`Name()`、`Read()`、`Scan()`）+ `Options` |
| `internal/sources/claude.go` | 读取 `~/.claude/projects/` — 遍历每个项目的 JSON 日志 |
| `internal/sources/codex.go` | 读取 `~/.codex/projects/` 的会话文件 + 提供者时间线缓存 |
| `internal/sources/gemini.go` | 读取 Gemini 遥测日志（`~/.gemini/telemetry.log`）+ 会话元信息 |
| `internal/sources/reasonix.go` | 读取 Reasonix 用量日志（`~/.reasonix/usage.jsonl`）+ 会话元信息 |
| `internal/sources/collect.go` | `ForEachConcurrent()` — 每个 Source 一个 goroutine，全部收集完毕再串行回调 |
| `internal/sources/jsonlines.go` | Claude / Gemini 日志共用的 NDJSON 行读取器 |
| `internal/sources/session_meta.go` | 线程 ID ↔ 名称映射（Claude 与 Gemini） |
| `internal/sources/provider_timeline_cache.go` | Codex 的提供者归属时间线缓存 |
| `internal/query/period.go` | `Period`（today/yesterday/this-week/last-week/this-month）+ `Window` + `WindowFor()` |
| `internal/query/query.go` | `Accumulator` / `ThreadAccumulator` — 按 groupBy 分桶聚合引擎，含排序与过滤 |
| `internal/query/query.go` | `Result`、`ThreadResult`、`ThreadTurn`、`Cost`、`Price`、`rebalanceMixedProviderThread` |
| `internal/pricing/config.go` | 用户价格覆盖配置文件加载（`~/.aitok/pricing.json`）+ `SaveUserPrice` |
| `internal/pricing/pricing.go` | `Catalog` 内含约 20 个内置模型价格 + 子串匹配 + `CostFor(event)` |
| `internal/report/report.go` | `Write()` 支持 table/json/markdown 三种格式 — `WriteTable`、`WriteThreadsTable`、`WriteMarkdown` |
| `internal/report/governance.go` | `WritePricingAudit()`、`WriteBudget()`、`WriteDoctor()` — 治理子命令的输出 |
| `internal/tui/model.go` | Bubble Tea 模型：面板（工具过滤、结果、线程）、搜索、排序、复制、恢复线程 |
| `internal/tui/view.go` | TUI 视图渲染 — 滚动、布局、聚焦面板高亮 |
| `internal/tui/widgets_*.go` | 卡片、用量、线程列表等小组件 |
| `internal/tui/styles.go` | Lipgloss 样式定义与配色方案 |
| `internal/tui/actions.go` | 剪贴板复制、键盘映射、语言切换 |
| `internal/tui/layout.go` | 面板布局 / 自动调整 |
| `internal/tui/filters.go` | 按工具过滤 + 搜索过滤 |
| `internal/tui/copy.go` | TUI i18n 字符串（en/zh-CN） |
| `internal/tui/format.go` | TUI 专用格式化辅助函数 |
| `internal/buildinfo/buildinfo.go` | 硬编码版本号 `Version = "0.2.0"` |
| `internal/updatecheck/updatecheck.go` | GitHub Release 版本检查 + homebrew/go 自动升级 |
| `internal/setup/gemini.go` | 配置 Gemini `settings.json`，启用本地遥测日志输出 |
| `tools/commitlint/` | 提交信息校验器 — 强制 `<emoji> <type>(<scope>): <subject>` 格式 |
| `tools/pricing-watch/` | GitHub Action — 监控上游价格 JSON 是否有更新 |
| `tools/validate-pr-body/` | PR 描述校验器（CI 关卡） |
| `tools/version/` | 同时更新 `VERSION` 和 `buildinfo.go` 中的版本号 |
| `harness/architecture_test.go` | 架构集成测试（包布局、导入规则） |

## 常用命令

| `make` 目标 | 作用 |
|-------------|------|
| `check` | `gofmt` + `go vet` |
| `test` | `go test ./...` |
| `build` | `go build ./cmd/aitok` |
| `run ARGS="..."` | `go run ./cmd/aitok -- $(ARGS)` |
| `validate` | `check` → `test` → `build`（完整的 PR 前验证） |
| `test-packages PKGS="..."` | 测试指定包 |
| `test-harness` | 测试 `./harness` |
| `commitlint` | 通过 `tools/commitlint` 校验暂存的提交信息 |
| `commitlint-range COMMIT_RANGE="..."` | 校验一段提交范围 |
| `validate-pr-body` | 校验 PR 描述 |
| `setup` | 安装 `.githooks/` 为 `core.hooksPath` |

## CLI 子命令

| `aitok <sub>` | 说明 |
|---------------|------|
| `summary` | 打印 token 用量摘要（table/json/markdown），支持 `--period`、`--group-by`、`--sort`、`--threads`、`--full` |
| `report` | 与 summary 相同，但无 `--threads` 默认 — 适合管道使用 |
| `pricing configure` | 交互式/标志驱动的定价覆盖配置，写入 `~/.aitok/pricing.json` |
| `pricing audit` | 列出无匹配价格的事件，同时输出可填充的 JSON 骨架 |
| `budget check --limit-usd N` | 若估算费用超出 `--limit-usd` 则退出码 1 |
| `tui` | 交互终端仪表盘（AltScreen），每 5 秒自动刷新 |
| `tui --render` | 单次渲染仪表盘到 stdout，不进入交互模式 |
| `doctor` | 检查本地数据源状态（Claude/Codex/Gemini 文件是否存在？）、定价覆盖率 |
| `version` | 打印当前版本 |
| `update` | 检查 GitHub Releases 并自动升级 |
| `setup gemini` | 启用 Gemini CLI 本地遥测（写入 `~/.gemini/settings.json`） |

查询子命令通用标志：`--period`、`--format`（table/json/markdown）、`--group-by`、`--sort`、`--tool`、`--model`、`--provider`、`--cwd`、`--home`、`--pricing`、`--no-version-check`。

## 数据流

1. `sources.Source.Scan()` → 发射 `usage.UsageEvent`（每个 API 调用/日志事件一条）
2. **四个 Source 实现**从不同路径读取本地日志，统一产出 `UsageEvent` 类型
3. `query.Accumulator.Add(event)` — 按 `groupBy` 键（tool/model/provider/day/cwd）分桶聚合，支持 `Filters`
4. `query.ThreadAccumulator` — 同上但按线程分组，带 turn 级明细，支持 `rebalanceMixedProviderThread`
5. `report.Write(w, format, payload)` — 将聚合结果输出为 table/JSON/markdown
6. 输出结构：`Payload{GeneratedAt, Window, GroupBy, SortBy, Results, Threads}`

## 工程约定

- **提交信息格式:** `<emoji> <type>(<scope>): <subject>`（最多 64 字符）。类型: `feat|fix|docs|ci|style|refactor|release|perf|test|chore|build`。范围: `cli|sources|query|report|setup|tui|usage|harness|docs|github|config|deps|build|tests|release`。`tools/commitlint/` 强制执行。
- **测试文件紧贴源码** — `*_test.go` 与源文件同目录同包。
- **`io.Writer` 注入** — 所有输出通过注入的 writer（非 `os.Stdout`），便于测试捕获。
- **`pricing.Catalog.match()`** — 模型名字串包含匹配，不区分大小写，长匹配模式优先。
- **`ForEachConcurrent`** — 先按 Source 各自收集到内存切片，再串行回调 — 保证顺序。
- **TUI** — bubbletea alt-screen，5 秒自动刷新，支持 `/` 搜索、`c` 复制线程 ID、`Enter` 恢复线程。
- **用户面向的进展说明** — 默认为 zh-CN（参考 `AGENTS.md`）。

## 注意事项

- **`buildinfo.Version`** 由 `tools/buildinfo-gen` 从根目录 `VERSION` 文件自动生成（通过 `go generate ./internal/buildinfo/...`）。执行 `make validate` 即可重新生成；禁止手动编辑 `internal/buildinfo/buildinfo.go`。
- **价格目录**（`internal/pricing/pricing.go:DefaultCatalog()`）内置约 20 个硬编码模型价格。上游价格变动需代码更新 + 发布 — `tools/pricing-watch` 负责监控。
- **缓存目录** — `.cache/` 存放 Go 构建缓存。可用 `AITOK_CACHE_DIR` 环境变量控制。
- **用户定价覆盖**在 `~/.aitok/pricing.json` — `Catalog.upsert()` 将用户条目插入 `DefaultCatalog()` 之前以获得优先级。
- **Codex 提供者重平衡** — `query.go` 的 `rebalanceMixedProviderThread()` 包含复杂启发式逻辑，用于将推断时间线的 turn 跨提供者拆分。修改需充分测试。
- **TUI** 在没有真实终端时会异常退出 — `--render` 标志可将单次输出打到 stdout 作为逃生出口。
- **`tools/`** 是独立的 `package main` — 每个工具都有自己的 `main_test.go`。它们不是主二进制的一部分。
- **根目录 `VERSION`** — 项目版本的唯一来源。`tools/buildinfo-gen` 从中读取生成 `internal/buildinfo/buildinfo.go`。版本升级只需更新 `VERSION`。
- **`.goreleaser.yml`** — homebrew tap + GitHub Release 自动化；对 tag 命名敏感。
