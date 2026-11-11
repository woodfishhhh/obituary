package regtxn_test

import (
	"testing"

	"github.com/woodfishhhh/obituary/internal/regtxn"
)

func TestProbeEmptyTransaction(t *testing.T) {
	if err := regtxn.Probe(); err != nil {
		t.Fatalf("empty KTM transaction: %v", err)
	}
}
