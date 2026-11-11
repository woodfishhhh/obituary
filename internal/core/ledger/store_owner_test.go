package ledger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreDataOwnership(t *testing.T) {
	for _, scenario := range []string{"owned", "missing marker", "missing snapshot", "foreign ledger", "copied directory"} {
		t.Run(scenario, func(t *testing.T) {
			home := t.TempDir()
			store := Store{Home: home}
			if err := store.Update(func(*State) error { return nil }); err != nil {
				t.Fatal(err)
			}
			state, err := store.ReadSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "missing marker" {
				if err := os.Remove(filepath.Join(home, DataOwnerFile)); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "foreign ledger" {
				if err := store.UpdateGraveyard(func(st *State) error { st.DataID = "foreign"; return nil }); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "missing snapshot" {
				path := filepath.Join(home, state.Graveyard.File)
				if err := os.Rename(path, path+".held"); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "copied directory" {
				copyHome := t.TempDir()
				for _, name := range []string{DataOwnerFile, "store.json", state.Graveyard.File} {
					data, err := os.ReadFile(filepath.Join(home, name))
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(copyHome, name), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
				home = copyHome
			}
			root, err := os.OpenRoot(home)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			err = VerifyDataOwner(root)
			if (err == nil) != (scenario == "owned") {
				t.Fatalf("ownership: %v", err)
			}
		})
	}
}
