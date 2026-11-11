// Package gui exposes the same service operations as the CLI, without a window
// framework dependency. The Wails shell binds only API.
package gui

import (
	"fmt"
	"sync"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/runkey"
	"github.com/woodfishhhh/obituary/internal/shortcut"
	"github.com/woodfishhhh/obituary/internal/update"
)

type Dependencies struct {
	Service        *core.Service
	Engine         *auto.Engine
	ConfigDir      string
	Executable     string
	Daemon         string
	ContextMenu    contextmenu.Registry
	RunKey         runkey.Registry
	Now            func() time.Time
	SystemLanguage func() string
	UpdateChecker  *update.Checker
	Shortcuts      shortcut.Codec
}

type API struct {
	deps Dependencies
	// Wails calls methods concurrently. Serialize access to the service's
	// mutable configuration and to the one confirmation currently displayed.
	mu       sync.Mutex
	nextPlan uint64
	plan     *pendingPlan
	// pickFolder opens the window's folder dialog (SetFolderPicker).
	pickFolder func() (string, error)
	setTaskbar func(bool) error
}

func New(deps Dependencies) (*API, error) {
	if deps.Service == nil || deps.Engine == nil || deps.Engine.Svc != deps.Service {
		return nil, fmt.Errorf("GUI requires a service and its engine")
	}
	if deps.ConfigDir == "" || deps.Executable == "" || deps.Daemon == "" || deps.ContextMenu == nil || deps.RunKey == nil {
		return nil, fmt.Errorf("GUI configuration directory, executables and registries are required")
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.SystemLanguage == nil {
		deps.SystemLanguage = i18n.SystemLanguage
	}
	if deps.Shortcuts == nil {
		deps.Shortcuts = shortcut.Native{}
	}
	return &API{deps: deps}, nil
}

type Receipt struct {
	Action  string `json:"action"`
	Path    string `json:"path,omitempty"`
	Changed bool   `json:"changed"`
}

// Batch errors live alongside receipts: Wails discards the value when a Go
// error rejects a binding promise. Never lose a successful move's actual path
// just because writing its audit log (or another member of the batch) failed.
type GraveReply struct {
	Results []core.GraveResult `json:"results"`
	Error   string             `json:"error,omitempty"`
}

type ItemReply struct {
	Results []core.Result `json:"results"`
	Error   string        `json:"error,omitempty"`
}

// list keeps an empty answer an empty JSON array. A nil slice marshals as
// null, and the window reads every list field as an array.
func list[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func errorText(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

func requireRefs(refs []string) error {
	if len(refs) == 0 {
		return fmt.Errorf("select at least one item")
	}
	return nil
}
