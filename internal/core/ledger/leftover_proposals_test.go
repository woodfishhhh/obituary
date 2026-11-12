package ledger_test

import (
	"path/filepath"
	"testing"

	core "github.com/woodfishhhh/obituary/internal/core/ledger"
)

func TestLeftoverPathRejectionScope(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "drafts", "research.md")
	st := core.State{LeftoverRejections: []core.LeftoverRejection{{Project: root, Session: "first", Paths: []string{path}}}}
	for _, rejected := range []string{path, filepath.Dir(path), filepath.Join(path, "child")} {
		if !st.LeftoverPathRejected(root, rejected) {
			t.Fatalf("file decision lost across sessions: %s", rejected)
		}
	}
	if st.LeftoverPathRejected(root, path+".other") || st.LeftoverPathRejected(root+"-other", path) {
		t.Fatal("file rejection crossed path or project boundary")
	}
	if st.LeftoverRejected(root, "second", path) {
		t.Fatal("session-scoped rejection lookup changed")
	}
}
