package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/woodfishhhh/obituary/internal/core/extres"
)

// KindContainer is the external-resource kind identifier for Docker containers.
const KindContainer = "container"

// ErrReclaimed reports that a buried container was started outside Obituary.
var ErrReclaimed = errors.New("container was restarted since burial; kept")

// StopEvidence is passed to MutateHooks.BeforeSend("stop", ...) immediately
// before docker stop is started so lifecycle can persist recovery state.
type StopEvidence struct {
	StartedAt   string `json:"startedAt,omitempty"`
	Running     bool   `json:"running"`
	StopTimeout *int   `json:"stopTimeout,omitempty"`
}

// RemovalEvidence is passed to MutateHooks.BeforeSend("rm", ...) immediately
// before docker rm is started so lifecycle can persist the pre-removal inventory.
type RemovalEvidence struct {
	IDs []string `json:"ids"`
}

// EncodeContainerCredential serializes a Docker container identity into an
// immutable extres.Credential without dropping any identity or target fields.
func EncodeContainerCredential(id Identity) extres.Credential {
	payload, _ := json.Marshal(id)
	return extres.Credential{
		Kind:    KindContainer,
		Key:     id.Key(),
		Display: id.Display(),
		Payload: payload,
	}
}

// DecodeContainerCredential restores a Docker container identity from cred.
func DecodeContainerCredential(cred extres.Credential) (Identity, error) {
	if cred.Kind != "" && cred.Kind != KindContainer || len(cred.Payload) == 0 {
		return Identity{}, ErrIdentity
	}
	var id Identity
	if err := json.Unmarshal(cred.Payload, &id); err != nil {
		return Identity{}, fmt.Errorf("%w: %w", ErrIdentity, err)
	}
	return id, nil
}

// ContainerProvider adapts a docker.Client to extres.Provider and RetentionPolicy.
type ContainerProvider struct {
	client Client
}

var (
	_ extres.Provider        = (*ContainerProvider)(nil)
	_ extres.RetentionPolicy = (*ContainerProvider)(nil)
)

// NewContainerProvider wraps client as an extres.Provider for Docker containers.
func NewContainerProvider(client Client) *ContainerProvider {
	return &ContainerProvider{client: client}
}

// Client returns the wrapped Docker client.
func (p *ContainerProvider) Client() Client {
	if p == nil {
		return nil
	}
	return p.client
}

// Kind implements extres.Provider.
func (p *ContainerProvider) Kind() string { return KindContainer }

// RequireElapsedRetention implements extres.RetentionPolicy.
func (p *ContainerProvider) RequireElapsedRetention() bool { return true }

// ExcludeFromRecycleMode implements extres.RetentionPolicy.
func (p *ContainerProvider) ExcludeFromRecycleMode() bool { return true }

// Validate checks that cred carries a valid 64-hex container ID and target.
func (p *ContainerProvider) Validate(cred extres.Credential) error {
	id, err := DecodeContainerCredential(cred)
	if err != nil || !id.Valid() {
		return ErrIdentity
	}
	return nil
}

// Inspect resolves ref on the current daemon and rejects AutoRemove containers.
func (p *ContainerProvider) Inspect(ctx context.Context, ref string) (extres.Credential, error) {
	if p == nil || p.client == nil {
		return extres.Credential{}, ErrUnavailable
	}
	got, err := p.client.Inspect(ctx, ref)
	if err != nil {
		return extres.Credential{}, err
	}
	if got.AutoRemove {
		return extres.Credential{}, ErrAutoRemove
	}
	if !got.Valid() {
		return extres.Credential{}, ErrIdentity
	}
	return EncodeContainerCredential(got.Identity), nil
}

// Verify re-inspects saved on its pinned target and classifies its live state.
func (p *ContainerProvider) Verify(ctx context.Context, saved extres.Credential) (extres.Observation, error) {
	want, err := DecodeContainerCredential(saved)
	if err != nil || !want.Valid() {
		return extres.Observation{Verdict: extres.VerdictIdentity, Detail: ErrIdentity.Error()}, ErrIdentity
	}
	if p == nil || p.client == nil {
		return extres.Observation{Verdict: extres.VerdictUnavailable, Detail: ErrUnavailable.Error()}, ErrUnavailable
	}
	got, err := p.client.Inspect(WithTarget(ctx, want.Target), want.ID)
	if err != nil {
		return classifyContainerError(err)
	}
	if !got.Same(want) {
		return extres.Observation{Verdict: extres.VerdictIdentity, Detail: ErrIdentity.Error()}, ErrIdentity
	}
	return observeContainer(want, got)
}

