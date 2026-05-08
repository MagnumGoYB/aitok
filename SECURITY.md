# Security Policy

[中文](SECURITY.zh-CN.md)

## Supported Versions

Security fixes target the latest released version and the `main` branch.

## Reporting a Vulnerability

Please do not disclose vulnerabilities publicly before maintainers have had a reasonable chance to respond.

Open a private security advisory on GitHub when available, or contact the maintainer through the repository issue tracker with a minimal, non-sensitive reproduction.

## Data and Privacy Boundaries

`aitok` is offline-first. Security-sensitive changes must preserve these rules:

- Do not upload local logs, prompts, API keys, or token usage data by default.
- Do not read, print, hash, fingerprint, or persist raw API keys.
- Do not add hidden telemetry or background network behavior.
- Keep Gemini setup configured with `logPrompts=false`.
- Prefer local fixture tests over real user log samples.

## Disclosure Expectations

Reports should include affected version or commit, operating system, reproduction steps, expected behavior, actual behavior, and impact. Remove real API keys, prompts, private paths, and personal data from reports.
