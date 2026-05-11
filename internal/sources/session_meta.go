package sources

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const titleMaxChars = 160

var codexUUIDRE = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

type threadMeta struct {
	ID           string
	Name         string
	Source       string
	CWD          string
	CreatedAt    time.Time
	LastActiveAt time.Time
	Skip         bool
}

func readHeadTailJSON(path string, headCount, tailCount int) ([]map[string]any, []map[string]any) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var head []map[string]any
	var tail []map[string]any
	for scanner.Scan() {
		var obj map[string]any
		if err := json.Unmarshal([]byte(scanner.Text()), &obj); err != nil {
			continue
		}
		if len(head) < headCount {
			head = append(head, obj)
		}
		tail = append(tail, obj)
		if len(tail) > tailCount {
			tail = tail[1:]
		}
	}
	return head, tail
}

func readCodexSessionIndex(path string) map[string]string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	titles := map[string]string{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var obj map[string]any
		if err := json.Unmarshal([]byte(scanner.Text()), &obj); err != nil {
			continue
		}
		id := stringValue(obj["id"])
		title := firstNonEmptyString(obj, "thread_name", "threadName", "title", "name")
		if id != "" && title != "" {
			titles[id] = title
		}
	}
	return titles
}

func parseCodexThreadMeta(path string) threadMeta {
	head, tail := readHeadTailJSON(path, 40, 80)
	meta := threadMeta{Source: path}
	var customTitle, summaryTitle, firstUser string
	for _, obj := range head {
		ts := parseTimestamp(stringValue(obj["timestamp"]))
		if meta.CreatedAt.IsZero() && !ts.IsZero() {
			meta.CreatedAt = ts
		}
		switch stringValue(obj["type"]) {
		case "session_meta":
			payload := objectValue(obj["payload"])
			if hasSubagentSource(payload) {
				meta.Skip = true
				return meta
			}
			if meta.ID == "" {
				meta.ID = stringValue(payload["id"])
			}
			if meta.CWD == "" {
				meta.CWD = stringValue(payload["cwd"])
			}
		case "response_item":
			if firstUser == "" {
				if text, ok := codexMessageText(obj, "user"); ok && isRealUserTitle(text) {
					firstUser = text
				}
			}
		}
	}
	for i := len(tail) - 1; i >= 0; i-- {
		obj := tail[i]
		if meta.LastActiveAt.IsZero() {
			meta.LastActiveAt = parseTimestamp(stringValue(obj["timestamp"]))
		}
		if customTitle == "" {
			customTitle = firstNonEmptyString(obj, "customTitle", "title")
			if customTitle == "" && stringValue(obj["type"]) == "custom-title" {
				customTitle = firstNonEmptyString(obj, "custom_title", "name")
			}
		}
		if summaryTitle == "" {
			summaryTitle = codexExplicitTitle(obj)
		}
	}
	if meta.ID == "" {
		meta.ID = inferCodexID(path)
	}
	meta.Name = chooseThreadTitle(customTitle, summaryTitle, firstUser, pathBase(meta.CWD), shortID(meta.ID))
	return meta
}

func codexExplicitTitle(obj map[string]any) string {
	title := firstNonEmptyString(obj, "summary", "summary_title", "summaryTitle", "thread_title", "threadTitle")
	if title != "" {
		return title
	}
	switch stringValue(obj["type"]) {
	case "thread-title", "thread_title", "conversation-title", "conversation_title", "title":
		if title := firstNonEmptyString(obj, "title", "name", "thread_name", "threadName", "summary", "text"); title != "" {
			return title
		}
	}
	payload := objectValue(obj["payload"])
	if payload == nil {
		return ""
	}
	title = firstNonEmptyString(payload, "summary", "summary_title", "summaryTitle", "thread_title", "threadTitle")
	if title != "" {
		return title
	}
	switch stringValue(payload["type"]) {
	case "thread-title", "thread_title", "conversation-title", "conversation_title", "title":
		return firstNonEmptyString(payload, "title", "name", "thread_name", "threadName", "summary", "text")
	default:
		return ""
	}
}

