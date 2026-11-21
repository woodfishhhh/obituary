package doctor

import (
	"strings"

	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/integrate"
)

func (lang diagnostics) diagnoseExtension(kind string, current cmdutil.Executable) DoctorCheck {
	c := DoctorCheck{Name: kind, Status: "PASS"}
	executable, err := current.Path()
	if err != nil {
		c.Status, c.Detail = "FAIL", err.Error()
		return c
	}
	var details []string
	for _, agent := range []string{"claude", "codex"} {
		st, err := integrate.InspectExtension(integrate.Scope{Global: true}, agent, kind, executable)
		if err != nil {
			c.Status = "FAIL"
			details = append(details, agent+": "+err.Error())
			continue
		}
		if st.Installed && !st.Current && c.Status != "FAIL" {
			c.Status = "WARN"
		}
		if !st.Installed {
			st.Detail += lang.Text("cli.doctor.install_with_obit_init_g_agent", nil) + agent
			if kind == "mcp" {
				st.Detail += " --mcp"
			}
		}
		details = append(details, agent+lang.Text("cli.doctor.global", nil)+st.Detail)
	}
	c.Detail = strings.Join(details, "; ")
	return c
}
