package main

import (
	"fmt"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/winfs"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {
	home := os.Getenv("OBIT_HOME")
	if !strings.Contains(home, "audit-6926") {
		panic(home)
	}
	s := core.Store{Home: home}
	st, e := s.ReadHot()
	if e != nil {
		panic(e)
	}
	fs := winfs.FS{}
	for i := 0; i < 7; i++ {
		for k := 0; k < 4; k++ {
			n := []int{1, 2, 4, 8}[(i+k)%4]
			t := time.Now()
			var wg sync.WaitGroup
			wg.Add(n)
			loc := make([]string, len(st.Items))
			for j := 0; j < n; j++ {
				go func(j int) {
					defer wg.Done()
					for a := j; a < len(st.Items); a += n {
						v, e := fs.Locate(*st.Items[a].Identity)
						if e != nil {
							panic(e)
						}
						loc[a] = v
					}
				}(j)
			}
			wg.Wait()
			ms := float64(time.Since(t)) / 1e6
			for k, v := range loc {
				if !strings.EqualFold(st.Items[k].Path, v) {
					panic(v)
				}
			}
			fmt.Println(n, ms)
			time.Sleep(125 * time.Millisecond)
		}
	}
}
