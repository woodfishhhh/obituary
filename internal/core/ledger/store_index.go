package ledger

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// Keys only select a full snapshot check; a hash collision adds a check. They
// never select a grave or authorize a move, purge, or recovery operation.
// This index commits with its snapshot reference, unlike the advisory notice file.
type graveyardIndex struct {
	Keys          []uint64 `json:"keys,omitempty"`
	Volumes       []string `json:"volumes,omitempty"`
	MaxID         int64    `json:"maxId"`
	ReservedAudit int      `json:"reservedAudit"`
}

func graveyardKey(kind, value string) uint64 {
	digest := sha256.Sum256([]byte(kind + ":" + value))
	return binary.LittleEndian.Uint64(digest[:8])
}

func graveyardPathKey(path string) uint64 {
	// samePath uses Unicode simple folding for conservative protection. Use
	// the least rune in each fold cycle so every equivalent spelling hits.
	path = strings.Map(func(r rune) rune {
		lowest := r
		for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
			lowest = min(lowest, next)
		}
		return lowest
	}, filepath.Clean(path))
	return graveyardKey("path", path)
}

func graveyardIdentityKey(id Identity) uint64 {
	data, _ := json.Marshal(id.Normalized())
	return graveyardKey("identity", string(data))
}

func buildGraveyardIndex(st State) *graveyardIndex {
	idx := &graveyardIndex{}
	addID := func(id int64) {
		idx.MaxID = max(idx.MaxID, id)
		idx.Keys = append(idx.Keys, graveyardKey("id", fmt.Sprint(id)))
	}
	add := func(b Burial) {
		addID(b.ID)
		idx.Keys = append(idx.Keys, graveyardPathKey(b.Path))
		if b.Identity != nil {
			idx.Keys = append(idx.Keys, graveyardIdentityKey(*b.Identity))
		}
		if b.Kind != KindContainer {
			idx.Volumes = append(idx.Volumes, pathutil.VolumeRoot(b.Path))
		}
		for _, id := range b.Contains {
			addID(id)
		}
	}
	for _, b := range st.Burials {
		add(b)
		if b.PurgeIntent != nil && FindContainerIntent(&st, b.ID) == nil {
			idx.ReservedAudit++
		}
	}
	for _, c := range st.RestoreCleanups {
		add(c.Burial)
	}
	slices.Sort(idx.Keys)
	idx.Keys = slices.Compact(idx.Keys)
	slices.Sort(idx.Volumes)
	idx.Volumes = slices.Compact(idx.Volumes)
	return idx
}

func GraveyardMayMatch(st *State, path string, identity *Identity) bool {
	if st.graveyardLoaded || st.GraveyardIndex == nil {
		return true
	}
	if _, hit := slices.BinarySearch(st.GraveyardIndex.Keys, graveyardPathKey(path)); hit {
		return true
	}
	if identity != nil {
		_, hit := slices.BinarySearch(st.GraveyardIndex.Keys, graveyardIdentityKey(*identity))
		return hit
	}
	return false
}

// Only called under the same store.lock that loaded the hot reference. It
// takes no graveyard lock, preserving the graveyard -> store lock order.
func (s Store) loadGraveyardForReference(st *State, ref string) error {
	if st.graveyardLoaded || st.GraveyardIndex == nil {
		return s.loadGraveyard(st)
	}
	if id, numeric, err := BurialReferenceID(ref); numeric {
		if err != nil {
			return err
		}
		if _, hit := slices.BinarySearch(st.GraveyardIndex.Keys, graveyardKey("id", fmt.Sprint(id))); !hit {
			return nil
		}
	} else if !GraveyardMayMatch(st, ref, nil) {
		return nil
	}
	return s.loadGraveyard(st)
}
