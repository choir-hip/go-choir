package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileKeyStoreSaveLoadDelete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "apikey.json")
	store := NewFileKeyStore(path)

	// Load before save -> ErrNotExist.
	if _, err := store.Load(); !os.IsNotExist(err) {
		t.Fatalf("Load before save: got %v, want os.ErrNotExist", err)
	}

	if err := store.Save("choir_sk_test_secret"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load after save: %v", err)
	}
	if got != "choir_sk_test_secret" {
		t.Fatalf("Load: got %q, want choir_sk_test_secret", got)
	}

	// File must be 0600.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perm: got %o, want 0600", perm)
	}

	// Overwrite.
	if err := store.Save("choir_sk_new_secret"); err != nil {
		t.Fatalf("Save overwrite: %v", err)
	}
	got, err = store.Load()
	if err != nil {
		t.Fatalf("Load after overwrite: %v", err)
	}
	if got != "choir_sk_new_secret" {
		t.Fatalf("Load: got %q, want choir_sk_new_secret", got)
	}

	// Delete.
	if err := store.Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Load(); !os.IsNotExist(err) {
		t.Fatalf("Load after delete: got %v, want os.ErrNotExist", err)
	}

	// Delete again (idempotent).
	if err := store.Delete(); err != nil {
		t.Fatalf("Delete idempotent: %v", err)
	}
}

func TestFileKeyStoreRejectsEmpty(t *testing.T) {
	store := NewFileKeyStore(filepath.Join(t.TempDir(), "apikey.json"))
	if err := store.Save(""); err == nil {
		t.Fatal("Save empty secret should fail")
	}
}

func TestKeychainKeyStoreSaveRejectsEmpty(t *testing.T) {
	store := NewKeychainKeyStore()
	if err := store.Save(""); err == nil {
		t.Fatal("Save empty secret should fail")
	}
}
