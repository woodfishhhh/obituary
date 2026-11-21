package update

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/testenv"
)

// A fake release program is a file that ends with "\nOBIT-FAKE:<behavior>\n".
// Small files are read by fakeRun; copies of the test binary with the trailer
// run for real, and answer --version as the trailer says.
const fakeMarker = "\nOBIT-FAKE:"

func TestMain(m *testing.M) {
	if behavior, ok := fakeBehavior(); ok {
		os.Exit(runFake(behavior, os.Args[1:]))
	}
	os.Exit(testenv.Main(m))
}

func fakeBehavior() (string, bool) {
	exe, err := os.Executable()
	if err != nil {
		return "", false
	}
	f, err := os.Open(exe)
	if err != nil {
		return "", false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", false
	}
	tail := make([]byte, min(info.Size(), 128))
	if _, err := f.ReadAt(tail, info.Size()-int64(len(tail))); err != nil {
		return "", false
	}
	return trailerBehavior(tail)
}

func trailerBehavior(data []byte) (string, bool) {
	i := bytes.LastIndex(data, []byte(fakeMarker))
	if i < 0 {
		return "", false
	}
	behavior, _, _ := strings.Cut(string(data[i+len(fakeMarker):]), "\n")
	return behavior, true
}

func runFake(behavior string, args []string) int {
	if behavior == "crash" {
		return 3
	}
	if version, ok := strings.CutPrefix(behavior, "version="); ok && len(args) == 1 && args[0] == "--version" {
		fmt.Println("obit version", version)
	}
	return 0
}
