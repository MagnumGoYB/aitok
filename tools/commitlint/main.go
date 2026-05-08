package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	maxHeaderLength = 64
	formatHelp      = `emoji + " " + type + optional scope + ": " + subject`
)

var (
	headerPattern        = regexp.MustCompile(`^(\S+)\s+([a-z]+)(?:\(([^)]+)\))?:\s+(.+)$`)
	missingEmojiPattern  = regexp.MustCompile(`^[a-z]+(?:\([^)]+\))?:\s+.+$`)
	allowedTypeValues    = []string{"feat", "fix", "docs", "ci", "style", "refactor", "release", "perf", "test", "chore"}
	allowedScopeValues   = []string{"cli", "sources", "query", "report", "setup", "tui", "usage", "harness", "docs", "github", "config", "deps", "build", "tests", "release"}
	allowedTypes         = set(allowedTypeValues...)
	allowedScopes        = set(allowedScopeValues...)
	allowedScopeSentinel = strings.Join([]string{
		"cli",
		"sources",
		"query",
		"report",
		"setup",
		"tui",
		"usage",
		"harness",
		"docs",
		"github",
		"config",
		"deps",
		"build",
		"tests",
		"release",
	}, ", ")
)

func main() {
	editPath := flag.String("edit", "", "path to a commit message file")
	flag.Parse()

	if *editPath == "" {
		fmt.Fprintln(os.Stderr, "missing --edit <commit-msg-file>")
		os.Exit(2)
	}

	data, err := os.ReadFile(*editPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if problems := lintCommitMessage(string(data)); len(problems) > 0 {
		fmt.Fprintln(os.Stderr, strings.Join(problems, "\n"))
		os.Exit(1)
	}
}

func lintCommitMessage(message string) []string {
	header := firstNonCommentLine(message)
	if header == "" {
		return []string{"commit message header is empty"}
	}

	var problems []string
	if utf8.RuneCountInString(header) > maxHeaderLength {
		problems = append(problems, fmt.Sprintf("header must be %d characters or fewer", maxHeaderLength))
	}

	parts := headerPattern.FindStringSubmatch(header)
	if len(parts) != 5 {
		if missingEmojiPattern.MatchString(header) {
			return append(problems, "header must start with an emoji")
		}
		return append(problems, "header must match "+formatHelp)
	}

	emoji, commitType, scope, subject := parts[1], parts[2], parts[3], strings.TrimSpace(parts[4])
	if !looksLikeEmoji(emoji) {
		problems = append(problems, "header must start with an emoji")
	}
	if !allowedTypes[commitType] {
		problems = append(problems, "type must be one of: "+strings.Join(allowedTypeValues, ", "))
	}
	if scope != "" && !allowedScopes[scope] {
		problems = append(problems, "scope must be one of: "+allowedScopeSentinel)
	}
	if subject == "" {
		problems = append(problems, "subject must not be empty")
	}
	return problems
}

func firstNonCommentLine(message string) string {
	for _, line := range strings.Split(message, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return line
	}
	return ""
}

func looksLikeEmoji(value string) bool {
	r, _ := utf8.DecodeRuneInString(value)
	return r >= 0x1F000 || strings.ContainsRune(value, '✨') || strings.ContainsRune(value, '♻')
}

func set(values ...string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}
