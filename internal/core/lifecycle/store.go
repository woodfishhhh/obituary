package lifecycle

import (
	"github.com/woodfishhhh/obituary/internal/core/internal/fileio"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

type Store = ledger.Store

func writeDurably(path string, data []byte) error { return fileio.WriteDurably(path, data) }
