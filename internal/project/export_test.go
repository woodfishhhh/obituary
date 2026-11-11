package project

import "time"

func SetGitTimeout(d time.Duration) func() {
	previous := gitTimeout
	gitTimeout = d
	return func() { gitTimeout = previous }
}
