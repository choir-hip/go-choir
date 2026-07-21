//go:build unix

package receiptsigner

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func guestSignerProjectionScript(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate projection test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "scripts", "guest-signer-state-project"))
}

func TestGuestSignerStateProjectionClassifiesKeyShapeWithoutChangingBytes(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		link    bool
		want    string
	}{
		{name: "absent", want: "absent\n"},
		{name: "invalid size", content: make([]byte, 63), want: "size-invalid\n"},
		{name: "exact size", content: make([]byte, 64), want: "exact-size\n"},
		{name: "symlink", content: make([]byte, 64), link: true, want: "unknown\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			keyPath := filepath.Join(root, "key.ed25519")
			if test.content != nil {
				writePath := keyPath
				if test.link {
					writePath = filepath.Join(t.TempDir(), "outside-key")
				}
				if err := os.WriteFile(writePath, test.content, 0o600); err != nil {
					t.Fatal(err)
				}
				if test.link {
					if err := os.Symlink(writePath, keyPath); err != nil {
						t.Fatal(err)
					}
				}
			}
			outputRoot := filepath.Join(root, "output")
			if err := os.Mkdir(outputRoot, 0o700); err != nil {
				t.Fatal(err)
			}
			command := exec.Command("bash", guestSignerProjectionScript(t), keyPath, outputRoot, "64")
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("project state: %v output=%s", err, output)
			}
			got, err := os.ReadFile(filepath.Join(outputRoot, "guest-signer-key-shape"))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != test.want {
				t.Fatalf("shape = %q, want %q", got, test.want)
			}
			if info, err := os.Stat(filepath.Join(outputRoot, "guest-signer-state-migrated")); err != nil {
				t.Fatal(err)
			} else if info.Mode().Perm() != 0o600 {
				t.Fatalf("migration marker mode = %o, want 600", info.Mode().Perm())
			}
			if test.content != nil {
				gotKey, err := os.ReadFile(keyPath)
				if err != nil {
					t.Fatal(err)
				}
				if string(gotKey) != string(test.content) {
					t.Fatal("key bytes changed")
				}
			}
		})
	}
}
