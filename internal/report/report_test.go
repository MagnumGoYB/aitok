package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/MagnumGoYB/aitok/internal/query"
	"github.com/MagnumGoYB/aitok/internal/usage"
)

func TestWriteJSONAndMarkdown(t *testing.T) {
	payload := Payload{
		GeneratedAt: time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC),
		GroupBy:     query.GroupBy{"tool"},
		Results: []query.Result{{
			Key:      map[string]string{"tool": "codex"},
			Events:   1,
			Requests: 1,
			Usage:    usage.TokenUsage{Input: 5, Output: 7},
			CostUSD:  0.0123,
		}},
	}
	var jsonOut bytes.Buffer
	if err := Write(&jsonOut, "json", payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(jsonOut.String(), `"generated_at"`) || !strings.Contains(jsonOut.String(), `"requests": 1`) || !strings.Contains(jsonOut.String(), `"cost_usd": 0.0123`) {
		t.Fatalf("json output missing stable fields: %s", jsonOut.String())
	}
	var mdOut bytes.Buffer
	if err := Write(&mdOut, "markdown", payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mdOut.String(), "| Group | Requests | Events | Cost USD |") || !strings.Contains(mdOut.String(), "| tool=codex | 1 | 1 | $0.0123 |") {
		t.Fatalf("markdown output unexpected: %s", mdOut.String())
	}
}

func TestWriteTableIncludesRequestsAndCost(t *testing.T) {
	var out bytes.Buffer
	err := WriteTable(&out, []query.Result{{
		Key:      map[string]string{"model": "gpt-5.4"},
		Requests: 3,
		Events:   3,
		Usage:    usage.TokenUsage{Input: 100, Output: 20},
		CostUSD:  1.2345,
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "REQUESTS") || !strings.Contains(got, "COST_USD") || !strings.Contains(got, "$1.2345") {
		t.Fatalf("table missing request/cost fields: %s", got)
	}
}
