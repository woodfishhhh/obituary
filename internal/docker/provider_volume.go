package docker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/extres"
)

// KindDockerVolume is the external-resource kind identifier for Docker volumes.
const KindDockerVolume = "docker-volume"

// ErrVolumeUnavailable reports that Docker volume operations cannot run.
var ErrVolumeUnavailable = errors.New("docker volume operations are unavailable")

// EncodeVolumeCredential serializes a Docker volume identity into an extres.Credential.
func EncodeVolumeCredential(v Volume) extres.Credential {
	payload, _ := json.Marshal(v)
	return extres.Credential{
		Kind:    KindDockerVolume,
		Key:     "docker-volume:" + v.Target.Endpoint + "/" + v.Target.DaemonID + "/" + v.Name,
		Display: v.Name,
		Payload: payload,
	}
}

// DecodeVolumeCredential restores a Docker volume identity from cred.
func DecodeVolumeCredential(cred extres.Credential) (Volume, error) {
	if cred.Kind != "" && cred.Kind != KindDockerVolume || len(cred.Payload) == 0 {
		return Volume{}, ErrIdentity
	}
	var v Volume
	if err := json.Unmarshal(cred.Payload, &v); err != nil {
		return Volume{}, fmt.Errorf("%w: %w", ErrIdentity, err)
	}
	return v, nil
}

// VolumeProvider adapts a docker.VolumeClient to extres.Provider and RetentionPolicy.
type VolumeProvider struct {
	client VolumeClient
}

var (
	_ extres.Provider        = (*VolumeProvider)(nil)
	_ extres.RetentionPolicy = (*VolumeProvider)(nil)
)

// NewVolumeProvider wraps client as an extres.Provider for Docker volumes.
func NewVolumeProvider(client VolumeClient) *VolumeProvider {
	return &VolumeProvider{client: client}
}

// Client returns the wrapped Docker volume client.
func (p *VolumeProvider) Client() VolumeClient {
	if p == nil {
		return nil
	}
	return p.client
}

// Kind implements extres.Provider.
func (p *VolumeProvider) Kind() string { return KindDockerVolume }

// RequireElapsedRetention implements extres.RetentionPolicy.
func (p *VolumeProvider) RequireElapsedRetention() bool { return true }

// ExcludeFromRecycleMode implements extres.RetentionPolicy.
func (p *VolumeProvider) ExcludeFromRecycleMode() bool { return true }

// Validate checks that cred carries a supported Linux local Docker volume identity.
func (p *VolumeProvider) Validate(cred extres.Credential) error {
	v, err := DecodeVolumeCredential(cred)
	if err != nil {
		return err
	}
	return v.Check()
}

// Inspect resolves a volume name on the current daemon.
func (p *VolumeProvider) Inspect(ctx context.Context, ref string) (extres.Credential, error) {
	if p == nil || p.client == nil {
		return extres.Credential{}, ErrVolumeUnavailable
	}
	v, err := p.client.InspectVolume(ctx, ref)
	if err != nil {
		return extres.Credential{}, err
	}
	return EncodeVolumeCredential(v), nil
}

// Verify re-inspects saved and checks for container references on its daemon.
func (p *VolumeProvider) Verify(ctx context.Context, saved extres.Credential) (extres.Observation, error) {
	want, err := p.decodeValidVolume(saved)
	if err != nil {
		return extres.Observation{Verdict: extres.VerdictIdentity, Detail: err.Error()}, err
	}
	if p == nil || p.client == nil {
		return extres.Observation{Verdict: extres.VerdictUnavailable, Detail: ErrVolumeUnavailable.Error()}, ErrVolumeUnavailable
	}
	got, err := p.client.InspectVolume(WithTarget(ctx, &want.Target), want.Name)
	if errors.Is(err, ErrNoVolume) {
		return extres.Observation{Verdict: extres.VerdictNotFound, Detail: err.Error()}, nil
	}
	if err != nil {
		return extres.Observation{Verdict: extres.VerdictUnavailable, Detail: err.Error()}, err
	}
	if !got.Same(want) {
		return extres.Observation{Verdict: extres.VerdictIdentity, Detail: "Docker volume identity changed"}, ErrIdentity
	}
	return p.verifyVolumeRefs(ctx, want, got)
}

