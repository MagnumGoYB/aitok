package main

import (
	"strings"
	"testing"
)

func TestLintCommitMessageAcceptsEmojiConventionalHeader(t *testing.T) {
	for _, message := range []string{
		"🔧 chore(harness): add commit workflow",
		"✨ feat(cli): add summary flag\n\nBody text.",
		"🐛 fix(query): preserve aggregation result",
		"⚡️ perf(query): reduce allocations",
		"✅ test(query): guard default grouping",
		"📝 docs: update readme",
		"♻️ refactor(sources): simplify parser",
		"👷 ci(github): validate every commit",
		"💄 style(tui): align spacing",
		"🔖 release: v0.1.33",
		"🏗️ build(deps): update build metadata",
	} {
		if problems := lintCommitMessage(message); len(problems) > 0 {
			t.Fatalf("expected %q to pass, got %v", message, problems)
		}
	}
}

func TestLintCommitMessageRejectsMalformedHeaders(t *testing.T) {
	barePerfEmoji := "⚡"
	bareRefactorEmoji := "♻"
	bareBuildEmoji := "🏗"
	cases := map[string]string{
		"chore(harness): missing emoji":                   "header must start with an emoji",
		"🚧 unknown(harness): bad type":                    "type must be one of:",
		"✨ test(query): wrong emoji":                      "emoji must match type \"test\"",
		barePerfEmoji + " perf(query): missing selector":  "emoji must match type \"perf\"",
		bareRefactorEmoji + " refactor(query): selector":  "emoji must match type \"refactor\"",
		bareBuildEmoji + " build(deps): missing selector": "emoji must match type \"build\"",
		"🚧 chore(harness): wrong emoji":                   "emoji must match type \"chore\"",
		"🚧 chore(other): bad scope":                       "scope must be one of:",
		"🚧 chore(harness) missing colon":                  "header must match",
		"# comment only\n\n":                              "commit message header is empty",
		"🚧 chore(harness): " + strings.Repeat("x", 80):    "header must be 64 characters or fewer",
	}
	for message, expected := range cases {
		problems := lintCommitMessage(message)
		if len(problems) == 0 {
			t.Fatalf("expected %q to fail", message)
		}
		if !strings.Contains(strings.Join(problems, "\n"), expected) {
			t.Fatalf("expected %q to contain %q, got %v", message, expected, problems)
		}
	}
}
