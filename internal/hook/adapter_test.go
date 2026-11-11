package hook

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
)

func TestPostToolUseDeliversReplayedProposal(t *testing.T) {
	if testing.Short() {
		t.Skip("observation replay uses a real Git repository")
	}
	for _, name := range []string{"claude", "codex"} {
		for _, background := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/background=%v", name, background), func(t *testing.T) {
				e := newEnv(t)
				path := e.write("debug.log")
				agent, _ := Lookup(name)
				ev := Event{Name: "PostToolUse", Cwd: e.root, Session: "s", Paths: []string{path}}
				if _, err := e.engine.Svc.Store.Read(); err != nil {
					t.Fatal(err)
				}
				release, err := core.LockFile(filepath.Join(e.engine.Svc.Store.Home, "store.lock"), true)
				if err != nil {
					t.Fatal(err)
				}
				unlock := sync.OnceFunc(release)
				t.Cleanup(unlock)
				var out bytes.Buffer
				RunEvent(agent, e.engine, ev, &out, &Runner{})
				unlock()
				rows, err := e.engine.Svc.Store.DeferredObservations()
				if err != nil || len(rows) != 1 || out.Len() != 0 {
					t.Fatalf("write was not deferred silently: rows=%+v output=%q err=%v", rows, &out, err)
				}
				if background {
					if err := e.engine.ReplayObservations(); err != nil {
						t.Fatal(err)
					}
				}
				ev.Paths = nil
				RunEvent(agent, e.engine, ev, &out, &Runner{})
				st := e.state()
				if len(st.Proposals) != 1 || len(st.Items) != 0 || !st.ObservationCompleted(&rows[0]) || e.engine.Svc.Store.HasDeferredObservations() {
					t.Fatalf("deferred proposal lost: %+v", st)
				}
				if !strings.Contains(out.String(), "obit approve") || !strings.Contains(out.String(), "debug.log") || !slices.Contains(st.Proposals[0].Notified, ev.Session) {
					t.Fatalf("replayed proposal was not delivered: output=%q notified=%v", &out, st.Proposals[0].Notified)
				}
				out.Reset()
				RunEvent(agent, e.engine, ev, &out, &Runner{})
				if out.Len() != 0 {
					t.Fatalf("notice repeated: %s", &out)
				}
			})
		}
	}
}

func TestPostToolUseRecoversProposalAfterConsentTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("proposal creation uses a real Git repository")
	}
	for _, name := range []string{"claude", "codex"} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			agent, _ := Lookup(name)
			ev := Event{Name: "PostToolUse", Cwd: e.root, Session: "s", Paths: []string{e.write("debug.log")}}
			previous := proposalConsent
			t.Cleanup(func() { proposalConsent = previous })
			proposalConsent = func(svc *core.Service, root string) (string, error) {
				_, err := svc.Store.WithDeadline(time.Now()).Read()
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("expected the post-commit read to exhaust its lock budget: %v", err)
				}
				return "", err
			}
			var out bytes.Buffer
			RunEvent(agent, e.engine, ev, &out, &Runner{})
			proposalConsent = previous
			st := e.state()
			if len(st.Proposals) != 1 || len(st.ObservationReceipts) != 1 || len(st.Proposals[0].Notified) != 0 || out.Len() != 0 || e.engine.Svc.Store.HasDeferredObservations() {
				t.Fatalf("post-commit timeout did not retain the pending notice: %+v %s", st, &out)
			}
			if data, err := os.ReadFile(filepath.Join(e.engine.Svc.Store.Home, "hook.log")); err != nil || !strings.Contains(string(data), "deadline exceeded") {
				t.Fatalf("post-commit timeout was not diagnosed: %s %v", data, err)
			}
			ev.Paths = []string{e.write("unmatched.txt")}
			RunEvent(agent, e.engine, ev, &out, &Runner{})
			if !strings.Contains(out.String(), "obit approve") || !slices.Contains(e.state().Proposals[0].Notified, ev.Session) {
				t.Fatalf("committed proposal was never delivered: %q", &out)
			}
			out.Reset()
			RunEvent(agent, e.engine, ev, &out, &Runner{})
			if out.Len() != 0 {
				t.Fatalf("notice repeated: %s", &out)
			}
		})
	}
}

