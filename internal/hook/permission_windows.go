package hook

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/woodfishhhh/obituary/internal/pathutil"
	"golang.org/x/sys/windows"
)

const permissionMarker = "; permission_diagnostic="

// Only identity and failed-operation metadata goes in this record: no command
// line, environment, token privileges, or file contents. Unknown is not false.
type permissionDiagnostic struct {
	User           string   `json:"user"`
	SID            string   `json:"sid"`
	PID            int      `json:"pid"`
	IntegritySID   string   `json:"integrity_sid"`
	AppContainer   *bool    `json:"app_container"`
	Restricted     *bool    `json:"restricted"`
	Elevated       *bool    `json:"elevated"`
	ParentPID      int      `json:"parent_pid"`
	ParentImage    string   `json:"parent_image"`
	Operation      string   `json:"operation"`
	Path           string   `json:"path"`
	Destination    string   `json:"destination,omitempty"`
	MetadataErrors []string `json:"metadata_errors,omitempty"`
}

func permissionDetails(err error, name, sid string) string {
	d := permissionDiagnostic{User: name, SID: sid, PID: os.Getpid(), ParentPID: os.Getppid()}
	token := windows.GetCurrentProcessToken()
	var queryErr error
	d.IntegritySID, queryErr = tokenIntegrity(token)
	d.recordError("integrity", queryErr)
	// TokenIsAppContainer is 29 in winnt.h, absent from x/sys v0.49.0.
	d.AppContainer, queryErr = tokenFlag(token, 29)
	d.recordError("app_container", queryErr)
	d.Elevated, queryErr = tokenFlag(token, windows.TokenElevation)
	d.recordError("elevated", queryErr)
	restricted, queryErr := token.IsRestricted()
	if queryErr == nil {
		d.Restricted = &restricted
	}
	d.recordError("restricted", queryErr)
	d.ParentImage, queryErr = processImage(d.ParentPID)
	d.recordError("parent_image", queryErr)
	d.failedOperation(err)
	data, _ := json.Marshal(d) // This fixed struct contains only JSON-supported values.
	return string(data)
}

func (d *permissionDiagnostic) recordError(field string, err error) {
	if err != nil {
		d.MetadataErrors = append(d.MetadataErrors, field+": "+err.Error())
	}
}

func (d *permissionDiagnostic) failedOperation(err error) {
	var pathErr *os.PathError
	var linkErr *os.LinkError
	switch {
	case errors.As(err, &pathErr):
		d.Operation, d.Path = pathErr.Op, pathErr.Path
	case errors.As(err, &linkErr):
		d.Operation, d.Path, d.Destination = linkErr.Op, linkErr.Old, linkErr.New
	}
	for _, path := range []*string{&d.Path, &d.Destination} {
		if *path != "" {
			absolute, err := pathutil.Abs(*path)
			if err == nil {
				*path = absolute
			}
			d.recordError("absolute path", err)
		}
	}
}

// Use x/sys GetTokenInformation (as its Token.IsElevated does), but retain the
// error: an unreadable token must not be reported as unrestricted/unelevated.
func tokenFlag(token windows.Token, class uint32) (*bool, error) {
	var value, size uint32
	err := windows.GetTokenInformation(token, class, (*byte)(unsafe.Pointer(&value)), uint32(unsafe.Sizeof(value)), &size)
	if err != nil {
		return nil, err
	}
	flag := value != 0
	return &flag, nil
}

func tokenIntegrity(token windows.Token) (string, error) {
	// winnt.h TOKEN_MANDATORY_LABEL is SID_AND_ATTRIBUTES followed by its SID.
	// x/sys has no label helper; reserve SECURITY_MAX_SID_SIZE (68) bytes.
	var label struct {
		Label windows.SIDAndAttributes
		SID   [68]byte
	}
	var size uint32
	err := windows.GetTokenInformation(token, windows.TokenIntegrityLevel, (*byte)(unsafe.Pointer(&label)), uint32(unsafe.Sizeof(label)), &size)
	if err != nil {
		return "", err
	}
	return label.Label.Sid.String(), nil
}

func processImage(pid int) (string, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	var name [32768]uint16
	size := uint32(len(name))
	if err := windows.QueryFullProcessImageName(h, 0, &name[0], &size); err != nil {
		return "", err
	}
	return filepath.Base(windows.UTF16ToString(name[:size])), nil
}

// PermissionSummary recognizes both structured diagnostics and old logs. It
// does not infer a sandbox identity or recommend changing access permissions.
func PermissionSummary(line string) (string, bool) {
	if _, raw, ok := strings.Cut(line, permissionMarker); ok {
		var d permissionDiagnostic
		if err := json.Unmarshal([]byte(raw), &d); err == nil {
			data, _ := json.Marshal(d)
			return string(data), true
		}
		return "permission diagnostic is unreadable", true
	}
	lower := strings.ToLower(line)
	for _, text := range []string{PermissionHint, "access is denied", "access denied", "permission denied", "拒绝访问"} {
		if strings.Contains(lower, strings.ToLower(text)) {
			return "identity and failed path unavailable in this unstructured log entry", true
		}
	}
	return "", false
}
