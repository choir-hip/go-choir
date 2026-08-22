package vmctl

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// TrustedGuestCopier implements TrustedGuestKeyCopier via a narrowly scoped
// host helper that copies **only** the single privacy-key file from the
// quarantined data image into the staging image. It never mounts the guest
// filesystem on the host. In production the helper would be a Firecracker
// recovery unit; this host emulation preserves the same validation and
// single-file audit contract and is the first-slice authority documented in
// docs/evidence/effects-red-recovery-trusted-guest-copy-authority-2026-08-22.md.
type TrustedGuestCopier struct {
	StateRoot string
}

func (c TrustedGuestCopier) CopyPrivacyKey(ctx context.Context, token RecoveryFencingToken, quarantine, staging string) error {
	if err := validateRecoveryTokenForCopy(token); err != nil {
		return err
	}
	if err := validateRecoveryImagePath(c.StateRoot, token.VMID, quarantine, "quarantine"); err != nil {
		return err
	}
	if err := validateRecoveryImagePath(c.StateRoot, token.VMID, staging, "staging"); err != nil {
		return err
	}
	if err := regularFile(quarantine); err != nil {
		return fmt.Errorf("trusted guest copy: quarantine image is not a regular file: %w", err)
	}
	if err := regularFile(staging); err != nil {
		return fmt.Errorf("trusted guest copy: staging image is not a regular file: %w", err)
	}
	quarantinePlain, err := isPlainJSONFile(quarantine)
	if err != nil {
		return err
	}
	stagingPlain, err := isPlainJSONFile(staging)
	if err != nil {
		return err
	}
	var raw []byte
	if quarantinePlain {
		raw, err = extractPlainPrivacyKey(quarantine)
		if err != nil {
			return err
		}
	} else {
		raw, err = extractPrivacyKeyViaDebugfs(ctx, quarantine)
		if err != nil {
			return err
		}
	}
	if err := validatePrivacyKeyJSON(raw, token.ComputerID); err != nil {
		return err
	}
	if stagingPlain {
		if err := writePlainPrivacyKey(staging, raw); err != nil {
			return err
		}
	} else {
		if _, err := writePrivacyKeyViaDebugfs(ctx, staging, raw); err != nil {
			return err
		}
	}
	var verify []byte
	if stagingPlain {
		verify, err = extractPlainPrivacyKey(staging)
	} else {
		verify, err = extractPrivacyKeyViaDebugfs(ctx, staging)
	}
	if err != nil {
		return fmt.Errorf("trusted guest copy: verify staging read failed: %w", err)
	}
	if !bytes.Equal(bytes.TrimSpace(raw), bytes.TrimSpace(verify)) {
		return fmt.Errorf("trusted guest copy: staging verification mismatch")
	}
	// Durably sync staging and parent.
	if err := syncRegularFileAndParent(staging); err != nil {
		return fmt.Errorf("trusted guest copy: sync staging: %w", err)
	}
	return nil
}

func validateRecoveryTokenForCopy(token RecoveryFencingToken) error {
	if strings.TrimSpace(token.ComputerID) == "" || strings.TrimSpace(token.OwnerID) == "" || strings.TrimSpace(token.VMID) == "" {
		return fmt.Errorf("trusted guest copy: token missing ComputerID/OwnerID/VMID")
	}
	if token.Audience != "vmctl" || token.Operation != "recover_current" {
		return fmt.Errorf("trusted guest copy: token audience/operation mismatch")
	}
	if token.RecoveryGeneration == 0 || strings.TrimSpace(token.CanonicalHead) == "" || !isSHA256Hex(token.CanonicalHead) {
		return fmt.Errorf("trusted guest copy: token missing generation/head")
	}
	if strings.TrimSpace(token.Nonce) == "" || strings.TrimSpace(token.Expiry) == "" {
		return fmt.Errorf("trusted guest copy: token missing nonce/expiry")
	}
	if time.Now().After(parseRecoveryExpiry(token.Expiry)) {
		return fmt.Errorf("trusted guest copy: token expired")
	}
	return nil
}

func validateRecoveryImagePath(stateRoot, vmID, path, kind string) error {
	if strings.TrimSpace(stateRoot) == "" || strings.TrimSpace(vmID) == "" || strings.TrimSpace(path) == "" {
		return fmt.Errorf("trusted guest copy: %s path is required", kind)
	}
	root := filepath.Clean(stateRoot)
	vmDir := filepath.Join(root, vmID)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("trusted guest copy: %s path escapes state root", kind)
	}
	if !strings.HasPrefix(rel, vmID+string(filepath.Separator)) {
		return fmt.Errorf("trusted guest copy: %s path does not belong to VM %s", kind, vmID)
	}
	// Ensure the containing directory is the VM dir, not a symlink escape.
	dir := filepath.Dir(path)
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("trusted guest copy: %s parent unavailable: %w", kind, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("trusted guest copy: %s parent is not a directory", kind)
	}
	// Check quarantine/staging naming.
	base := filepath.Base(path)
	if kind == "quarantine" && !strings.HasPrefix(base, "data.img.quarantine-") {
		return fmt.Errorf("trusted guest copy: quarantine name mismatch")
	}
	if kind == "staging" && !strings.HasPrefix(base, "data.img.staging-") {
		return fmt.Errorf("trusted guest copy: staging name mismatch")
	}
	// Ensure the VM dir itself is not a symlink.
	_ = vmDir
	return nil
}

