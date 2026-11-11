package core_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
)

func TestWatchPolicyPersistenceApprovalAndRemoval(t *testing.T) {
	f := newFixture(t)
	f.svc.Owner = auto.Owner
	path := filepath.Dir(f.file("downloads/old.txt"))
	p, err := f.svc.SetWatch(path, "30d", core.WatchModified)
	if err != nil || p.Consent != "pending" {
		t.Fatalf("policy: %+v %v", p, err)
	}
	st, err := f.svc.Store.Read()
	if err != nil || st.Version != 15 || len(st.Proposals) != 1 || len(st.Items) != 0 {
		t.Fatalf("persisted policy: %+v %v", st, err)
	}
	e := &auto.Engine{Svc: f.svc}
	results, err := e.Approve([]string{fmt.Sprint(st.Proposals[0].ID)})
	if err != nil || results[0].Error != "" {
		t.Fatalf("approve: %+v %v", results, err)
	}
	policies, err := f.svc.Watches()
	if err != nil || len(policies) != 1 || policies[0].Consent != core.ConsentApproved {
		t.Fatalf("approved policy: %+v %v", policies, err)
	}
	if err := f.svc.RemoveWatch(path); err != nil {
		t.Fatal(err)
	}
	policies, err = f.svc.Watches()
	if err != nil || len(policies) != 0 || !exists(filepath.Join(path, "old.txt")) {
		t.Fatalf("remove: %+v %v", policies, err)
	}
}

func TestWatchPolicyChangedRevisionCannotUseOldApproval(t *testing.T) {
	f := newFixture(t)
	p := filepath.Dir(f.file("downloads/file"))
	if _, err := f.svc.SetWatch(p, "30d", core.WatchModified); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	old := st.Proposals[0]
	if _, err := f.svc.SetWatch(p, "1d", core.WatchModified); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ApproveWatch(old); err == nil {
		t.Fatal("stale approval enabled a changed policy")
	}
	st, err = f.svc.Store.Read()
	if err != nil || st.Watches[0].Consent != "pending" || len(st.Proposals) != 1 {
		t.Fatalf("changed policy: %+v %v", st, err)
	}
	if _, err := f.svc.Reject([]string{fmt.Sprint(st.Proposals[0].ID)}); err != nil {
		t.Fatal(err)
	}
	st, err = f.svc.Store.Read()
	if err != nil || st.Watches[0].Consent != core.ConsentDeclined || len(st.Proposals) != 0 {
		t.Fatalf("rejection: %+v %v", st, err)
	}
}

func TestWatchFirstSeenRequiresApproval(t *testing.T) {
	f := newFixture(t)
	f.svc.Owner = auto.Owner
	path := f.file("downloads/first.txt")
	p, err := f.svc.SetWatch(filepath.Dir(path), "2s", core.WatchFirstSeen)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.RegisterWatchChild(p, path); err == nil {
		t.Fatal("unapproved first-seen policy registered a child")
	}
	f.clock = f.clock.Add(3 * time.Second)
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.Read()
	if err != nil || p.Consent != "pending" || len(st.Proposals) != 1 || len(st.Items) != 0 || !exists(path) {
		t.Fatalf("first-seen policy bypassed approval: %+v %v", st, err)
	}
	if _, err := (&auto.Engine{Svc: f.svc}).Approve([]string{fmt.Sprint(st.Proposals[0].ID)}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.RegisterWatchChild(p, path); err != nil {
		t.Fatal(err)
	}
}

func TestWatchPolicyChangesRequireNewApproval(t *testing.T) {
	f := newFixture(t)
	path := filepath.Dir(f.file("downloads/first.txt"))
	p, err := f.svc.SetWatch(path, "30d", core.WatchModified)
	if err != nil {
		t.Fatal(err)
	}
	for _, settings := range [][2]string{{"30d", core.WatchFirstSeen}, {"2s", core.WatchFirstSeen}} {
		st, err := f.svc.Store.Read()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := (&auto.Engine{Svc: f.svc}).Approve([]string{fmt.Sprint(st.Proposals[0].ID)}); err != nil {
			t.Fatal(err)
		}
		p, err = f.svc.SetWatch(path, settings[0], settings[1])
		if err != nil || p.Consent != "pending" {
			t.Fatalf("changed policy bypassed approval: %+v %v", p, err)
		}
	}
	st, _ := f.svc.Store.Read()
	again, err := f.svc.SetWatch(path, p.Lifespan, p.Age)
	after, _ := f.svc.Store.Read()
	if err != nil || again.Rev != p.Rev || after.Proposals[0].ID != st.Proposals[0].ID {
		t.Fatalf("unchanged policy replaced pending decision: %+v %v", again, err)
	}
}

func TestWatchFolderProtectedFromManualBurialAndLifespan(t *testing.T) {
	for _, action := range []string{"bury", "mark"} {
		t.Run(action, func(t *testing.T) {
			f := newFixture(t)
			path := filepath.Dir(f.file("downloads/keep.txt"))
			if _, err := f.svc.SetWatch(path, "30d", core.WatchFirstSeen); err != nil {
				t.Fatal(err)
			}
			if action == "bury" {
				results, err := f.svc.Bury([]string{path}, false, "")
				if err != nil || len(results) != 1 || results[0].Action != core.ActionRefused {
					t.Fatalf("watched folder must be refused: %+v %v", results, err)
				}
			} else {
				results, err := f.svc.Add([]string{path}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour), Force: true})
				if err != nil || !core.Failed(results) {
					t.Fatalf("watched folder must not get a lifespan: %+v %v", results, err)
				}
			}
			if _, err := os.Stat(filepath.Join(path, "keep.txt")); err != nil {
				t.Fatal(err)
			}
		})
	}
}
