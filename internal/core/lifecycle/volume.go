package lifecycle

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/docker"
)

const KindDockerVolume = ledger.KindDockerVolume

func (s *Service) volumeClient() (docker.VolumeClient, error) {
	client, ok := s.Docker.(docker.VolumeClient)
	if !ok || client == nil {
		return nil, errors.New("Docker volume operations are unavailable")
	}
	return client, nil
}

// AddDockerVolume records a volume after re-inspecting it on its pinned daemon.
func (s *Service) AddDockerVolume(names []string, opts AddOptions) ([]Result, error) {
	client, err := s.volumeClient()
	if err != nil {
		return nil, err
	}
	if !opts.ExpiresAt.After(s.now()) {
		return nil, errors.New("expiry must be in the future")
	}
	results := make([]Result, len(names))
	for i, name := range names {
		results[i] = Result{Ref: name}
		volume, inspectErr := client.InspectVolume(s.Store.Context(), name)
		if inspectErr != nil {
			results[i].Error, results[i].Err = inspectErr.Error(), inspectErr
			continue
		}
		item, saveErr := s.saveDockerVolumeItem(volume, opts)
		if saveErr != nil {
			return nil, saveErr
		}
		results[i].Action, results[i].Item = ActionAdded, &item
	}
	return results, nil
}

func (s *Service) saveDockerVolumeItem(volume docker.Volume, opts AddOptions) (item Item, err error) {
	err = s.Store.UpdateHot(func(st *State) error {
		for i := range st.Burials {
			if st.Burials[i].Kind == KindDockerVolume && st.Burials[i].Volume != nil && st.Burials[i].Volume.Name == volume.Name && st.Burials[i].Volume.Target == volume.Target {
				return fmt.Errorf("Docker volume is buried as #%d; obit resurrect %d", st.Burials[i].ID, st.Burials[i].ID)
			}
		}
		for i := range st.Items {
			current := &st.Items[i]
			if current.Kind != KindDockerVolume || current.Volume == nil || current.Volume.Name != volume.Name || current.Volume.Target != volume.Target {
				continue
			}
			current.ExpiresAt, current.Note, current.Rev = opts.ExpiresAt, opts.Note, current.Rev+1
			item = *current
			return nil
		}
		item = Item{Kind: KindDockerVolume, Volume: &volume, ID: st.NextID, Rev: 1, Path: volume.Name, CreatedAt: s.now(), ExpiresAt: opts.ExpiresAt, Note: opts.Note, Source: opts.Source}
		if item.Source == "" {
			item.Source = "cli"
		}
		st.NextID++
		st.Items = append(st.Items, item)
		queueAudit(st, Event{Time: s.now(), Action: ActionAdded, ID: item.ID, Path: item.Path, Detail: "Docker volume expires " + item.ExpiresAt.Format(time.RFC3339)})
		return nil
	})
	return item, err
}

// BuryDockerVolumes explicitly archives and removes named volumes.
func (s *Service) BuryDockerVolumes(names []string, note string) ([]GraveResult, error) {
	client, err := s.volumeClient()
	if err != nil {
		return nil, err
	}
	results := make([]GraveResult, len(names))
	for i, name := range names {
		results[i] = GraveResult{Ref: name}
		volume, inspectErr := client.InspectVolume(s.Store.Context(), name)
		if inspectErr != nil {
			results[i].Error = inspectErr.Error()
			continue
		}
		item, saveErr := s.saveDockerVolumeItem(volume, AddOptions{ExpiresAt: s.now().Add(s.retention()), Note: note, Source: "delete-command"})
		if saveErr != nil {
			results[i].Error = saveErr.Error()
			continue
		}
		results[i] = s.buryDockerVolume(item, ReasonBury)
	}
	return results, nil
}