func TestPostToolUsePendingNoticeScopeAndSilence(t *testing.T) {
	for _, name := range []string{"claude", "codex"} {
		t.Run(name, func(t *testing.T) {
			root, elsewhere := t.TempDir(), t.TempDir()
			if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "pending.log")
			if err := os.WriteFile(path, []byte("pending"), 0o600); err != nil {
				t.Fatal(err)
			}
			e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: t.TempDir()}}}
			if err := e.Svc.Store.Update(func(st *core.State) error {
				for i, scope := range []struct{ project, session string }{{root, "other"}, {elsewhere, "s"}, {elsewhere, "other"}} {
					st.Proposals = append(st.Proposals, core.Proposal{ID: int64(i + 1), Kind: core.ProposalOnboard, Project: scope.project, Session: scope.session,
						Candidates: []core.Candidate{{Path: path, Lifespan: "3d"}}})
				}
				st.NextID = 4
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			agent, _ := Lookup(name)
			runner := &Runner{Tick: func() {}, Finish: func(string, string, bool) {}}
			for _, ev := range []Event{{Name: "Stop"}, {Name: "SessionEnd"}, {Name: "PostToolUseFailure"}, {Name: "PostToolUse", DeferContext: true}, {Name: "PostToolUse"}, {Name: "PostToolUse"}} {
				ev.Cwd, ev.Session = root, "s"
				var out bytes.Buffer
				before, err := e.Svc.Store.Read()
				if err != nil {
					t.Fatal(err)
				}
				RunEvent(agent, e, ev, &out, runner)
				want := ev.Name == "PostToolUse" && !ev.DeferContext && len(before.Proposals[0].Notified) == 0
				if !want && out.Len() != 0 || strings.Contains(out.String(), "obit approve 1") != want || strings.Contains(out.String(), "obit approve 2") != want || strings.Contains(out.String(), "obit approve 3") {
					t.Fatalf("scope/silence failed for %+v: %s", ev, &out)
				}
				st, err := e.Svc.Store.Read()
				if err != nil || len(st.Proposals[2].Notified) != 0 || want && (!slices.Contains(st.Proposals[0].Notified, "s") || !slices.Contains(st.Proposals[1].Notified, "s")) {
					t.Fatalf("pending acknowledgement scope: %+v %v", st.Proposals, err)
				}
				if !want && (!slices.Equal(st.Proposals[0].Notified, before.Proposals[0].Notified) || !slices.Equal(st.Proposals[1].Notified, before.Proposals[1].Notified)) {
					t.Fatalf("silent callback acknowledged pending context: %+v", st.Proposals)
				}
			}
		})
	}
}

func TestAdapterRegistry(t *testing.T) {
	for _, name := range []string{"claude", "codex"} {
		agent, ok := Lookup(name)
		if !ok || agent.Name != name || agent.Adapter == nil {
			t.Fatalf("missing adapter %q: %+v", name, agent)
		}
	}
	if _, ok := Lookup("unknown"); ok {
		t.Fatal("unknown agent found")
	}
	all := Agents()
	for i := 1; i < len(all); i++ {
		if all[i-1].Name >= all[i].Name {
			t.Fatal("unordered registry")
		}
	}
	all[0].Name = "changed"
	if Agents()[0].Name == "changed" {
		t.Fatal("caller changed registry")
	}
	for _, agent := range []Agent{{}, {Name: "Bad", Adapter: claudeAdapter{}}, {Name: "audit", Adapter: claudeAdapter{}}, {Name: "claude", Adapter: claudeAdapter{}}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("invalid registration accepted: %+v", agent)
				}
			}()
			Register(agent)
		}()
	}
}

