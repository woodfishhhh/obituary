package doctor

import (
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
	os.Exit(testenv.Main(m))
}
