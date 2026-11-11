package integrate

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillDirectoryContainsOnlyOwnedFiles(t *testing.T) {
	for _, agent := range []string{"claude", "codex"} {
		t.Run(agent, func(t *testing.T) {
			s := Scope{Project: t.TempDir()}
			path, err := skillPath(s, agent)
			if err != nil {
				t.Fatal(err)
			}
			for _, content := range [][]byte{[]byte("old"), skillContent, skillContent} {
				plan, err := planSkillContent(s, agent, content, false)
				applyIntegration(t, plan, err)
				entries, err := os.ReadDir(filepath.Dir(path))
				if err != nil || len(entries) != 2 || entries[0].Name() != ".obituary-skill" || entries[1].Name() != "SKILL.md" {
					t.Fatal("auxiliary files in skill directory", entries, err)
				}
			}
			if agent == "codex" {
				writeIntegrationFile(t, path+".lock", "")
			}
			plan, err := PlanSkill(s, agent, false)
			applyIntegration(t, plan, err)
			plan, err = PlanSkill(s, agent, true)
			applyIntegration(t, plan, err)
			entries, err := os.ReadDir(filepath.Dir(path))
			if err != nil || len(entries) != 0 {
				t.Fatal("owned files remain after uninstall", entries, err)
			}
		})
	}
}

func TestSkillOrphanedUpgradeReceipt(t *testing.T) {
	t.Parallel()
	for _, uninstall := range []bool{false, true} {
		t.Run(fmt.Sprint(uninstall), func(t *testing.T) {
			t.Parallel()
			scope := Scope{Project: t.TempDir()}
			path, err := skillPath(scope, "codex")
			if err != nil {
				t.Fatal(err)
			}
			receipt := filepath.Join(filepath.Dir(path), ".obituary-skill")
			writeIntegrationFile(t, receipt, fmt.Sprintf("%s%x\n%x\n", skillPendingPrefix, sha256.Sum256([]byte("old")), sha256.Sum256([]byte("new"))))
			plan, err := PlanSkill(scope, "codex", uninstall)
			if err != nil {
				t.Fatalf("orphaned upgrade receipt rejected: %v", err)
			}
			applyIntegration(t, plan, nil)
			if uninstall {
				if _, err := os.Stat(receipt); !os.IsNotExist(err) {
					t.Fatal("orphaned upgrade receipt retained")
				}
			} else if read(t, path) != string(skillContent) {
				t.Fatal("orphaned upgrade could not reinstall")
			}
		})
	}
}

func TestSkillInterruptedWritesRemainOwned(t *testing.T) {
	t.Parallel()
	for _, agent := range []string{"claude", "codex"} {
		t.Run(agent, func(t *testing.T) {
			t.Parallel()
			for _, upgrade := range []bool{false, true} {
				for _, step := range []string{".obituary-skill", "SKILL.md"} {
					t.Run(fmt.Sprintf("upgrade=%t/%s", upgrade, step), func(t *testing.T) {
						t.Parallel()
						scope := Scope{Project: t.TempDir()}
						if upgrade {
							plan, err := planSkillContent(scope, agent, []byte("previous"), false)
							applyIntegration(t, plan, err)
						}
						plan, err := PlanSkill(scope, agent, false)
						if err != nil {
							t.Fatal(err)
						}
						stop := errors.New("process interrupted")
						afterIntegrationWrite := func(path string) error {
							if filepath.Base(path) == step {
								return stop
							}
							return nil
						}

						if err := apply(plan, afterIntegrationWrite); !errors.Is(err, stop) {
							t.Fatalf("interruption not reached: %v", err)
						}
						plan, err = PlanSkill(scope, agent, false)
						if err != nil {
							t.Fatalf("interrupted write lost ownership: %v", err)
						}
						applyIntegration(t, plan, nil)
						st, err := InspectExtension(scope, agent, "skill", "obit")
						if err != nil || !st.Current {
							t.Fatalf("interrupted write did not recover: %+v %v", st, err)
						}
					})
				}
			}
		})
	}
}

