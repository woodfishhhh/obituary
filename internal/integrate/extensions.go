package integrate

import (
	"errors"
	"fmt"
	"strings"
)

// ExtensionStatus distinguishes ownership from currentness without writes.
type ExtensionStatus struct {
	Installed bool   `json:"installed"`
	Current   bool   `json:"current"`
	Detail    string `json:"detail"`
	Manual    bool   `json:"manual,omitempty"`
}

func InspectExtension(s Scope, agent, kind, executable string) (ExtensionStatus, error) {
	s = s.ForInspection()
	installer, ok := LookupAgent(agent)
	if !ok {
		return ExtensionStatus{}, fmt.Errorf("unknown agent %q", agent)
	}
	plan := func(uninstall bool) ([]Change, error) {
		if kind == "skill" {
			if installer.PlanSkill == nil {
				return nil, fmt.Errorf("%s skill is unsupported: %s", agent, installer.Capabilities.Skill.Detail)
			}
			return installer.PlanSkill(s, uninstall)
		}
		if installer.PlanMCP == nil {
			return nil, fmt.Errorf("%s MCP is unsupported: %s", agent, installer.Capabilities.MCP.Detail)
		}
		return installer.PlanMCP(s, executable, uninstall)
	}
	remove, err := plan(true)
	var manual *ManualSteps
	if errors.As(err, &manual) || hasManualChanges(remove) {
		install, installErr := plan(false)
		if errors.As(installErr, &manual) {
			return ExtensionStatus{Manual: true, Detail: "需要手动步骤 (manual steps required): " + manual.Steps}, nil
		}
		if installErr != nil {
			return ExtensionStatus{}, installErr
		}
		var steps []string
		for _, c := range install {
			if c.Manual {
				steps = append(steps, c.Detail)
			}
		}
		return ExtensionStatus{Manual: true, Detail: "需要手动步骤 (manual steps required): " + strings.Join(steps, "\n")}, nil
	}
	if err != nil {
		return ExtensionStatus{}, err
	}
	status := ExtensionStatus{Installed: Pending(remove), Detail: "not installed"}
	if kind == "skill" && (agent == "claude" || agent == "codex") {
		status.Installed = remove[0].delete // an orphaned receipt is not an installed skill
	}
	if !status.Installed {
		for _, c := range remove {
			if strings.Contains(c.Detail, "preserved") {
				return status, fmt.Errorf("%s: %s", c.Path, c.Detail)
			}
		}
		return status, nil
	}
	install, err := plan(false)
	if err != nil {
		return status, err
	}
	status.Current = status.Installed && !Pending(install)
	if status.Current {
		status.Detail = "installed, current"
	} else if status.Installed {
		status.Detail = "installed, outdated"
	}
	return status, nil
}

func hasManualChanges(changes []Change) bool {
	for _, c := range changes {
		if c.Manual {
			return true
		}
	}
	return false
}
