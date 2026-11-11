package rules_test

import (
	"errors"
	"fmt"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/core/guard"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/rules"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func TestRuleScanLimitIsDefinitive(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	const limit = 4
	rules.SetWalkLimit(t, limit)
	base := t.TempDir()
	root, home := filepath.Join(base, "repo"), filepath.Join(base, "home")
	testenv.InitRepo(t, root)
	root, err := winfs.Canonical(root)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	svc := &core.Service{
		Store: ledger.Store{Home: home},
		Guard: guard.Guard{System: []string{home}},
		FS:    winfs.FS{},
		Now:   func() time.Time { return now },
	}
	engine := &auto.Engine{Svc: svc, Trust: rules.TrustStore{Path: filepath.Join(home, "trusted_rules.json")}}
	svc.Owner, svc.Recheck = auto.Owner, engine.RecheckRules
	file := filepath.Join(root, rules.FileName)
	if err := os.WriteFile(file, []byte("!*.keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Trust.Trust(file, ""); err != nil {
		t.Fatal(err)
	}
	set, _, err := engine.RuleSet(root)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "test-results")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(i int) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d.log", i)), []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// WalkDir counts the root too, so this reaches the budget exactly.
	for i := 0; i < limit-1; i++ {
		write(i)
	}
	if protected, err := set.Protects(dir, true); err != nil || protected {
		t.Fatalf("at the scan limit: Protects = %v, %v; want false, nil", protected, err)
	}
	registered, err := engine.Apply([]string{dir}, auto.Options{})
	if err != nil || len(registered) != 1 || registered[0].Action != auto.Registered || registered[0].Unit != dir {
		t.Fatalf("register directory at the scan limit: %+v, %v", registered, err)
	}
	for i := limit - 1; i < limit+1; i++ {
		write(i)
	}
	child := filepath.Join(dir, "0.log")
	t.Run("MatchSmallerUnit", func(t *testing.T) {
		m, ok, err := set.Match(child, false)
		if err != nil || !ok || m.Unit != child {
			t.Errorf("Match = %+v, %v, %v; want child unit without error", m, ok, err)
		}
	})
	t.Run("MatchDirectory", func(t *testing.T) {
		if m, ok, err := set.Match(dir, true); err != nil || ok {
			t.Errorf("Match = %+v, %v, %v; want no unit without error", m, ok, err)
		}
	})
	t.Run("Protects", func(t *testing.T) {
		if protected, err := set.Protects(dir, true); err != nil || !protected {
			t.Errorf("Protects = %v, %v; want true, nil", protected, err)
		}
	})
	t.Run("ExpiryRefused", func(t *testing.T) {
		if err := engine.RecheckRules(root, dir, true); !errors.Is(err, core.ErrProtected) {
			t.Errorf("RecheckRules = %v; want ErrProtected", err)
		}
		now = registered[0].Item.ExpiresAt.Add(time.Second)
		out, err := svc.Sweep(false)
		if err != nil || len(out) != 1 || out[0].Action != ledger.ActionRefused {
			t.Errorf("scan limit must refuse, not retry: %+v, %v", out, err)
		}
		st, err := svc.Store.Read()
		if err != nil || len(st.Items) != 0 || len(st.Burials) != 0 {
			t.Errorf("refused item must settle without burial: %+v, %v", st, err)
		}
		for i := 0; i < limit+1; i++ {
			path := filepath.Join(dir, fmt.Sprintf("%d.log", i))
			if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
				t.Errorf("file must remain: %s: %q, %v", path, data, err)
			}
		}
	})
}
