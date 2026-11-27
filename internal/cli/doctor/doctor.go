package doctor

import (
	"errors"
	"fmt"
	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"io"
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
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"github.com/woodfishhhh/obituary/internal/project"
	"github.com/woodfishhhh/obituary/internal/rules"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

// JSON diagnostics use the historical English text, regardless of the interface language.
type diagnostics struct {
	*i18n.Localizer
	machine bool
}

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
	var asJSON, verbose, open bool
	cmd := &cobra.Command{
		Use: "doctor", Short: "Check installation, integrations and data; the only thing it writes is its report", Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if asJSON && open {
				return errors.New(cmdutil.Text("cli.doctor.open_with_json"))
			}
			lang := diagnostics{cmdutil.OutputLocalizer(asJSON), asJSON}
			checks := lang.diagnose(time.Now(), executable, agents...)
			if verbose || !asJSON { // the report lists capabilities too
				attachCapabilities(checks)
			}
			switch {
			case asJSON:
				if err := cmdutil.PrintJSON(checks); err != nil {
					return err
				}
			case verbose && !open:
				lang.writeVerbose(os.Stdout, checks)
			default:
				note := lang.publish(checks, executable)
				if verbose {
					lang.writeVerbose(os.Stdout, checks)
				} else {
					lang.writeSummary(os.Stdout, checks, note)
				}
				if open {
					lang.openNote(note)
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
	cmd.Flags().BoolVar(&verbose, "verbose", false, "print every check in full text, with agent capability details")
	cmd.Flags().BoolVar(&open, "open", false, "open the full HTML report in your default browser")
	return cmd
}

// attachCapabilities adds each agent's capability table to its check.
func attachCapabilities(checks []DoctorCheck) {
	for i := range checks {
		if installer, ok := integrate.LookupAgent(checks[i].Name); ok {
			checks[i].Capabilities = &installer.Capabilities
		}
	}
}

// writeVerbose prints every check, one per line: the output of earlier versions.
func (lang diagnostics) writeVerbose(w io.Writer, checks []DoctorCheck) {
	for _, c := range checks {
		fmt.Fprintf(w, "%-4s %-12s %s\n", cmdutil.Column(lang.Localizer, lang.Label(c.Status)), c.Name, c.Detail)
		if c.Capabilities != nil {
			fmt.Fprintf(w, "  %s\n", initcmd.CapabilityText(*c.Capabilities))
		}
	}
}

// publish saves the HTML report. Failing to write it never fails the check.
func (lang diagnostics) publish(checks []DoctorCheck, executable cmdutil.Executable) reportNote {
	now := time.Now()
	home, _ := userdirs.Data()
	self, _ := executable.Path()
	page, err := lang.renderReport(checks, now, home, self)
	var path string
	if err == nil {
		path, err = saveReport(page, now)
	}
	var missing missingDataDirectory
	switch {
	case errors.As(err, &missing):
		return reportNote{Reason: lang.Text("cli.doctor.report.no_data", nil)}
	case err != nil:
		return reportNote{Reason: err.Error()}
	}
	return reportNote{Path: path}
}

// openNote opens the report, or says why there is none to open.
func (lang diagnostics) openNote(note reportNote) {
	err := errors.New(note.Reason)
	if note.Path != "" {
		if err = openReport(note.Path); err == nil {
			return
		}
	}
	fmt.Fprintln(os.Stderr, lang.Text("cli.warning", map[string]any{"Text": err}))
}

//nolint:funlen // Retain the diagnostic flow while injecting executable discovery and the agent list.
func (lang diagnostics) diagnose(now time.Time, executable cmdutil.Executable, agents ...string) []DoctorCheck {
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
	pathCheck := DoctorCheck{Name: "path", Status: "PASS", Detail: lang.Text("cli.doctor.path_resolves_to_the_current_executable", nil) + path}
	switch {
	case selfErr != nil:
		pathCheck.Status, pathCheck.Detail = "FAIL", selfErr.Error()
	case pathErr != nil:
		pathCheck.Status = "WARN"
		pathCheck.Detail = fmt.Sprintf(lang.Text("cli.doctor.obit_is_not_found_on_path_hooks", nil), self, pathErr)
	default:
		a, err := os.Stat(self)
		if err == nil {
			var b os.FileInfo
			b, err = os.Stat(path)
			if err == nil && !os.SameFile(a, b) {
				pathCheck.Status = "WARN"
				pathCheck.Detail = fmt.Sprintf(lang.Text("cli.doctor.path_resolves_to_hooks_use_an_absolute", nil), path, self)
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
		checks = append(checks, DoctorCheck{Name: "automatic-burial", Status: "INFO", Detail: lang.Text("cli.doctor.graveyard_burial_and_resurrection_use_a_no", nil)})
	}
	daemonErr := selfErr
	if daemonErr == nil {
		daemonErr = lang.regularFile(daemon)
	}
	check("obitd", lang.Text("cli.doctor.companion_executable_exists", nil)+daemon, daemonErr)
	checks = append(checks, lang.diagnoseGUI(daemon, daemonErr, binaryVersion), lang.diagnoseRegistryTransactions(regtxnProbe))
	checks = append(checks, lang.diagnoseShortcut(self, shortcutTarget), lang.diagnoseDesktop(self, shortcutTarget))
	checks = append(checks, lang.diagnoseIcons()...)
	home, homeErr := userdirs.Data()
	if homeErr != nil {
		checks = append(checks, DoctorCheck{Name: "update", Status: "INFO", Detail: homeErr.Error()})
		check("runner", "", homeErr)
		check("tray-startup", "", homeErr)
	} else {
		checks = append(checks, lang.diagnoseUpdate(home))
		checks = append(checks, lang.diagnoseRunner(home))
		startup := lang.diagnoseStartup(daemon, home)
		if problem, err := app.SupervisorProblem(home); err != nil {
			startup.Status, startup.Detail = "FAIL", err.Error()
		} else if problem != "" {
			startup.Status, startup.Detail = "WARN", problem
		}
		checks = append(checks, startup)
	}
	available := 0
	scope := (integrate.Scope{Global: true}).ForInspection()
	inspector := initcmd.NewAgentInspector(scope, executable, lang.Localizer)
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
		checks = append(checks, lang.diagnoseIntegration(agent, inspector))
		if agent != "claude" && agent != "codex" {
			continue // shared installer/guard wiring follows in the next phase
		}
		changes, err := planner.IntegrationPlan(agent, scope, false)
		registered, effective := integrate.GuardRMStatus(changes, agent)
		status, warning := initcmd.DeleteGuardStatus(lang.Localizer, registered, effective)
		check(agent+"-guard-rm", status, err)
		if err == nil && warning {
			checks[len(checks)-1].Status = "WARN"
		}
	}
	checks = append(checks, lang.diagnoseExtension("mcp", executable), lang.diagnoseExtension("skill", executable), lang.diagnoseContextMenu(executable))
	if homeErr != nil {
		for _, name := range []string{"data", "store", "trust", "hook-log", "hook-queue", "graveyard-recovery", "slow-git", "watch", "explorer", "registration-identities", "docker-targets"} {
			check(name, "", homeErr)
		}
	} else {
		checks = append(checks, lang.diagnoseData(home))
		state, err := (ledger.Store{Home: home}).Inspect()
		check("store", lang.Text("cli.doctor.store_json_and_its_referenced_graveyard_snapshot", nil), err)
		checks = append(checks, lang.diagnoseWatches(state, err, now, (&lifecycle.Service{FS: winfs.FS{}}).WatchPath))
		if err == nil {
			checks = append(checks, lang.diagnoseDockerTargets(state))
		} else {
			checks = append(checks, DoctorCheck{Name: "docker-targets", Status: "WARN", Detail: err.Error()})
		}
		checks = append(checks, lang.diagnoseExplorer(home))
		checks = append(checks, lang.diagnoseRecovery(home))
		checks = append(checks, lang.diagnoseVolumeRecovery(home, state)...)
		check("trust", lang.Text("cli.doctor.trusted_rules_json_is_readable_absence_means", nil),
			(rules.TrustStore{Path: filepath.Join(home, "trusted_rules.json")}).Inspect())
		checks = append(checks, lang.diagnoseProjectRules(home)...)
		checks = append(checks, lang.diagnoseHookLog(home, now))
		checks = append(checks, lang.diagnoseHookQueue(home, now))
		checks = append(checks, lang.diagnoseSlowGit(home, now))
		checks = append(checks, lang.diagnoseIdentities(home))
	}
	cfg, err := config.Load()
	detail := lang.Text("cli.doctor.config_toml_is_valid_defaults_apply_when", nil)
	if err == nil && runtime.GOOS != "windows" {
		detail += lang.Text("cli.doctor.watch_file_expiry_mode", map[string]any{"Mode": cfg.Watch.FileExpiry})
	}
	check("config", detail, err)
	checks = append(checks, lang.diagnoseCodexWindowsShell(), lang.diagnoseDocker())
	if available > 0 {
		checks = append(checks, DoctorCheck{Name: "agents", Status: "INFO", Detail: fmt.Sprintf(lang.Text("cli.doctor.additional_agents", nil), available)})
	}
	return checks
}

func (lang diagnostics) diagnoseProjectRules(home string) []DoctorCheck {
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
	return []DoctorCheck{{Name: "project-rules", Status: "PASS", Detail: lang.Text("cli.doctor.project_rules_permit_automatic_registration", nil)}}
}

func (lang diagnostics) diagnoseCodexWindowsShell() DoctorCheck {
	c := DoctorCheck{Name: "codex-windows-shell", Status: "WARN"}
	if runtime.GOOS != "windows" {
		c.Status, c.Detail = "INFO", lang.Text("cli.doctor.native_posix_shell_no_windows_shell_declaration", nil)
		return c
	}
	cfg, err := config.Load()
	if err != nil {
		c.Status, c.Detail = "FAIL", err.Error()
	} else if cfg.WindowsShell("codex") == "" {
		c.Detail = lang.Text("cli.doctor.codex_windows_shell_is_unset_ambiguous_windows", nil)
	} else {
		c.Status = "PASS"
		c.Detail = lang.Text("cli.doctor.codex_windows_shell", nil) + cfg.WindowsShell("codex") + lang.Text("cli.doctor.agent_shells_codex_or_codex_windows_shell", nil)
	}
	return c
}

func (lang diagnostics) diagnoseRecovery(home string) DoctorCheck {
	c := DoctorCheck{Name: "graveyard-recovery", Status: "PASS"}
	pending, err := (&lifecycle.Service{Store: ledger.Store{Home: home}, FS: winfs.FS{}}).PendingGraves()
	c.Detail = fmt.Sprintf(lang.Text("cli.doctor.graveyard_records_need_attention", nil), len(pending))
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

func (lang diagnostics) regularFile(path string) error {
	info, err := os.Stat(path)
	if err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf(lang.Text("cli.doctor.is_not_a_regular_file", nil), path)
	}
	return err
}

func (lang diagnostics) diagnoseHookLog(home string, now time.Time, protectedRoots ...string) DoctorCheck {
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
	c.Detail = fmt.Sprintf(lang.Text("cli.doctor.error_lines_in_the_last_hours_including", nil), c.Errors)
	if permission != "" {
		c.Detail += lang.Text("cli.doctor.a_hook_account_reported_denied_access", nil) + hook.PermissionHint
		c.Detail += fmt.Sprintf(lang.Text("cli.doctor.latest_denied_access_at", nil), permissionAt.Format(time.RFC3339), permission, hook.PermissionReportHint)
	}
	return c
}
