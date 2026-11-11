package contextmenu

import (
	"fmt"
	"reflect"
)

// ApplyReceipt records committed paths and all paths still needing a new plan.
// Each native change is transactional; a failed path has no partial writes.
type ApplyReceipt struct {
	Applied   []string `json:"applied"`
	Remaining []string `json:"remaining"`
}

func ApplyWithReceipt(reg Registry, changes []Change) (ApplyReceipt, error) {
	receipt := ApplyReceipt{Applied: []string{}, Remaining: make([]string, 0, len(changes))}
	for _, c := range changes {
		receipt.Remaining = append(receipt.Remaining, c.Path)
	}
	release, err := reg.Lock()
	if err != nil {
		return receipt, err
	}
	defer release()
	if err := preflight(reg, changes); err != nil {
		return receipt, err
	}
	for _, c := range changes {
		if err := reg.CompareAndSwap(c); err != nil {
			return receipt, fmt.Errorf("context menu %s: %w", c.Path, err)
		}
		receipt.Applied = append(receipt.Applied, c.Path)
		receipt.Remaining = receipt.Remaining[1:]
	}
	return receipt, nil
}

func preflight(reg Registry, changes []Change) error {
	for _, c := range changes {
		current, err := reg.ReadTree(c.Path)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(current, c.Before) {
			return fmt.Errorf("preserved %s: registry changed after planning", c.Path)
		}
	}
	return nil
}
