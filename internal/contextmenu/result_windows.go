package contextmenu

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func runSelection(args []string, execute func(string, []string) ([]byte, error), notify func(string, string) error) error {
	language := config.UILanguage()
	var report strings.Builder
	var failures []error
	completed := 0
	// One literal operand per CLI call preserves its rules and avoids the
	// native command-line limit, even for large selections of long paths.
	for _, path := range args[2:] {
		command, _ := menuCommand([]string{args[0], args[1], path})
		output, err := execute(args[0], command)
		fmt.Fprintf(&report, "%s\n%s\n", path, strings.TrimSpace(string(output)))
		if err != nil {
			fmt.Fprintln(&report, err)
			failures = append(failures, fmt.Errorf("%s: %w", path, err))
		} else {
			completed++
		}
	}
	message := selectionSummary(language, args[1], completed, len(failures))
	result, err := saveSelectionResult(message + "\n\n" + report.String())
	if err != nil {
		failures = append(failures, err)
		message += "\n" + err.Error()
	} else {
		message += "\n" + fmt.Sprintf(language.Text(i18n.MenuResultDetails), result)
	}
	title := i18n.OperationComplete
	if len(failures) != 0 {
		title = i18n.OperationIncomplete
	}
	return errors.Join(errors.Join(failures...), notify(language.Text(title), message))
}

func selectionSummary(language i18n.Language, action string, completed, failed int) string {
	if action == "bury" {
		return fmt.Sprintf(language.Text(i18n.MenuBuried), completed, failed)
	}
	duration := map[string]i18n.Key{"1h": i18n.OneHour, "1d": i18n.OneDay, "7d": i18n.SevenDays, "30d": i18n.ThirtyDays}[action]
	return fmt.Sprintf(language.Text(i18n.MenuMarked), completed, language.Text(duration), failed)
}

func saveSelectionResult(text string) (string, error) {
	home, err := userdirs.Data()
	if err != nil {
		return "", err
	}
	path := filepath.Join(home, "context-menu", "last-result.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	release, err := core.LockFile(path+".lock", true)
	if err != nil {
		return "", err
	}
	defer release()
	return path, core.WriteMetadata(path, []byte(text))
}
