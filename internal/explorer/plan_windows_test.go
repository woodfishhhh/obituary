package explorer

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/winfs"
	"golang.org/x/sys/windows"
)

func FuzzPrepareINI(f *testing.F) {
	for _, seed := range iniFuzzSeeds() {
		f.Add(seed, uint8(0))
	}
	dir := f.TempDir()
	path := filepath.Join(dir, iniName)
	if err := os.WriteFile(filepath.Join(dir, "report.log"), []byte("payload"), 0o600); err != nil {
		f.Fatal(err)
	}
	dates := map[string]time.Time{"report.log": time.Date(2026, 11, 13, 0, 0, 0, 0, time.Local)}
	f.Fuzz(func(t *testing.T, original []byte, style uint8) {
		if err := os.WriteFile(path, original, 0o600); err != nil {
			t.Fatal(err)
		}
		p, err := prepareLabels(t, dir, dates)
		if !bytes.Equal(readINI(t, dir), original) {
			t.Fatal("planning modified desktop.ini")
		}
		if err == nil {
			text, decodeErr := decode(p.after)
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}
			base, _, unmergeErr := unmerge(text)
			before, _ := decode(original)
			want, _, beforeErr := unmerge(before)
			if beforeErr != nil || unmergeErr != nil || base != want {
				t.Fatalf("plan changed user text: %v / %v", beforeErr, unmergeErr)
			}
		}
		// Known user-owned keys give an oracle independent of section(). Vary
		// encoding, line endings, case, duplicate sections and the user value.
		nl := []string{"\n", "\r\n", "\r"}[style%3]
		value := strings.Map(func(r rune) rune {
			if r == '\r' || r == '\n' || r == 0 {
				return -1
			}
			return r
		}, string(original))
		user := "[Other]" + nl + "; keep" + nl + "x=1" + nl + "[LocalizedFileNames]" + nl + "REPORT.LOG=" + value + nl
		user += "[LocalizedFileNames]" + nl + "report.log=Last user name" + nl
		data := []byte(user)
		if style&4 != 0 {
			data = encode(user)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		p, err = prepareLabels(t, dir, dates)
		if err != nil {
			return
		} // oversized data or malformed ownership marker
		if !bytes.Equal(p.after, data) || p.remove {
			t.Fatal("an Obituary display name overrides a user-owned name")
		}
	})
}

func applyDates(t *testing.T, dir string, dates map[string]time.Time) {
	t.Helper()
	p, err := prepareLabels(t, dir, dates)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(); err != nil {
		t.Fatal(err)
	}
}

func prepareLabels(t *testing.T, dir string, dates map[string]time.Time) (*Plan, error) {
	t.Helper()
	id, err := (winfs.FS{}).Identify(dir)
	if err != nil {
		t.Fatal(err)
	}
	return Prepare(dir, authorization{Identity: id}, dates)
}

func readINI(t *testing.T, dir string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, iniName))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func fixture(t *testing.T) (string, map[string]time.Time) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "report.log"), []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "build-cache"), 0o700); err != nil {
		t.Fatal(err)
	}
	return dir, map[string]time.Time{"report.log": time.Date(2026, 11, 13, 0, 0, 0, 0, time.Local), "build-cache": time.Date(2026, 11, 20, 0, 0, 0, 0, time.Local)}
}

func TestLabelsMergeAndRestore(t *testing.T) {
	for _, kind := range []string{"absent", "utf8", "utf16", "readonly"} {
		t.Run(kind, func(t *testing.T) {
			dir, dates := fixture(t)
			path := filepath.Join(dir, iniName)
			original := []byte("; keep comment\n[.ShellClassInfo]\nIconResource = own.ico,0\n[LocalizedFileNames]\nexisting.txt = User name\n[Other]\nX=1")
			if kind == "utf16" {
				original = encode(string(original))
			}
			if kind != "absent" {
				if err := os.WriteFile(path, original, 0o600); err != nil {
					t.Fatal(err)
				}
				if kind == "readonly" {
					if err := setAttributes(path, windows.FILE_ATTRIBUTE_READONLY|windows.FILE_ATTRIBUTE_HIDDEN); err != nil {
						t.Fatal(err)
					}
					if err := setAttributes(dir, windows.FILE_ATTRIBUTE_READONLY|windows.FILE_ATTRIBUTE_SYSTEM); err != nil {
						t.Fatal(err)
					}
				}
			}
			beforeDir, _ := attributes(dir)
			beforeFile, _ := attributes(path)
			applyDates(t, dir, dates)
			data := readINI(t, dir)
			if !bytes.HasPrefix(data, []byte{255, 254}) {
				t.Fatal("missing UTF-16 LE BOM")
			}
			text, err := decode(data)
			if err != nil || !strings.Contains(text, "report.log=report（11-13 下葬）") || !strings.Contains(text, "build-cache=build-cache（11-20 下葬）") {
				t.Fatalf("labels: %q %v", text, err)
			}
			base, _, err := unmerge(text)
			if err != nil {
				t.Fatal(err)
			}
			if kind != "absent" {
				want, _ := decode(original)
				if base != want {
					t.Fatalf("user text changed: %q", base)
				}
			}
			fileAttrs, _ := attributes(path)
			dirAttrs, _ := attributes(dir)
			if fileAttrs&6 != 6 || dirAttrs&1 == 0 {
				t.Fatalf("attributes file=%x directory=%x", fileAttrs, dirAttrs)
			}
			applyDates(t, dir, dates)
			if !bytes.Equal(data, readINI(t, dir)) {
				t.Fatal("same dates rewrote content")
			}
			applyDates(t, dir, nil)
			afterDir, _ := attributes(dir)
			if beforeDir != afterDir {
				t.Fatalf("directory attributes %x => %x", beforeDir, afterDir)
			}
			if kind == "absent" {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("created INI retained: %v", err)
				}
				return
			}
			if !bytes.Equal(original, readINI(t, dir)) {
				t.Fatal("original bytes not restored")
			}
			afterFile, _ := attributes(path)
			if beforeFile != afterFile {
				t.Fatalf("file attributes %x => %x", beforeFile, afterFile)
			}
		})
	}
}

