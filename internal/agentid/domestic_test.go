package agentid_test

import (
	"reflect"
	"testing"

	"github.com/woodfishhhh/obituary/internal/agentid"
)

func TestDomesticSlugs(t *testing.T) {
	t.Parallel()
	want := []string{"codebuddy", "codebuddy-ide", "workbuddy", "qoder", "qoder-cn", "lingma", "qoderwork", "zcode", "minimax-code", "deepseek-harness"}
	got := agentid.DomesticSlugs()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("slugs = %v", got)
	}
	got[0] = "modified"
	if !reflect.DeepEqual(agentid.DomesticSlugs(), want) {
		t.Fatal("caller changed the constant table")
	}
}
