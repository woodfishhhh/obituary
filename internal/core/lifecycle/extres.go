package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/extres"
	"github.com/woodfishhhh/obituary/internal/docker"
)

type containerIDsLister interface {
	ListIDs(context.Context, extres.Credential) ([]string, error)
}

type volumeStreamRestorer interface {
	EnsureAbsent(context.Context, extres.Credential) error
	ResurrectFrom(context.Context, extres.Credential, io.ReadSeeker, string) error
}

func (s *Service) provider(kind string) (extres.Provider, bool) {
	if p, ok := s.Providers[kind]; ok && p != nil {
		if adapted, ok := s.adaptOverriddenDockerProvider(kind, p); ok {
			return adapted, true
		}
		return p, true
	}
	switch kind {
	case KindContainer:
		if s.Docker != nil {
			return docker.NewContainerProvider(s.Docker), true
		}
	case KindDockerVolume:
		if vc, ok := s.Docker.(docker.VolumeClient); ok && vc != nil {
			return docker.NewVolumeProvider(vc), true
		}
	}
	return nil, false
}

// explicitProvider returns a provider only when Service.Providers explicitly
// registered kind. Callers that only set Service.Docker fall through to legacy
// Docker call sites so existing mutation anchors remain live.
func (s *Service) explicitProvider(kind string) (extres.Provider, bool) {
	if s.Providers[kind] == nil {
		return nil, false
	}
	return s.provider(kind)
}

func (s *Service) adaptOverriddenDockerProvider(kind string, p extres.Provider) (extres.Provider, bool) {
	if s.Docker == nil {
		return nil, false
	}
	if kind == KindContainer {
		return s.adaptContainerProvider(p)
	}
	if kind == KindDockerVolume {
		return s.adaptVolumeProvider(p)
	}
	return nil, false
}

func (s *Service) adaptContainerProvider(p extres.Provider) (extres.Provider, bool) {
	cp, ok := p.(*docker.ContainerProvider)
	if !ok {
		return nil, false
	}
	cli, ok := cp.Client().(docker.CLI)
	if !ok {
		return nil, false
	}
	if cur, ok := s.Docker.(docker.CLI); ok && cur == cli {
		return nil, false
	}
	return docker.NewContainerProvider(s.Docker), true
}

func (s *Service) adaptVolumeProvider(p extres.Provider) (extres.Provider, bool) {
	vp, ok := p.(*docker.VolumeProvider)
	if !ok {
		return nil, false
	}
	cli, ok := vp.Client().(docker.CLI)
	if !ok {
		return nil, false
	}
	if cur, ok := s.Docker.(docker.CLI); ok && cur == cli {
		return nil, false
	}
	vc, ok := s.Docker.(docker.VolumeClient)
	if !ok || vc == nil {
		return nil, false
	}
	return docker.NewVolumeProvider(vc), true
}

func (s *Service) retentionPolicy(kind string) (extres.RetentionPolicy, bool) {
	p, ok := s.provider(kind)
	if !ok {
		return nil, false
	}
	rp, ok := p.(extres.RetentionPolicy)
	return rp, ok
}

func (s *Service) requireElapsedRetention(kind string) bool {
	if rp, ok := s.retentionPolicy(kind); ok {
		return rp.RequireElapsedRetention()
	}
	return kind == KindContainer || kind == KindDockerVolume
}

func (s *Service) excludeFromRecycleMode(kind string) bool {
	if rp, ok := s.retentionPolicy(kind); ok {
		return rp.ExcludeFromRecycleMode()
	}
	return kind != ""
}

func (s *Service) validateExternalItem(it Item) error {
	if prov, ok := s.provider(it.Kind); ok {
		if cred, ok := itemCredential(it); ok {
			return prov.Validate(cred)
		}
	}
	if it.Kind == KindContainer {
		return containerIdentity(it.Kind, it.Container)
	}
	if it.Kind == KindDockerVolume {
		if it.Volume == nil {
			return docker.ErrIdentity
		}
		return it.Volume.Check()
	}
	return ErrUnsafe
}

func itemCredential(it Item) (extres.Credential, bool) {
	if it.Kind == KindContainer && it.Container != nil {
		return docker.EncodeContainerCredential(*it.Container), true
	}
	if it.Kind == KindDockerVolume && it.Volume != nil {
		return docker.EncodeVolumeCredential(*it.Volume), true
	}
	return extres.Credential{}, false
}

