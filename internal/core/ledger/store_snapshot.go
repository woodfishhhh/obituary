package ledger

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
)

// GraveyardReference binds one immutable snapshot to the hot ledger commit.
type GraveyardReference struct {
	Generation string `json:"generation"`
	File       string `json:"file"`
	Hash       string `json:"hash"`
}

type graveyardSnapshot struct {
	Owner           string             `json:"owner"`
	Version         int                `json:"version"`
	DataID          string             `json:"dataId"`
	Generation      string             `json:"generation"`
	Burials         []ledgerBurial     `json:"burials,omitempty"`
	RestoreCleanups []ledgerCleanup    `json:"restoreCleanups,omitempty"`
	ParentChains    [][]BurialParent   `json:"parentChains,omitempty"`
	Operations      []*BurialOperation `json:"operations,omitempty"`
	Volumes         []*ledgerVolume    `json:"volumes,omitempty"`
	Origins         []*ledgerOrigin    `json:"origins,omitempty"`
}

const graveyardSnapshotOwner = "obituary-graveyard-v1"

var readGraveyardSnapshot = readShared

func copyGraveyardReference(ref *GraveyardReference) *GraveyardReference {
	if ref == nil {
		return nil
	}
	copy := *ref
	return &copy
}

func marshalGraveyard(st State, generation string) ([]byte, error) {
	wire, err := packLedger(st)
	if err != nil {
		return nil, err
	}
	return json.Marshal(graveyardSnapshot{
		Owner: graveyardSnapshotOwner, Version: stateVersion, DataID: st.DataID, Generation: generation,
		Burials: wire.Burials, RestoreCleanups: wire.RestoreCleanups, ParentChains: wire.ParentChains,
		Operations: wire.Operations, Volumes: wire.Volumes, Origins: wire.Origins,
	})
}

func (snap graveyardSnapshot) state() (State, error) {
	return unpackLedger(ledgerState{plainState: &plainState{Version: snap.Version, DataID: snap.DataID},
		Burials: snap.Burials, RestoreCleanups: snap.RestoreCleanups, ParentChains: snap.ParentChains,
		Operations: snap.Operations, Volumes: snap.Volumes, Origins: snap.Origins})
}

func validGraveyardReference(ref *GraveyardReference) bool {
	return ref != nil && validGraveyardGeneration(ref.Generation) &&
		ref.File == "graveyard."+ref.Generation+".json" && len(ref.Hash) == 64
}

func validGraveyardGeneration(generation string) bool {
	if len(generation) != 26 {
		return false
	}
	for _, r := range generation {
		if (r < 'A' || r > 'Z') && (r < '2' || r > '7') {
			return false
		}
	}
	return true
}

func decodeGraveyard(data []byte, st State) (State, error) {
	ref := st.Graveyard
	if !validGraveyardReference(ref) || fmt.Sprintf("%x", sha256.Sum256(data)) != ref.Hash {
		return State{}, errors.New("graveyard snapshot hash or reference mismatch")
	}
	var snap graveyardSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return State{}, err
	}
	if snap.Owner != graveyardSnapshotOwner || snap.Version != stateVersion || snap.DataID == "" || snap.DataID != st.DataID || snap.Generation != ref.Generation {
		return State{}, errors.New("graveyard snapshot version, data owner or generation mismatch")
	}
	return snap.state()
}

func (s Store) loadGraveyard(st *State) error {
	if st.graveyardLoaded || st.Version < 17 || st.Graveyard == nil {
		st.graveyardLoaded = true
		if st.graveyardOriginal == [32]byte{} {
			var err error
			st.graveyardOriginal, err = graveyardFingerprint(*st)
			return err
		}
		return nil
	}
	data, err := readGraveyardSnapshot(filepath.Join(s.Home, st.Graveyard.File))
	if err != nil {
		return fmt.Errorf("read referenced graveyard snapshot: %w", err)
	}
	cold, err := decodeGraveyard(data, *st)
	if err != nil {
		return fmt.Errorf("invalid referenced graveyard snapshot: %w", err)
	}
	next := *st
	next.Burials, next.RestoreCleanups = cold.Burials, cold.RestoreCleanups
	if !reflect.DeepEqual(next.GraveyardIndex, buildGraveyardIndex(next)) {
		return errors.New("graveyard safety index does not match committed snapshot")
	}
	next.graveyardOriginal = sha256.Sum256(data)
	if normalizeSnapshotIdentities(&next) {
		next.graveyardOriginal, err = graveyardFingerprint(next)
	}
	if err != nil {
		return err
	}
	next.graveyardLoaded = true
	*st = next
	return nil
}

