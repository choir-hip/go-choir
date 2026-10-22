package vmmanager

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// Repair round 2 (bunch 31 review), the low findings on the vault:
//
// L1: the probe's claim "a mount point with a root-only mount root" was not
// checked: mountinfo field 4 (the root of the mount within its file system)
// was ignored, so a bind mount of a subdirectory passed. A mount whose root
// is not "/" is refused.
//
// L5: capture selection read the running config's computer id; with that id
// empty and VM_SNAPSHOT_VAULT_ONLY naming computers, the capture was not
// selected and landed in the state directory in plaintext. A computer
// without an id is refused when the list does not name its VM id.

func TestVaultProbeRefusesAMountWhoseRootIsNotTheFileSystemRoot(t *testing.T) {
	f := newVaultProbeFixture(t, "vault", "CRYPT-PLAIN-choir-snapshot-vault")
	f.appendMountinfo(t, fmt.Sprintf("77 22 254:3 /sub %s rw,relatime shared:12 - ext4 /dev/mapper/choir-snapshot-vault rw", escapeMountinfo(f.dir)))
	if err := probeVaultAt(f.dir, false, f.env); err == nil {
		t.Fatal("a bind mount of a subdirectory of the vault device was accepted")
	}
}

func TestVaultCaptureRefusesAComputerWithoutAnIDWhenTheListNamesComputers(t *testing.T) {
	f := newVaultFixture(t)
	f.mgr.cfg.SnapshotVaultOnly = []string{"computer-s3"}
	f.inst.Config.ComputerID = ""
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	if exists(f.vaultSnap()) {
		t.Fatal("a computer without an id was captured into the vault")
	}
	assertNoPlaintextSnapshot(t, f.snapshotFixture)
	if f.guest.prepares != 0 {
		t.Fatal("the guest was fenced although the capture could not be placed")
	}
	tl := readHibernateTimeline(t, f.mgr, f.vmID)
	if tl.Snapshot == nil || tl.Snapshot.Outcome != "kill" || tl.Snapshot.Reason != snapshotReasonVaultUnknownComputer {
		t.Fatalf("timeline = %+v", tl.Snapshot)
	}
}

func TestVaultCaptureOfAComputerWithoutAnIDNamedByVMIDStillUsesTheVault(t *testing.T) {
	f := newVaultFixture(t)
	f.mgr.cfg.SnapshotVaultOnly = []string{f.vmID}
	f.inst.Config.ComputerID = ""
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	if !exists(f.vaultSnap()) {
		t.Fatal("a VM named by its VM id was not captured into the vault")
	}
	assertNoPlaintextSnapshot(t, f.snapshotFixture)
}

// Repair round 1 of station FP9 (bunch 4b review, docs/definitions/
// choir-launch-fp9-keys-recovery-2026-10-21.md, "Review round 1").
//
// B1: the probe is a point in time; the writes after it went by path. The
// capture must be bound to the directory that was verified: opened once,
// checked, and every capture and commit path resolved through it, so a vault
// unmounted or replaced during the capture can never receive guest RAM on
// the main file system.
//
// B2: unsetting VM_SNAPSHOT_VAULT_DIR must not strand a vault snapshot: the
// switch selects where new snapshots are written; discovery keeps every
// vault root a snapshot was written to until that root is empty.

