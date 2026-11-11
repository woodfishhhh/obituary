package integrate

import "strconv"

func PlanTrayConfig(path string, enabled bool) (Change, error) {
	return planConfigValue(path, "ui", "tray", strconv.FormatBool(enabled))
}

func PlanTaskbarConfig(path string, enabled bool) (Change, error) {
	return planConfigValue(path, "ui", "taskbar", strconv.FormatBool(enabled))
}
