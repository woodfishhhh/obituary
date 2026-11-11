package doctor

import (
	"fmt"
	"time"

	"github.com/woodfishhhh/obituary/internal/buildinfo"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/update"
)

func diagnoseUpdate(home string) DoctorCheck {
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
	checked := "never"
	if !state.CheckedAt.IsZero() {
		checked = state.CheckedAt.Format(time.RFC3339)
	}
	out.Detail = fmt.Sprintf("check=%t; last checked=%s; newer=%t", cfg.Update.Check, checked, update.Newer(buildinfo.Version, state.Latest.Version))
	if state.Latest.Version != "" {
		out.Detail += "; latest=" + state.Latest.Version
	}
	if state.LastError != "" {
		out.Detail += "; lastError=" + state.LastError
	}
	return out
}
