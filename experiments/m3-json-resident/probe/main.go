// Command probe seeds and pokes a throw-away Obituary home for the
// resident-single-writer evaluation. It refuses any home that is not a
// benchmark directory, so it can never touch an installed ledger.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	home, err := userdirs.Data()
	must(err)
	if !strings.Contains(filepath.ToSlash(home), "/obit-resident-bench-") {
		panic("not a benchmark home: " + home)
	}
	store := ledger.Store{Home: home}
	switch os.Args[1] {
	case "seed": // seed N <repo> [real]: N live items and, from 1000 up, N buried entries
		n, _ := strconv.Atoi(os.Args[2])
		seed(store, home, n, os.Args[3], len(os.Args) > 4 && os.Args[4] == "real")
	case "due": // move item 1's expiry: one targeted write by a short process
		must(store.UpdateHot(func(st *ledger.State) error {
			for i := range st.Items {
				if st.Items[i].ID == 1 {
					st.Items[i].ExpiresAt = time.Now().Add(-time.Hour)
					return nil
				}
			}
			return fmt.Errorf("item 1 missing")
		}))
	case "replay": // what a one-shot obitd does with the queue
		svc, err := app.Open()
		must(err)
		svc.AfterChange = nil
		must(app.ReplayObservations(svc))
	}
}

// seed writes the benchmark ledger. Bare burials are what the earlier gate runs used; with real,
// each burial carries what `obit bury` writes (expanded long names, a parent chain, the deletion
// operation and its origin), so list reads the ledger a real user has.
func seed(store ledger.Store, home string, n int, repo string, real bool) {
	fs, now := winfs.FS{}, time.Now()
	root := filepath.Join(filepath.Dir(home), "data")
	must(os.MkdirAll(root, 0o700))
	st := ledger.State{NextID: int64(2*n + 1), Projects: map[string]ledger.ProjectState{ledger.ProjectKey(repo): {Consent: ledger.ConsentApproved}}}
	for i := 0; i < n; i++ {
		p := filepath.Join(root, fmt.Sprintf("live-%05d", i))
		must(os.WriteFile(p, nil, 0o600))
		id, err := fs.Identify(p)
		must(err)
		st.Items = append(st.Items, ledger.Item{ID: int64(i + 1), Rev: 1, Source: "cli", Path: p, Identity: &id, CreatedAt: now, ExpiresAt: now.Add(time.Hour)})
		if n < 1000 {
			continue
		}
		j := int64(n + i + 1)
		g := filepath.Join(home, "graveyard", fmt.Sprintf("%d-%s-abcdef", j, now.UTC().Format("20060102T150405Z")), "buried")
		must(os.MkdirAll(filepath.Dir(g), 0o700))
		must(os.WriteFile(g, nil, 0o600))
		gid, err := fs.Identify(filepath.Dir(g))
		must(err)
		obj, err := fs.Identify(g)
		must(err)
		b := ledger.Burial{ID: j, Path: filepath.Join(root, fmt.Sprintf("old-%d", j), "buried"), Grave: g, Identity: &obj, GraveID: &gid, BuriedAt: now, PurgeAt: now.Add(24 * time.Hour), Reason: ledger.ReasonBury}
		if real {
			b.LongNames, b.Source, b.Project, b.Session = true, "cli", repo, "bench-session"
			b.Operation = &ledger.BurialOperation{ID: fmt.Sprintf("batch-%04d", i/80), StartedAt: now, Project: repo, Session: "bench-session"}
			for dir := filepath.Dir(b.Path); len(b.Parents) < 4 && dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
				b.Parents = append(b.Parents, ledger.BurialParent{Path: dir, Identity: gid})
			}
		}
		st.Burials = append(st.Burials, b)
	}
	must(store.Update(func(v *ledger.State) error { *v = st; return nil }))
	fmt.Println(n, len(st.Burials))
}
