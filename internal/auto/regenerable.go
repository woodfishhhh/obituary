package auto

import (
	"path/filepath"
	"strings"
)

// regenerableNames are names of what a package manager, build or tool
// recreates on demand. A session leftover is proposed only when Git ignores
// it and its name is listed here: untracked files that are not ignored are
// uncommitted work, and ignored is not the same as disposable (.env,
// .claude/settings.local.json, terraform.tfstate, local databases). Every
// leftovers proposal must be one the agent can recommend approving; anything
// else stays unmanaged unless a rule or `obit mark` gives it a lifespan.
var regenerableNames = map[string]bool{
	// dependencies
	"node_modules": true, "bower_components": true, ".pnpm-store": true, ".venv": true, "venv": true,
	// build output
	"dist": true, "build": true, "out": true, "target": true, "bin": true, "obj": true,
	".output": true, ".next": true, ".nuxt": true, ".svelte-kit": true, ".turbo": true,
	".angular": true, ".docusaurus": true, "storybook-static": true,
	// caches
	".cache": true, ".parcel-cache": true, ".vite": true, ".tanstack": true, ".eslintcache": true,
	"coverage": true, ".nyc_output": true, "htmlcov": true, ".gradle": true, ".dart_tool": true,
	".tox": true, ".nox": true, ".hypothesis": true,
}

var regenerableSuffixes = []string{".tsbuildinfo", ".pyc"}

func regenerable(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	if regenerableNames[name] {
		return true
	}
	for _, suffix := range regenerableSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}
