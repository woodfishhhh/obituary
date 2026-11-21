package app

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type schedulerExit uint32

func schedulerUser(t *testing.T) string {
	t.Helper()
	u, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	return u.Username
}

func TestLegacyRunnerPreservesEditedAndDisabledDefinitions(t *testing.T) {
	config := t.TempDir()
	t.Setenv("OBIT_CONFIG_DIR", config)
	exe, home := "C:/bin/obitd.exe", t.TempDir()
	definition := taskXML(exe, home, schedulerUser(t), time.Now())
	if !generatedFallback(definition, exe, home, config) {
		t.Fatal("generated fallback rejected")
	}
	for _, edit := range [][2]string{
		{"PT5M", "PT20M"}, {"PT30M", "PT1H"}, {"<Enabled>true</Enabled>", "<Enabled>false</Enabled>"},
		{"LeastPrivilege", "HighestAvailable"}, {"</Settings>", "<RestartOnFailure><Interval>PT1M</Interval><Count>9</Count></RestartOnFailure></Settings>"},
	} {
		changed := strings.ReplaceAll(definition, edit[0], edit[1])
		if changed == definition || generatedFallback(changed, exe, home, config) {
			t.Errorf("accepted changed definition: %s", edit[1])
		}
	}
}

func (e schedulerExit) Error() string { return fmt.Sprintf("HRESULT %#x", uint32(e)) }
func (e schedulerExit) ExitCode() int { return int(e) }

func TestFallbackReceiptDistinguishesEditedStartFromAnotherValidInstall(t *testing.T) {
	home, config := t.TempDir(), t.TempDir()
	t.Setenv("OBIT_CONFIG_DIR", config)
	const daemon = `C:\bin\obitd.exe`
	user, at := schedulerUser(t), time.Now()
	first := taskXML(daemon, home, user, at)
	later := taskXML(daemon, home, user, at.Add(time.Hour))
	if strings.Replace(first, taskStartBoundary(first), taskStartBoundary(later), 1) != later {
		t.Fatal("fixture changed more than StartBoundary")
	}
	if ownedSchedule(first, daemon, home, config) || ownedSchedule(later, daemon, home, config) {
		t.Fatal("adopted task without an original receipt")
	}
	if err := os.WriteFile(filepath.Join(home, fallbackReceipt), []byte(first), 0600); err != nil {
		t.Fatal(err)
	}
	if !ownedSchedule(first, daemon, home, config) || ownedSchedule(later, daemon, home, config) {
		t.Fatal("original task export not enforced")
	}
}

func savedFallback(t *testing.T) (string, string) {
	t.Helper()
	home := t.TempDir()
	definition := taskXML(`C:\bin\obitd.exe`, home, schedulerUser(t), time.Now())
	if err := os.WriteFile(filepath.Join(home, fallbackReceipt), []byte(definition), 0600); err != nil {
		t.Fatal(err)
	}
	return home, definition
}

func TestScheduleRemovalPreviewUsesOwnership(t *testing.T) {
	old := schtasks
	t.Cleanup(func() { schtasks = old })
	t.Setenv("OBIT_CONFIG_DIR", `C:\config`)
	home, definition := savedFallback(t)
	for _, tc := range []struct {
		name, daemon, home, config, want string
		err                              error
	}{
		{"owned", `C:\bin\obitd.exe`, home, `C:\config`, "remove scheduled task", nil},
		{"foreign executable", `C:\other\obitd.exe`, home, `C:\config`, "preserve scheduled task", nil},
		{"foreign home", `C:\bin\obitd.exe`, `C:\isolated`, `C:\config`, "preserve scheduled task", nil},
		{"foreign config", `C:\bin\obitd.exe`, home, `C:\isolated`, "preserve scheduled task", nil},
		{"query denied", "", "", "", "query failed", schedulerExit(0x80070005)},
		{"absent", "", "", "", "unchanged scheduled task", schedulerExit(0x80070002)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schtasks = func(args ...string) (string, error) {
				if args[0] != "/Query" {
					t.Fatalf("preview changed task: %v", args)
				}
				return definition, tc.err
			}
			got := PreviewScheduleRemoval(tc.daemon, tc.home, tc.config)
			if !strings.Contains(got, tc.want) {
				t.Fatal(got)
			}
			if tc.err == nil && (!strings.Contains(got, `C:\bin\obitd.exe`) || !strings.Contains(got, home)) {
				t.Fatal(got)
			}
		})
	}
}

func TestRemoveScheduleChecksDefinitionAndDeletion(t *testing.T) {
	old := schtasks
	t.Cleanup(func() { schtasks = old })
	t.Setenv("OBIT_CONFIG_DIR", `C:\config`)
	home, definition := savedFallback(t)
	for _, scenario := range []string{"owned", "absent", "query denied", "changed before", "replaced after", "verify denied", "delete denied"} {
		t.Run(scenario, func(t *testing.T) {
			queries, deletes := 0, 0
			schtasks = func(args ...string) (string, error) {
				if args[0] == "/Delete" {
					deletes++
					if scenario == "delete denied" {
						return "denied", schedulerExit(0x80070005)
					}
					return "deleted", nil
				}
				queries++
				if len(args) != 5 || args[3] != "/XML" || args[4] != "/HRESULT" {
					t.Fatalf("definition not queried: %v", args)
				}
				if scenario == "absent" {
					return "localized missing", schedulerExit(0x80070002)
				}
				if scenario == "query denied" || scenario == "verify denied" && deletes > 0 {
					return "localized failure", schedulerExit(0x80070005)
				}
				if scenario == "changed before" && queries == 2 || scenario == "replaced after" && deletes > 0 {
					return "<Task><Actions><Exec><Command>foreign.exe</Command></Exec></Actions></Task>", nil
				}
				if deletes > 0 {
					return "missing", schedulerExit(0x80070002)
				}
				return definition, nil
			}
			err := removeScheduleFor(`C:\bin\obitd.exe`, home, `C:\config`)
			wantErr := scenario != "owned" && scenario != "absent"
			if scenario == "absent" || scenario == "query denied" || scenario == "changed before" {
				if deletes != 0 {
					t.Fatalf("unverified task deleted %d times", deletes)
				}
			} else if deletes != 1 {
				t.Fatalf("delete calls=%d", deletes)
			}
			if (err != nil) != wantErr {
				t.Fatalf("%s: %v", scenario, err)
			}
		})
	}
}
