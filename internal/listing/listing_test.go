package listing_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/listing"
)

func write(t *testing.T, items []ledger.Item, opt listing.Options) string {
	t.Helper()
	var out bytes.Buffer
	if err := listing.Write(&out, items, time.Now(), opt); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestWriteTableNotesAndLanguages(t *testing.T) {
	now := time.Now()
	items := []ledger.Item{
		{ID: 1, Path: "kept.txt", ExpiresAt: now.Add(48 * time.Hour), Note: "scratch"},
		{ID: 7, Path: "locked", ExpiresAt: now.Add(-time.Hour), Attempts: 12, NextAttempt: ledger.RetryStopped, LastError: "in use"},
		{ID: 9, Source: "watch", Path: "watched", ExpiresAt: now.Add(time.Hour), Watch: &ledger.WatchTiming{StableSince: now}},
	}
	en := write(t, items, listing.Options{Language: i18n.English, StableWindow: 30 * time.Minute})
	for _, want := range []string{"#1", "kept.txt", "scratch", "stopped retrying after 12 attempts: in use", "obit extend #7 1m", "stable_window=30m0s"} {
		if !strings.Contains(en, want) {
			t.Errorf("missing %q in\n%s", want, en)
		}
	}
	if zh := write(t, items, listing.Options{Language: i18n.Chinese}); zh == en || !strings.Contains(zh, "kept.txt") {
		t.Errorf("the language must change the text, not the rows:\n%s", zh)
	}
}

func TestWriteEmptyAndJSON(t *testing.T) {
	if out := write(t, nil, listing.Options{Language: i18n.English}); strings.Count(out, "\n") != 1 {
		t.Fatalf("an empty ledger prints one line: %q", out)
	}
	now := time.Now()
	out := write(t, []ledger.Item{{ID: 9, Source: "watch", Path: "watched", ExpiresAt: now, Watch: &ledger.WatchTiming{StableSince: now}}},
		listing.Options{JSON: true, StableWindow: 30 * time.Minute})
	for _, want := range []string{`"id": 9`, `"stableWindow": "30m0s"`, `"quietFor"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if write(t, nil, listing.Options{JSON: true}) != "[]\n" {
		t.Error("an empty JSON list is []")
	}
}
