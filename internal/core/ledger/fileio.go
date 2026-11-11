package ledger

import (
	"context"
	"os"

	"github.com/woodfishhhh/obituary/internal/core/internal/fileio"
	"github.com/woodfishhhh/obituary/internal/core/internal/pathcheck"
)

func LockFile(path string, exclusive bool) (func(), error) { return fileio.LockFile(path, exclusive) }
func TryLockExclusive(path string) (func(), bool, error)   { return fileio.TryLockExclusive(path) }
func lockFile(path string, exclusive bool) (func(), error) { return fileio.LockFile(path, exclusive) }
func tryLockFile(path string, exclusive bool) (func(), bool, error) {
	return fileio.TryLockFile(path, exclusive)
}
func lockFileContext(ctx context.Context, path string, exclusive bool) (func(), error) {
	return fileio.LockFileContext(ctx, path, exclusive)
}
func replaceFile(src, dst string) error           { return fileio.ReplaceFile(src, dst) }
func readShared(path string) ([]byte, error)      { return fileio.ReadShared(path) }
func writeDurably(path string, data []byte) error { return fileio.WriteDurably(path, data) }
func replaceDurably(root *os.Root, tmp, name string, file *os.File) error {
	return fileio.ReplaceDurably(root, tmp, name, file)
}
func syncReplacement(file *os.File) error { return fileio.SyncReplacement(file) }
func syncFile(file *os.File) error        { return fileio.SyncFile(file) }
func writeStoreBytes(file *os.File, data []byte) (int, error) {
	return fileio.WriteStoreBytes(file, data)
}
func samePath(a, b string) bool      { return pathcheck.SamePath(a, b) }
func isInside(path, dir string) bool { return pathcheck.IsInside(path, dir) }
func matchingPaths(paths []string, path string) ([]int, bool) {
	return pathcheck.MatchingPaths(paths, path)
}

func syncParentDirectory(root *os.Root) error { return fileio.SyncParentDirectory(root) }

func SetFileSyncForTesting(fn func(*os.File) error) func() { return fileio.SetFileSyncForTesting(fn) }
