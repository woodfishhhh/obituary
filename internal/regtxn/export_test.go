//go:build windows

package regtxn

// AbortForTest makes the subsequent native commit fail without racing a writer.
func (tx *Transaction) AbortForTest() error {
	if ok, _, err := rollbackTransaction.Call(uintptr(tx.handle)); ok == 0 {
		return err
	}
	return nil
}
