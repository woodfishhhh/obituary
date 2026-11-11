package regtxn

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// Probe follows Microsoft's CreateTransaction/RollbackTransaction contract:
// one empty transaction, no registry handles, no commit or persistent writes.
func Probe() error {
	for _, proc := range []*windows.LazyProc{createTransaction, rollbackTransaction} {
		if err := proc.Find(); err != nil {
			return fmt.Errorf("load KTM: %w", err)
		}
	}
	h, _, err := createTransaction.Call(0, 0, 0, 0, 0, 0, 0)
	if windows.Handle(h) == windows.InvalidHandle {
		return fmt.Errorf("create empty transaction: %w", err)
	}
	defer windows.CloseHandle(windows.Handle(h))
	if ok, _, err := rollbackTransaction.Call(h); ok == 0 {
		return fmt.Errorf("rollback empty transaction: %w", err)
	}
	return nil
}
