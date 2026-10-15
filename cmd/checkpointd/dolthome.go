package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// ensureDoltHome makes sure the directory the embedded Dolt engine uses for
// its global config is writable before any projection job is claimed.
//
// The projection rebuilder opens a scratch embedded Dolt workspace, and Dolt
// creates $HOME/.dolt (or $DOLT_ROOT_PATH/.dolt) on CREATE DATABASE. Under
// ProtectHome=tmpfs the default /root is read-only, the mkdir fails, and Dolt
// dereferences a nil config and panics after the job was already claimed
// (docs/problems/dolt-clustering-assessment-2026-10-15.md section 7.1). Failing
// here is a clear one-line error that claims nothing.
func ensureDoltHome(getenv func(string) string) error {
	home := getenv("DOLT_ROOT_PATH")
	if home == "" {
		home = getenv("HOME")
	}
	if home == "" {
		return fmt.Errorf("dolt home: neither DOLT_ROOT_PATH nor HOME is set")
	}
	if err := os.MkdirAll(filepath.Join(home, ".dolt"), 0o700); err != nil {
		return fmt.Errorf("dolt home %q is not writable: %w", home, err)
	}
	return nil
}
