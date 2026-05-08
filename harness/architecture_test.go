package harness_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestHarnessDocsAndCommandsStayAligned(t *testing.T) {
	files := map[string]string{
		"agents":    read(t, "AGENTS.md"),
		"agentsZH":  read(t, "AGENTS.zh-CN.md"),
		"harness":   read(t, "docs", "harness-engineering.md"),
		"harnessZH": read(t, "docs", "zh-CN", "harness-engineering.md"),
		"readme":    read(t, "README.md"),
	}
	for _, command := range []string{
		"make check",
		"make test",
		"make test-harness",
		"make vet",
		"make build",
		"make validate",
		"make validate-pr-body",
	} {
		for name, content := range files {
			if !strings.Contains(content, command) {
				t.Fatalf("%s must mention %s", name, command)
			}
		}
	}
}

func TestPrivacyAndOfflineConstraintsStayVisible(t *testing.T) {
	combined := strings.Join([]string{
		read(t, "AGENTS.md"),
		read(t, "AGENTS.zh-CN.md"),
		read(t, "CONTRIBUTING.md"),
		read(t, "docs", "harness-engineering.md"),
		read(t, "docs", "zh-CN", "harness-engineering.md"),
	}, "\n")
	for _, pattern := range []*regexp.Regexp{
		regexp.MustCompile(`(?i)offline|离线`),
		regexp.MustCompile(`(?i)API Key|API KEY`),
		regexp.MustCompile(`(?i)network upload|网络上传`),
		regexp.MustCompile(`(?i)streaming|流式`),
		regexp.MustCompile(`logPrompts=false|logPrompts`),
	} {
		if !pattern.MatchString(combined) {
			t.Fatalf("missing privacy/offline constraint matching %s", pattern)
		}
	}
}

func TestPublicDocsHaveChineseCounterparts(t *testing.T) {
	pairs := map[string]string{
		"README.md":                             "README.zh-CN.md",
		"CONTRIBUTING.md":                       "CONTRIBUTING.zh-CN.md",
		"SECURITY.md":                           "SECURITY.zh-CN.md",
		"CODE_OF_CONDUCT.md":                    "CODE_OF_CONDUCT.zh-CN.md",
		"SUPPORT.md":                            "SUPPORT.zh-CN.md",
		"AGENTS.md":                             "AGENTS.zh-CN.md",
		"docs/harness-engineering.md":           "docs/zh-CN/harness-engineering.md",
		"docs/github-automation.md":             "docs/zh-CN/github-automation.md",
		".github/pull_request_template.md":      ".github/pull_request_template.zh-CN.md",
		".github/ISSUE_TEMPLATE/bug_report.yml": ".github/ISSUE_TEMPLATE/bug_report_zh-CN.yml",
	}
	for english, chinese := range pairs {
		if _, err := os.Stat(filepath.Join(repoRoot(t), english)); err != nil {
			t.Fatalf("missing English document %s: %v", english, err)
		}
		if _, err := os.Stat(filepath.Join(repoRoot(t), chinese)); err != nil {
			t.Fatalf("missing zh-CN counterpart %s for %s: %v", chinese, english, err)
		}
	}
}

func TestGitHubAutomationWorkflowsAreDocumentedAndPresent(t *testing.T) {
	doc := read(t, "docs", "github-automation.md") + "\n" + read(t, "docs", "zh-CN", "github-automation.md")
	for _, path := range []string{
		".github/workflows/ci.yml",
		".github/workflows/pr.yml",
		".github/workflows/pr-review.yml",
		".github/workflows/build.yml",
		".github/workflows/release.yml",
		".github/dependabot.yml",
		".github/CODEOWNERS",
		".github/ISSUE_TEMPLATE/bug_report.yml",
	} {
		if _, err := os.Stat(filepath.Join(repoRoot(t), path)); err != nil {
			t.Fatalf("missing GitHub automation file %s: %v", path, err)
		}
		if !strings.Contains(doc, path) {
			t.Fatalf("GitHub automation docs must mention %s", path)
		}
	}
}

func TestBuildAndReleaseAutomationUseProjectVersion(t *testing.T) {
	for _, path := range []string{
		"VERSION",
		"tools/version/main.go",
		"tools/version/main_test.go",
	} {
		if _, err := os.Stat(filepath.Join(repoRoot(t), path)); err != nil {
			t.Fatalf("missing release version file %s: %v", path, err)
		}
	}

	build := read(t, ".github", "workflows", "build.yml")
	release := read(t, ".github", "workflows", "release.yml")
	for name, workflow := range map[string]string{"build": build, "release": release} {
		for _, expected := range []string{
			"branches: [main]",
			"tags:",
			`- "v*"`,
		} {
			if !strings.Contains(workflow, expected) {
				t.Fatalf("%s workflow must contain %s", name, expected)
			}
		}
		for _, forbidden := range []string{"pull_request:", "workflow_dispatch:"} {
			if strings.Contains(workflow, forbidden) {
				t.Fatalf("%s workflow must not contain %s", name, forbidden)
			}
		}
	}

	releaseExpectations := []string{
		"go run ./tools/version",
		"go run ./tools/version --check-ref",
		"GORELEASER_CURRENT_TAG",
	}
	for _, expected := range releaseExpectations {
		if !strings.Contains(release, expected) {
			t.Fatalf("release workflow must contain %s", expected)
		}
	}

	docs := read(t, "docs", "github-automation.md") + "\n" + read(t, "docs", "zh-CN", "github-automation.md")
	for _, expected := range []string{"VERSION", "main", "v*", "tools/version"} {
		if !strings.Contains(docs, expected) {
			t.Fatalf("GitHub automation docs must mention %s", expected)
		}
	}
}

