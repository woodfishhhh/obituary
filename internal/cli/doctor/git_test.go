package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestDoctorSlowGit(t *testing.T) {
	now := time.Now().UTC()
	timeout := "could not ask Git whether it is tracked: git ls-files: context deadline exceeded"
	for _, count := range []int{0, 1, 2, 3} {
		home := t.TempDir()
		var lines []byte
		for i := 0; i < count; i++ {
			data, err := json.Marshal(core.Event{Time: now.Add(-time.Minute).Add(time.Duration(i) * time.Second), Action: core.ActionRetry, ID: 7, Detail: timeout})
			if err != nil {
				t.Fatal(err)
			}
			lines = append(lines, append(data, '\n')...)
		}
		path := filepath.Join(home, "audit.jsonl.1")
		if err := os.WriteFile(path, lines, 0600); err != nil {
			t.Fatal(err)
		}
		got := diagnoseSlowGit(home, now)
		if got.Errors != count || (got.Status == "WARN") != (count >= 3) {
			t.Fatalf("doctor hid repeated slow Git: count=%d %+v", count, got)
		}
		if count >= 3 && (!strings.Contains(got.Detail, "latest #7: "+timeout) || !strings.Contains(got.Detail, "kept files in place")) {
			t.Fatalf("doctor lost slow Git reason: %+v", got)
		}
		after, err := os.ReadFile(path)
		if err != nil || string(after) != string(lines) {
			t.Fatal("doctor changed audit evidence")
		}
	}
}

func TestDoctorSlowGitDeduplicatesEventsAcrossRotation(t *testing.T) {
	now := time.Now().UTC()
	base := core.Event{Time: now.Add(-time.Minute), Action: core.ActionRetry, ID: 7, Path: "keep.log", Detail: "could not ask Git whether it is tracked: context deadline exceeded"}
	for _, distinct := range []string{"none", "time", "id", "path", "detail"} {
		t.Run(distinct, func(t *testing.T) {
			home := t.TempDir()
			other := base
			switch distinct {
			case "time":
				other.Time = other.Time.Add(time.Second)
			case "id":
				other.ID++
			case "path":
				other.Path = "other.log"
			case "detail":
				other.Detail = "final Git verification budget exhausted: context deadline exceeded"
			}
			for name, events := range map[string][]core.Event{"audit.jsonl.1": {base, base}, "audit.jsonl": {base, other, other}} {
				var lines []byte
				for _, event := range events {
					data, err := json.Marshal(event)
					if err != nil {
						t.Fatal(err)
					}
					lines = append(lines, append(data, '\n')...)
				}
				if err := os.WriteFile(filepath.Join(home, name), lines, 0600); err != nil {
					t.Fatal(err)
				}
			}
			want := 2
			if distinct == "none" {
				want = 1
			}
			if got := diagnoseSlowGit(home, now); got.Errors != want || got.Status != "PASS" {
				t.Fatalf("duplicate audit deliveries inflated timeouts: want=%d %+v", want, got)
			}
		})
	}
}

func TestGitTimeoutRetry(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		action, detail string
		age            time.Duration
		want           bool
	}{
		{core.ActionRetry, "could not ask Git whether it is tracked: context deadline exceeded", 0, true},
		{core.ActionRetry, "final Git verification budget exhausted: context deadline exceeded", 24 * time.Hour, true},
		{core.ActionRetry, "could not ask Git whether it is tracked: context deadline exceeded", 25 * time.Hour, false},
		{core.ActionRetry, "could not ask Git whether it is tracked: context deadline exceeded", -time.Second, false},
		{core.ActionRetry, "could not ask Git whether it is tracked: context canceled", 0, false},
		{core.ActionRetry, "could not ask Git whether it is tracked: access denied", 0, false},
		{core.ActionRetry, "Guard: context deadline exceeded", 0, false},
		{core.ActionBuried, "could not ask Git whether it is tracked: context deadline exceeded", 0, false},
	} {
		if got := gitTimeoutRetry(core.Event{Time: now.Add(-tc.age), Action: tc.action, Detail: tc.detail}, now); got != tc.want {
			t.Errorf("Git timeout classification: %+v got=%v", tc, got)
		}
	}
}
