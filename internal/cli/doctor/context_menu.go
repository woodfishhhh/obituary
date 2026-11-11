package doctor

import (
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
)

var ContextMenuRegistry = contextmenu.UserRegistry

func diagnoseContextMenu(executable cmdutil.Executable) DoctorCheck {
	c := DoctorCheck{Name: "context-menu", Status: "PASS"}
	exe, err := executable.Path()
	if err != nil {
		c.Status, c.Detail = "FAIL", err.Error()
		return c
	}
	st, err := contextmenu.Inspect(ContextMenuRegistry(), exe)
	if err != nil {
		c.Status, c.Detail = "FAIL", err.Error()
		return c
	}
	c.Detail = st.Detail
	if !st.Installed {
		c.Status = "INFO"
	} else if !st.Current {
		c.Status = "WARN"
	}
	return c
}
