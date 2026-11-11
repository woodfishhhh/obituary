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

func diagnoseExplorer(home string) DoctorCheck {
	if runtime.GOOS != "windows" {
		return DoctorCheck{Name: "explorer", Status: "INFO", Detail: "Explorer labels are Windows-only"}
	}
	c := DoctorCheck{Name: "explorer", Status: "PASS"}
	cfg, err := config.Load()
	if err != nil {
		c.Status, c.Detail = "FAIL", err.Error()+"; check config.toml [explorer]"
		return c
	}
	e := &ExplorerCheck{Enabled: cfg.Explorer.Enabled, Everywhere: cfg.Explorer.Everywhere, InGit: cfg.Explorer.InGit}
	c.Explorer = e
	paths, err := explorer.CleanupTargets(home)
	if err == nil {
		e.CleanupTargets = len(paths)
		err = explorerFailures(filepath.Join(home, "explorer.log"), paths, e)
	}
	if err != nil {
		c.Status, c.Detail = "FAIL", err.Error()+"; inspect explorer.json and explorer.log"
		return c
	}
	scope := "Downloads, Desktop and watched folders"
	if e.Everywhere {
		scope = "all tracked locations"
	}
	c.Detail = fmt.Sprintf("labels enabled=%t; scope=%s; include Git=%t; %d retained cleanup targets, %d with repeated errors in explorer.log; configure [explorer] in config.toml", e.Enabled, scope, e.InGit, e.CleanupTargets, e.RepeatedFailures)
	if e.RepeatedFailures > 0 {
		c.Status = "WARN"
		c.Detail += "; first: " + e.FirstFailure + "; inspect explorer.log and that folder's desktop.ini ownership/permissions"
	}
	return c
}

func explorerFailures(path string, targets []string, result *ExplorerCheck) error {
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
		if _, detail, ok := strings.Cut(line, " Explorer labels: "); ok {
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
