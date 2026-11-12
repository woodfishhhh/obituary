package lifecycle

import (
	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

func BurialPathMatches(buried string, directory bool, path string) bool {
	return ledger.BurialPathMatches(buried, directory, path)
}
