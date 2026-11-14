//go:build windows

// Package contextmenu owns the opt-in classic Explorer verbs, not background notifications.
package contextmenu

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"golang.org/x/sys/windows"
)

const AppID = "Obituary.ContextMenu"
const ownerName = "ObituaryOwner"
const manifestName = "ObituaryManifest"
const owner = "obituary-context-menu-v1"

// Tree contains relative key paths and REG_SZ values. A nil Tree means absent.
type Tree map[string]map[string]string

// Registry is rooted at HKCU\Software\Classes in production, a disposable key in tests.
type Registry interface {
	ReadTree(path string) (Tree, error)
	// CompareAndSwap rechecks Before and mutates through the same native boundary.
	CompareAndSwap(change Change) error
	Lock() (func(), error)
}

type Change struct {
	Path          string
	Before, After Tree
}

// Plan inspects existing registration before proposing installation changes.
// The registry manifest proves the whole owned subtree; foreign edits are preserved.
func Plan(reg Registry, executable string, uninstall bool) ([]Change, error) {
	return PlanLanguage(reg, executable, uninstall, config.UILanguage())
}

// PlanLanguage previews the requested language without changing config or keys.
func PlanLanguage(reg Registry, executable string, uninstall bool, language i18n.Language) ([]Change, error) {
	wanted, err := registration(executable, language)
	if err != nil {
		return nil, err
	}
	var changes []Change
	for _, path := range sortedKeys(wanted) {
		before, err := reg.ReadTree(path)
		if err != nil {
			return nil, fmt.Errorf("inspect %s: %w", path, err)
		}
		if before != nil && !owned(before) {
			return nil, fmt.Errorf("preserved %s: not owned by Obituary or changed since installation", path)
		}
		after := wanted[path]
		if uninstall {
			after = nil
		}
		if !reflect.DeepEqual(before, after) {
			changes = append(changes, Change{path, before, after})
		}
	}
	return changes, nil
}

func Apply(reg Registry, changes []Change) error {
	_, err := ApplyWithReceipt(reg, changes)
	return err
}

type Status struct {
	Installed, Current bool
	Detail             string
}

func Inspect(reg Registry, executable string) (Status, error) {
	return InspectLanguage(reg, executable, config.UILanguage())
}

func InspectLanguage(reg Registry, executable string, language i18n.Language) (Status, error) {
	wanted, err := registration(executable, language)
	if err != nil {
		return Status{}, err
	}
	st := Status{Current: true}
	var entries []string
	for _, path := range sortedKeys(wanted) {
		tree, err := reg.ReadTree(path)
		if err != nil {
			return st, err
		}
		st.Installed = st.Installed || tree != nil
		entries = append(entries, registrationDetails(path, tree, wanted[path])...)
		if !reflect.DeepEqual(tree, wanted[path]) {
			st.Current = false
			st.Detail += path + "; "
		}
	}
	if !st.Installed {
		st.Detail = "not installed; obit init --context-menu --yes"
	} else if st.Current {
		st.Detail = "classic context menu installed; commands use current executable: " + executable
	} else {
		st.Detail = "incomplete, modified or commands do not use current executable (" + executable + "): " + st.Detail
	}
	st.Detail += "\n" + strings.Join(entries, "\n")
	return st, nil
}

