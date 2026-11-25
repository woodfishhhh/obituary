package explorer

import (
	"errors"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"testing"
)

func TestDolphinLabelLifecycle(t *testing.T) {
	svc := syncService(t)
	item := trackedFile(t, t.TempDir(), "letter")
	if err := unix.Setxattr(item.Path, "user.xdg.tags", []byte("work,旅行"), 0); err != nil {
		t.Fatal(err)
	}
	storeItems(t, svc, item)
	check := func(lang i18n.Language, want string) {
		t.Helper()
		if err := Sync(svc, Options{Enabled: true, Everywhere: true, Language: lang}); err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, 2048)
		n, err := unix.Getxattr(item.Path, "user.xdg.tags", buf)
		if err != nil {
			t.Fatal(err)
		}
		if string(buf[:n]) != want {
			t.Fatalf("tags: %q, %v; want %q", buf[:n], err, want)
		}
	}
	check(i18n.English, "work,旅行,11-13 burial")
	item.ExpiresAt = item.ExpiresAt.AddDate(0, 0, 1)
	storeItems(t, svc, item)
	check(i18n.Chinese, "work,旅行,11-14 下葬")
	storeItems(t, svc)
	check(i18n.Chinese, "work,旅行")
}

func TestScopeAndUserOwnedTags(t *testing.T) {
	s := syncService(t)
	root := t.TempDir()
	downloads := filepath.Join(root, "Downloads")
	a := trackedFile(t, downloads, "a")
	b := trackedFile(t, root, "b")
	c := trackedFile(t, root, "c")
	c.WatchID = 7
	if err := unix.Setxattr(a.Path, tagAttr, []byte("11-13 burial"), 0); err != nil {
		t.Fatal(err)
	}
	storeItems(t, s, a, b, c)
	opts := Options{Enabled: true, Folders: []string{downloads}, Language: i18n.English}
	if err := Sync(s, opts); err != nil {
		t.Fatal(err)
	}
	if _, err := unix.Getxattr(b.Path, tagAttr, nil); !errors.Is(err, noTagAttr) {
		t.Fatalf("out-of-scope file: %v", err)
	}
	targets, err := CleanupTargets(s.Store.Home)
	if err != nil || len(targets) != 1 || targets[0] != c.Path {
		t.Fatalf("cleanup targets: %v %v", targets, err)
	}
	if err := os.Rename(c.Path, c.Path+"-moved"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.Path, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := unix.Setxattr(c.Path, tagAttr, []byte("11-13 burial"), 0); err != nil {
		t.Fatal(err)
	}
	storeItems(t, s)
	if err := Sync(s, opts); err == nil {
		t.Fatal("silently dropped unresolved identity")
	}
	for _, p := range []string{a.Path, c.Path} {
		buf := make([]byte, 128)
		n, err := unix.Getxattr(p, tagAttr, buf)
		if err != nil || string(buf[:n]) != "11-13 burial" {
			t.Fatalf("lost user tag on %s: %v", p, err)
		}
	}
}
