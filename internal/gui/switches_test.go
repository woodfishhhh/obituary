package gui

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/runkey"
)

type failedRun struct{ runkey.Registry }

func (failedRun) Read(string) (runkey.Value, error) {
	return runkey.Value{}, errors.New("registry denied")
}

func TestContextMenuPlanApplyAndFailure(t *testing.T) {
	testenv.WindowsOnly(t, "Windows desktop shortcuts, Explorer menu, or Run registration")
	a, _ := fixture(t)
	reg := a.deps.ContextMenu.(*fakeMenu)
	plan, err := a.PlanContextMenu(true)
	if err != nil || len(plan.Changes) == 0 || len(reg.trees) != 0 {
		t.Fatalf("preview: %+v %v", plan, err)
	}
	if _, err := a.ApplyPlan(plan.ID); err != nil {
		t.Fatal(err)
	}
	st, err := a.Settings()
	if err != nil || !st.ContextMenuCurrent {
		t.Fatalf("installed: %+v %v", st, err)
	}
	plan, err = a.PlanContextMenu(false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.ApplyPlan(plan.ID); err != nil || len(reg.trees) != 0 {
		t.Fatalf("uninstall: %+v %v", reg, err)
	}
	reg.err = errors.New("registry denied")
	if _, err := a.PlanContextMenu(true); err == nil {
		t.Fatal("registry error hidden")
	}
	if _, err := a.Settings(); err == nil {
		t.Fatal("inspect error hidden")
	}
	reg.err = nil
	plan, err = a.PlanContextMenu(true)
	if err != nil {
		t.Fatal(err)
	}
	reg.trees[plan.Changes[0].Key] = contextmenu.Tree{"": {"foreign": "do not overwrite"}}
	if receipt, err := a.ApplyPlan(plan.ID); err != nil || receipt.Error == "" || len(receipt.Applied) != 0 || len(receipt.Remaining) != len(plan.Changes) {
		t.Fatalf("stale registry plan applied: %+v %v", receipt, err)
	}
	delete(reg.trees, plan.Changes[0].Key)
	a.deps.Executable = filepath.Join(a.deps.ConfigDir, "missing.exe")
	if _, err := a.PlanContextMenu(true); err == nil {
		t.Fatal("installed missing executable")
	}
}

func TestContextMenuPlanReturnsPartialReceipt(t *testing.T) {
	testenv.WindowsOnly(t, "Windows desktop shortcuts, Explorer menu, or Run registration")
	a, _ := fixture(t)
	reg := a.deps.ContextMenu.(*fakeMenu)
	plan, err := a.PlanContextMenu(true)
	if err != nil {
		t.Fatal(err)
	}
	reg.failAt = 3
	receipt, err := a.ApplyPlan(plan.ID)
	if err != nil || receipt.Error == "" || !receipt.Changed || len(receipt.Applied) != 2 || len(receipt.Remaining) != len(plan.Changes)-2 {
		t.Fatalf("partial registry receipt lost to binding error: %+v %v", receipt, err)
	}
	for i, path := range receipt.Applied {
		if path != plan.Changes[i].Key || reg.trees[path] == nil {
			t.Fatalf("missing committed path: %s", path)
		}
	}
	if receipt.Remaining[0] != plan.Changes[2].Key {
		t.Fatalf("failed path absent from receipt: %+v", receipt)
	}
}

func TestAutostartAndPausePlanApplyAndFailures(t *testing.T) {
	testenv.WindowsOnly(t, "Windows desktop shortcuts, Explorer menu, or Run registration")
	a, _ := fixture(t)
	for _, enabled := range []bool{true, false} {
		plan, err := a.PlanAutostart(enabled)
		if err != nil {
			t.Fatal(err)
		}
		before, err := a.Settings()
		if err != nil || before.Autostart == enabled {
			t.Fatalf("preview mutated run key: %+v %v", before, err)
		}
		if _, err := a.ApplyPlan(plan.ID); err != nil {
			t.Fatal(err)
		}
		after, err := a.Settings()
		if err != nil || after.Autostart != enabled {
			t.Fatalf("autostart: %+v %v", after, err)
		}
		plan, err = a.PlanPause(enabled)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := a.ApplyPlan(plan.ID); err != nil {
			t.Fatal(err)
		}
		paused, err := app.Paused(a.deps.Service.Store.Home)
		if err != nil || paused != enabled {
			t.Fatalf("pause=%v %v", paused, err)
		}
	}
	plan, err := a.PlanPause(true)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.SetPaused(a.deps.Service.Store.Home, true); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ApplyPlan(plan.ID); err == nil {
		t.Fatal("stale pause plan accepted")
	}
	a.deps.RunKey = failedRun{a.deps.RunKey}
	if _, err := a.PlanAutostart(true); err == nil {
		t.Fatal("registry error hidden")
	}
	a.deps.Service.Store.Home = "bad\x00home"
	if _, err := a.PlanPause(true); err == nil {
		t.Fatal("pause error hidden")
	}
}

func TestOnlyCurrentPlanCanBeApplied(t *testing.T) {
	testenv.WindowsOnly(t, "Windows desktop shortcuts, Explorer menu, or Run registration")
	a, _ := fixture(t)
	first, err := a.PlanAutostart(true)
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.PlanPause(true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.ApplyPlan(first.ID); err == nil {
		t.Fatal("obsolete plan accepted")
	}
	if _, err := a.ApplyPlan(second.ID); err != nil {
		t.Fatal(err)
	}
}

// The confirmation shows where each entry appears and what it is called, not
// registry paths or raw trees.
func TestContextMenuPreviewIsReadable(t *testing.T) {
	testenv.WindowsOnly(t, "Windows desktop shortcuts, Explorer menu, or Run registration")
	a, _ := fixture(t)
	a.deps.SystemLanguage = func() string { return "zh-CN" }
	plan, err := a.PlanContextMenu(true)
	if err != nil || len(plan.Changes) == 0 {
		t.Fatalf("plan: %+v %v", plan, err)
	}
	seen := map[string]bool{}
	for _, c := range plan.Changes {
		if c.Key == "" || strings.Contains(c.Target, `\`) || strings.ContainsAny(c.After, "{}\\") || c.After == "" || c.Before != "" {
			t.Fatalf("unreadable install preview: %+v", c)
		}
		seen[c.Target+"="+c.After] = true
	}
	for _, want := range []string{"文件的右键菜单=写讣告", "文件夹空白处的右键菜单=设置文件夹策略", "「写讣告」的子菜单=1 小时、1 天、7 天、30 天"} {
		if !seen[want] {
			t.Fatalf("preview lacks %q: %+v", want, plan.Changes)
		}
	}
}
