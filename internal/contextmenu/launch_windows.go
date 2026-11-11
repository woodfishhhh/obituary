package contextmenu

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

// Run is called only by obitd --context-menu, never by a daemon tick or hook.
// Explorer uses Document selection: one process per selected item (up to its
// 15-item legacy-verb limit). This entry also accepts multiple literal paths.
func Run(args []string, execute func(string, []string) ([]byte, error), notify func(string, string) error) error {
	command, err := menuCommand(args)
	if err != nil {
		return err
	}
	output, runErr := execute(args[0], command)
	language := config.UILanguage()
	title := language.Text(i18n.OperationComplete)
	if runErr != nil {
		title = language.Text(i18n.OperationIncomplete)
	}
	message := strings.TrimSpace(string(output))
	if message == "" {
		message = language.Text(i18n.OperationDone)
		if runErr != nil {
			message = runErr.Error()
		}
	}
	return errors.Join(runErr, notify(title, message))
}

func menuCommand(args []string) ([]string, error) {
	if len(args) < 3 {
		return nil, errors.New("context menu requires an executable, action and path")
	}
	if !filepath.IsAbs(args[0]) || !strings.EqualFold(filepath.Base(args[0]), "obit.exe") {
		return nil, errors.New("context menu requires the absolute obit.exe path")
	}
	switch args[1] {
	case "1h", "1d", "7d", "30d":
		command := append([]string{"mark", "--"}, args[2:]...)
		return append(command, args[1]), nil
	case "resurrect", "policy":
		return append([]string{"context-menu", args[1], "--"}, args[2:]...), nil
	default:
		return nil, fmt.Errorf("unknown context menu action %q", args[1])
	}
}

// Execute uses Go's Windows argv quoting, never cmd.exe or PowerShell.
func Execute(executable string, args []string) ([]byte, error) {
	cmd := exec.Command(executable, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	return cmd.CombinedOutput()
}
