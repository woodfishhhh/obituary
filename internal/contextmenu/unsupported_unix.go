//go:build linux || darwin

package contextmenu

import (
	"fmt"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

// AppID matches the Windows registration so shared previews compile.
const AppID = "Obituary.ContextMenu"

type Tree map[string]map[string]string
type Registry interface {
	ReadTree(string) (Tree, error)
	CompareAndSwap(Change) error
	Lock() (func(), error)
}
type Change struct {
	Path          string
	Before, After Tree
}
type Status struct {
	Installed, Current    bool
	LabelLanguageMismatch bool
	Detail                string
}

func unsupported() error {
	return fmt.Errorf("the Explorer context menu and notifications are Windows-only: %w", corefs.ErrUnsupported)
}

func InspectDisplay(reg Registry, exe string, language i18n.Language, _ *i18n.Localizer) (Status, error) {
	return InspectLanguage(reg, exe, language)
}
func Run([]string, func(string, []string) ([]byte, error), func(string, string) error) error {
	return unsupported()
}
func Execute(string, []string) ([]byte, error) { return nil, unsupported() }
func Notify(string, string) error              { return unsupported() }
func RunDropServer([]string, func(string, []string) ([]byte, error), func(string, string) error) error {
	return unsupported()
}
