package docker

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"unicode"
)

var terminalEscape = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

func errorSummary(err error) string {
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return ""
	}
	text := terminalEscape.ReplaceAllString(string(exit.Stderr), "")
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text)
	runes := []rune(strings.Join(strings.Fields(text), " "))
	if len(runes) > 512 {
		runes = append(runes[:512], []rune("…")...)
	}
	if len(runes) == 0 {
		return ""
	}
	return ": " + string(runes)
}

// Diagnostic keeps daemon details bounded and gives the user an actionable next step.
func Diagnostic(err error) string {
	hint := "Start Docker Desktop or the Docker daemon and check the selected endpoint."
	text := strings.ToLower(err.Error())
	switch {
	case errors.Is(err, exec.ErrNotFound), errors.Is(err, os.ErrNotExist):
		hint = "Install Docker CLI and ensure docker is on PATH."
	case errors.Is(err, context.DeadlineExceeded):
		hint = "Docker timed out; check daemon responsiveness and endpoint connectivity."
	case strings.Contains(text, "permission"), strings.Contains(text, "access is denied"), strings.Contains(text, "access denied"):
		hint = "Docker access denied; check socket permissions and Docker Desktop access."
	}
	return err.Error() + "; " + hint
}