func parseClaudeThreadMeta(path string) threadMeta {
	if strings.HasPrefix(filepath.Base(path), "agent-") {
		return threadMeta{Source: path, Skip: true}
	}
	head, tail := readHeadTailJSON(path, 40, 80)
	meta := threadMeta{Source: path}
	var customTitle, summaryTitle, firstUser string
	for _, obj := range head {
		if meta.ID == "" {
			meta.ID = stringValue(obj["sessionId"])
		}
		if meta.CWD == "" {
			meta.CWD = stringValue(obj["cwd"])
		}
		ts := parseTimestamp(stringValue(obj["timestamp"]))
		if meta.CreatedAt.IsZero() && !ts.IsZero() {
			meta.CreatedAt = ts
		}
		if firstUser == "" && isClaudeUserMessage(obj) {
			if msg := objectValue(obj["message"]); msg != nil {
				text := extractText(msg["content"])
				if isRealClaudeTitle(text) {
					firstUser = text
				}
			}
		}
	}
	for i := len(tail) - 1; i >= 0; i-- {
		obj := tail[i]
		if meta.LastActiveAt.IsZero() {
			meta.LastActiveAt = parseTimestamp(stringValue(obj["timestamp"]))
		}
		if customTitle == "" && stringValue(obj["type"]) == "custom-title" {
			customTitle = firstNonEmptyString(obj, "customTitle", "custom_title", "title")
		}
		if summaryTitle == "" && objectValue(obj["message"]) != nil {
			text := extractText(objectValue(obj["message"])["content"])
			if strings.TrimSpace(text) != "" {
				summaryTitle = text
			}
		}
	}
	if meta.ID == "" {
		meta.ID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	meta.Name = chooseThreadTitle(customTitle, summaryTitle, firstUser, pathBase(meta.CWD), shortID(meta.ID))
	return meta
}

func parseTimestamp(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000Z"} {
		if ts, err := time.Parse(layout, raw); err == nil {
			return ts
		}
	}
	return time.Time{}
}

func firstNonEmptyString(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := cleanTitleCandidate(stringValue(obj[key])); value != "" {
			return value
		}
	}
	return ""
}

func cleanTitleCandidate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	switch strings.ToLower(value) {
	case "none", "null", "nil", "unknown":
		return ""
	default:
		return value
	}
}

func chooseThreadTitle(values ...string) string {
	for _, value := range values {
		if text := truncateTitle(strings.TrimSpace(value)); text != "" {
			return text
		}
	}
	return "unknown"
}

func truncateTitle(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= titleMaxChars {
		return value
	}
	if titleMaxChars <= 3 {
		return value[:titleMaxChars]
	}
	return value[:titleMaxChars-3] + "..."
}

func shortID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}

func pathBase(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}

func inferCodexID(path string) string {
	name := filepath.Base(path)
	if match := codexUUIDRE.FindString(name); match != "" {
		return match
	}
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func hasSubagentSource(payload map[string]any) bool {
	if payload == nil {
		return false
	}
	source := objectValue(payload["source"])
	if source == nil {
		return false
	}
	_, ok := source["subagent"]
	return ok
}

func codexMessageText(obj map[string]any, role string) (string, bool) {
	if stringValue(obj["type"]) != "response_item" {
		return "", false
	}
	payload := objectValue(obj["payload"])
	if payload == nil || stringValue(payload["type"]) != "message" || stringValue(payload["role"]) != role {
		return "", false
	}
	text := extractText(payload["content"])
	return strings.TrimSpace(text), text != ""
}

func isRealUserTitle(text string) bool {
	text = strings.TrimSpace(text)
	return text != "" &&
		!strings.HasPrefix(text, "# AGENTS.md") &&
		!strings.HasPrefix(text, "<environment_context>") &&
		!strings.HasPrefix(text, "<turn_aborted>") &&
		!strings.HasPrefix(text, "# Context from my IDE setup:")
}

func isClaudeUserMessage(obj map[string]any) bool {
	if stringValue(obj["type"]) == "user" {
		return true
	}
	msg := objectValue(obj["message"])
	return msg != nil && stringValue(msg["role"]) == "user"
}

func isRealClaudeTitle(text string) bool {
	text = strings.TrimSpace(text)
	return text != "" && !strings.Contains(text, "<local-command-caveat>") && !strings.HasPrefix(text, "<command-name>")
}

func extractText(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			if m := objectValue(item); m != nil {
				if text := stringValue(m["text"]); text != "" {
					parts = append(parts, text)
				} else if text := stringValue(m["content"]); text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}
