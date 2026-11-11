package ledger

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
)

// Recognize content as well as the filename. A malformed/foreign file is never
// ours to collect, including partially written snapshots after interruption.
func ownedGraveyardSnapshot(root *os.Root, name, dataID string) bool {
	if dataID == "" || !graveyardSnapshotName(name) {
		return false
	}
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	data, err := root.ReadFile(name)
	if err != nil {
		return false
	}
	var snap graveyardSnapshot
	if json.Unmarshal(data, &snap) != nil || snap.Owner != graveyardSnapshotOwner || snap.Version != stateVersion || snap.DataID != dataID {
		return false
	}
	ref := &GraveyardReference{snap.Generation, "graveyard." + snap.Generation + ".json", fmt.Sprintf("%x", sha256.Sum256(data))}
	if !validGraveyardReference(ref) || name != ref.File && name != ref.File+".tmp" {
		return false
	}
	_, err = snap.state()
	return err == nil
}

var removeGraveyardSnapshot = (*os.Root).Remove

func graveyardSnapshotName(name string) bool {
	name = strings.TrimSuffix(name, ".tmp")
	if !strings.HasPrefix(name, "graveyard.") || !strings.HasSuffix(name, ".json") {
		return false
	}
	return validGraveyardGeneration(strings.TrimSuffix(strings.TrimPrefix(name, "graveyard."), ".json"))
}

// Bound validation attempts, including damaged or foreign files, so orphan
// accumulation cannot monopolize store.lock. Two leaves room for hook writers
// while retiring snapshots faster than one new generation per joint commit.
const snapshotCollectionLimit = 2

// Advance the cursor in the existing hot commit, without another write/flush.
// Sorted cyclic traversal survives new Store instances and does not let an
// unreadable or undeletable early name starve later candidates.
func (s Store) snapshotCollectionBatch(st State) []string {
	root, err := os.OpenRoot(s.Home)
	if err != nil {
		return nil
	}
	defer root.Close()
	dir, err := root.Open(".")
	if err != nil {
		return nil
	}
	names, err := dir.Readdirnames(-1)
	dir.Close()
	if err != nil {
		return nil
	}
	names = slices.DeleteFunc(names, func(name string) bool {
		return !graveyardSnapshotName(name) ||
			st.Graveyard != nil && name == st.Graveyard.File ||
			st.PreviousGraveyard != nil && name == st.PreviousGraveyard.File
	})
	slices.Sort(names)
	start, found := slices.BinarySearch(names, st.GraveyardGCCursor)
	if found {
		start++
	}
	var batch []string
	for i := range min(len(names), snapshotCollectionLimit) {
		batch = append(batch, names[(start+i)%len(names)])
	}
	return batch
}

// The caller owns store.lock exclusively and has confirmed the hot commit.
// Keep its predecessor for unlocked diagnostics; sharing-delete reads finish
// on their open handle, and a reader that has not opened it retries the reference.
func (s Store) collectGraveyardSnapshots(st State, names []string) {
	root, err := os.OpenRoot(s.Home)
	if err != nil {
		return
	}
	defer root.Close()
	for _, name := range names {
		if !ownedGraveyardSnapshot(root, name, st.DataID) {
			continue
		}
		// Failure retains an orphan; it never invalidates the committed state.
		_ = removeGraveyardSnapshot(root, name)
	}
}

// OwnedLedgerFile extends uninstall's allowlist only with validated snapshots
// belonging to its already verified ledger. Migration backups remain evidence.
func OwnedLedgerFile(root *os.Root, name string) bool {
	data, err := root.ReadFile("store.json")
	if err != nil {
		return false
	}
	st, err := unmarshalLedger(data)
	if err != nil {
		return false
	}
	return ownedGraveyardSnapshot(root, name, st.DataID)
}
