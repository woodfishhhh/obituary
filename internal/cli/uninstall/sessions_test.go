package uninstall

import (
	"errors"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPurgeSessionMetadata(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		t.Run(map[bool]string{false: "owned", true: "unknown"}[unknown], func(t *testing.T) {
			d, _, _ := fixture(t)
			home := os.Getenv("OBIT_HOME")
			if err := (ledger.Store{Home: home}).Update(func(*ledger.State) error { return nil }); err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(home, "sessions", "123456abcdef-session.json"), `{"project":"C:\\fixture","at":"2026-11-10T00:00:00Z"}`)
			write(t, filepath.Join(home, "sessions", "123456abcdef1234.finish"), "")
			name := "grave-notices-" + strings.Repeat("a", 64) + ".json"
			write(t, filepath.Join(home, "sessions", name), `{"obituaryGraveNotices":1,"seen":[42]}`)
			write(t, filepath.Join(home, "sessions", name+".lock"), "")
			if unknown {
				write(t, filepath.Join(home, "user.txt"), "USER CONTENT")
			}
			before := snapshot(t, home)
			_, err := execute(t, d, "", "--yes", "--purge-data")
			if unknown {
				if err == nil || !reflect.DeepEqual(before, snapshot(t, home)) {
					t.Fatal("preflight changed data", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(home); !errors.Is(err, os.ErrNotExist) {
					t.Fatal(err)
				}
			}
		})
	}
}
