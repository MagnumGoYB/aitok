package usage

import "testing"

func TestNormalizedTotalFallbackIncludesReasoning(t *testing.T) {
	tokens := TokenUsage{
		Input:     10,
		Output:    2,
		Reasoning: 7,
		Tool:      1,
	}
	if got := tokens.NormalizedTotal(); got != 20 {
		t.Fatalf("NormalizedTotal() = %d, want 20", got)
	}
}
