package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/MagnumGoYB/aitok/internal/query"
	"github.com/MagnumGoYB/aitok/internal/usage"
	"github.com/mattn/go-runewidth"
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
	if !strings.Contains(got, "+-") || !strings.Contains(got, "| GROUP") || !strings.Contains(got, "| REQUESTS") {
		t.Fatalf("table must render borders and column separators: %s", got)
	}
}

func TestWriteThreadsTableAlignsWideCharacters(t *testing.T) {
	var out bytes.Buffer
	err := WriteThreadsTable(&out, []query.ThreadResult{
		{
			ID:       "019e167b-b7e8-7743-8bb3-fd9951e5ef2f",
			Name:     "修正日期范围与threads列表",
			Tool:     "codex",
			Model:    "gpt-5.5",
			Provider: "bcb",
			Requests: 199,
			Events:   199,
			Usage:    usage.TokenUsage{Input: 28_345_680},
			CostUSD:  22.0954,
		},
		{
			ID:       "019e1522-e729-70c2-b013-bf66207c6b51",
			Name:     "mini_program_wechat",
			Tool:     "codex",
			Model:    "gpt-5.5",
			Provider: "bcb",
			Requests: 61,
			Events:   61,
			Usage:    usage.TokenUsage{Input: 6_125_217},
			CostUSD:  6.7136,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := nonEmptyLines(out.String())
	if len(lines) < 5 {
		t.Fatalf("expected bordered table, got: %s", out.String())
	}
	width := runewidth.StringWidth(lines[0])
	for _, line := range lines[1:] {
		if got := runewidth.StringWidth(line); got != width {
			t.Fatalf("table row display width = %d, want %d\nrow: %q\nfull table:\n%s", got, width, line, out.String())
		}
	}
}

func TestWriteThreadsWhenPayloadIncludesThreads(t *testing.T) {
	payload := Payload{
		Results: []query.Result{{
			Key:      map[string]string{"tool": "codex"},
			Requests: 1,
			Events:   1,
			Usage:    usage.TokenUsage{Input: 5},
		}},
		Threads: []query.ThreadResult{{
			ID:       "thread-a",
			Name:     "Custom title",
			Tool:     "codex",
			Model:    "gpt-5.4",
			Provider: "openai",
			Requests: 1,
			Events:   1,
			Usage:    usage.TokenUsage{Input: 5},
			CostUSD:  0.0001,
		}},
	}
	var out bytes.Buffer
	if err := Write(&out, "markdown", payload); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "## Threads") || !strings.Contains(got, "| thread-a | Custom title | codex | gpt-5.4 | openai |") {
		t.Fatalf("markdown output missing threads: %s", got)
	}
	out.Reset()
	if err := Write(&out, "json", payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"threads"`) || !strings.Contains(out.String(), `"id": "thread-a"`) {
		t.Fatalf("json output missing threads: %s", out.String())
	}
}

func nonEmptyLines(value string) []string {
	var lines []string
	for _, line := range strings.Split(value, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
