package integrate

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/userdirs"
)

//go:embed skill/SKILL.md
var skillContent []byte

const skillReceiptPrefix = "obituary-skill-v1\n"
const skillPendingPrefix = "obituary-skill-pending-v1\n"

// PlanSkill keeps SKILL.md byte-for-byte intact, including its frontmatter.
// Ownership lives in a sibling receipt; a user-edited file is never replaced
// or removed. The receipt stores the installed version for future upgrades.
func PlanSkill(s Scope, agent string, uninstall bool) ([]Change, error) {
	return planSkillContent(s, agent, skillContent, uninstall)
}

func skillPath(s Scope, agent string) (string, error) {
	var dir string
	var err error
	switch agent {
	case "claude":
		dir = filepath.Join(s.Project, ".claude")
		if s.Global {
			dir, err = userdirs.ClaudeDir()
		}
		dir = filepath.Join(dir, "skills")
	case "codex":
		dir = filepath.Join(s.Project, ".agents", "skills")
		if s.Global {
			dir, err = userdirs.CodexSkillsDir()
		}
	default:
		return "", fmt.Errorf("unsupported agent %q", agent)
	}
	return filepath.Join(dir, "obituary", "SKILL.md"), err
}

func skillReceipt(content []byte) []byte {
	return fmt.Appendf(nil, "%s%x\n", skillReceiptPrefix, sha256.Sum256(content))
}

func planSkillContent(s Scope, agent string, content []byte, uninstall bool) ([]Change, error) {
	path, err := skillPath(s, agent)
	if err != nil {
		return nil, err
	}
	file, err := readIntegrationChange(s, path)
	if err != nil {
		return nil, err
	}
	receipt, err := readIntegrationChange(s, filepath.Join(filepath.Dir(path), ".obituary-skill"))
	if err != nil {
		return nil, err
	}
	ours := file.existed && (bytes.Equal(receipt.orig, skillReceipt(file.orig)) || pendingSkillMatches(receipt.orig, file))
	staleReceipt := !file.existed && (strings.HasPrefix(string(receipt.orig), skillReceiptPrefix) || pendingSkillMatches(receipt.orig, file))
	if (file.existed && !ours) || (receipt.existed && !ours && !staleReceipt) {
		if uninstall {
			file.Detail = "unowned or user-edited skill preserved"
			return []Change{file, receipt}, nil
		}
		return nil, fmt.Errorf("%s is unowned or changed since installation; left untouched", path)
	}
	if uninstall {
		if ours {
			file.Action, file.delete = "remove", true
		}
		if ours || staleReceipt {
			receipt.Action, receipt.delete = "remove", true
		}
	} else {
		file = setIntegrationData(file, bytes.Clone(content))
		final := setIntegrationData(receipt, skillReceipt(content))
		if file.Action != "unchanged" {
			oldHash := "absent"
			if file.existed {
				oldHash = fmt.Sprintf("%x", sha256.Sum256(file.orig))
			}
			pending := fmt.Appendf(nil, "%s%s\n%x\n", skillPendingPrefix, oldHash, sha256.Sum256(content))
			receipt = setIntegrationData(receipt, pending)
			receipt.phase = -1
			final.orig, final.existed, final.Action, final.phase = pending, true, "update", 1
			receipt.next = &final
			return []Change{file, receipt}, nil
		}
		receipt = final
	}
	receipt.phase = 1 // commit the file before its ownership receipt
	return []Change{file, receipt}, nil
}

func pendingSkillMatches(receipt []byte, file Change) bool {
	text, ok := strings.CutPrefix(string(receipt), skillPendingPrefix)
	if !ok {
		return false
	}
	parts := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if len(parts) != 2 {
		return false
	}
	for i, hash := range parts {
		if i == 0 && hash == "absent" {
			continue
		}
		decoded, err := hex.DecodeString(hash)
		if err != nil || len(decoded) != sha256.Size {
			return false
		}
	}
	if !file.existed {
		// An interrupted uninstall can remove either version before removing
		// the valid pending upgrade receipt. No file remains to claim.
		return true
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(file.orig))
	return hash == parts[0] || hash == parts[1]
}
