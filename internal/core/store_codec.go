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
	Parents      json.RawMessage `json:"parents,omitempty"`
	ParentRef    *int            `json:"parentRef,omitempty"`
	Operation    json.RawMessage `json:"operation,omitempty"`
	OperationRef *int            `json:"operationRef,omitempty"`
	Identity     *ledgerIdentity `json:"identity,omitempty"`
	GraveID      *ledgerIdentity `json:"graveId,omitempty"`
	Project      json.RawMessage `json:"project,omitempty"`
	Session      json.RawMessage `json:"session,omitempty"`
	OriginRef    *int            `json:"originRef,omitempty"`
}

type ledgerVolume struct {
	Volume string `json:"volume"`
	Serial uint64 `json:"serial"`
}

type ledgerIdentity struct {
	Volume    json.RawMessage `json:"volume,omitempty"`
	Serial    json.RawMessage `json:"serial,omitempty"`
	FileID    string          `json:"fileId"`
	VolumeRef *int            `json:"volumeRef,omitempty"`
}

type ledgerOrigin struct {
	Project string `json:"project,omitempty"`
	Session string `json:"session,omitempty"`
}

type ledgerCleanup struct {
	RestoreCleanup
	Burial ledgerBurial `json:"burial"`
}

type ledgerState struct {
	*plainState
	Burials         []ledgerBurial     `json:"burials,omitempty"`
	RestoreCleanups []ledgerCleanup    `json:"restoreCleanups,omitempty"`
	ParentChains    [][]BurialParent   `json:"parentChains,omitempty"`
	Operations      []*BurialOperation `json:"operations,omitempty"`
	Volumes         []*ledgerVolume    `json:"volumes,omitempty"`
	Origins         []*ledgerOrigin    `json:"origins,omitempty"`
}

type ledgerTables struct {
	chains     map[string]*int
	operations map[string]*int
	volumes    map[ledgerVolume]*int
	origins    map[ledgerOrigin]*int
}

// MarshalJSON keeps legacy fixtures/exported snapshots readable by their
// stated version. v15 interns parents; v16 also interns operations and metadata.
func (st State) MarshalJSON() ([]byte, error) {
	if st.Version < 17 {
		st.Graveyard, st.PreviousGraveyard, st.GraveyardIndex = nil, nil, nil
	}
	if st.Version < 15 {
		return json.Marshal(plainState(st))
	}
	return marshalLedger(st)
}

func marshalLedger(st State) ([]byte, error) {
	wire, err := packLedger(st)
	if err != nil {
		return nil, err
	}
	return json.Marshal(wire)
}

func packLedger(st State) (ledgerState, error) {
	wire := ledgerState{plainState: (*plainState)(&st),
		Burials: make([]ledgerBurial, 0, len(st.Burials)), RestoreCleanups: make([]ledgerCleanup, 0, len(st.RestoreCleanups))}
	tables := ledgerTables{chains: make(map[string]*int), operations: make(map[string]*int),
		volumes: make(map[ledgerVolume]*int), origins: make(map[ledgerOrigin]*int)}
	for _, b := range st.Burials {
		row, err := wire.packBurial(b, tables)
		if err != nil {
			return wire, err
		}
		wire.Burials = append(wire.Burials, row)
	}
	for _, c := range st.RestoreCleanups {
		row, err := wire.packBurial(c.Burial, tables)
		if err != nil {
			return wire, err
		}
		wire.RestoreCleanups = append(wire.RestoreCleanups, ledgerCleanup{RestoreCleanup: c, Burial: row})
	}
	return wire, nil
}

func (wire *ledgerState) packBurial(b Burial, tables ledgerTables) (ledgerBurial, error) {
	row := ledgerBurial{Burial: b}
	var err error
	if b.Parents != nil {
		row.ParentRef, err = internLedgerValue(b.Parents, &wire.ParentChains, tables.chains)
		if err != nil {
			return row, err
		}
	}
	if b.Operation != nil {
		if wire.Version < 16 {
			row.Operation, err = json.Marshal(b.Operation)
		} else {
			row.OperationRef, err = internLedgerValue(b.Operation, &wire.Operations, tables.operations)
		}
	}
	if err != nil {
		return row, err
	}
	return wire.packBurialMetadata(row, tables)
}

