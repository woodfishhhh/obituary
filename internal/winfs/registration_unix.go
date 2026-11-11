//go:build linux || darwin

package winfs

import corefs "github.com/woodfishhhh/obituary/internal/core/fs"

// POSIX descriptors do not provide the Windows delete-sharing exclusion used
// by this ancestry proof. Keep the existing full registration lookup instead.
func (FS) OpenFileAncestry(string, corefs.Identity) (corefs.FileAncestry, error) {
	return nil, corefs.ErrUnsafe
}
