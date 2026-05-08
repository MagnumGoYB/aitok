package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/MagnumGoYB/aitok/internal/query"
)

type Payload struct {
	GeneratedAt time.Time      `json:"generated_at"`
	Window      query.Window   `json:"window"`
	GroupBy     query.GroupBy  `json:"group_by"`
	Results     []query.Result `json:"results"`
}

func Write(w io.Writer, format string, payload Payload) error {
	switch format {
	case "", "table":
		return WriteTable(w, payload.Results)
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	case "markdown":
		return WriteMarkdown(w, payload.Results)
	default:
		return fmt.Errorf("unknown format %q", format)
	}
}

func WriteTable(w io.Writer, results []query.Result) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "GROUP\tEVENTS\tINPUT\tOUTPUT\tCACHED\tREASONING\tTOOL\tTOTAL")
	for _, result := range results {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
			formatKey(result.Key),
			result.Events,
			result.Usage.Input,
			result.Usage.Output,
			result.Usage.CachedInput,
			result.Usage.Reasoning,
			result.Usage.Tool,
			result.Usage.NormalizedTotal(),
		)
	}
	return tw.Flush()
}

func WriteMarkdown(w io.Writer, results []query.Result) error {
	fmt.Fprintln(w, "| Group | Events | Input | Output | Cached | Reasoning | Tool | Total |")
	fmt.Fprintln(w, "| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |")
	for _, result := range results {
		fmt.Fprintf(w, "| %s | %d | %d | %d | %d | %d | %d | %d |\n",
			escapeMarkdown(formatKey(result.Key)),
			result.Events,
			result.Usage.Input,
			result.Usage.Output,
			result.Usage.CachedInput,
			result.Usage.Reasoning,
			result.Usage.Tool,
			result.Usage.NormalizedTotal(),
		)
	}
	return nil
}

func formatKey(key map[string]string) string {
	keys := make([]string, 0, len(key))
	for k := range key {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, k+"="+key[k])
	}
	return strings.Join(parts, ", ")
}

func escapeMarkdown(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}
