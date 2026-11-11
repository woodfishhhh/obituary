package gui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LogQuery struct {
	Source string `json:"source"`
	Lines  int    `json:"lines"`
	Text   string `json:"text"`
	From   string `json:"from"` // inclusive RFC3339; empty means unbounded
	To     string `json:"to"`
	Level  string `json:"level"`
}
type LogLine struct {
	Text  string    `json:"text"`
	Time  time.Time `json:"time,omitzero"`
	Level string    `json:"level"`
	// Audit entries also carry their parts, so the window can write one
	// readable sentence while search and copy keep the raw line.
	Action string `json:"action,omitempty"`
	ID     int64  `json:"id,omitempty"`
	Path   string `json:"path,omitempty"`
	Detail string `json:"detail,omitempty"`
}
type LogPage struct {
	Lines        []LogLine `json:"lines"`
	Truncated    bool      `json:"truncated"`
	ScannedLines int       `json:"scannedLines"`
}

func (a *API) Logs(query LogQuery) (LogPage, error) {
	from, to, err := logBounds(&query)
	if err != nil {
		return LogPage{}, err
	}
	file, err := openLog(filepath.Join(a.deps.Service.Store.Home, query.Source))
	if errors.Is(err, os.ErrNotExist) {
		return LogPage{Lines: []LogLine{}}, nil
	}
	if err != nil {
		return LogPage{}, err
	}
	defer file.Close()
	lines, truncated, err := tailLines(file, query.Lines)
	if err != nil {
		return LogPage{}, err
	}
	page := LogPage{Lines: []LogLine{}, Truncated: truncated, ScannedLines: len(lines)}
	for _, text := range lines {
		line := parseLogLine(text, query.Source)
		if !matchesLog(line, query, from, to) {
			continue
		}
		page.Lines = append(page.Lines, line)
	}
	return page, nil
}

func logBounds(query *LogQuery) (time.Time, time.Time, error) {
	switch query.Source {
	case "hook.log", "obitd.log", "hook-audit.log", "audit.jsonl":
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("unknown log source")
	}
	if query.Lines < 0 || query.Lines > 20000 {
		return time.Time{}, time.Time{}, fmt.Errorf("lines must be between 0 and 20000")
	}
	if query.Lines == 0 {
		query.Lines = 2000
	}
	query.Level = strings.ToLower(query.Level)
	switch query.Level {
	case "", "debug", "info", "warn", "error":
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("unknown log level")
	}
	var from, to time.Time
	var err error
	if query.From != "" {
		from, err = time.Parse(time.RFC3339Nano, query.From)
	}
	if err != nil {
		return from, to, err
	}
	if query.To != "" {
		to, err = time.Parse(time.RFC3339Nano, query.To)
	}
	if err == nil && !from.IsZero() && !to.IsZero() && from.After(to) {
		err = fmt.Errorf("from must not be after to")
	}
	return from, to, err
}

// Seek backwards in bounded blocks; never scan the complete log. A single
// malformed huge line is bounded too. Truncated tells the UI to disclose that
// the byte/line limit cut off older data; filters apply to this selected tail.
func tailLines(file *os.File, limit int) ([]string, bool, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, false, err
	}
	const maxTail = 8 << 20
	position := info.Size()
	var blocks [][]byte
	count, size := 0, 0
	for position > 0 && count <= limit && size < maxTail {
		n := min(int64(32<<10), position, int64(maxTail-size))
		position -= n
		block := make([]byte, n)
		read, err := file.ReadAt(block, position)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, false, err
		}
		block = block[:read]
		blocks = append(blocks, block)
		count += bytes.Count(block, []byte{'\n'})
		// Count the attempted range as well: a concurrent truncation may make
		// ReadAt return no bytes, but must not turn the tail into an unbounded scan.
		size += int(n)
	}
	data := make([]byte, 0, size)
	for i := len(blocks) - 1; i >= 0; i-- {
		data = append(data, blocks[i]...)
	}
	if position > 0 {
		if end := bytes.IndexByte(data, '\n'); end >= 0 {
			data = data[end+1:]
		} else {
			data = nil
		}
	}
	if len(data) == 0 {
		return []string{}, position > 0, nil
	}
	text := strings.TrimSuffix(string(data), "\n")
	lines := strings.Split(text, "\n")
	truncated := position > 0 || len(lines) > limit
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	return lines, truncated, nil
}

func parseLogLine(text, source string) LogLine {
	line := LogLine{Text: text, Level: "info"}
	var event struct {
		Time   time.Time `json:"time"`
		At     time.Time `json:"at"`
		Level  string    `json:"level"`
		Action string    `json:"action"`
		ID     int64     `json:"id"`
		Path   string    `json:"path"`
		Detail string    `json:"detail"`
	}
	if json.Unmarshal([]byte(text), &event) == nil {
		line.Time = event.Time
		line.Action, line.ID, line.Path, line.Detail = event.Action, event.ID, event.Path, event.Detail
		if line.Time.IsZero() {
			line.Time = event.At
		}
		if event.Level != "" {
			line.Level = strings.ToLower(event.Level)
		}
		switch event.Action {
		case "retry", "blocked":
			line.Level = "error"
		case "refused", "skipped":
			line.Level = "warn"
		}
		return line
	}
	if len(text) >= 19 {
		line.Time, _ = time.ParseInLocation("2006/01/02 15:04:05", text[:19], time.Local)
	}
	if stamp, _, ok := strings.Cut(text, " "); ok && line.Time.IsZero() {
		line.Time, _ = time.Parse(time.RFC3339Nano, stamp)
	}
	// The hook's error-only log has no explicit level. Unlabelled daemon and
	// hook-audit messages remain info; do not guess a level from arbitrary paths.
	if source == "hook.log" {
		line.Level = "error"
	}
	for _, level := range []string{"DEBUG", "INFO", "WARN", "ERROR"} {
		if strings.Contains(text, "["+level+"]") {
			line.Level = strings.ToLower(level)
			break
		}
	}
	return line
}

func matchesLog(line LogLine, query LogQuery, from, to time.Time) bool {
	if !strings.Contains(line.Text, query.Text) || query.Level != "" && line.Level != query.Level {
		return false
	}
	if !from.IsZero() && (line.Time.IsZero() || line.Time.Before(from)) {
		return false
	}
	if !to.IsZero() && (line.Time.IsZero() || line.Time.After(to)) {
		return false
	}
	return true
}
