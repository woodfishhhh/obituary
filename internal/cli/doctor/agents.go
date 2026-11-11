package doctor

import (
	initcmd "github.com/woodfishhhh/obituary/internal/cli/init"
	"github.com/woodfishhhh/obituary/internal/integrate"
)

func diagnoseIntegration(agent string, inspector *initcmd.AgentInspector) DoctorCheck {
	state, err := inspector.Inspect(agent)
	c := DoctorCheck{Name: agent, Status: "PASS", Detail: state.Detail, Installation: state.Installation}
	if err != nil {
		c.Status, c.Detail = "FAIL", err.Error()
		return c
	}
	detail, warning, err := integrate.AgentWindowsShellStatus(agent)
	if err != nil {
		c.Status, c.Detail = "FAIL", err.Error()
		return c
	}
	if detail != "" {
		c.Detail += "; " + detail
	}
	if warning {
		c.Status = "WARN"
	}
	switch state.Installation {
	case "missing", "outdated", "manual":
		c.Status = "WARN"
	case "unavailable":
		c.Status = "INFO"
	}
	return c
}
