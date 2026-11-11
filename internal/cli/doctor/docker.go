package doctor

import (
	"context"
	"fmt"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/docker"
)

func diagnoseDockerTargets(st core.State) DoctorCheck {
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
	status, detail := "PASS", "container Docker endpoints and daemon identities are pinned"
	if count > 0 {
		status = "WARN"
		detail = fmt.Sprintf("%d legacy v14 container references have no Docker target; select their original daemon before first operation to bind matching identities; absent containers remain pending", count)
	}
	return DoctorCheck{Name: "docker-targets", Status: status, Detail: detail}
}

func diagnoseDocker() DoctorCheck {
	err := (docker.CLI{}).Available(context.Background())
	detail := "containers stop at expiry and retain their writable layer; after graveyard retention, stopped containers may be removed; recycle mode uses the same policy; images, volumes and networks are preserved"
	status := "PASS"
	if err != nil {
		status, detail = "INFO", "Docker unavailable; container discovery skipped: "+docker.Diagnostic(err)
	}
	return DoctorCheck{Name: "docker", Status: status, Detail: detail}
}
