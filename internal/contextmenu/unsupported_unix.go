//go:build linux || darwin

package contextmenu

import (
	"fmt"

	"github.com/woodfishhhh/obituary/internal/core"
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
	Installed, Current bool
	Detail             string
}
type unsupportedRegistry struct{}

func (unsupportedRegistry) CompareAndSwap(Change) error { return unsupported() }
func (unsupportedRegistry) Lock() (func(), error)       { return nil, unsupported() }

func UserRegistry() Registry { return unsupportedRegistry{} }
func unsupported() error {
	return fmt.Errorf("the Explorer context menu and notifications are Windows-only: %w", core.ErrUnsupported)
}
func (unsupportedRegistry) ReadTree(string) (Tree, error) { return nil, unsupported() }
func (unsupportedRegistry) WriteTree(string, Tree) error  { return unsupported() }
func (unsupportedRegistry) DeleteTree(string, Tree) error { return unsupported() }
func Plan(Registry, string, bool) ([]Change, error)       { return nil, unsupported() }
func Apply(Registry, []Change) error                      { return unsupported() }
func Inspect(Registry, string) (Status, error)            { return Status{Detail: unsupported().Error()}, nil }
func PlanLanguage(Registry, string, bool, i18n.Language) ([]Change, error) {
	return nil, unsupported()
}
func InspectLanguage(Registry, string, i18n.Language) (Status, error) {
	return Status{Detail: unsupported().Error()}, nil
}
func Run([]string, func(string, []string) ([]byte, error), func(string, string) error) error {
	return unsupported()
}
func Execute(string, []string) ([]byte, error) { return nil, unsupported() }
func Notify(string, string) error              { return unsupported() }
func RunDropServer([]string, func(string, []string) ([]byte, error), func(string, string) error) error {
	return unsupported()
}
