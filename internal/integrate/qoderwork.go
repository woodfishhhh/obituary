package integrate

import (
	"fmt"

	"github.com/woodfishhhh/obituary/internal/hook"
)

// QoderWork documents global hooks with exit-2 denial, not a verified exit-0
// JSON deny contract. #35 must supply the managed wrapper before hooks can be
// installed. Rules/MCP paths are also unverified: return reviewable manual
// content, never install into a guessed .qoderwork configuration member.
func PlanQoderWork(s Scope, _ Commands, uninstall bool) ([]Change, error) {
	if s.GuardRM && !uninstall {
		return nil, fmt.Errorf("QoderWork delete guard is limited: requires #35's managed exit-code wrapper (validated deny to exit 2 + stderr, errors to exit 0); no hooks installed")
	}
	detail := "QoderWork persistent rule location is unverified. Paste these instructions into a local task prompt; this is manual context, not automatic registration:\n" + Awareness
	if uninstall {
		detail = "No QoderWork hooks or persistent rules were installed. Remove only the Obituary text previously pasted into a task or a rule you created manually"
	}
	return []Change{{Path: "QoderWork task instructions (manual)", Action: "preserve", Detail: detail}}, nil
}

func PlanQoderWorkMCP(_ Scope, executable string, uninstall bool) ([]Change, error) {
	if uninstall {
		return nil, manualf("QoderWork MCP target is unverified: open the MCP configuration through this installed version's MCP/connector UI; remove only obituary after verifying command obit.exe and args [\"mcp\"]; no automatic change was made")
	}
	entry, err := (CommandArgsMCP{}).Encode(executable, nil)
	if err != nil {
		return nil, err
	}
	return nil, manualf("QoderWork MCP target/schema is unverified: open its MCP/connector UI, choose a custom local stdio server named obituary, command %s, args [\"mcp\"]; if the UI accepts mcpServers JSON, use obituary = %s after checking for a name collision; no file was written", executable, entry)
}

func init() {
	RegisterAgent(Agent{Name: "qoderwork", Method: RulesAndMCP, Plan: PlanQoderWork, PlanMCP: PlanQoderWorkMCP, Capabilities: hook.Capabilities{
		DeleteGuard:      hook.Support{Level: hook.Limited, Detail: "Global hooks only; exit-2 denial needs the #35 wrapper; no native hook installed"},
		WriteObservation: hook.Support{Detail: "Awaiting verified tool fields and wrapper"},
		SessionStart:     hook.Support{Detail: "Documented event, not installed"},
		SessionEnd:       hook.Support{Detail: "Documented event, not installed"},
		Stop:             hook.Support{Detail: "Documented event, not installed"},
		Context:          hook.Support{Level: hook.Limited, Detail: "Manual task instructions only; persistent rule metadata/path unverified"},
		MCP:              hook.Support{Level: hook.Limited, Detail: "Manual stdio configuration; physical target and this host version's schema require UI verification"},
		Skill:            hook.Support{Detail: "No verified skill installation target"},
	}})
}