func (s *Service) buryDockerVolume(item Item, reason string) (result GraveResult) {
	result = GraveResult{Ref: item.Path, Path: item.Path}
	if item.Volume == nil {
		result.Action, result.Error = ActionRefused, "record has no Docker volume identity"
		return result
	}
	client, err := s.volumeClient()
	if err != nil {
		result.Action, result.Error = ActionRetry, err.Error()
		return result
	}
	current, err := client.InspectVolume(s.Store.Context(), item.Volume.Name)
	if errors.Is(err, docker.ErrNoVolume) {
		result.Action = ActionVanished
		return result
	}
	if err != nil {
		result.Action, result.Error = ActionRetry, err.Error()
		return result
	}
	if !current.Same(*item.Volume) {
		result.Action, result.Error = ActionRefused, "Docker volume identity changed"
		return result
	}
	refs, err := client.ListVolumeReferences(s.Store.Context(), current)
	if err != nil {
		result.Action, result.Error = ActionRetry, err.Error()
		return result
	}
	if len(refs) != 0 {
		result.Action, result.Error = ActionRefused, fmt.Sprintf("Docker volume is referenced by %s", refs[0].Display())
		return result
	}
	if err := s.archiveAndRemoveVolume(client, item, reason, &result); err != nil {
		result.Action = ActionRetry
		result.Error = err.Error()
	}
	return result
}

