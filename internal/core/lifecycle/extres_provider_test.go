package lifecycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/extres"
	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/docker"
)

type memoryContainerProvider struct {
	id             docker.Identity
	running        bool
	missing        bool
	reclaimed      bool
	requireElapsed bool
	excludeRecycle bool
	calls          []string
}

func (m *memoryContainerProvider) Kind() string                  { return core.KindContainer }
func (m *memoryContainerProvider) RequireElapsedRetention() bool { return m.requireElapsed }
func (m *memoryContainerProvider) ExcludeFromRecycleMode() bool  { return m.excludeRecycle }

func (m *memoryContainerProvider) Validate(cred extres.Credential) error {
	m.calls = append(m.calls, "validate")
	id, err := docker.DecodeContainerCredential(cred)
	if err != nil || !id.Valid() {
		return docker.ErrIdentity
	}
	return nil
}

func (m *memoryContainerProvider) Inspect(_ context.Context, _ string) (extres.Credential, error) {
	m.calls = append(m.calls, "inspect")
	if m.missing {
		return extres.Credential{}, docker.ErrNotFound
	}
	return docker.EncodeContainerCredential(m.id), nil
}

func (m *memoryContainerProvider) Verify(_ context.Context, _ extres.Credential) (extres.Observation, error) {
	m.calls = append(m.calls, "verify")
	if m.missing {
		return extres.Observation{Verdict: extres.VerdictNotFound}, nil
	}
	updated := docker.EncodeContainerCredential(m.id)
	if m.reclaimed {
		return extres.Observation{Verdict: extres.VerdictReclaimed, Active: m.running, Updated: &updated}, nil
	}
	return extres.Observation{Verdict: extres.VerdictPresent, Active: m.running, Updated: &updated}, nil
}

func (m *memoryContainerProvider) Bury(_ context.Context, _ extres.Credential, hooks extres.MutateHooks) (extres.GraveArtifact, error) {
	m.calls = append(m.calls, "bury")
	if hooks.BeforeSend != nil {
		raw, _ := json.Marshal(docker.StopEvidence{StartedAt: m.id.StartedAt, Running: m.running})
		if err := hooks.BeforeSend("stop", raw); err != nil {
			return extres.GraveArtifact{}, errors.Join(docker.ErrNotSent, err)
		}
	}
	m.running = false
	return extres.GraveArtifact{}, nil
}

func (m *memoryContainerProvider) Resurrect(_ context.Context, _ extres.Credential, _ extres.GraveArtifact, _ extres.MutateHooks) error {
	m.calls = append(m.calls, "resurrect")
	m.running = true
	return nil
}

func (m *memoryContainerProvider) Purge(_ context.Context, _ extres.Credential, _ extres.GraveArtifact, _ extres.MutateHooks) error {
	m.calls = append(m.calls, "purge")
	m.missing = true
	return nil
}

type memoryVolumeProvider struct {
	volume         docker.Volume
	missing        bool
	busyDetail     string
	identityBad    bool
	requireElapsed bool
	excludeRecycle bool
	calls          []string
}

func (m *memoryVolumeProvider) Kind() string                  { return core.KindDockerVolume }
func (m *memoryVolumeProvider) RequireElapsedRetention() bool { return m.requireElapsed }
func (m *memoryVolumeProvider) ExcludeFromRecycleMode() bool  { return m.excludeRecycle }

func (m *memoryVolumeProvider) Validate(cred extres.Credential) error {
	m.calls = append(m.calls, "validate")
	v, err := docker.DecodeVolumeCredential(cred)
	if err != nil {
		return err
	}
	return v.Check()
}

func (m *memoryVolumeProvider) Inspect(_ context.Context, _ string) (extres.Credential, error) {
	m.calls = append(m.calls, "inspect")
	if m.missing {
		return extres.Credential{}, docker.ErrNoVolume
	}
	return docker.EncodeVolumeCredential(m.volume), nil
}

func (m *memoryVolumeProvider) Verify(_ context.Context, _ extres.Credential) (extres.Observation, error) {
	m.calls = append(m.calls, "verify")
	if m.missing {
		return extres.Observation{Verdict: extres.VerdictNotFound}, nil
	}
	if m.identityBad {
		return extres.Observation{Verdict: extres.VerdictIdentity, Detail: "Docker volume identity changed"}, docker.ErrIdentity
	}
	if m.busyDetail != "" {
		return extres.Observation{Verdict: extres.VerdictBusy, Active: true, Detail: m.busyDetail}, nil
	}
	updated := docker.EncodeVolumeCredential(m.volume)
	return extres.Observation{Verdict: extres.VerdictPresent, Updated: &updated}, nil
}

