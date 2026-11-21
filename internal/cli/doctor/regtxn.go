package doctor

import (
	"runtime"

	"github.com/woodfishhhh/obituary/internal/regtxn"
)

var regtxnProbe = regtxn.Probe

type KTMCheck struct {
	Available bool `json:"available"`
}

func (lang diagnostics) diagnoseRegistryTransactions(probe func() error) DoctorCheck {
	if runtime.GOOS != "windows" {
		return DoctorCheck{Name: "registry-transactions", Status: "INFO", Detail: lang.Text("cli.doctor.registry_transactions_are_windows_only", nil)}
	}
	c := DoctorCheck{Name: "registry-transactions", Status: "PASS", Detail: lang.Text("cli.doctor.ktm_is_available_empty_transaction_rolled_back", nil), KTM: &KTMCheck{Available: true}}
	if err := probe(); err != nil {
		c.Status = "WARN"
		c.KTM.Available = false
		c.Detail = lang.Text("cli.doctor.ktm_unavailable", nil) + err.Error() + lang.Text("cli.doctor.context_menu_and_login_startup_changes_fail", nil)
	}
	return c
}
