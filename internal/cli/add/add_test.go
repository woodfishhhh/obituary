package add

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
)

func TestAddMigrationWarningStillPrintsResultAndStartsRunner(t *testing.T) {
	starts := clitest.IsolateRunner(t)
	repo, store := clitest.CommandDir(t)
	oldPath, path := filepath.Join(repo, "old.log"), filepath.Join(repo, "new.log")
	clitest.WriteFixture(t, oldPath, "old")
	clitest.WriteFixture(t, path, "new")
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	added, err := svc.Add([]string{oldPath}, core.AddOptions{ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil || core.Failed(added) {
		t.Fatal(err)
	}
	first, second := *added[0].Item, *added[0].Item
	second.ID = 7
	data, err := json.Marshal(core.State{Version: 4, Items: []core.Item{first, second}})
	if err != nil {
		t.Fatal(err)
	}
	clitest.WriteFixture(t, filepath.Join(store.Home, "store.json"), string(data))
	audit := filepath.Join(store.Home, "audit.jsonl")
	if err := os.Rename(audit, audit+".saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(audit, 0o700); err != nil {
		t.Fatal(err)
	}
	out, stderr, err := clitest.Execute(t, NewCmd(), path, "1d", "--json")
	var rows []core.Result
	if err != nil || json.Unmarshal([]byte(out), &rows) != nil || len(rows) != 1 || rows[0].Item == nil || rows[0].Item.Path != path || len(*starts) != 1 || !strings.Contains(stderr, "warning:") {
		t.Fatalf("committed result/runner lost: %s %s, %v; starts=%v", out, stderr, err, *starts)
	}
}
