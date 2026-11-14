package auto

import (
	"path/filepath"
	"strings"
)

// regenerableNames are names of what a package manager, build or tool
// recreates on demand: dependencies, build output and caches. Not being
// tracked by Git does not make them worthless (a project's node_modules is in
// use), so automatic registration never registers or proposes them, nor
// anything inside them; only an explicit `obit mark` gives them a lifespan.
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
	".tox": true, ".nox": true, ".hypothesis": true, "__pycache__": true, ".pytest_cache": true,
	".mypy_cache": true, ".ruff_cache": true,
}

var regenerableSuffixes = []string{".tsbuildinfo", ".pyc"}

// NeverAutomatic returns the first dependency, build output or cache folder
// on the way from root down to path (path included), or "": automatic
// registration leaves such a path alone.
func NeverAutomatic(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ""
	}
	dir := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		dir = filepath.Join(dir, part)
		if part != "." && regenerable(dir) {
			return dir
		}
	}
	return ""
}

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
