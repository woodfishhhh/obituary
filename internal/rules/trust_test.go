//go:build windows

package rules

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestMissingTrustedRulesStayPausedUntilUntrust(t *testing.T) {
	root := t.TempDir()
	store := TrustStore{Path: filepath.Join(t.TempDir(), "trust.json")}
	path := filepath.Join(root, FileName)
	if err := os.WriteFile(path, []byte("*.log off\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Trust(path, ""); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(root, store); err == nil {
		t.Fatal("missing trusted rules resumed builtin rules")
	}
	if _, err := store.Trust(path, ""); err == nil {
		t.Fatal("missing rules can be trusted")
	}
	if was, err := store.Untrust(path); err != nil || !was {
		t.Fatalf("untrust: %v %v", was, err)
	}
	set, state, err := Load(root, store)
	if err != nil || state.Status != Absent || len(set.Rules) == 0 {
		t.Fatalf("explicit untrust did not resume builtins: %+v %v", state, err)
	}
}

func TestTrustDistinctUnicodeRepositories(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store := TrustStore{Path: filepath.Join(root, "trust.json")}
	paths := []string{filepath.Join(root, "K", FileName), filepath.Join(root, "K", FileName)}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("notes/** 1m\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Trust(paths[0], ""); err != nil {
		t.Fatal(err)
	}
	state, err := store.Check(paths[1])
	if err != nil || state.Status != Untrusted {
		t.Fatalf("Unicode neighbor inherited trust: %+v %v", state, err)
	}
}

func TestTrustAliasesRevokeTogether(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, kind := range []string{"junction", "short"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			store := TrustStore{Path: filepath.Join(root, "trust.json")}
			dir := filepath.Join(root, "Long Rules Directory")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, FileName)
			if err := os.WriteFile(path, []byte("notes/** 1m\n"), 0600); err != nil {
				t.Fatal(err)
			}
			alias := filepath.Join(root, "alias")
			if kind == "junction" {
				if out, err := exec.Command("cmd", "/c", "mklink", "/J", alias, dir).CombinedOutput(); err != nil {
					t.Fatalf("mklink: %v %s", err, out)
				}
			} else {
				p, _ := windows.UTF16PtrFromString(dir)
				buf := make([]uint16, 32768)
				n, err := windows.GetShortPathName(p, &buf[0], uint32(len(buf)))
				if err != nil {
					t.Fatal(err)
				}
				alias = windows.UTF16ToString(buf[:n])
				if strings.EqualFold(alias, dir) {
					t.Skip("8.3 names unavailable")
				}
			}
			alias = filepath.Join(alias, FileName)
			if _, err := store.Trust(path, ""); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Trust(alias, ""); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(store.Path)
			if err != nil {
				t.Fatal(err)
			}
			var saved trustFile
			if err := json.Unmarshal(data, &saved); err != nil {
				t.Fatal(err)
			}
			if len(saved.Trusted) != 1 {
				t.Fatalf("alias trust stored duplicate records: %s", data)
			}
			// A previously duplicated store must also be completely revocable.
			for _, entry := range saved.Trusted {
				saved.Trusted[alias] = entry
				break
			}
			data, err = json.Marshal(saved)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(store.Path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if was, err := store.Untrust(alias); err != nil || !was {
				t.Fatalf("untrust alias: %v %v", was, err)
			}
			state, err := store.Check(path)
			if err != nil || state.Status != Untrusted {
				t.Fatalf("alias untrust left file trusted: %+v %v", state, err)
			}
			data, err = os.ReadFile(store.Path)
			if err != nil {
				t.Fatal(err)
			}
			saved = trustFile{}
			if err := json.Unmarshal(data, &saved); err != nil || len(saved.Trusted) != 0 {
				t.Fatalf("untrust retained an alias record: %s %v", data, err)
			}
		})
	}
}

