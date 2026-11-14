package hook

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// stop runs a Claude Stop event and returns the block reason ("" when silent).
func (e *env) stop(session string, active bool) string {
	e.t.Helper()
	in, _ := json.Marshal(map[string]any{"hook_event_name": "Stop", "session_id": session, "cwd": e.root, "stop_hook_active": active})
	var out bytes.Buffer
	Claude(e.engine, bytes.NewReader(in), &out, &Runner{Finish: func(string, string, bool) {}})
	if out.Len() == 0 {
		return ""
	}
	var reply struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal(out.Bytes(), &reply); err != nil || reply.Decision != "block" || reply.Reason == "" {
		e.t.Fatalf("Stop printed something other than a block decision: %q", out.String())
	}
	return reply.Reason
}

func newCleanupEnv(t *testing.T) *env {
	e := newEnv(t)
	e.engine.Svc.SessionCleanup = true
	return e
}

// The Stop request lists only files the agent created in this session that
// nothing else accounts for, and is made once per session.
func TestStopAsksOnceAboutTheAgentsOwnUnsortedFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newCleanupEnv(t)
	notes := e.write("notes.md") // the user's, untracked before the session
	tracked := e.write("README.md")
	gitCommit(t, e.root, "README.md")
	e.event(map[string]any{"hook_event_name": "SessionStart", "source": "startup"})
	if reason := e.stop("s1", false); reason != "" {
		t.Fatalf("nothing written yet, got %q", reason)
	}
	for _, path := range []string{e.write("scratch.py"), e.write("debug.log"), notes, tracked} {
		e.wrote(path)
	}
	e.write("unseen.txt") // created, but no hook saw the agent write it
	reason := e.stop("s1", false)
	for _, want := range []string{"scratch.py", "obit mark <path> 2d", "obit bury <path>", "once per session"} {
		if !strings.Contains(reason, want) {
			t.Fatalf("cleanup request lacks %q: %q", want, reason)
		}
	}
	for _, unwanted := range []string{"notes.md", "README.md", "debug.log", "unseen.txt"} {
		if strings.Contains(reason, unwanted) {
			t.Fatalf("cleanup request names %s, which is not the agent's unsorted file: %q", unwanted, reason)
		}
	}
	e.wrote(e.write("later.py"))
	if again := e.stop("s1", false); again != "" {
		t.Fatalf("a session is asked once, got %q", again)
	}
}

func TestStopCleanupStaysQuiet(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, tc := range []string{"disabled", "continuing", "nothing unsorted"} {
		t.Run(tc, func(t *testing.T) {
			e := newCleanupEnv(t)
			e.event(map[string]any{"hook_event_name": "SessionStart", "source": "startup"})
			switch tc {
			case "disabled":
				e.engine.Svc.SessionCleanup = false
				e.wrote(e.write("scratch.py"))
			case "continuing":
				e.wrote(e.write("scratch.py"))
			case "nothing unsorted":
				e.wrote(e.write("debug.log")) // registered by rule
			}
			if reason := e.stop("s1", tc == "continuing"); reason != "" {
				t.Fatalf("%s: Stop must stay silent, got %q", tc, reason)
			}
		})
	}
}

// A host that cannot be asked to continue hears about its session's files at
// the next session start in the project, once.
func TestCleanupFallsBackToTheNextSessionStart(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newCleanupEnv(t)
	agent := Agent{Name: "plugin", Adapter: BridgeAdapter{}}
	var out bytes.Buffer
	RunEvent(agent, e.engine, Event{Name: "SessionStart", Cwd: e.root, Session: "plugin:s"}, &out, &Runner{})
	RunEvent(agent, e.engine, Event{Name: "PostToolUse", Cwd: e.root, Session: "plugin:s", Paths: []string{e.write("scratch.py")}}, &out, &Runner{})
	text, err := Handle(e.engine, Event{Name: "Stop", Cwd: e.root, Session: "plugin:s"}, &Runner{Finish: func(string, string, bool) {}})
	if err != nil || text != "" {
		t.Fatalf("a host without continuation stays silent at Stop: %q %v", text, err)
	}
	next := e.event(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "s2"})
	if !strings.Contains(next, "earlier agent session") || !strings.Contains(next, "scratch.py") || !strings.Contains(next, "obit mark <path> 2d") {
		t.Fatalf("the next session start must hear of the earlier files, got %q", next)
	}
	if again := e.event(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "s3"}); strings.Contains(again, "scratch.py") {
		t.Fatalf("told once, got %q", again)
	}
}

func gitCommit(t *testing.T, root string, rels ...string) {
	t.Helper()
	for _, args := range [][]string{append([]string{"add", "-f", "--"}, rels...), {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "c"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
}
