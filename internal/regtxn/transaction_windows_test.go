package regtxn_test

import (
	"errors"
	"testing"

	"github.com/woodfishhhh/obituary/internal/regtxn"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"golang.org/x/sys/windows/registry"
)

func TestTransactionRollback(t *testing.T) {
	root := testenv.ClassesKey(t)
	key := testenv.ClassesSubkey(t, root, "owned")
	if err := key.SetStringValue("owned", "old"); err != nil {
		t.Fatal(err)
	}
	err := regtxn.Do(func(tx *regtxn.Transaction) error {
		handle, err := tx.OpenPinned(registry.CURRENT_USER, root+`\owned`, registry.ALL_ACCESS)
		if err != nil {
			return err
		}
		defer handle.Close()
		if err := handle.SetStringValue("owned", "uncommitted"); err != nil {
			return err
		}
		return errors.New("injected rollback")
	})
	if err == nil {
		t.Fatal("rollback failure hidden")
	}
	if got, _, err := key.GetStringValue("owned"); err != nil || got != "old" {
		t.Fatalf("transaction rollback lost original: %q %v", got, err)
	}
	if names, err := key.ReadValueNames(-1); err != nil || len(names) != 1 {
		t.Fatalf("transaction marker leaked: %+v %v", names, err)
	}
}

func TestTransactionRollbackAndConcurrentWrite(t *testing.T) {
	root := testenv.ClassesKey(t)
	key := testenv.ClassesSubkey(t, root, "owned")
	if err := key.SetStringValue("owned", "old"); err != nil {
		t.Fatal(err)
	}
	for _, fail := range []bool{true, false} {
		published := false
		err := regtxn.Do(func(tx *regtxn.Transaction) error {
			handle, err := tx.Open(registry.CURRENT_USER, root+`\owned`, registry.ALL_ACCESS)
			if err != nil {
				return err
			}
			defer handle.Close()
			if _, _, err := handle.GetStringValue("owned"); err != nil {
				return err
			}
			if err := handle.SetStringValue("owned", "new"); err != nil {
				return err
			}
			// Ordinary writes conflict with a transactional writer. Whichever
			// commits, a successfully published foreign value must survive.
			externalErr := key.SetStringValue("owned", "foreign")
			published = externalErr == nil
			if fail {
				return errors.New("injected rollback")
			}
			return nil
		})
		want := "new"
		if fail {
			want = "old"
		}
		if published {
			want = "foreign"
		}
		if !published && (err != nil) != fail {
			t.Fatalf("transaction result: %v", err)
		}
		if got, _, err := key.GetStringValue("owned"); err != nil || got != want {
			t.Fatalf("transaction lost value: %q %v", got, err)
		}
	}
}

func TestTransactionDeleteRollback(t *testing.T) {
	root := testenv.ClassesKey(t)
	key := testenv.ClassesSubkey(t, root, "owned")
	if err := key.SetStringValue("owned", "old"); err != nil {
		t.Fatal(err)
	}
	err := regtxn.Do(func(tx *regtxn.Transaction) error {
		handle, err := tx.OpenPinned(registry.CURRENT_USER, root+`\owned`, registry.ALL_ACCESS)
		if err != nil {
			return err
		}
		defer handle.Close()
		if err := tx.Delete(handle); err != nil {
			return err
		}
		return errors.New("injected rollback after delete")
	})
	if err == nil {
		t.Fatal("delete rollback failure hidden")
	}
	if value, _, err := key.GetStringValue("owned"); err != nil || value != "old" {
		t.Fatalf("transaction deletion escaped rollback: %q %v", value, err)
	}
}

func TestTransactionDeletePreservesPublishedValue(t *testing.T) {
	root := testenv.ClassesKey(t)
	key := testenv.ClassesSubkey(t, root, "owned")
	if err := key.SetStringValue("owned", "old"); err != nil {
		t.Fatal(err)
	}
	published := false
	err := regtxn.Do(func(tx *regtxn.Transaction) error {
		handle, err := tx.Open(registry.CURRENT_USER, root+`\owned`, registry.ALL_ACCESS)
		if err != nil {
			return err
		}
		defer handle.Close()
		if err := tx.PinWrite(handle); err != nil {
			return err
		}
		if _, _, err := handle.GetStringValue("owned"); err != nil {
			return err
		}
		published = key.SetStringValue("foreign", "keep") == nil
		return tx.Delete(handle)
	})
	if !published && err != nil {
		t.Fatalf("pinned deletion failed: %v", err)
	}
	if published {
		if err == nil {
			t.Fatal("transaction deleted concurrently published value")
		}
		if value, _, err := key.GetStringValue("foreign"); err != nil || value != "keep" {
			t.Fatalf("transaction deleted concurrently published value: %q %v", value, err)
		}
	}
}

func TestTransactionExclusiveCreation(t *testing.T) {
	root := testenv.ClassesKey(t)
	key := testenv.ClassesSubkey(t, root, "appeared")
	if err := key.SetStringValue("foreign", "keep"); err != nil {
		t.Fatal(err)
	}
	err := regtxn.Do(func(tx *regtxn.Transaction) error {
		handle, err := tx.CreateNew(registry.CURRENT_USER, root+`\appeared`, registry.ALL_ACCESS)
		if handle != 0 {
			handle.Close()
		}
		return err
	})
	if err == nil {
		t.Fatal("exclusive registry create reused foreign key")
	}
	if got, _, err := key.GetStringValue("foreign"); err != nil || got != "keep" {
		t.Fatal(got, err)
	}
}
