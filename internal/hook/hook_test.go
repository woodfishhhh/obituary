package hook

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/rules"
)

func TestIsErrorLogLineConservativelyFiltersDiscoveryDenials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Program Files")
	cwd := filepath.Join(path, "app", "resources")
	setProtectedDiscoveryRoots(t, path)
	message := fmt.Sprintf("discover project from %q: resolve project directory %s: Access is denied.", cwd, path)
	line := func(message, metadata string) string {
		return fmt.Sprintf("2026-11-11T00:00:00Z codex SessionEnd: %q%s%s", message, permissionMarker, metadata)
	}
	metadata := fmt.Sprintf(`{"operation":"resolve project directory","path":%q}`, path)
	deniedLine := func(denied string) string {
		message := fmt.Sprintf("discover project from %q: resolve project directory %s: Access is denied.", cwd, denied)
		return line(message, fmt.Sprintf(`{"operation":"resolve project directory","path":%q}`, denied))
	}
	for _, tc := range []struct {
		name, line string
		wantError  bool
	}{
		{"skip", `2026-11-11T00:00:00Z skip: project discovery unavailable; cwd="C:\\fixture" reason="permission denied"`, false},
		{"legacy", line(message, metadata), false},
		{"outside-protected", deniedLine(filepath.Dir(path)), true},
		{"protected-prefix", deniedLine(path + "X"), true},
		{"diagnostic-path-mismatch", line(message, `{"operation":"resolve project directory","path":"C:\\other"}`), true},
		{"missing-operation", line(message, `{"path":"C:\\fixture"}`), true},
		{"missing-diagnostic", fmt.Sprintf("2026-11-11T00:00:00Z codex SessionEnd: %q", message), true},
		{"permission-in-cwd-only", line(fmt.Sprintf("discover project from %q: resolve project directory %s: path changed", cwd+`\permission denied`, path), metadata), true},
		{"joined-error", line(message+"\ncontext canceled", metadata), true},
		{"skip-in-error", line("skip: permission denied", metadata), true},
		{"truncated", `2026-11-11T00:00:00Z codex SessionEnd: "discover project from`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsErrorLogLine(tc.line); got != tc.wantError {
				t.Fatalf("error=%v, want %v: %s", got, tc.wantError, tc.line)
			}
		})
	}
}

