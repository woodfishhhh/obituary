package doctor

import (
	"context"
	"fmt"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/docker"
)

func (lang diagnostics) diagnoseDockerTargets(st ledger.State) DoctorCheck {
	count := 0
	check := func(id *docker.Identity) {
		if id != nil && id.Target == nil {
			count++
		}
	}
	for _, it := range st.Items {
		check(it.Container)
	}
	for _, b := range st.Burials {
		check(b.Container)
	}
	for _, p := range st.Proposals {
		for _, c := range p.Candidates {
			check(c.Container)
		}
	}
	for _, p := range st.ContainerIntents {
		if p.Item != nil {
			check(p.Item.Container)
		}
		if p.Burial != nil {
			check(p.Burial.Container)
		}
	}
	status, detail := "PASS", lang.Text("cli.doctor.container_docker_endpoints_and_daemon_identities_are", nil)
	if count > 0 {
		status = "WARN"
		detail = fmt.Sprintf(lang.Text("cli.doctor.legacy_v14_container_references_have_no_docker", nil), count)
	}
	return DoctorCheck{Name: "docker-targets", Status: status, Detail: detail}
}

func (lang diagnostics) diagnoseDocker() DoctorCheck {
	err := (docker.CLI{}).Available(context.Background())
	detail := lang.Text("cli.doctor.containers_stop_at_expiry_and_retain_their", nil)
	status := "PASS"
	if err != nil {
		status, detail = "INFO", lang.Text("cli.doctor.docker_unavailable_container_discovery_skipped", nil)+docker.Diagnostic(err)
	}
	return DoctorCheck{Name: "docker", Status: status, Detail: detail}
}
