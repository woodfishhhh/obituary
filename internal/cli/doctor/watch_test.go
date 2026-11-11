package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func TestDoctorWatches(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, consent, status string
		pathErr               error
	}{
		{"approved", core.ConsentApproved, "PASS", nil},
		{"pending", "pending", "WARN", nil},
		{"missing", core.ConsentApproved, "WARN", core.ErrNotFound},
		{"changed", core.ConsentApproved, "WARN", core.ErrUnsafe},
		{"unreadable", core.ConsentApproved, "WARN", os.ErrPermission},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := ledger.State{Watches: []ledger.WatchPolicy{{ID: 1, Path: `C:\downloads`, Consent: tc.consent,
				Exclusions: []core.WatchExclusion{{ID: 2}}}}, WatchExemptions: []ledger.WatchExemption{{WatchID: 1}}}
			calls := 0
			folder := t.TempDir()
			got := diagnoseWatches(state, nil, now, func(policy ledger.WatchPolicy) (string, error) {
				calls++
				return folder, tc.pathErr
			})
			if !strings.Contains(got.Detail, "1 folder policy,") || !strings.Contains(got.Detail, "1 exemption,") || !strings.Contains(got.Detail, "obit watch --list shows exemptions") {
				t.Fatal(got.Detail)
			}
			w := got.Watch
			if got.Status != tc.status || calls != 1 || w.Policies != 1 || w.Exclusions != 1 || w.Exemptions != 1 || w.Unstable != 0 {
				t.Fatalf("folder policy diagnosis: %+v %+v", got, w)
			}
			if (w.Pending == 1) != (tc.consent == "pending") || (w.Unavailable == 1) != (tc.pathErr != nil) {
				t.Fatalf("policy counts: %+v", w)
			}
			if tc.pathErr != nil && (!strings.Contains(got.Detail, tc.pathErr.Error()) || w.FirstUnavailable != `C:\downloads`) {
				t.Fatalf("missing folder failure reason: %+v", got)
			}
		})
	}
	got := diagnoseWatches(ledger.State{}, errors.New("bad store"), now, nil)
	if got.Status != "FAIL" || !strings.Contains(got.Detail, "bad store") {
		t.Fatal(got)
	}
}

func TestDoctorWatchStability(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name     string
		age      time.Duration
		stable   time.Time
		lastErr  string
		source   string
		unstable bool
	}{
		{"old active", 25 * time.Hour, now, "", "watch", true},
		{"boundary age", 24 * time.Hour, now, "", "watch", false},
		{"new active", time.Hour, now, "", "watch", false},
		{"old stable", 48 * time.Hour, now.Add(-24 * time.Hour), "", "watch", false},
		{"stable window elapsed", 25 * time.Hour, now.Add(-ledger.WatchStableWindow), "", "watch", false},
		{"clock in future", 25 * time.Hour, now.Add(time.Hour), "", "watch", false},
		{"legacy unknown", 25 * time.Hour, time.Time{}, "", "watch", false},
		{"unfinished download", 25 * time.Hour, time.Time{}, "download temporary name is still present: part", "watch", true},
		{"saved active retry", 25 * time.Hour, now.Add(-time.Hour), "folder child is still active or its modified deadline was extended", "watch", true},
		{"unrelated retry", 25 * time.Hour, now.Add(-time.Hour), "access denied", "watch", false},
		{"manual item", 25 * time.Hour, now, "", "cli", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			it := ledger.Item{ID: 1, Path: `C:\downloads\file.zip`, CreatedAt: now.Add(-tc.age), Source: tc.source,
				LastError: tc.lastErr, Watch: &ledger.WatchTiming{StableSince: tc.stable}}
			got := diagnoseWatches(ledger.State{Items: []ledger.Item{it}}, nil, now, nil)
			if (got.Status == "WARN") != tc.unstable || (got.Watch.Unstable == 1) != tc.unstable {
				t.Fatalf("stability diagnosis: %+v %+v", got, got.Watch)
			}
			if tc.unstable && (got.Watch.FirstUnstable != it.Path || !strings.Contains(got.Detail, "unfinished downloads")) {
				t.Fatalf("missing download advice: %+v", got)
			}
		})
	}
}

func TestDoctorWatchIdentityReadOnly(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "watched")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	fs := winfs.FS{}
	id, err := fs.Identify(path)
	if err != nil {
		t.Fatal(err)
	}
	state := ledger.State{Watches: []ledger.WatchPolicy{{ID: 1, Path: path, Identity: id, Consent: core.ConsentApproved}}}
	svc := &core.Service{FS: fs}
	for _, status := range []string{"PASS", "WARN"} {
		before := clitest.TreeSnapshot(t, root)
		got := diagnoseWatches(state, nil, time.Now(), svc.WatchPath)
		if got.Status != status || !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
			t.Fatalf("identity check changed files or missed replacement: %+v", got)
		}
		if status == "PASS" {
			if err := os.Rename(path, path+"-old"); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestDoctorExplainsDownloadSkips(t *testing.T) {
	path := t.TempDir()
	clitest.WriteFixture(t, filepath.Join(path, "archive.tmp"), "ordinary archive")
	id, err := (winfs.FS{}).Identify(path)
	if err != nil {
		t.Fatal(err)
	}
	st := ledger.State{Watches: []ledger.WatchPolicy{{ID: 1, Path: path, Identity: id, Consent: core.ConsentApproved}}}
	svc := &core.Service{FS: winfs.FS{}}
	got := diagnoseWatches(st, nil, time.Now(), svc.WatchPath)
	if !strings.Contains(got.Detail, "archive.tmp") || !strings.Contains(got.Detail, "obit add") || !strings.Contains(got.Detail, "download") {
		t.Fatalf("doctor omitted download skip explanation: %+v", got)
	}
}
