# Security Policy

[English](SECURITY.md)

## 支持版本

安全修复面向最新发布版本和 `main` 分支。

## 漏洞报告

在维护者有合理时间响应前，请不要公开披露漏洞。

如 GitHub 支持，请创建 private security advisory；否则通过仓库 issue tracker 联系维护者，并提供最小化、脱敏的复现信息。

## 数据和隐私边界

`aitok` 默认离线。安全相关变更必须保持以下规则：

- 默认不上传本地日志、prompt、API Key 或 Token 用量数据。
- 不读取、打印、哈希、指纹化或持久化真实 API Key。
- 不添加隐藏 telemetry 或后台网络行为。
- Gemini setup 必须保持 `logPrompts=false`。
- 优先使用本地 fixture 测试，不使用真实用户日志样本。

## 披露信息要求

报告应包含受影响版本或 commit、操作系统、复现步骤、预期行为、实际行为和影响范围。请移除真实 API Key、prompt、私有路径和个人数据。
