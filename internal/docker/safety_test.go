package docker

import (
	"testing"
	"time"
)

func TestStopTimeoutIsIsolated(t *testing.T) {
	for _, timeout := range []time.Duration{0, time.Second} {
		cli := CLI{Timeout: timeout}
		stop := cli.forStop(Container{StopTimeout: new(-1)})
		want := timeout
		if want == 0 {
			want = 24 * time.Hour
		}
		if cli.Timeout != timeout || stop.Timeout != want {
			t.Fatal("stop timeout escaped command copy", cli.Timeout, stop.Timeout)
		}
	}
}

func TestContainerStopWait(t *testing.T) {
	for _, tc := range []struct {
		name    string
		seconds *int
		os      string
		want    time.Duration
	}{
		{"windows", nil, "windows", 35 * time.Second},
		{"linux", nil, "linux", 15 * time.Second},
		{"custom", new(90), "windows", 95 * time.Second},
		{"immediate", new(0), "linux", 5 * time.Second},
		{"infinite", new(-1), "windows", 24 * time.Hour},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := Container{Identity: Identity{Target: &Target{OSType: tc.os}}, StopTimeout: tc.seconds}
			if got := c.StopWait(); got != tc.want {
				t.Fatal(got, tc.want)
			}
		})
	}
}
