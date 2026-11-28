package lifecycle

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/extres"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/docker"
)

const KindDockerVolume = ledger.KindDockerVolume

var errDockerVolumeLifespanChanged = errors.New("docker volume lifespan changed during archival")

func (s *Service) volumeProvider() (extres.Provider, error) {
	prov, ok := s.provider(KindDockerVolume)
	if !ok {
		return nil, docker.ErrVolumeUnavailable
	}
	return prov, nil
}

// AddDockerVolume records a volume after re-inspecting it on its pinned daemon.
func (s *Service) AddDockerVolume(names []string, opts AddOptions) ([]Result, error) {
	prov, err := s.volumeProvider()
	if err != nil {
		return nil, err
	}
	if !opts.ExpiresAt.After(s.now()) {
		return nil, errors.New("expiry must be in the future")
	}
	results := make([]Result, len(names))
	for i, name := range names {
		results[i] = Result{Ref: name}
		volume, inspectErr := inspectVolumeViaProvider(s.Store.Context(), prov, name)
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
				return fmt.Errorf("docker volume is buried as #%d; obit resurrect %d", st.Burials[i].ID, st.Burials[i].ID)
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
	prov, err := s.volumeProvider()
	if err != nil {
		return nil, err
	}
	results := make([]GraveResult, len(names))
	for i, name := range names {
		results[i] = GraveResult{Ref: name}
		volume, inspectErr := inspectVolumeViaProvider(s.Store.Context(), prov, name)
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
	if s.recycleMode() && !s.excludeFromRecycleMode(item.Kind) {
		result.Action, result.Error = ActionRefused, ErrUnsupported.Error()
		return result
	}
	prov, err := s.volumeProvider()
	if err != nil {
		result.Action, result.Error = ActionRetry, err.Error()
		return result
	}
	obs, err := prov.Verify(s.Store.Context(), docker.EncodeVolumeCredential(*item.Volume))
	if action, detail, stop := classifyVolumeObservation(obs, err); stop {
		result.Action, result.Error = action, detail
		return result
	}
	if err := s.archiveAndRemoveVolume(prov, item, reason, &result); err != nil {
		result.Action = ActionRetry
		result.Error = err.Error()
	}
	return result
}

func classifyVolumeObservation(obs extres.Observation, err error) (string, string, bool) {
	switch obs.Verdict {
	case extres.VerdictNotFound:
		return ActionVanished, "", true
	case extres.VerdictIdentity:
		detail := obs.Detail
		if detail == "" {
			detail = "Docker volume identity changed"
		}
		return ActionRefused, detail, true
	case extres.VerdictBusy:
		return ActionRefused, obs.Detail, true
	case extres.VerdictUnavailable:
		if err != nil {
			return ActionRetry, err.Error(), true
		}
		return ActionRetry, obs.Detail, true
	}
	if err != nil {
		return ActionRetry, err.Error(), true
	}
	return "", "", false
}

func (s *Service) archiveAndRemoveVolume(prov extres.Provider, item Item, reason string, result *GraveResult) error {
	var manifest string
	hooks := extres.MutateHooks{
		PrepareGraveDir: func(int64) (string, func(), error) {
			dir, cleanup, err := s.prepareVolumeArchiveDir(item.ID)
			if err != nil {
				return "", nil, err
			}
			manifest = filepath.Join(dir, "volume.tar.json")
			return dir, cleanup, nil
		},
		BeforeSend: func(phase string, raw json.RawMessage) error {
			return s.writeVolumePhaseManifest(manifest, item, reason, phase, raw)
		},
	}
	artifact, err := prov.Bury(s.Store.Context(), docker.EncodeVolumeCredential(*item.Volume), hooks)
	if err != nil {
		return err
	}
	b := s.volumeBurialFromArtifact(item, reason, artifact)
	err = s.Store.UpdateGraveyard(func(st *State) error {
		idx := st.Index(item.ID)
		if idx < 0 || st.Items[idx].Kind != KindDockerVolume || st.Items[idx].Volume == nil || !st.Items[idx].Volume.Same(*item.Volume) {
			return errDockerVolumeLifespanChanged
		}
		return entombDockerVolume(st, b, s.now())
	})
	if err != nil {
		if !errors.Is(err, errDockerVolumeLifespanChanged) {
			result.Err = err
		}
		return err
	}
	result.Action, result.Burial, result.Grave = ActionBuried, &b, b.Grave
	if manifest != "" {
		_ = os.Remove(manifest)
	}
	return nil
}

func (s *Service) writeVolumePhaseManifest(manifest string, item Item, reason, phase string, raw json.RawMessage) error {
	if manifest == "" {
		return nil
	}
	var artifact extres.GraveArtifact
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &artifact); err != nil {
			return err
		}
	}
	b := s.volumeBurialFromArtifact(item, reason, artifact)
	switch phase {
	case "archived":
		return writeManifest(manifest, b)
	case "events-incomplete", "removed":
		return writeGraveManifest(manifest, burialManifest{Version: 1, Burial: b, VolumePhase: phase})
	}
	return nil
}

