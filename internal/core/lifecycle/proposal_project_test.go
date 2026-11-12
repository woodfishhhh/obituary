package lifecycle_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestDeletedProposalProjectRequiresAccessibleOrdinaryAncestors(t *testing.T) {
	for _, mode := range []string{"deleted", "volume-offline", "different-volume", "ancestor-denied", "existing", "metadata-missing"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			project := filepath.Join(f.dir, "parent", "missing", "repo")
			if err := os.MkdirAll(filepath.Dir(filepath.Dir(project)), 0700); err != nil {
				t.Fatal(err)
			}
			if mode == "existing" || mode == "metadata-missing" {
				if err := os.MkdirAll(project, 0700); err != nil {
					t.Fatal(err)
				}
			}
			p, _, _, err := f.svc.Propose(core.ProposalLeftovers, project, "s", []core.Candidate{{Path: filepath.Join(project, "candidate.log"), Lifespan: "1d"}})
			if err != nil {
				t.Fatal(err)
			}
			before, _ := f.svc.Store.Read()
			fs := proposalProjectFS{FS: f.svc.FS, volume: pathutil.VolumeRoot(project), ancestor: filepath.Dir(filepath.Dir(project)), mode: mode}
			f.svc.FS = fs
			err = f.svc.PruneDeletedProposal(p)
			after, readErr := f.svc.Store.Read()
			if readErr != nil {
				t.Fatal(readErr)
			}
			if mode == "deleted" {
				if err != nil || len(after.Proposals) != 0 {
					t.Fatalf("missing project retained: %+v %v", after, err)
				}
			} else if err == nil || !reflect.DeepEqual(before.Proposals, after.Proposals) {
				t.Fatalf("uncertain proposal pruned: %+v %v", after, err)
			}
			if len(after.Items) != 0 || len(after.Burials) != 0 || len(after.Projects) != 0 || len(after.LeftoverRejections) != 0 {
				t.Fatalf("automatic decision: %+v", after)
			}
		})
	}
}

type proposalProjectFS struct {
	core.FS
	volume, ancestor, mode string
}

func (f proposalProjectFS) Identify(path string) (core.Identity, error) {
	if filepath.Clean(path) == filepath.Clean(f.volume) && f.mode == "volume-offline" {
		return core.Identity{}, core.ErrNotFound
	}
	if path == f.ancestor && f.mode == "ancestor-denied" {
		return core.Identity{}, os.ErrPermission
	}
	id, err := f.FS.Identify(path)
	if path == f.ancestor && f.mode == "different-volume" {
		id.Serial++
	}
	return id, err
}

func TestDeletedProposalProjectRetainsJunctionAndUnreadableAncestor(t *testing.T) {
	if testing.Short() {
		t.Skip("uses Windows junction and ACL fixtures")
	}
	if runtime.GOOS != "windows" {
		t.Skip("Windows junction and ACL fixtures")
	}
	for _, mode := range []string{"junction", "nested-junction", "unreadable"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			parent := filepath.Join(f.dir, "parent")
			if mode == "unreadable" {
				if err := os.Mkdir(parent, 0700); err != nil {
					t.Fatal(err)
				}
			} else {
				target := filepath.Join(f.dir, "target")
				if err := os.Mkdir(target, 0700); err != nil {
					t.Fatal(err)
				}
				if out, err := exec.Command("cmd", "/c", "mklink", "/J", parent, target).CombinedOutput(); err != nil {
					t.Fatalf("junction: %v %s", err, out)
				}
				if mode == "nested-junction" {
					parent = filepath.Join(parent, "ordinary")
					if err := os.Mkdir(parent, 0700); err != nil {
						t.Fatal(err)
					}
				}
			}
			project := filepath.Join(parent, "missing")
			p, _, _, err := f.svc.Propose(core.ProposalLeftovers, project, "s", []core.Candidate{{Path: filepath.Join(project, "candidate"), Lifespan: "1d"}})
			if err != nil {
				t.Fatal(err)
			}
			if mode == "unreadable" {
				testenv.DenyEnumeration(t, parent)
			}
			err = f.svc.PruneDeletedProposal(p)
			t.Logf("ancestor diagnosis: %v", err)
			st, readErr := f.svc.Store.Read()
			if err == nil || readErr != nil || len(st.Proposals) != 1 || len(st.Items) != 0 || len(st.Burials) != 0 {
				t.Fatalf("unsafe ancestor pruned: %+v %v %v", st, err, readErr)
			}
		})
	}
}

func TestDeletedProposalPruningPreservesRefreshedRevision(t *testing.T) {
	f := newFixture(t)
	project := filepath.Join(f.dir, "missing")
	c := core.Candidate{Path: filepath.Join(project, "candidate"), Lifespan: "1d"}
	p, _, _, err := f.svc.Propose(core.ProposalLeftovers, project, "s", []core.Candidate{c})
	if err != nil {
		t.Fatal(err)
	}
	c.Lifespan = "2d"
	if _, _, _, err := f.svc.Propose(p.Kind, project, "s", []core.Candidate{c}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.PruneDeletedProposal(p); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 || len(st.Proposals[0].Candidates) != 1 || st.Proposals[0].Candidates[0].Lifespan != "2d" || len(st.Items) != 0 {
		t.Fatalf("deleted project lost refreshed revision: %+v %v", st, err)
	}
	events, err := f.svc.Store.History(20)
	if err != nil || len(events) != 0 {
		t.Fatalf("unchanged proposal audited as deleted: %+v %v", events, err)
	}
}

func TestDeletedProposalProjectRecheckPreservesRecreatedProject(t *testing.T) {
	f := newFixture(t)
	project := filepath.Join(f.dir, "missing")
	p, _, _, err := f.svc.Propose(core.ProposalLeftovers, project, "s", []core.Candidate{{Path: filepath.Join(project, "candidate"), Lifespan: "1d"}})
	if err != nil {
		t.Fatal(err)
	}
	if deleted, err := f.svc.ProposalProjectDeleted(project); !deleted || err != nil {
		t.Fatalf("absence: %v %v", deleted, err)
	}
	if err := os.Mkdir(project, 0700); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.PruneDeletedProposal(p); err == nil {
		t.Fatal("recreated project pruned")
	}
	st, err := f.svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 || len(st.Items) != 0 {
		t.Fatalf("recreated proposal changed: %+v %v", st, err)
	}
	if _, err := os.Stat(project); errors.Is(err, os.ErrNotExist) {
		t.Fatal("project moved")
	}
}
