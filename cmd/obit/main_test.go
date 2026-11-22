package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestMain(m *testing.M) {
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
