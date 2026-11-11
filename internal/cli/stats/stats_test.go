package stats

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	gitcleancmd "github.com/woodfishhhh/obituary/internal/cli/gitclean"
	rmcmd "github.com/woodfishhhh/obituary/internal/cli/rm"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func TestStatsDueWording(t *testing.T) {
	for _, delta := range []time.Duration{-time.Hour, time.Hour} {
		t.Run(delta.String(), func(t *testing.T) {
			_, store := clitest.CommandDir(t)
			if err := store.Update(func(st *core.State) error {
				st.Items = []core.Item{{ID: 1, ExpiresAt: time.Now().Add(delta)}}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			out, _, err := clitest.Execute(t, NewCmd())
			want := "(due)"
			if delta > 0 {
				want = "(in 1h)"
			}
			if err != nil || !strings.Contains(out, want) || strings.Contains(out, "in due") {
				t.Fatalf("stats = %s, %v", out, err)
			}
		})
	}
}

// Match BenchmarkList's ledger size; stats reads counters and both collections
// without dereferencing their files. Setup and the build are outside b.Loop.
func BenchmarkStats(b *testing.B) {
	base := b.TempDir()
	exe := filepath.Join(base, "obit.exe")
	build := exec.Command("go", "build", "-o", exe, "./cmd/obit")
	build.Dir = "../../.."
	if out, err := build.CombinedOutput(); err != nil {
		b.Fatal(string(out), err)
	}
	store := core.Store{Home: filepath.Join(base, "data")}
	now := time.Now()
	if err := store.Update(func(st *core.State) error {
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
			st.Items = append(st.Items, core.Item{ID: int64(i + 1), Rev: 1, Path: path, Identity: &id, ExpiresAt: now.Add(time.Hour)})
			st.Burials = append(st.Burials, core.Burial{ID: int64(1001 + i), Path: filepath.Join(base, fmt.Sprintf("old-%04d", i)), Grave: grave, Identity: &objectID, GraveID: &graveID, BuriedAt: now, PurgeAt: now.Add(24 * time.Hour), Reason: core.ReasonBury})
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
		cmd := exec.CommandContext(b.Context(), exe, "stats")
		cmd.Env = env
		var err error
		output, err = cmd.Output()
		samples = append(samples, float64(time.Since(start))/float64(time.Millisecond))
		if err != nil {
			b.Fatal(err)
		}
	}
	if !strings.Contains(string(output), "tracked      1000") || !strings.Contains(string(output), "graveyard    1000") {
		b.Fatalf("stats lost counts: %s", output)
	}
	b.Logf("samples_ms=%v", samples)
	slices.Sort(samples)
	b.ReportMetric((samples[(len(samples)-1)/2]+samples[len(samples)/2])/2, "p50-ms")
	b.ReportMetric(samples[(95*len(samples)+99)/100-1], "p95-ms")
}

func TestDeleteStats(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	clitest.IsolateRunner(t)
	repo, _ := clitest.CommandRepo(t)
	svc, err := app.Open()
	if err != nil {
		t.Fatal(err)
	}
	hook.Claude(app.Engine(svc), strings.NewReader(`{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"rm x"}}`), io.Discard, nil)
	clitest.WriteFixture(t, filepath.Join(repo, "one"), "one")
	if _, _, err := clitest.Execute(t, rmcmd.NewCmd(), "one"); err != nil {
		t.Fatal(err)
	}
	clitest.WriteFixture(t, filepath.Join(repo, "two"), "two")
	if _, _, err := clitest.Execute(t, itemCommand(), "two"); err != nil {
		t.Fatal(err)
	}
	clitest.WriteFixture(t, filepath.Join(repo, "three"), "three")
	if _, _, err := clitest.Execute(t, gitcleancmd.NewCmd(), "-f"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := clitest.Execute(t, rmcmd.NewCmd(), "missing"); err == nil {
		t.Fatal("missing should fail")
	}
	out, stderr, err := clitest.Execute(t, NewCmd(), "--json")
	var summary struct {
		DeleteGuard guardrm.Counts `json:"deleteGuard"`
	}
	if err != nil || json.Unmarshal([]byte(out), &summary) != nil || summary.DeleteGuard.Denied != 1 || summary.DeleteGuard.Buried != 3 {
		t.Fatalf("stats: %s %s %v", out, stderr, err)
	}
}

func TestDeleteStatsWithLongCommand(t *testing.T) {
	repo, store := clitest.CommandDir(t)
	svc, err := app.Open()
	if err != nil {
		t.Fatal(err)
	}
	command := "rm -- '" + strings.Repeat("&", 700*1024) + "'"
	var input, output bytes.Buffer
	enc := json.NewEncoder(&input)
	enc.SetEscapeHTML(false) // shell envelopes need not escape ampersands
	if err := enc.Encode(map[string]any{"hook_event_name": "PreToolUse", "cwd": repo, "tool_name": "Bash", "tool_input": map[string]string{"command": command}}); err != nil {
		t.Fatal(err)
	}
	hook.Claude(app.Engine(svc), &input, &output, nil)
	if !strings.Contains(output.String(), `"permissionDecision":"deny"`) {
		t.Fatalf("long command was not denied: %s", output.String())
	}
	data, err := os.ReadFile(filepath.Join(store.Home, "delete-guard.jsonl"))
	var receipt guardrm.AuditEvent
	if err != nil || json.Unmarshal(data, &receipt) != nil {
		t.Fatalf("audit receipt: %v", err)
	}
	for name, field := range map[string]string{"command": receipt.Command, "equivalent": receipt.Equivalent} {
		if len(field) > 8*1024 || !strings.HasSuffix(field, " [truncated]") {
			t.Errorf("%s not bounded and marked: %d bytes", name, len(field))
		}
	}
	for _, args := range [][]string{nil, {"--json"}} {
		out, stderr, err := clitest.Execute(t, NewCmd(), args...)
		if err != nil || !strings.Contains(out, "denied") {
			t.Errorf("stats after long command: %s %s %v", out, stderr, err)
		}
		if len(args) > 0 {
			var result struct {
				DeleteGuard guardrm.Counts `json:"deleteGuard"`
			}
			if err := json.Unmarshal([]byte(out), &result); err != nil || result.DeleteGuard.Denied != 1 {
				t.Errorf("long command count: %+v %v", result, err)
			}
		}
	}
}

func TestDeleteStatsSkipsOversizedAuditLines(t *testing.T) {
	_, store := clitest.CommandDir(t)
	// Old receipts can exceed the limit even though new writes are bounded.
	long := `{"kind":"denied","command":"` + strings.Repeat("x", 8<<20) + `"}`
	data := "{\"kind\":\"denied\"}\n" + long + "\n{\"kind\":\"buried\",\"count\":3}\n" + long
	clitest.WriteFixture(t, filepath.Join(store.Home, "delete-guard.jsonl"), data)
	out, stderr, err := clitest.Execute(t, NewCmd(), "--json")
	var result struct {
		DeleteGuard struct {
			Denied    int64 `json:"denied"`
			Buried    int64 `json:"buried"`
			Oversized int64 `json:"oversized"`
		} `json:"deleteGuard"`
	}
	if err != nil || json.Unmarshal([]byte(out), &result) != nil || result.DeleteGuard.Denied != 1 || result.DeleteGuard.Buried != 3 || result.DeleteGuard.Oversized != 2 {
		t.Fatalf("stats with old oversized lines: %s %s %v", out, stderr, err)
	}
	out, stderr, err = clitest.Execute(t, NewCmd())
	if err != nil || !strings.Contains(out, "delete audit lines skipped (too long)  2") {
		t.Fatalf("skipped-line warning: %s %s %v", out, stderr, err)
	}
}

func TestDeleteStatsAfterRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	clitest.IsolateRunner(t)
	repo, store := clitest.CommandDir(t)
	clitest.WriteFixture(t, filepath.Join(repo, "victim"), "keep")
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(store.Home, "store.json.tmp")
	svc.FS = &failBurialCommitFS{FS: svc.FS, blocked: blocked}
	old := cmdutil.Open
	cmdutil.Open = func() (*core.Service, error) { return svc, nil }
	t.Cleanup(func() { cmdutil.Open = old })
	if _, _, err := clitest.Execute(t, rmcmd.NewCmd(), "victim"); err == nil {
		t.Fatal("commit fault not injected")
	}
	// Keep the fault fixture for t.TempDir cleanup; no direct removal is needed.
	if err := os.Rename(blocked, blocked+".fault"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, _, err := svc.Tick(); err != nil {
			t.Fatal(err)
		}
		out, stderr, err := clitest.Execute(t, NewCmd(), "--json")
		var result struct {
			DeleteGuard guardrm.Counts `json:"deleteGuard"`
		}
		if err != nil || json.Unmarshal([]byte(out), &result) != nil || result.DeleteGuard.Buried != 1 {
			t.Fatalf("recovery count (tick %d): %s %s %v", i, out, stderr, err)
		}
	}
}
