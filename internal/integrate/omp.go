package integrate

import "github.com/woodfishhhh/obituary/internal/hook"

// PlanOMP targets the default OMP profile or its explicit directory override.
func PlanOMP(s Scope, commands Commands, uninstall bool) ([]Change, error) {
	return planPiHost(s, commands, "omp", uninstall)
}

func PlanOMPMCP(s Scope, executable string, uninstall bool) ([]Change, error) {
	return planPiMCP(s, executable, "omp", uninstall)
}

func init() {
	host, _ := hook.Lookup("omp")
	caps := host.Capabilities
	caps.DeleteGuard.Detail = "tool_call block/reason; requires OBIT_OMP_LOCAL=1 and OBIT_OMP_SHELL=bash or powershell after verifying the backend"
	caps.WriteObservation.Detail = "Successful local write/edit tool_result; requires OBIT_OMP_LOCAL=1; passive additionalContext; apply_patch is skipped"
	caps.SessionStart.Detail = "session_start uses stable sessionManager id and passive sendMessage; local backend attestation required"
	caps.Stop.Detail = "agent_end and main-session session_stop map to silent Stop"
	caps.SessionEnd.Detail = "session_shutdown maps to silent SessionEnd with a 1.2-second subprocess budget"
	caps.Context.Detail = "tool_result.additionalContext and session_start sendMessage with triggerTurn:false; actual host loader pending"
	caps.MCP = hook.Support{Level: hook.Supported, Detail: "Native user/project mcp.json; default profile with shared Pi/OMP ownership"}
	RegisterAgent(Agent{Name: "omp", Method: Plugin, Capabilities: caps, Plan: PlanOMP, PlanMCP: PlanOMPMCP, PlanSkill: unsupportedPluginSkill("omp")})
}
