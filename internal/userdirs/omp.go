package userdirs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// OMPDir resolves the default profile's native Windows agent root.
// Source: can1357/oh-my-pi packages/utils/src/dirs.ts at
// 21f1188cf88b2f534c092181968cd5a4d1a9c296. PI_CONFIG_DIR is home-relative;
// PI_CODING_AGENT_DIR is a full override. Named profiles have separate override
// precedence and are deliberately rejected until their installation is supported.
func OMPDir() (string, error) {
	profile, set := os.LookupEnv("OMP_PROFILE")
	if !set {
		profile = os.Getenv("PI_PROFILE")
	}
	if profile = strings.TrimSpace(profile); profile != "" && profile != "default" {
		return "", fmt.Errorf("OMP named profiles are not supported by this installer")
	}
	if dir := os.Getenv("PI_CODING_AGENT_DIR"); dir != "" {
		return directoryAbs(dir)
	}
	home, err := Home()
	if err != nil {
		return "", err
	}
	name := os.Getenv("PI_CONFIG_DIR")
	if name == "" {
		name = ".omp"
	}
	if !filepath.IsLocal(name) {
		return "", fmt.Errorf("PI_CONFIG_DIR must be a home-relative directory name")
	}
	// Check the joined spelling before Abs can trim a final dot or space.
	root := filepath.Join(home, name)
	if err := pathutil.Unchanged(root); err != nil {
		return "", err
	}
	return directoryAbs(filepath.Join(root, "agent"))
}

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "omp", Resolve: OMPDir})
}
