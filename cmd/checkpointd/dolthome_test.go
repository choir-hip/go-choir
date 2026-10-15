package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureDoltHomeCreatesConfigDirUnderWritableHome(t *testing.T) {
	home := t.TempDir()
	env := map[string]string{"HOME": home}
	if err := ensureDoltHome(func(k string) string { return env[k] }); err != nil {
		t.Fatalf("ensureDoltHome: %v", err)
	}
	if st, err := os.Stat(filepath.Join(home, ".dolt")); err != nil || !st.IsDir() {
		t.Fatalf(".dolt not created: %v", err)
	}
}

func TestEnsureDoltHomePrefersDoltRootPath(t *testing.T) {
	root := t.TempDir()
	env := map[string]string{"HOME": "/nonexistent-readonly", "DOLT_ROOT_PATH": root}
	if err := ensureDoltHome(func(k string) string { return env[k] }); err != nil {
		t.Fatalf("ensureDoltHome: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".dolt")); err != nil {
		t.Fatalf(".dolt not created under DOLT_ROOT_PATH: %v", err)
	}
}

func TestEnsureDoltHomeFailsClearlyWhenHomeUnwritable(t *testing.T) {
	// A regular file as HOME makes MkdirAll fail the way a read-only /root does.
	file := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"HOME": file}
	err := ensureDoltHome(func(k string) string { return env[k] })
	if err == nil || !strings.Contains(err.Error(), "dolt home") {
		t.Fatalf("want a clear dolt home error, got %v", err)
	}
}

func TestEnsureDoltHomeFailsWhenHomeUnset(t *testing.T) {
	err := ensureDoltHome(func(string) string { return "" })
	if err == nil || !strings.Contains(err.Error(), "dolt home") {
		t.Fatalf("want a clear dolt home error, got %v", err)
	}
}