func classifyContainerError(err error) (extres.Observation, error) {
	if errors.Is(err, ErrNotFound) {
		return extres.Observation{Verdict: extres.VerdictNotFound, Detail: err.Error()}, nil
	}
	if errors.Is(err, ErrIdentity) {
		return extres.Observation{Verdict: extres.VerdictIdentity, Detail: err.Error()}, err
	}
	return extres.Observation{Verdict: extres.VerdictUnavailable, Detail: err.Error()}, err
}

func observeContainer(want Identity, got Container) (extres.Observation, error) {
	updated := EncodeContainerCredential(got.Identity)
	if got.AutoRemove {
		return extres.Observation{Verdict: extres.VerdictPresent, Active: got.Running, Detail: ErrAutoRemove.Error(), Updated: &updated}, ErrAutoRemove
	}
	if want.StartedAt != "" {
		reclaimed, err := got.Reclaimed(want)
		if err == nil && reclaimed {
			return extres.Observation{Verdict: extres.VerdictReclaimed, Active: got.Running, Detail: ErrReclaimed.Error(), Updated: &updated}, nil
		}
	}
	return extres.Observation{Verdict: extres.VerdictPresent, Active: got.Running, Updated: &updated}, nil
}

// Bury stops the pinned container, forwarding pre-send evidence through hooks.
func (p *ContainerProvider) Bury(ctx context.Context, saved extres.Credential, hooks extres.MutateHooks) (extres.GraveArtifact, error) {
	want, err := DecodeContainerCredential(saved)
	if err != nil || !want.Valid() {
		return extres.GraveArtifact{}, ErrIdentity
	}
	if p == nil || p.client == nil {
		return extres.GraveArtifact{}, ErrUnavailable
	}
	var beforeSend func(Container) error
	if hooks.BeforeSend != nil {
		beforeSend = func(got Container) error {
			raw, marshalErr := json.Marshal(StopEvidence{
				StartedAt:   got.StartedAt,
				Running:     got.Running,
				StopTimeout: got.StopTimeout,
			})
			if marshalErr != nil {
				return marshalErr
			}
			return hooks.BeforeSend("stop", raw)
		}
	}
	return extres.GraveArtifact{}, p.client.Stop(ctx, want, beforeSend)
}

// Resurrect starts the pinned container on its original daemon.
func (p *ContainerProvider) Resurrect(ctx context.Context, saved extres.Credential, _ extres.GraveArtifact, _ extres.MutateHooks) error {
	want, err := DecodeContainerCredential(saved)
	if err != nil || !want.Valid() {
		return ErrIdentity
	}
	if p == nil || p.client == nil {
		return ErrUnavailable
	}
	return p.client.Start(ctx, want)
}

// Purge removes the pinned stopped container, recording pre-removal IDs via hooks.
func (p *ContainerProvider) Purge(ctx context.Context, saved extres.Credential, _ extres.GraveArtifact, hooks extres.MutateHooks) error {
	want, err := DecodeContainerCredential(saved)
	if err != nil || !want.Valid() {
		return ErrIdentity
	}
	if p == nil || p.client == nil {
		return ErrUnavailable
	}
	var beforeSend func([]string) error
	if hooks.BeforeSend != nil {
		beforeSend = func(ids []string) error {
			raw, marshalErr := json.Marshal(RemovalEvidence{IDs: ids})
			if marshalErr != nil {
				return marshalErr
			}
			return hooks.BeforeSend("rm", raw)
		}
	}
	reclaimed, err := p.client.Remove(ctx, want, beforeSend)
	if err != nil {
		return err
	}
	if reclaimed {
		return ErrReclaimed
	}
	return nil
}

// ListIDs lists container IDs on the target pinned in saved.
func (p *ContainerProvider) ListIDs(ctx context.Context, saved extres.Credential) ([]string, error) {
	want, err := DecodeContainerCredential(saved)
	if err != nil || !want.Valid() {
		return nil, ErrIdentity
	}
	if p == nil || p.client == nil {
		return nil, ErrUnavailable
	}
	return p.client.ListIDs(WithTarget(ctx, want.Target))
}
