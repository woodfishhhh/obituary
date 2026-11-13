package gui

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/contextmenu"
)

func TestLanguageSettingsAndPlan(t *testing.T) {
	testenv.WindowsOnly(t, "Windows desktop shortcuts, Explorer menu, or Run registration")
	a, _ := fixture(t)
	a.deps.SystemLanguage = func() string { return "zh-TW" }
	settings, err := a.Settings()
	if err != nil || settings.Language != "auto" || settings.EffectiveLanguage != "zh-CN" {
		t.Fatalf("defaults: %+v %v", settings, err)
	}
	data, err := json.Marshal(settings)
	if err != nil || !strings.Contains(string(data), `"language":"auto"`) || !strings.Contains(string(data), `"effectiveLanguage":"zh-CN"`) {
		t.Fatalf("frontend contract: %s %v", data, err)
	}
	plan, err := a.PlanLanguage("en")
	if err != nil || plan.Action != "language" || len(plan.Changes) != 1 || plan.Changes[0].Before != "language=auto" || plan.Changes[0].After != "language=en effectiveLanguage=en" {
		t.Fatalf("plan: %+v %v", plan, err)
	}
	before, err := a.Settings()
	if err != nil || before.Language != "auto" || before.ContextMenuInstalled {
		t.Fatalf("preview changed state: %+v %v", before, err)
	}
	receipt, err := a.ApplyPlan(plan.ID)
	if err != nil || receipt.Error != "" || !receipt.Changed || !reflect.DeepEqual(receipt.Applied, []string{settings.ConfigPath}) || len(receipt.Remaining) != 0 {
		t.Fatalf("apply: %+v %v", receipt, err)
	}
	after, err := a.Settings()
	if err != nil || after.Language != "en" || after.EffectiveLanguage != "en" || after.ContextMenuInstalled {
		t.Fatalf("applied: %+v %v", after, err)
	}
	if _, err := a.ApplyPlan(plan.ID); err == nil {
		t.Fatal("plan replayed")
	}
	if _, err := a.PlanLanguage("fr"); err == nil {
		t.Fatal("invalid language accepted")
	}
	a.deps.SystemLanguage = func() string { return "de-DE" }
	plan, err = a.PlanLanguage("auto")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.ApplyPlan(plan.ID); err != nil {
		t.Fatal(err)
	}
	after, err = a.Settings()
	if err != nil || after.Language != "auto" || after.EffectiveLanguage != "en" {
		t.Fatalf("auto: %+v %v", after, err)
	}
}