func (wire *ledgerState) packBurialMetadata(row ledgerBurial, tables ledgerTables) (ledgerBurial, error) {
	var err error
	row.Identity, err = wire.packIdentity(row.Burial.Identity, tables.volumes)
	if err != nil {
		return row, err
	}
	row.GraveID, err = wire.packIdentity(row.Burial.GraveID, tables.volumes)
	if err != nil {
		return row, err
	}
	if wire.Version < 16 {
		if row.Burial.Project != "" {
			row.Project, err = json.Marshal(row.Burial.Project)
		}
		if err == nil && row.Burial.Session != "" {
			row.Session, err = json.Marshal(row.Burial.Session)
		}
	} else if row.Burial.Project != "" || row.Burial.Session != "" {
		row.OriginRef = internLedgerFields(ledgerOrigin{Project: row.Burial.Project, Session: row.Burial.Session}, &wire.Origins, tables.origins)
	}
	return row, err
}

func (wire *ledgerState) packIdentity(id *Identity, refs map[ledgerVolume]*int) (*ledgerIdentity, error) {
	if id == nil {
		return nil, nil
	}
	row := &ledgerIdentity{FileID: id.FileID}
	if wire.Version < 16 {
		row.Volume, _ = json.Marshal(id.Volume)
		row.Serial, _ = json.Marshal(id.Serial)
		return row, nil
	}
	row.VolumeRef = internLedgerFields(ledgerVolume{Volume: id.Volume, Serial: id.Serial}, &wire.Volumes, refs)
	return row, nil
}

// These wire-only structs contain just strings and integers, so Go equality
// compares all JSON fields exactly without allocating an encoded key per row.
func internLedgerFields[T comparable](value T, values *[]*T, refs map[T]*int) *int {
	if ref, found := refs[value]; found {
		return ref
	}
	ref := len(*values)
	refs[value] = &ref
	stored := value
	*values = append(*values, &stored)
	return &ref
}

// Full JSON values preserve every field and path spelling. No filesystem
// normalization or hash collision can merge distinct recovery evidence.
func internLedgerValue[T any](value T, values *[]T, refs map[string]*int) (*int, error) {
	key, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if ref, found := refs[string(key)]; found {
		return ref, nil
	}
	ref := len(*values)
	refs[string(key)] = &ref
	*values = append(*values, value)
	return &ref, nil
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
	err := json.Unmarshal(data, &wire)
	if err != nil {
		// A future format may change field types. Decode just its version
		// before classifying that incompatibility as malformed current data.
		var header struct {
			Version int `json:"version"`
		}
		if json.Unmarshal(data, &header) == nil && header.Version > stateVersion {
			return State{}, newerStoreError{header.Version}
		}
		return State{}, err
	}
	if decoded.Version > stateVersion {
		return State{}, newerStoreError{decoded.Version}
	}
	return unpackLedger(wire)
}

