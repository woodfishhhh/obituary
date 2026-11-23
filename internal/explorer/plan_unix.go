//go:build linux || darwin

package explorer

import (
	"fmt"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

const iniName = "desktop.ini"

type Plan struct{ Changed bool }

func Prepare(string, authorization, map[string]time.Time, i18n.Language) (*Plan, error) {
	return nil, fmt.Errorf("the Explorer label feature is Windows-only: %w", core.ErrUnsupported)
}

func (projection) add(*core.Service, string, core.Item, Options, []string) error {
	return fmt.Errorf("the Explorer label feature is Windows-only: %w", core.ErrUnsupported)
}
func (*Plan) Apply() error { return core.ErrUnsupported }
