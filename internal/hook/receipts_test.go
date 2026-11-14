package hook

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestReceiptNoticeSaysWhatAndHowToUndo(t *testing.T) {
	now := time.Now()
	repo, other := filepath.Join(t.TempDir(), "repo"), filepath.Join(t.TempDir(), "other")
	st := core.State{Items: []core.Item{
		{ID: 3, Path: filepath.Join(repo, "debug.log"), ExpiresAt: now.Add(72 * time.Hour)},
		{ID: 4, Path: filepath.Join(other, "dist"), ExpiresAt: now.Add(time.Hour)},
	}}
	st.AutoReceipts = []core.AutoReceipt{
		{ID: 3, Class: core.AutoCreated, Project: repo, Session: "s"},
		{ID: 4, Class: core.AutoRegenerable, Project: other, Session: "elsewhere"},
		{ID: 9, Class: core.AutoAll, Project: repo}, // its item is gone
	}
	handled := untoldReceipts(st, repo, "s", true)
	if len(handled) != 1 || handled[0].Item.ID != 3 {
		t.Fatalf("receipts for this project or session only: %+v", handled)
	}
	st.AutoReceipts[0].Told = true
	if again := untoldReceipts(st, repo, "s", true); len(again) != 0 {
		t.Fatalf("a told receipt is not repeated: %+v", again)
	}
	text := describeReceipts(handled, repo, now)
	for _, want := range []string{"#3 debug.log", "created in its session", "expires in 3d", "obit spare <id>", "obit extend <id> <lifespan>", "obit resurrect <id>"} {
		if !strings.Contains(text, want) {
			t.Fatalf("notice lacks %q: %s", want, text)
		}
	}
}
