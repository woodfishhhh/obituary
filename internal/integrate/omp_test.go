package integrate

import "testing"

func TestOMPInstallationLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	testPluginLifecycle(t, "omp")
}

func TestOMPSkillTargets(t *testing.T) {
	checkAgentSkillTargets(t, "omp")
}

func TestOMPSkillSharedOwners(t *testing.T) {
	checkSharedSkillOwners(t, Scope{Project: t.TempDir()}, "codex", "omp")
}
