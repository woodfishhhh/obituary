package init

import (
	"fmt"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"strings"
)

func (p Planner) printSkillTarget(scope integrate.Scope, agent string) {
	installer, _ := integrate.LookupAgent(agent)
	if installer.PlanSkill == nil {
		fmt.Printf(cmdutil.Text("cli.init.no_skill_directory"), agent)
		return
	}
	changes, err := installer.PlanSkill(scope.ForInspection(), true)
	if err != nil {
		fmt.Printf(cmdutil.Text("cli.init.agent_skill"), agent, err)
		return
	}
	for _, change := range changes {
		if strings.HasSuffix(change.Path, "SKILL.md") {
			fmt.Printf("%s Agent Skill → %s\n", agent, change.Path)
			return
		}
	}
	fmt.Printf(cmdutil.Text("cli.init.no_skill_directory"), agent)
}
