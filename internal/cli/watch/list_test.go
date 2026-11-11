package watch

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

func TestWatchListShowsExemptionsAndStability(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OBIT_HOME", filepath.Join(root, "home"))
	t.Setenv("OBIT_CONFIG_DIR", filepath.Join(root, "config"))
	clitest.WriteFixture(t, filepath.Join(root, "config", "config.toml"), "[watch]\nstable_window = \"1h\"\n")
	path := filepath.Join(root, "downloads", "file.txt")
	clitest.WriteFixture(t, path, "keep")
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	p, err := svc.SetWatch(filepath.Dir(path), "1s", core.WatchFirstSeen)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	if err := svc.Store.Update(func(st *ledger.State) error {
		for _, reason := range []string{"rejected", "restored", "spared"} {
			st.WatchExemptions = append(st.WatchExemptions, ledger.WatchExemption{WatchID: p.ID, Path: path, Reason: reason, KeptAt: now})
		}
		st.Items = []ledger.Item{{ID: st.NextID, Source: "watch", WatchID: p.ID, Path: path, ExpiresAt: now, Watch: &ledger.WatchTiming{StableSince: now}}}
		st.NextID++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	out, _, err := clitest.Execute(t, NewCmd(), "--list")
	for _, want := range []string{path, "rejected", "restored", "spared", now.Format(time.RFC3339), "quiet for", "stable_window=1h0m0s", "next check"} {
		if err != nil || !strings.Contains(out, want) {
			t.Fatalf("missing %q: %s %v", want, out, err)
		}
	}
	out, _, err = clitest.Execute(t, NewCmd(), "--list", "--json")
	var rows []listing
	if err != nil || json.Unmarshal([]byte(out), &rows) != nil || len(rows) != 1 || len(rows[0].Exemptions) != 3 || len(rows[0].Stability) != 1 {
		t.Fatalf("JSON detail: %s %v", out, err)
	}
}
