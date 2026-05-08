package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/sosbs/aitok/internal/query"
	"github.com/sosbs/aitok/internal/usage"
)

func TestWriteJSONAndMarkdown(t *testing.T) {
	payload := Payload{
		GeneratedAt: time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC),
		GroupBy:     query.GroupBy{"tool"},
		Results: []query.Result{{
			Key:    map[string]string{"tool": "codex"},
			Events: 1,
			Usage:  usage.TokenUsage{Input: 5, Output: 7},
		}},
	}
	var jsonOut bytes.Buffer
	if err := Write(&jsonOut, "json", payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(jsonOut.String(), `"generated_at"`) || !strings.Contains(jsonOut.String(), `"total": 0`) {
		t.Fatalf("json output missing stable fields: %s", jsonOut.String())
	}
	var mdOut bytes.Buffer
	if err := Write(&mdOut, "markdown", payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mdOut.String(), "| Group | Events |") || !strings.Contains(mdOut.String(), "| tool=codex | 1 |") {
		t.Fatalf("markdown output unexpected: %s", mdOut.String())
	}
}
