package contextmenu

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

func TestContextMenuActionsAndNotifications(t *testing.T) {
	paths := []string{`C:\one & two %PATH% $(echo)!.txt`, `C:\中文 😀\two.txt`}
	for _, action := range []string{"resurrect", "policy"} {
		for _, failed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%v", action, failed), func(t *testing.T) {
				calls, notices := 0, 0
				failure := errors.New("operation failed")
				err := Run(append([]string{`C:\Program Files\obit.exe`, action}, paths...), func(exe string, args []string) ([]byte, error) {
					calls++
					want := append([]string{"context-menu", action, "--"}, paths...)
					if strings.HasSuffix(action, "d") || action == "1h" {
						want = append(append([]string{"mark", "--"}, paths...), action)
					}
					if exe != `C:\Program Files\obit.exe` || !reflect.DeepEqual(args, want) {
						t.Fatal(exe, args)
					}
					if failed {
						return []byte("permission denied"), failure
					}
					return []byte("done"), nil
				}, func(title, message string) error {
					notices++
					if failed && (title != config.UILanguage().Text(i18n.OperationIncomplete) || message != "permission denied") {
						t.Fatal(title, message)
					}
					if !failed && (title != config.UILanguage().Text(i18n.OperationComplete) || message != "done") {
						t.Fatal(title, message)
					}
					return nil
				})
				if calls != 1 || notices != 1 || errors.Is(err, failure) != failed {
					t.Fatal(calls, notices, err)
				}
			})
		}
	}
}

func TestContextMenuNotificationLanguages(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OBIT_CONFIG_DIR", root)
	for _, tc := range []struct{ language, success, failure, message string }{
		{"zh-CN", "讣告：操作完成", "讣告：操作未完成", "操作已完成。"},
		{"en", "Obituary: operation completed", "Obituary: operation not completed", "Operation completed."},
	} {
		if err := os.WriteFile(filepath.Join(root, "config.toml"), []byte("[ui]\nlanguage='"+tc.language+"'"), 0o600); err != nil {
			t.Fatal(err)
		}
		for _, failed := range []bool{false, true} {
			failure := errors.New("permission denied")
			notices := 0
			err := Run([]string{`C:\obit.exe`, "policy", `C:\file.txt`}, func(string, []string) ([]byte, error) {
				if failed {
					return nil, failure
				}
				return nil, nil
			}, func(title, message string) error {
				notices++
				if failed && (title != tc.failure || message != failure.Error()) || !failed && (title != tc.success || message != tc.message) {
					t.Fatalf("notification: %s %s", title, message)
				}
				return nil
			})
			if notices != 1 || errors.Is(err, failure) != failed {
				t.Fatalf("notification outcome: %d %v", notices, err)
			}
		}
	}
}

func TestContextMenuInvalidActionIsSilent(t *testing.T) {
	for _, args := range [][]string{nil, {"obit.exe", "1d", "file"}, {`C:\obit.exe`, "purge", "file"}} {
		err := Run(args, func(string, []string) ([]byte, error) { t.Fatal("executed invalid action"); return nil, nil }, func(string, string) error { t.Fatal("notified invalid action"); return nil })
		if err == nil {
			t.Fatal("accepted", args)
		}
	}
}

func TestContextMenuExecuteLiteralArguments(t *testing.T) {
	if os.Getenv("OBIT_TEST_MENU_ARGV") == "1" {
		data, _ := json.Marshal(os.Args[3:])
		fmt.Println(string(data))
		os.Exit(0)
	}
	t.Setenv("OBIT_TEST_MENU_ARGV", "1")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	args := []string{`C:\文件 & %PATH% ! ' $(echo) 😀.txt`, `C:\trailing slash\`, "", `a"b`}
	out, err := Execute(exe, append([]string{"-test.run=^TestContextMenuExecuteLiteralArguments$", "--"}, args...))
	if err != nil {
		t.Fatalf("execute: %v %s", err, out)
	}
	var got []string
	if err := json.Unmarshal(out, &got); err != nil || !reflect.DeepEqual(got, args) {
		t.Fatalf("argv: %q %s %v", got, out, err)
	}
}
