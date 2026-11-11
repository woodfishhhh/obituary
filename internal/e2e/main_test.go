//go:build windows

package e2e_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/woodfishhhh/obituary/internal/testenv"
)

var binaries func() (string, error)

func TestMain(m *testing.M) {
	if binary := os.Getenv("OBIT_E2E_ARGV_BINARY"); binary != "" {
		// This child inherits the already isolated fixture. Record native argv
		// at the process boundary, then pass it unchanged to the real CLI.
		data, _ := json.Marshal(os.Args[1:])
		if err := os.WriteFile(os.Getenv("OBIT_DELETE_ARGV_CAPTURE"), data, 0o600); err != nil {
			panic(err)
		}
		cmd := exec.Command(binary, os.Args[1:]...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				os.Exit(exit.ExitCode())
			}
			panic(err)
		}
		os.Exit(0)
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		panic(err)
	}
	// testenv.Main establishes isolation before the first fixture calls this
	// builder. Its temporary root owns the binaries and their cleanup, too.
	binaries = sync.OnceValues(func() (string, error) {
		dir, err := os.MkdirTemp("", "obit-e2e-bin-")
		if err != nil {
			return "", err
		}
		for _, name := range []string{"obit", "obitd"} {
			args := []string{"build", "-o", filepath.Join(dir, name+".exe")}
			if name == "obitd" {
				args = append(args, "-ldflags=-H=windowsgui -X main.defaultTray=false")
			}
			if overlay := os.Getenv("OBIT_E2E_OVERLAY"); overlay != "" {
				args = append(args, "-overlay", overlay)
			}
			cmd := exec.Command("go", append(args, "./cmd/"+name)...)
			cmd.Dir = root
			if out, err := cmd.CombinedOutput(); err != nil {
				return "", fmt.Errorf("build failed (%s): %w\n%s", name, err, out)
			}
		}
		return dir, nil
	})
	os.Exit(testenv.Main(m))
}
