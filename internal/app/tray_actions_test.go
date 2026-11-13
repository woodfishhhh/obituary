//go:build windows

package app

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func TestTrayRecentIsBoundedAndUpcomingIsOrdered(t *testing.T) {
	now := time.Now()
	svc := &core.Service{Store: core.Store{Home: t.TempDir()}}
	if err := svc.Store.Update(func(st *core.State) error {
		for i := 1; i <= 12; i++ {
			st.Burials = append(st.Burials, core.Burial{ID: int64(i), Path: fmt.Sprint(i), BuriedAt: now.Add(time.Duration(i) * time.Minute), Reason: core.ReasonExpired})
		}
		st.Items = []core.Item{{ID: 20, ExpiresAt: now.Add(25 * time.Hour)}, {ID: 21, ExpiresAt: now.Add(time.Hour)}, {ID: 22, ExpiresAt: now.Add(-time.Hour)}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	r := resident(svc, log.New(io.Discard, "", 0))
	recent, err := r.RecentBurials()
	if err != nil || len(recent) != 10 || recent[0].ID != 12 || recent[9].ID != 3 {
		t.Fatalf("recent: %v %v", recent, err)
	}
	items, err := r.Upcoming(now)
	if err != nil || len(items) != 2 || items[0].ID != 22 || items[1].ID != 21 {
		t.Fatalf("upcoming: %v %v", items, err)
	}
	if due, graves, err := r.Counts(now); err != nil || due != 2 || graves != 12 {
		t.Fatalf("counts: %d %d %v", due, graves, err)
	}
}

func TestTrayRestoreOnlySelectedIDAndPreservesConflict(t *testing.T) {
	root := t.TempDir()
	svc := &core.Service{Store: core.Store{Home: filepath.Join(root, "data")}, FS: winfs.FS{}}
	paths := []string{filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt")}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte(path), 0600); err != nil {
			t.Fatal(err)
		}
	}
	buried, err := svc.Bury(paths, false, "")
	if err != nil || len(buried) != 2 {
		t.Fatalf("bury: %v %v", buried, err)
	}
	r := resident(svc, log.New(io.Discard, "", 0))
	if err := os.WriteFile(paths[1], []byte("new occupant"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(buried[0].Burial.Grave), "unexpected.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Restore(buried[0].Burial.ID); err == nil {
		t.Fatal("unsafe grave reported success")
	}
	restored, err := r.Restore(buried[1].Burial.ID)
	if err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(paths[1])
	if err != nil || string(first) != "new occupant" {
		t.Fatalf("overwrote conflict: %s %v", first, err)
	}
	second, err := os.ReadFile(restored.Path)
	if err != nil || string(second) != paths[1] || restored.Path == paths[1] {
		t.Fatalf("wrong restored content: %s %v", second, err)
	}
	state, err := svc.Store.Read()
	if err != nil || len(state.Burials) != 1 || state.Burials[0].ID != buried[0].Burial.ID {
		t.Fatalf("restored unselected batch member: %+v %v", state.Burials, err)
	}
}
