//go:build unix

package receiptsigner

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

func guestSignerMigrationScript(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate migration test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "scripts", "guest-signer-state-migrate"))
}

func currentUserAndGroup(t *testing.T) (string, string) {
	t.Helper()
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	group, err := user.LookupGroupId(current.Gid)
	if err != nil {
		t.Fatal(err)
	}
	return current.Username, group.Name
}

func runGuestSignerMigration(t *testing.T, root string) error {
	t.Helper()
	return runGuestSignerMigrationWithPath(t, root, os.Getenv("PATH"))
}

func runGuestSignerMigrationWithPath(t *testing.T, root, path string) error {
	t.Helper()
	owner, group := currentUserAndGroup(t)
	command := exec.Command("bash", guestSignerMigrationScript(t), root, owner, group)
	environment := replaceEnvironmentVariable(os.Environ(), "BASH_ENV", "")
	environment = replaceEnvironmentVariable(environment, "ENV", "")
	command.Env = replaceEnvironmentVariable(environment, "PATH", path)
	if output, err := command.CombinedOutput(); err != nil {
		return &migrationCommandError{err: err, output: string(output)}
	}
	return nil
}

func replaceEnvironmentVariable(environment []string, name, value string) []string {
	prefix := name + "="
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}
	return append(result, prefix+value)
}

type migrationCommandError struct {
	err    error
	output string
}

func (e *migrationCommandError) Error() string { return e.err.Error() + ": " + e.output }

func TestGuestSignerStateMigrationPreservesBytesAndNormalizesModes(t *testing.T) {
	root := filepath.Join(t.TempDir(), "guest-core")
	receipts := filepath.Join(root, "receipts", "nested")
	if err := os.MkdirAll(receipts, 0o755); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(root, "key.ed25519")
	receiptPath := filepath.Join(receipts, "receipt.json")
	key := []byte("unchanged signing key bytes")
	receipt := []byte("unchanged receipt bytes")
	if err := os.WriteFile(keyPath, key, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receiptPath, receipt, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runGuestSignerMigration(t, root); err != nil {
		t.Fatal(err)
	}
	for path, wantMode := range map[string]os.FileMode{
		root:                      0o700,
		filepath.Dir(receiptPath): 0o700,
		keyPath:                   0o600,
		receiptPath:               0o600,
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != wantMode {
			t.Errorf("%s mode = %o, want %o", path, got, wantMode)
		}
	}
	if got, err := os.ReadFile(keyPath); err != nil || string(got) != string(key) {
		t.Fatalf("key changed: got=%q err=%v", got, err)
	}
	if got, err := os.ReadFile(receiptPath); err != nil || string(got) != string(receipt) {
		t.Fatalf("receipt changed: got=%q err=%v", got, err)
	}
	if err := runGuestSignerMigration(t, root); err != nil {
		t.Fatalf("idempotent rerun: %v", err)
	}
}

func TestGuestSignerStateMigrationRejectsSymlinksWithoutChangingTargets(t *testing.T) {
	for _, name := range []string{"key.ed25519", "receipts", filepath.Join("receipts", "nested-link")} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "guest-core")
			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(t.TempDir(), "target")
			original := []byte("outside signer state")
			if err := os.WriteFile(target, original, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
				t.Fatal(err)
			}
			if err := runGuestSignerMigration(t, root); err == nil {
				t.Fatal("symlink migration succeeded")
			}
			if got, err := os.ReadFile(target); err != nil || string(got) != string(original) {
				t.Fatalf("symlink target changed: got=%q err=%v", got, err)
			}
		})
	}
}

func TestGuestSignerStateMigrationRejectsRootSymlink(t *testing.T) {
	target := t.TempDir()
	root := filepath.Join(t.TempDir(), "guest-core")
	if err := os.Symlink(target, root); err != nil {
		t.Fatal(err)
	}
	if err := runGuestSignerMigration(t, root); err == nil {
		t.Fatal("root symlink migration succeeded")
	}
}

func TestGuestSignerStateMigrationRejectsHardLinksWithoutMetadataMutation(t *testing.T) {
	for _, name := range []string{"key", "receipt"} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "guest-core")
			if err := os.MkdirAll(filepath.Join(root, "receipts"), 0o755); err != nil {
				t.Fatal(err)
			}
			inside := filepath.Join(root, "key.ed25519")
			if name == "receipt" {
				inside = filepath.Join(root, "receipts", "receipt.json")
			}
			original := []byte("aliased protected bytes")
			if err := os.WriteFile(inside, original, 0o644); err != nil {
				t.Fatal(err)
			}
			outside := filepath.Join(t.TempDir(), "outside-link")
			if err := os.Link(inside, outside); err != nil {
				t.Fatal(err)
			}
			if err := runGuestSignerMigration(t, root); err == nil {
				t.Fatal("hard-linked state migration succeeded")
			}
			info, err := os.Stat(outside)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode().Perm(); got != 0o644 {
				t.Fatalf("outside link mode = %o, want 644", got)
			}
			if got, err := os.ReadFile(outside); err != nil || string(got) != string(original) {
				t.Fatalf("outside link changed: got=%q err=%v", got, err)
			}
		})
	}
}

func TestGuestSignerStateMigrationRejectsSpecialReceiptEntry(t *testing.T) {
	root := filepath.Join(t.TempDir(), "guest-core")
	receipts := filepath.Join(root, "receipts")
	if err := os.MkdirAll(receipts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(receipts, "receipt.fifo"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runGuestSignerMigration(t, root); err == nil {
		t.Fatal("special receipt entry migration succeeded")
	}
}

func TestGuestSignerStateMigrationFailsClosedWhenDiscoveryFails(t *testing.T) {
	root := filepath.Join(t.TempDir(), "guest-core")
	receipts := filepath.Join(root, "receipts")
	if err := os.MkdirAll(receipts, 0o755); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(root, "key.ed25519")
	if err := os.WriteFile(keyPath, []byte("unchanged"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeBin := t.TempDir()
	if err := os.WriteFile(filepath.Join(fakeBin, "find"), []byte("#!/bin/sh\nexit 73\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runGuestSignerMigrationWithPath(t, root, fakeBin+string(os.PathListSeparator)+os.Getenv("PATH")); err == nil {
		t.Fatal("migration succeeded after discovery failure")
	}
	for path := range map[string]struct{}{root: {}, receipts: {}, keyPath: {}} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o755 && path != keyPath {
			t.Fatalf("%s mode changed to %o", path, got)
		}
		if path == keyPath && info.Mode().Perm() != 0o644 {
			t.Fatalf("key mode changed to %o", info.Mode().Perm())
		}
	}
}
