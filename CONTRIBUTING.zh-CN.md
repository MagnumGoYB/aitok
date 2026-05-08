# Contributing

[English](CONTRIBUTING.md)

感谢参与 `aitok`。本项目是离线优先的开源 Go CLI，贡献必须保护本机数据、隐私和可脚本化输出。

## 必跑检查

提交前运行：

```bash
make check
make test
make test-harness
make build
```

PR 元数据或 GitHub workflow 变化时，还要运行：

```bash
make validate-pr-body
```

## Vibe Coding 约束

- 每个行为变更都需要测试或 fixture 覆盖。
- Parser 变更必须包含异常输入用例。
- Source adapter 不读取 prompt/body，除非 Token 元数据解析确实需要。
- 保持默认离线。
- 不添加会启动后台服务或主动联网的依赖。
- 修改 Harness、CI、PR workflow 或验证脚本时，同步更新 `docs/harness-engineering.md` 和 `docs/zh-CN/harness-engineering.md`。

## PR 要求

PR body 必须填写模板中的需求分类、验收标准、测试证据、验证结果、回滚和残余风险。`make validate-pr-body` 会校验这些内容。
