# Contributing

[中文](CONTRIBUTING.zh-CN.md)

## Required Checks

Run before submitting changes:

```bash
make validate
```

Or step-by-step:

```bash
make generate  # regenerate buildinfo.go from VERSION
make check
make test
make test-harness
make build
```

## Vibe Coding Constraints

- Every behavior change needs tests or fixture coverage.
- Parser changes must include malformed input cases.
- Source adapters must avoid prompt/body inspection unless the token metadata requires it.
- Keep the tool offline by default.
- Do not add dependencies that run background services or phone home.

- Update `docs/harness-engineering.md` and `docs/zh-CN/harness-engineering.md` when changing harness, CI, PR workflow, or validation scripts.