func (p *VolumeProvider) verifyVolumeRefs(ctx context.Context, want, got Volume) (extres.Observation, error) {
	refs, err := p.client.ListVolumeReferences(WithTarget(ctx, &want.Target), got)
	if err != nil {
		return extres.Observation{Verdict: extres.VerdictUnavailable, Detail: err.Error()}, err
	}
	if len(refs) != 0 {
		return extres.Observation{
			Verdict: extres.VerdictBusy,
			Active:  true,
			Detail:  fmt.Sprintf("Docker volume is referenced by %s", refs[0].Display()),
		}, nil
	}
	updated := EncodeVolumeCredential(got)
	return extres.Observation{Verdict: extres.VerdictPresent, Updated: &updated}, nil
}

// Bury archives the pinned volume into the directory supplied by hooks.PrepareGraveDir,
// verifies the tar stream, notifies hooks.BeforeSend for recovery manifests, and removes the volume.
func (p *VolumeProvider) Bury(ctx context.Context, saved extres.Credential, hooks extres.MutateHooks) (extres.GraveArtifact, error) {
	vol, err := p.decodeValidVolume(saved)
	if err != nil {
		return extres.GraveArtifact{}, err
	}
	if p == nil || p.client == nil {
		return extres.GraveArtifact{}, ErrVolumeUnavailable
	}
	if hooks.PrepareGraveDir == nil {
		return extres.GraveArtifact{}, errors.New("volume burial requires a grave directory hook")
	}
	dir, cleanup, err := hooks.PrepareGraveDir(0)
	if err != nil {
		return extres.GraveArtifact{}, err
	}
	return p.archiveAndRemove(ctx, &vol, dir, cleanup, hooks)
}

func (p *VolumeProvider) archiveAndRemove(ctx context.Context, vol *Volume, dir string, cleanup func(), hooks extres.MutateHooks) (extres.GraveArtifact, error) {
	releaseHolder := true
	defer func() {
		if cleanup != nil {
			cleanup()
		}
		if releaseHolder && vol.HolderID != "" {
			_ = p.client.ReleaseVolumeHolder(ctx, vol)
		}
	}()
	archive := filepath.Join(dir, "volume.tar")
	check, err := p.streamVerifiedArchive(ctx, vol, archive)
	if err != nil {
		return extres.GraveArtifact{}, err
	}
	artifact := encodeVolumeArtifact(archive, check, *vol)
	raw, err := json.Marshal(artifact)
	if err != nil {
		return extres.GraveArtifact{}, err
	}
	if hooks.BeforeSend != nil {
		if err := hooks.BeforeSend("archived", raw); err != nil {
			return artifact, err
		}
	}
	// Once archived and persisted by the caller, keep evidence on any later failure.
	cleanup = nil
	releaseHolder = false
	return artifact, p.removeVolumeWithHooks(ctx, vol, raw, hooks)
}

func (p *VolumeProvider) streamVerifiedArchive(ctx context.Context, vol *Volume, archive string) (VolumeArchive, error) {
	partial := archive + ".partial"
	f, err := os.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return VolumeArchive{}, err
	}
	_, streamErr := p.client.ArchiveVolume(ctx, vol, f)
	closeErr := f.Close()
	if streamErr != nil || closeErr != nil {
		_ = p.client.ReleaseVolumeHolder(ctx, vol)
		return VolumeArchive{}, errors.Join(streamErr, closeErr)
	}
	check, err := verifyVolumeArchivePath(partial)
	if err != nil {
		_ = p.client.ReleaseVolumeHolder(ctx, vol)
		return VolumeArchive{}, err
	}
	if err := os.Rename(partial, archive); err != nil {
		_ = p.client.ReleaseVolumeHolder(ctx, vol)
		return VolumeArchive{}, err
	}
	return check, nil
}