func TestLanguagePlanIncludesInstalledMenu(t *testing.T) {
	testenv.WindowsOnly(t, "Windows desktop shortcuts, Explorer menu, or Run registration")
	a, _ := fixture(t)
	a.deps.SystemLanguage = func() string { return "zh-CN" }
	menu, err := a.PlanContextMenu(true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.ApplyPlan(menu.ID); err != nil {
		t.Fatal(err)
	}
	reg := a.deps.ContextMenu.(*fakeMenu)
	const markPath = `*\shell\Obituary.Mark`
	plan, err := a.PlanLanguage("en")
	if err != nil || len(plan.Changes) != 8 || reg.trees[markPath][""]["MUIVerb"] != "写讣告" {
		t.Fatalf("combined preview: %+v %v", plan, err)
	}
	found := false
	for _, change := range plan.Changes {
		if change.Key == markPath {
			found = strings.Contains(change.Before, "写讣告") && strings.Contains(change.After, "Give a lifespan")
		}
	}
	if !found {
		t.Fatal("menu translation absent from preview")
	}
	receipt, err := a.ApplyPlan(plan.ID)
	if err != nil || receipt.Error != "" || len(receipt.Applied) != 8 || len(receipt.Remaining) != 0 {
		t.Fatalf("combined apply: %+v %v", receipt, err)
	}
	settings, err := a.Settings()
	if err != nil || settings.Language != "en" || !settings.ContextMenuCurrent || reg.trees[markPath][""]["MUIVerb"] != "Give a lifespan" {
		t.Fatalf("settings/menu differ: %+v %v", settings, err)
	}
	plan, err = a.PlanLanguage("en")
	if err != nil {
		t.Fatal(err)
	}
	receipt, err = a.ApplyPlan(plan.ID)
	if err != nil || receipt.Error != "" || receipt.Changed || len(receipt.Applied) != 0 {
		t.Fatalf("unchanged: %+v %v", receipt, err)
	}
}

func TestLanguagePlanRetainsPartialReceipt(t *testing.T) {
	testenv.WindowsOnly(t, "Windows desktop shortcuts, Explorer menu, or Run registration")
	a, _ := fixture(t)
	a.deps.SystemLanguage = func() string { return "zh-CN" }
	menu, err := a.PlanContextMenu(true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.ApplyPlan(menu.ID); err != nil {
		t.Fatal(err)
	}
	plan, err := a.PlanLanguage("en")
	if err != nil {
		t.Fatal(err)
	}
	reg := a.deps.ContextMenu.(*fakeMenu)
	reg.failAt = reg.writes + 3
	receipt, err := a.ApplyPlan(plan.ID)
	if err != nil || receipt.Error == "" || !receipt.Changed || len(receipt.Applied) != 3 || len(receipt.Remaining) != 5 || receipt.Applied[0] != plan.Changes[0].Key {
		t.Fatalf("partial receipt: %+v %v", receipt, err)
	}
	settings, err := a.Settings()
	if err != nil || settings.Language != "en" || settings.ContextMenuCurrent {
		t.Fatalf("partial state: %+v %v", settings, err)
	}
	reg.failAt = 0
	plan, err = a.PlanLanguage("en")
	if err != nil || len(plan.Changes) != 6 {
		t.Fatalf("repair preview: %+v %v", plan, err)
	}
	receipt, err = a.ApplyPlan(plan.ID)
	if err != nil || receipt.Error != "" || len(receipt.Applied) != 5 || len(receipt.Remaining) != 0 {
		t.Fatalf("repair: %+v %v", receipt, err)
	}
	settings, err = a.Settings()
	if err != nil || !settings.ContextMenuCurrent {
		t.Fatalf("repair state: %+v %v", settings, err)
	}
}

func TestLanguagePlanPreservesConcurrentEdits(t *testing.T) {
	testenv.WindowsOnly(t, "Windows desktop shortcuts, Explorer menu, or Run registration")
	for _, changed := range []string{"config", "registry"} {
		t.Run(changed, func(t *testing.T) {
			a, _ := fixture(t)
			a.deps.SystemLanguage = func() string { return "zh-CN" }
			menu, err := a.PlanContextMenu(true)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := a.ApplyPlan(menu.ID); err != nil {
				t.Fatal(err)
			}
			plan, err := a.PlanLanguage("en")
			if err != nil {
				t.Fatal(err)
			}
			reg := a.deps.ContextMenu.(*fakeMenu)
			const path = `*\shell\Obituary.Mark`
			if changed == "config" {
				write(t, filepath.Join(a.deps.ConfigDir, "config.toml"), "# editor\n[ui]\nlanguage='zh-CN'\n")
			} else {
				reg.trees[path] = contextmenu.Tree{"": {"MUIVerb": "user edit"}}
			}
			writes := reg.writes
			receipt, err := a.ApplyPlan(plan.ID)
			if err != nil || receipt.Error == "" || reg.writes != writes {
				t.Fatalf("stale plan: %+v %v", receipt, err)
			}
			if changed == "config" && (receipt.Changed || len(receipt.Applied) != 0 || len(receipt.Remaining) != len(plan.Changes)) {
				t.Fatalf("config conflict: %+v", receipt)
			}
			if changed == "registry" && (len(receipt.Applied) != 1 || reg.trees[path][""]["MUIVerb"] != "user edit") {
				t.Fatalf("registry conflict: %+v", receipt)
			}
		})
	}
}
