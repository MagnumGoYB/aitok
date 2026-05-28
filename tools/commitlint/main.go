package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	maxHeaderLength = 64
	formatHelp      = `type-specific emoji + " " + type + optional scope + ": " + subject`
)

var (
	headerPattern       = regexp.MustCompile(`^(\S+)\s+([a-z]+)(?:\(([^)]+)\))?:\s+(.+)$`)
	missingEmojiPattern = regexp.MustCompile(`^[a-z]+(?:\([^)]+\))?:\s+.+$`)
	allowedTypeValues   = []string{"feat", "fix", "docs", "ci", "style", "refactor", "release", "perf", "test", "chore", "build"}
	allowedScopeValues  = []string{"cli", "sources", "query", "report", "setup", "tui", "usage", "harness", "docs", "github", "config", "deps", "build", "tests", "release", "reasonix"}
	allowedTypes        = set(allowedTypeValues...)
	allowedScopes       = set(allowedScopeValues...)
	typeEmojiValues     = map[string]string{
		"feat":     "✨",
		"fix":      "🐛",
		"docs":     "📝",
		"ci":       "👷",
		"style":    "💄",
		"refactor": "♻️",
		"release":  "🔖",
		"perf":     "⚡️",
		"test":     "✅",
		"chore":    "🔧",
		"build":    "🏗️",
	}
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
		"reasonix",
	}, ", ")
)

func main() {
	editPath := flag.String("edit", "", "path to a commit message file")
	rangeSpec := flag.String("range", "", "git commit range to validate, for example base..head")
	flag.Parse()

	if (*editPath == "" && *rangeSpec == "") || (*editPath != "" && *rangeSpec != "") {
		fmt.Fprintln(os.Stderr, "set exactly one of --edit <commit-msg-file> or --range <base..head>")
		os.Exit(2)
	}

	if *rangeSpec != "" {
		if problems := lintCommitRange(*rangeSpec); len(problems) > 0 {
			fmt.Fprintln(os.Stderr, strings.Join(problems, "\n"))
			os.Exit(1)
		}
		return
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

func lintCommitRange(rangeSpec string) []string {
	commits, err := gitOutput("log", "--format=%H", rangeSpec)
	if err != nil {
		return []string{err.Error()}
	}
	var problems []string
	for _, commit := range strings.Fields(commits) {
		message, err := gitOutput("log", "-1", "--format=%B", commit)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		for _, problem := range lintCommitMessage(message) {
			problems = append(problems, commit[:12]+": "+problem)
		}
	}
	return problems
}

func gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	data, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %v\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(data)))
	}
	return string(data), nil
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
	} else if !emojiMatchesType(emoji, commitType) {
		problems = append(problems, fmt.Sprintf("emoji must match type %q; expected: %s", commitType, typeEmojiValues[commitType]))
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
	return r >= 0x1F000 || strings.ContainsAny(value, "✨♻⚡✅")
}

func emojiMatchesType(emoji string, commitType string) bool {
	return emoji == typeEmojiValues[commitType]
}

func set(values ...string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}
