package main

import (
	"strings"
	"testing"
)

func TestLintCommitMessageAcceptsEmojiConventionalHeader(t *testing.T) {
	for _, message := range []string{
		"🚧 chore(harness): add commit workflow",
		"✨ feat(cli): add summary flag\n\nBody text.",
		"📝 docs: update readme",
		"♻️ refactor(sources): simplify parser",
	} {
		if problems := lintCommitMessage(message); len(problems) > 0 {
			t.Fatalf("expected %q to pass, got %v", message, problems)
		}
	}
}

func TestLintCommitMessageRejectsMalformedHeaders(t *testing.T) {
	cases := map[string]string{
		"chore(harness): missing emoji":                "header must start with an emoji",
		"🚧 unknown(harness): bad type":                 "type must be one of:",
		"🚧 chore(other): bad scope":                    "scope must be one of:",
		"🚧 chore(harness) missing colon":               "header must match",
		"# comment only\n\n":                           "commit message header is empty",
		"🚧 chore(harness): " + strings.Repeat("x", 80): "header must be 64 characters or fewer",
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