func TestSessionRulesPauseDoesNotExposeContent(t *testing.T) {
	for _, body := range []string{"[PRIVATE_RULE_TOKEN_83917 2h\n", "name EXTERNAL_PRIVATE_TOKEN_83917\n"} {
		root := t.TempDir()
		svc := &core.Service{Store: core.Store{Home: filepath.Join(t.TempDir(), "data")}}
		if err := os.WriteFile(filepath.Join(root, rules.FileName), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		e := &auto.Engine{Svc: svc, Trust: rules.TrustStore{Path: filepath.Join(svc.Store.Home, "trust.json")}}
		d, err := sessionContext(e, root, "s")
		text := strings.Join(d.lines, "\n")
		if err != nil || !strings.Contains(text, "paused") || !strings.Contains(text, rules.FileName) || strings.Contains(text, "PRIVATE_TOKEN") || strings.Contains(text, "PRIVATE_RULE_TOKEN") || strings.Contains(text, "2h") {
			t.Fatalf("agent pause leaked rule content: %s %v", text, err)
		}
	}
}

func TestSessionMissingTrustedRulesPause(t *testing.T) {
	root := t.TempDir()
	svc := &core.Service{Store: core.Store{Home: t.TempDir()}}
	path := filepath.Join(root, rules.FileName)
	if err := os.WriteFile(path, []byte("*.log off\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e := &auto.Engine{Svc: svc, Trust: rules.TrustStore{Path: filepath.Join(svc.Store.Home, "trust.json")}}
	if _, err := e.Trust.Trust(path, ""); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	d, err := sessionContext(e, root, "s")
	text := strings.Join(d.lines, "\n")
	if err != nil || !strings.Contains(text, "missing-trusted") || !strings.Contains(text, "paused") || strings.Contains(text, "*.log") {
		t.Fatalf("session did not explain missing protection safely: %s %v", text, err)
	}
}

// By default expiring and buried items are announced only when a tool call
// touches them; the session start stays quiet to save the agent's tokens.
func TestSessionStartDoesNotListExpiringOrBuriedItems(t *testing.T) {
	for _, mode := range []string{core.ModeGraveyard, core.ModeRecycle} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			svc := &core.Service{Store: core.Store{Home: filepath.Join(root, "data")}, Mode: mode}
			if err := svc.Store.Update(func(st *core.State) error {
				st.Items = []core.Item{{ID: 1, Project: root, Path: filepath.Join(root, "due.log"), ExpiresAt: time.Now().Add(-time.Hour)}}
				st.Burials = []core.Burial{{ID: 2, Project: root, Path: filepath.Join(root, "old.log"), BuriedAt: time.Now().Add(-time.Hour), PurgeAt: time.Now().Add(time.Hour)}}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			context, err := sessionContext(&auto.Engine{Svc: svc}, root, "s")
			if text := strings.Join(context.lines, "\n"); err != nil || text != "" {
				t.Fatalf("session start listed items: %q %v", text, err)
			}
		})
	}
}

func TestSessionExpiryWording(t *testing.T) {
	for _, n := range []int{1, 2} {
		for _, mode := range []string{core.ModeGraveyard, core.ModeRecycle} {
			t.Run(fmt.Sprintf("%d/%s", n, mode), func(t *testing.T) {
				root := t.TempDir()
				svc := &core.Service{Store: core.Store{Home: filepath.Join(root, "data")}, Mode: mode, SessionSummary: true}
				if err := svc.Store.Update(func(st *core.State) error {
					for i := range n {
						st.Items = append(st.Items, core.Item{ID: int64(i + 1), Project: root, Path: filepath.Join(root, fmt.Sprintf("%d.log", i)), ExpiresAt: time.Now().Add(-time.Hour)})
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				context, err := sessionContext(&auto.Engine{Svc: svc}, root, "s")
				text := strings.Join(context.lines, "\n")
				want := "1 item in this project will expire"
				if n == 2 {
					want = "2 items in this project will expire"
				}
				if err != nil || !strings.Contains(text, want) || !strings.Contains(text, "(due)") || strings.Contains(text, "in due") {
					t.Fatalf("context = %s, %v", text, err)
				}
				if mode == core.ModeRecycle && (!strings.Contains(text, "restore from the Recycle Bin") || strings.Contains(text, "obit resurrect")) {
					t.Fatal("wrong recovery advice", text)
				}
			})
		}
	}
}

// Explicit registrations carry no project; the summary attributes them by
// path. Active mktemp scratch directories are left out (dogfood round 4).
func TestSessionSummaryAttributesExplicitItemsAndSkipsScratch(t *testing.T) {
	root := t.TempDir()
	svc := &core.Service{Store: core.Store{Home: filepath.Join(root, "data")}, SessionSummary: true}
	soon := time.Now().Add(2 * time.Hour)
	if err := svc.Store.Update(func(st *core.State) error {
		st.Items = []core.Item{
			{ID: 1, Path: filepath.Join(root, "manual.txt"), ExpiresAt: soon, Source: "cli"},
			{ID: 2, Project: root, Path: filepath.Join(root, ".obit", "tmp", "tmp-1"), IsDir: true, ExpiresAt: soon, Source: "mktemp"},
			{ID: 3, Path: filepath.Join(filepath.Dir(root), "elsewhere.txt"), ExpiresAt: soon, Source: "cli"},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	context, err := sessionContext(&auto.Engine{Svc: svc}, root, "s")
	text := strings.Join(context.lines, "\n")
	if err != nil || !strings.Contains(text, "#1 manual.txt") || strings.Contains(text, "#2") || strings.Contains(text, "#3") {
		t.Fatalf("summary = %q %v", text, err)
	}
}

func TestSessionContextOmitsMissingProposalCandidatesWithoutGit(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	e := newEnv(t)
	keep, gone := e.write("keep.txt"), e.write("gone.txt")
	if err := e.engine.Snapshot(e.root, "s"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := e.engine.Svc.Propose(core.ProposalLeftovers, e.root, "s", []core.Candidate{{Path: keep, Lifespan: "1d"}, {Path: gone, Lifespan: "1d"}}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	text, err := Handle(e.engine, Event{Name: "SessionStart", Cwd: e.root, Session: "s"}, nil)
	if err != nil || !strings.Contains(text, "keep.txt") || strings.Contains(text, "gone.txt") {
		t.Fatalf("session displayed missing candidates or queried git: %s %v", text, err)
	}
	if err := os.Remove(keep); err != nil {
		t.Fatal(err)
	}
	text, err = Handle(e.engine, Event{Name: "SessionStart", Cwd: e.root, Session: "s"}, nil)
	if err != nil || text != "" {
		t.Fatalf("session displayed empty proposal: %s %v", text, err)
	}
	st := e.state()
	if len(st.Proposals) != 1 || len(st.Proposals[0].Candidates) != 2 {
		t.Fatal("display changed durable proposal candidates")
	}
}

func TestLogfDoesNotWaitIndefinitelyForLogLock(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	release, err := core.LockFile(filepath.Join(home, "hook.log.lock"), true)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		Logf(home, "contended")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Error("Logf blocked on a held log lock")
	}
	release()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Logf did not return after releasing the lock")
	}
	Logf(home, "after release")
	data, err := os.ReadFile(filepath.Join(home, "hook.log"))
	if err != nil || strings.Contains(string(data), "contended") || !strings.Contains(string(data), "after release") {
		t.Errorf("contended line must be dropped and later logging must work: %q, %v", data, err)
	}
}

func TestSessionStartExplainsPausedProjectRules(t *testing.T) {
	if testing.Short() {
		t.Skip("session context uses a real Git repository")
	}
	e := newEnv(t)
	path := filepath.Join(e.root, ".obitrules")
	if err := os.WriteFile(path, []byte("# explanation\n[broken 2h\n"), 0600); err != nil {
		t.Fatal(err)
	}
	context, err := sessionContext(e.engine, e.root, "broken-rules")
	text := strings.Join(context.lines, "\n")
	if err != nil || !strings.Contains(text, ".obitrules") || !strings.Contains(text, "line 2; pattern") || !strings.Contains(text, "automatic registration paused") || strings.Contains(text, "[broken") {
		t.Fatalf("session hid paused rules: %s %v", text, err)
	}
}

func TestSessionStartOnlyAcknowledgesCompleteProposals(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	for i := 0; i < 9; i++ {
		_, _, _, err := e.engine.Svc.Propose(core.ProposalLeftovers, e.root, fmt.Sprint(i), []core.Candidate{{Path: e.write(fmt.Sprintf("leftover-%d.txt", i)), Lifespan: "3d"}})
		if err != nil {
			t.Fatal(err)
		}
	}
	seen := map[int64]bool{}
	for attempt := 0; attempt < 9; attempt++ {
		text, err := Handle(e.engine, Event{Name: "SessionStart", Cwd: e.root, Session: "s"}, nil)
		if err != nil || len(text) > maxContext {
			t.Fatalf("context: %q %v", text, err)
		}
		for _, p := range e.state().Proposals {
			complete := strings.Contains(text, describeProposal(p, ""))
			if slices.Contains(p.Notified, "s") && !seen[p.ID] && !complete {
				t.Fatalf("proposal #%d acknowledged without full output: %q", p.ID, text)
			}
			if complete {
				seen[p.ID] = true
			}
		}
		if len(seen) == 9 {
			return
		}
	}
	t.Fatalf("truncated proposals never redelivered: %v", seen)
}

func TestSessionStartDeliversOversizedProposalSummary(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	if err := e.engine.Snapshot(e.root, "s"); err != nil {
		t.Fatal(err)
	}
	var candidates []core.Candidate
	for i := 0; i < 5; i++ {
		candidates = append(candidates, core.Candidate{Path: e.write(strings.Repeat("subdirectory/", 45) + fmt.Sprintf("%d.txt", i)), Lifespan: "3d"})
	}
	p, _, _, err := e.engine.Svc.Propose(core.ProposalLeftovers, e.root, "previous", candidates)
	if err != nil {
		t.Fatal(err)
	}
	text, err := Handle(e.engine, Event{Name: "SessionStart", Cwd: e.root, Session: "s"}, nil)
	if err != nil || len(text) > maxContext || !strings.Contains(text, fmt.Sprintf("obit approve %d", p.ID)) || !strings.Contains(text, "obit proposals") || !slices.Contains(e.state().Proposals[0].Notified, "s") {
		t.Fatalf("oversized proposal cannot be delivered: %q %v", text, err)
	}
}

func TestSessionContextDescribesApprovedPendingCandidates(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	good, bad := e.write("good.log"), e.write("bad.log")
	e.wrote(good)
	e.wrote(bad)
	p := e.state().Proposals[0]
	ctx := e.event(map[string]any{"hook_event_name": "SessionStart"})
	if !strings.Contains(ctx, "until the user approves") {
		t.Fatalf("unapproved context: %q", ctx)
	}
	e.engine.Svc.Guard.Canonical = func(path string) (string, error) {
		if strings.EqualFold(path, bad) {
			return "", errors.New("temporarily busy")
		}
		return path, nil
	}
	if _, err := e.engine.Approve([]string{fmt.Sprint(p.ID)}); err != nil {
		t.Fatal(err)
	}
	st := e.state()
	if len(st.Items) != 1 || len(st.Proposals) != 1 || st.Projects[core.ProjectKey(e.root)].Consent != core.ConsentApproved {
		t.Fatalf("not partially approved: %+v", st)
	}
	ctx = e.event(map[string]any{"hook_event_name": "SessionStart"})
	if strings.Contains(ctx, "until the user approves") || !strings.Contains(ctx, "already approved") || !strings.Contains(ctx, "retry") || !strings.Contains(ctx, "bad.log") || !strings.Contains(ctx, fmt.Sprintf("obit approve %d", p.ID)) {
		t.Fatalf("approved pending context: %q", ctx)
	}
}
