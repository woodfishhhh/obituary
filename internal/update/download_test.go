package update

import (
	"reflect"
	"testing"
)

func TestApplyUsesPinnedScriptURLs(t *testing.T) {
	p := plan{version: "1.4.0", channel: "script", source: "mirror"}
	if got := p.scriptURLs("install.ps1"); !reflect.DeepEqual(got, []string{
		"https://cn.obituary.woodfish.site/download/v1.4.0/install.ps1",
		"https://raw.githubusercontent.com/woodfishhhh/obituary/v1.4.0/packaging/install.ps1",
	}) {
		t.Fatal(got)
	}
	p.source = "github"
	if got := p.scriptURLs("install.sh"); !reflect.DeepEqual(got, []string{"https://raw.githubusercontent.com/woodfishhhh/obituary/v1.4.0/install.sh"}) {
		t.Fatal(got)
	}
}