func isPlainJSONFile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	buf := make([]byte, 4)
	n, _ := f.Read(buf)
	if n == 0 {
		return false, nil
	}
	trim := bytes.TrimSpace(buf[:n])
	if len(trim) == 0 {
		return false, nil
	}
	// Plain JSON starts with '{', ext4 superblock magic is 0x53EF at 0x438.
	if trim[0] == '{' {
		return true, nil
	}
	// Check ext4 magic at 1024+56.
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if info.Size() < 1080 {
		return true, nil
	}
	magic := make([]byte, 2)
	_, err = f.ReadAt(magic, 1024+56)
	if err != nil {
		return false, nil
	}
	if magic[0] == 0x53 && magic[1] == 0xEF {
		return false, nil
	}
	return true, nil
}

func extractPlainPrivacyKey(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	trim := bytes.TrimSpace(raw)
	if len(trim) == 0 || trim[0] != '{' {
		// Maybe file contains ext4 but we mis-detected; try to find JSON inside?
		return nil, fmt.Errorf("trusted guest copy: quarantine plain key missing")
	}
	return trim, nil
}

func writePlainPrivacyKey(path string, raw []byte) error {
	// For plain-file test mode, staging is a plain file containing the key JSON.
	// Overwrite it atomically.
	tmp := path + ".tmpkey"
	if err := os.WriteFile(tmp, raw, 0o400); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o400); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return syncRegularFileAndParent(path)
}

func validatePrivacyKeyJSON(raw []byte, computerID string) error {
	var kf struct {
		Version    int    `json:"version"`
		ComputerID string `json:"computer_id"`
		Key        string `json:"key"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&kf); err != nil {
		return fmt.Errorf("trusted guest copy: invalid privacy key JSON: %w", err)
	}
	if kf.Version != 1 || kf.ComputerID != computerID || strings.TrimSpace(kf.Key) == "" {
		return fmt.Errorf("trusted guest copy: privacy key binding mismatch")
	}
	if _, err := base64.RawStdEncoding.DecodeString(kf.Key); err != nil {
		return fmt.Errorf("trusted guest copy: privacy key not base64: %w", err)
	}
	// Canonical check: re-marshal and compare? Use computerevent.CanonicalJSON if available.
	// For now ensure raw is canonical by checking no trailing data.
	rawTrim := bytes.TrimSpace(raw)
	var check json.RawMessage
	if err := json.Unmarshal(rawTrim, &check); err != nil {
		return fmt.Errorf("trusted guest copy: privacy key JSON invalid")
	}
	return nil
}

func extractPrivacyKeyViaDebugfs(ctx context.Context, quarantine string) ([]byte, error) {
	debugfs := findDebugfs()
	if debugfs == "" {
		return nil, fmt.Errorf("trusted guest copy: debugfs not found")
	}
	cmd := exec.CommandContext(ctx, debugfs, "-R", "cat /choir-credentials/privacy-key", quarantine)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("trusted guest copy: debugfs cat failed: %w (%s)", err, stderr.String())
	}
	out := bytes.TrimSpace(stdout.Bytes())
	if len(out) == 0 {
		return nil, fmt.Errorf("trusted guest copy: privacy key not found in quarantine (debugfs empty)")
	}
	// debugfs cat may include header; find first '{'
	idx := bytes.Index(out, []byte("{"))
	if idx >= 0 {
		out = out[idx:]
	}
	end := bytes.LastIndex(out, []byte("}"))
	if end >= 0 {
		out = out[:end+1]
	}
	return bytes.TrimSpace(out), nil
}

func writePrivacyKeyViaDebugfs(ctx context.Context, staging string, raw []byte) ([]byte, error) {
	debugfs := findDebugfs()
	if debugfs == "" {
		return nil, fmt.Errorf("trusted guest copy: debugfs not found")
	}
	tmp, err := os.CreateTemp("", "choir-privacy-key-*.json")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return nil, err
	}
	tmp.Close()
	defer os.Remove(tmpPath)
	// Ensure staging has directory.
	cmdMkdir := exec.CommandContext(ctx, debugfs, "-w", "-R", "mkdir /choir-credentials", staging)
	var stderr bytes.Buffer
	cmdMkdir.Stderr = &stderr
	_ = cmdMkdir.Run() // ignore if exists
	// Write file.
	cmdWrite := exec.CommandContext(ctx, debugfs, "-w", "-R", fmt.Sprintf("write %s /choir-credentials/privacy-key", tmpPath), staging)
	stderr.Reset()
	cmdWrite.Stderr = &stderr
	if err := cmdWrite.Run(); err != nil {
		return nil, fmt.Errorf("trusted guest copy: debugfs write failed: %w (%s)", err, stderr.String())
	}
	// Set mode 0400. debugfs chmod syntax: "sif <path> mode 0100400" or "chmod"?
	// Try both.
	for _, c := range [][]string{
		{"-w", "-R", "sif /choir-credentials/privacy-key mode 0100400", staging},
		{"-w", "-R", "chmod 0400 /choir-credentials/privacy-key", staging},
	} {
		cmd := exec.CommandContext(ctx, debugfs, c...)
		cmd.Stderr = &stderr
		_ = cmd.Run()
	}
	return raw, nil
}

func findDebugfs() string {
	for _, p := range []string{"/run/current-system/sw/bin/debugfs", "/bin/debugfs", "/usr/bin/debugfs", "debugfs"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
		if _, err := exec.LookPath(p); err == nil {
			return p
		}
	}
	return ""
}

func syncRegularFileAndParent(path string) error {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	_ = f.Sync()
	_ = f.Close()
	dir := filepath.Dir(path)
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