func TestLabelsKeepUserKeysAndLaterEdits(t *testing.T) {
	dir, dates := fixture(t)
	path := filepath.Join(dir, iniName)
	original := "[LocalizedFileNames]\r\nreport.log=My report\r\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	applyDates(t, dir, dates)
	text, _ := decode(readINI(t, dir))
	if strings.Count(text, "report.log=") != 1 || !strings.Contains(text, "report.log=My report") {
		t.Fatal(text)
	}
	if err := os.WriteFile(path, encode(text+"[User]\r\nNew=kept\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	applyDates(t, dir, nil)
	text, _ = decode(readINI(t, dir))
	if text != original+"[User]\r\nNew=kept\r\n" {
		t.Fatal(text)
	}
}

func TestLabelsRefuseChangedPlanAndLinks(t *testing.T) {
	for _, change := range []string{"content", "replacement", "symlink", "junction", "hardlink", "attributes"} {
		t.Run(change, func(t *testing.T) {
			if testing.Short() && change == "junction" {
				t.Skip("creates a junction using a real cmd process")
			}
			dir, dates := fixture(t)
			path := filepath.Join(dir, iniName)
			if err := os.WriteFile(path, []byte("; original"), 0o600); err != nil {
				t.Fatal(err)
			}
			p, err := prepareLabels(t, dir, dates)
			if err != nil {
				t.Fatal(err)
			}
			outside := filepath.Join(t.TempDir(), "outside.ini")
			if err := os.WriteFile(outside, []byte("outside untouched"), 0o600); err != nil {
				t.Fatal(err)
			}
			switch change {
			case "content":
				err = os.WriteFile(path, []byte("; changed"), 0o600)
			case "attributes":
				err = setAttributes(dir, windows.FILE_ATTRIBUTE_SYSTEM)
			default:
				if err := os.Rename(path, path+".old"); err != nil {
					t.Fatal(err)
				}
				switch change {
				case "replacement":
					err = os.WriteFile(path, []byte("; original"), 0o600)
				case "symlink":
					err = os.Symlink(outside, path)
					if errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
						t.Skip("Windows account lacks symlink privilege; junction and hardlink cases still run")
					}
				case "hardlink":
					err = os.Link(outside, path)
				case "junction":
					out, linkErr := exec.Command("cmd", "/c", "mklink", "/J", path, filepath.Dir(outside)).CombinedOutput()
					if linkErr != nil {
						t.Fatalf("junction: %v %s", linkErr, out)
					}
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(path)
			attrs, _ := attributes(dir)
			if err := p.Apply(); err == nil {
				t.Fatal("stale/linked plan accepted")
			}
			after, _ := os.ReadFile(path)
			afterAttrs, _ := attributes(dir)
			if !bytes.Equal(before, after) || attrs != afterAttrs {
				t.Fatal("refusal changed source")
			}
			data, err := os.ReadFile(outside)
			if err != nil || string(data) != "outside untouched" {
				t.Fatalf("outside changed: %q %v", data, err)
			}
			if change == "symlink" || change == "hardlink" || change == "junction" {
				if _, err := prepareLabels(t, dir, dates); err == nil {
					t.Fatal("linked INI planned")
				}
			}
		})
	}
}

func TestLabelsKeepNewUserContentInCreatedINI(t *testing.T) {
	dir, dates := fixture(t)
	applyDates(t, dir, dates)
	text, _ := decode(readINI(t, dir))
	if err := os.WriteFile(filepath.Join(dir, iniName), encode(text+"[User]\r\nNew=kept\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	applyDates(t, dir, nil)
	text, _ = decode(readINI(t, dir))
	if text != "[User]\r\nNew=kept\r\n" {
		t.Fatal(text)
	}
}

func TestLabelsRefuseReplacedParent(t *testing.T) {
	dir, dates := fixture(t)
	p, err := prepareLabels(t, dir, dates)
	if err != nil {
		t.Fatal(err)
	}
	old := dir + "-old"
	if err := os.Rename(dir, old); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(old); err != nil {
			t.Error(err)
		}
	})
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(); err == nil {
		t.Fatal("replaced parent accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, iniName)); !os.IsNotExist(err) {
		t.Fatalf("replacement modified: %v", err)
	}
}

func TestLabelsPreserveExistingNamedStreams(t *testing.T) {
	dir, dates := fixture(t)
	path := filepath.Join(dir, iniName)
	if err := os.WriteFile(path, []byte("; original"), 0o600); err != nil {
		t.Fatal(err)
	}
	stream := path + ":Zone.Identifier"
	if err := os.WriteFile(stream, []byte("[ZoneTransfer]\r\nZoneId=3\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, labels := range []map[string]time.Time{dates, nil} {
		applyDates(t, dir, labels)
		data, err := os.ReadFile(stream)
		if err != nil || string(data) != "[ZoneTransfer]\r\nZoneId=3\r\n" {
			t.Fatalf("original named stream lost: %q %v", data, err)
		}
	}
}

func TestLabelsCleanupKeepsAddedStream(t *testing.T) {
	for _, afterPlan := range []bool{false, true} {
		t.Run(fmt.Sprint(afterPlan), func(t *testing.T) {
			dir, dates := fixture(t)
			applyDates(t, dir, dates)
			stream := filepath.Join(dir, iniName) + ":user-note"
			addStream := func() {
				if err := os.WriteFile(stream, []byte("user stream"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if !afterPlan {
				addStream()
			}
			p, err := prepareLabels(t, dir, nil)
			if err != nil {
				t.Fatal(err)
			}
			if afterPlan {
				addStream()
				if err := setAttributes(filepath.Join(dir, iniName), p.before.attrs); err != nil {
					t.Fatal(err)
				}
			}
			if err := p.Apply(); err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(stream); err != nil || string(data) != "user stream" {
				t.Fatalf("cleanup lost added named stream: %q %v", data, err)
			}
			if text, err := decode(readINI(t, dir)); err != nil || strings.Contains(text, marker) {
				t.Fatalf("cleanup retained labels: %q %v", text, err)
			}
		})
	}
}

func TestLabelsCleanupKeepsUserAttributes(t *testing.T) {
	dir, dates := fixture(t)
	path := filepath.Join(dir, iniName)
	if err := os.WriteFile(path, []byte("; user"), 0o600); err != nil {
		t.Fatal(err)
	}
	applyDates(t, dir, dates)
	attrs, err := attributes(path)
	if err != nil {
		t.Fatal(err)
	}
	const user = windows.FILE_ATTRIBUTE_READONLY | windows.FILE_ATTRIBUTE_NOT_CONTENT_INDEXED
	if err := setAttributes(path, attrs|user); err != nil {
		t.Fatal(err)
	}
	applyDates(t, dir, nil)
	if got, err := attributes(path); err != nil || got&user != user || got&(windows.FILE_ATTRIBUTE_HIDDEN|windows.FILE_ATTRIBUTE_SYSTEM) != 0 {
		t.Fatalf("cleanup lost user attributes: %x %v", got, err)
	}
}

func TestLabelsCleanupKeepsNewFileUserAttributes(t *testing.T) {
	dir, dates := fixture(t)
	applyDates(t, dir, dates)
	path := filepath.Join(dir, iniName)
	attrs, err := attributes(path)
	if err != nil {
		t.Fatal(err)
	}
	const user = windows.FILE_ATTRIBUTE_READONLY | windows.FILE_ATTRIBUTE_NOT_CONTENT_INDEXED
	if err := setAttributes(path, attrs|user); err != nil {
		t.Fatal(err)
	}
	applyDates(t, dir, nil)
	if got, err := attributes(path); err != nil || got&user != user || got&6 != 0 {
		t.Fatalf("cleanup deleted customized new INI: %x %v", got, err)
	}
	if text, err := decode(readINI(t, dir)); err != nil || strings.Contains(text, marker) {
		t.Fatalf("cleanup retained labels: %q %v", text, err)
	}
}

func TestLabelsPreserveExistingDACL(t *testing.T) {
	dir, dates := fixture(t)
	path := filepath.Join(dir, iniName)
	if err := os.WriteFile(path, []byte("; original"), 0o600); err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;SY)(A;;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
	before, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	for _, labels := range []map[string]time.Time{dates, nil} {
		applyDates(t, dir, labels)
		after, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil || after.String() != before.String() {
			t.Fatalf("DACL changed: before=%s after=%v err=%v", before.String(), after, err)
		}
	}
}

func TestLabelsRejectEditedOwnershipAndInvalidNames(t *testing.T) {
	dir, dates := fixture(t)
	applyDates(t, dir, dates)
	text, _ := decode(readINI(t, dir))
	text = strings.Replace(text, "report.log=report（11-13 下葬）", "report.log=My renamed report", 1)
	if err := os.WriteFile(filepath.Join(dir, iniName), encode(text), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareLabels(t, dir, nil); err == nil {
		t.Fatal("edited ownership accepted")
	}
	for _, name := range []string{"../outside", "bad=name", "[x]", " x"} {
		other := t.TempDir()
		if _, err := prepareLabels(t, other, map[string]time.Time{name: time.Now()}); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
}
