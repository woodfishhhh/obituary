package testenv

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type desktopLocations struct {
	classes, run string
	shortcuts    []string
}

func realDesktopSnapshot() (desktopFootprint, error) {
	locations := desktopLocations{
		classes: `Software\Classes`,
		run:     `Software\Microsoft\Windows\CurrentVersion\Run`,
	}
	// DONT_VERIFY resolves redirected user folders without creating them;
	// test userdirs overrides must not hide changes to the real shell folders.
	for _, id := range []*windows.KNOWNFOLDERID{windows.FOLDERID_Programs, windows.FOLDERID_Desktop} {
		dir, err := windows.KnownFolderPath(id, windows.KF_FLAG_DONT_VERIFY)
		if err != nil {
			return nil, err
		}
		locations.shortcuts = append(locations.shortcuts, filepath.Join(dir, "讣告 Obituary.lnk"))
	}
	return snapshotDesktop(locations)
}

func snapshotDesktop(locations desktopLocations) (desktopFootprint, error) {
	state := desktopFootprint{}
	for _, parent := range []string{`*\shell`, `Directory\shell`, `Directory\Background\shell`} {
		if err := snapshotVerbs(state, locations.classes+`\`+parent); err != nil {
			return nil, err
		}
	}
	for _, path := range []string{`Obituary.ContextMenu`, `AppUserModelId\Obituary.ContextMenu`} {
		if err := snapshotRegistryTree(state, locations.classes+`\`+path); err != nil {
			return nil, err
		}
	}
	for _, suffix := range []string{"1001", "1007", "1030", "1000"} {
		path := locations.classes + `\CLSID\{BF07A7ED-0FE1-468B-A4D7-0E2A145C` + suffix + `}`
		if err := snapshotRegistryTree(state, path); err != nil {
			return nil, err
		}
	}
	if err := snapshotRun(state, locations.run); err != nil {
		return nil, err
	}
	for _, path := range locations.shortcuts {
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("snapshot shortcut %s: %w", path, err)
		}
		state[path] = fmt.Sprintf("%d %d %s", info.Size(), info.ModTime().UnixNano(), info.Mode())
	}
	return state, nil
}

func snapshotVerbs(state desktopFootprint, parent string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, parent, registry.ENUMERATE_SUB_KEYS)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("snapshot HKCU\\%s: %w", parent, err)
	}
	defer key.Close()
	names, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		if strings.HasPrefix(strings.ToLower(name), "obituary.") {
			if err := snapshotRegistryTree(state, parent+`\`+name); err != nil {
				return err
			}
		}
	}
	return nil
}

func snapshotRegistryTree(state desktopFootprint, path string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.READ)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("snapshot HKCU\\%s: %w", path, err)
	}
	defer key.Close()
	state[`HKCU\`+path] = "key"
	if err := snapshotValues(state, key, path, ""); err != nil {
		return err
	}
	names, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := snapshotRegistryTree(state, path+`\`+name); err != nil {
			return err
		}
	}
	return nil
}

func snapshotRun(state desktopFootprint, path string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("snapshot HKCU\\%s: %w", path, err)
	}
	defer key.Close()
	return snapshotValues(state, key, path, "obituary-")
}

func snapshotValues(state desktopFootprint, key registry.Key, path, prefix string) error {
	names, err := key.ReadValueNames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		if !strings.HasPrefix(strings.ToLower(name), prefix) {
			continue
		}
		size, _, err := key.GetValue(name, nil)
		if err != nil {
			return err
		}
		data := make([]byte, size)
		n, kind, err := key.GetValue(name, data)
		if err != nil {
			return fmt.Errorf("snapshot HKCU\\%s value %q: %w", path, name, err)
		}
		state[fmt.Sprintf("HKCU\\%s value %q", path, name)] = fmt.Sprintf("%d:%x", kind, data[:n])
	}
	return nil
}
