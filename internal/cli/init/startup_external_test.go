package init_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	initcmd "github.com/woodfishhhh/obituary/internal/cli/init"
	"github.com/woodfishhhh/obituary/internal/runkey"
)

func TestInitPlansInstallsAndRemovesTrayStartup(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	clitest.CommandRepo(t)
	root, executable := clitest.DoctorFixture(t)
	plan, err := app.PlanAutostart(runkey.Current, filepath.Join(root, "bin", "obitd.exe"), os.Getenv("OBIT_HOME"), os.Getenv("OBIT_CONFIG_DIR"), true)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if got, _ := runkey.Current.Get(plan.Name); got != "" {
		t.Fatal("dry-run wrote Run value")
	}
	if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--yes"); err != nil {
		t.Fatal(err)
	}
	if got, _ := runkey.Current.Get(plan.Name); got != plan.After {
		t.Fatalf("startup missing: %q", got)
	}
	if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--yes", "--uninstall"); err != nil {
		t.Fatal(err)
	}
	if got, _ := runkey.Current.Get(plan.Name); got != "" {
		t.Fatal("uninstall left owned Run value")
	}
}
