# aitok 本地成本治理一期实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建性能优先、离线优先的本地成本治理闭环，覆盖流式聚合、价格覆盖率审计、预算检查和 doctor 深化。

**Architecture:** 查询路径从“先收集全量事件再聚合”改为“source 逐行扫描并回调 accumulator”。新增命令复用同一套过滤、窗口、价格估算和报告格式，不引入网络同步、后台服务或持久预算配置。虽然 `aitok` 是命令行工具，但第一优先级是为 AI Agent 和自动化脚本提供稳定、可解析、可预期的调用契约。

**Tech Stack:** Go 1.26.3+、Cobra CLI、标准库 JSON/文件扫描、现有 `internal/sources` / `internal/query` / `internal/pricing` / `internal/report` 边界。

---

## Summary

第一期聚焦“性能优先、离线优先”的成本治理闭环：流式读取/聚合、价格覆盖率审计、预算检查、doctor 深化。暂不实现趋势对比和 git root 项目归一化，避免第一期范围过大。

## Key Changes

- 新增流式事件处理路径：在 `internal/sources` 增加逐事件扫描接口，Claude/Codex/Gemini 适配器边读 JSONL 边回调事件；保留现有 `Read()` 兼容测试和旧调用。
- 在 `internal/query` 增加流式 accumulator，`summary/report/tui` 改为边扫描边聚合，不再为查询路径构建全量 `[]UsageEvent`。
- 固化 AI Agent 调用契约：Agent 默认使用 `--format json --no-version-check`；JSON 命令 stdout 只输出结构化 payload，warning/错误摘要走 stderr 或退出码；预算超限时保留 stdout JSON 并返回非零状态。
- 新增 `aitok pricing audit`：支持现有过滤、时间窗、价格文件和 `table|json|markdown`，输出未匹配价格的 `tool/model/provider`、事件数、token 数、示例 cwd，以及可复制的 `pricing.json` skeleton。
- 新增 `aitok budget check --limit-usd <amount>`：复用现有过滤和分组参数；未超限 exit 0，超限 stdout 输出报告、stderr 输出摘要并 exit 1。
- 增强 `aitok doctor --format table|json|markdown`：输出每个 source 状态、事件数、最近事件时间、Gemini telemetry 安全配置、价格覆盖率和未知模型数量。

## Implementation Plan

- Source/query 基础设施：
  - 为 source 增加 `Scan(ctx, handle)` 能力，并用现有 `readJSONLines` 保持逐行解析。
  - 新增 `sources.ForEach(ctx, sources, handle)`，聚合错误但不因单个 source 失败丢弃其他 source。
  - 新增 `query.Accumulator`，封装 window/filter/group/cost 逻辑，结果排序保持与当前 `AggregateWithCosts` 一致。
- CLI 命令：
  - 把 `buildPayload` 改为流式 accumulator。
  - 增加 `pricing audit` 子命令，内部只扫描一次日志并聚合未覆盖价格项。
  - 增加 `budget check` 子命令，使用同一聚合结果计算总成本和超限状态。
  - 扩展 `doctor`，不要调用全量 `Collect`；改为 source-by-source 流式扫描并收集诊断摘要。
- Report/API：
  - 保持现有 summary/report JSON 字段兼容。
  - 新增独立 report payload：`pricing_audit` 和 `budget_check`，不要混入现有 `report.Payload`，避免破坏现有 JSON 用户。
  - JSON stdout 是 Agent 契约，不得混入 warning、版本提示或人类说明文案。
  - 表格和 Markdown 输出保持脚本可读，不使用交互式 UI。
- Docs:
  - 更新 `README.md` / `README.zh-CN.md`，补充 `pricing audit`、`budget check`、增强 doctor 示例。
  - 若 PR 模板或 harness 规则未变化，不改 `.github/pull_request_template*` 或 harness docs。

## Test Plan

- Unit tests:
  - Source 扫描测试：正常 JSONL、空目录、malformed JSONL、缺字段、重复事件仍按现有语义处理。
  - Query accumulator 测试：与 `AggregateWithCosts` 在同一 fixture 下输出一致。
  - Pricing audit 测试：已覆盖模型不出现，未知模型生成 skeleton，过滤条件生效。
  - Budget check 测试：未超限 exit 0，超限返回错误，`--limit-usd <= 0` 报错，未知价格有 warning。
  - Agent 调用契约测试：`--format json --no-version-check` 成功时 stderr 为空；预算超限时 stdout 保持完整 JSON，错误通过返回值/退出码表达。
  - Doctor 测试：Gemini 未配置、`logPrompts=true`、正常配置三种状态。
- CLI smoke:
  - `go test ./internal/sources ./internal/query ./internal/pricing ./internal/report ./internal/cli`
  - `make check`
  - `make test`
  - `make build`
- Performance evidence:
  - 增加 accumulator benchmark 或大 fixture 测试，记录 `go test ./internal/query -bench . -benchmem`。
  - 验证新查询路径不再调用 `sources.Collect` 构建全量事件切片。

## Assumptions

- 第一期预算只支持命令参数，不引入 `~/.aitok/budget.json`。
- 所有新增能力默认离线运行；不会新增网络同步、云上传、后台服务或 API key 读取。
- 预算检查基于估算成本，不宣称等同 provider 账单。
- 趋势对比和 git root 项目归一化作为第二期计划处理。