func TestLegacyTrustMigration(t *testing.T) {
	t.Parallel()
	for _, ambiguous := range []bool{false, true} {
		t.Run(map[bool]string{false: "unique", true: "ambiguous"}[ambiguous], func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			path := filepath.Join(root, "K", FileName)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			content := "notes/** 1m\n"
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			if ambiguous {
				other := filepath.Join(root, "K", FileName)
				if err := os.MkdirAll(filepath.Dir(other), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(other, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			store := TrustStore{Path: filepath.Join(root, "trust.json")}
			data, err := json.Marshal(trustFile{Version: 1, Trusted: map[string]trustEntry{strings.ToLower(path): {SHA256: digest(content)}}})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(store.Path, data, 0600); err != nil {
				t.Fatal(err)
			}
			state, err := store.Check(path)
			want := Untrusted
			if err != nil || state.Status != want {
				t.Fatalf("legacy trust migration: %+v %v; want %s", state, err, want)
			}
			if state.Warning != "trust store format was upgraded; review the rules and run obit trust again" {
				t.Fatalf("legacy trust has no review notice: %+v", state)
			}
			if ambiguous {
				neighbor, err := store.Check(filepath.Join(root, "K", FileName))
				if err != nil || neighbor.Status != Untrusted {
					t.Fatalf("legacy Unicode neighbor inherited trust: %+v %v", neighbor, err)
				}
			}
		})
	}
}

func TestLegacyTrustReviewUpgradesStorage(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"trust", "untrust"} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			path, other := filepath.Join(root, FileName), filepath.Join(root, "other.rules")
			content := "notes/** 1m\n"
			for _, name := range []string{path, other} {
				if err := os.WriteFile(name, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			store := TrustStore{Path: filepath.Join(root, "trust.json")}
			data, err := json.Marshal(trustFile{Version: 1, Trusted: map[string]trustEntry{strings.ToLower(path): {SHA256: digest(content)}, strings.ToLower(other): {SHA256: digest(content)}}})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(store.Path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if action == "trust" {
				_, err = store.Trust(path, digest(content))
			} else {
				_, err = store.Untrust(path)
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err = os.ReadFile(store.Path)
			var stored trustFile
			if err != nil || json.Unmarshal(data, &stored) != nil || stored.Version != 2 {
				t.Fatalf("review did not write v2: %s %v", data, err)
			}
			state, err := store.Check(path)
			want := Untrusted
			if action == "trust" {
				want = Trusted
			}
			if err != nil || state.Status != want || state.Warning != "" {
				t.Fatalf("reviewed state: %+v %v", state, err)
			}
			if state, err := store.Check(other); err != nil || state.Status != Untrusted {
				t.Fatalf("v2 upgrade kept another legacy grant: %+v %v", state, err)
			}
		})
	}
}

func TestUntrustFileIdentityAliases(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"hardlink", "unc"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			path := filepath.Join(root, FileName)
			if err := os.WriteFile(path, []byte("notes/** 1m\n"), 0600); err != nil {
				t.Fatal(err)
			}
			alias := filepath.Join(root, "alias.rules")
			if kind == "hardlink" {
				if err := os.Link(path, alias); err != nil {
					t.Fatal(err)
				}
			} else {
				alias = `\\localhost\` + path[:1] + "$" + path[2:]
				if _, err := os.Stat(alias); err != nil {
					t.Skipf("administrative share unavailable: %v", err)
				}
			}
			store := TrustStore{Path: filepath.Join(root, "trust.json")}
			if _, err := store.Trust(path, ""); err != nil {
				t.Fatal(err)
			}
			if state, err := store.Check(alias); err != nil || state.Status != Untrusted {
				t.Fatalf("identity alias inherited trust without review: %+v %v", state, err)
			}
			if _, err := store.Trust(alias, ""); err != nil {
				t.Fatal(err)
			}
			if was, err := store.Untrust(path); err != nil || !was {
				t.Fatalf("untrust: %v %v", was, err)
			}
			for _, check := range []string{path, alias} {
				if state, err := store.Check(check); err != nil || state.Status != Untrusted {
					t.Fatalf("untrust left file identity alias trusted: %+v %v", state, err)
				}
			}
		})
	}
}

func TestUntrustMissingRules(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, alias := range []bool{false, true} {
		t.Run(fmt.Sprint(alias), func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			dir := filepath.Join(root, "K")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, FileName)
			content := []byte("notes/** 1m\n")
			if err := os.WriteFile(path, content, 0600); err != nil {
				t.Fatal(err)
			}
			store := TrustStore{Path: filepath.Join(root, "trust.json")}
			if _, err := store.Trust(path, ""); err != nil {
				t.Fatal(err)
			}
			ref := path
			if alias {
				link := filepath.Join(root, "alias")
				if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, dir).CombinedOutput(); err != nil {
					t.Fatalf("junction: %s %v", out, err)
				}
				ref = filepath.Join(link, FileName)
			}
			if err := os.Rename(path, path+".moved"); err != nil {
				t.Fatal(err)
			}
			ref = filepath.Join(filepath.Dir(ref), strings.ToUpper(filepath.Base(ref)))
			if was, err := store.Untrust(ref); err != nil || !was {
				t.Fatalf("missing rules could not be revoked: %v %v", was, err)
			}
			if err := os.Rename(path+".moved", path); err != nil {
				t.Fatal(err)
			}
			if state, err := store.Check(path); err != nil || state.Status != Untrusted {
				t.Fatalf("recreated rules inherited revoked trust: %+v %v", state, err)
			}
		})
	}
}

func TestUntrustCaseSensitiveNeighbors(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, missing := range []bool{false, true} {
		for _, leaf := range []bool{false, true} {
			t.Run(fmt.Sprintf("missing=%v/leaf=%v", missing, leaf), func(t *testing.T) {
				t.Parallel()
				root := t.TempDir()
				if out, err := exec.Command("fsutil", "file", "setCaseSensitiveInfo", root, "enable").CombinedOutput(); err != nil {
					t.Skipf("case sensitive directories unavailable: %v %s", err, out)
				}
				paths := []string{filepath.Join(root, "K", FileName), filepath.Join(root, "k", FileName)}
				if leaf {
					paths = []string{filepath.Join(root, "K.rules"), filepath.Join(root, "k.rules")}
				}
				t.Cleanup(func() {
					for _, path := range paths {
						target := filepath.Dir(path)
						if leaf {
							target = path
						}
						if err := os.RemoveAll(target); err != nil {
							t.Error(err)
						}
					}
					if out, err := exec.Command("fsutil", "file", "setCaseSensitiveInfo", root, "disable").CombinedOutput(); err != nil {
						t.Errorf("disable case sensitivity: %v %s", err, out)
					}
				})
				store := TrustStore{Path: filepath.Join(root, "trust.json")}
				for _, path := range paths {
					if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte("notes/** 1m\n"), 0600); err != nil {
						t.Fatal(err)
					}
					if _, err := store.Trust(path, ""); err != nil {
						t.Fatal(err)
					}
				}
				if missing {
					for _, path := range paths {
						if err := os.Rename(path, path+".moved"); err != nil {
							t.Fatal(err)
						}
					}
				}
				was, err := store.Untrust(paths[0])
				if missing {
					for _, path := range paths {
						if err := os.Rename(path+".moved", path); err != nil {
							t.Fatal(err)
						}
					}
				}
				other, checkErr := store.Check(paths[1])
				if err != nil || !was || checkErr != nil || other.Status != Trusted {
					t.Fatalf("untrust revoked distinct case-sensitive neighbor: was=%v other=%+v errors=%v/%v", was, other, err, checkErr)
				}
				if own, err := store.Check(paths[0]); err != nil || own.Status != Untrusted {
					t.Fatalf("selected rules remained trusted: %+v %v", own, err)
				}
			})
		}
	}
}
