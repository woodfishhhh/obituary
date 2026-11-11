package app

import (
	"os"
	"testing"

	"github.com/woodfishhhh/obituary/internal/testenv"
)

var beforeTests = func() {}

func TestMain(m *testing.M) {
	beforeTests()
	os.Exit(testenv.Main(m))
}
