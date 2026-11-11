package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
)

func noticeFixture(t testing.TB, directory bool) (*auto.Engine, string, core.Burial) {
	t.Helper()
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	b := core.Burial{ID: 42, Path: filepath.Join(root, "buried"), Project: root, IsDir: directory, BuriedAt: time.Now().Add(-time.Hour), PurgeAt: time.Now().Add(7 * 24 * time.Hour)}
	e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: filepath.Join(root, "home")}}}
	if err := e.Svc.Store.Update(func(st *core.State) error { st.Burials = []core.Burial{b}; return nil }); err != nil {
		t.Fatal(err)
	}
	return e, root, b
}

func runNotice(t testing.TB, e *auto.Engine, agent, session, root, tool string, input map[string]any) string {
	t.Helper()
	data, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "session_id": session, "cwd": root, "tool_name": tool, "tool_input": input})
	var out bytes.Buffer
	Run(agent, e, bytes.NewReader(data), &out, nil)
	return out.String()
}

func TestGraveNoticeReadDirectoryAndDedup(t *testing.T) {
	for _, agent := range []string{"claude", "codex"} {
		for _, dir := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/dir=%v", agent, dir), func(t *testing.T) {
				e, root, burial := noticeFixture(t, dir)
				path := burial.Path
				if dir {
					path = filepath.Join(path, "child.txt")
				}
				input := map[string]any{"file_path": path}
				before, _ := os.ReadFile(filepath.Join(e.Svc.Store.Home, "store.json"))
				out := runNotice(t, e, agent, "s1", root, "Read", input)
				for _, want := range []string{"#42", "obit resurrect 42", "restorable until", burial.BuriedAt.Format(time.RFC3339)} {
					if !strings.Contains(out, want) {
						t.Fatalf("missing %q: %s", want, out)
					}
				}
				if strings.Contains(out, "permissionDecision") {
					t.Fatal("notice affected permissions", out)
				}
				if dir && !strings.Contains(out, "entire directory #42") {
					t.Fatal(out)
				}
				if out := runNotice(t, e, agent, "s1", root, "Read", input); out != "" {
					t.Fatal("duplicate", out)
				}
				if out := runNotice(t, e, agent, "s2", root, "Read", input); out == "" {
					t.Fatal("new session lost notice")
				}
				after, _ := os.ReadFile(filepath.Join(e.Svc.Store.Home, "store.json"))
				if !bytes.Equal(before, after) {
					t.Fatal("advisory rewrote ledger")
				}
			})
		}
	}
}

func TestGraveNoticeSkipsUnavailableAndUnrelated(t *testing.T) {
	for _, variant := range []string{"existing", "purged", "expired", "purging", "unrelated", "sibling"} {
		t.Run(variant, func(t *testing.T) {
			e, root, b := noticeFixture(t, true)
			path := b.Path
			switch variant {
			case "existing":
				if err := os.WriteFile(path, []byte("restored"), 0600); err != nil {
					t.Fatal(err)
				}
			case "purged":
				index := filepath.Join(e.Svc.Store.Home, "grave-notice-paths.json")
				stale, err := os.ReadFile(index)
				if err != nil {
					t.Fatal(err)
				}
				if err := e.Svc.Store.Update(func(st *core.State) error { st.Burials = nil; return nil }); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(index, stale, 0600); err != nil {
					t.Fatal(err)
				}
			case "expired":
				if err := e.Svc.Store.Update(func(st *core.State) error { st.Burials[0].PurgeAt = time.Now().Add(-time.Second); return nil }); err != nil {
					t.Fatal(err)
				}
			case "purging":
				if err := e.Svc.Store.Update(func(st *core.State) error { st.Burials[0].PurgeIntent = &core.PurgeIntent{}; return nil }); err != nil {
					t.Fatal(err)
				}
			case "unrelated":
				path = filepath.Join(root, "elsewhere")
			case "sibling":
				path += "-neighbour"
			}
			calls := 0
			old := readNoticeSnapshot
			readNoticeSnapshot = func(s core.Store) (core.State, error) { calls++; return old(s) }
			defer func() { readNoticeSnapshot = old }()
			if out := runNotice(t, e, "claude", "s", root, "Read", map[string]any{"file_path": path}); out != "" {
				t.Fatal(out)
			}
			if (variant == "existing" || variant == "unrelated" || variant == "sibling") && calls != 0 {
				t.Fatalf("unrelated ledger reads=%d", calls)
			}
		})
	}
}

