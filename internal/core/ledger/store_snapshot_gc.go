package ledger

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
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

// The caller owns store.lock exclusively and has confirmed the hot commit.
// Keep its predecessor for unlocked diagnostics; sharing-delete reads finish
// on their open handle, and a reader that has not opened it retries the reference.
func (s Store) collectGraveyardSnapshots(st State) {
	root, err := os.OpenRoot(s.Home)
	if err != nil {
		return
	}
	defer root.Close()
	dir, err := root.Open(".")
	if err != nil {
		return
	}
	names, err := dir.Readdirnames(-1)
	dir.Close()
	if err != nil {
		return
	}
	for _, name := range names {
		if !graveyardSnapshotName(name) {
			continue
		}
		if st.Graveyard != nil && name == st.Graveyard.File || st.PreviousGraveyard != nil && name == st.PreviousGraveyard.File {
			continue
		}
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
