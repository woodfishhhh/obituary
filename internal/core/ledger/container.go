package ledger

import (
	"github.com/woodfishhhh/obituary/internal/docker"
)

const (
	KindContainer    = "container"
	KindDockerVolume = "docker-volume"
)

func validateContainerState(st State) error {
	for _, it := range st.Items {
		if err := validateObjectKind(it.Kind, it.Container, it.Volume, it.Identity); err != nil {
			return err
		}
	}
	for _, b := range st.Burials {
		if err := validateObjectKind(b.Kind, b.Container, b.Volume, b.Identity); err != nil {
			return err
		}
	}
	for _, p := range st.Proposals {
		for _, c := range p.Candidates {
			if err := validateObjectKind(c.Kind, c.Container, c.Volume, nil); err != nil {
				return err
			}
		}
	}
	return validateContainerIntents(st)
}

func validateObjectKind(kind string, container *docker.Identity, volume *docker.Volume, file *Identity) error {
	if kind == "" && container == nil && volume == nil {
		return nil
	}
	if kind == KindContainer && container != nil && container.Valid() && volume == nil && file == nil {
		return nil
	}
	if kind == KindDockerVolume && volume != nil && volume.Check() == nil && container == nil && file == nil {
		return nil
	}
	return docker.ErrIdentity
}

func ContainerIdentity(kind string, id *docker.Identity) error {
	if kind != KindContainer || id == nil || !id.Valid() {
		return docker.ErrIdentity
	}
	return nil
}
