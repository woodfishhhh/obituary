package app

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"sync/atomic"
)

const (
	// A resident decodes the ledger for every list and replay batch, and garbage from
	// the decode must fit below the limit or the collector runs back to back (the
	// runtime caps it at half the CPU), which made decoding three to four times slower
	// than in a command line process. 32 MiB holds the decode of a thousand active and a
	// thousand buried entries; it grows with the ledger for larger ones.
	minMemoryLimit = 32 << 20
	maxMemoryLimit = 512 << 20
	// The decoded ledger, the copy kept for the next replay, the bytes read and the
	// garbage of one decode together are about this many times the size of store.json.
	memoryPerLedgerByte = 64
)

func memoryLimitFor(ledgerBytes int64) int64 {
	return min(maxMemoryLimit, max(minMemoryLimit, ledgerBytes*memoryPerLedgerByte))
}

var memoryLimit atomic.Int64

// fitMemoryLimit sets the resident's soft memory limit for the ledger it holds now.
// It is cheap (one stat) and only touches the runtime when the size class changed.
func fitMemoryLimit(home string) {
	info, err := os.Stat(filepath.Join(home, "store.json"))
	if err != nil {
		info = nil
	}
	var size int64
	if info != nil {
		size = info.Size()
	}
	want := memoryLimitFor(size)
	if prev := memoryLimit.Load(); prev != 0 && want*4 < prev*5 && want*5 > prev*4 {
		return
	}
	memoryLimit.Store(want)
	debug.SetMemoryLimit(want)
}
