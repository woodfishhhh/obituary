package hook

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/project"
	"golang.org/x/sys/windows"
)

func TestDiscoveryDenialUsesSkipLogWithoutPermissionDiagnostic(t *testing.T) {
	home := t.TempDir()
	original := findProject
	findProject = func(string) (project.Project, bool, error) {
		return project.Project{}, false, &os.PathError{Op: "resolve project directory", Path: home, Err: windows.ERROR_ACCESS_DENIED}
	}
	t.Cleanup(func() { findProject = original })
	e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: home}}}
	agent, _ := Lookup("codex")
	var out bytes.Buffer
	RunEvent(agent, e, Event{Name: "SessionEnd", Cwd: home, Session: "denied"}, &out, nil)
	data, err := os.ReadFile(filepath.Join(home, "hook.log"))
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(string(data))
	if out.Len() != 0 || strings.Count(string(data), "\n") != 1 || IsErrorLogLine(line) || strings.Contains(line, permissionMarker) {
		t.Fatalf("native discovery denial was not a single silent skip: output=%s log=%s", &out, data)
	}
}

func TestPermissionLogIdentifiesProcessAndFailedOperation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_DIAGNOSTIC_SECRET", "private-context-sentinel")
	path := filepath.Join(home, "store.lock")
	err := fmt.Errorf("open service: %w", &os.PathError{Op: "open", Path: path, Err: windows.ERROR_ACCESS_DENIED})
	LogEventError(home, "codex", Event{Name: "SessionStart", Session: "test", Cwd: home}, err)
	data, readErr := os.ReadFile(filepath.Join(home, "hook.log"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	line := string(data)
	if strings.Count(line, "\n") != 1 || strings.Contains(line, "private-context-sentinel") {
		t.Fatalf("diagnostic escaped its metadata boundary: %s", line)
	}
	_, raw, found := strings.Cut(line, permissionMarker)
	var got permissionDiagnostic
	if !found || json.Unmarshal([]byte(raw), &got) != nil {
		t.Fatalf("missing structured diagnostic: %s", line)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	if got.User == "" || got.User == "unknown" || got.SID != user.User.Sid.String() || got.PID != os.Getpid() {
		t.Fatalf("wrong process identity: %+v", got)
	}
	if !strings.HasPrefix(got.IntegritySID, "S-1-16-") || got.AppContainer == nil || got.Restricted == nil || got.Elevated == nil {
		t.Fatalf("missing token properties: %+v", got)
	}
	if got.ParentPID != os.Getppid() || got.ParentImage == "" || filepath.Base(got.ParentImage) != got.ParentImage {
		t.Fatalf("missing parent metadata or leaked parent path: %+v", got)
	}
	if got.Operation != "open" || got.Path != path || len(got.MetadataErrors) != 0 {
		t.Fatalf("failed operation lost: %+v", got)
	}
	if _, denied := PermissionSummary(line); !denied {
		t.Fatal("doctor cannot identify the emitted denial")
	}
}

func TestPermissionLogDoesNotInventMissingOperation(t *testing.T) {
	var raw permissionDiagnostic
	if err := json.Unmarshal([]byte(permissionDetails(os.ErrPermission, "user", "sid")), &raw); err != nil {
		t.Fatal(err)
	}
	if raw.Operation != "" || raw.Path != "" {
		t.Fatalf("invented an operation for a bare OS error: %+v", raw)
	}
	var link permissionDiagnostic
	err := &os.LinkError{Op: "rename", Old: "relative-old", New: "relative-new", Err: os.ErrPermission}
	if err := json.Unmarshal([]byte(permissionDetails(err, "user", "sid")), &link); err != nil {
		t.Fatal(err)
	}
	old, _ := filepath.Abs("relative-old")
	newPath, _ := filepath.Abs("relative-new")
	if link.Operation != "rename" || link.Path != old || link.Destination != newPath {
		t.Fatalf("link operation paths are not absolute: %+v", link)
	}
}

func TestPermissionTokenQueryFailureIsUnknown(t *testing.T) {
	if got, err := tokenFlag(0, windows.TokenElevation); got != nil || err == nil {
		t.Fatalf("invalid token reported a definite elevation: %v %v", got, err)
	}
	if got, err := tokenIntegrity(0); got != "" || err == nil {
		t.Fatalf("invalid token reported an integrity level: %q %v", got, err)
	}
}

func TestPermissionSummaryRecognizesLegacyAndRejectsUnrelatedErrors(t *testing.T) {
	for _, line := range []string{
		"codex Stop: Access is denied.", "claude SessionStart: permission denied",
		"codex Stop: 拒绝访问。", PermissionHint, permissionMarker + "{",
	} {
		if summary, denied := PermissionSummary(line); !denied || summary == "" {
			t.Errorf("hidden denial: %q -> %q %v", line, summary, denied)
		}
	}
	home := t.TempDir()
	LogEventError(home, "codex", Event{Name: "Stop"}, errors.New("unrelated error"))
	data, err := os.ReadFile(filepath.Join(home, "hook.log"))
	if err != nil {
		t.Fatal(err)
	}
	if summary, denied := PermissionSummary(string(data)); denied || summary != "" || strings.Contains(string(data), permissionMarker) {
		t.Fatalf("non-permission error acquired a denial diagnostic: %s", data)
	}
}
