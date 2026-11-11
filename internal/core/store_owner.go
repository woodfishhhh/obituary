package core

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const DataOwnerFile = ".obituary-data"

type dataOwner struct {
	Owner     string `json:"owner"`
	DataID    string `json:"dataId"`
	Directory string `json:"directory"`
}

// A successful ledger write establishes provenance. Inspect/uninstall never
// manufacture evidence for a directory that happens to contain log names.
func (s Store) prepareDataOwner(state *State) error {
	if state.DataID != "" {
		return nil
	}
	root, err := os.OpenRoot(s.Home)
	if err != nil {
		return err
	}
	defer root.Close()
	identity, err := dataDirectoryID(root)
	if err != nil {
		return err
	}
	data, err := root.ReadFile(DataOwnerFile)
	if err == nil {
		var owner dataOwner
		if json.Unmarshal(data, &owner) != nil || owner.Owner != "obituary-data-v1" || owner.Directory != identity || owner.DataID == "" {
			// A copied, restored or moved data directory keeps working; it
			// only loses the provenance --purge-data needs, so purge refuses.
			return nil
		}
		state.DataID = owner.DataID // recover a marker written before a failed ledger commit
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		// The marker only serves uninstall; never block ledger writes on it.
		return nil
	}
	id := rand.Text()
	data, err = json.Marshal(dataOwner{Owner: "obituary-data-v1", DataID: id, Directory: identity})
	if err != nil {
		return err
	}
	// The marker only serves --purge-data, so it never blocks the ledger
	// write. A marker that was renamed into place but not confirmed on disk
	// is still the visible one; any other failure leaves the ledger
	// unmarked and the next save tries again.
	var unconfirmed *unconfirmedCommit
	if err := writeDurably(filepath.Join(s.Home, DataOwnerFile), append(data, '\n')); err == nil || errors.As(err, &unconfirmed) {
		state.DataID = id
	}
	return nil
}

// VerifyDataOwner checks both the pinned directory and its matching ledger.
// Older unmarked data remains usable, but cannot be automatically removed.
func VerifyDataOwner(root *os.Root) error {
	marker, err := root.Lstat(DataOwnerFile)
	if err != nil {
		return fmt.Errorf("data retained: ownership marker unavailable: %w", err)
	}
	ledger, err := root.Lstat("store.json")
	if err != nil {
		return fmt.Errorf("data retained: ledger unavailable: %w", err)
	}
	if !marker.Mode().IsRegular() || !ledger.Mode().IsRegular() {
		return errors.New("data retained: ownership evidence is not regular files")
	}
	data, err := root.ReadFile(DataOwnerFile)
	if err != nil {
		return err
	}
	var owner dataOwner
	if err := json.Unmarshal(data, &owner); err != nil {
		return err
	}
	data, err = root.ReadFile("store.json")
	if err != nil {
		return err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	if state.Version >= 17 {
		if err := validateHotLedger(state); err != nil {
			return err
		}
		snapshot, err := root.ReadFile(state.Graveyard.File)
		if err != nil {
			return fmt.Errorf("data retained: referenced graveyard unavailable: %w", err)
		}
		if _, err := decodeGraveyard(snapshot, state); err != nil {
			return err
		}
	}
	identity, err := dataDirectoryID(root)
	if err != nil {
		return err
	}
	if owner.Owner != "obituary-data-v1" || owner.Directory != identity || owner.DataID == "" || owner.DataID != state.DataID || state.Version < 1 || state.Version > stateVersion || state.NextID < 1 {
		return errors.New("data retained: directory marker and ledger ownership do not match")
	}
	return nil
}
