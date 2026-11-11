package doctor

import (
	"runtime"

	"github.com/woodfishhhh/obituary/internal/regtxn"
)

var regtxnProbe = regtxn.Probe

type KTMCheck struct {
	Available bool `json:"available"`
}

func diagnoseRegistryTransactions(probe func() error) DoctorCheck {
	if runtime.GOOS != "windows" {
		return DoctorCheck{Name: "registry-transactions", Status: "INFO", Detail: "registry transactions are Windows-only"}
	}
	c := DoctorCheck{Name: "registry-transactions", Status: "PASS", Detail: "KTM is available (empty transaction rolled back; no registry keys touched); no action needed", KTM: &KTMCheck{Available: true}}
	if err := probe(); err != nil {
		c.Status = "WARN"
		c.KTM.Available = false
		c.Detail = "KTM unavailable: " + err.Error() + "; context-menu and login-startup changes fail closed; check Windows KTM availability before retrying those changes"
	}
	return c
}
