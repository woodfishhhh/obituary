package docker

import (
	"os/exec"
	"strings"
	"testing"
	"unicode"
)

func TestErrorSummarySanitizedAndBounded(t *testing.T) {
	err := &exec.ExitError{Stderr: []byte("\x1b[31mpermission\x1b[0m\r\n denied\x00 " + strings.Repeat("字", 800))}
	summary := errorSummary(err)
	if !strings.Contains(summary, "permission denied") || len([]rune(summary)) > 515 {
		t.Fatal("stderr summary invalid", summary)
	}
	for _, r := range summary {
		if unicode.IsControl(r) {
			t.Fatal("control character preserved", r)
		}
	}
}
