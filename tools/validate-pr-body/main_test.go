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

func TestTemplateSectionsAreNotFilled(t *testing.T) {
	if isFilled("-") {
		t.Fatal("dash-only section should be unfilled")
	}
	if isFilled("TODO") {
		t.Fatal("TODO section should be unfilled")
	}
}
