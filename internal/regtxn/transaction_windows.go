// Package regtxn supplies the transactional operations missing from x/sys/registry.
package regtxn

import (
	"crypto/rand"
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// x/sys/windows/registry/key.go supplies value and enumeration operations but
// no KTM transaction or handle-relative deletion. These bindings follow the
// Microsoft RegOpenKeyTransactedW / RegCreateKeyTransactedW / NtDeleteKey APIs.
// An unavailable transaction fails closed; ordinary concurrent edits must not
// fit between comparison and deletion.
var (
	ktm                 = windows.NewLazySystemDLL("KtmW32.dll")
	createTransaction   = ktm.NewProc("CreateTransaction")
	commitTransaction   = ktm.NewProc("CommitTransaction")
	rollbackTransaction = ktm.NewProc("RollbackTransaction")
	advapi              = windows.NewLazySystemDLL("advapi32.dll")
	openKey             = advapi.NewProc("RegOpenKeyTransactedW")
	createKey           = advapi.NewProc("RegCreateKeyTransactedW")
	deleteKey           = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtDeleteKey")
)

type Transaction struct{ handle windows.Handle }

func Do(fn func(*Transaction) error) error {
	h, _, err := createTransaction.Call(0, 0, 0, 0, 0, 0, 0)
	if windows.Handle(h) == windows.InvalidHandle {
		return fmt.Errorf("create registry transaction: %w", err)
	}
	tx := &Transaction{handle: windows.Handle(h)}
	defer windows.CloseHandle(tx.handle)
	defer rollbackTransaction.Call(h)
	if err := fn(tx); err != nil {
		return err
	}
	if ok, _, err := commitTransaction.Call(h); ok == 0 {
		return fmt.Errorf("commit registry transaction: %w", err)
	}
	return nil
}

func (tx *Transaction) Open(parent registry.Key, path string, access uint32) (registry.Key, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var key registry.Key
	status, _, _ := openKey.Call(uintptr(parent), uintptr(unsafe.Pointer(name)), 0,
		uintptr(access), uintptr(unsafe.Pointer(&key)), uintptr(tx.handle), 0)
	if status != 0 {
		return 0, windows.Errno(status)
	}
	return key, nil
}

func (tx *Transaction) OpenPinned(parent registry.Key, path string, access uint32) (registry.Key, error) {
	key, err := tx.Open(parent, path, access|registry.SET_VALUE)
	if err != nil {
		return 0, err
	}
	if err := tx.PinWrite(key); err != nil {
		key.Close()
		return 0, err
	}
	return key, nil
}

func (tx *Transaction) Create(parent registry.Key, path string, access uint32) (registry.Key, error) {
	return tx.create(parent, path, access, false)
}

func (tx *Transaction) CreateNew(parent registry.Key, path string, access uint32) (registry.Key, error) {
	return tx.create(parent, path, access, true)
}

func (tx *Transaction) create(parent registry.Key, path string, access uint32, exclusive bool) (registry.Key, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var key registry.Key
	var disposition uint32
	status, _, _ := createKey.Call(uintptr(parent), uintptr(unsafe.Pointer(name)), 0, 0, 0,
		uintptr(access), 0, uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&disposition)), uintptr(tx.handle), 0)
	if status != 0 {
		return 0, windows.Errno(status)
	}
	if exclusive && disposition != 1 { // REG_CREATED_NEW_KEY
		key.Close()
		return 0, fmt.Errorf("registry key appeared after inspection")
	}
	return key, nil
}

// PinWrite acquires KTM's write conflict protection before reading expected
// values. Transacted reads alone do not exclude ordinary registry writers.
// The temporary value is rolled back on failure and absent at commit.
func (tx *Transaction) PinWrite(key registry.Key) error {
	name := "Obituary.Transaction." + rand.Text()
	if _, _, err := key.GetStringValue(name); !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("registry transaction marker is occupied or unreadable")
	}
	if err := key.SetStringValue(name, ""); err != nil {
		return err
	}
	return key.DeleteValue(name)
}

// Delete operates on the same transactional handle that was inspected.
func (tx *Transaction) Delete(key registry.Key) error {
	status, _, _ := deleteKey.Call(uintptr(key))
	if status != 0 {
		return windows.NTStatus(status)
	}
	return nil
}
