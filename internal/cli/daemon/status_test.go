package daemon

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func TestStatusReportsScheduleForAnotherHome(t *testing.T) {
	clitest.IsolateRunner(t)
	cmdutil.RunnerInstalled = func() bool { return true }
	cmdutil.RunnerTarget = func() (string, error) { return `D:\obitd.exe --home "D:\other"`, nil }
	_, store := clitest.CommandDir(t)
	out, _, err := clitest.Execute(t, StatusCmd(userdirs.Data), "--json")
	var status DaemonStatus
	if err != nil || json.Unmarshal([]byte(out), &status) != nil || status.Mode != "off" || !status.ScheduleInstalled || status.ScheduleServesHome || status.ScheduleTarget == "" || status.Home != store.Home {
		t.Fatalf("status = %s: %v", out, err)
	}
	out, _, err = clitest.Execute(t, StatusCmd(userdirs.Data))
	if err != nil || !strings.Contains(out, "does not serve this data directory") {
		t.Fatalf("%s: %v", out, err)
	}
}
