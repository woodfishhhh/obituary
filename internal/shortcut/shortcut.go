// Package shortcut plans user-owned desktop entry points using integration receipts.
package shortcut

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/integrate"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

const Name = "讣告 Obituary.lnk"

type Location string

const (
	StartMenu Location = "start-menu"
	Desktop   Location = "desktop"
)

// Codec separates Shell Link serialization from ownership and installation.
type Codec interface {
	Encode(target string) ([]byte, error)
	Target(data []byte) (string, error)
}

func Paths(place Location) ([]string, error) {
	if place == Desktop {
		dir, err := userdirs.Desktop()
		return []string{filepath.Join(dir, Name)}, err
	}
	if place != StartMenu {
		return nil, fmt.Errorf("unknown shortcut location %q", place)
	}
	dir, err := userdirs.Programs()
	return []string{filepath.Join(dir, Name), filepath.Join(dir, "Obituary.lnk"), filepath.Join(dir, "Scoop Apps", Name), filepath.Join(dir, "Scoop Apps", "Obituary.lnk")}, err
}

func Target(executable string) string { return filepath.Join(filepath.Dir(executable), "obit-gui.exe") }

// Inspect includes installer and package-manager entries without claiming them.
func Inspect(codec Codec, executable string, place Location) (string, error) {
	paths, err := Paths(place)
	if err != nil {
		return "", err
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		got, err := codec.Target(data)
		if err != nil {
			return "", err
		}
		if strings.EqualFold(filepath.Clean(got), filepath.Clean(Target(executable))) {
			return path, nil
		}
	}
	return "", nil
}

func Plan(codec Codec, executable string, place Location, enabled bool) ([]integrate.Change, error) {
	paths, err := Paths(place)
	if err != nil {
		return nil, err
	}
	if enabled {
		present, err := Inspect(codec, executable, place)
		if err != nil {
			return nil, err
		}
		if present != "" {
			return []integrate.Change{}, nil
		}
	}
	content, err := shortcutContent(codec, executable, enabled)
	if err != nil {
		return nil, err
	}
	// ID and Version bind the receipt to this installation's absolute target.
	// The shared asset protocol also verifies path, bytes and concurrent edits.
	asset := integrate.ManagedAsset{Path: paths[0], ID: "shortcut:" + paths[0] + " -> " + Target(executable), Owner: "obit-init", Version: Target(executable), Content: content}
	return integrate.PlanManagedAssets(integrate.Scope{Global: true}, []integrate.ManagedAsset{asset}, !enabled)
}

func shortcutContent(codec Codec, executable string, enabled bool) ([]byte, error) {
	if !enabled {
		return nil, nil
	}
	info, err := os.Stat(Target(executable))
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("shortcut target is not a regular executable")
	}
	return codec.Encode(Target(executable))
}

func ReadTarget(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return Native{}.Target(data)
}
