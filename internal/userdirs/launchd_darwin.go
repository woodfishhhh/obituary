package userdirs

import "path/filepath"

// LaunchAgents is per user and follows the same home override as agent configs.
func LaunchAgents() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents"), nil
}
