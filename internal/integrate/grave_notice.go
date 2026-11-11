package integrate

// Notice hooks run for every tool. Existing guard registrations retain their
// opt-in; a new default installation uses the explicit advisory-only flag.
func graveNoticeCommand(s Scope, path, agent, command string) (string, error) {
	ch, err := readIntegrationChange(s, path)
	if err != nil {
		return "", err
	}
	registered, _ := GuardRMStatus([]Change{ch}, agent)
	if !s.GuardRM && !registered {
		command += " --notice-only"
	}
	return command, nil
}
