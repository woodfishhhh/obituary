package mcp

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/woodfishhhh/obituary/internal/core"
)

// Paths are native Windows absolute paths. Numeric references are ledger IDs,
// never filenames; a numeric filename is addressed by its absolute path.
func mcpPaths(paths []string, allowIDs bool) error {
	return mcpReferences(paths, allowIDs, false)
}

func mcpReferences(paths []string, allowIDs, burialIDs bool) error {
	if len(paths) == 0 {
		return errors.New("at least one path or ID is required")
	}
	if len(paths) > 100 {
		return errors.New("at most 100 paths or IDs are allowed per call")
	}
	for i, path := range paths {
		if burialIDs {
			_, numeric, err := core.BurialReferenceID(path)
			if err != nil {
				return err
			}
			if numeric {
				continue
			}
		}
		if allowIDs {
			if id, err := strconv.ParseInt(strings.TrimPrefix(path, "#"), 10, 64); err == nil && id > 0 {
				paths[i] = "#" + strconv.FormatInt(id, 10) // strict ID, even if the record disappears concurrently
				continue
			}
		}
		if !filepath.IsAbs(path) {
			return fmt.Errorf("%q: an absolute Windows path is required; relative paths are not resolved against the MCP client's cwd", path)
		}
	}
	return nil
}
