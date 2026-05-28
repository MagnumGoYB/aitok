package sources

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/MagnumGoYB/aitok/internal/usage"
	_ "modernc.org/sqlite"
)

type OpenCode struct {
	Home        string
	WindowStart time.Time
	WindowEnd   time.Time
}

func NewOpenCode(opts Options) OpenCode {
	return OpenCode{Home: cleanHome(opts.Home), WindowStart: opts.WindowStart, WindowEnd: opts.WindowEnd}
}

func (o OpenCode) Name() usage.Tool {
	return usage.ToolOpenCode
}

func (o OpenCode) Read(ctx context.Context) ([]usage.UsageEvent, error) {
	var events []usage.UsageEvent
	err := o.Scan(ctx, func(event usage.UsageEvent) error {
		events = append(events, event)
		return nil
	})
	return events, err
}

func (o OpenCode) Scan(ctx context.Context, handle func(usage.UsageEvent) error) error {
	dbPath := filepath.Join(o.Home, ".local", "share", "opencode", "opencode.db")
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return nil
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return nil
	}

	sessions, err := o.readSessions(ctx, db)
	if err != nil {
		return nil
	}
	o.readFirstUserMessages(ctx, db, sessions)

	query := o.messageQuery()
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var id, sessionID, dataStr string
		var timeCreated int64
		if err := rows.Scan(&id, &sessionID, &timeCreated, &dataStr); err != nil {
			continue
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
			continue
		}
		role := stringValue(data["role"])
		if role != "assistant" {
			continue
		}
		tokens := o.parseTokens(data)
		if tokens.NormalizedTotal() == 0 && tokens.CachedInput == 0 {
			continue
		}
		ts := time.UnixMilli(timeCreated)
		// Use data.time.created if available (may differ from time_created column)
		if dataTime := objectValue(data["time"]); dataTime != nil {
			if created := intValue(dataTime["created"]); created > 0 {
				ts = time.UnixMilli(created)
			}
		}
		// Double-check window in Go: SQL filters on time_created column,
		// but we re-check here using the JSON data.time.created field
		// which may differ from the column value.
		if !o.inWindow(ts) {
			continue
		}
		model := usage.Unknown(stringValue(data["modelID"]))
		provider := usage.Unknown(stringValue(data["providerID"]))
		cwd := ""
		if path := objectValue(data["path"]); path != nil {
			cwd = stringValue(path["cwd"])
		}
		session := sessions[sessionID]
		event := usage.UsageEvent{
			ID:                 id,
			Timestamp:          ts,
			Tool:               usage.ToolOpenCode,
			Model:              model,
			Provider:           provider,
			CWD:                cwd,
			Source:             dbPath,
			ThreadID:           sessionID,
			ThreadName:         session.name,
			ThreadSource:       session.directory,
			ThreadCreatedAt:    session.timeCreated,
			ThreadLastActiveAt: session.timeUpdated,
			Usage:              tokens,
		}
		if err := handle(event); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (o OpenCode) inWindow(ts time.Time) bool {
	if o.WindowStart.IsZero() {
		return true
	}
	if ts.Before(o.WindowStart) {
		return false
	}
	if !o.WindowEnd.IsZero() && (ts.Equal(o.WindowEnd) || ts.After(o.WindowEnd)) {
		return false
	}
	return true
}

func (o OpenCode) messageQuery() string {
	// Fetch all messages; role filtering happens in Go to handle malformed JSON gracefully.
	base := "select id, session_id, time_created, data from message"
	if o.WindowStart.IsZero() {
		return base + " order by time_created asc"
	}
	if o.WindowEnd.IsZero() {
		return fmt.Sprintf("%s where time_created >= %d order by time_created asc", base, o.WindowStart.UnixMilli())
	}
	return fmt.Sprintf("%s where time_created >= %d and time_created < %d order by time_created asc", base, o.WindowStart.UnixMilli(), o.WindowEnd.UnixMilli())
}

