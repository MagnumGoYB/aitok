package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var requiredSections = []string{
	"Summary",
	"Requirement Classification",
	"Acceptance Criteria",
	"Changed Areas",
	"TDD / Test Evidence",
	"Validation",
	"Risk and Rollback",
}

var checks = []struct {
	name    string
	pattern *regexp.Regexp
	message string
}{
	{"unit", regexp.MustCompile(`(?i)\bunit\b|单元测试`), "PR body must mention unit test coverage or explain why absent."},
	{"cli", regexp.MustCompile(`(?i)\bcli\b|manual|platform|手动|平台`), "PR body must mention CLI/manual/platform evidence or explain why absent."},
	{"rollback", regexp.MustCompile(`(?i)rollback|回滚`), "PR body must include rollback notes."},
	{"classification", regexp.MustCompile(`(?i)feature|bugfix|refactor|harness/tooling|analysis-only`), "PR body must classify the requirement iteration type."},
	{"outcome", regexp.MustCompile(`(?i)user-visible outcome|visible outcome|用户可见`), "PR body must state the user-visible outcome or explain none."},
	{"platform", regexp.MustCompile(`(?i)target platform|repository-only|darwin|linux|windows|CLI|平台`), "PR body must state the target platform scope."},
	{"edge", regexp.MustCompile(`(?i)non-happy|failure|edge|边界|失败`), "PR body must include failure/edge coverage or explain why absent."},
	{"test-first", regexp.MustCompile(`(?i)test/sensor added before implementation|failing test|sensor-first|test-first|测试先行|传感器先行`), "PR body must include test-first or sensor-first evidence."},
	{"evidence-map", regexp.MustCompile(`(?i)acceptance criteria evidence map|evidence map|验收.*证据`), "PR body must map acceptance criteria to evidence."},
	{"skipped", regexp.MustCompile(`(?i)skipped validation|not skipped|none skipped|跳过`), "PR body must state skipped validation, even when none was skipped."},
	{"residual", regexp.MustCompile(`(?i)residual risk|none|残余风险`), "PR body must state residual risk."},
}

func main() {
	body, err := extractBody()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var errors []string
	if strings.TrimSpace(body) == "" {
		errors = append(errors, "Pull request body is empty.")
	}
	for _, section := range requiredSections {
		content := getSection(body, section)
		if content == "" {
			errors = append(errors, "Missing required section: "+section)
			continue
		}
		if !isFilled(content) {
			errors = append(errors, "Section still looks unfilled: "+section)
		}
	}
	for _, check := range checks {
		if !check.pattern.MatchString(body) {
			errors = append(errors, check.message)
		}
	}
	if len(errors) > 0 {
		fmt.Fprintln(os.Stderr, strings.Join(errors, "\n"))
		os.Exit(1)
	}
	fmt.Println("Pull request body contains required harness evidence.")
}

func extractBody() (string, error) {
	if eventPath := os.Getenv("GITHUB_EVENT_PATH"); eventPath != "" {
		data, err := os.ReadFile(eventPath)
		if err != nil {
			return "", err
		}
		var event struct {
			PullRequest struct {
				Body string `json:"body"`
			} `json:"pull_request"`
		}
		if err := json.Unmarshal(data, &event); err != nil {
			return "", err
		}
		return event.PullRequest.Body, nil
	}
	return os.Getenv("PR_BODY"), nil
}

func getSection(body, section string) string {
	pattern := regexp.MustCompile(`(?is)(?:^|\n)##\s+` + regexp.QuoteMeta(section) + `\s*\n(.*?)(?:\n##\s+|$)`)
	match := pattern.FindStringSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func isFilled(section string) bool {
	compact := strings.TrimSpace(section)
	if len(compact) < 12 {
		return false
	}
	switch compact {
	case "-", "- ", "- [ ]", "TBD", "TODO", "N/A", "Not applicable":
		return false
	default:
		return true
	}
}
