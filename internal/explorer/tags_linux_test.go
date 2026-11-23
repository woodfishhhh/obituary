package explorer

import (
	"github.com/woodfishhhh/obituary/internal/i18n"
	"golang.org/x/sys/unix"
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
		if err != nil || string(buf[:n]) != want {
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
