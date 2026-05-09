# 2026-05-09 Agent 成本治理迭代记忆

这份记录用于在 PR #11 和 `v0.1.15` 发布后，为后续 AI 编码代理恢复关键项目上下文。

## 结果

- PR：`https://github.com/MagnumGoYB/aitok/pull/11`
- Release：`https://github.com/MagnumGoYB/aitok/releases/tag/v0.1.15`
- 使用过的功能分支：`codex/aitok-local-cost-governance`
- 发布提交：`918137e75c9412fe831eeef60de82a23427b870e`
- 发布标签：`v0.1.15`

## 产品决策

- `aitok` 仍然是面向人类的 CLI，但 AI Agent 和自动化调用可靠性是一级产品优先级。
- 主要 Agent 调用契约是 `--format json --no-version-check`。
- JSON stdout 必须保持完整、可机器解析。
- 人类可读 warning、版本提示、预算失败解释应进入 stderr 或错误路径。
- `budget check` 在预算超限时可以返回非零退出码，但仍必须在 stdout 输出结构化 JSON。
- 离线优先是硬约束：不上传用量数据、不做远程同步、不自动联网同步价格。

## 已交付范围

- 增加流式 source 扫描：`internal/sources.Scan` 和 `sources.ForEach`。
- 增加 `internal/query.Accumulator`，支持更低内存的聚合路径。
- 增加 `aitok pricing audit`。
- 增加 `aitok budget check --limit-usd`。
- 增强 `aitok doctor --format table|json|markdown`，作为 onboarding/audit 入口。
- 增加治理报告代码：`internal/report/governance.go`。
- 增加查询 benchmark：`internal/query/query_bench_test.go`。
- 更新 README、AGENTS 和计划文档。

## 价格覆盖修复

当时观察到的报告是：

- `known 20231`
- `unknown 30`
- `unknown_models 1`

根因是默认价格表缺少 `codex-auto-review / bcb` 这个 model/provider 组合。

修复方式是增加该 model/provider 的默认价格覆盖，并把误导性的 `unknown_*` 语言改成更清晰的 `unpriced_*` / `priced_events`。后续价格相关工作要继续区分“可解析事件”和“未定价事件”。

## 工具和缓存决策

早期验证使用 `/tmp` 或 `/private/tmp` 存 Go cache，只是因为沙箱拒绝写入 Go 默认的 `~/Library/Caches`。

长期项目内约定是：

- 验证缓存放在 `.cache/aitok`
- 优先使用 Makefile 目标，不使用临时拼接命令作为项目工具
- 提交信息校验使用 `make commitlint COMMIT_MSG_FILE=<commit-msg-file>`

类似 `/tmp/aitok-commit-msg` 的文件不是项目工具，只是一次性命令输入。

## 验证证据

功能 PR 本地跑过：

- `go test ./internal/cli`
- `go test ./internal/pricing ./internal/report ./internal/cli`
- `go test ./internal/query -bench . -benchmem`
- `make check`
- `make test`
- `make build`

发布验证：

- 打 tag 前本地 `make validate` 通过。
- GitHub Actions release run 成功：`https://github.com/MagnumGoYB/aitok/actions/runs/25598864490`
- GoReleaser 已上传 darwin、linux、windows 产物和 `checksums.txt`。

## GitHub 自动化注意事项

本仓库里 `gh pr edit` 可能因为 GitHub Projects classic GraphQL 字段废弃而失败。如果 PR body 编辑遇到这个问题，改用 REST API：

```sh
jq -Rs '{body:.}' <body.md> > <body.json>
gh api repos/MagnumGoYB/aitok/pulls/<number> -X PATCH --input <body.json>
```

PR metadata 校验需要明确覆盖需求分类、验收标准证据、单元测试、失败/边界覆盖、跳过验证说明和残余风险。

## 后续建议

- 增加 trend/compare：today vs yesterday、this week vs last week、month daily breakdown。
- 改进 project/repo 归一化，把深层 `cwd` 折叠到 git root。
- 继续深化 `doctor`：数据源路径、最近事件时间、Gemini telemetry 安全、价格覆盖率、未定价模型。
- 后续性能工作保持 streaming-first，并用 benchmark 验证。
