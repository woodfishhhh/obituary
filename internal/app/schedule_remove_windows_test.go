package app

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

type schedulerExit uint32

func (e schedulerExit) Error() string { return fmt.Sprintf("HRESULT %#x", uint32(e)) }
func (e schedulerExit) ExitCode() int { return int(e) }

func TestScheduleRemovalPreviewUsesOwnership(t *testing.T) {
	old := schtasks
	t.Cleanup(func() { schtasks = old })
	t.Setenv("OBIT_CONFIG_DIR", `C:\config`)
	definition := taskXML(`C:\bin\obitd.exe`, `C:\home`, "test", time.Now())
	for _, tc := range []struct {
		name, daemon, home, config, want string
		err                              error
	}{
		{"owned", `C:\bin\obitd.exe`, `C:\home`, `C:\config`, "remove scheduled task", nil},
		{"foreign executable", `C:\other\obitd.exe`, `C:\home`, `C:\config`, "preserve scheduled task", nil},
		{"foreign home", `C:\bin\obitd.exe`, `C:\isolated`, `C:\config`, "preserve scheduled task", nil},
		{"foreign config", `C:\bin\obitd.exe`, `C:\home`, `C:\isolated`, "preserve scheduled task", nil},
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
			if tc.err == nil && (!strings.Contains(got, `C:\bin\obitd.exe`) || !strings.Contains(got, `C:\home`)) {
				t.Fatal(got)
			}
		})
	}
}

func TestRemoveScheduleChecksDefinitionAndDeletion(t *testing.T) {
	old := schtasks
	t.Cleanup(func() { schtasks = old })
	t.Setenv("OBIT_CONFIG_DIR", `C:\config`)
	definition := taskXML(`C:\bin\obitd.exe`, `C:\home`, "test", time.Now())
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
			err := removeScheduleFor(`C:\bin\obitd.exe`, `C:\home`, `C:\config`)
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
