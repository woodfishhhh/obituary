//go:build windows

package daemonipc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"strings"

	"github.com/Microsoft/go-winio"
)

func endpoint(home string) string {
	sum := sha256.Sum256([]byte(home))
	return strings.Join([]string{"\\.", "pipe", "obituary-resident-" + hex.EncodeToString(sum[:])}, "\\")
}

func listen(home string) (net.Listener, func() error, error) {
	listener, err := winio.ListenPipe(endpoint(home), nil)
	return listener, func() error { return nil }, err
}

func dial(ctx context.Context, home string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, endpoint(home))
}