func unpackLedger(wire ledgerState) (State, error) {
	decoded := *wire.plainState
	if len(wire.Burials) > 0 {
		decoded.Burials = make([]Burial, 0, len(wire.Burials))
	}
	if len(wire.RestoreCleanups) > 0 {
		decoded.RestoreCleanups = make([]RestoreCleanup, 0, len(wire.RestoreCleanups))
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

func (wire ledgerState) unpackOperation(row ledgerBurial) (*BurialOperation, error) {
	if wire.Version < 16 {
		if row.OperationRef != nil {
			return nil, fmt.Errorf("legacy burial #%d has an operationRef", row.ID)
		}
		var operation *BurialOperation
		if len(row.Operation) == 0 {
			return nil, nil
		}
		err := json.Unmarshal(row.Operation, &operation)
		return operation, err
	}
	if len(row.Operation) != 0 {
		return nil, fmt.Errorf("v%d burial #%d has inline operation", wire.Version, row.ID)
	}
	if row.OperationRef == nil {
		return nil, nil
	}
	ref := *row.OperationRef
	if ref < 0 || ref >= len(wire.Operations) || wire.Operations[ref] == nil {
		return nil, fmt.Errorf("burial #%d has invalid operationRef %d", row.ID, ref)
	}
	operation := *wire.Operations[ref]
	return &operation, nil
}

func (wire ledgerState) unpackBurialMetadata(row ledgerBurial) (ledgerBurial, error) {
	var err error
	row.Burial.Identity, err = wire.unpackIdentity(row.Identity)
	if err != nil {
		return row, err
	}
	row.Burial.GraveID, err = wire.unpackIdentity(row.GraveID)
	if err != nil {
		return row, err
	}
	if wire.Version < 16 {
		if row.OriginRef != nil {
			return row, fmt.Errorf("legacy burial #%d has an originRef", row.ID)
		}
		if len(row.Project) > 0 {
			err = json.Unmarshal(row.Project, &row.Burial.Project)
		}
		if err == nil && len(row.Session) > 0 {
			err = json.Unmarshal(row.Session, &row.Burial.Session)
		}
		return row, err
	}
	if len(row.Project) > 0 || len(row.Session) > 0 {
		return row, fmt.Errorf("v%d burial #%d has inline origin", wire.Version, row.ID)
	}
	if row.OriginRef != nil {
		ref := *row.OriginRef
		if ref < 0 || ref >= len(wire.Origins) || wire.Origins[ref] == nil {
			return row, fmt.Errorf("burial #%d has invalid originRef %d", row.ID, ref)
		}
		row.Burial.Project, row.Burial.Session = wire.Origins[ref].Project, wire.Origins[ref].Session
	}
	return row, nil
}

func (wire ledgerState) unpackIdentity(row *ledgerIdentity) (*Identity, error) {
	if row == nil {
		return nil, nil
	}
	if wire.Version < 16 {
		return unpackLegacyIdentity(row)
	}
	id := &Identity{FileID: row.FileID}
	if len(row.Volume) > 0 || len(row.Serial) > 0 || row.VolumeRef == nil {
		return nil, fmt.Errorf("v%d identity requires a volumeRef without inline volume or serial", wire.Version)
	}
	ref := *row.VolumeRef
	if ref < 0 || ref >= len(wire.Volumes) || wire.Volumes[ref] == nil {
		return nil, fmt.Errorf("identity has invalid volumeRef %d", ref)
	}
	id.Volume, id.Serial = wire.Volumes[ref].Volume, wire.Volumes[ref].Serial
	return id, nil
}

func unpackLegacyIdentity(row *ledgerIdentity) (*Identity, error) {
	if row.VolumeRef != nil {
		return nil, fmt.Errorf("legacy identity has a volumeRef")
	}
	id := &Identity{FileID: row.FileID}
	if len(row.Volume) > 0 {
		if err := json.Unmarshal(row.Volume, &id.Volume); err != nil {
			return nil, err
		}
	}
	if len(row.Serial) > 0 {
		if err := json.Unmarshal(row.Serial, &id.Serial); err != nil {
			return nil, err
		}
	}
	return id, nil
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
	row, err := wire.unpackBurialMetadata(row)
	if err != nil {
		return Burial{}, err
	}
	operation, err := wire.unpackOperation(row)
	if err != nil {
		return Burial{}, err
	}
	row.Burial.Operation = operation
	if wire.Version < 15 {
		return unpackLegacyBurial(row)
	}
	if len(row.Parents) != 0 {
		return Burial{}, fmt.Errorf("v%d burial #%d has inline parents", wire.Version, row.ID)
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
