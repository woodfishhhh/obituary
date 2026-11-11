package buildinfo

import (
	"strings"
	"testing"
)

func TestDefaultVersionIsDevelopmentBuild(t *testing.T) {
	if !strings.Contains(Version, "-dev") {
		t.Fatalf("default version=%q", Version)
	}
}
