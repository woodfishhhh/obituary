package testenv_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/woodfishhhh/obituary/internal/testenv"
)

// Parallel tests fork constantly. Without ForkLock about half of these execs
// failed with "text file busy" on Linux; with it none can.
func TestWriteExecutableRunsWhileOthersFork(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("text file busy is a Linux exec error")
	}
	if testing.Short() {
		t.Skip("starts hundreds of processes")
	}
	data, err := os.ReadFile("/bin/true")
	if err != nil {
		t.Skip("no /bin/true:", err)
	}
	var stop atomic.Bool
	var forks sync.WaitGroup
	for range 4 {
		forks.Go(func() {
			for !stop.Load() {
				_ = exec.Command("/bin/true").Run()
			}
		})
	}
	defer forks.Wait()
	defer stop.Store(true)
	dir := t.TempDir()
	for i := range 100 {
		path := filepath.Join(dir, fmt.Sprint(i))
		testenv.WriteExecutable(t, path, data)
		if err := exec.Command(path).Run(); err != nil {
			t.Fatalf("run %d: %v (text file busy: %t)", i, err, errors.Is(err, syscall.ETXTBSY))
		}
	}
}
