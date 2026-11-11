package fileio

import "context"

func TryLockFile(path string, exclusive bool) (func(), bool, error) {
	return tryLockFile(path, exclusive)
}

func LockFileContext(ctx context.Context, path string, exclusive bool) (func(), error) {
	return lockFileContext(ctx, path, exclusive)
}

func ReplaceFile(src, dst string) error { return replaceFile(src, dst) }