func registrationDetails(path string, actual, wanted Tree) []string {
	var details []string
	for _, key := range sortedKeys(wanted) {
		label := wanted[key]["MUIVerb"]
		if label == "" {
			label = wanted[key]["DisplayName"]
		}
		if label == "" {
			continue
		}
		status := "PASS"
		for child, values := range wanted {
			if (key == "" || child == key || strings.HasPrefix(child, key+`\`)) && !reflect.DeepEqual(actual[child], values) {
				status = "WARN"
			}
		}
		if actual == nil {
			status = "INFO (missing)"
		}
		details = append(details, fmt.Sprintf("%s %s [%s] %s", status, path, key, label))
	}
	return details
}

func owned(tree Tree) bool {
	root := tree[""]
	if root[ownerName] != owner {
		return false
	}
	var saved Tree
	if json.Unmarshal([]byte(root[manifestName]), &saved) != nil {
		return false
	}
	actual := maps.Clone(tree)
	actual[""] = maps.Clone(root)
	delete(actual[""], ownerName)
	delete(actual[""], manifestName)
	return reflect.DeepEqual(actual, saved)
}

func registration(executable string, language i18n.Language) (map[string]Tree, error) {
	abs, err := pathutil.Abs(executable)
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(executable) || strings.ContainsAny(executable, "\x00\"") {
		return nil, errors.New("context menu requires an absolute executable path without quotes")
	}
	daemon := filepath.Join(filepath.Dir(abs), "obitd.exe")
	command := func(action, operand string) string {
		// No shell: Explorer -> GUI obitd -> CreateProcess(obit.exe). Quote even
		// placeholder arguments with no spaces; Explorer substitutes them later.
		return windows.ComposeCommandLine([]string{daemon, "--context-menu", abs, action}) + " \"" + operand + "\""
	}
	wanted := map[string]Tree{AppID: markTree(language)}
	for action, clsid := range dropClasses {
		wanted[`CLSID\`+clsid] = Tree{
			"":              {"DisplayName": language.Text(i18n.AppName)},
			"LocalServer32": {"": windows.ComposeCommandLine([]string{daemon, "--context-menu-server", abs, action})},
		}
	}
	for _, association := range []string{"*", "Directory"} {
		wanted[association+`\shell\Obituary.Mark`] = Tree{"": {
			"MUIVerb": language.Text(i18n.Mark), "ExtendedSubCommandsKey": AppID, "MultiSelectModel": "Player",
		}}
		wanted[association+`\shell\Obituary.Bury`] = Tree{
			"":           {"MUIVerb": language.Text(i18n.BuryNow), "MultiSelectModel": "Player"},
			"DropTarget": {"CLSID": dropClasses["bury"]},
		}
		wanted[association+`\shell\Obituary.Resurrect`] = Tree{
			"":        {"MUIVerb": language.Text(i18n.ResurrectLatest), "MultiSelectModel": "Single"},
			"command": {"": command("resurrect", "%1")},
		}
	}
	wanted[`Directory\Background\shell\Obituary.Policy`] = Tree{
		// A volume root ends in a backslash, which would escape the closing
		// argv quote. The directory's \. suffix preserves its meaning.
		"": {"MUIVerb": language.Text(i18n.FolderPolicy)}, "command": {"": command("policy", `%V\.`)},
	}
	wanted[`AppUserModelId\`+AppID] = Tree{"": {"DisplayName": language.Text(i18n.AppName)}}
	for _, tree := range wanted {
		data, err := json.Marshal(tree)
		if err != nil {
			return nil, err
		}
		tree[""][ownerName], tree[""][manifestName] = owner, string(data)
	}
	return wanted, nil
}

func markTree(language i18n.Language) Tree {
	// Microsoft Learn: "Creating Cascading Menus with ExtendedSubCommandsKey".
	// A shared verb set under HKCU\Software\Classes avoids HKLM CommandStore.
	tree := Tree{"": {}, "shell": {}}
	for i, p := range []struct {
		key      i18n.Key
		duration string
	}{{i18n.OneDay, "1d"}, {i18n.SevenDays, "7d"}, {i18n.ThirtyDays, "30d"}} {
		key := fmt.Sprintf(`shell\%02d`, i+1)
		tree[key] = map[string]string{"MUIVerb": language.Text(p.key), "MultiSelectModel": "Player"}
		tree[key+`\DropTarget`] = map[string]string{"CLSID": dropClasses[p.duration]}
	}
	return tree
}
