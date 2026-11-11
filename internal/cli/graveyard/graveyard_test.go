//go:build windows

package graveyard

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	daemoncmd "github.com/woodfishhhh/obituary/internal/cli/daemon"
	mcpcmd "github.com/woodfishhhh/obituary/internal/cli/mcp"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func TestGraveyardHereTextAndJSON(t *testing.T) {
	clitest.IsolateRunner(t)
	repo, _ := clitest.CommandDir(t)
	paths := []string{filepath.Join(repo, "a", "one.txt"), filepath.Join(repo, "a", "sub", "two.txt"), filepath.Join(repo, "ab", "outside.txt")}
	for _, path := range paths {
		clitest.WriteFixture(t, path, "keep")
	}
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := svc.Bury(paths, false, "")
	if err != nil || core.GraveFailed(rows) {
		t.Fatal(rows, err)
	}
	t.Chdir(filepath.Join(repo, "a"))
	for _, asJSON := range []bool{false, true} {
		args := []string{"--here"}
		if asJSON {
			args = append(args, "--json")
		}
		out, stderr, err := clitest.Execute(t, NewCmd(), args...)
		if err != nil {
			t.Fatal(out, stderr, err)
		}
		if asJSON {
			var listing GraveyardListing
			if err := json.Unmarshal([]byte(out), &listing); err != nil || len(listing.Burials) != 2 || listing.PendingCount != 0 {
				t.Fatal(out, err)
			}
		}
		if !strings.Contains(out, "one.txt") || !strings.Contains(out, "two.txt") || strings.Contains(out, "outside.txt") {
			t.Fatalf("--here scope not applied: %s", out)
		}
	}
}

func TestPurgeInProgressPresentation(t *testing.T) {
	svc, b := clitest.PurgingCommandFixture(t)
	t.Run("text", func(t *testing.T) {
		out, _, err := clitest.Execute(t, NewCmd())
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"purging", "retry at " + b.NextPurgeAttempt.Local().Format(cmdutil.TimeLayout), b.LastPurgeError, "pending recovery: 1"} {
			if !strings.Contains(out, want) {
				t.Fatalf("purge in progress hidden from text listing: missing %q in %s", want, out)
			}
		}
	})
	for _, mode := range []string{"json", "mcp"} {
		t.Run(mode, func(t *testing.T) {
			var listing GraveyardListing
			if mode == "json" {
				out, _, err := clitest.Execute(t, NewCmd(), "--json")
				if err != nil || json.Unmarshal([]byte(out), &listing) != nil {
					t.Fatal(out, err)
				}
			} else {
				clitest.CallMCP(t, clitest.ConnectMCP(t, mcpcmd.NewServer(svc, "0.1.0-dev")), "obit_graveyard", map[string]any{}, false, &listing)
			}
			if len(listing.Pending) != 1 || listing.Pending[0].Status != "purging" || listing.Pending[0].Burial == nil || listing.Pending[0].Burial.PurgeIntent == nil || !listing.Pending[0].Burial.NextPurgeAttempt.Equal(b.NextPurgeAttempt) || listing.Pending[0].Error != b.LastPurgeError {
				t.Fatalf("purge in progress hidden from structured listing: %+v", listing)
			}
		})
	}
}