// B1, the reviewer's scenario: the vault is unmounted while the guest is
// being fenced, so the path becomes a plain directory on the main file
// system. Nothing may be written there, and the capture that went to the
// held (still encrypted) directory is removed from it.
func TestVaultCaptureIsBoundToTheVerifiedDirectoryNotThePath(t *testing.T) {
	f := newVaultFixture(t)
	held := f.vault + ".held"
	t.Cleanup(func() { _ = os.RemoveAll(held) })
	f.guest.onPrepare = func() {
		if err := os.Rename(f.vault, held); err != nil {
			t.Errorf("unmount stand-in: %v", err)
			return
		}
		if err := os.Mkdir(f.vault, 0o700); err != nil {
			t.Errorf("plain mount point: %v", err)
		}
	}
	// Where guest RAM lands at the moment it is written is the property;
	// a later cleanup cannot remove blocks already written.
	memInHeld, memAtPath := false, false
	f.fc.onCreate = func(_, _ string) {
		memInHeld = exists(filepath.Join(held, f.vmID, snapshotPartialDirName, snapshotMemName))
		memAtPath = exists(filepath.Join(f.vault, f.vmID, snapshotPartialDirName, snapshotMemName))
	}
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	if memAtPath || !memInHeld {
		t.Fatalf("guest RAM was written at the vault path (%v) / not in the held vault (%v)", memAtPath, !memInHeld)
	}
	if f.guest.prepares != 1 {
		t.Fatalf("prepares = %d", f.guest.prepares)
	}
	entries, err := os.ReadDir(f.vault)
	if err != nil || len(entries) != 0 {
		t.Fatalf("the plain directory at the vault path received files: %v %v", entries, err)
	}
	assertNoPlaintextSnapshot(t, f.snapshotFixture)
	if exists(filepath.Join(held, f.vmID, snapshotDirName)) || exists(filepath.Join(held, f.vmID, snapshotPartialDirName)) {
		t.Fatal("the capture in the held vault directory was not removed")
	}
	tl := readHibernateTimeline(t, f.mgr, f.vmID)
	if tl.Snapshot == nil || tl.Snapshot.Outcome != "kill" || tl.Snapshot.Reason != snapshotReasonCommitFailed {
		t.Fatalf("timeline = %+v", tl.Snapshot)
	}
}

// B1: the VMM is told paths that resolve through vmctl's open vault
// directory, never the vault path string.
func TestVaultCapturePathsResolveThroughTheHeldDirectory(t *testing.T) {
	f := newVaultFixture(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	if _, err := readSnapshotMeta(f.vaultSnap()); err != nil {
		t.Fatalf("no committed vault snapshot: %v", err)
	}
	var body map[string]any
	f.fc.mu.Lock()
	for _, c := range f.fc.calls {
		if c.Path == "/snapshot/create" {
			body = c.Body
		}
	}
	f.fc.mu.Unlock()
	prefix := fmt.Sprintf("/proc/%d/fd/", os.Getpid())
	for _, key := range []string{"snapshot_path", "mem_file_path"} {
		p, _ := body[key].(string)
		if !strings.HasPrefix(p, prefix) || strings.Contains(p, f.vault) {
			t.Fatalf("%s = %q; want a path through %s<fd>", key, p, prefix)
		}
	}
}

// B1: a handle that does not verify refuses before the fence.
func TestVaultHandleCheckFailureRefusesBeforeTheFence(t *testing.T) {
	f := newVaultFixture(t)
	f.mgr.vaultHandleCheck = func(*os.File, string, bool) error { return errors.New("not the dm-crypt device") }
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	if f.guest.prepares != 0 {
		t.Fatal("the guest was fenced although the vault handle did not verify")
	}
	if entries, _ := os.ReadDir(f.vault); len(entries) != 0 {
		t.Fatalf("the vault was written to: %v", entries)
	}
	assertNoPlaintextSnapshot(t, f.snapshotFixture)
	tl := readHibernateTimeline(t, f.mgr, f.vmID)
	if tl.Snapshot == nil || tl.Snapshot.Reason != snapshotReasonVaultUnavailable {
		t.Fatalf("timeline = %+v", tl.Snapshot)
	}
}

func majminOfPath(t *testing.T, path string) string {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Skip("no stat_t on this platform")
	}
	return devMajmin(uint64(st.Dev))
}

