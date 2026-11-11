package ledger

import (
	"github.com/woodfishhhh/obituary/internal/docker"
)

const KindContainer = "container"

func validateContainerState(st State) error {
	for _, it := range st.Items {
		if err := validateObjectKind(it.Kind, it.Container, it.Identity); err != nil {
			return err
		}
	}
	for _, b := range st.Burials {
		if err := validateObjectKind(b.Kind, b.Container, b.Identity); err != nil {
			return err
		}
	}
	for _, p := range st.Proposals {
		for _, c := range p.Candidates {
			if err := validateObjectKind(c.Kind, c.Container, nil); err != nil {
				return err
			}
		}
	}
	return validateContainerIntents(st)
}

func validateObjectKind(kind string, container *docker.Identity, file *Identity) error {
	if kind == "" && container == nil {
		return nil
	}
	if kind != KindContainer || container == nil || !container.Valid() || file != nil {
		return docker.ErrIdentity
	}
	return nil
}

func ContainerIdentity(kind string, id *docker.Identity) error {
	if kind != KindContainer || id == nil || !id.Valid() {
		return docker.ErrIdentity
	}
	return nil
}