func (m *memoryVolumeProvider) Bury(_ context.Context, _ extres.Credential, _ extres.MutateHooks) (extres.GraveArtifact, error) {
	m.calls = append(m.calls, "bury")
	m.missing = true
	return extres.GraveArtifact{
		Path:   "memory://volume/archive.tar",
		Digest: strings.Repeat("d", 64),
		Size:   1024,
	}, nil
}

func (m *memoryVolumeProvider) Resurrect(_ context.Context, _ extres.Credential, _ extres.GraveArtifact, _ extres.MutateHooks) error {
	m.calls = append(m.calls, "resurrect")
	m.missing = false
	return nil
}

func (m *memoryVolumeProvider) Purge(_ context.Context, _ extres.Credential, _ extres.GraveArtifact, _ extres.MutateHooks) error {
	m.calls = append(m.calls, "purge")
	return nil
}

func TestLifecycleDispatchesCustomContainerProviderWithoutDocker(t *testing.T) {
	f := newFixture(t)
	id := docker.Identity{
		Target:    &docker.Target{Context: "stub", Endpoint: "memory://stub", DaemonID: "stub-daemon", OSType: "linux"},
		ID:        strings.Repeat("c", 64),
		ShortID:   strings.Repeat("c", 12),
		Name:      "stub-web",
		Created:   f.clock.Add(-time.Hour),
		StartedAt: f.clock.Add(-30 * time.Minute).Format(time.RFC3339Nano),
	}
	stub := &memoryContainerProvider{id: id, running: true, requireElapsed: true, excludeRecycle: true}
	f.svc.Docker = nil
	f.svc.Providers = map[string]extres.Provider{core.KindContainer: stub}

	// Register and sweep via custom provider even in recycle mode (ExcludeFromRecycleMode=true).
	f.svc.Mode = core.ModeRecycle
	it, err := f.svc.AddContainer("stub-web", core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err != nil {
		t.Fatalf("AddContainer via stub provider: %v", err)
	}
	f.clock = f.clock.Add(2 * time.Hour)
	outcomes, err := f.svc.Sweep(false)
	if err != nil || len(outcomes) != 1 || outcomes[0].Action != core.ActionBuried || stub.running {
		t.Fatalf("Sweep via stub provider = %+v running=%v err=%v", outcomes, stub.running, err)
	}

	// Early purge is refused when RetentionPolicy.RequireElapsedRetention() is true.
	early, err := f.svc.PurgeRefs([]string{fmt.Sprintf("#%d", it.ID)}, false)
	if err != nil || len(early) != 1 || !strings.Contains(early[0].Error, "container retention has not elapsed") {
		t.Fatalf("expected retention refusal, got %+v err=%v", early, err)
	}

	// Resurrect calls stub.Resurrect.
	res, err := f.svc.Resurrect([]string{fmt.Sprintf("#%d", it.ID)})
	if err != nil || len(res) != 1 || res[0].Action != core.ActionResurrected || !stub.running {
		t.Fatalf("Resurrect via stub provider = %+v running=%v err=%v", res, stub.running, err)
	}

	// Re-add, bury tracked, and verify RequireElapsedRetention=false allows immediate purge.
	it2, err := f.svc.AddContainer("stub-web", core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err != nil {
		t.Fatalf("re-AddContainer: %v", err)
	}
	buried, err := f.svc.BuryTracked([]string{fmt.Sprintf("#%d", it2.ID)})
	if err != nil || len(buried) != 1 || buried[0].Action != core.ActionBuried {
		t.Fatalf("BuryTracked via stub provider = %+v err=%v", buried, err)
	}
	stub.requireElapsed = false
	purged, err := f.svc.PurgeRefs([]string{fmt.Sprintf("#%d", it2.ID)}, false)
	if err != nil || len(purged) != 1 || purged[0].Action != core.ActionPurged || !stub.missing {
		t.Fatalf("PurgeRefs via stub provider = %+v missing=%v err=%v", purged, stub.missing, err)
	}
	for _, wantCall := range []string{"inspect", "verify", "bury", "resurrect", "purge"} {
		if !strings.Contains(strings.Join(stub.calls, ","), wantCall) {
			t.Fatalf("stub provider missing %q call; calls=%v", wantCall, stub.calls)
		}
	}
}

func TestLifecycleContainerProviderReclaimedAndRecyclePolicy(t *testing.T) {
	f := newFixture(t)
	id := docker.Identity{
		Target:    &docker.Target{Context: "stub", Endpoint: "memory://stub", DaemonID: "stub-daemon", OSType: "linux"},
		ID:        strings.Repeat("e", 64),
		ShortID:   strings.Repeat("e", 12),
		Name:      "stub-worker",
		Created:   f.clock.Add(-time.Hour),
		StartedAt: f.clock.Add(-30 * time.Minute).Format(time.RFC3339Nano),
	}
	stub := &memoryContainerProvider{id: id, running: true, requireElapsed: true, excludeRecycle: false}
	f.svc.Docker = nil
	f.svc.Providers = map[string]extres.Provider{core.KindContainer: stub}
	f.svc.Mode = core.ModeRecycle

	it, err := f.svc.AddContainer("stub-worker", core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	f.clock = f.clock.Add(2 * time.Hour)
	outcomes, err := f.svc.Sweep(false)
	if err != nil || len(outcomes) != 1 || outcomes[0].Action != core.ActionRefused {
		t.Fatalf("expected recycle-mode refusal when ExcludeFromRecycleMode=false, got %+v err=%v", outcomes, err)
	}

	// Switch to graveyard mode, re-add, bury, then mark reclaimed so Purge reconciles ActionResurrected.
	f.svc.Mode = core.ModeGraveyard
	it, err = f.svc.AddContainer("stub-worker", core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if rows, err := f.svc.BuryTracked([]string{fmt.Sprintf("#%d", it.ID)}); err != nil || rows[0].Action != core.ActionBuried {
		t.Fatalf("bury: %+v %v", rows, err)
	}
	stub.reclaimed = true
	f.clock = f.clock.Add(8 * 24 * time.Hour)
	_, purged, err := f.svc.Tick()
	if err != nil || len(purged) != 1 || purged[0].Action != core.ActionResurrected {
		t.Fatalf("expected reclaimed container to reconcile as resurrected, got %+v err=%v", purged, err)
	}
}

func TestLifecycleDispatchesCustomVolumeProviderWithoutDocker(t *testing.T) {
	f := newFixture(t)
	vol := docker.Volume{
		Name:      "stub-vol",
		Driver:    "local",
		Scope:     "local",
		CreatedAt: f.clock,
		Target:    docker.Target{Context: "stub", Endpoint: "memory://stub", DaemonID: "stub-daemon", OSType: "linux"},
	}
	stub := &memoryVolumeProvider{volume: vol, requireElapsed: true, excludeRecycle: true}
	f.svc.Docker = nil
	f.svc.Providers = map[string]extres.Provider{core.KindDockerVolume: stub}
	f.svc.Mode = core.ModeRecycle

	// Add and sweep via custom volume provider with Service.Docker == nil.
	added, err := f.svc.AddDockerVolume([]string{"stub-vol"}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err != nil || core.Failed(added) || len(added) != 1 || added[0].Item == nil {
		t.Fatalf("AddDockerVolume via stub provider: %+v %v", added, err)
	}
	volID := added[0].Item.ID

	// Busy verdict refuses burial and retains tracked item.
	stub.busyDetail = "Docker volume is referenced by stub-holder"
	f.clock = f.clock.Add(2 * time.Hour)
	busyOut, err := f.svc.Sweep(false)
	if err != nil || len(busyOut) != 1 || busyOut[0].Action != core.ActionRefused {
		t.Fatalf("busy sweep = %+v err=%v", busyOut, err)
	}
	stub.busyDetail = ""

	// Clear busy -> sweep buries via stub.Bury.
	outcomes, err := f.svc.Sweep(false)
	if err != nil || len(outcomes) != 1 || outcomes[0].Action != core.ActionBuried {
		t.Fatalf("sweep via stub volume provider = %+v err=%v", outcomes, err)
	}

	// RetentionPolicy.RequireElapsedRetention() == true refuses early purge.
	early, err := f.svc.PurgeRefs([]string{fmt.Sprintf("#%d", volID)}, false)
	if err != nil || len(early) != 1 || !strings.Contains(early[0].Error, "volume retention has not elapsed") {
		t.Fatalf("expected early volume purge refusal, got %+v err=%v", early, err)
	}

	// Resurrect dispatches to stub.Resurrect.
	res, err := f.svc.Resurrect([]string{fmt.Sprintf("#%d", volID)})
	if err != nil || len(res) != 1 || res[0].Action != core.ActionResurrected || stub.missing {
		t.Fatalf("Resurrect via stub volume provider = %+v missing=%v err=%v", res, stub.missing, err)
	}

	// Explicit bury, advance past retention, and Tick purges via stub.Purge.
	buried, err := f.svc.BuryDockerVolumes([]string{"stub-vol"}, "explicit")
	if err != nil || len(buried) != 1 || buried[0].Action != core.ActionBuried {
		t.Fatalf("BuryDockerVolumes via stub provider = %+v err=%v", buried, err)
	}
	f.clock = f.clock.Add(8 * 24 * time.Hour)
	_, purged, err := f.svc.Tick()
	if err != nil || len(purged) != 1 || purged[0].Action != core.ActionPurged {
		t.Fatalf("Tick purge via stub volume provider = %+v err=%v", purged, err)
	}
	for _, wantCall := range []string{"inspect", "verify", "bury", "resurrect", "purge"} {
		if !strings.Contains(strings.Join(stub.calls, ","), wantCall) {
			t.Fatalf("stub volume provider missing %q call; calls=%v", wantCall, stub.calls)
		}
	}
}
