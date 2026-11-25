//go:build !windows

package cmdutil

// SkipExplorerCheck: Cobra's double-click check exists only on Windows.
func SkipExplorerCheck() {}