func TestGraveNoticeLockedLedgerSkipsWithoutQueue(t *testing.T) {
	e, root, b := noticeFixture(t, false)
	unlock, ok, err := core.TryLockExclusive(filepath.Join(e.Svc.Store.Home, "store.lock"))
	if err != nil || !ok {
		t.Fatal(err)
	}
	out := runNotice(t, e, "claude", "s", root, "Read", map[string]any{"file_path": b.Path})
	unlock()
	if out != "" {
		t.Fatal(out)
	}
	state := readGraveSession(e.Svc.Store.Home, Event{Host: "claude", Session: "s"})
	if len(state.Seen)+len(state.Pending) != 0 || e.Svc.Store.HasDeferredObservations() {
		t.Fatal("locked advisory queued")
	}
	if _, err := os.Stat(filepath.Join(e.Svc.Store.Home, "hook.log")); !os.IsNotExist(err) {
		t.Fatal("advisory error logged")
	}
	if out := runNotice(t, e, "claude", "s", root, "Read", map[string]any{"file_path": b.Path}); out == "" {
		t.Fatal("retry lost notice")
	}
}

func TestGraveNoticeWriteAndGuardBytes(t *testing.T) {
	for _, agent := range []string{"claude", "codex"} {
		e, root, b := noticeFixture(t, false)
		out := runNotice(t, e, agent, "s", root, "Write", map[string]any{"file_path": b.Path})
		if !strings.Contains(out, "old version") || strings.Contains(out, "deny") {
			t.Fatal(out)
		}
		input := map[string]any{"command": "rm buried", "shell": "bash"}
		before := runNotice(t, e, agent, "s", root, "Bash", input)
		if err := e.Svc.Store.Update(func(st *core.State) error { st.Burials = nil; return nil }); err != nil {
			t.Fatal(err)
		}
		after := runNotice(t, e, agent, "s", root, "Bash", input)
		if before != after || !strings.Contains(before, `"permissionDecision":"deny"`) {
			t.Fatalf("guard bytes changed: %q / %q", before, after)
		}
	}
}

func TestGraveNoticeConcurrentSessionOnce(t *testing.T) {
	e, root, b := noticeFixture(t, false)
	var wg sync.WaitGroup
	outputs := make(chan string, 8)
	for range 8 {
		wg.Go(func() {
			outputs <- runNotice(t, e, "claude", "same", root, "Read", map[string]any{"file_path": b.Path})
		})
	}
	wg.Wait()
	close(outputs)
	count := 0
	for out := range outputs {
		if out != "" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("delivered %d notices", count)
	}
}

func BenchmarkGraveNoticePreToolUse(b *testing.B) {
	for _, agent := range []string{"claude", "codex"} {
		for _, exists := range []bool{true, false} {
			name := "missing"
			if exists {
				name = "existing"
			}
			b.Run(agent+"/"+name, func(b *testing.B) {
				dir := b.TempDir()
				target := filepath.Join(dir, "old.txt")
				if exists {
					if err := os.WriteFile(target, []byte("content"), 0600); err != nil {
						b.Fatal(err)
					}
				}
				store := core.Store{Home: filepath.Join(dir, "home")}
				if err := store.Update(func(st *core.State) error {
					st.Burials = []core.Burial{{ID: 1, Path: target, BuriedAt: time.Now(), PurgeAt: time.Now().Add(time.Hour)}}
					return nil
				}); err != nil {
					b.Fatal(err)
				}
				e := &auto.Engine{Svc: &core.Service{Store: store}}
				data, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "session_id": "benchmark", "cwd": dir, "tool_name": "Read", "tool_input": map[string]any{"file_path": target}})
				var samples []float64
				for b.Loop() {
					start := time.Now()
					Run(agent, e, bytes.NewReader(data), io.Discard, nil)
					samples = append(samples, float64(time.Since(start))/float64(time.Millisecond))
				}
				slices.Sort(samples)
				b.ReportMetric(samples[len(samples)/2], "p50-ms")
				b.ReportMetric(samples[(95*len(samples)+99)/100-1], "p95-ms")
			})
		}
	}
}
