package guardrm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

func TestConcurrentAuditCounts(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	var wg sync.WaitGroup
	for range 24 {
		wg.Go(func() {
			if err := Record(home, AuditEvent{Kind: "denied", Command: "rm x", Equivalent: "obit rm x"}); err != nil {
				t.Error(err)
			}
			if err := Record(home, AuditEvent{Kind: "buried", Command: "rm", Count: 2}); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	counts, err := ReadCounts(home)
	if err != nil || counts.Denied != 24 || counts.Buried != 48 {
		t.Fatalf("lost concurrent records: %+v %v", counts, err)
	}
}

func TestAuditCommandByteLimit(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"rm x", strings.Repeat("&", 8*1024), strings.Repeat("&", 8*1024+1), strings.Repeat("字", 3000)} {
		home := t.TempDir()
		if err := Record(home, AuditEvent{Kind: "denied", Command: command, Equivalent: command}); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(home, "delete-guard.jsonl"))
		var got AuditEvent
		if err != nil || json.Unmarshal(data, &got) != nil {
			t.Fatalf("invalid receipt: %v", err)
		}
		for _, field := range []string{got.Command, got.Equivalent} {
			if len(command) <= 8*1024 {
				if field != command {
					t.Errorf("short command changed (%d bytes)", len(command))
				}
			} else if len(field) > 8*1024 || !strings.HasSuffix(field, " [truncated]") || !strings.HasPrefix(command, strings.TrimSuffix(field, " [truncated]")) || !utf8.ValidString(field) || strings.ContainsRune(field, utf8.RuneError) {
				t.Errorf("invalid truncated command: %d bytes", len(field))
			}
		}
	}
}
