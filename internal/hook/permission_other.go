//go:build !windows

package hook

import (
	"strings"
)

const permissionMarker = "; permission_diagnostic="

// permissionDetails has no token or SID details to add outside Windows.
func permissionDetails(error, string, string) string { return "{}" }

// PermissionSummary reports whether a hook.log line records a denied access.
func PermissionSummary(line string) (string, bool) {
	before, _, ok := strings.Cut(line, permissionMarker)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(before), true
}
