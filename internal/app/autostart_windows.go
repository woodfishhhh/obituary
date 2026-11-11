package app

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/runkey"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	"golang.org/x/sys/windows"
)

// RunRegistry is injected in unit tests; subprocess tests use a disposable key.
type RunRegistry = runkey.Registry

func startupName(home string) string {
	sum := sha256.Sum256([]byte(strings.ToUpper(filepath.Clean(home))))
	return fmt.Sprintf("Obituary-%x", sum[:16])
}

// AutostartPlan preserves foreign values and rechecks the original at Apply.
type AutostartPlan struct {
	Registry            RunRegistry
	Name, Before, After string
	before, after       runkey.Value
}

func PlanAutostart(store RunRegistry, daemon, home, config string, enabled bool) (*AutostartPlan, error) {
	command := windows.ComposeCommandLine([]string{daemon, "--tray", "--home", home, "--config-dir", config})
	name := startupName(home)
	before, err := store.Read(name)
	if err != nil {
		return nil, err
	}
	if before.Exists && !ownedStartup(before.Text, home, config) {
		return nil, fmt.Errorf("preserving unowned Run value %s: command differs from this installation", name)
	}
	after := runkey.Value{}
	if enabled {
		after = runkey.StringValue(command)
	}
	return &AutostartPlan{Registry: store, Name: name, Before: before.Text, After: after.Text, before: before, after: after}, nil
}

// Like integrate's owned hook commands, recognize the complete command shape.
// The executable directory may move when an installation is refreshed.
func ownedStartup(command, home, config string) bool {
	args, err := windows.DecomposeCommandLine(command)
	return err == nil && len(args) == 6 && filepath.IsAbs(args[0]) &&
		strings.EqualFold(filepath.Base(args[0]), daemonName) && args[1] == "--tray" &&
		args[2] == "--home" && args[3] == home && args[4] == "--config-dir" && args[5] == config
}

func (p *AutostartPlan) Pending() bool { return p.before != p.after }

func (p *AutostartPlan) Apply() error {
	release, err := p.Registry.Lock()
	if err != nil {
		return err
	}
	defer release()
	return p.Registry.CompareAndSwap(p.Name, p.before, p.after)
}

func SetAutostart(home string, enabled bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	path := filepath.Join(filepath.Dir(exe), daemonName)
	if enabled {
		if _, err := os.Stat(path); err != nil {
			return err
		}
	}
	config, err := userdirs.Config()
	if err != nil {
		return err
	}
	plan, err := PlanAutostart(runkey.Current, path, home, config, enabled)
	if err != nil {
		return err
	}
	return plan.Apply()
}

func Autostart() string {
	home, err := userdirs.Data()
	if err != nil {
		return ""
	}
	value, _ := runkey.Current.Get(startupName(home))
	return value
}
