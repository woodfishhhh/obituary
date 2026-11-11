package core

import (
	"encoding/json"
	"fmt"
	"slices"
)

// Keep the wire representation local to the ledger. Manifests and callers
// retain self-contained Burial values, including independent parent slices.
type plainState State

// newerStoreError is not corruption: the ledger is intact but belongs to a
// newer release, so callers must not suggest repairing it.
type newerStoreError struct{ version int }

func (e newerStoreError) Error() string {
	return fmt.Sprintf("store was written by a newer Obituary (version %d)", e.version)
}

type ledgerBurial struct {
	Burial
	Parents   json.RawMessage `json:"parents,omitempty"`
	ParentRef *int            `json:"parentRef,omitempty"`
}

type ledgerCleanup struct {
	RestoreCleanup
	Burial ledgerBurial `json:"burial"`
}

type ledgerState struct {
	*plainState
	Burials         []ledgerBurial   `json:"burials,omitempty"`
	RestoreCleanups []ledgerCleanup  `json:"restoreCleanups,omitempty"`
	ParentChains    [][]BurialParent `json:"parentChains,omitempty"`
}

// MarshalJSON keeps legacy fixtures/exported snapshots readable by their
// stated version. Only v15 ledgers replace inline parents with references.
func (st State) MarshalJSON() ([]byte, error) {
	if st.Version < 15 {
		return json.Marshal(plainState(st))
	}
	return marshalLedger(st)
}

func marshalLedger(st State) ([]byte, error) {
	wire := ledgerState{plainState: (*plainState)(&st)}
	chains := make(map[string]int)
	for _, b := range st.Burials {
		row, err := wire.packBurial(b, chains)
		if err != nil {
			return nil, err
		}
		wire.Burials = append(wire.Burials, row)
	}
	for _, c := range st.RestoreCleanups {
		row, err := wire.packBurial(c.Burial, chains)
		if err != nil {
			return nil, err
		}
		wire.RestoreCleanups = append(wire.RestoreCleanups, ledgerCleanup{RestoreCleanup: c, Burial: row})
	}
	return json.Marshal(wire)
}

func (wire *ledgerState) packBurial(b Burial, chains map[string]int) (ledgerBurial, error) {
	row := ledgerBurial{Burial: b}
	if b.Parents == nil {
		return row, nil
	}
	// Full JSON values preserve path spelling and every identity field. No
	// filesystem normalization or hash collision can merge distinct evidence.
	key, err := json.Marshal(b.Parents)
	if err != nil {
		return row, err
	}
	ref, found := chains[string(key)]
	if !found {
		ref = len(wire.ParentChains)
		chains[string(key)] = ref
		wire.ParentChains = append(wire.ParentChains, b.Parents)
	}
	row.ParentRef = &ref
	return row, nil
}

func (st *State) UnmarshalJSON(data []byte) error {
	decoded, err := unmarshalLedger(data)
	if err == nil {
		*st = decoded
	}
	return err
}

func unmarshalLedger(data []byte) (State, error) {
	var decoded plainState
	wire := ledgerState{plainState: &decoded}
	if err := json.Unmarshal(data, &wire); err != nil {
		return State{}, err
	}
	if decoded.Version > stateVersion {
		return State{}, newerStoreError{decoded.Version}
	}
	for _, row := range wire.Burials {
		b, err := wire.unpackBurial(row)
		if err != nil {
			return State{}, err
		}
		decoded.Burials = append(decoded.Burials, b)
	}
	for _, row := range wire.RestoreCleanups {
		b, err := wire.unpackBurial(row.Burial)
		if err != nil {
			return State{}, err
		}
		row.RestoreCleanup.Burial = b
		decoded.RestoreCleanups = append(decoded.RestoreCleanups, row.RestoreCleanup)
	}
	return State(decoded), nil
}

func unpackLegacyBurial(row ledgerBurial) (Burial, error) {
	if row.ParentRef != nil {
		return Burial{}, fmt.Errorf("legacy burial #%d has a parentRef", row.ID)
	}
	if len(row.Parents) == 0 {
		return row.Burial, nil
	}
	err := json.Unmarshal(row.Parents, &row.Burial.Parents)
	return row.Burial, err
}

func (wire ledgerState) unpackBurial(row ledgerBurial) (Burial, error) {
	if wire.Version < 15 {
		return unpackLegacyBurial(row)
	}
	if len(row.Parents) != 0 {
		return Burial{}, fmt.Errorf("v15 burial #%d has inline parents", row.ID)
	}
	if row.ParentRef != nil {
		ref := *row.ParentRef
		if ref < 0 || ref >= len(wire.ParentChains) || wire.ParentChains[ref] == nil {
			return Burial{}, fmt.Errorf("burial #%d has invalid parentRef %d", row.ID, ref)
		}
		row.Burial.Parents = slices.Clone(wire.ParentChains[ref])
	}
	return row.Burial, nil
}
