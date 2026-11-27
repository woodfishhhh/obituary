//go:build linux || darwin

package explorer

import (
	"fmt"
	"time"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

const iniName = "desktop.ini"

type Plan struct{ Changed bool }

func Prepare(string, authorization, map[string]time.Time, i18n.Language) (*Plan, error) {
	return nil, fmt.Errorf("the Explorer label feature is Windows-only: %w", corefs.ErrUnsupported)
}

func (projection) add(*lifecycle.Service, string, ledger.Item, Options, []string) error {
	return fmt.Errorf("the Explorer label feature is Windows-only: %w", corefs.ErrUnsupported)
}
func (*Plan) Apply() error { return corefs.ErrUnsupported }