func (s *Service) volumeBurialFromArtifact(item Item, reason string, artifact extres.GraveArtifact) Burial {
	var started time.Time
	if artifact.EventStarted != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, artifact.EventStarted); err == nil {
			started = parsed
		}
	}
	return Burial{
		Kind: KindDockerVolume, Volume: item.Volume, ID: item.ID, Path: item.Path,
		Grave: artifact.Path, Size: artifact.Size, BuriedAt: s.now(), PurgeAt: s.now().Add(s.retention()),
		Reason: reason, Note: item.Note, Source: item.Source, Project: item.Project, Session: item.Session,
		VolumeArchiveDigest: artifact.Digest, VolumeHolderID: artifact.HolderID, VolumeEventStarted: started,
	}
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

func (s *Service) prepareVolumeArchiveDir(id int64) (dir string, cleanup func(), err error) {
	root := filepath.Join(s.Store.Home, "graveyard", "docker-volumes")
	if err = os.MkdirAll(root, 0o700); err != nil {
		return "", nil, err
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		return "", nil, err
	}
	dir = filepath.Join(root, fmt.Sprintf("%d-%s", id, hex.EncodeToString(suffix[:])))
	if err = os.Mkdir(dir, 0o700); err != nil {
		return "", nil, err
	}
	return dir, func() { _ = os.RemoveAll(dir) }, nil
}

// Docker volumes are removed from the file sweep snapshot before graveyard or
// recycle mode gates so both modes share the same archival path.
func (s *Service) sweepDockerVolumes(state *State, dryRun bool, blocked map[int64]bool) ([]Outcome, error) {
	var rows []Outcome
	var files []Item
	var auditErr error
	for _, snapshot := range state.Items {
		if snapshot.Kind != KindDockerVolume {
			files = append(files, snapshot)
			continue
		}
		if blocked[snapshot.ID] || !snapshot.Due(s.now()) {
			continue
		}
		item, ok, err := s.claimDockerVolume(snapshot, dryRun)
		if err != nil {
			return rows, err
		}
		if !ok {
			continue
		}
		out := s.settleDockerVolume(item, dryRun, ReasonExpired)
		rows = append(rows, out)
		if dryRun || out.Action == ActionBuried || out.Action == ActionRefused {
			continue
		}
		if err := s.commit(out); err != nil {
			if commitFailed(err) {
				return rows, err
			}
			auditErr = errors.Join(auditErr, err)
		}
		auditErr = errors.Join(auditErr, s.Store.RecordCommitted(Event{Time: s.now(), Action: out.Action, ID: item.ID, Path: out.Path, Detail: out.Detail}))
	}
	state.Items = files
	return rows, auditErr
}

func (s *Service) claimDockerVolume(snapshot Item, dryRun bool) (Item, bool, error) {
	if dryRun {
		return snapshot, true, nil
	}
	item, ok, err := s.claim(snapshot.ID)
	if err != nil || !ok || item.Kind != KindDockerVolume {
		return Item{}, false, err
	}
	return item, true, nil
}

func (s *Service) settleDockerVolume(item Item, dryRun bool, reason string) Outcome {
	out := Outcome{Item: item, Path: item.Path}
	if dryRun {
		out.Action = ActionBuried
		return out
	}
	result := s.buryDockerVolume(item, reason)
	out.Action, out.Detail, out.Burial, out.Grave, out.Size, out.err = result.Action, result.Error, result.Burial, result.Grave, result.BurialSize(), result.Err
	return out
}

func (r GraveResult) BurialSize() int64 {
	if r.Burial == nil {
		return 0
	}
	return r.Burial.Size
}

func (s *Service) resurrectDockerVolume(result *GraveResult, b Burial) error {
	if b.PurgeIntent != nil {
		result.Error = fmt.Sprintf("#%d: purge is in progress; retry obit purge %d", b.ID, b.ID)
		return nil
	}
	prov, err := s.volumeProvider()
	if err != nil {
		result.Error = err.Error()
		return nil
	}
	if b.Volume == nil {
		result.Error = "grave has no Docker volume identity"
		return nil
	}
	cred := docker.EncodeVolumeCredential(*b.Volume)
	if restorer, ok := prov.(volumeStreamRestorer); ok {
		return s.resurrectRootedDockerVolume(result, restorer, cred, b)
	}
	if err := prov.Resurrect(s.Store.Context(), cred, burialVolumeArtifact(b), extres.MutateHooks{}); err != nil {
		result.Error = err.Error()
		return nil
	}
	if err := s.commitVolumeResurrection(b); err != nil {
		return err
	}
	result.Action, result.Burial, result.Path = ActionResurrected, &b, b.Path
	return nil
}

