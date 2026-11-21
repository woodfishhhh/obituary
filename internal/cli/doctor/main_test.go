package doctor

import (
	"github.com/woodfishhhh/obituary/internal/i18n"
	"os"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	initcmd "github.com/woodfishhhh/obituary/internal/cli/init"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestMain(m *testing.M) {
	contextmenu.Current = clitest.NewMenuRegistry()
	initcmd.ShortcutCodec = clitest.ShortcutCodec{}
	ContextMenuRegistry = func() contextmenu.Registry { return readOnlyMenuRegistry{} }
	regtxnProbe = func() error { return nil }
	// Run outside any project: doctor reports the project rules of the working
	// directory, and the repository's own .obitrules would add a row to every check.
	cwd, err := os.MkdirTemp("", "obit-doctor-cwd-")
	if err != nil {
		panic(err)
	}
	if err := os.Chdir(cwd); err != nil {
		panic(err)
	}
	code := testenv.Main(m)
	_ = os.Chdir(os.TempDir())
	_ = os.RemoveAll(cwd)
	os.Exit(code)
}

var english = diagnostics{i18n.NewLocalizer(i18n.English)}
