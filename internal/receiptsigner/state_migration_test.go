package receiptsigner

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
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
	owner, group := currentUserAndGroup(t)
	command := exec.Command("bash", guestSignerMigrationScript(t), root, owner, group)
	command.Env = os.Environ()
	if output, err := command.CombinedOutput(); err != nil {
		return &migrationCommandError{err: err, output: string(output)}
	}
	return nil
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
	for _, name := range []string{"key.ed25519", "receipts"} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "guest-core")
			if err := os.MkdirAll(root, 0o700); err != nil {
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