func (s *Service) verifyContainerProvider(p ContainerIntent) (extres.Observation, docker.Container, error) {
	prov, ok := s.provider(KindContainer)
	if !ok {
		return extres.Observation{Verdict: extres.VerdictUnavailable}, docker.Container{}, docker.ErrUnavailable
	}
	id := containerIntentIdentity(p)
	if id == nil {
		return extres.Observation{Verdict: extres.VerdictIdentity}, docker.Container{}, docker.ErrIdentity
	}
	obs, err := prov.Verify(s.Store.Context(), docker.EncodeContainerCredential(*id))
	if errors.Is(err, docker.ErrAutoRemove) {
		got := containerFromObservation(*id, obs)
		got.AutoRemove = true
		return obs, got, nil
	}
	switch obs.Verdict {
	case extres.VerdictNotFound:
		return obs, docker.Container{}, docker.ErrNotFound
	case extres.VerdictIdentity:
		if err == nil {
			err = docker.ErrIdentity
		}
		return obs, docker.Container{}, err
	case extres.VerdictUnavailable:
		if err == nil {
			err = docker.ErrUnavailable
		}
		return obs, docker.Container{}, err
	}
	if err != nil {
		return obs, docker.Container{}, err
	}
	return obs, containerFromObservation(*id, obs), nil
}

func containerFromObservation(fallback docker.Identity, obs extres.Observation) docker.Container {
	id := fallback
	if obs.Updated != nil {
		if decoded, err := docker.DecodeContainerCredential(*obs.Updated); err == nil {
			id = decoded
		}
	}
	return docker.Container{Identity: id, Running: obs.Active}
}

func (s *Service) sendContainerStopViaProvider(prov extres.Provider, p *ContainerIntent, beforeSend func(docker.Container) error) (error, error) {
	var writeErr error
	hooks := extres.MutateHooks{
		BeforeSend: func(_ string, raw json.RawMessage) error {
			var ev docker.StopEvidence
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &ev); err != nil {
					return err
				}
			}
			id := *containerIntentIdentity(*p)
			if ev.StartedAt != "" {
				id.StartedAt = ev.StartedAt
			}
			writeErr = beforeSend(docker.Container{
				Identity:    id,
				Running:     ev.Running,
				StopTimeout: ev.StopTimeout,
			})
			return writeErr
		},
	}
	_, err := prov.Bury(s.Store.Context(), docker.EncodeContainerCredential(*containerIntentIdentity(*p)), hooks)
	if errors.Is(err, docker.ErrNotSent) && p.Phase == "sent" {
		writeErr = s.updateContainerIntent(p, func(p *ContainerIntent) { p.Phase = "prepared" })
	}
	if err == nil {
		writeErr = s.updateContainerIntent(p, func(p *ContainerIntent) { p.Phase = "confirmed" })
	}
	return err, writeErr
}

func (s *Service) resurrectContainerViaProvider(prov extres.Provider, b Burial) error {
	return prov.Resurrect(s.Store.Context(), docker.EncodeContainerCredential(*b.Container), extres.GraveArtifact{}, extres.MutateHooks{})
}

func (s *Service) purgeContainerViaProvider(prov extres.Provider, p *ContainerIntent, b Burial) error {
	hooks := extres.MutateHooks{
		BeforeSend: func(_ string, raw json.RawMessage) error {
			var ev docker.RemovalEvidence
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &ev); err != nil {
					return err
				}
			}
			return s.updateContainerIntent(p, func(p *ContainerIntent) { p.RemovalIDs = ev.IDs })
		},
	}
	err := prov.Purge(s.Store.Context(), docker.EncodeContainerCredential(*b.Container), extres.GraveArtifact{}, hooks)
	if errors.Is(err, docker.ErrReclaimed) {
		return nil
	}
	return err
}

func (s *Service) listContainerIDs(p ContainerIntent) ([]string, error) {
	id := containerIntentIdentity(p)
	if id == nil {
		return nil, docker.ErrIdentity
	}
	if prov, ok := s.provider(KindContainer); ok {
		if lister, ok := prov.(containerIDsLister); ok {
			return lister.ListIDs(s.Store.Context(), docker.EncodeContainerCredential(*id))
		}
	}
	if s.Docker != nil {
		return s.Docker.ListIDs(docker.WithTarget(s.Store.Context(), id.Target))
	}
	return nil, docker.ErrUnavailable
}

func burialVolumeArtifact(b Burial) extres.GraveArtifact {
	var started string
	if !b.VolumeEventStarted.IsZero() {
		started = b.VolumeEventStarted.UTC().Format(time.RFC3339Nano)
	}
	return extres.GraveArtifact{
		Path:         b.Grave,
		Digest:       b.VolumeArchiveDigest,
		Size:         b.Size,
		HolderID:     b.VolumeHolderID,
		EventStarted: started,
	}
}
