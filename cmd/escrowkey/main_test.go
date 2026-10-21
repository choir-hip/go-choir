package main

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/keyescrow"
)

func runTool(t *testing.T, env map[string]string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr, func(k string) string { return env[k] })
	return code, stdout.String(), stderr.String()
}

// The operator path of the deployed proof, end to end on a fixture: check,
// kek, seal, check, unseal, with the fingerprint unchanged throughout and
// no key material on stdout or stderr.
func TestEscrowKeyToolRoundTrip(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "key-escrow-key")
	priv, pub, err := keyescrow.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	encoded := base64.RawStdEncoding.EncodeToString(priv[:])
	if err := os.WriteFile(keyPath, []byte(encoded+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	kekPath := filepath.Join(dir, "key-escrow-kek")
	fp := keyescrow.Fingerprint(pub)
	env := map[string]string{keyescrow.EnvKEKPath: kekPath, "CHOIR_KEY_ESCROW_KEY_PATH": keyPath}

	noSecrets := func(out ...string) {
		t.Helper()
		kek, _ := os.ReadFile(kekPath)
		for _, o := range out {
			if strings.Contains(o, encoded) || (len(kek) > 0 && strings.Contains(o, string(kek))) {
				t.Fatal("tool output carries key material")
			}
		}
	}

	code, out, errOut := runTool(t, env, "check")
	if code != 0 || !strings.Contains(out, "state=plaintext") || !strings.Contains(out, "public_key_sha256="+fp) {
		t.Fatalf("check plaintext: %d %q %q", code, out, errOut)
	}
	noSecrets(out, errOut)

	// Seal before a KEK exists: refused, file unchanged.
	if code, _, _ := runTool(t, env, "seal"); code == 0 {
		t.Fatal("seal without a KEK succeeded")
	}
	if code, out, errOut := runTool(t, env, "kek-generate"); code != 0 {
		t.Fatalf("kek-generate: %d %q %q", code, out, errOut)
	}
	if code, _, _ := runTool(t, env, "kek-generate"); code == 0 {
		t.Fatal("kek-generate overwrote an existing KEK")
	}
	code, out, errOut = runTool(t, env, "seal")
	if code != 0 || !strings.Contains(out, fp) {
		t.Fatalf("seal: %d %q %q", code, out, errOut)
	}
	noSecrets(out, errOut)
	code, out, _ = runTool(t, env, "check")
	if code != 0 || !strings.Contains(out, "state=sealed") || !strings.Contains(out, "opens=yes") || !strings.Contains(out, fp) {
		t.Fatalf("check sealed: %d %q", code, out)
	}
	// Without the KEK the check reports that it cannot open, and fails.
	code, out, _ = runTool(t, map[string]string{"CHOIR_KEY_ESCROW_KEY_PATH": keyPath}, "check")
	if code == 0 || !strings.Contains(out, "state=sealed") || !strings.Contains(out, "opens=no") {
		t.Fatalf("check sealed without KEK: %d %q", code, out)
	}
	code, out, errOut = runTool(t, env, "unseal")
	if code != 0 || !strings.Contains(out, fp) {
		t.Fatalf("unseal: %d %q %q", code, out, errOut)
	}
	noSecrets(out, errOut)
	got, state, err := keyescrow.LoadPrivateKey(keyPath, keyescrow.SealConfig{Mode: keyescrow.SealOff})
	if err != nil || got != priv || state != keyescrow.KeyFilePlaintext {
		t.Fatalf("after unseal: %s %v", state, err)
	}
}

func TestEscrowKeyToolRefusals(t *testing.T) {
	dir := t.TempDir()
	if code, _, _ := runTool(t, nil); code == 0 {
		t.Fatal("no subcommand accepted")
	}
	if code, _, _ := runTool(t, nil, "bogus"); code == 0 {
		t.Fatal("unknown subcommand accepted")
	}
	// check on a missing file reports missing and fails; it never creates one.
	missing := filepath.Join(dir, "absent")
	code, out, _ := runTool(t, map[string]string{"CHOIR_KEY_ESCROW_KEY_PATH": missing}, "check")
	if code == 0 || !strings.Contains(out, "state=missing") {
		t.Fatalf("check missing: %d %q", code, out)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("check created a key file")
	}
	// B3: kek-generate refuses a name the backup does not exclude.
	if code, _, _ := runTool(t, nil, "kek-generate", "--kek", filepath.Join(dir, "custom-kek")); code == 0 {
		t.Fatal("kek-generate wrote a KEK the backup would carry")
	}
	if _, err := os.Stat(filepath.Join(dir, "custom-kek")); !os.IsNotExist(err) {
		t.Fatal("kek-generate created a misnamed KEK")
	}
	// Flags override the environment.
	code, out, _ = runTool(t, nil, "check", "--key", missing)
	if code == 0 || !strings.Contains(out, "state=missing") {
		t.Fatalf("check --key: %d %q", code, out)
	}
}
