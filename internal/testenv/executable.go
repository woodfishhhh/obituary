package testenv

import (
	"os"
	"syscall"
	"testing"
)

// WriteExecutable writes a program the test is about to run. Linux refuses to
// exec a file that any process holds open for writing ("text file busy"), and
// a child forked by a parallel test keeps every inherited descriptor until its
// own exec (golang/go#22315). Holding ForkLock while the file is open keeps
// forks out of that window; Windows does not use the lock.
func WriteExecutable(t testing.TB, path string, data []byte) {
	t.Helper()
	if err := withoutForks(func() error { return os.WriteFile(path, data, 0o700) }); err != nil {
		t.Fatal(err)
	}
}

// withoutForks runs write, which must open and close the file it creates.
func withoutForks(write func() error) error {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	return write()
}
