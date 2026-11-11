package core

import (
	"os"

	"github.com/woodfishhhh/obituary/internal/core/internal/fileio"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

type Store = ledger.Store

func writeDurably(path string, data []byte) error { return fileio.WriteDurably(path, data) }

func replaceDurably(root *os.Root, tmp, name string, file *os.File) error {
	return fileio.ReplaceDurably(root, tmp, name, file)
}

func syncReplacement(file *os.File) error { return fileio.SyncReplacement(file) }

func writeStoreBytes(file *os.File, data []byte) (int, error) {
	return fileio.WriteStoreBytes(file, data)
}
