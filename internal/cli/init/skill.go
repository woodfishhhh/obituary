package init

import (
	"fmt"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"strings"
)

func (p Planner) printSkillTarget(scope integrate.Scope, agent string) {
	installer, _ := integrate.LookupAgent(agent)
	if installer.PlanSkill == nil {
		fmt.Printf("%s Agent Skill：跳过：这个 agent 没有 skill 目录\n", agent)
		return
	}
	changes, err := installer.PlanSkill(scope.ForInspection(), true)
	if err != nil {
		fmt.Printf("%s Agent Skill: %v\n", agent, err)
		return
	}
	for _, change := range changes {
		if strings.HasSuffix(change.Path, "SKILL.md") {
			fmt.Printf("%s Agent Skill → %s\n", agent, change.Path)
			return
		}
	}
	fmt.Printf("%s Agent Skill：跳过：这个 agent 没有 skill 目录\n", agent)
}
