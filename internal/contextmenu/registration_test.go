//go:build windows

package contextmenu

import (
	"reflect"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"golang.org/x/sys/windows"
)

func TestContextMenuInstallUpdateUninstall(t *testing.T) {
	reg := WindowsRegistry{Root: testenv.ClassesKey(t)}
	exe := `C:\Program Files\讣告 & 资料\obit.exe`
	foreign := Tree{"": {"": "other app"}}
	if err := reg.WriteTree(`*\shell\Other`, foreign); err != nil {
		t.Fatal(err)
	}
	plan, err := Plan(reg, exe, false)
	if err != nil || len(plan) != 13 {
		t.Fatalf("plan: %d %v", len(plan), err)
	}
	if st, err := Inspect(reg, exe); err != nil || st.Installed {
		t.Fatalf("planning wrote: %+v %v", st, err)
	}
	if err := Apply(reg, plan); err != nil {
		t.Fatal(err)
	}
	st, err := Inspect(reg, exe)
	if err != nil || !st.Installed || !st.Current {
		t.Fatalf("installed: %+v %v", st, err)
	}
	if plan, err := Plan(reg, exe, false); err != nil || len(plan) != 0 {
		t.Fatalf("idempotency: %+v %v", plan, err)
	}
	tree, err := reg.ReadTree(AppID)
	if err != nil {
		t.Fatal(err)
	}
	for i, duration := range []string{"1d", "7d", "30d"} {
		key := []string{`shell\01`, `shell\02`, `shell\03`}[i]
		if tree[key]["MultiSelectModel"] != "Player" {
			t.Fatal("multi-select missing")
		}
		clsid := tree[key+`\DropTarget`]["CLSID"]
		server, err := reg.ReadTree(`CLSID\` + clsid)
		if err != nil || clsid != dropClasses[duration] {
			t.Fatal(clsid, err)
		}
		command := server["LocalServer32"][""]
		args, err := windows.DecomposeCommandLine(command)
		want := []string{`C:\Program Files\讣告 & 资料\obitd.exe`, "--context-menu-server", exe, duration}
		if err != nil || !reflect.DeepEqual(args, want) {
			t.Fatalf("quoted command: %q %v", args, err)
		}
	}
	for _, assoc := range []string{"*", "Directory"} {
		tree, _ := reg.ReadTree(assoc + `\shell\Obituary.Mark`)
		if tree[""]["ExtendedSubCommandsKey"] != AppID || tree[""]["MUIVerb"] != config.UILanguage().Text(i18n.Mark) {
			t.Fatal(tree)
		}
		bury, err := reg.ReadTree(assoc + `\shell\Obituary.Bury`)
		if err != nil || bury[""]["MultiSelectModel"] != "Player" || bury["DropTarget"]["CLSID"] != dropClasses["bury"] || bury[""]["MUIVerb"] != config.UILanguage().Text(i18n.BuryNow) {
			t.Fatal(bury, err)
		}
	}
	background, err := reg.ReadTree(`Directory\Background\shell\Obituary.Policy`)
	if err != nil {
		t.Fatal(err)
	}
	rootArgs, err := windows.DecomposeCommandLine(strings.ReplaceAll(background["command"][""], "%V", `C:\`))
	if err != nil || len(rootArgs) != 5 || rootArgs[4] != `C:\\.` {
		t.Fatalf("background volume root: %q %v", rootArgs, err)
	}
	newExe := `D:\Updated\obit.exe`
	if st, err := Inspect(reg, newExe); err != nil || st.Current {
		t.Fatal(st, err)
	}
	plan, err = Plan(reg, newExe, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(reg, plan); err != nil {
		t.Fatal(err)
	}
	if st, err := Inspect(reg, newExe); err != nil || !st.Current {
		t.Fatal(st, err)
	}
	plan, err = Plan(reg, exe, true) // uninstall still works from the old binary location
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(reg, plan); err != nil {
		t.Fatal(err)
	}
	if st, err := Inspect(reg, exe); err != nil || st.Installed {
		t.Fatal(st, err)
	}
	if got, err := reg.ReadTree(`*\shell\Other`); err != nil || !reflect.DeepEqual(got, foreign) {
		t.Fatal(got, err)
	}
}

func TestContextMenuLanguageLabels(t *testing.T) {
	reg := &lockedRegistry{trees: map[string]Tree{}}
	exe := `C:\obit\obit.exe`
	var originalCommand string
	for _, tc := range []struct {
		language                    i18n.Language
		mark, restore, policy, name string
		durations                   []string
	}{
		{i18n.Chinese, "放进讣告", "复活最近下葬的", "设置文件夹策略", "讣告 Obituary", []string{"1 天", "7 天", "30 天"}},
		{i18n.English, "Add to Obituary", "Resurrect the most recent burial", "Set folder policy", "Obituary", []string{"1 day", "7 days", "30 days"}},
	} {
		before := reg.calls
		plan, err := PlanLanguage(reg, exe, false, tc.language)
		if err != nil || len(plan) != 13 || reg.calls != before {
			t.Fatalf("preview: %+v %v", plan, err)
		}
		if err := Apply(reg, plan); err != nil {
			t.Fatal(err)
		}
		for _, association := range []string{"*", "Directory"} {
			if reg.trees[association+`\shell\Obituary.Mark`][""]["MUIVerb"] != tc.mark || reg.trees[association+`\shell\Obituary.Resurrect`][""]["MUIVerb"] != tc.restore {
				t.Fatalf("%s labels differ", association)
			}
		}
		if reg.trees[`Directory\Background\shell\Obituary.Policy`][""]["MUIVerb"] != tc.policy || reg.trees[`AppUserModelId\`+AppID][""]["DisplayName"] != tc.name {
			t.Fatal("policy or app name differs")
		}
		for i, key := range []string{`shell\01`, `shell\02`, `shell\03`} {
			if reg.trees[AppID][key]["MUIVerb"] != tc.durations[i] {
				t.Fatalf("duration %s differs", key)
			}
		}
		command := reg.trees[`CLSID\`+dropClasses["1d"]][`LocalServer32`][""]
		if originalCommand != "" && command != originalCommand {
			t.Fatal("translation changed the command")
		}
		originalCommand = command
		status, err := InspectLanguage(reg, exe, tc.language)
		if err != nil || !status.Current || !status.Installed {
			t.Fatalf("inspect: %+v %v", status, err)
		}
	}
}

func TestContextMenuPreservesForeignAndChangedKeys(t *testing.T) {
	for _, mode := range []string{"foreign", "changed-value", "added-value", "added-subkey", "removed-owner"} {
		t.Run(mode, func(t *testing.T) {
			reg := WindowsRegistry{Root: testenv.ClassesKey(t)}
			exe, path := `C:\obit\obit.exe`, `*\shell\Obituary.Mark`
			if mode == "foreign" {
				if err := reg.WriteTree(path, Tree{"": {"": "mine"}}); err != nil {
					t.Fatal(err)
				}
			} else {
				plan, err := Plan(reg, exe, false)
				if err != nil {
					t.Fatal(err)
				}
				if err := Apply(reg, plan); err != nil {
					t.Fatal(err)
				}
				values := map[string]string{"MUIVerb": "my label"}
				switch mode {
				case "added-value":
					values = map[string]string{"extra": "mine"}
				case "added-subkey":
					path += `\UserChild`
				case "removed-owner":
					values = map[string]string{ownerName: "someone else"}
				}
				if err := reg.WriteTree(path, Tree{"": values}); err != nil {
					t.Fatal(err)
				}
			}
			before, err := reg.ReadTree(`*`)
			if err != nil {
				t.Fatal(err)
			}
			for _, uninstall := range []bool{false, true} {
				if _, err := Plan(reg, exe, uninstall); err == nil || !strings.Contains(err.Error(), "preserved") {
					t.Fatal(err)
				}
			}
			after, err := reg.ReadTree(`*`)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("changed foreign content", err)
			}
		})
	}
}

func TestContextMenuRejectsChangedPlanAndRelativeExecutable(t *testing.T) {
	reg := WindowsRegistry{Root: testenv.ClassesKey(t)}
	for _, path := range []string{"obit.exe", `C:\bad"path\obit.exe`} {
		if _, err := Plan(reg, path, false); err == nil {
			t.Fatal("accepted", path)
		}
	}
	plan, err := Plan(reg, `C:\obit\obit.exe`, false)
	if err != nil {
		t.Fatal(err)
	}
	foreign := Tree{"": {"": "created after plan"}}
	if err := reg.WriteTree(plan[0].Path, foreign); err != nil {
		t.Fatal(err)
	}
	if err := Apply(reg, plan); err == nil {
		t.Fatal("overwrote concurrent change")
	}
	if got, err := reg.ReadTree(plan[0].Path); err != nil || !reflect.DeepEqual(got, foreign) {
		t.Fatal(got, err)
	}
}
