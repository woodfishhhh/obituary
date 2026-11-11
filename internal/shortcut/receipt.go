package shortcut

import (
	"errors"
	"os"

	"github.com/woodfishhhh/obituary/internal/integrate"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// ReceiptPath separates ownership metadata from Shell-visible entry points.
func ReceiptPath(path string) (string, error) {
	return userdirs.ReceiptPath(path, "shortcut-receipts", ".obituary-asset")
}

func planReceipt(executable, path string, content []byte, uninstall bool) ([]integrate.Change, error) {
	receipt, err := ReceiptPath(path)
	if err != nil {
		return nil, err
	}
	// ID and Version bind the receipt to this installation's absolute target.
	asset := integrate.ManagedAsset{Path: path, ID: "shortcut:" + path + " -> " + Target(executable), Owner: "obit-init", Version: Target(executable), Content: content}
	return integrate.PlanManagedAssetReceipt(integrate.Scope{Global: true}, asset, receipt, uninstall)
}

func hasReceipt(path string) (bool, error) {
	receipt, err := ReceiptPath(path)
	if err != nil {
		return false, err
	}
	for _, candidate := range []string{receipt, integrate.ManagedAssetReceiptPath(path)} {
		if _, err := os.Lstat(candidate); err == nil {
			return true, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	return false, nil
}

func planExistingShortcut(executable, primary, present string) ([]integrate.Change, error) {
	if present != primary {
		return []integrate.Change{}, nil
	}
	has, err := hasReceipt(present)
	if err != nil || !has {
		return []integrate.Change{}, err
	}
	// Keep the original serialized link; Shell metadata need not be stable.
	content, err := os.ReadFile(present)
	if err != nil {
		return nil, err
	}
	return planReceipt(executable, present, content, false)
}

// CheckReceipt verifies managed links read-only and accepts unclaimed installer
// links. A legacy receipt remains readable until a later Apply migrates it.
func CheckReceipt(executable, path string) error {
	has, err := hasReceipt(path)
	if err != nil || !has {
		return err
	}
	plan, err := planReceipt(executable, path, nil, true)
	if err != nil {
		return err
	}
	return integrate.PreservedError(plan)
}
