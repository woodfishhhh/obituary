package main

import (
	"bytes"
	"debug/pe"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
)

func checkRelease(root, dist string) error {
	data, err := os.ReadFile(filepath.Join(dist, "metadata.json"))
	if err != nil {
		return err
	}
	var metadata struct{ Version string }
	if err := json.Unmarshal(data, &metadata); err != nil {
		return err
	}
	icon, err := os.ReadFile(filepath.Join(root, "internal", "tray", "obituary.ico"))
	if err != nil {
		return err
	}
	for _, app := range applications {
		for _, arch := range app.arches {
			paths, err := filepath.Glob(filepath.Join(dist, app.name+"_windows_"+string(arch)+"*", app.name+".exe"))
			if err != nil {
				return err
			}
			if len(paths) != 1 {
				return fmt.Errorf("expected one %s/%s executable, found %d", app.name, arch, len(paths))
			}
			if err := checkExecutable(root, paths[0], app, arch, metadata.Version, icon); err != nil {
				return fmt.Errorf("%s: %w", paths[0], err)
			}
			fmt.Printf("OK %s/%s: %s; version, icon, manifest, subsystem\n", app.name, arch, metadata.Version)
		}
	}
	return nil
}

func checkExecutable(root, path string, app application, arch winres.Arch, release string, icon []byte) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	peFile, err := pe.NewFile(file)
	if err != nil {
		return err
	}
	header, ok := peFile.OptionalHeader.(*pe.OptionalHeader64)
	machine := map[winres.Arch]uint16{winres.ArchAMD64: pe.IMAGE_FILE_MACHINE_AMD64, winres.ArchARM64: pe.IMAGE_FILE_MACHINE_ARM64}[arch]
	if !ok || header.Subsystem != app.subsystem || peFile.Machine != machine {
		return fmt.Errorf("wrong PE architecture or subsystem for %s/%s", app.name, arch)
	}
	rs, err := winres.LoadFromEXE(file)
	if err != nil {
		return err
	}
	return checkResources(root, rs, app, release, icon)
}

func checkResources(root string, rs *winres.ResourceSet, app application, release string, icon []byte) error {
	actual, err := version.FromBytes(rs.Get(winres.RT_VERSION, winres.ID(1), winres.LCIDDefault))
	if err != nil {
		return fmt.Errorf("version resource: %w", err)
	}
	expected, err := versionInfo(root, app, release)
	if err != nil {
		return err
	}
	if actual.FileVersion != expected.FileVersion || actual.ProductVersion != expected.ProductVersion {
		return fmt.Errorf("fixed version does not match %s", release)
	}
	table := actual.Table().GetMainTranslation()
	for key, value := range expected.Table().GetMainTranslation() {
		if table[key] != value {
			return fmt.Errorf("%s = %q, want %q", key, table[key], value)
		}
	}
	if !bytes.Equal(rs.Get(winres.RT_MANIFEST, winres.ID(1), winres.LCIDDefault), manifest) {
		return fmt.Errorf("missing or incorrect application manifest")
	}
	if group := rs.Get(winres.RT_GROUP_ICON, appIconID, winres.LCIDNeutral); len(group) == 0 || !bytes.Equal(rs.Get(winres.RT_GROUP_ICON, classIconID, winres.LCIDNeutral), group) {
		return fmt.Errorf("taskbar icon (IDI_APPLICATION) missing or different from the application icon")
	}
	extracted, err := rs.GetIcon(appIconID)
	if err != nil {
		return fmt.Errorf("application icon: %w", err)
	}
	var data bytes.Buffer
	if err := extracted.SaveICO(&data); err != nil {
		return err
	}
	if !bytes.Equal(data.Bytes(), icon) {
		return fmt.Errorf("application icon differs from the multi-size tray ICO")
	}
	return nil
}
