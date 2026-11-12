package hook

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestGraveSessionContextFiltersAndOrders(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	var burials []core.Burial
	for i := range 7 {
		burials = append(burials, core.Burial{ID: int64(i + 1), Path: filepath.Join(root, fmt.Sprint(i+1)), Project: root, BuriedAt: now.Add(time.Duration(i) * time.Minute), PurgeAt: now.Add(time.Hour)})
	}
	excluded := burials[0]
	for _, kind := range []string{"mktemp", "same-session", "same-operation", "expired", "purging", "other-project"} {
		b := excluded
		b.ID = 99
		b.Path = filepath.Join(root, "excluded-"+kind)
		switch kind {
		case "mktemp":
			b.Source = "mktemp"
		case "same-session":
			b.Session = "current"
		case "same-operation":
			b.Operation = &core.BurialOperation{Session: "claude:current"}
		case "expired":
			b.PurgeAt = now.Add(-time.Second)
		case "purging":
			b.PurgeIntent = &core.PurgeIntent{}
		case "other-project":
			b.Project = filepath.Dir(root)
		}
		burials = append(burials, b)
	}
	out := graveSessionContext(burials, root, "current")
	for _, want := range []string{"7 restorable", "#7", "#3", "and 2 more", "obit graveyard --here", "obit resurrect <id>"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q: %s", want, out)
		}
	}
	if strings.Contains(out, "excluded") || strings.Contains(out, "#2") || strings.Index(out, "#7") > strings.Index(out, "#6") {
		t.Fatal(out)
	}
}

func TestGraveSessionExplicitBurialScope(t *testing.T) {
	root := t.TempDir()
	b := core.Burial{ID: 42, Path: filepath.Join(root, "old"), BuriedAt: time.Now(), PurgeAt: time.Now().Add(time.Hour), Session: "current", Operation: &core.BurialOperation{Project: root}}
	if out := graveSessionContext([]core.Burial{b}, root, "current"); !strings.Contains(out, "#42") {
		t.Fatal("registration session hid another actor's burial", out)
	}
	b.Session = ""
	b.Operation = nil
	if out := graveSessionContext([]core.Burial{b}, root, "current"); !strings.Contains(out, "#42") {
		t.Fatal("unregistered explicit burial missing", out)
	}
}

func TestGraveSessionStartDelivery(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git repository for session snapshot")
	}
	for _, agent := range []string{"claude", "codex"} {
		t.Run(agent, func(t *testing.T) {
			e := newEnv(t)
			b := core.Burial{ID: 42, Path: filepath.Join(e.root, "old"), Project: e.root, BuriedAt: time.Now().Add(-time.Hour), PurgeAt: time.Now().Add(time.Hour)}
			if err := e.engine.Svc.Store.Update(func(st *core.State) error { st.Burials = []core.Burial{b}; return nil }); err != nil {
				t.Fatal(err)
			}
			host, _ := Lookup(agent)
			ev := Event{Name: "SessionStart", Cwd: e.root, Session: "new"}
			var out bytes.Buffer
			runner := &Runner{Tick: func() {}}
			RunEvent(host, e.engine, ev, &out, runner)
			if strings.Contains(out.String(), "#42") {
				t.Fatal("summary is opt-in", out.String())
			}
			e.engine.Svc.SessionSummary = true
			out.Reset()
			RunEvent(host, e.engine, ev, &out, runner)
			if !strings.Contains(out.String(), "#42") || !strings.Contains(out.String(), "obit graveyard --here") {
				t.Fatal(out.String())
			}
			e.engine.Svc.Mode = core.ModeRecycle
			out.Reset()
			RunEvent(host, e.engine, ev, &out, runner)
			if strings.Contains(out.String(), "graveyard") {
				t.Fatal("recycle summary", out.String())
			}
		})
	}
}

func TestGraveSessionContextBudget(t *testing.T) {
	b := core.Burial{ID: 1, Project: testenv.NativeFixture(`C:\repo`), Path: testenv.NativeFixture(`C:\repo\`) + strings.Repeat("文", 3000), PurgeAt: time.Now().Add(time.Hour)}
	out := graveSessionContext([]core.Burial{b}, b.Project, "new")
	if len(out) > maxContext/2 || !strings.Contains(out, "obit graveyard --here") {
		t.Fatal(out)
	}
	if out := graveSessionContext(nil, b.Project, "new"); out != "" {
		t.Fatal(out)
	}
}
