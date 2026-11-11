package update

import (
	"strings"
	"testing"
)

func TestChannel(t *testing.T) {
	const packages = `C:\Users\test\AppData\Local\Microsoft\WinGet\Packages`
	for _, tc := range []struct {
		exe     string
		receipt bool
		want    string
	}{
		{`C:\Users\test\scoop\apps\obituary\current\obit.exe`, false, "scoop"},
		{`D:\SCOOP\APPS\OBITUARY\1.0.0\obitd.exe`, true, "scoop"},
		{packages + `\woodfishhhh.Obituary_abc\obit-gui.exe`, false, "winget"},
		{packages + `\other.Obituary_abc\obit.exe`, false, "other"},
		{`D:\Microsoft\WinGet\Packages\woodfishhhh.Obituary_abc\obit.exe`, false, "other"},
		{`C:\scoop\apps\obituary-extra\obit.exe`, false, "other"},
		{`C:\scoop\apps\obituary\..\other\obit.exe`, false, "other"},
		{`C:\Programs\obituary\obit.exe`, true, "script"},
		{`D:\source\bin\obit.exe`, false, "other"},
	} {
		t.Run(tc.exe, func(t *testing.T) {
			if got := Channel(tc.exe, packages, tc.receipt); got != tc.want {
				t.Fatalf("channel=%s want %s", got, tc.want)
			}
		})
	}
}

func TestCommand(t *testing.T) {
	if got := Command("scoop", "1.0.1", ""); got != "scoop update obituary" {
		t.Fatal(got)
	}
	if got := Command("winget", "1.0.1", ""); got != "winget upgrade woodfishhhh.Obituary" {
		t.Fatal(got)
	}
	if got := Command("other", "1.0.1", ""); got != "" {
		t.Fatal(got)
	}
	got := Command("script", "v1.0.1", `D:\O'Brien\obit.exe`)
	if !strings.Contains(got, `-Version 1.0.1`) || !strings.Contains(got, `-InstallDir 'D:/O''Brien'`) {
		t.Fatal(got)
	}
	if got := Command("script", "1.0.1;bad", ""); got != "" {
		t.Fatal(got)
	}
}
