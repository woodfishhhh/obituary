package core

import (
	"os"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

const DataOwnerFile = ledger.DataOwnerFile

func VerifyDataOwner(root *os.Root) error { return ledger.VerifyDataOwner(root) }
