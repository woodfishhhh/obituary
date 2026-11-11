package update

import (
	"strings"

	"golang.org/x/mod/semver"
)

func version(value string) string { return "v" + strings.TrimPrefix(value, "v") }

func stable(value string) bool {
	v := version(value)
	return semver.IsValid(v) && semver.Prerelease(v) == "" && strings.Count(strings.SplitN(v, "+", 2)[0], ".") == 2
}

// Newer compares development builds against their base release number so a
// development build of 1.0.0 does not prompt for the same 1.0.0 release.
func Newer(current, latest string) bool {
	current = version(current)
	if !semver.IsValid(current) || !stable(latest) {
		return false
	}
	if pre := semver.Prerelease(current); strings.HasPrefix(pre, "-dev") {
		current = strings.SplitN(current, "-", 2)[0]
	}
	return semver.Compare(version(latest), current) > 0
}
