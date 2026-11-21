package main

import (
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/cli/doctor"
	"github.com/woodfishhhh/obituary/internal/cli/graveyard"
	"github.com/woodfishhhh/obituary/internal/cli/list"
	"github.com/woodfishhhh/obituary/internal/cli/purge"
	"github.com/woodfishhhh/obituary/internal/cli/stats"
	"github.com/woodfishhhh/obituary/internal/cli/sweep"
	"os"
	"time"
)

func main() {
	out := os.Stdout
	dev, e := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if e != nil {
		panic(e)
	}
	os.Stdout = dev
	for _, v := range []struct {
		n    string
		f    func() *cobra.Command
		args []string
	}{{"list", list.NewCmd, nil}, {"stats", stats.NewCmd, nil}, {"graveyard", graveyard.NewCmd, []string{"--json"}}, {"doctor", doctor.NewCmd, []string{"--json"}}, {"reap", sweep.NewCmd, nil}, {"purge", purge.NewCmd, nil}} {
		for i := 0; i < 6; i++ {
			c := v.f()
			c.SetArgs(append([]string{}, v.args...))
			c.SilenceUsage = true
			c.SilenceErrors = true
			t := time.Now()
			e := c.Execute()
			if e != nil && !(v.n == "doctor" && errors.Is(e, cmdutil.ErrReported)) {
				panic(e)
			}
			fmt.Fprintln(out, v.n, float64(time.Since(t))/1e6)
		}
	}
}
