package core

import (
	"os"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

func OwnedLedgerFile(root *os.Root, name string) bool { return ledger.OwnedLedgerFile(root, name) }
