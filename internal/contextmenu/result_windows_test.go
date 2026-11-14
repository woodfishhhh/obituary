package contextmenu

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

func TestSelectionProcessesFiftyItemsWithOneNotification(t *testing.T) {
	t.Setenv("OBIT_HOME", t.TempDir())
	for _, action := range []string{"1d", "7d", "30d", "bury"} {
		t.Run(action, func(t *testing.T) {
			args := []string{`C:\obit.exe`, action}
			for i := range 50 {
				args = append(args, fmt.Sprintf(`C:\目录 %d\file & ' $(cmd) %%PATH%% 😀.txt`, i))
			}
			calls, notifications := 0, 0
			err := Run(args, func(exe string, command []string) ([]byte, error) {
				path := args[calls+2]
				calls++
				want := []string{"bury", "--", path}
				if action != "bury" {
					want = []string{"mark", "--", path, action}
				}
				if exe != args[0] || !reflect.DeepEqual(command, want) {
					t.Fatal(exe, command)
				}
				if calls > 47 {
					return []byte("permission denied: " + path), errors.New("exit status 1")
				}
				return []byte("completed: " + path), nil
			}, func(title, message string) error {
				notifications++
				if title != config.UILanguage().Text(i18n.OperationIncomplete) || !strings.Contains(message, "47") || !strings.Contains(message, "3") || !strings.Contains(message, "last-result.txt") {
					t.Fatal(title, message)
				}
				return nil
			})
			if err == nil || calls != 50 || notifications != 1 {
				t.Fatal(calls, notifications, err)
			}
			data, err := os.ReadFile(filepath.Join(os.Getenv("OBIT_HOME"), "context-menu", "last-result.txt"))
			if err != nil || !strings.Contains(string(data), "permission denied") {
				t.Fatal(string(data), err)
			}
			for _, path := range args[2:] {
				if !strings.Contains(string(data), path) {
					t.Fatal("result omitted", path)
				}
			}
		})
	}
}

func TestSelectionResultReplacesPreviousAndReportsWriteFailure(t *testing.T) {
	t.Setenv("OBIT_HOME", t.TempDir())
	path, err := saveSelectionResult("previous")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := saveSelectionResult("latest"); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "latest" {
		t.Fatal(string(data), err)
	}
	t.Setenv("OBIT_HOME", path) // a file cannot contain the result directory
	notified := false
	err = Run([]string{`C:\obit.exe`, "1d", `C:\file`}, func(string, []string) ([]byte, error) { return nil, nil }, func(title, message string) error {
		notified = title == config.UILanguage().Text(i18n.OperationIncomplete)
		return nil
	})
	if err == nil || !notified {
		t.Fatal("write failure hidden", err)
	}
}

func TestSelectionSummariesLocalized(t *testing.T) {
	for _, tc := range []struct {
		language     i18n.Language
		action, want string
	}{
		{i18n.Chinese, "1d", "已放进讣告 2 项，1 天后下葬，1 项失败。"},
		{i18n.English, "7d", "Added 2 items to Obituary; burial in 7 days; 1 failed."},
		{i18n.Chinese, "bury", "已下葬 2 项，1 项失败。"},
		{i18n.English, "bury", "Buried 2 items; 1 failed."},
	} {
		if got := selectionSummary(tc.language, tc.action, 2, 1); got != tc.want {
			t.Fatal(got)
		}
	}
}

func TestConcurrentSelectionsPublishOneCompleteResult(t *testing.T) {
	t.Setenv("OBIT_HOME", t.TempDir())
	var wg sync.WaitGroup
	for _, content := range []string{strings.Repeat("a", 4096), strings.Repeat("b", 2048)} {
		wg.Go(func() {
			if _, err := saveSelectionResult(content); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	data, err := os.ReadFile(filepath.Join(os.Getenv("OBIT_HOME"), "context-menu", "last-result.txt"))
	if err != nil || string(data) != strings.Repeat("a", 4096) && string(data) != strings.Repeat("b", 2048) {
		t.Fatal("interleaved result", string(data), err)
	}
}
