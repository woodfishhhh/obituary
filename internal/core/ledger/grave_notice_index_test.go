package ledger_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/ledger"
)

func TestGraveNoticeIndexNegativeFilter(t *testing.T) {
	s := core.Store{Home: t.TempDir()}
	path := filepath.Join(s.Home, "old")
	if s.MightHaveBurial(path) {
		t.Fatal("missing index matched")
	}
	if err := s.Update(func(st *core.State) error {
		st.Burials = []core.Burial{{ID: 1, Path: path, IsDir: true, PurgeAt: time.Now().Add(time.Hour)}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path string
		want bool
	}{{path, true}, {filepath.Join(path, "child"), true}, {path + "-sibling", false}, {filepath.Join(s.Home, "OLD"), false}} {
		if got := s.MightHaveBurial(tc.path); got != tc.want {
			t.Fatalf("%s: %v", tc.path, got)
		}
	}
	before, _ := os.ReadFile(filepath.Join(s.Home, "store.json"))
	s.RefreshGraveNoticeIndex()
	after, _ := os.ReadFile(filepath.Join(s.Home, "store.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("index refresh changed ledger")
	}
	if err := s.Update(func(st *core.State) error { st.Burials = nil; return nil }); err != nil {
		t.Fatal(err)
	}
	if s.MightHaveBurial(path) {
		t.Fatal("removed record retained")
	}
}

func TestGraveNoticeIndexFailurePreservesCommit(t *testing.T) {
	s := core.Store{Home: t.TempDir()}
	if err := os.Mkdir(filepath.Join(s.Home, "grave-notice-paths.json"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := s.Update(func(st *core.State) error {
		st.Burials = []core.Burial{{ID: 1, Path: filepath.Join(s.Home, "old")}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	st, err := s.ReadSnapshot()
	if err != nil || len(st.Burials) != 1 {
		t.Fatalf("advisory cache affected commit: %+v %v", st, err)
	}
}