func (s *Service) archiveAndRemoveVolume(client docker.VolumeClient, item Item, reason string, result *GraveResult) error {
	archive, manifest, cleanup, err := s.prepareVolumeArchive(item.ID)
	if err != nil {
		return err
	}
	releaseHolder := true
	defer func() {
		if cleanup != nil {
			cleanup()
		}
		if releaseHolder && item.Volume != nil && item.Volume.HolderID != "" {
			_ = client.ReleaseVolumeHolder(s.Store.Context(), item.Volume)
		}
	}()
	partial := archive + ".partial"
	f, err := os.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, streamErr := client.ArchiveVolume(s.Store.Context(), item.Volume, f)
	closeErr := f.Close()
	if streamErr != nil || closeErr != nil {
		_ = client.ReleaseVolumeHolder(s.Store.Context(), item.Volume)
		return errors.Join(streamErr, closeErr)
	}
	check, err := verifyVolumeArchiveFile(partial)
	if err != nil {
		_ = client.ReleaseVolumeHolder(s.Store.Context(), item.Volume)
		return err
	}
	if err := os.Rename(partial, archive); err != nil {
		_ = client.ReleaseVolumeHolder(s.Store.Context(), item.Volume)
		return err
	}
	b := Burial{Kind: KindDockerVolume, Volume: item.Volume, ID: item.ID, Path: item.Path, Grave: archive, Size: check.Bytes, BuriedAt: s.now(), PurgeAt: s.now().Add(s.retention()), Reason: reason, Note: item.Note, Source: item.Source, Project: item.Project, Session: item.Session, VolumeArchiveDigest: check.Digest, VolumeHolderID: item.Volume.HolderID}
	if err := writeManifest(manifest, b); err != nil {
		return err
	}
	// From this point the archive and manifest are recovery evidence; keep them on any later failure.
	cleanup = nil
	releaseHolder = false
	if err := client.RemoveVolume(s.Store.Context(), item.Volume); err != nil {
		return err
	}
	err = s.Store.UpdateGraveyard(func(st *State) error {
		idx := st.Index(item.ID)
		if idx < 0 || st.Items[idx].Kind != KindDockerVolume || st.Items[idx].Volume == nil || !st.Items[idx].Volume.Same(*item.Volume) {
			return errors.New("Docker volume lifespan changed during archival")
		}
		if err := entombDockerVolume(st, b, s.now()); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	result.Action, result.Burial, result.Grave = ActionBuried, &b, b.Grave
	_ = os.Remove(manifest)
	return nil
}

func entombDockerVolume(st *State, b Burial, at time.Time) error {
	if st.BurialIndex(b.ID) >= 0 {
		return fmt.Errorf("#%d is already in use", b.ID)
	}
	idx := st.Index(b.ID)
	if idx < 0 {
		return fmt.Errorf("#%d is no longer tracked", b.ID)
	}
	st.Burials = append(st.Burials, b)
	st.Items = append(st.Items[:idx], st.Items[idx+1:]...)
	st.Stats.Buried++
	st.Stats.BuriedBytes += b.Size
	queueAudit(st, Event{Time: at, Action: ActionBuried, ID: b.ID, Path: b.Path, Detail: b.Grave})
	return nil
}

func (s *Service) prepareVolumeArchive(id int64) (archive, manifest string, cleanup func(), err error) {
	root := filepath.Join(s.Store.Home, "graveyard", "docker-volumes")
	if err = os.MkdirAll(root, 0700); err != nil {
		return "", "", nil, err
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		return "", "", nil, err
	}
	dir := filepath.Join(root, fmt.Sprintf("%d-%s", id, hex.EncodeToString(suffix[:])))
	if err = os.Mkdir(dir, 0700); err != nil {
		return "", "", nil, err
	}
	archive = filepath.Join(dir, "volume.tar")
	manifest = archive + ".json"
	cleanup = func() { _ = os.RemoveAll(dir) }
	return archive, manifest, cleanup, nil
}

func verifyVolumeArchiveFile(path string) (docker.VolumeArchive, error) {
	f, err := os.Open(path)
	if err != nil {
		return docker.VolumeArchive{}, err
	}
	defer f.Close()
	return docker.VerifyVolumeArchive(f)
}

func (s *Service) settleDockerVolume(item Item, dryRun bool, reason string) Outcome {
	out := Outcome{Item: item, Path: item.Path}
	if dryRun {
		out.Action = ActionBuried
		return out
	}
	result := s.buryDockerVolume(item, reason)
	out.Action, out.Detail, out.Burial, out.Grave, out.Size = result.Action, result.Error, result.Burial, result.Grave, result.BurialSize()
	if result.Error != "" {
		out.err = errors.New(result.Error)
	}
	return out
}

func (r GraveResult) BurialSize() int64 {
	if r.Burial == nil {
		return 0
	}
	return r.Burial.Size
}

func (s *Service) resurrectDockerVolume(result *GraveResult, b Burial) error {
	client, err := s.volumeClient()
	if err != nil {
		result.Error = err.Error()
		return nil
	}
	if b.Volume == nil {
		result.Error = "grave has no Docker volume identity"
		return nil
	}
	if _, err := client.InspectVolume(s.Store.Context(), b.Volume.Name); err == nil {
		result.Error = "a Docker volume with the original name already exists; nothing was written"
		return nil
	} else if !errors.Is(err, docker.ErrNoVolume) {
		result.Error = err.Error()
		return nil
	}
	archive, err := os.Open(b.Grave)
	if err != nil {
		result.Error = err.Error()
		return nil
	}
	defer archive.Close()
	if check, err := docker.VerifyVolumeArchive(archive); err != nil || check.Digest != b.VolumeArchiveDigest {
		result.Error = "Docker volume archive failed verification"
		return nil
	}
	if _, err := client.CreateVolume(s.Store.Context(), *b.Volume, randomToken()); err != nil {
		result.Error = err.Error()
		return nil
	}
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		result.Error = err.Error()
		return nil
	}
	if err := client.RestoreVolume(s.Store.Context(), *b.Volume, archive); err != nil {
		result.Error = err.Error()
		return nil
	}
	if err := s.Store.UpdateGraveyard(func(st *State) error {
		idx := st.BurialIndex(b.ID)
		if idx < 0 {
			return errors.New("Docker volume burial disappeared")
		}
		st.Burials = append(st.Burials[:idx], st.Burials[idx+1:]...)
		st.Stats.Resurrected++
		st.Stats.ResurrectedBytes += b.Size
		queueAudit(st, Event{Time: s.now(), Action: ActionResurrected, ID: b.ID, Path: b.Path, Detail: b.Grave})
		return nil
	}); err != nil {
		return err
	}
	result.Action, result.Burial, result.Path = ActionResurrected, &b, b.Path
	_ = os.Remove(b.Grave + ".json")
	_ = os.Remove(b.Grave)
	return nil
}

func randomToken() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return hex.EncodeToString(raw[:])
	}
	return hex.EncodeToString(raw[:])
}

func (s *Service) purgeDockerVolume(result *GraveResult, dryRun bool) error {
	b := result.Burial
	if b == nil || b.VolumeArchiveDigest == "" {
		result.Error = "Docker volume archive has no verification digest"
		return nil
	}
	if b.PurgeAt.After(s.now()) {
		result.Error = fmt.Sprintf("#%d: volume retention has not elapsed", b.ID)
		return nil
	}
	if _, err := verifyVolumeArchiveFile(b.Grave); err != nil {
		result.Error = "refusing to purge unverified Docker volume archive: " + err.Error()
		return nil
	}
	if dryRun {
		result.Action = ActionPurged
		return nil
	}
	if err := s.beginPurge(b, "volume retention elapsed"); err != nil {
		return err
	}
	if err := os.Remove(b.Grave); err != nil && !errors.Is(err, os.ErrNotExist) {
		result.Error = err.Error()
		return nil
	}
	_ = os.Remove(b.Grave + ".json")
	_ = os.Remove(filepath.Dir(b.Grave))
	result.Action = ActionPurged
	return s.commitPurge(*result)
}
