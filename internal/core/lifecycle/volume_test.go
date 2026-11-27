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
	"strings"
	"testing"
	"time"

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

func makeVolumeTar(t *testing.T, body string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := tar.NewWriter(&buf)
	if err := w.WriteHeader(&tar.Header{Name: "./", Typeflag: tar.TypeDir, Mode: 0700}); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteHeader(&tar.Header{Name: "./data", Typeflag: tar.TypeReg, Mode: 0600, Size: int64(len(body))}); err != nil {
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

func TestDockerVolumeExpirySweepAndTickPurgeInGraveyardMode(t *testing.T) {
	f, client := volumeFixture(t)
	if f.svc.Mode != "" {
		t.Fatalf("expected default graveyard mode, got %q", f.svc.Mode)
	}
	filePath := f.file("alongside.txt")
	if rows, err := f.svc.Add([]string{filePath}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)}); err != nil || core.Failed(rows) {
		t.Fatalf("add file: %+v %v", rows, err)
	}
	added, err := f.svc.AddDockerVolume([]string{"cache"}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err != nil || core.Failed(added) || len(added) != 1 || added[0].Item == nil {
		t.Fatalf("add volume: %+v %v", added, err)
	}
	volID := added[0].Item.ID

	// Before expiry: nothing is buried.
	if rows, err := f.svc.Sweep(false); err != nil || len(rows) != 0 || client.removed {
		t.Fatalf("pre-expiry sweep mutated state: %+v removed=%v err=%v", rows, client.removed, err)
	}

	// Past expiry: dry-run reports burial without removing volume.
	f.clock = f.clock.Add(2 * time.Hour)
	dryRows, err := f.svc.Sweep(true)
	if err != nil || len(dryRows) != 2 || client.removed {
		t.Fatalf("dry-run sweep: %+v removed=%v err=%v", dryRows, client.removed, err)
	}

	// Real sweep in default graveyard mode settles the volume via settleDockerVolume
	// rather than falling into file prepareSettlement (ActionRefused).
	rows, err := f.svc.Sweep(false)
	if err != nil {
		t.Fatalf("sweep failed: %v", err)
	}
	var volBurial *core.Burial
	for _, out := range rows {
		if out.Item.ID == volID {
			if out.Action != core.ActionBuried || out.Burial == nil || out.Burial.Reason != core.ReasonExpired {
				t.Fatalf("volume sweep outcome = %+v, want ActionBuried/ReasonExpired", out)
			}
			volBurial = out.Burial
		}
	}
	if volBurial == nil || !client.removed || volBurial.VolumeArchiveDigest == "" {
		t.Fatalf("volume was not archived and removed: burial=%+v removed=%v rows=%+v", volBurial, client.removed, rows)
	}
	if _, err := os.Stat(volBurial.Grave); err != nil {
		t.Fatalf("volume archive missing on disk: %v", err)
	}

	// Retention not elapsed: explicit purge, purge --all, and Tick all keep the archive.
	f.clock = f.clock.Add(3 * 24 * time.Hour)
	refPurge, err := f.svc.PurgeRefs([]string{fmt.Sprintf("#%d", volID)}, false)
	if err != nil || len(refPurge) != 1 || !strings.Contains(refPurge[0].Error, "volume retention has not elapsed") {
		t.Fatalf("expected retention refusal on PurgeRefs: %+v %v", refPurge, err)
	}
	allPurge, err := f.svc.Purge(true, false)
	if err != nil {
		t.Fatalf("purge --all: %v", err)
	}
	foundRefusal := false
	for _, r := range allPurge {
		if r.Burial != nil && r.Burial.ID == volID && strings.Contains(r.Error, "volume retention has not elapsed") {
			foundRefusal = true
		}
	}
	if !foundRefusal {
		t.Fatalf("purge --all bypassed volume retention: %+v", allPurge)
	}
	if _, err := os.Stat(volBurial.Grave); err != nil {
		t.Fatalf("archive deleted before retention elapsed: %v", err)
	}

	// Advance past retention: Tick runs background purge and removes the volume archive.
	f.clock = f.clock.Add(5 * 24 * time.Hour)
	_, purged, err := f.svc.Tick()
	if err != nil {
		t.Fatalf("tick purge failed: %v", err)
	}
	volPurged := false
	for _, r := range purged {
		if r.Burial != nil && r.Burial.ID == volID && r.Action == core.ActionPurged && r.Error == "" {
			volPurged = true
		}
	}
	if !volPurged {
		t.Fatalf("tick did not purge expired volume burial: %+v", purged)
	}
	if _, err := os.Stat(volBurial.Grave); !os.IsNotExist(err) {
		t.Fatalf("volume archive survived purge: %v", err)
	}
	st, err := f.svc.Store.Read()
	if err != nil || len(st.Burials) != 0 || st.Stats.Purged < 1 {
		t.Fatalf("post-purge state unexpected: burials=%+v stats=%+v err=%v", st.Burials, st.Stats, err)
	}
}

func TestDockerVolumePurgeRefusesIncompleteOrTamperedEvidence(t *testing.T) {
	t.Run("digest-mismatch", func(t *testing.T) {
		f, _ := volumeFixture(t)
		rows, err := f.svc.BuryDockerVolumes([]string{"cache"}, "explicit")
		if err != nil || len(rows) != 1 || rows[0].Action != core.ActionBuried {
			t.Fatalf("bury: %+v %v", rows, err)
		}
		b := rows[0].Burial
		if err := os.WriteFile(b.Grave, makeVolumeTar(t, "tampered"), 0600); err != nil {
			t.Fatal(err)
		}
		f.clock = f.clock.Add(8 * 24 * time.Hour)
		_, purged, err := f.svc.Tick()
		if err != nil || len(purged) != 1 || !strings.Contains(purged[0].Error, "digest mismatch") {
			t.Fatalf("tampered archive was not refused: %+v %v", purged, err)
		}
		if _, err := os.Stat(b.Grave); err != nil {
			t.Fatalf("tampered archive was deleted: %v", err)
		}
	})

	t.Run("events-incomplete-manifest", func(t *testing.T) {
		f, _ := volumeFixture(t)
		rows, err := f.svc.BuryDockerVolumes([]string{"cache"}, "explicit")
		if err != nil || len(rows) != 1 || rows[0].Action != core.ActionBuried {
			t.Fatalf("bury: %+v %v", rows, err)
		}
		b := rows[0].Burial
		if err := os.WriteFile(b.Grave+".json", []byte(`{"version":1,"volumePhase":"events-incomplete"}`), 0600); err != nil {
			t.Fatal(err)
		}
		f.clock = f.clock.Add(8 * 24 * time.Hour)
		_, purged, err := f.svc.Tick()
		if err != nil || len(purged) != 1 || !strings.Contains(purged[0].Error, "incomplete evidence") {
			t.Fatalf("incomplete manifest was not refused: %+v %v", purged, err)
		}
		if _, err := os.Stat(b.Grave); err != nil {
			t.Fatalf("archive with incomplete evidence was deleted: %v", err)
		}
	})

	t.Run("missing-archive-without-intent", func(t *testing.T) {
		f, _ := volumeFixture(t)
		rows, err := f.svc.BuryDockerVolumes([]string{"cache"}, "explicit")
		if err != nil || len(rows) != 1 || rows[0].Action != core.ActionBuried {
			t.Fatalf("bury: %+v %v", rows, err)
		}
		b := rows[0].Burial
		if err := os.Remove(b.Grave); err != nil {
			t.Fatal(err)
		}
		f.clock = f.clock.Add(8 * 24 * time.Hour)
		_, purged, err := f.svc.Tick()
		if err != nil || len(purged) != 1 || !strings.Contains(purged[0].Error, "unverified Docker volume archive") {
			t.Fatalf("missing archive was not refused: %+v %v", purged, err)
		}
		st, err := f.svc.Store.Read()
		if err != nil || len(st.Burials) != 1 {
			t.Fatalf("burial should remain after refusal: %+v %v", st.Burials, err)
		}
	})
}

func TestDockerVolumeSweepOutcomesAndFileIsolation(t *testing.T) {
	t.Run("referenced-retained-and-file-paths-isolated", func(t *testing.T) {
		f, client := volumeFixture(t)
		client.refs = []docker.Container{{Identity: docker.Identity{ID: strings.Repeat("a", 64), Name: "holder"}}}
		added, err := f.svc.AddDockerVolume([]string{"cache"}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
		if err != nil || core.Failed(added) {
			t.Fatalf("add volume: %+v %v", added, err)
		}

		// File registration with OnlyNew (RegistrationScope), diagnostics,
		// SettleVanished, and directory burial must ignore the tracked volume.
		volID := added[0].Item.ID
		filePath := f.file("isolated.txt")
		if rows, err := f.svc.Add([]string{filePath}, core.AddOptions{ExpiresAt: f.clock.Add(2 * time.Hour), OnlyNew: true}); err != nil || core.Failed(rows) {
			t.Fatalf("tracked volume broke OnlyNew file add: %+v %v", rows, err)
		}
		events, err := f.svc.RegistrationLocationDiagnostics()
		if err != nil {
			t.Fatalf("RegistrationLocationDiagnostics: %v", err)
		}
		for _, ev := range events {
			if ev.ID == volID {
				t.Fatalf("tracked volume produced bogus identity diagnostic: %+v", ev)
			}
		}
		if vanished, err := f.svc.SettleVanished(); err != nil || len(vanished) != 0 {
			t.Fatalf("SettleVanished touched volume: %+v %v", vanished, err)
		}
		dir := filepath.Join(f.dir, "ordinary-folder")
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if rows, err := f.svc.Bury([]string{dir}, false, ""); err != nil || core.GraveFailed(rows) || rows[0].Action != core.ActionBuried {
			t.Fatalf("tracked volume blocked directory burial: %+v %v", rows, err)
		}

		// Referenced volume at expiry is refused and retained in items.
		f.clock = f.clock.Add(90 * time.Minute)
		rows, err := f.svc.Sweep(false)
		if err != nil || len(rows) != 1 || rows[0].Action != core.ActionRefused || client.removed {
			t.Fatalf("referenced volume sweep = %+v removed=%v err=%v", rows, client.removed, err)
		}
		st, err := f.svc.Store.Read()
		if err != nil || len(st.Items) != 2 {
			t.Fatalf("refused volume item was dropped: %+v %v", st.Items, err)
		}
	})

	t.Run("vanished-volume-settled", func(t *testing.T) {
		f, client := volumeFixture(t)
		if added, err := f.svc.AddDockerVolume([]string{"cache"}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)}); err != nil || core.Failed(added) {
			t.Fatalf("add volume: %+v %v", added, err)
		}
		client.removed = true
		f.clock = f.clock.Add(2 * time.Hour)
		rows, err := f.svc.Sweep(false)
		if err != nil || len(rows) != 1 || rows[0].Action != core.ActionVanished {
			t.Fatalf("vanished volume sweep = %+v err=%v", rows, err)
		}
		st, err := f.svc.Store.Read()
		if err != nil || len(st.Items) != 0 {
			t.Fatalf("vanished volume item remained: %+v %v", st.Items, err)
		}
	})

	t.Run("existing-proposals-retains-external-candidates", func(t *testing.T) {
		in := []core.Proposal{{
			ID:      1,
			Kind:    "volume",
			Project: "project",
			Candidates: []core.Candidate{
				{Kind: core.KindDockerVolume, Path: "nonexistent-volume-name", Lifespan: "1d"},
			},
		}}
		got := core.ExistingProposals(in)
		if len(got) != 1 || len(got[0].Candidates) != 1 {
			t.Fatalf("ExistingProposals filtered out external candidate: %+v", got)
		}
	})
}