// A lazy hot-transaction read must expose the same identities as a full read.
// Keep its mutation baseline after normalization, so spelling alone neither
// bypasses identity checks nor looks like a callback changed cold evidence.
func normalizeSnapshotIdentities(st *State) bool {
	changed := false
	for _, b := range st.Burials {
		for _, id := range []*Identity{b.Identity, b.GraveID} {
			if id != nil {
				normalized := id.Normalized()
				changed = changed || normalized != *id
				*id = normalized
			}
		}
	}
	return changed
}

func marshalHot(st State) ([]byte, error) {
	st.Burials, st.RestoreCleanups = nil, nil
	return json.Marshal(plainState(st))
}

// Snapshot bytes are published with an exclusive link: no existing name can
// be replaced. Only store.json's later replacement makes them authoritative.
// os.Root supplies confined publication on both Windows and Unix.
func (s Store) writeGraveyard(st *State, migrating bool) error {
	generation := rand.Text()
	data, err := marshalGraveyard(*st, generation)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	ref := &GraveyardReference{generation, "graveyard." + generation + ".json", fmt.Sprintf("%x", sha256.Sum256(data))}
	if migrating {
		if err := verifyMigratedGraveyard(*st, ref, data); err != nil {
			return err
		}
	}
	if err := s.publishGraveyard(ref.File, data); err != nil {
		return err
	}
	st.PreviousGraveyard, st.Graveyard = st.Graveyard, ref
	st.GraveyardIndex = buildGraveyardIndex(*st)
	return nil
}

func verifyMigratedGraveyard(st State, ref *GraveyardReference, data []byte) error {
	st.Graveyard = ref
	decoded, err := decodeGraveyard(data, st)
	if err != nil {
		return err
	}
	checked, err := marshalGraveyard(decoded, ref.Generation)
	if err != nil || !bytes.Equal(checked, bytes.TrimSuffix(data, []byte{'\n'})) {
		return errors.New("graveyard snapshot changed recovery evidence")
	}
	return nil
}

var publishGraveyardLink = (*os.Root).Link

func (s Store) publishGraveyard(name string, data []byte) (err error) {
	root, err := os.OpenRoot(s.Home)
	if err != nil {
		return err
	}
	defer root.Close()
	tmp := name + ".tmp"
	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	if _, err := writeStoreBytes(f, data); err != nil {
		return err
	}
	if err := syncFile(f); err != nil {
		return err
	}
	if err := publishGraveyardLink(root, tmp, name); err != nil {
		return err
	}
	if err := syncReplacement(f); err != nil {
		return fmt.Errorf("confirm graveyard snapshot publication: %w", err)
	}
	if err := syncParentDirectory(root); err != nil {
		return err
	}
	return root.Remove(tmp)
}

func (s Store) confirmGraveyard(st State) error {
	if st.Graveyard == nil {
		return nil
	}
	f, err := os.OpenFile(filepath.Join(s.Home, st.Graveyard.File), os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	err = syncReplacement(f)
	return errors.Join(err, f.Close())
}

func graveyardChanged(st State) (bool, error) {
	digest, err := graveyardFingerprint(st)
	return st.Graveyard == nil || digest != st.graveyardOriginal, err
}

func graveyardFingerprint(st State) ([32]byte, error) {
	generation := ""
	if st.Graveyard != nil {
		generation = st.Graveyard.Generation
	}
	data, err := marshalGraveyard(st, generation)
	return sha256.Sum256(append(data, '\n')), err
}
