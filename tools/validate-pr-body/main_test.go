package main

import "testing"

func TestPRBodyValidationHelpers(t *testing.T) {
	body := `## Summary

- Adds harness constraints.

## Requirement Classification

- Type: harness/tooling
- User-visible outcome: none; repository-only.
- Target platform(s): CLI, repository-only.
- Scope assumptions: Harness only.

## Acceptance Criteria

- PR validation rejects incomplete metadata.
- Failure edge: template-only sections fail.

## Changed Areas

- Harness docs and scripts.

## Release Decision

- Release not required: harness/tooling only; no user-facing binary behavior changed.

## TDD / Test Evidence

- Test/sensor added before implementation: failing harness case.
- Unit tests: validate-pr-body helper tests.
- CLI/manual/platform evidence: CI runs the validator.
- Failure and edge cases covered: empty/template PR bodies.
- Acceptance criteria evidence map: covered by harness tests.

## Validation

- [x] make check
- Skipped validation and reason: none skipped.

## Risk and Rollback

- Risk: stricter PR checks.
- Rollback: remove validator from CI.
- Residual risk: none.
`
	for _, section := range requiredSections {
		if !isFilled(getSection(body, section)) {
			t.Fatalf("section %s should be filled", section)
		}
	}
	for _, check := range checks {
		if !check.pattern.MatchString(body) {
			t.Fatalf("body should satisfy %s", check.name)
		}
	}
}

func TestFeatureOrBugfixRequiresReleaseDecision(t *testing.T) {
	body := `## Summary

- Adds a new CLI budget command.

## Requirement Classification

- Type: feature
- User-visible outcome: new CLI behavior.
- Target platform(s): CLI.
- Scope assumptions: Product feature.

## Acceptance Criteria

- Feature works.
- Failure edge is covered.

## Changed Areas

- CLI command implementation and report output.

## Release Decision

- Release not required: feature can wait.

## TDD / Test Evidence

- Test/sensor added before implementation: failing test.
- Unit tests: covered.
- CLI/manual/platform evidence: CLI smoke.
- Failure and edge cases covered: failure path.
- Acceptance criteria evidence map: mapped.

## Validation

- [x] make check
- Skipped validation and reason: none skipped.

## Risk and Rollback

- Risk: new command behavior.
- Rollback: revert command.
- Residual risk: none.
`
	problems := validateBody(body)
	if !containsProblem(problems, "Feature and bugfix PRs must require a follow-up release or explicit deferral") {
		t.Fatalf("feature PR with release-not-required should fail, got %v", problems)
	}
}

func TestChineseFeatureReleaseNotRequiredFails(t *testing.T) {
	body := `## Summary

- 修复 CLI 预算检查错误。

## Requirement Classification

- Type: bugfix
- User-visible outcome: 修复 CLI 行为。
- Target platform(s): CLI.
- Scope assumptions: 产品 BUG 修复。

## Acceptance Criteria

- BUG 已修复。
- 覆盖失败边界。

## Changed Areas

- CLI command implementation and report output.

## Release Decision

- 无需发版：BUG 修复可以等。

## TDD / Test Evidence

- Test/sensor added before implementation: failing test.
- Unit tests: covered.
- CLI/manual/platform evidence: CLI smoke.
- Failure and edge cases covered: failure path.
- Acceptance criteria evidence map: mapped.

## Validation

- [x] make check
- Skipped validation and reason: none skipped.

## Risk and Rollback

- Risk: new command behavior.
- Rollback: revert command.
- Residual risk: none.
`
	problems := validateBody(body)
	if !containsProblem(problems, "Feature and bugfix PRs must require a follow-up release or explicit deferral") {
		t.Fatalf("bugfix PR with Chinese release-not-required should fail, got %v", problems)
	}
}

func TestFeatureReleaseDecisionPasses(t *testing.T) {
	body := `## Summary

- Adds a new CLI budget command.

## Requirement Classification

- Type: feature
- User-visible outcome: new CLI behavior.
- Target platform(s): CLI.
- Scope assumptions: Product feature.

## Acceptance Criteria

- Feature works.
- Failure edge is covered.

## Changed Areas

- CLI command implementation and report output.

## Release Decision

- Release required after merge: bump VERSION, tag v*, and run release workflow.

## TDD / Test Evidence

- Test/sensor added before implementation: failing test.
- Unit tests: covered.
- CLI/manual/platform evidence: CLI smoke.
- Failure and edge cases covered: failure path.
- Acceptance criteria evidence map: mapped.

## Validation

- [x] make check
- Skipped validation and reason: none skipped.

## Risk and Rollback

- Risk: new command behavior.
- Rollback: revert command.
- Residual risk: none.
`
	if problems := validateBody(body); len(problems) > 0 {
		t.Fatalf("feature PR with release-required should pass, got %v", problems)
	}
}

func TestTemplateSectionsAreNotFilled(t *testing.T) {
	if isFilled("-") {
		t.Fatal("dash-only section should be unfilled")
	}
	if isFilled("TODO") {
		t.Fatal("TODO section should be unfilled")
	}
}

func containsProblem(problems []string, want string) bool {
	for _, problem := range problems {
		if problem == want {
			return true
		}
	}
	return false
}
