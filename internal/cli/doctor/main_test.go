package doctor

import (
	"os"
	"testing"

	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestMain(m *testing.M) {
	ContextMenuRegistry = func() contextmenu.Registry { return readOnlyMenuRegistry{} }
	regtxnProbe = func() error { return nil }
	os.Exit(testenv.Main(m))
}
