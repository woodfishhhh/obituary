package integrate

import "strconv"

func PlanUpdateCheckConfig(path string, enabled bool) (Change, error) {
	return planConfigValue(path, "update", "check", strconv.FormatBool(enabled))
}
