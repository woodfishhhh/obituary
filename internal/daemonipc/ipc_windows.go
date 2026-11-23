//go:build windows

package daemonipc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

func endpoint(home string) string {
	sum := sha256.Sum256([]byte(home))
	return `\\.\pipe\obituary-resident-` + hex.EncodeToString(sum[:])
}

func listen(home string) (net.Listener, func() error, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, nil, err
	}
	sid := user.User.Sid.String()
	listener, err := winio.ListenPipe(endpoint(home), &winio.PipeConfig{SecurityDescriptor: "O:" + sid + "D:P(A;;GA;;;" + sid + ")"})
	return listener, func() error { return nil }, err
}

func dial(ctx context.Context, home string) (net.Conn, error) {
	conn, err := winio.DialPipeContext(ctx, endpoint(home))
	if err != nil {
		return nil, err
	}
	if err := verifyPipeOwner(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func verifyPipeOwner(conn net.Conn) error {
	f, ok := conn.(interface{ Fd() uintptr })
	if !ok {
		return errors.New("resident pipe handle unavailable")
	}
	sd, err := windows.GetSecurityInfo(windows.Handle(f.Fd()), windows.SE_KERNEL_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	if !owner.Equals(user.User.Sid) {
		return errors.New("resident pipe owner differs from current user")
	}
	return nil
}
