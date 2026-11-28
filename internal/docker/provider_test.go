package docker_test

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/extres"
	"github.com/woodfishhhh/obituary/internal/docker"
)

type fakeContainerClient struct {
	row         docker.Container
	ids         []string
	missing     bool
	inspectErr  error
	stopErr     error
	startErr    error
	removeErr   error
	reclaimed   bool
	stopped     bool
	started     bool
	removed     bool
	lastStopID  docker.Identity
	lastStartID docker.Identity
}

func (f *fakeContainerClient) Inspect(context.Context, string) (docker.Container, error) {
	if f.inspectErr != nil {
		return docker.Container{}, f.inspectErr
	}
	if f.missing {
		return docker.Container{}, docker.ErrNotFound
	}
	return f.row, nil
}
func (f *fakeContainerClient) List(context.Context) ([]docker.Container, error) {
	return []docker.Container{f.row}, nil
}
func (f *fakeContainerClient) ListIDs(context.Context) ([]string, error) {
	if f.ids != nil {
		return f.ids, nil
	}
	return []string{f.row.ID}, nil
}
func (f *fakeContainerClient) Stop(_ context.Context, id docker.Identity, beforeSend func(docker.Container) error) error {
	f.lastStopID = id
	if beforeSend != nil {
		if err := beforeSend(f.row); err != nil {
			return errors.Join(docker.ErrNotSent, err)
		}
	}
	if f.stopErr != nil {
		return f.stopErr
	}
	f.stopped = true
	f.row.Running = false
	return nil
}
func (f *fakeContainerClient) Start(_ context.Context, id docker.Identity) error {
	f.lastStartID = id
	if f.startErr != nil {
		return f.startErr
	}
	f.started = true
	f.row.Running = true
	return nil
}
func (f *fakeContainerClient) Remove(_ context.Context, _ docker.Identity, beforeSend func([]string) error) (bool, error) {
	if f.reclaimed {
		return true, nil
	}
	if beforeSend != nil {
		ids := f.ids
		if ids == nil {
			ids = []string{f.row.ID}
		}
		if err := beforeSend(ids); err != nil {
			return false, errors.Join(docker.ErrNotSent, err)
		}
	}
	if f.removeErr != nil {
		return false, f.removeErr
	}
	f.removed = true
	f.missing = true
	return false, nil
}

type fakeVolumeClient struct {
	volume         docker.Volume
	archive        []byte
	refs           []docker.Container
	missing        bool
	inspectErr     error
	refsErr        error
	archiveErr     error
	removeErr      error
	holderID       string
	eventStarted   time.Time
	releasedHolder bool
	removed        bool
	restored       []byte
}

func (f *fakeVolumeClient) InspectVolume(context.Context, string) (docker.Volume, error) {
	if f.inspectErr != nil {
		return docker.Volume{}, f.inspectErr
	}
	if f.missing {
		return docker.Volume{}, docker.ErrNoVolume
	}
	return f.volume, nil
}
func (f *fakeVolumeClient) ListVolumeReferences(context.Context, docker.Volume) ([]docker.Container, error) {
	if f.refsErr != nil {
		return nil, f.refsErr
	}
	return f.refs, nil
}
func (f *fakeVolumeClient) ArchiveVolume(_ context.Context, vol *docker.Volume, out io.Writer) (docker.VolumeArchive, error) {
	vol.HolderID = f.holderID
	vol.EventStarted = f.eventStarted
	if f.archiveErr != nil {
		return docker.VolumeArchive{}, f.archiveErr
	}
	_, err := out.Write(f.archive)
	return docker.VolumeArchive{}, err
}
func (f *fakeVolumeClient) RemoveVolume(context.Context, *docker.Volume) error {
	f.removed = true
	f.missing = true
	return f.removeErr
}
func (f *fakeVolumeClient) ReleaseVolumeHolder(_ context.Context, vol *docker.Volume) error {
	f.releasedHolder = true
	vol.HolderID = ""
	return nil
}
func (f *fakeVolumeClient) CreateVolume(_ context.Context, want docker.Volume, token string) (docker.Volume, error) {
	want.Labels = map[string]string{docker.RestoreLabel: token}
	want.RestoreLabelOwned = true
	f.volume = want
	f.missing = false
	return want, nil
}
func (f *fakeVolumeClient) RestoreVolume(_ context.Context, _ docker.Volume, in io.Reader) error {
	data, err := io.ReadAll(in)
	if err != nil {
		return err
	}
	f.restored = data
	return nil
}

