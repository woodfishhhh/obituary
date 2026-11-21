package doctor

import (
	"fmt"
	"time"

	"github.com/woodfishhhh/obituary/internal/buildinfo"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/update"
)

func (lang diagnostics) diagnoseUpdate(home string) DoctorCheck {
	out := DoctorCheck{Name: "update", Status: "INFO"}
	cfg, err := config.Load()
	if err != nil {
		out.Detail = err.Error()
		return out
	}
	state, err := update.Read(home)
	if err != nil {
		state.LastError = err.Error()
	}
	checked := lang.Text("cli.doctor.never", nil)
	if !state.CheckedAt.IsZero() {
		checked = state.CheckedAt.Format(time.RFC3339)
	}
	out.Detail = fmt.Sprintf(lang.Text("cli.doctor.check_last_checked_newer", nil), cfg.Update.Check, checked, update.Newer(buildinfo.Version, state.Latest.Version))
	if state.Latest.Version != "" {
		out.Detail += lang.Text("cli.doctor.latest", nil) + state.Latest.Version
	}
	if state.LastError != "" {
		out.Detail += lang.Text("cli.doctor.lasterror", nil) + state.LastError
	}
	return out
}