// B1, the production handle check over a fixture mount table and sysfs
// whose device is the real device of the test directory.
func TestVaultHandleCheckOverFixtures(t *testing.T) {
	f := newVaultProbeFixture(t, "vault", "")
	majmin := majminOfPath(t, f.dir)
	writeUUID := func(uuid string) {
		dm := filepath.Join(f.env.SysRoot, "dev", "block", majmin, "dm")
		if err := os.MkdirAll(dm, 0o755); err != nil {
			t.Fatal(err)
		}
		_ = os.Remove(filepath.Join(dm, "uuid"))
		if err := os.WriteFile(filepath.Join(dm, "uuid"), []byte(uuid+"\n"), 0o444); err != nil {
			t.Fatal(err)
		}
	}
	f.writeMountinfo(t, escapeMountinfo(f.dir), majmin, "ext4", "/dev/mapper/choir-snapshot-vault")
	writeUUID("CRYPT-PLAIN-choir-snapshot-vault")
	h, err := os.Open(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := verifyVaultHandleAt(h, f.dir, false, f.env); err != nil {
		t.Fatalf("a held plain-crypt vault directory was refused: %v", err)
	}
	writeUUID("CRYPT-LUKS2-abc-choir")
	if err := verifyVaultHandleAt(h, f.dir, false, f.env); err == nil {
		t.Fatal("a LUKS device was accepted without the flag")
	}
	if err := verifyVaultHandleAt(h, f.dir, true, f.env); err != nil {
		t.Fatalf("a LUKS device was refused with the flag: %v", err)
	}
	writeUUID("CRYPT-PLAIN-choir-snapshot-vault")
	// The mount table names another device for the path: the open
	// directory is not on the mounted device.
	f.writeMountinfo(t, escapeMountinfo(f.dir), "254:3", "ext4", "/dev/mapper/choir-snapshot-vault")
	if err := verifyVaultHandleAt(h, f.dir, false, f.env); err == nil {
		t.Fatal("an open directory on another device than the mount was accepted")
	}
	f.writeMountinfo(t, escapeMountinfo(f.dir), majmin, "ext4", "/dev/mapper/choir-snapshot-vault")
	// The path now names another directory.
	if err := os.Rename(f.dir, f.dir+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(f.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := verifyVaultHandleAt(h, f.dir, false, f.env); err == nil {
		t.Fatal("a held directory no longer at the path was accepted")
	}
}

// rolledBack is a new manager over the same state directory with the vault
// switch unset, as after the rollback in the station file.
func rolledBack(f *vaultFixture, probe func(dir string, accept bool) error) *Manager {
	cfg := f.mgr.cfg
	cfg.SnapshotVaultDir = ""
	cfg.SnapshotVaultOnly = nil
	m := NewManager(cfg)
	m.vaultProbe = probe
	// The host floor fails a start whose rules do not install (SEC-03): the
	// rolled-back manager keeps the fixture's fake tap networking.
	m.hostNet = f.mgr.hostNet
	m.vms[f.vmID] = f.inst
	return m
}

// B2: a vault snapshot written before the switch was unset is listed,
// reserved and resumed from the vault, and the vault is probed first.
func TestVaultUnsetSwitchStillFindsAndResumesTheVaultSnapshot(t *testing.T) {
	f := newVaultFixture(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	meta, err := readSnapshotMeta(f.vaultSnap())
	if err != nil {
		t.Fatal(err)
	}
	var probed []string
	f.mgr = rolledBack(f, func(dir string, _ bool) error { probed = append(probed, dir); return nil })
	entries := f.mgr.listSnapshots()
	if len(entries) != 1 || entries[0].dir != f.vaultSnap() {
		t.Fatalf("listSnapshots after the switch was unset = %+v", entries)
	}
	f.mgr.loadSnapshotReservations()
	if !f.mgr.snapshotPortHeldByOther("other-vm", meta.HostPort) {
		t.Fatal("the vault snapshot's port is not reserved after the switch was unset")
	}
	restoreAPI := &fakeFirecrackerAPI{}
	var args []string
	f.restoreStarter(t, restoreAPI, &args)
	if _, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, nil); err != nil {
		t.Fatalf("resume after the switch was unset: %v", err)
	}
	if restoreAPI.loadMem != filepath.Join(f.vaultSnap(), snapshotMemName) {
		t.Fatalf("loaded %s, want the vault's", restoreAPI.loadMem)
	}
	if len(probed) == 0 || probed[len(probed)-1] != f.vault {
		t.Fatalf("the recorded vault was not probed before the resume: %v", probed)
	}
	if exists(filepath.Join(f.vault, f.vmID)) {
		t.Fatal("the vault still holds the resumed VM")
	}
}

// B2: a recorded vault that no longer probes is never loaded; its snapshot
// is discarded (cold wake), and nothing new is written into it.
func TestVaultUnsetSwitchRefusesAnUnverifiedRecordedVault(t *testing.T) {
	f := newVaultFixture(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	f.mgr = rolledBack(f, func(string, bool) error { return errors.New("not mounted") })
	started := false
	f.mgr.firecrackerStarter = func(string, []string) error { started = true; return nil }
	_, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, nil)
	var fb *SnapshotFallbackError
	if !errors.As(err, &fb) || fb.Reason != snapshotReasonVaultUnavailable || started {
		t.Fatalf("resume = %v started=%v; want vault_unavailable without a start", err, started)
	}
	if exists(f.vaultSnap()) {
		t.Fatal("the unverified vault snapshot was kept")
	}
}

// B2: with the switch unset, a new capture goes to the state directory as
// on main, never into the recorded vault.
func TestVaultUnsetSwitchWritesNewSnapshotsToTheStateDirectory(t *testing.T) {
	f := newVaultFixture(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	f.mgr = rolledBack(f, func(string, bool) error { return nil })
	// The VM resumes (consuming the vault copy), then hibernates again.
	restoreAPI := &fakeFirecrackerAPI{memBytes: 4096}
	var args []string
	f.restoreStarter(t, restoreAPI, &args)
	if _, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, nil); err != nil {
		t.Fatal(err)
	}
	f.inst.State = StateRunning
	f.mgr.vms[f.vmID] = f.inst
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(f.vault, f.vmID)) {
		t.Fatal("a capture with the switch unset was written into the recorded vault")
	}
	if _, err := readSnapshotMeta(f.mgr.snapshotDir(f.vmID)); err != nil {
		t.Fatalf("no snapshot in the state directory: %v", err)
	}
}

// B2: the record is written when a snapshot goes into the vault, and the
// boot sweep drops a recorded root that is no longer configured once it
// holds no snapshot (and keeps it while it does).
func TestVaultRootRecordLifecycle(t *testing.T) {
	f := newVaultFixture(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(f.mgr.cfg.StateDir, snapshotVaultRootsFile)
	raw, err := os.ReadFile(record)
	if err != nil || strings.TrimSpace(string(raw)) != f.vault {
		t.Fatalf("record = %q, %v; want the vault path", raw, err)
	}
	if info, err := os.Stat(record); err != nil || info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("record mode = %v %v", info, err)
	}
	f.mgr = rolledBack(f, func(string, bool) error { return nil })
	f.mgr.SweepOrphanSnapshots(func(string) bool { return true })
	if raw, _ := os.ReadFile(record); strings.TrimSpace(string(raw)) != f.vault {
		t.Fatal("the record was dropped while its vault still holds a snapshot")
	}
	f.mgr.SweepOrphanSnapshots(func(string) bool { return false })
	if exists(f.vaultSnap()) {
		t.Fatal("the orphaned vault snapshot was not swept")
	}
	f.mgr.SweepOrphanSnapshots(nil)
	if raw, err := os.ReadFile(record); err == nil && strings.TrimSpace(string(raw)) != "" {
		t.Fatalf("the record of an empty, unconfigured vault was kept: %q", raw)
	}
}