func TestSkillRecoversPendingCommit(t *testing.T) {
	t.Parallel()
	for _, agent := range []string{"claude", "codex"} {
		t.Run(agent, func(t *testing.T) {
			t.Parallel()
			for _, old := range []string{"", "previous skill"} {
				for _, step := range []string{"prepared", "written", "edited"} {
					for _, uninstall := range []bool{false, true} {
						t.Run(fmt.Sprintf("old=%t/%s/remove=%t", old != "", step, uninstall), func(t *testing.T) {
							t.Parallel()
							scope := Scope{Project: t.TempDir()}
							path, err := skillPath(scope, agent)
							if err != nil {
								t.Fatal(err)
							}
							newContent := []byte("interrupted upgrade")
							previous := "absent"
							if old != "" {
								previous = fmt.Sprintf("%x", sha256.Sum256([]byte(old)))
							}
							receipt := fmt.Sprintf("obituary-skill-pending-v1\n%s\n%x\n", previous, sha256.Sum256(newContent))
							writeIntegrationFile(t, filepath.Join(filepath.Dir(path), ".obituary-skill"), receipt)
							if step == "written" {
								writeIntegrationFile(t, path, string(newContent))
							} else if step == "edited" {
								writeIntegrationFile(t, path, "user content")
							} else if old != "" {
								writeIntegrationFile(t, path, old)
							}
							plan, err := PlanSkill(scope, agent, uninstall)
							if step == "edited" {
								if !uninstall && err == nil {
									t.Fatal("pending receipt claimed user edit")
								}
								if uninstall {
									if agent == "codex" {
										applyPreservedSkill(t, plan, err)
									} else {
										if err != nil || Pending(plan) || len(plan) == 0 || !strings.Contains(plan[0].Detail, "preserved") {
											t.Fatalf("edited skill preservation plan: %+v %v", plan, err)
										}
										applyIntegration(t, plan, nil)
									}
								}
								if read(t, path) != "user content" || read(t, filepath.Join(filepath.Dir(path), ".obituary-skill")) != receipt {
									t.Fatal("pending receipt removed user edit")
								}
								return
							}
							if err != nil {
								t.Fatalf("interrupted skill cannot recover: %v", err)
							}
							applyIntegration(t, plan, nil)
							if uninstall {
								if _, err := os.Stat(path); !os.IsNotExist(err) {
									t.Fatalf("recoverable skill not removed: %v", err)
								}
							} else if read(t, path) != string(skillContent) {
								t.Fatal("interrupted skill not upgraded")
							}
						})
					}
				}
			}
		})
	}
}

func TestSkillLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, agent := range []string{"claude", "codex"} {
		for _, global := range []bool{false, true} {
			t.Run(agent+map[bool]string{true: "-global", false: "-project"}[global], func(t *testing.T) {
				root := t.TempDir()
				t.Setenv("OBIT_USER_HOME", root)
				t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(root, ".claude"))
				scope := Scope{Global: global, Project: root}
				plan, err := planSkillContent(scope, agent, []byte("---\nname: obituary\n---\nold version\n"), false)
				applyIntegration(t, plan, err)
				path := plan[0].Path
				folder := ".claude"
				if agent == "codex" {
					folder = ".agents"
				}
				if path != filepath.Join(root, folder, "skills", "obituary", "SKILL.md") {
					t.Fatal("wrong discovery path", path)
				}
				st, err := InspectExtension(scope, agent, "skill", "obit")
				if err != nil || !st.Installed || st.Current {
					t.Fatalf("old status: %+v %v", st, err)
				}
				plan, err = PlanSkill(scope, agent, false)
				applyIntegration(t, plan, err)
				if read(t, path) != string(skillContent) {
					t.Fatal("embedded content changed")
				}
				plan, err = PlanSkill(scope, agent, false)
				if err != nil || Pending(plan) {
					t.Fatalf("not idempotentent: %+v %v", plan, err)
				}
				st, err = InspectExtension(scope, agent, "skill", "obit")
				if err != nil || !st.Current {
					t.Fatalf("current status: %+v %v", st, err)
				}
				plan, err = PlanSkill(scope, agent, true)
				applyIntegration(t, plan, err)
				for _, change := range plan {
					if _, err := os.Stat(change.Path); !os.IsNotExist(err) {
						t.Fatal("owned file remains", change.Path, err)
					}
				}
			})
		}
	}
}

func TestSkillPreservesUserEdits(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	scope := Scope{Project: t.TempDir()}
	plan, err := PlanSkill(scope, "codex", false)
	applyIntegration(t, plan, err)
	path := plan[0].Path
	writeIntegrationFile(t, path, string(skillContent)+"\nUser additions\n")
	if _, err := PlanSkill(scope, "codex", false); err == nil {
		t.Fatal("overwrote user-edited skill")
	}
	plan, err = PlanSkill(scope, "codex", true)
	applyPreservedSkill(t, plan, err)
	if !strings.Contains(read(t, path), "User additions") {
		t.Fatal("uninstall removed user-edited skill")
	}
	scope = Scope{Project: t.TempDir()}
	path, err = skillPath(scope, "codex")
	if err != nil {
		t.Fatal(err)
	}
	writeIntegrationFile(t, path, string(skillContent))
	plan, err = PlanSkill(scope, "codex", true)
	applyPreservedSkill(t, plan, err)
	if read(t, path) != string(skillContent) {
		t.Fatal("unowned copy removed")
	}
}

func TestSkillRejectsConcurrentEditAndLinks(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, agent := range []string{"claude", "codex"} {
		t.Run(agent, func(t *testing.T) {
			t.Parallel()
			scope := Scope{Project: t.TempDir()}
			plan, err := PlanSkill(scope, agent, false)
			if err != nil {
				t.Fatal(err)
			}
			writeIntegrationFile(t, plan[0].Path, "saved while planning")
			if err := Apply(plan); err == nil {
				t.Fatal("concurrent edit accepted")
			}
			other := t.TempDir()
			scope = Scope{Project: t.TempDir()}
			folder := ".claude"
			if agent == "codex" {
				folder = ".agents"
			}
			makeIntegrationJunction(t, filepath.Join(scope.Project, folder), other)
			if _, err := PlanSkill(scope, agent, false); err == nil {
				t.Fatal("skill parent link accepted")
			}
			entries, err := os.ReadDir(other)
			if err != nil || len(entries) != 0 {
				t.Fatalf("outside changed: %v %v", entries, err)
			}
		})
	}
}
