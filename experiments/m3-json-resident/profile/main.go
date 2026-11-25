// Command profile CPU-profiles the resident's observation replay against a
// seeded benchmark home (the one bench.py leaves behind): it queues N new
// writes of the benchmark repository and replays them the way obitd does.
//
//	OBIT_HOME=<bench home> OBIT_CONFIG_DIR=<bench config> profile <repo> <N> <cpu.prof>
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/userdirs"
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
	repo, n := os.Args[1], 0
	n, _ = strconv.Atoi(os.Args[2])
	svc, err := app.Open()
	must(err)
	svc.AfterChange = nil
	svc.Store = svc.Store.WithResidentReplay()
	for i := range n {
		path := filepath.Join(repo, fmt.Sprintf("profile-%d-%d.log", time.Now().UnixNano(), i))
		must(os.WriteFile(path, []byte("x"), 0o600))
		rows, err := svc.CaptureObservations([]string{path}, fmt.Sprintf("profile-%d", i))
		must(err)
		must(svc.Store.DeferObservations(rows))
	}
	f, err := os.Create(os.Args[3])
	must(err)
	must(pprof.StartCPUProfile(f))
	start := time.Now()
	must(app.Engine(svc).ReplayObservations())
	pprof.StopCPUProfile()
	must(f.Close())
	fmt.Printf("replayed %d requests in %s\n", n, time.Since(start))
}
