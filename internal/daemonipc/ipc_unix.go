//go:build !windows

package daemonipc

import (
	"context"
	"net"
	"os"
	"path/filepath"
)

func endpoint(home string) string { return filepath.Join(home, "resident.sock") }

func listen(home string) (net.Listener, func() error, error) {
	path := endpoint(home)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = listener.Close()
		_ = os.Remove(path)
		return nil, nil, err
	}
	return listener, func() error { return os.Remove(path) }, nil
}

func dial(ctx context.Context, home string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "unix", endpoint(home))
}
