//go:build !windows

package daemonipc

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

func endpoint(home string, create bool) (string, error) {
	path := filepath.Join(home, "resident.sock")
	if len(path) < len(unix.RawSockaddrUnix{}.Path) {
		return path, nil
	}
	// sockaddr_un cannot hold a long Home. Both processes use the same private directory.
	digest := sha256.Sum256([]byte(filepath.Clean(home)))
	dir := fmt.Sprintf("/tmp/obituary-ipc-%d", os.Geteuid())
	if create {
		if err := os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode().Perm() != 0700 || int(info.Sys().(*syscall.Stat_t).Uid) != os.Geteuid() {
		return "", fmt.Errorf("resident socket directory is not private: %s", dir)
	}
	return filepath.Join(dir, fmt.Sprintf("%x.sock", digest[:16])), nil
}

func listen(home string) (net.Listener, func() error, error) {
	path, err := endpoint(home, true)
	if err != nil {
		return nil, nil, err
	}
	if info, statErr := os.Lstat(path); statErr == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, nil, errors.New("resident endpoint is not a socket")
		}
		err = os.Remove(path) // only the resident holding the Home lock can replace a stale socket
	} else if !errors.Is(statErr, os.ErrNotExist) {
		err = statErr
	}
	if err != nil {
		return nil, nil, err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = listener.Close()
		return nil, nil, err
	}
	return listener, func() error { return nil }, nil // UnixListener unlinks on Close
}

func dial(ctx context.Context, home string) (net.Conn, error) {
	path, err := endpoint(home, false)
	if err != nil {
		return nil, err
	}
	return (&net.Dialer{}).DialContext(ctx, "unix", path)
}
