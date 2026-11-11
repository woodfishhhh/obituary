package uninstall

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

type menuRegistry map[string]contextmenu.Tree

func (r menuRegistry) ReadTree(path string) (contextmenu.Tree, error) { return r[path], nil }
func (r menuRegistry) Lock() (func(), error)                          { return func() {}, nil }
func (r menuRegistry) CompareAndSwap(c contextmenu.Change) error {
	if !reflect.DeepEqual(r[c.Path], c.Before) {
		return errors.New("menu changed")
	}
	if c.After == nil {
		delete(r, c.Path)
	} else {
		r[c.Path] = c.After
	}
	return nil
}

func fixture(t *testing.T) (dependencies, string, *[]string) {
	t.Helper()
	root := t.TempDir()
	t.Chdir(root)
	for _, key := range []string{"OBIT_HOME", "OBIT_CONFIG_DIR", "OBIT_USER_HOME", "CLAUDE_CONFIG_DIR", "CODEX_HOME"} {
		t.Setenv(key, filepath.Join(root, key))
	}
	var calls []string
	d := dependencies{
		executable:  func() (string, error) { return filepath.Join(root, "bin", "obit.exe"), nil },
		runRegistry: testenv.NewRunRegistry(), menuRegistry: menuRegistry{},
		previewSchedule: func(string, string, string) string { return "remove scheduled task (owned fixture)" },
		removeSchedule:  func() error { calls = append(calls, "schedule"); return nil },
		stop:            func(home string) error { calls = append(calls, "stop:"+home); return nil },
		terminal:        func() bool { return false },
	}
	return d, root, &calls
}

func execute(t *testing.T, d dependencies, input string, args ...string) (string, error) {
	t.Helper()
	cmd := newCmd(d)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(bytes.NewBufferString(input))
	cmd.SetArgs(args)
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	err := cmd.Execute()
	return out.String(), err
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if entry.IsDir() {
			result[path] = "directory"
			return nil
		}
		data, err := os.ReadFile(path)
		result[path] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func write(t *testing.T, path, data string) { t.Helper(); clitest.WriteFixture(t, path, data) }