func (o OpenCode) parseTokens(data map[string]any) usage.TokenUsage {
	tokens := objectValue(data["tokens"])
	if tokens == nil {
		return usage.TokenUsage{}
	}
	cache := objectValue(tokens["cache"])
	var cachedInput, cacheCreation int64
	if cache != nil {
		cachedInput = intValue(cache["read"])
		cacheCreation = intValue(cache["write"])
	}
	tu := usage.TokenUsage{
		Input:         intValue(tokens["input"]),
		Output:        intValue(tokens["output"]),
		Reasoning:     intValue(tokens["reasoning"]),
		CachedInput:   cachedInput,
		CacheCreation: cacheCreation,
		Total:         intValue(tokens["total"]),
	}
	return tu
}

type opencodeSession struct {
	name           string
	slug           string
	directory      string
	titleIsDefault bool
	timeCreated    time.Time
	timeUpdated    time.Time
}

func (o OpenCode) readSessions(ctx context.Context, db *sql.DB) (map[string]opencodeSession, error) {
	rows, err := db.QueryContext(ctx, `select id, title, slug, directory, time_created, time_updated from session`)
	if err != nil {
		// If session table is missing or query fails, return empty map.
		// Messages will still be processed but without thread metadata.
		return map[string]opencodeSession{}, nil
	}
	defer rows.Close()

	sessions := map[string]opencodeSession{}
	for rows.Next() {
		var id, title, slug, directory string
		var timeCreated, timeUpdated int64
		if err := rows.Scan(&id, &title, &slug, &directory, &timeCreated, &timeUpdated); err != nil {
			continue
		}
		title = strings.TrimSpace(title)
		sessions[id] = opencodeSession{
			name:           opencodeThreadName(title, "", slug, directory),
			slug:           slug,
			directory:      directory,
			titleIsDefault: title == "" || strings.HasPrefix(title, "New session - "),
			timeCreated:    time.UnixMilli(timeCreated),
			timeUpdated:    time.UnixMilli(timeUpdated),
		}
	}
	return sessions, rows.Err()
}

func opencodeThreadName(title, firstUserMessage, slug, directory string) string {
	title = strings.TrimSpace(title)
	if title != "" && !strings.HasPrefix(title, "New session - ") {
		return truncateTitle(title)
	}
	if firstUserMessage != "" {
		return truncateTitle(firstUserMessage)
	}
	if directory != "" {
		return pathBase(directory)
	}
	if slug != "" {
		return truncateTitle(slug)
	}
	return "unknown"
}

func (o OpenCode) readFirstUserMessages(ctx context.Context, db *sql.DB, sessions map[string]opencodeSession) {
	rows, err := db.QueryContext(ctx,
		`select m.session_id, json_extract(p.data, '$.text')
		 from message m
		 join part p on p.message_id = m.id
		 where json_extract(m.data, '$.role') = 'user'
		 and json_extract(p.data, '$.type') = 'text'
		 order by m.time_created asc`)
	if err != nil {
		return
	}
	defer rows.Close()

	seen := map[string]bool{}
	for rows.Next() {
		var sessionID, text string
		if err := rows.Scan(&sessionID, &text); err != nil {
			continue
		}
		if seen[sessionID] {
			continue
		}
		text = strings.TrimSpace(text)
		if !isOpenCodeRealTitle(text) {
			continue
		}
		seen[sessionID] = true
		session, ok := sessions[sessionID]
		if !ok {
			continue
		}
		if session.titleIsDefault {
			session.name = truncateTitle(text)
			sessions[sessionID] = session
		}
	}
}

func isOpenCodeRealTitle(text string) bool {
	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) < 2 {
		return false
	}
	if text == "say hi" || text == "hi" {
		return false
	}
	if _, err := fmt.Sscanf(text, "%d", new(int)); err == nil {
		return false
	}
	return true
}
