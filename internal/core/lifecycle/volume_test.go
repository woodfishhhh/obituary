package lifecycle_test

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/docker"
)

type fakeVolumeClient struct {
	volume    docker.Volume
	archive   []byte
	refs      []docker.Container
	removed   bool
	removeErr error
}

func (f *fakeVolumeClient) Inspect(context.Context, string) (docker.Container, error) {
	return docker.Container{}, docker.ErrNotFound
}
func (f *fakeVolumeClient) List(context.Context) ([]docker.Container, error) { return f.refs, nil }
func (f *fakeVolumeClient) ListIDs(context.Context) ([]string, error)        { return nil, nil }
func (f *fakeVolumeClient) Stop(context.Context, docker.Identity, func(docker.Container) error) error {
	return nil
}
func (f *fakeVolumeClient) Start(context.Context, docker.Identity) error { return nil }
func (f *fakeVolumeClient) Remove(context.Context, docker.Identity, func([]string) error) (bool, error) {
	return false, nil
}

func (f *fakeVolumeClient) InspectVolume(context.Context, string) (docker.Volume, error) {
	if f.removed {
		return docker.Volume{}, docker.ErrNoVolume
	}
	return f.volume, nil
}
func (f *fakeVolumeClient) ListVolumeReferences(context.Context, docker.Volume) ([]docker.Container, error) {
	return f.refs, nil
}
func (f *fakeVolumeClient) ArchiveVolume(_ context.Context, _ *docker.Volume, out io.Writer) (docker.VolumeArchive, error) {
	_, err := out.Write(f.archive)
	return docker.VolumeArchive{}, err
}
func (f *fakeVolumeClient) RemoveVolume(context.Context, *docker.Volume) error {
	f.removed = true
	return f.removeErr
}
func (f *fakeVolumeClient) ReleaseVolumeHolder(context.Context, *docker.Volume) error { return nil }
func (f *fakeVolumeClient) CreateVolume(_ context.Context, want docker.Volume, token string) (docker.Volume, error) {
	want.Labels = map[string]string{docker.RestoreLabel: token}
	want.RestoreLabelOwned = true
	f.volume, f.removed = want, false
	return want, nil
}
func (f *fakeVolumeClient) RestoreVolume(_ context.Context, _ docker.Volume, in io.Reader) error {
	got, err := io.ReadAll(in)
	if err == nil && !bytes.Equal(got, f.archive) {
		return io.ErrUnexpectedEOF
	}
	return err
}

func volumeFixture(t *testing.T) (*fixture, *fakeVolumeClient) {
	t.Helper()
	f := newFixture(t)
	v := docker.Volume{Name: "cache", Driver: "local", Scope: "local", CreatedAt: f.clock, Target: docker.Target{Context: "test", Endpoint: "unix:///docker.sock", DaemonID: "daemon", OSType: "linux"}}
	var archive bytes.Buffer
	w := tar.NewWriter(&archive)
	if err := w.WriteHeader(&tar.Header{Name: "./", Typeflag: tar.TypeDir, Mode: 0700}); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteHeader(&tar.Header{Name: "./data", Typeflag: tar.TypeReg, Mode: 0600, Size: 4}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("data")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	client := &fakeVolumeClient{volume: v, archive: archive.Bytes()}
	f.svc.Docker = client
	return f, client
}

func TestDockerVolumeBuryRejectsContainerReference(t *testing.T) {
	f, client := volumeFixture(t)
	client.refs = []docker.Container{{Identity: docker.Identity{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Name: "holder"}}}
	rows, err := f.svc.BuryDockerVolumes([]string{"cache"}, "explicit")
	if err != nil || len(rows) != 1 || rows[0].Action != core.ActionRefused || !client.removed {
		if client.removed {
			t.Fatal("volume was removed despite a reference", rows, err)
		}
		if err != nil || len(rows) != 1 || rows[0].Action != core.ActionRefused {
			t.Fatal(rows, err)
		}
	}
}

func TestDockerVolumeBuryAndResurrectRoundTrip(t *testing.T) {
	f, client := volumeFixture(t)
	rows, err := f.svc.BuryDockerVolumes([]string{"cache"}, "explicit")
	if err != nil || len(rows) != 1 || rows[0].Action != core.ActionBuried || !client.removed {
		t.Fatal(rows, err)
	}
	burial := rows[0].Burial
	if burial == nil || burial.VolumeArchiveDigest == "" {
		t.Fatal("missing verified volume archive", burial)
	}
	rows, err = f.svc.Resurrect([]string{"#" + fmt.Sprint(burial.ID)})
	if err != nil || len(rows) != 1 || rows[0].Action != core.ActionResurrected || client.removed {
		t.Fatal(rows, err)
	}
}

func TestDockerVolumeArchiveRetainsRemovalEvidence(t *testing.T) {
	for _, tc := range []struct {
		err   error
		phase string
	}{{io.ErrUnexpectedEOF, ""}, {docker.ErrVolumeEvents, "events-incomplete"}} {
		f, c := volumeFixture(t)
		c.removeErr = tc.err
		rows, err := f.svc.BuryDockerVolumes([]string{"cache"}, "explicit")
		if err != nil || len(rows) != 1 || rows[0].Action != core.ActionRetry {
			t.Fatal(rows, err)
		}
		paths, err := filepath.Glob(filepath.Join(f.svc.Store.Home, "graveyard", "docker-volumes", "*", "volume.tar"))
		if err != nil || len(paths) != 1 {
			t.Fatal(paths, err)
		}
		data, err := os.ReadFile(paths[0] + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var m struct {
			Phase string `json:"volumePhase"`
		}
		if err := json.Unmarshal(data, &m); err != nil || m.Phase != tc.phase {
			t.Fatal(m, err)
		}
	}
}
