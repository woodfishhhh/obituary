//go:build linux || darwin

package winfs

import "github.com/woodfishhhh/obituary/internal/core"

// POSIX descriptors do not provide the Windows delete-sharing exclusion used
// by this ancestry proof. Keep the existing full registration lookup instead.
func (FS) OpenFileAncestry(string, core.Identity) (core.FileAncestry, error) {
	return nil, core.ErrUnsafe
}
