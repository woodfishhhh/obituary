package lifecycle

import (
	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

func graveyardMayMatch(st *State, path string, identity *Identity) bool {
	return ledger.GraveyardMayMatch(st, path, identity)
}