func TestWorkflowFilesKeepHarnessGates(t *testing.T) {
	makefile := read(t, "Makefile")
	ci := read(t, ".github", "workflows", "ci.yml")
	prTemplate := read(t, ".github", "pull_request_template.md")
	gitignore := read(t, ".gitignore")

	for _, target := range []string{"check:", "test:", "test-harness:", "build:", "validate-pr-body:", "validate:"} {
		if !strings.Contains(makefile, target) {
			t.Fatalf("Makefile missing %s", target)
		}
	}
	for _, command := range []string{"make validate", "make test-harness"} {
		if !strings.Contains(ci, command) {
			t.Fatalf("CI must run %s", command)
		}
	}
	for _, section := range []string{
		"Requirement Classification",
		"Acceptance Criteria",
		"TDD / Test Evidence",
		"Validation",
		"Risk and Rollback",
	} {
		if !strings.Contains(prTemplate, section) {
			t.Fatalf("PR template missing %s", section)
		}
	}
	if !strings.Contains(makefile, "AITOK_CACHE_DIR ?= /tmp/aitok-cache") {
		t.Fatal("Makefile must default Go caches to a cross-platform runner-writable cache directory")
	}
	for _, forbidden := range []string{"/private/tmp/aitok-gocache", "/private/tmp/aitok-gomodcache", "$(CURDIR)/$(AITOK_CACHE_DIR)"} {
		if strings.Contains(makefile, forbidden) {
			t.Fatalf("Makefile must not default to macOS-only cache path %s", forbidden)
		}
	}
	if !strings.Contains(gitignore, "/aitok") || strings.Contains(gitignore, "\naitok\n") {
		t.Fatal(".gitignore must ignore only the root aitok binary, not cmd/aitok")
	}
	assertGitTracks(t, "cmd/aitok/main.go")
}

func TestCommitWorkflowConfigurationStaysExecutable(t *testing.T) {
	for _, path := range []string{
		"tools/commitlint/main.go",
		"tools/commitlint/main_test.go",
		".githooks/commit-msg",
	} {
		if _, err := os.Stat(filepath.Join(repoRoot(t), path)); err != nil {
			t.Fatalf("missing commit workflow file %s: %v", path, err)
		}
	}

	commitlint := read(t, "tools", "commitlint", "main.go")
	for _, expected := range []string{
		`maxHeaderLength = 64`,
		`emoji + " " + type + optional scope + ": " + subject`,
		`"feat"`,
		`"fix"`,
		`"harness"`,
		`"sources"`,
	} {
		if !strings.Contains(commitlint, expected) {
			t.Fatalf("tools/commitlint/main.go must contain %s", expected)
		}
	}

	hook := read(t, ".githooks", "commit-msg")
	if !strings.Contains(hook, `go run ./tools/commitlint --edit "$1"`) {
		t.Fatal(".githooks/commit-msg must run the Go commitlint tool")
	}

	docs := strings.Join([]string{
		read(t, "AGENTS.md"),
		read(t, "AGENTS.zh-CN.md"),
		read(t, "docs", "harness-engineering.md"),
		read(t, "docs", "zh-CN", "harness-engineering.md"),
		read(t, ".github", "pull_request_template.md"),
		read(t, ".github", "pull_request_template.zh-CN.md"),
	}, "\n")
	for _, expected := range []string{"go run ./tools/commitlint", ".githooks/commit-msg", "{emoji} {type}{scope}: {subject}"} {
		if !strings.Contains(docs, expected) {
			t.Fatalf("agent and PR docs must mention %s", expected)
		}
	}
}

func TestHarnessPackageDoesNotImportProductionInternals(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join(repoRoot(t), "harness"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		content := read(t, "harness", entry.Name())
		for _, forbidden := range []string{
			`github.com/sosbs/aitok/internal/sources`,
			`github.com/sosbs/aitok/internal/query`,
			`github.com/sosbs/aitok/internal/report`,
			`github.com/sosbs/aitok/internal/setup`,
		} {
			quotedImport := `"` + forbidden + `"`
			if strings.Contains(content, quotedImport) {
				t.Fatalf("harness test %s imports production package %s", entry.Name(), forbidden)
			}
		}
	}
}

func read(t *testing.T, segments ...string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(append([]string{repoRoot(t)}, segments...)...))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func assertGitTracks(t *testing.T, path string) {
	t.Helper()
	cmd := exec.Command("git", "ls-files", "--error-unmatch", path)
	cmd.Dir = repoRoot(t)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git must track %s: %v\n%s", path, err, string(output))
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(dir) == "harness" {
		return filepath.Dir(dir)
	}
	return dir
}
