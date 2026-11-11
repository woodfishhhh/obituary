package userdirs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MiniMaxDir resolves mcode's data root; the mmx CLI is a separate product.
// MiniMax-AI/minimax-code 3c21163c packages/tui/src/runtime/data-dir.ts gives
// MINIMAX_DATA_DIR precedence over MAVIS_DATA_DIR and the selected profile.
func MiniMaxDir() (string, error) { return MiniMaxProfileDir("") }

// MiniMaxProfileDir takes the host's explicit --profile selection. Obituary
// never runs the host's legacy .mavis migration or guesses its process flags.
func MiniMaxProfileDir(profile string) (string, error) {
	for _, name := range []string{"MINIMAX_DATA_DIR", "MAVIS_DATA_DIR"} {
		if dir := strings.TrimSpace(os.Getenv(name)); dir != "" {
			return directoryAbs(dir)
		}
	}
	if strings.ContainsAny(profile, `/\:<>"|?*`) || strings.TrimSpace(profile) != profile || strings.ContainsAny(profile, "\x00\r\n") || strings.HasSuffix(profile, ".") {
		return "", fmt.Errorf("MiniMax profile must be a single stable directory suffix")
	}
	home, err := Home()
	if err != nil {
		return "", err
	}
	name := ".minimax"
	if profile != "" {
		name += "-" + profile
	}
	return directoryAbs(filepath.Join(home, name))
}

// MiniMaxInstallDir is the native CLI distribution directory, independent of
// the data root and profile (MiniMax Code CLI configuration reference).
func MiniMaxInstallDir() (string, error) {
	if dir := os.Getenv("MCODE_INSTALL_DIR"); dir != "" {
		return directoryAbs(dir)
	}
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".minimax-code"), nil
}

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "minimax-code", Resolve: MiniMaxDir})
}
