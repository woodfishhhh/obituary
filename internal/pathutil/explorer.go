package pathutil

import "strings"

// IsExplorerMetadata identifies display infrastructure which must never be
// registered by a folder policy or projected into its own display labels.
func IsExplorerMetadata(name string) bool {
	name = strings.ToLower(name)
	return name == "desktop.ini" || strings.HasPrefix(name, ".obit-labels-") && strings.HasSuffix(name, ".tmp")
}
