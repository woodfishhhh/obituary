package userdirs

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// ReceiptPath keeps generated metadata in the data directory. ASCII folding
// preserves the existing shortcut keys without conflating Unicode names.
func ReceiptPath(path, directory, suffix string) (string, error) {
	abs, err := pathutil.Abs(path)
	if err != nil {
		return "", err
	}
	home, err := Data()
	if err != nil {
		return "", err
	}
	key := strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, filepath.Clean(abs))
	return filepath.Join(home, directory, fmt.Sprintf("%x%s", sha256.Sum256([]byte(key)), suffix)), nil
}
