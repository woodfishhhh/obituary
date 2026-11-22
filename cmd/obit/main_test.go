package main

import (
	"bytes"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == "catalog-init-trace" {
		return // Package initializers have run; no test environment or child process is needed.
	}
	os.Exit(testenv.Main(m))
}

func TestRootHelpReadsCurrentLanguage(t *testing.T) {
	root := newRoot(nil)
	var out bytes.Buffer
	root.SetOut(&out)
	for _, tc := range []struct{ lang, want string }{{"zh-CN", "按到期时间显示受保护的路径"}, {"en", "Show tracked paths"}} {
		t.Setenv("OBIT_LANG", tc.lang)
		out.Reset()
		if err := root.Help(); err != nil || !strings.Contains(out.String(), tc.want) {
			t.Fatalf("root help did not render %s: %v", tc.lang, err)
		}
	}
}

func TestHookRootOnlyRegistersHookCommands(t *testing.T) {
	root := newRoot([]string{"hook", "claude"})
	commands := root.Commands()
	if len(commands) != 1 || commands[0].Name() != "hook" {
		t.Fatalf("hook root registered unrelated commands: %v", commands)
	}
}

func TestStartupLeavesCatalogUnparsed(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), exe, "catalog-init-trace")
	cmd.Env = append(os.Environ(), "GODEBUG=inittrace=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("startup trace: %v: %s", err, out)
	}
	m := regexp.MustCompile("(?m)^init github.com/woodfishhhh/obituary/internal/i18n @[^\n]+ clock, ([0-9]+) bytes,").FindSubmatch(out)
	if m == nil {
		t.Fatal("missing i18n startup trace")
	}
	n, err := strconv.Atoi(string(m[1]))
	if err != nil || n > 1<<20 {
		t.Fatalf("catalog parsed during startup: %s bytes", m[1])
	}
}
