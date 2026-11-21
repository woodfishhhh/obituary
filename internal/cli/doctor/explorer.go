package doctor

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/explorer"
)

type ExplorerCheck struct {
	Enabled          bool   `json:"enabled"`
	Everywhere       bool   `json:"everywhere"`
	InGit            bool   `json:"inGit"`
	CleanupTargets   int    `json:"cleanupTargets"`
	RepeatedFailures int    `json:"repeatedFailures"`
	FirstFailure     string `json:"firstFailure,omitempty"`
}

func (lang diagnostics) diagnoseExplorer(home string) DoctorCheck {
	if runtime.GOOS != "windows" {
		return DoctorCheck{Name: "explorer", Status: "INFO", Detail: lang.Text("cli.doctor.explorer_labels_are_windows_only", nil)}
	}
	c := DoctorCheck{Name: "explorer", Status: "PASS"}
	cfg, err := config.Load()
	if err != nil {
		c.Status, c.Detail = "FAIL", err.Error()+lang.Text("cli.doctor.check_config_toml_explorer", nil)
		return c
	}
	e := &ExplorerCheck{Enabled: cfg.Explorer.Enabled, Everywhere: cfg.Explorer.Everywhere, InGit: cfg.Explorer.InGit}
	c.Explorer = e
	paths, err := explorer.CleanupTargets(home)
	if err == nil {
		e.CleanupTargets = len(paths)
		err = lang.explorerFailures(filepath.Join(home, "explorer.log"), paths, e)
	}
	if err != nil {
		c.Status, c.Detail = "FAIL", err.Error()+lang.Text("cli.doctor.inspect_explorer_json_and_explorer_log", nil)
		return c
	}
	scope := lang.Text("cli.doctor.downloads_desktop_and_watched_folders", nil)
	if e.Everywhere {
		scope = lang.Text("cli.doctor.all_tracked_locations", nil)
	}
	c.Detail = fmt.Sprintf(lang.Text("cli.doctor.labels_enabled_scope_include_git_retained_cleanup", nil), e.Enabled, scope, e.InGit, e.CleanupTargets, e.RepeatedFailures)
	if e.RepeatedFailures > 0 {
		c.Status = "WARN"
		c.Detail += lang.Text("cli.doctor.first", nil) + e.FirstFailure + lang.Text("cli.doctor.inspect_explorer_log_and_that_folder_s", nil)
	}
	return c
}

func (lang diagnostics) explorerFailures(path string, targets []string, result *ExplorerCheck) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	counts := make(map[string]int)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if _, detail, ok := strings.Cut(line, lang.Text("cli.doctor.explorer_labels", nil)); ok {
			line = detail
		}
		// errors.Join produces continuation lines without the log prefix.
		for _, target := range targets {
			if strings.HasPrefix(line, target+": ") {
				counts[target]++
			}
		}
	}
	for _, target := range targets {
		if counts[target] >= 2 {
			result.RepeatedFailures++
			if result.FirstFailure == "" {
				result.FirstFailure = target
			}
		}
	}
	return scanner.Err()
}