func (p *VolumeProvider) removeVolumeWithHooks(ctx context.Context, vol *Volume, raw json.RawMessage, hooks extres.MutateHooks) error {
	if err := p.client.RemoveVolume(ctx, vol); err != nil {
		if errors.Is(err, ErrVolumeEvents) && hooks.BeforeSend != nil {
			return errors.Join(err, hooks.BeforeSend("events-incomplete", raw))
		}
		return err
	}
	if hooks.BeforeSend != nil {
		return hooks.BeforeSend("removed", raw)
	}
	return nil
}

// EnsureAbsent confirms that the original volume name does not already exist on the daemon.
func (p *VolumeProvider) EnsureAbsent(ctx context.Context, saved extres.Credential) error {
	want, err := p.decodeValidVolume(saved)
	if err != nil {
		return err
	}
	if p == nil || p.client == nil {
		return ErrVolumeUnavailable
	}
	_, err = p.client.InspectVolume(WithTarget(ctx, &want.Target), want.Name)
	if err == nil {
		return errors.New("a Docker volume with the original name already exists; nothing was written")
	}
	if errors.Is(err, ErrNoVolume) {
		return nil
	}
	return err
}

// ResurrectFrom restores saved from an already-opened, root-confined archive handle.
func (p *VolumeProvider) ResurrectFrom(ctx context.Context, saved extres.Credential, archive io.ReadSeeker, digest string) error {
	want, err := p.decodeValidVolume(saved)
	if err != nil {
		return err
	}
	if p == nil || p.client == nil {
		return ErrVolumeUnavailable
	}
	if check, err := VerifyVolumeArchive(archive); err != nil || digest == "" || check.Digest != digest {
		return errors.New("docker volume archive failed verification")
	}
	if _, err := p.client.CreateVolume(ctx, want, randomRestoreToken()); err != nil {
		return err
	}
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return p.client.RestoreVolume(ctx, want, archive)
}

// Resurrect ensures the original name is absent, verifies grave.Path, and restores the volume.
func (p *VolumeProvider) Resurrect(ctx context.Context, saved extres.Credential, grave extres.GraveArtifact, _ extres.MutateHooks) error {
	if err := p.EnsureAbsent(ctx, saved); err != nil {
		return err
	}
	f, err := os.Open(grave.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	return p.ResurrectFrom(ctx, saved, f, grave.Digest)
}

// Purge validates the credential and archive digest, delegating root-confined
// archive removal to hooks.BeforeSend("purge", nil) when provided.
func (p *VolumeProvider) Purge(_ context.Context, saved extres.Credential, grave extres.GraveArtifact, hooks extres.MutateHooks) error {
	if _, err := p.decodeValidVolume(saved); err != nil {
		return err
	}
	if grave.Digest == "" {
		return errors.New("docker volume archive has no verification digest")
	}
	if hooks.BeforeSend != nil {
		return hooks.BeforeSend("purge", nil)
	}
	return nil
}

func (p *VolumeProvider) decodeValidVolume(cred extres.Credential) (Volume, error) {
	v, err := DecodeVolumeCredential(cred)
	if err != nil {
		return Volume{}, err
	}
	if err := v.Check(); err != nil {
		return Volume{}, err
	}
	return v, nil
}

func encodeVolumeArtifact(path string, check VolumeArchive, vol Volume) extres.GraveArtifact {
	var started string
	if !vol.EventStarted.IsZero() {
		started = vol.EventStarted.UTC().Format(time.RFC3339Nano)
	}
	return extres.GraveArtifact{
		Path:         path,
		Digest:       check.Digest,
		Size:         check.Bytes,
		HolderID:     vol.HolderID,
		EventStarted: started,
	}
}

func verifyVolumeArchivePath(path string) (VolumeArchive, error) {
	f, err := os.Open(path)
	if err != nil {
		return VolumeArchive{}, err
	}
	defer f.Close()
	return VerifyVolumeArchive(f)
}

func randomRestoreToken() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return hex.EncodeToString(raw[:])
	}
	return hex.EncodeToString(raw[:])
}