func TestAdapterMatchesLegacyEntryPoints(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"claude", "codex"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			agent, _ := Lookup(name)
			input := []byte(`{"hook_event_name":"PreToolUse","cwd":"C:\\repo","tool_name":"Bash","tool_input":{"command":"rm scratch.log","shell":"bash"}}`)
			ev, err := agent.Adapter.Decode(input, Invocation{Agent: name})
			if err != nil || ev.Name != "PreToolUse" || ev.Command != "rm scratch.log" {
				t.Fatalf("decode: %+v %v", ev, err)
			}
			e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: t.TempDir()}}}
			var direct, decoded bytes.Buffer
			if name == "claude" {
				Claude(e, bytes.NewReader(input), &direct, nil)
			} else {
				Codex(e, bytes.NewReader(input), &direct, nil)
			}
			RunEvent(agent, e, ev, &decoded, nil)
			if !bytes.Equal(direct.Bytes(), decoded.Bytes()) || !bytes.Contains(decoded.Bytes(), []byte(`"permissionDecision":"deny"`)) {
				t.Fatalf("legacy mismatch: %s / %s", direct.Bytes(), decoded.Bytes())
			}
			if got := agent.Adapter.Fallback(Event{}); len(got) != 0 {
				t.Fatalf("legacy fallback changed: %s", got)
			}
		})
	}
}

func TestAdapterReadLimitAndBOM(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"claude", "codex"} {
		agent, _ := Lookup(name)
		plain := []byte(`{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"echo ok"}}`)
		a, err := Decode(agent.Adapter, plain, Invocation{Agent: name})
		if err != nil {
			t.Fatal(err)
		}
		b, err := Decode(agent.Adapter, append([]byte("\xef\xbb\xbf"), plain...), Invocation{Agent: name})
		if err != nil || !reflect.DeepEqual(a, b) {
			t.Fatalf("BOM changed event: %+v %+v %v", a, b, err)
		}
	}
	for _, length := range []int{MaxInput, MaxInput + 1} {
		data, err := ReadInput(strings.NewReader(strings.Repeat(" ", length)))
		if length == MaxInput && (err != nil || len(data) != length) || length > MaxInput && err == nil {
			t.Fatalf("length=%d read=%d err=%v", length, len(data), err)
		}
	}
}

type brokenAdapter struct{ stage string }

func (a brokenAdapter) Decode([]byte, Invocation) (Event, error) {
	if a.stage == "panic" {
		panic("decoder panic")
	}
	return Event{}, errors.New("decode failed")
}
func (a brokenAdapter) Encode(Event, Result) ([]byte, error) {
	return nil, errors.New("encode failed")
}
func (a brokenAdapter) Fallback(Event) []byte { return []byte("{}\n") }

type failedReader struct{}

func (failedReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestAdapterFailureUsesNeutralOutput(t *testing.T) {
	for _, stage := range []string{"panic", "decode", "read", "disabled"} {
		t.Run(stage, func(t *testing.T) {
			name := "test-adapter-" + stage
			Register(Agent{Name: name, Adapter: brokenAdapter{stage: stage}})
			t.Cleanup(func() { adapters.Lock(); delete(adapters.byName, name); adapters.Unlock() })
			e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: t.TempDir()}}}
			var out bytes.Buffer
			var in io.Reader = strings.NewReader("{}")
			if stage == "read" {
				in = failedReader{}
			}
			if stage == "disabled" {
				t.Setenv("OBIT_DISABLED", "1")
			}
			Run(name, e, in, &out, nil)
			if out.String() != "{}\n" {
				t.Fatalf("missing neutral output: %q", out.String())
			}
		})
	}
	var out bytes.Buffer
	e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: t.TempDir()}}}
	RunEvent(Agent{Name: "broken", Adapter: brokenAdapter{}}, e, Event{Name: "PreToolUse", SkipReason: "unsupported"}, &out, nil)
	if out.String() != "{}\n" {
		t.Fatalf("encode failure lost neutral response: %q", out.String())
	}
}
