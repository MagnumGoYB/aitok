# 2026-05-18 TUI Dashboard 现代化

这是一份随仓库版本化的交接记录。后续 AI 编码代理继续处理 dashboard 布局、Threads/Selected Thread 对齐、Model Usage bar、TUI render hardening，或 `v0.1.38` 之后的发版跟进前，先读这里。

## 快速恢复

- 仓库：`/Users/sosbs/coding/aitok`
- 使用过的分支：`feat/tui-modernize`
- 功能提交：`99e2332`
- 发布标签：`v0.1.38`、`v0.1.39`、`v0.1.40`、`v0.1.41`
- 发版状态：`v0.1.41` 是 dashboard render hardening、整页滚动，以及把 modal 通知替换为右上角 header notice 的跟进发布版本。
- 更早的 TUI 上下文：`docs/zh-CN/iteration-notes/2026-05-11-tui-period-threads.md`

## 这轮迭代为什么发生

Dashboard 已经功能可用，但在真实终端宽度下视觉稳定性不足。用户通过截图连续指出布局问题：右边框溢出、指标标签重复、Model Usage bar 太多、颜色层级不明显、help 位置别扭、Threads 行换行。

长期经验是：截图驱动的 TUI 打磨必须沉淀成 render 约束，而不能只靠人工看终端。

## 已确定决策

- `Usage Dashboard` 前保留顶部呼吸空间；标题不能贴到渲染区域最顶部。
- 移除冗余副标题，把紧凑的 `? help` 放到剩余状态文本附近。
- Toolbar 元信息保持紧凑：tabs、sort、model count、thread count、search 和 date range 组成紧凑头部，不要撑出视口。
- 展示 `Models: N` 和 `Threads: N`，不要展示 `N/N`。
- 当 toolbar 已展示全局排序指标时，section 标题不要重复 metric badge。
- Threads 与 Selected Thread 是锁定宽度的左右结构。两者渲染宽度加间距不能超过 dashboard 宽度。
- Threads 列表行不能换行。列表只保留高信息量列；完整 model/provider/cost 明细放到 Selected Thread。
- Selected Thread 展示 `Model`、`Provider`、`Last Active`、`Tokens`，以及一个有实际信息量的成本行，例如 `Cost: $306.0250 (toska $299.77 / openai $6.25)`。不要恢复单独的 `Split: toska/openai` 行。
- Model Usage bar 使用 Threads 高亮色系。用量越高颜色越深，用量越低颜色越浅。
- Model Usage 最多展示 4 条 chart bar。超出部分用提示折叠，完整查看依赖下方表格。
- 不要重新增加饼图，除非有可靠的终端图表实现，并且明确优于当前 bar/table 组合。
- 当 dashboard 内容超过一个终端屏幕时，page scroll 必须移动整个 dashboard viewport，让用户可以回到折叠线以上的内容。
- 不要再用 modal/dialog overlay 展示复制 thread ID 或 help 这类 TUI 临时通知。终端渲染下这一路线视觉稳定性不足。此类提示应放到右上角 header 区域，不要新增底部通知条。

## 实现注意事项

- 原先单体的 `internal/tui/tui.go` 已拆成 `actions`、`copy`、`filters`、`format`、`layout`、`model`、`view` 和 widgets 文件。后续 TUI 修改应落在拥有对应职责的窄文件里。
- 时间敏感测试必须使用与生产代码一致的 location 转换来生成期望值。不要硬编码开发机本地时区渲染出来的时间。
- `v0.1.39` 已推送但 Release workflow 失败，因为 TUI 测试期望 Asia/Shanghai 时间，而 GitHub runner 渲染 UTC。不要移动已推送 release tag；应补修复并 bump 下一个 patch 版本。
- 截图问题应尽量转换成文本渲染断言：右边框对齐、不换行、可见行数上限、折叠提示、排序标签、help 位置都可以从去 ANSI 后的输出里测试。
- `? help` 入口和复制状态都是 header notice。测试应断言它们留在顶部 header 区域，并且不会遮挡 dashboard 主体。

## 验证证据

- `v0.1.40` 本地验证已通过：`make validate`、`make test-packages PKGS="./internal/tui"`，以及 `GITHUB_REF_NAME=v0.1.40 GITHUB_REF_TYPE=tag go run ./tools/version --check-ref`。
- `v0.1.40` GitHub Actions 已通过：tag `Release` run `26027319067` 与 tag `Build` run `26027319217`。
- `v0.1.40` GitHub Release 已成功发布：`https://github.com/MagnumGoYB/aitok/releases/tag/v0.1.40`。
- `v0.1.41` tag 创建前本地验证已通过：`make validate`、`make test-packages PKGS="./internal/tui ./internal/sources ./internal/query ./internal/report"`、`git diff --check`，以及 `make run ARGS="--no-version-check tui --period today --render"`。

## 后续工作

- 增加轻量 TUI render snapshot/golden harness，覆盖固定终端宽度，让截图驱动布局问题沉淀成稳定回归测试。
- 后续 dashboard polish 保持在聚焦的 widget/layout 文件中，不要回到一个巨大的 TUI 文件。