func (s *Service) resurrectRootedDockerVolume(result *GraveResult, restorer volumeStreamRestorer, cred extres.Credential, b Burial) error {
	if err := restorer.EnsureAbsent(s.Store.Context(), cred); err != nil {
		result.Error = err.Error()
		return nil
	}
	h, err := s.openVolumeArchive(b, false)
	if err != nil {
		result.Error = err.Error()
		return nil
	}
	defer h.Close()
	archive, _, err := openRegularInRoot(h.sub, "volume.tar")
	if err != nil {
		result.Error = err.Error()
		return nil
	}
	restoreErr := restorer.ResurrectFrom(s.Store.Context(), cred, archive, b.VolumeArchiveDigest)
	_ = archive.Close()
	if restoreErr != nil {
		result.Error = restoreErr.Error()
		return nil
	}
	if err := s.commitVolumeResurrection(b); err != nil {
		return err
	}
	result.Action, result.Burial, result.Path = ActionResurrected, &b, b.Path
	_ = removeVolumeArchive(h)
	return nil
}

func (s *Service) commitVolumeResurrection(b Burial) error {
	return s.Store.UpdateGraveyard(func(st *State) error {
		idx := st.BurialIndex(b.ID)
		if idx < 0 {
			return errors.New("docker volume burial disappeared")
		}
		st.Burials = append(st.Burials[:idx], st.Burials[idx+1:]...)
		st.Stats.Resurrected++
		st.Stats.ResurrectedBytes += b.Size
		queueAudit(st, Event{Time: s.now(), Action: ActionResurrected, ID: b.ID, Path: b.Path, Detail: b.Grave})
		return nil
	})
}

func (s *Service) purgeDockerVolume(result *GraveResult, dryRun bool) error {
	b := result.Burial
	if b == nil || b.VolumeArchiveDigest == "" {
		result.Error = "Docker volume archive has no verification digest"
		return nil
	}
	if b.PurgeIntent == nil && s.requireElapsedRetention(b.Kind) && b.PurgeAt.After(s.now()) {
		result.Error = fmt.Sprintf("#%d: volume retention has not elapsed", b.ID)
		return nil
	}
	if prov, ok := s.provider(b.Kind); ok {
		if _, isDockerVol := prov.(*docker.VolumeProvider); !isDockerVol {
			return s.purgeCustomVolumeProvider(result, prov, dryRun)
		}
	}
	return s.purgeRootedDockerVolume(result, dryRun)
}

func (s *Service) purgeCustomVolumeProvider(result *GraveResult, prov extres.Provider, dryRun bool) error {
	if dryRun {
		result.Action = ActionPurged
		return nil
	}
	b := result.Burial
	if b.Volume == nil {
		result.Error = "grave has no Docker volume identity"
		return nil
	}
	if err := s.beginPurge(b, "volume retention elapsed"); err != nil {
		return err
	}
	if err := prov.Purge(s.Store.Context(), docker.EncodeVolumeCredential(*b.Volume), burialVolumeArtifact(*b), extres.MutateHooks{}); err != nil {
		result.Error = err.Error()
		return nil
	}
	result.Action = ActionPurged
	return s.commitPurge(*result)
}

func (s *Service) purgeRootedDockerVolume(result *GraveResult, dryRun bool) error {
	b := result.Burial
	h, err := s.openVolumeArchive(*b, b.PurgeIntent != nil)
	if err != nil {
		result.Error = err.Error()
		return nil
	}
	defer h.Close()
	if err := verifyVolumePurgeArchive(h, *b); err != nil {
		result.Error = err.Error()
		return nil
	}
	if dryRun {
		result.Action = ActionPurged
		return nil
	}
	if err := s.beginPurge(b, "volume retention elapsed"); err != nil {
		return err
	}
	if err := s.removeRootedVolumeArchive(h, *b); err != nil {
		result.Error = err.Error()
		return nil
	}
	result.Action = ActionPurged
	return s.commitPurge(*result)
}

func (s *Service) removeRootedVolumeArchive(h *volumeArchiveHandle, b Burial) error {
	if prov, ok := s.provider(b.Kind); ok && b.Volume != nil {
		hooks := extres.MutateHooks{
			BeforeSend: func(string, json.RawMessage) error { return removeVolumeArchive(h) },
		}
		return prov.Purge(s.Store.Context(), docker.EncodeVolumeCredential(*b.Volume), burialVolumeArtifact(b), hooks)
	}
	return removeVolumeArchive(h)
}

func inspectVolumeViaProvider(ctx context.Context, prov extres.Provider, name string) (docker.Volume, error) {
	cred, err := prov.Inspect(ctx, name)
	if err != nil {
		return docker.Volume{}, err
	}
	return docker.DecodeVolumeCredential(cred)
}
