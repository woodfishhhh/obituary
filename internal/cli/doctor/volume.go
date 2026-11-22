package doctor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

const volumeRecovery = "docker-volume-recovery"

func (lang diagnostics) diagnoseVolumeRecovery(home string, st ledger.State) []DoctorCheck {
	root, err := os.OpenRoot(home)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return []DoctorCheck{{Name: volumeRecovery, Status: "WARN", Detail: err.Error()}}
	}
	defer root.Close()
	const dir = "graveyard/docker-volumes"
	entries, err := fs.ReadDir(root.FS(), dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return []DoctorCheck{{Name: volumeRecovery, Status: "WARN", Detail: err.Error()}}
	}
	var rows []DoctorCheck
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		detail, err := lang.volumeArchiveDetail(root, filepath.Join(dir, entry.Name(), "volume.tar"), st)
		if err != nil {
			detail = fmt.Sprintf("%s: %v", filepath.Join(home, dir, entry.Name()), err)
		}
		if detail != "" {
			rows = append(rows, DoctorCheck{Name: volumeRecovery, Status: "WARN", Detail: detail})
		}
	}
	return rows
}

func (lang diagnostics) volumeArchiveDetail(root *os.Root, archive string, st ledger.State) (string, error) {
	prefix, _, _ := strings.Cut(filepath.Base(filepath.Dir(archive)), "-")
	id, err := strconv.ParseInt(prefix, 10, 64)
	if err != nil || id < 1 {
		return "", fmt.Errorf("invalid Docker volume entry %q", prefix)
	}
	full := filepath.Join(root.Name(), archive)
	if recorded := st.BurialIndex(id); recorded >= 0 {
		b := st.Burials[recorded]
		if b.Kind == ledger.KindDockerVolume && b.Grave == full {
			return "", nil
		}
	}
	return lang.volumeManifest(root, archive, id, st)
}

func (lang diagnostics) volumeManifest(root *os.Root, archive string, id int64, st ledger.State) (string, error) {
	full := filepath.Join(root.Name(), archive)
	manifest := archive + ".json"
	info, err := root.Lstat(manifest)
	if errors.Is(err, fs.ErrNotExist) {
		return lang.incompleteVolume(root, archive, id, st)
	}
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return "", fmt.Errorf("invalid volume manifest: %s", manifest)
	}
	data, err := root.ReadFile(manifest)
	if err != nil {
		return "", err
	}
	var m struct {
		ledger.Burial
		Version int    `json:"version"`
		Phase   string `json:"volumePhase"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return "", err
	}
	if m.Version != 1 || m.ID != id || m.Kind != ledger.KindDockerVolume || m.Grave != full || m.Volume == nil || m.Volume.Check() != nil || m.Volume.Name != m.Path {
		return "", fmt.Errorf("volume manifest does not match archive %s", full)
	}
	key := "cli.doctor.volume_unconfirmed"
	switch m.Phase {
	case "":
	case "events-incomplete":
		key = "cli.doctor.volume_events"
	case "removed":
		key = "cli.doctor.volume_commit"
	default:
		return "", fmt.Errorf("unknown volume manifest phase %q", m.Phase)
	}
	return lang.Text(key, nil) + "; " + fmt.Sprintf(lang.Text("cli.doctor.volume_inspect", nil), m.Volume.Target.DaemonID, m.Volume.Name, full), nil
}

func (lang diagnostics) incompleteVolume(root *os.Root, archive string, id int64, st ledger.State) (string, error) {
	for _, file := range []string{archive + ".partial", archive} {
		if _, err := root.Lstat(file); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return "", err
		}
		name := fmt.Sprintf("#%d", id)
		if idx := st.Index(id); idx >= 0 && st.Items[idx].Volume != nil {
			name += " " + st.Items[idx].Volume.Name
		}
		return fmt.Sprintf(lang.Text("cli.doctor.volume_incomplete", nil), name, filepath.Join(root.Name(), file)), nil
	}
	return "", nil
}
