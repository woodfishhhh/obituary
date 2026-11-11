package integrate

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func vibeBlock(command string) TOMLBlock {
	return TOMLBlock{
		ID: "vibe-hooks", Content: []byte("[[hooks]]\nname = 'obituary'\ntype = 'pre_tool'\ncommand = '" + command + "'\n"),
		Owns: func(full map[string]any) bool {
			entries, ok := full["hooks"].([]any)
			if !ok || len(full) != 1 || len(entries) != 1 {
				return false
			}
			entry, ok := entries[0].(map[string]any)
			cmd, _ := entry["command"].(string)
			return ok && OwnedHookCommand(cmd, "vibe", nil)
		},
		Refresh: func(block []byte) ([]byte, error) {
			return RefreshTOMLString(block, []string{"hooks"}, "command", command)
		},
	}
}

func TestTOMLBlockLifecycleCommentsAndScope(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "hooks.toml")
	input := "\xef\xbb\xbf# user settings\nroot = 'value'\n[[hooks]]\nname = 'user'\ncommand = 'echo user' # user command\n"
	writeIntegrationFile(t, path, input)
	ch, err := PlanTOMLBlock(s, path, vibeBlock("C:/tools/obit.exe hook vibe"), false)
	applyIntegration(t, []Change{ch}, err)
	installed := read(t, path)
	writeIntegrationFile(t, path, strings.Replace(installed, "command = 'C:/tools/obit.exe hook vibe'", "command = 'C:/tools/obit.exe hook vibe' # owned comment", 1))
	for _, command := range []string{"C:/new/obit.exe hook vibe", "C:/new/obit.exe hook vibe"} {
		ch, err = PlanTOMLBlock(s, path, vibeBlock(command), false)
		applyIntegration(t, []Change{ch}, err)
		got := read(t, path)
		if !strings.HasPrefix(got, input) || !strings.Contains(got, "# owned comment") || !strings.Contains(got, command) {
			t.Fatal("comments/prefix changed", got)
		}
	}
	ch, err = PlanTOMLBlock(s, path, vibeBlock("C:/new/obit.exe hook vibe"), false)
	if err != nil || ch.Action != "unchanged" {
		t.Fatalf("not idempotent: %+v %v", ch, err)
	}
	ch, err = PlanTOMLBlock(s, path, vibeBlock(""), true)
	applyIntegration(t, []Change{ch}, err)
	if strings.TrimSpace(read(t, path)) != strings.TrimSpace(input) || read(t, testBackupPath(t, path)) != input {
		t.Fatal("uninstall or backup changed original", read(t, path))
	}
}

func TestTOMLBlockUninstallRestoresOriginalNewlines(t *testing.T) {
	t.Parallel()
	for _, original := range []string{"", "# user", "# user\n", "# user\n\n", "\xef\xbb\xbf# user\r\n"} {
		t.Run(original, func(t *testing.T) {
			t.Parallel()
			s := Scope{Project: t.TempDir()}
			path := filepath.Join(s.Project, "hooks.toml")
			writeIntegrationFile(t, path, original)
			for _, uninstall := range []bool{false, false, true} {
				ch, err := PlanTOMLBlock(s, path, vibeBlock("C:/tools/obit.exe hook vibe"), uninstall)
				applyIntegration(t, []Change{ch}, err)
			}
			if got := read(t, path); got != original {
				t.Fatalf("TOML changed original newline: got %q want %q", got, original)
			}
		})
	}
}

func TestTOMLBlockPreservesLaterUserArrayEntries(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "hooks.toml")
	before := "[[hooks]]\ncommand='first user'\n"
	after := "[[hooks]]\ncommand='later user' # added later\n"
	writeIntegrationFile(t, path, before)
	ch, err := PlanTOMLBlock(s, path, vibeBlock("C:/tools/obit.exe hook vibe"), false)
	applyIntegration(t, []Change{ch}, err)
	writeIntegrationFile(t, path, read(t, path)+after)
	ch, err = PlanTOMLBlock(s, path, vibeBlock("C:/new/obit.exe hook vibe"), false)
	applyIntegration(t, []Change{ch}, err)
	ch, err = PlanTOMLBlock(s, path, vibeBlock(""), true)
	applyIntegration(t, []Change{ch}, err)
	if !strings.HasPrefix(read(t, path), before) || !strings.HasSuffix(read(t, path), after) {
		t.Fatal("user hook order or text changed", read(t, path))
	}
}

func TestTOMLBlockRejectsScopeLeaksAndForgedMarkers(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	for _, input := range []string{
		"# obit:vibe-hooks:begin\n[[hooks]]\ncommand='C:/tools/obit.exe hook vibe'\n# obit:vibe-hooks:end\nuser='belongs to hook'\n",
		"# obit:vibe-hooks:begin\n[[hooks]]\ncommand='user'\n# obit:vibe-hooks:end\n",
		"# obit:vibe-hooks:begin\n[[hooks]]\ncommand='C:/tools/obit.exe hook vibe'\n",
		"# obit:vibe-hooks:end\n",
		"# obit:vibe-hooks:begin\n# obit:vibe-hooks:begin\n# obit:vibe-hooks:end\n",
		"broken = [",
	} {
		s := Scope{Project: t.TempDir()}
		path := filepath.Join(s.Project, "hooks.toml")
		writeIntegrationFile(t, path, input)
		for _, uninstall := range []bool{false, true} {
			if _, err := PlanTOMLBlock(s, path, vibeBlock("C:/tools/obit.exe hook vibe"), uninstall); err == nil {
				t.Fatal("accepted unsafe TOML", input)
			}
		}
		if read(t, path) != input {
			t.Fatal("planner wrote config")
		}
	}
	input := "text = '''\n# obit:vibe-hooks:begin\n# obit:vibe-hooks:end\n'''\n"
	start, end, err := tomlMarkedRange([]byte(input), "# obit:vibe-hooks:begin", "# obit:vibe-hooks:end")
	if err != nil || start != -1 || end != -1 {
		t.Fatalf("string marker interpreted as ownership: %d %d %v", start, end, err)
	}
}

func TestRefreshTOMLStringUsesASTRanges(t *testing.T) {
	t.Parallel()
	input := []byte("# c\r\n[[hooks]]\r\ncommand = '''old\nmultiline''' # after\r\nother='value'\r\n[[hooks]]\r\ncommand='old'\r\n[unrelated]\r\ncommand='keep'\r\n")
	out, err := RefreshTOMLString(input, []string{"hooks"}, "command", `C:\space & tools\obit.exe hook vibe`)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := toml.Unmarshal(out, &parsed); err != nil || strings.Count(string(out), "space & tools") != 2 || !strings.Contains(string(out), "# after\r\nother='value'") || !strings.HasSuffix(string(out), "command='keep'\r\n") {
		t.Fatalf("range replacement: %s %v", out, err)
	}
}
