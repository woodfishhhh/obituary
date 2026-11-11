package list

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func TestListShowsConfiguredQuietWindow(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OBIT_HOME", filepath.Join(root, "data"))
	t.Setenv("OBIT_CONFIG_DIR", filepath.Join(root, "config"))
	clitest.WriteFixture(t, filepath.Join(root, "config", "config.toml"), "[watch]\nstable_window = \"30m\"\n")
	now := time.Now()
	store := ledger.Store{Home: os.Getenv("OBIT_HOME")}
	if err := store.Update(func(st *ledger.State) error {
		st.Items = []ledger.Item{{ID: 1, Source: "watch", Path: filepath.Join(root, "active.txt"), ExpiresAt: now.Add(-time.Minute), Attempts: 1, LastError: "folder child is still active", Watch: &ledger.WatchTiming{StableSince: now}}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, asJSON := range []bool{false, true} {
		args := []string{}
		want := []string{"quiet for", "stable_window=30m0s", "next check"}
		if asJSON {
			args = append(args, "--json")
			want = []string{"\"quietFor\"", "\"stableWindow\": \"30m0s\"", "\"nextCheck\""}
		}
		out, _, err := clitest.Execute(t, NewCmd(), args...)
		for _, text := range want {
			if err != nil || !strings.Contains(out, text) {
				t.Fatalf("missing %q: %s %v", text, out, err)
			}
		}
	}
}

// Measure the real CLI, including process startup and text formatting, with
// both collections populated. list only reads the ledger, not the files.
func BenchmarkList(b *testing.B) {
	base := b.TempDir()
	exe := filepath.Join(base, "obit.exe")
	build := exec.Command("go", "build", "-o", exe, "./cmd/obit")
	build.Dir = "../../.."
	if out, err := build.CombinedOutput(); err != nil {
		b.Fatal(string(out), err)
	}
	store := ledger.Store{Home: filepath.Join(base, "data")}
	now := time.Now()
	if err := store.Update(func(st *ledger.State) error {
		for i := range 1000 {
			path := filepath.Join(base, fmt.Sprintf("live-%04d", i))
			grave := filepath.Join(store.Home, "graveyard", fmt.Sprintf("%d-%s-abcdef", 1001+i, now.UTC().Format("20060102T150405Z")), "buried")
			if err := os.MkdirAll(filepath.Dir(grave), 0o700); err != nil {
				return err
			}
			for _, p := range []string{path, grave} {
				if err := os.WriteFile(p, nil, 0o600); err != nil {
					return err
				}
			}
			id, err := (winfs.FS{}).Identify(path)
			if err != nil {
				return err
			}
			objectID, err := (winfs.FS{}).Identify(grave)
			if err != nil {
				return err
			}
			graveID, err := (winfs.FS{}).Identify(filepath.Dir(grave))
			if err != nil {
				return err
			}
			st.Items = append(st.Items, ledger.Item{ID: int64(i + 1), Rev: 1, Path: path, Identity: &id, ExpiresAt: now.Add(time.Hour)})
			st.Burials = append(st.Burials, ledger.Burial{ID: int64(1001 + i), Path: filepath.Join(base, fmt.Sprintf("old-%04d", i)), Grave: grave, Identity: &objectID, GraveID: &graveID, BuriedAt: now, PurgeAt: now.Add(24 * time.Hour), Reason: core.ReasonBury})
		}
		st.NextID = 2001
		return nil
	}); err != nil {
		b.Fatal(err)
	}
	env := append(os.Environ(), "OBIT_HOME="+store.Home, "OBIT_CONFIG_DIR="+filepath.Join(base, "config"))
	var samples []float64
	var output []byte
	for b.Loop() {
		start := time.Now()
		cmd := exec.CommandContext(b.Context(), exe, "list")
		cmd.Env = env
		var err error
		output, err = cmd.Output()
		samples = append(samples, float64(time.Since(start))/float64(time.Millisecond))
		if err != nil {
			b.Fatal(err)
		}
	}
	if lines := strings.Split(strings.TrimSpace(string(output)), "\n"); len(lines) != 1001 || !strings.Contains(string(output), "live-0999") {
		b.Fatalf("list lost tracked rows: %d lines", len(lines))
	}
	b.Logf("samples_ms=%v", samples)
	slices.Sort(samples)
	b.ReportMetric((samples[(len(samples)-1)/2]+samples[len(samples)/2])/2, "p50-ms")
	b.ReportMetric(samples[(95*len(samples)+99)/100-1], "p95-ms")
}
