package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/winfs"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func ck(e error) {
	if e != nil {
		panic(e)
	}
}
func ms(d time.Duration) float64 { return float64(d) / 1e6 }

type hooked struct {
	core.FS
	once  sync.Once
	enter chan struct{}
	id    core.Identity
}

func (h *hooked) Locate(id core.Identity) (string, error) {
	if id == h.id {
		h.once.Do(func() { close(h.enter) })
	}
	return h.FS.Locate(id)
}
func main() {
	home := os.Getenv("OBIT_HOME")
	if !strings.Contains(home, "audit-6926") {
		panic(home)
	}
	s := core.Store{Home: home}
	base := filepath.Dir(home)
	op := os.Args[1]
	f := winfs.FS{}
	now := time.Now()
	svc, e := app.Open()
	ck(e)
	svc.AfterChange = nil
	switch op {
	case "seed":
		n, _ := strconv.Atoi(os.Args[2])
		isBig := n >= 1000
		root := filepath.Join(base, "data")
		ck(os.MkdirAll(root, 0700))
		st := core.State{NextID: int64(2*n + 1), Projects: map[string]core.ProjectState{core.ProjectKey(os.Args[3]): {Consent: "approved"}}}
		for i := 0; i < n; i++ {
			p := filepath.Join(root, fmt.Sprintf("live-%05d", i))
			ck(os.WriteFile(p, nil, 0600))
			id, e := f.Identify(p)
			ck(e)
			st.Items = append(st.Items, core.Item{ID: int64(i + 1), Rev: 1, Source: "cli", Path: p, Identity: &id, CreatedAt: now, ExpiresAt: now.Add(time.Hour)})
			if !isBig {
				continue
			}
			j := int64(n + i + 1)
			g := filepath.Join(home, "graveyard", fmt.Sprintf("%d-%s-abcdef", j, now.UTC().Format("20060102T150405Z")), "buried")
			ck(os.MkdirAll(filepath.Dir(g), 0700))
			ck(os.WriteFile(g, nil, 0600))
			gid, e := f.Identify(filepath.Dir(g))
			ck(e)
			obj, e := f.Identify(g)
			ck(e)
			st.Burials = append(st.Burials, core.Burial{ID: j, Path: filepath.Join(root, fmt.Sprintf("old-%d", j), "buried"), Grave: g, Identity: &obj, GraveID: &gid, BuriedAt: now, PurgeAt: now.Add(time.Hour * 24), Reason: core.ReasonBury})
		}
		ck(s.Update(func(v *core.State) error { *v = st; return nil }))
		fmt.Println(n, len(st.Burials))
	case "phase":
		for i := 0; i < 21; i++ {
			for _, x := range []string{"config", "open", "hot", "full", "write"} {
				t := time.Now()
				switch x {
				case "config":
					_, e = config.Load()
				case "open":
					_, e = app.Open()
				case "hot":
					_, e = s.ReadHot()
				case "full":
					_, e = s.ReadSnapshot()
				case "write":
					e = s.UpdateHot(func(v *core.State) error { v.Items[0].Rev++; return nil })
				}
				ck(e)
				fmt.Println(x, ms(time.Since(t)))
			}
		}
	case "tick":
		for i := 0; i < 6; i++ {
			t := time.Now()
			a, b, e := app.Tick(svc)
			ck(e)
			fmt.Println("tick", ms(time.Since(t)), len(a), len(b))
			time.Sleep(200 * time.Millisecond)
		}
	case "replay":
		ck(app.ReplayObservations(svc))
	case "medium":
		st, e := s.ReadHot()
		ck(e)
		p := filepath.Join(base, "empty-dir")
		for i := 0; i < 6; i++ {
			ck(os.MkdirAll(p, 0700))
			h := &hooked{FS: f, id: *st.Items[0].Identity, enter: make(chan struct{})}
			svc.FS = h
			done := make(chan error, 1)
			go func() {
				r, e := svc.Bury([]string{p}, false, "")
				if e == nil && (len(r) != 1 || r[0].Error != "") {
					e = fmt.Errorf("rows: %+v", r)
				}
				done <- e
			}()
			select {
			case <-h.enter:
			case e := <-done:
				panic(fmt.Sprint("no scan ", e))
			}
			t := time.Now()
			err := s.WithLock("store.lock", false, func() error { return nil })
			ck(err)
			fmt.Println("reader", ms(time.Since(t)))
			ck(<-done)
			time.Sleep(150 * time.Millisecond)
		}
	case "resident":
		r, e := app.NewResident(svc, log.New(io.Discard, "", 0))
		ck(e)
		defer r.Close()
		var steps atomic.Int32
		ctx, cancel := context.WithCancel(context.Background())
		r.Changed = func() { steps.Add(1) }
		done := make(chan error, 1)
		go func() { done <- r.Run(ctx) }()
		for steps.Load() == 0 {
			time.Sleep(10 * time.Millisecond)
		}
		fmt.Println("ready")
		var q string
		fmt.Scanln(&q)
		fmt.Println("idle", steps.Load()-1)
		if q == "burst" {
			start := time.Now()
			for i := 0; i < 1000; i++ {
				ck(s.UpdateHot(func(v *core.State) error { v.NextID++; return nil }))
			}
			elapsed := ms(time.Since(start))
			time.Sleep(900 * time.Millisecond)
			fmt.Println("burst", elapsed, steps.Load()-1)
			st, e := s.ReadHot()
			ck(e)
			fmt.Println("final-next", st.NextID)
		}
		cancel()
		ck(<-done)
	case "due":
		ck(s.UpdateHot(func(v *core.State) error {
			for i := range v.Items {
				if v.Items[i].ID == 1 {
					v.Items[i].ExpiresAt = now.Add(-time.Hour)
					return nil
				}
			}
			return errors.New("no seed")
		}))
	case "arm":
		ck(s.Update(func(v *core.State) error {
			for i := range v.Burials {
				if v.Burials[i].ID == 1 {
					v.Burials[i].PurgeAt = now.Add(-time.Hour)
					return nil
				}
			}
			return errors.New("no grave")
		}))
	case "resetone":
		p := filepath.Join(base, "data", "live-00000")
		ck(os.WriteFile(p, nil, 0600))
		id, e := f.Identify(p)
		ck(e)
		ck(s.UpdateHot(func(v *core.State) error {
			if v.Index(1) >= 0 {
				return errors.New("still alive")
			}
			v.Items = append(v.Items, core.Item{ID: 1, Rev: 1, Source: "cli", Path: p, Identity: &id, ExpiresAt: now.Add(time.Hour), CreatedAt: now})
			return nil
		}))
	}
}
