package doctor

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	initcmd "github.com/woodfishhhh/obituary/internal/cli/init"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"github.com/woodfishhhh/obituary/internal/project"
	"github.com/woodfishhhh/obituary/internal/rules"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

type DoctorCheck struct {
	Name         string                         `json:"name"`
	Status       string                         `json:"status"`
	Detail       string                         `json:"detail"`
	Errors       int                            `json:"errors,omitempty"`
	Capabilities *hook.Capabilities             `json:"capabilities,omitempty"`
	Installation string                         `json:"installation,omitempty"`
	Watch        *WatchCheck                    `json:"watch,omitempty"`
	Explorer     *ExplorerCheck                 `json:"explorer,omitempty"`
	GUI          *GUICheck                      `json:"gui,omitempty"`
	KTM          *KTMCheck                      `json:"ktm,omitempty"`
	Queue        *ledger.ObservationQueueStatus `json:"queue,omitempty"`
}

func NewCmd() *cobra.Command { return NewCmdWithExecutable(nil) }

func NewCmdWithExecutable(executable cmdutil.Executable, agents ...string) *cobra.Command {
	return newCmdWithAgents(executable, agents)
}

func newCmdWithAgents(executable cmdutil.Executable, agents []string) *cobra.Command {
	var asJSON, verbose bool
	cmd := &cobra.Command{
		Use: "doctor", Short: "Check installation, integrations and data without changing anything", Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			checks := diagnose(time.Now(), executable, agents...)
			if verbose {
				for i := range checks {
					if installer, ok := integrate.LookupAgent(checks[i].Name); ok {
						checks[i].Capabilities = &installer.Capabilities
					}
				}
			}
			if asJSON {
				if err := cmdutil.PrintJSON(checks); err != nil {
					return err
				}
			} else {
				for _, c := range checks {
					fmt.Printf("%-4s %-12s %s\n", c.Status, c.Name, c.Detail)
					if c.Capabilities != nil {
						fmt.Printf("  %s\n", initcmd.CapabilityText(*c.Capabilities))
					}
				}
			}
			for _, c := range checks {
				if c.Status == "FAIL" {
					return cmdutil.ErrReported
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "include agent capability details")
	return cmd
}

//nolint:funlen // Retain the diagnostic flow while injecting executable discovery and the agent list.
func diagnose(now time.Time, executable cmdutil.Executable, agents ...string) []DoctorCheck {
	planner := initcmd.Planner{Executable: executable}
	var checks []DoctorCheck
	check := func(name, detail string, err error) {
		c := DoctorCheck{Name: name, Status: "PASS", Detail: detail}
		if err != nil {
			c.Status, c.Detail = "FAIL", err.Error()
		}
		checks = append(checks, c)
	}
	self, selfErr := executable.Path()
	path, pathErr := exec.LookPath("obit")
	pathCheck := DoctorCheck{Name: "path", Status: "PASS", Detail: "PATH resolves to the current executable: " + path}
	switch {
	case selfErr != nil:
		pathCheck.Status, pathCheck.Detail = "FAIL", selfErr.Error()
	case pathErr != nil:
		pathCheck.Status = "WARN"
		pathCheck.Detail = fmt.Sprintf("obit is not found on PATH; hooks use an absolute path (%s); typing obit will fail: %v", self, pathErr)
	default:
		a, err := os.Stat(self)
		if err == nil {
			var b os.FileInfo
			b, err = os.Stat(path)
			if err == nil && !os.SameFile(a, b) {
				pathCheck.Status = "WARN"
				pathCheck.Detail = fmt.Sprintf("PATH resolves to %s; hooks use an absolute path (%s); typing obit runs another executable", path, self)
			}
		}
		if err != nil {
			pathCheck.Status, pathCheck.Detail = "FAIL", err.Error()
		}
	}
	checks = append(checks, pathCheck)
	daemon := filepath.Join(filepath.Dir(self), "obitd.exe")
	if runtime.GOOS != "windows" {
		daemon = filepath.Join(filepath.Dir(self), "obitd")
		checks = append(checks, DoctorCheck{Name: "automatic-burial", Status: "WARN", Detail: "automatic burial and resurrection are unsupported on this platform; files and tracked entries are preserved"})
	}
	daemonErr := selfErr
	if daemonErr == nil {
		daemonErr = regularFile(daemon)
	}
	check("obitd", "companion executable exists: "+daemon, daemonErr)
	checks = append(checks, diagnoseGUI(daemon, daemonErr, binaryVersion), diagnoseRegistryTransactions(regtxnProbe))
	checks = append(checks, diagnoseShortcut(self, shortcutTarget), diagnoseDesktop(self, shortcutTarget))
	checks = append(checks, diagnoseIcons()...)
	home, homeErr := userdirs.Data()
	if homeErr != nil {
		checks = append(checks, DoctorCheck{Name: "update", Status: "INFO", Detail: homeErr.Error()})
		check("runner", "", homeErr)
		check("tray-startup", "", homeErr)
	} else {
		checks = append(checks, diagnoseUpdate(home))
		checks = append(checks, diagnoseRunner(home))
		checks = append(checks, diagnoseStartup(daemon, home))
	}
	available := 0
	scope := (integrate.Scope{Global: true}).ForInspection()
	inspector := initcmd.NewAgentInspector(scope, executable)
	if agents == nil {
		agents = initcmd.AgentNames()
	}
	for _, agent := range agents {
		if agent != "claude" && agent != "codex" {
			present, err := inspector.Present(agent)
			if !present {
				// A damaged config with no Obituary evidence belongs to the
				// agent, not to us; uninstall still refuses on it.
				available++
				continue
			}
			if err != nil {
				check(agent, "", err)
				continue
			}
		}
		checks = append(checks, diagnoseIntegration(agent, inspector))
		if agent != "claude" && agent != "codex" {
			continue // shared installer/guard wiring follows in the next phase
		}
		changes, err := planner.IntegrationPlan(agent, scope, false)
		registered, effective := integrate.GuardRMStatus(changes, agent)
		status, warning := initcmd.DeleteGuardStatus(registered, effective)
		check(agent+"-guard-rm", status, err)
		if err == nil && warning {
			checks[len(checks)-1].Status = "WARN"
		}
	}
	checks = append(checks, diagnoseExtension("mcp", executable), diagnoseExtension("skill", executable), diagnoseContextMenu(executable))
	if homeErr != nil {
		for _, name := range []string{"data", "store", "trust", "hook-log", "hook-queue", "graveyard-recovery", "slow-git", "watch", "explorer", "registration-identities", "docker-targets"} {
			check(name, "", homeErr)
		}
	} else {
		checks = append(checks, diagnoseData(home))
		state, err := (ledger.Store{Home: home}).Inspect()
		check("store", "store.json and its referenced graveyard snapshot are readable (absent store means empty)", err)
		checks = append(checks, diagnoseWatches(state, err, now, (&core.Service{FS: winfs.FS{}}).WatchPath))
		if err == nil {
			checks = append(checks, diagnoseDockerTargets(state))
		} else {
			checks = append(checks, DoctorCheck{Name: "docker-targets", Status: "WARN", Detail: err.Error()})
		}
		checks = append(checks, diagnoseExplorer(home))
		checks = append(checks, diagnoseRecovery(home))
		check("trust", "trusted_rules.json is readable (absence means no trusted rules)",
			(rules.TrustStore{Path: filepath.Join(home, "trusted_rules.json")}).Inspect())
		checks = append(checks, diagnoseProjectRules(home)...)
		checks = append(checks, diagnoseHookLog(home, now))
		checks = append(checks, diagnoseHookQueue(home, now))
		checks = append(checks, diagnoseSlowGit(home, now))
		checks = append(checks, diagnoseIdentities(home))
	}
	_, err := config.Load()
	check("config", "config.toml is valid (defaults apply when absent)", err)
	checks = append(checks, diagnoseCodexWindowsShell(), diagnoseDocker())
	if available > 0 {
		checks = append(checks, DoctorCheck{Name: "agents", Status: "INFO", Detail: fmt.Sprintf("另有 %d 家 agent 可接入：obit init --show 查看", available)})
	}
	return checks
}

func diagnoseProjectRules(home string) []DoctorCheck {
	cwd, err := os.Getwd()
	if err != nil {
		return []DoctorCheck{{Name: "project-rules", Status: "FAIL", Detail: err.Error()}}
	}
	p, found, err := project.Find(cwd)
	if err == nil && !found {
		return nil
	}
	if err == nil {
		var state rules.FileState
		_, state, err = rules.Load(p.Root, rules.TrustStore{Path: filepath.Join(home, "trusted_rules.json")})
		if err == nil && state.Status == rules.Absent {
			return nil
		}
	}
	if err != nil {
		return []DoctorCheck{{Name: "project-rules", Status: "WARN", Detail: err.Error()}}
	}
	return []DoctorCheck{{Name: "project-rules", Status: "PASS", Detail: "project rules permit automatic registration"}}
}

func diagnoseCodexWindowsShell() DoctorCheck {
	c := DoctorCheck{Name: "codex-windows-shell", Status: "WARN"}
	cfg, err := config.Load()
	if err != nil {
		c.Status, c.Detail = "FAIL", err.Error()
	} else if cfg.WindowsShell("codex") == "" {
		c.Detail = "codex_windows_shell is unset; ambiguous Windows shell commands pass through; obit init --agent codex --guard-rm can infer a declaration, or set bash/powershell in Obituary config.toml"
	} else {
		c.Status = "PASS"
		c.Detail = "Codex Windows shell = " + cfg.WindowsShell("codex") + " (agent_shells.codex or codex_windows_shell); declaration used for ambiguous native Windows commands; actual per-call shell is not reported by Codex"
	}
	return c
}

func diagnoseRecovery(home string) DoctorCheck {
	c := DoctorCheck{Name: "graveyard-recovery", Status: "PASS"}
	pending, err := (&core.Service{Store: ledger.Store{Home: home}, FS: winfs.FS{}}).PendingGraves()
	c.Detail = fmt.Sprintf("%d graveyard records need attention", len(pending))
	if len(pending) > 0 || err != nil {
		c.Status = "WARN"
	}
	for _, p := range pending {
		c.Detail += "; " + p.Manifest + " (" + p.Status + ")"
		if p.Error != "" {
			c.Detail += ": " + p.Error
		}
	}
	if err != nil {
		c.Detail += "; " + err.Error()
	}
	return c
}

func regularFile(path string) error {
	info, err := os.Stat(path)
	if err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	return err
}

func diagnoseHookLog(home string, now time.Time, protectedRoots ...string) DoctorCheck {
	c := DoctorCheck{Name: "hook-log", Status: "PASS"}
	var permissionAt time.Time
	permission := ""
	// Old file first, then later lines win ties in the second-resolution log.
	for _, name := range []string{"hook.log.1", "hook.log"} {
		data, err := os.ReadFile(filepath.Join(home, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			c.Status, c.Detail = "FAIL", err.Error()
			return c
		}
		for _, line := range strings.Split(string(data), "\n") {
			if !hook.IsErrorLogLine(line, protectedRoots...) {
				continue
			}
			stamp, _, _ := strings.Cut(line, " ")
			at, err := time.Parse(time.RFC3339, stamp)
			if err == nil && !at.Before(now.Add(-24*time.Hour)) && !at.After(now) {
				c.Errors++
				if summary, denied := hook.PermissionSummary(line); denied && !at.Before(permissionAt) {
					permissionAt, permission = at, summary
				}
			}
		}
	}
	if c.Errors > 0 {
		c.Status = "WARN"
	}
	c.Detail = fmt.Sprintf("%d error lines in the last 24 hours (including rotated hook.log)", c.Errors)
	if permission != "" {
		c.Detail += "; a hook account reported denied access. " + hook.PermissionHint
		c.Detail += fmt.Sprintf("; latest denied access at %s: %s; %s", permissionAt.Format(time.RFC3339), permission, hook.PermissionReportHint)
	}
	return c
}