func BenchmarkGraveyardJSON(b *testing.B) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		b.Fatal(err)
	}
	exe := filepath.Join(b.TempDir(), "obit.exe")
	args := []string{"build", "-o", exe}
	if overlay := os.Getenv("OBIT_BENCH_OVERLAY"); overlay != "" {
		args = append(args, "-overlay", overlay)
	}
	args = append(args, "./cmd/obit")
	build := exec.Command("go", args...)
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		b.Fatal(string(out), err)
	}
	for _, n := range []int{1000, 5000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			base := b.TempDir()
			store := core.Store{Home: filepath.Join(base, "home")}
			fs := winfs.FS{}
			now := time.Now()
			st := core.State{NextID: int64(2*n + 1)}
			for i := range n {
				path := filepath.Join(base, fmt.Sprintf("live-%d", i))
				if err := os.WriteFile(path, nil, 0600); err != nil {
					b.Fatal(err)
				}
				id, err := fs.Identify(path)
				if err != nil {
					b.Fatal(err)
				}
				st.Items = append(st.Items, core.Item{ID: int64(i + 1), Rev: 1, Path: path, Identity: &id, ExpiresAt: now.Add(time.Hour)})
				grave := filepath.Join(store.Home, "graveyard", fmt.Sprintf("%d-%s-abcdef", n+i+1, now.UTC().Format("20060102T150405Z")), "buried")
				if err := os.MkdirAll(filepath.Dir(grave), 0700); err != nil {
					b.Fatal(err)
				}
				if err := os.WriteFile(grave, nil, 0600); err != nil {
					b.Fatal(err)
				}
				graveID, err := fs.Identify(filepath.Dir(grave))
				if err != nil {
					b.Fatal(err)
				}
				objectID, err := fs.Identify(grave)
				if err != nil {
					b.Fatal(err)
				}
				st.Burials = append(st.Burials, core.Burial{ID: int64(n + i + 1), Path: filepath.Join(base, fmt.Sprintf("old-%d", i), "buried"), Grave: grave, Identity: &objectID, GraveID: &graveID, BuriedAt: now, PurgeAt: now.Add(24 * time.Hour), Reason: core.ReasonBury})
			}
			if err := store.Update(func(current *core.State) error { *current = st; return nil }); err != nil {
				b.Fatal(err)
			}
			var output []byte
			var samples []float64
			b.ResetTimer()
			for b.Loop() {
				timeout := 15 * time.Minute
				if limit := os.Getenv("OBIT_BENCH_COMMAND_TIMEOUT"); limit != "" {
					var err error
					timeout, err = time.ParseDuration(limit)
					if err != nil {
						b.Fatal(err)
					}
				}
				ctx, cancel := context.WithTimeout(b.Context(), timeout)
				start := time.Now()
				cmd := exec.CommandContext(ctx, exe, "graveyard", "--json")
				cmd.Env = append(os.Environ(), "OBIT_HOME="+store.Home, "OBIT_CONFIG_DIR="+filepath.Join(base, "config"))
				var err error
				output, err = cmd.Output()
				samples = append(samples, float64(time.Since(start))/float64(time.Millisecond))
				deadline := ctx.Err()
				cancel()
				if deadline != nil {
					b.Fatalf("graveyard JSON benchmark exceeded %s: %v", timeout, deadline)
				}
				if err != nil {
					b.Fatal(err)
				}
			}
			var listing GraveyardListing
			if err := json.Unmarshal(output, &listing); err != nil || len(listing.Burials) != n || len(listing.Pending) != 0 {
				b.Fatalf("benchmark command lost listing: %d %v", len(listing.Burials), err)
			}
			b.Logf("samples_ms=%v", samples)
			slices.Sort(samples)
			b.ReportMetric((samples[(len(samples)-1)/2]+samples[len(samples)/2])/2, "p50-ms")
			b.ReportMetric(samples[(95*len(samples)+99)/100-1], "p95-ms")
		})
	}
}

func TestGraveyardIncludesConcurrentRecovery(t *testing.T) {
	svc, _, pending := clitest.PendingCommandBurial(t, false)
	finish := clitest.RecoverDuringGraveyardRead(t, svc)
	original := cmdutil.Open
	cmdutil.Open = func() (*core.Service, error) { return svc, nil }
	t.Cleanup(func() { cmdutil.Open = original })
	for range 2 {
		out, _, err := clitest.Execute(t, NewCmd(), "--json")
		var page GraveyardListing
		if err != nil || json.Unmarshal([]byte(out), &page) != nil {
			t.Fatalf("graveyard read failed: %s %v", out, err)
		}
		if len(page.Burials)+len(page.Pending) != 1 || len(page.Burials) == 1 && page.Burials[0].ID != pending.Burial.ID || len(page.Pending) == 1 && page.Pending[0].Burial.ID != pending.Burial.ID {
			t.Fatalf("graveyard view lost burial during recovery: %+v", page)
		}
		if finish != nil {
			finish()
			finish = nil
		}
	}
}

