package gui

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

func FuzzLogs(f *testing.F) {
	for _, seed := range []string{
		"", "\n", "\r\n", "a\r\nb\nc\rd\n", "\xff\xe4\xb8\n\xf0\x9f",
		"2026/11/10 10:00:00 [WARN] warning\n2026-11-10T10:00:00Z [ERROR] error\n",
		"{\"time\":\"2026-11-10T10:00:00Z\",\"action\":\"retry\"}\n",
		"{\"at\":\"2026-11-10T10:00:00Z\",\"level\":\"DEBUG\"}\n",
		strings.Repeat("long", 20000),
	} {
		f.Add([]byte(seed), int32(3), uint8(0), "", "", "", "")
	}
	f.Add([]byte("\nlast\n"), int32(20000), uint8(255), "", "", "", "")
	f.Add([]byte("2026-11-10T10:00:00Z [WARN] warning\n"), int32(1), uint8(1), "WARN", "2026-11-10T10:00:00Z", "2026-11-10T10:00:00Z", "warning")
	home := f.TempDir()
	a := &API{deps: Dependencies{Service: &core.Service{Store: core.Store{Home: home}}}}
	f.Fuzz(func(t *testing.T, data []byte, limit int32, style uint8, level, from, to, text string) {
		source := []string{"hook.log", "obitd.log", "hook-audit.log", "audit.jsonl"}[style%4]
		query := LogQuery{Source: source, Lines: int(limit), Level: level, From: from, To: to, Text: text}
		bounds := query
		lower, upper, boundsErr := logBounds(&bounds)
		if style == 255 {
			// Expand a small corpus entry past the byte cap without an 8 MiB
			// corpus file; also exercise truncation in the middle of a line.
			data = append(bytes.Repeat([]byte{'x'}, (8<<20)+1), data...)
		}
		path := filepath.Join(home, source)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		page, err := a.Logs(query)
		if boundsErr != nil {
			if err == nil {
				t.Fatalf("invalid query accepted: %+v", query)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		lines, truncated := referenceLogTail(data, bounds.Lines)
		if page.ScannedLines != len(lines) || page.ScannedLines > bounds.Lines || page.Truncated != truncated {
			t.Fatalf("tail bounds: scanned=%d want=%d limit=%d truncated=%v want=%v", page.ScannedLines, len(lines), bounds.Lines, page.Truncated, truncated)
		}
		want := []LogLine{}
		for _, raw := range lines {
			line := parseLogLine(raw, source)
			if line.Text != raw {
				t.Fatal("parsing changed the original log bytes")
			}
			keep := strings.Contains(raw, text) && (bounds.Level == "" || bounds.Level == line.Level)
			if !lower.IsZero() {
				keep = keep && !line.Time.IsZero() && !line.Time.Before(lower)
			}
			if !upper.IsZero() {
				keep = keep && !line.Time.IsZero() && !line.Time.After(upper)
			}
			if keep {
				want = append(want, line)
			}
		}
		if !reflect.DeepEqual(page.Lines, want) {
			t.Fatal("tail content, order or level/time filtering changed")
		}
		size := 0
		for _, line := range page.Lines {
			size += len(line.Text)
		}
		if size > 8<<20 || len(page.Lines) > bounds.Lines {
			t.Fatalf("unbounded response: %d bytes / %d lines", size, len(page.Lines))
		}
	})
}

// This oracle splits a complete byte slice rather than reading backwards in
// chunks. Empty lines count; an unfinished UTF-8 sequence remains raw bytes.
func referenceLogTail(data []byte, limit int) ([]string, bool) {
	truncated := len(data) > 8<<20
	if truncated {
		data = data[len(data)-(8<<20):]
		if end := bytes.IndexByte(data, '\n'); end >= 0 {
			data = data[end+1:]
		} else {
			data = nil
		}
	}
	lines := []string{}
	if len(data) > 0 {
		lines = strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	}
	if len(lines) > limit {
		truncated = true
		lines = lines[len(lines)-limit:]
	}
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	return lines, truncated
}

func TestLogsTailTextTimeAndLevelFilters(t *testing.T) {
	a, _ := fixture(t)
	var lines strings.Builder
	for i := 0; i < 2010; i++ {
		fmt.Fprintf(&lines, "2026-11-10T10:00:00Z line %04d\n", i)
	}
	write(t, filepath.Join(a.deps.Service.Store.Home, "hook.log"), lines.String())
	page, err := a.Logs(LogQuery{Source: "hook.log"})
	if err != nil || len(page.Lines) != 2000 || !page.Truncated || !strings.HasSuffix(page.Lines[0].Text, "0010") || page.Lines[0].Level != "error" {
		t.Fatalf("tail: count=%d page=%+v err=%v", len(page.Lines), page, err)
	}
	page, err = a.Logs(LogQuery{Source: "hook.log", Lines: 20, Text: "2009", Level: "error", From: "2026-11-10T10:00:00Z", To: "2026-11-10T10:00:00Z"})
	if err != nil || len(page.Lines) != 1 || page.ScannedLines != 20 || page.Lines[0].Time.IsZero() {
		t.Fatalf("filter: %+v %v", page, err)
	}
	page, err = a.Logs(LogQuery{Source: "hook.log", Level: "info"})
	if err != nil || len(page.Lines) != 0 {
		t.Fatalf("level: %+v %v", page, err)
	}
	write(t, filepath.Join(a.deps.Service.Store.Home, "audit.jsonl"), "{\"time\":\"2026-11-10T10:00:00Z\",\"action\":\"retry\",\"id\":8,\"path\":\"sample\",\"detail\":\"busy\"}\n")
	page, err = a.Logs(LogQuery{Source: "audit.jsonl", Level: "error"})
	if err != nil || len(page.Lines) != 1 || page.Lines[0].Time.IsZero() {
		t.Fatalf("audit: %+v %v", page, err)
	}
	if l := page.Lines[0]; l.Action != "retry" || l.ID != 8 || l.Path != "sample" || l.Detail != "busy" || !strings.Contains(l.Text, `"action":"retry"`) {
		t.Fatalf("audit parts for a readable line: %+v", l)
	}
	for _, source := range []string{"obitd.log", "hook-audit.log"} {
		write(t, filepath.Join(a.deps.Service.Store.Home, source), "2026/11/10 10:00:00 [WARN] check\n")
		page, err = a.Logs(LogQuery{Source: source, Level: "warn"})
		if err != nil || len(page.Lines) != 1 || page.Lines[0].Time.IsZero() {
			t.Fatalf("%s: %+v %v", source, page, err)
		}
	}
}

func TestLogsRejectInvalidQueriesAndReportReadErrors(t *testing.T) {
	a, _ := fixture(t)
	page, err := a.Logs(LogQuery{Source: "obitd.log"})
	if err != nil || len(page.Lines) != 0 {
		t.Fatalf("missing log: %+v %v", page, err)
	}
	for _, q := range []LogQuery{
		{Source: "../store.json"}, {Source: "hook.log", Lines: -1}, {Source: "hook.log", Lines: 20001},
		{Source: "hook.log", Level: "wrong"}, {Source: "hook.log", From: "wrong"}, {Source: "hook.log", To: "wrong"},
		{Source: "hook.log", From: "2026-11-11T00:00:00Z", To: "2026-11-10T00:00:00Z"},
	} {
		if _, err := a.Logs(q); err == nil {
			t.Fatalf("accepted %+v", q)
		}
	}
	if err := os.MkdirAll(filepath.Join(a.deps.Service.Store.Home, "hook.log"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Logs(LogQuery{Source: "hook.log"}); err == nil {
		t.Fatal("read failure hidden")
	}
}

func TestLogReaderSharesRotationAndBoundsHugeLines(t *testing.T) {
	a, _ := fixture(t)
	path := filepath.Join(a.deps.Service.Store.Home, "hook.log")
	write(t, path, strings.Repeat("x", 9<<20)+"\n2026-11-10T00:00:00Z tail\n")
	page, err := a.Logs(LogQuery{Source: "hook.log"})
	if err != nil || len(page.Lines) != 1 || !page.Truncated || !strings.HasSuffix(page.Lines[0].Text, " tail") {
		t.Fatalf("bounded tail: %+v %v", page, err)
	}
	reader, err := openLog(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	writer, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("reader blocks writer: %v", err)
	}
	if _, err := writer.WriteString("new line\n"); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatalf("reader blocks rotation: %v", err)
	}
	write(t, path, "replacement\n")
	page, err = a.Logs(LogQuery{Source: "hook.log", Lines: 1})
	if err != nil || len(page.Lines) != 1 || page.Lines[0].Text != "replacement" {
		t.Fatalf("new log: %+v %v", page, err)
	}
	if matchesLog(LogLine{Text: "undated", Level: "info"}, LogQuery{}, time.Now(), time.Time{}) {
		t.Fatal("undated line passed time range")
	}
}
