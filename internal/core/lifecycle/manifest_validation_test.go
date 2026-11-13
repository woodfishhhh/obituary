package lifecycle_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func TestWatchManifestRetainsUnverifiedFilesAndDirectories(t *testing.T) {
	for _, kind := range []string{"file", "directory"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("bundle/payload.zip")
			if kind == "directory" {
				path = filepath.Dir(path)
			}
			item, grave := interruptBurial(t, f, path, "bury")
			manifest := filepath.Dir(grave) + ".json"
			rewriteManifest(t, manifest, func(m map[string]any) {
				m["version"], m["source"] = 3, "watch"
				m["watchCheck"] = core.WatchSnapshot{Identity: *item.Identity, Directory: kind == "directory"}
			})
			pending, err := f.svc.PendingGraves()
			if err != nil || len(pending) != 1 || pending[0].Status != "unverifiable" || !strings.Contains(pending[0].Error, "watch object needs manual attention") {
				t.Fatalf("unverified watch object lost its recovery diagnostic: %+v %v", pending, err)
			}
			if !exists(grave) || !exists(manifest) {
				t.Fatal("unverified watch content or evidence was removed")
			}
			rewriteManifest(t, manifest, func(m map[string]any) { m["source"] = "cli" })
			pending, err = f.svc.PendingGraves()
			if err != nil || len(pending) != 1 || !strings.Contains(pending[0].Error, "invalid manifest version") {
				t.Fatalf("non-watch source accepted a watch verification manifest: %+v %v", pending, err)
			}
		})
	}
}

func TestManifestRejectsInvalidFields(t *testing.T) {
	for _, field := range []string{"version", "zero-id", "identity", "graveId", "buriedAt", "purgeAt", "size", "reason", "watch-error", "child-parent-id", "child-identity", "duplicate-child", "contains-parent", "duplicate-contains", "identity-hex", "identity-length", "identity-volume", "identity-relative-volume"} {
		t.Run(field, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("output/payload.txt")
			_, grave := interruptBurial(t, f, filepath.Dir(path), "bury")
			manifest := filepath.Dir(grave) + ".json"
			if field == "identity-volume" || field == "identity-relative-volume" {
				t.Chdir(filepath.Dir(grave))
			}
			rewriteManifest(t, manifest, func(m map[string]any) {
				switch field {
				case "version":
					m["version"] = 4
				case "zero-id":
					m["id"] = 0
				case "identity", "graveId":
					m[field] = nil
				case "buriedAt":
					m[field] = time.Time{}
				case "purgeAt":
					m[field] = m["buriedAt"]
				case "size":
					m[field] = 1
				case "reason":
					m[field] = "unrecognized"
				case "watch-error":
					m["watchError"] = "unexpected"
				case "child-parent-id":
					m["children"] = []any{map[string]any{"id": m["id"], "identity": m["identity"]}}
				case "child-identity":
					m["children"] = []any{map[string]any{"id": 99}}
				case "duplicate-child":
					child := map[string]any{"id": 99, "identity": m["identity"]}
					m["children"] = []any{child, child}
				case "contains-parent":
					m["contains"] = []any{m["id"]}
				case "duplicate-contains":
					m["children"] = []any{map[string]any{"id": 99, "identity": m["identity"]}}
					m["contains"] = []int{99, 99}
				case "identity-hex":
					m["identity"].(map[string]any)["fileId"] = strings.Repeat("z", 32)
				case "identity-length":
					m["identity"].(map[string]any)["fileId"] = "aabb"
				case "identity-volume":
					m["identity"].(map[string]any)["volume"] = "?:invalid"
				case "identity-relative-volume":
					id := m["identity"].(map[string]any)
					cwd, err := os.Getwd()
					if err != nil {
						t.Fatal(err)
					}
					relative, err := filepath.Rel(cwd, id["volume"].(string))
					if err != nil {
						t.Fatal(err)
					}
					id["volume"] = relative
				}
			})
			pending, err := f.svc.PendingGraves()
			if err != nil || len(pending) != 1 || pending[0].Status != "unverifiable" || pending[0].Error == "" {
				t.Fatalf("invalid manifest accepted: %+v %v", pending, err)
			}
			if strings.HasPrefix(field, "identity-") && !strings.Contains(pending[0].Error, "invalid manifest object identity") {
				t.Fatalf("invalid manifest accepted without identity diagnostic: %+v", pending)
			}
			if read(t, filepath.Join(grave, "payload.txt")) != "output/payload.txt" || !exists(manifest) {
				t.Fatal("invalid manifest changed content or evidence")
			}
		})
	}
}

func TestManifestRejectsOversizedJSON(t *testing.T) {
	f := newFixture(t)
	_, grave := interruptBurial(t, f, f.file("payload.txt"), "bury")
	manifest := filepath.Dir(grave) + ".json"
	data := read(t, manifest) + strings.Repeat(" ", 1<<20)
	if err := os.WriteFile(manifest, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	pending, err := f.svc.PendingGraves()
	if err != nil || len(pending) != 1 || pending[0].Status != "unverifiable" || !strings.Contains(pending[0].Error, "at most 1 MiB") {
		t.Fatalf("oversized JSON accepted: %+v %v", pending, err)
	}
	if read(t, grave) != "payload.txt" || !exists(manifest) {
		t.Fatal("oversized JSON changed content or evidence")
	}
}