func makeValidTar(t *testing.T, body string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := tar.NewWriter(&buf)
	if err := w.WriteHeader(&tar.Header{Name: "./", Typeflag: tar.TypeDir, Mode: 0o700}); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteHeader(&tar.Header{Name: "./file", Typeflag: tar.TypeReg, Mode: 0o600, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sampleContainerID(t *testing.T) docker.Identity {
	t.Helper()
	now := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)
	return docker.Identity{
		Target:    &docker.Target{Context: "default", Endpoint: "unix:///docker.sock", DaemonID: "daemon-1", OSType: "linux"},
		ID:        strings.Repeat("a", 64),
		ShortID:   strings.Repeat("a", 12),
		Name:      "cache",
		Created:   now,
		StartedAt: now.Add(time.Second).Format(time.RFC3339Nano),
	}
}

func sampleVolume(t *testing.T) docker.Volume {
	t.Helper()
	return docker.Volume{
		Name:      "data-vol",
		Driver:    "local",
		Scope:     "local",
		CreatedAt: time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC),
		Target:    docker.Target{Context: "default", Endpoint: "unix:///docker.sock", DaemonID: "daemon-1", OSType: "linux"},
	}
}

func TestContainerProviderContractAndInvariants(t *testing.T) {
	ctx := t.Context()
	id := sampleContainerID(t)
	stopTimeout := 15
	client := &fakeContainerClient{
		row: docker.Container{Identity: id, Running: true, StopTimeout: &stopTimeout},
	}
	prov := docker.NewContainerProvider(client)

	if prov.Kind() != docker.KindContainer || !prov.RequireElapsedRetention() || !prov.ExcludeFromRecycleMode() {
		t.Fatalf("unexpected container provider metadata/policy")
	}

	// Credential encode/decode round-trip preserves all fields.
	cred, err := prov.Inspect(ctx, "cache")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if err := prov.Validate(cred); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	decoded, err := docker.DecodeContainerCredential(cred)
	if err != nil || !decoded.Same(id) || decoded.StartedAt != id.StartedAt || decoded.Name != id.Name || decoded.ShortID != id.ShortID {
		t.Fatalf("DecodeContainerCredential lost fields: %+v vs %+v (err=%v)", decoded, id, err)
	}

	// Invalid credentials are rejected.
	if err := prov.Validate(extres.Credential{Kind: docker.KindContainer, Payload: []byte(`{"id":"short"}`)}); !errors.Is(err, docker.ErrIdentity) {
		t.Fatalf("expected ErrIdentity for short ID, got %v", err)
	}

	// AutoRemove is rejected on Inspect and Verify.
	client.row.AutoRemove = true
	if _, err := prov.Inspect(ctx, "cache"); !errors.Is(err, docker.ErrAutoRemove) {
		t.Fatalf("expected ErrAutoRemove on Inspect, got %v", err)
	}
	if _, err := prov.Verify(ctx, cred); !errors.Is(err, docker.ErrAutoRemove) {
		t.Fatalf("expected ErrAutoRemove on Verify, got %v", err)
	}
	client.row.AutoRemove = false

	// Verify verdicts: Reclaimed (running when StartedAt saved), Present (stopped same StartedAt),
	// Reclaimed (StartedAt drift while stopped), Identity mismatch, NotFound, Unavailable.
	obs, err := prov.Verify(ctx, cred)
	if err != nil || obs.Verdict != extres.VerdictReclaimed || !obs.Active {
		t.Fatalf("running buried container verdict = %+v err=%v", obs, err)
	}

	client.row.Running = false
	obs, err = prov.Verify(ctx, cred)
	if err != nil || obs.Verdict != extres.VerdictPresent || obs.Active {
		t.Fatalf("stopped buried container verdict = %+v err=%v", obs, err)
	}

	client.row.StartedAt = id.Created.Add(5 * time.Minute).Format(time.RFC3339Nano)
	obs, err = prov.Verify(ctx, cred)
	if err != nil || obs.Verdict != extres.VerdictReclaimed || obs.Active {
		t.Fatalf("StartedAt drift verdict = %+v err=%v", obs, err)
	}
	client.row.StartedAt = id.StartedAt

	client.row.Created = id.Created.Add(time.Hour)
	obs, err = prov.Verify(ctx, cred)
	if !errors.Is(err, docker.ErrIdentity) || obs.Verdict != extres.VerdictIdentity {
		t.Fatalf("identity mismatch verdict = %+v err=%v", obs, err)
	}
	client.row.Created = id.Created

	client.missing = true
	obs, err = prov.Verify(ctx, cred)
	if err != nil || obs.Verdict != extres.VerdictNotFound {
		t.Fatalf("missing container verdict = %+v err=%v", obs, err)
	}
	client.missing = false

	client.inspectErr = docker.ErrUnavailable
	obs, err = prov.Verify(ctx, cred)
	if !errors.Is(err, docker.ErrUnavailable) || obs.Verdict != extres.VerdictUnavailable {
		t.Fatalf("unavailable container verdict = %+v err=%v", obs, err)
	}
	client.inspectErr = nil

	// Bury forwards StopEvidence via BeforeSend hook.
	client.row.Running = true
	var gotPhase string
	var gotStop docker.StopEvidence
	_, err = prov.Bury(ctx, cred, extres.MutateHooks{
		BeforeSend: func(phase string, raw json.RawMessage) error {
			gotPhase = phase
			return json.Unmarshal(raw, &gotStop)
		},
	})
	if err != nil || !client.stopped || gotPhase != "stop" || !gotStop.Running || gotStop.StartedAt != id.StartedAt || gotStop.StopTimeout == nil || *gotStop.StopTimeout != 15 {
		t.Fatalf("Bury = err:%v stopped:%v phase:%q evidence:%+v", err, client.stopped, gotPhase, gotStop)
	}

	// Resurrect starts the container.
	if err := prov.Resurrect(ctx, cred, extres.GraveArtifact{}, extres.MutateHooks{}); err != nil || !client.started {
		t.Fatalf("Resurrect = err:%v started:%v", err, client.started)
	}

	// Purge rejects reclaimed container.
	client.reclaimed = true
	if err := prov.Purge(ctx, cred, extres.GraveArtifact{}, extres.MutateHooks{}); !errors.Is(err, docker.ErrReclaimed) {
		t.Fatalf("expected ErrReclaimed on Purge, got %v", err)
	}
	client.reclaimed = false

	// Purge forwards RemovalEvidence via BeforeSend hook and removes container.
	var gotRemoval docker.RemovalEvidence
	err = prov.Purge(ctx, cred, extres.GraveArtifact{}, extres.MutateHooks{
		BeforeSend: func(phase string, raw json.RawMessage) error {
			gotPhase = phase
			return json.Unmarshal(raw, &gotRemoval)
		},
	})
	if err != nil || !client.removed || gotPhase != "rm" || len(gotRemoval.IDs) != 1 || gotRemoval.IDs[0] != id.ID {
		t.Fatalf("Purge = err:%v removed:%v phase:%q evidence:%+v", err, client.removed, gotPhase, gotRemoval)
	}
}

func TestContainerProviderNameCollisionPropagatesFromCLI(t *testing.T) {
	ctx := t.Context()
	id := sampleContainerID(t)
	client := &fakeContainerClient{
		row:     docker.Container{Identity: id, Running: true},
		stopErr: docker.ErrNameCollision,
	}
	prov := docker.NewContainerProvider(client)
	_, err := prov.Bury(ctx, docker.EncodeContainerCredential(id), extres.MutateHooks{})
	if !errors.Is(err, docker.ErrNameCollision) {
		t.Fatalf("expected ErrNameCollision to propagate, got %v", err)
	}
}

func TestVolumeProviderContractAndInvariants(t *testing.T) {
	ctx := t.Context()
	vol := sampleVolume(t)
	payload := makeValidTar(t, "hello-volume")
	client := &fakeVolumeClient{
		volume:       vol,
		archive:      payload,
		holderID:     strings.Repeat("b", 64),
		eventStarted: time.Date(2026, 11, 28, 12, 5, 0, 0, time.UTC),
	}
	prov := docker.NewVolumeProvider(client)

	if prov.Kind() != docker.KindDockerVolume || !prov.RequireElapsedRetention() || !prov.ExcludeFromRecycleMode() {
		t.Fatalf("unexpected volume provider metadata/policy")
	}

	cred, err := prov.Inspect(ctx, vol.Name)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if err := prov.Validate(cred); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// Verify verdicts: Busy, Present, Identity mismatch, NotFound, Unavailable.
	client.refs = []docker.Container{{Identity: sampleContainerID(t)}}
	obs, err := prov.Verify(ctx, cred)
	if err != nil || obs.Verdict != extres.VerdictBusy || !obs.Active {
		t.Fatalf("busy volume verdict = %+v err=%v", obs, err)
	}
	client.refs = nil

	obs, err = prov.Verify(ctx, cred)
	if err != nil || obs.Verdict != extres.VerdictPresent || obs.Active {
		t.Fatalf("present volume verdict = %+v err=%v", obs, err)
	}

	client.volume.CreatedAt = vol.CreatedAt.Add(time.Minute)
	obs, err = prov.Verify(ctx, cred)
	if !errors.Is(err, docker.ErrIdentity) || obs.Verdict != extres.VerdictIdentity {
		t.Fatalf("identity mismatch verdict = %+v err=%v", obs, err)
	}
	client.volume.CreatedAt = vol.CreatedAt

	client.missing = true
	obs, err = prov.Verify(ctx, cred)
	if err != nil || obs.Verdict != extres.VerdictNotFound {
		t.Fatalf("missing volume verdict = %+v err=%v", obs, err)
	}
	client.missing = false

	client.inspectErr = docker.ErrUnavailable
	obs, err = prov.Verify(ctx, cred)
	if !errors.Is(err, docker.ErrUnavailable) || obs.Verdict != extres.VerdictUnavailable {
		t.Fatalf("unavailable volume verdict = %+v err=%v", obs, err)
	}
	client.inspectErr = nil

	// Bury archives, verifies, emits phases ("archived" -> "removed"), and removes volume.
	graveDir := filepath.Join(t.TempDir(), "grave")
	if err := os.MkdirAll(graveDir, 0o700); err != nil {
		t.Fatal(err)
	}
	var phases []string
	artifact, err := prov.Bury(ctx, cred, extres.MutateHooks{
		PrepareGraveDir: func(int64) (string, func(), error) {
			return graveDir, func() { _ = os.RemoveAll(graveDir) }, nil
		},
		BeforeSend: func(phase string, raw json.RawMessage) error {
			phases = append(phases, phase)
			var a extres.GraveArtifact
			if err := json.Unmarshal(raw, &a); err != nil || a.Digest == "" {
				return errors.New("missing artifact evidence")
			}
			return nil
		},
	})
	if err != nil || !client.removed || artifact.Digest == "" || artifact.Size == 0 || artifact.HolderID != client.holderID {
		t.Fatalf("Bury = artifact:%+v removed:%v err:%v", artifact, client.removed, err)
	}
	if strings.Join(phases, ",") != "archived,removed" {
		t.Fatalf("phases = %v, want [archived removed]", phases)
	}

	// Resurrect verifies archive digest and restores volume contents.
	if err := prov.Resurrect(ctx, cred, artifact, extres.MutateHooks{}); err != nil {
		t.Fatalf("Resurrect: %v", err)
	}
	if !bytes.Equal(client.restored, payload) {
		t.Fatalf("restored archive bytes mismatch")
	}

	// Resurrect refuses when a volume with the original name already exists.
	if err := prov.Resurrect(ctx, cred, artifact, extres.MutateHooks{}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected existing volume refusal, got %v", err)
	}

	// Purge requires digest and invokes BeforeSend("purge").
	if err := prov.Purge(ctx, cred, extres.GraveArtifact{}, extres.MutateHooks{}); err == nil {
		t.Fatal("expected Purge to reject empty digest")
	}
	purged := false
	err = prov.Purge(ctx, cred, artifact, extres.MutateHooks{
		BeforeSend: func(phase string, _ json.RawMessage) error {
			purged = phase == "purge"
			return nil
		},
	})
	if err != nil || !purged {
		t.Fatalf("Purge = err:%v purged:%v", err, purged)
	}
}
