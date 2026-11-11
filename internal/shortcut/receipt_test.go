package shortcut

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/integrate"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func TestReceiptPathUsesDataDirectory(t *testing.T) {
	exe := fixture(t)
	paths, _ := Paths(Desktop)
	path, err := ReceiptPath(paths[0])
	home, _ := userdirs.Data()
	if err != nil || filepath.Dir(path) != filepath.Join(home, "shortcut-receipts") || len(filepath.Base(path)) != 64+len(".obituary-asset") {
		t.Fatal(path, err)
	}
	if runtime.GOOS == "windows" {
		alias, err := ReceiptPath(strings.ReplaceAll(strings.ToUpper(paths[0]), `\`, "/"))
		if err != nil || alias != path {
			t.Fatal("inconsistent path normalization", alias, path, err)
		}
	}
	plan, err := Plan(textCodec{}, exe, Desktop, true)
	applyShortcut(t, plan, err)
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(data, []byte(fmt.Sprintf("%x", sha256.Sum256([]byte(Target(exe)))))) {
		t.Fatal("missing content digest", string(data), err)
	}
	var receipt struct{ ID, Version string }
	if err := json.Unmarshal(data, &receipt); err != nil || receipt.ID != "shortcut:"+paths[0]+" -> "+Target(exe) || receipt.Version != Target(exe) {
		t.Fatal("receipt lost absolute path or target", receipt, err)
	}
}

func TestShortcutLegacyReceiptMigration(t *testing.T) {
	for _, place := range []Location{StartMenu, Desktop} {
		for _, uninstall := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/uninstall=%t", place, uninstall), func(t *testing.T) {
				exe := fixture(t)
				paths, _ := Paths(place)
				legacy := writeLegacyReceipt(t, exe, paths[0])
				before, _ := os.ReadFile(legacy)
				if err := CheckReceipt(exe, paths[0]); err != nil {
					t.Fatal(err)
				}
				plan, err := Plan(textCodec{}, exe, place, !uninstall)
				if data, _ := os.ReadFile(legacy); !bytes.Equal(data, before) {
					t.Fatal("read or planning changed legacy receipt")
				}
				applyShortcut(t, plan, err)
				assertShortcutDirectory(t, paths[0], !uninstall)
				receipt, _ := ReceiptPath(paths[0])
				if uninstall {
					if _, err := os.Stat(receipt); !os.IsNotExist(err) {
						t.Fatal("uninstall retained receipt", err)
					}
					return
				}
				if data, err := os.ReadFile(receipt); err != nil || !bytes.Equal(data, before) {
					t.Fatal("migration changed receipt contents", string(data), err)
				}
				plan, err = Plan(textCodec{}, exe, place, true)
				if err != nil || integrate.Pending(plan) {
					t.Fatal("repeated installation changed shortcut", plan, err)
				}
			})
		}
	}
}

func TestShortcutMigrationPreservesEditedAndConcurrentReceipts(t *testing.T) {
	for _, stage := range []string{"before-plan", "after-plan"} {
		t.Run(stage, func(t *testing.T) {
			exe := fixture(t)
			paths, _ := Paths(Desktop)
			legacy := writeLegacyReceipt(t, exe, paths[0])
			plan, err := Plan(textCodec{}, exe, Desktop, true)
			if err != nil {
				t.Fatal(err)
			}
			writeLink(t, legacy, "user receipt")
			if stage == "before-plan" {
				if _, err := Plan(textCodec{}, exe, Desktop, true); err == nil {
					t.Fatal("accepted edited legacy receipt")
				}
			} else if err := integrate.Apply(plan); err == nil {
				t.Fatal("accepted stale migration plan")
			}
			if data, _ := os.ReadFile(legacy); string(data) != "user receipt" {
				t.Fatal("edited receipt removed")
			}
			receipt, _ := ReceiptPath(paths[0])
			if _, err := os.Stat(receipt); !os.IsNotExist(err) {
				t.Fatal("wrote receipt despite conflict", err)
			}
		})
	}
}

func TestShortcutMigrationLeavesUnclaimedBackups(t *testing.T) {
	exe := fixture(t)
	paths, _ := Paths(Desktop)
	legacy := writeLegacyReceipt(t, exe, paths[0])
	writeLink(t, legacy+".bak", "unclaimed backup")
	plan, err := Plan(textCodec{}, exe, Desktop, true)
	applyShortcut(t, plan, err)
	if data, _ := os.ReadFile(legacy + ".bak"); string(data) != "unclaimed backup" {
		t.Fatal("migration changed unclaimed backup")
	}
}

func TestShortcutUninstallRemovesReceipt(t *testing.T) {
	exe := fixture(t)
	paths, _ := Paths(Desktop)
	plan, err := Plan(textCodec{}, exe, Desktop, true)
	applyShortcut(t, plan, err)
	receipt, _ := ReceiptPath(paths[0])
	if _, err := os.Stat(receipt); err != nil {
		t.Fatal(err)
	}
	plan, err = Plan(textCodec{}, exe, Desktop, false)
	applyShortcut(t, plan, err)
	if _, err := os.Stat(receipt); !os.IsNotExist(err) {
		t.Fatal("receipt remains after uninstall", err)
	}
}

func writeLegacyReceipt(t *testing.T, exe, path string) string {
	t.Helper()
	state := struct {
		Schema  int      `json:"schema"`
		ID      string   `json:"id"`
		File    string   `json:"file"`
		Version string   `json:"version"`
		Owners  []string `json:"owners"`
		SHA256  string   `json:"sha256"`
	}{1, "shortcut:" + path + " -> " + Target(exe), filepath.Base(path), Target(exe), []string{"obit-init"}, fmt.Sprintf("%x", sha256.Sum256([]byte(Target(exe))))}
	var data bytes.Buffer
	encoder := json.NewEncoder(&data)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(state); err != nil {
		t.Fatal(err)
	}
	writeLink(t, path, Target(exe))
	legacy := integrate.ManagedAssetReceiptPath(path)
	writeLink(t, legacy, data.String())
	return legacy
}

func applyShortcut(t *testing.T, plan []integrate.Change, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	if err := integrate.Apply(plan); err != nil {
		t.Fatal(err)
	}
}
