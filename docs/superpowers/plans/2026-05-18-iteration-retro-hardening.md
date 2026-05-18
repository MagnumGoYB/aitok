# aitok 最近迭代问题收敛实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将最近几天暴露的 TUI 截图回归、provider/cost 归因验证、release 失败 tag 处理经验沉淀为测试、文档和流程约束。

**Architecture:** 不改 CLI/TUI 用户功能。通过固定宽度 TUI render 断言、provider/mixed cost fixture 覆盖、release 文档规则和 iteration notes 拆分，降低后续重复踩坑概率。

**Tech Stack:** Go 1.26.3+、现有 `testing`、Lip Gloss render 输出、Makefile validation targets、Markdown docs。

---

## Summary

本计划最初按工程质量 / harness / docs 优化处理；后续同一轮叠加了 TUI 用户可见修复，包括整页滚动和右上角 header notice，因此最终需要软件发版。

## Implementation Checklist

- [x] 新增或增强 TUI 固定宽度 render 测试，覆盖 `100/120/160` 宽度下的顶部留白、help 位置、section 右边界、Threads 不换行、Selected Thread 对齐、Model Usage 4 条 bar 折叠提示。
- [x] 保持 provider/cost 回归测试覆盖：provider-qualified model、裸模型 + 唯一 request-host evidence、same-turn evidence、unknown/ambiguous host fallback、mixed price components。
- [x] 新增 `2026-05-18-tui-dashboard-modernization` 中英文 iteration notes；`2026-05-11` 文档只保留早期 TUI threads 线索并链接到新文档。
- [x] 更新 GitHub release 文档：已推送 tag 的 release 失败时，不移动远端 tag；补修复后 bump 下一个 patch 版本重新发版。
- [x] 验证文档和测试：`git diff --check`、`make test-packages PKGS="./internal/tui ./internal/sources ./internal/query ./internal/report"`、`make validate`。

## Acceptance Criteria

- TUI render tests 能在不依赖本地时区硬编码的情况下捕获 dashboard 右边界、换行、折叠和 help 位置问题。
- Provider/cost 测试意图在源码测试或 iteration notes 中明确可追踪，后续真实总数不一致问题要求 live `summary` smoke。
- Release 文档明确禁止移动已推送失败 tag，并说明 fix-forward + patch bump 策略。
- 英文和中文文档保持同步。

## Release Decision

Release required. 本轮最终包含 TUI 用户可见 bugfix/polish，版本 bump 到 `0.1.41`，按 tag release 流程发布。
