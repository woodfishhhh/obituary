package ledger_test

import (
	"testing"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

func TestObjectKindClassification(t *testing.T) {
	cases := []struct {
		kind     string
		wantFile bool
	}{
		{"", true},
		{ledger.KindContainer, false},
		{ledger.KindDockerVolume, false},
		{"podman-container", false},
	}
	for _, tc := range cases {
		it := ledger.Item{Kind: tc.kind}
		if it.IsFile() != tc.wantFile || it.IsExternal() == tc.wantFile {
			t.Fatalf("Item{Kind:%q}: IsFile=%v IsExternal=%v wantFile=%v", tc.kind, it.IsFile(), it.IsExternal(), tc.wantFile)
		}
		b := ledger.Burial{Kind: tc.kind}
		if b.IsFile() != tc.wantFile || b.IsExternal() == tc.wantFile {
			t.Fatalf("Burial{Kind:%q}: IsFile=%v IsExternal=%v wantFile=%v", tc.kind, b.IsFile(), b.IsExternal(), tc.wantFile)
		}
		c := ledger.Candidate{Kind: tc.kind}
		if c.IsFile() != tc.wantFile || c.IsExternal() == tc.wantFile {
			t.Fatalf("Candidate{Kind:%q}: IsFile=%v IsExternal=%v wantFile=%v", tc.kind, c.IsFile(), c.IsExternal(), tc.wantFile)
		}
	}
}