func TestGraveyardShowsContainedLifespans(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	clitest.IsolateRunner(t)
	root, store := clitest.CommandDir(t)
	child := filepath.Join(root, "output", "keep.txt")
	clitest.WriteFixture(t, child, "keep")
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := svc.Add([]string{filepath.Dir(child), child}, core.AddOptions{ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil || core.Failed(rows) {
		t.Fatalf("add: %+v, %v", rows, err)
	}
	svc.Now = func() time.Time { return rows[0].Item.ExpiresAt }
	if _, err := svc.Bury([]string{filepath.Dir(child)}, false, ""); err != nil {
		t.Fatal(err)
	}
	out, _, err := clitest.Execute(t, NewCmd())
	if err != nil || !strings.Contains(out, fmt.Sprintf("contains #%d", rows[1].Item.ID)) {
		t.Fatalf("missing contained IDs: %s, %v", out, err)
	}
	st, _ := store.Read()
	if len(st.Items) != 0 {
		t.Fatal("child still tracked")
	}
}

func TestPendingManifestVisibleInQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, corrupt := range []bool{false, true} {
		t.Run(map[bool]string{false: "pending", true: "unverifiable"}[corrupt], func(t *testing.T) {
			root, executable := clitest.DoctorFixture(t)
			svc, err := cmdutil.Open()
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "work", "keep.txt")
			clitest.WriteFixture(t, path, "keep")
			blocked := filepath.Join(svc.Store.Home, "store.json.tmp")
			svc.FS = &failBurialCommitFS{FS: svc.FS, blocked: blocked}
			if _, err := svc.Bury([]string{path}, false, ""); err == nil {
				t.Fatal("commit fault not injected")
			}
			if err := os.Remove(blocked); err != nil {
				t.Fatal(err)
			}
			pending, err := svc.PendingGraves()
			if err != nil || len(pending) != 1 {
				t.Fatalf("pending manifest: %+v, %v", pending, err)
			}
			if corrupt {
				clitest.WriteFixture(t, pending[0].Manifest, "{broken")
			}
			before := clitest.TreeSnapshot(t, root)
			checks, _ := doctorChecks(t, executable)
			if checks["graveyard-recovery"].Status != "WARN" || !strings.Contains(checks["graveyard-recovery"].Detail, "1") {
				t.Errorf("doctor did not warn about manifest: %+v", checks)
			}
			if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
				t.Fatal("doctor changed pending recovery")
			}
			out, _, err := clitest.Execute(t, NewCmd(), "--json")
			var graves struct {
				Burials      []core.Burial       `json:"burials"`
				Pending      []core.PendingGrave `json:"pending"`
				PendingCount int                 `json:"pendingCount"`
			}
			if err != nil || json.Unmarshal([]byte(out), &graves) != nil || graves.PendingCount != 1 || len(graves.Pending) != 1 || len(graves.Burials) != 0 {
				t.Errorf("graveyard pending receipt: %s, %v", out, err)
			}
			out, _, err = clitest.Execute(t, NewCmd())
			if err != nil || !strings.Contains(out, "pending recovery: 1") || !strings.Contains(out, pending[0].Manifest) {
				t.Errorf("graveyard text: %s, %v", out, err)
			}
			out, _, err = clitest.Execute(t, daemoncmd.StatusCmd(userdirs.Data), "--json")
			var status struct {
				Pending int `json:"pendingRecovery"`
			}
			if err != nil || json.Unmarshal([]byte(out), &status) != nil || status.Pending != 1 {
				t.Errorf("daemon pending count: %s, %v", out, err)
			}
		})
	}
}
