package i18n

// These persisted CLI contracts predate localized diagnostics. Human output
// follows the catalog; changing these strings would change doctor --json.
var diagnosticJSON = map[string]string{
	"cli.doctor.additional_agents":     "另有 %d 家 agent 可接入：obit init --show 查看",
	"cli.init.installer_not_available": "安装尚未提供 (installer not available)",
	"cli.init.scope_unavailable":       "安装尚未提供 (no automatic configuration target for this scope)",
	"cli.init.integration_current":     "已安装 (integration is current)",
	"cli.init.integration_missing":     "未安装 (integration is missing)",
	"cli.init.integration_outdated":    "需要刷新 (integration is outdated or incomplete)",
	"cli.init.manual_missing":          "未安装；接入需要手动步骤 (not installed; manual setup required): ",
	"cli.init.manual_steps":            "需要手动步骤 (manual steps required): ",
}

func NewProtocolLocalizer() *Localizer { l := NewLocalizer(English); l.protocol = true; return l }
