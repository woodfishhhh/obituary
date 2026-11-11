package explorer

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestLabelsRefuseResourceAttributes(t *testing.T) {
	for _, remove := range []bool{false, true} {
		for _, concurrent := range []bool{false, true} {
			t.Run(fmt.Sprintf("remove=%v/concurrent=%v", remove, concurrent), func(t *testing.T) {
				dir, dates := fixture(t)
				applyDates(t, dir, dates)
				path := filepath.Join(dir, iniName)
				if err := setLowIntegrityLabel(path); err != nil {
					t.Fatal(err)
				}
				attrs, err := attributes(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := setAttributes(path, attrs|windows.FILE_ATTRIBUTE_ARCHIVE); err != nil {
					t.Fatal(err)
				}
				if remove {
					dates = nil
				} else {
					dates["report.log"] = dates["report.log"].Add(24 * time.Hour)
				}
				edit := func() error { return addResourceAttribute(path) }
				if concurrent {
					beforeLabelsPublish = func(string) error { return edit() }
					beforeLabelsDelete = edit
					t.Cleanup(func() { beforeLabelsPublish, beforeLabelsDelete = nil, nil })
				} else if err := edit(); err != nil {
					t.Fatal(err)
				}
				p, err := prepareLabels(t, dir, dates)
				if err != nil {
					t.Fatal(err)
				}
				err = p.Apply()
				if err == nil || !strings.Contains(err.Error(), "unsupported visible security") || !bytes.Equal(readINI(t, dir), p.before.data) {
					t.Fatalf("unsupported security was rewritten: %v", err)
				}
				sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.ATTRIBUTE_SECURITY_INFORMATION)
				if err != nil || !strings.Contains(sd.String(), "Project") {
					t.Fatalf("resource attribute lost: %v %v", sd, err)
				}
				assertNoLabelTemporary(t, dir)
			})
		}
	}
}

func addResourceAttribute(path string) error {
	sd, err := windows.SecurityDescriptorFromString(`S:(RA;;;;;WD;("Project",TS,0,"Obituary"))`)
	if err != nil {
		return err
	}
	acl, _, err := sd.SACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.ATTRIBUTE_SECURITY_INFORMATION, nil, nil, nil, acl)
}

func TestLabelsPreserveIntegrityLabel(t *testing.T) {
	dir, dates := fixture(t)
	path := filepath.Join(dir, iniName)
	if err := os.WriteFile(path, []byte("; original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := setLowIntegrityLabel(path); err != nil {
		t.Fatal(err)
	}
	before, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.LABEL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	for _, labels := range []map[string]time.Time{dates, nil} {
		applyDates(t, dir, labels)
		after, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.LABEL_SECURITY_INFORMATION)
		if err != nil || after.String() != before.String() {
			t.Fatalf("integrity label lost: before=%s after=%v err=%v", before.String(), after, err)
		}
	}
}

func setLowIntegrityLabel(path string) error {
	sd, err := windows.SecurityDescriptorFromString("S:(ML;;NW;;;LW)")
	if err != nil {
		return err
	}
	sacl, _, err := sd.SACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.LABEL_SECURITY_INFORMATION, nil, nil, nil, sacl)
}
