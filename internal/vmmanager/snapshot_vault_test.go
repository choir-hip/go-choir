package vmmanager

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// SDC NET-06 / review F12: guest memory must not rest in plaintext on the
// host's disks. The snapshot vault is an ephemeral-key dm-crypt plain volume;
// vmmanager writes a vault computer's snapshot only into a mount it has just
// probed as such, and takes the kill path (a cold wake) otherwise.
//
// The failure modes below are written in the goal file
// (docs/archive/definitions/choir-launch-fp9-keys-recovery-2026-10-21.md, "Failure
// modes, written before code", V1 to V15) before this code. The probe tests
// run the real probe over a fixture mountinfo and a fixture sysfs: no test
// weakens the probe to pass on a development machine.

// --- the real probe over fixtures (V1 to V6) -------------------------------

type vaultProbeFixture struct {
	dir  string
	env  vaultProbeEnv
	uuid string
}

// newVaultProbeFixture builds a 0700 directory standing for the vault mount,
// a mountinfo file that mounts it from major:minor 254:3, and a sysfs tree
// whose dm/uuid for 254:3 is uuid ("" writes no dm directory at all).
func newVaultProbeFixture(t *testing.T, name, uuid string) *vaultProbeFixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, name)
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	f := &vaultProbeFixture{dir: dir, uuid: uuid}
	f.env = vaultProbeEnv{
		Mountinfo: filepath.Join(root, "mountinfo"),
		SysRoot:   filepath.Join(root, "sys"),
		OwnerOf:   func(os.FileInfo) (uint32, bool) { return 0, true },
	}
	f.writeMountinfo(t, escapeMountinfo(dir), "254:3", "ext4", "/dev/mapper/choir-snapshot-vault")
	if uuid != "" {
		dm := filepath.Join(f.env.SysRoot, "dev", "block", "254:3", "dm")
		if err := os.MkdirAll(dm, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dm, "uuid"), []byte(uuid+"\n"), 0o444); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func escapeMountinfo(path string) string {
	r := strings.NewReplacer(`\`, `\134`, " ", `\040`, "\t", `\011`, "\n", `\012`)
	return r.Replace(path)
}

// writeMountinfo writes a mountinfo with the root mount and one entry for the
// vault; extra lines may be appended by the caller afterwards.
func (f *vaultProbeFixture) writeMountinfo(t *testing.T, mountPoint, majmin, fstype, source string) {
	t.Helper()
	lines := []string{
		"22 1 8:1 / / rw,relatime shared:1 - ext4 /dev/sda1 rw",
		fmt.Sprintf("55 22 %s / %s rw,relatime shared:9 - %s %s rw", majmin, mountPoint, fstype, source),
	}
	if err := os.WriteFile(f.env.Mountinfo, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *vaultProbeFixture) appendMountinfo(t *testing.T, line string) {
	t.Helper()
	fh, err := os.OpenFile(f.env.Mountinfo, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	if _, err := fh.WriteString(line + "\n"); err != nil {
		t.Fatal(err)
	}
}

func TestVaultProbeAcceptsAnEphemeralPlainCryptMount(t *testing.T) {
	f := newVaultProbeFixture(t, "vault", "CRYPT-PLAIN-choir-snapshot-vault")
	if err := probeVaultAt(f.dir, false, f.env); err != nil {
		t.Fatalf("a plain dm-crypt mount was refused: %v", err)
	}
}

// V1: the mount did not happen; the vault path is a plain directory.
func TestVaultProbeRefusesAPathThatIsNotAMountPoint(t *testing.T) {
	f := newVaultProbeFixture(t, "vault", "CRYPT-PLAIN-choir-snapshot-vault")
	f.writeMountinfo(t, "/somewhere/else", "254:3", "ext4", "/dev/mapper/x")
	err := probeVaultAt(f.dir, false, f.env)
	if err == nil || !strings.Contains(err.Error(), "mount point") {
		t.Fatalf("a directory that is not a mount point was accepted: %v", err)
	}
}

// V2: the mount is on a device that is not device-mapper (a loop device, a
// plain partition, tmpfs): no dm/uuid exists.
func TestVaultProbeRefusesAMountOnANonDeviceMapperDevice(t *testing.T) {
	f := newVaultProbeFixture(t, "vault", "")
	err := probeVaultAt(f.dir, false, f.env)
	if err == nil || !strings.Contains(err.Error(), "device-mapper") {
		t.Fatalf("a non device-mapper mount was accepted: %v", err)
	}
	err = probeVaultAt(f.dir, true, f.env)
	if err == nil {
		t.Fatal("the accept-any-crypt flag must not accept a non device-mapper mount")
	}
}

// V2: a device-mapper device that is not a crypt target (LVM, verity).
func TestVaultProbeRefusesANonCryptDeviceMapperTarget(t *testing.T) {
	for _, uuid := range []string{"LVM-abcdef", "CRYPT", "crypt-plain-x", "VERITY-x"} {
		f := newVaultProbeFixture(t, "vault", uuid)
		if err := probeVaultAt(f.dir, true, f.env); err == nil {
			t.Fatalf("dm uuid %q was accepted even with accept-any-crypt", uuid)
		}
		if err := probeVaultAt(f.dir, false, f.env); err == nil {
			t.Fatalf("dm uuid %q was accepted", uuid)
		}
	}
}

// V3: LUKS keeps a key slot on disk; only plain is accepted unless the
// operator says otherwise.
func TestVaultProbeAcceptsLUKSOnlyWhenAnyCryptIsAllowed(t *testing.T) {
	f := newVaultProbeFixture(t, "vault", "CRYPT-LUKS2-0123456789abcdef-vault")
	err := probeVaultAt(f.dir, false, f.env)
	if err == nil || !strings.Contains(err.Error(), "plain") {
		t.Fatalf("a LUKS mapping was accepted as an ephemeral vault: %v", err)
	}
	if err := probeVaultAt(f.dir, true, f.env); err != nil {
		t.Fatalf("LUKS refused although accept-any-crypt is set: %v", err)
	}
}

// V4: nothing is assumed when the host cannot be read.
func TestVaultProbeRefusesWhenTheHostCannotBeRead(t *testing.T) {
	f := newVaultProbeFixture(t, "vault", "CRYPT-PLAIN-choir-snapshot-vault")
	missingMountinfo := f.env
	missingMountinfo.Mountinfo = filepath.Join(filepath.Dir(f.dir), "no-such-mountinfo")
	if err := probeVaultAt(f.dir, false, missingMountinfo); err == nil {
		t.Fatal("an unreadable mountinfo was accepted")
	}
	missingSys := f.env
	missingSys.SysRoot = filepath.Join(filepath.Dir(f.dir), "no-such-sys")
	if err := probeVaultAt(f.dir, false, missingSys); err == nil {
		t.Fatal("an unreadable sysfs was accepted")
	}
	if runtime.GOOS != "linux" {
		if err := probeSnapshotVault(f.dir, false); err == nil {
			t.Fatalf("the real probe accepted a vault on %s, which has no /proc/self/mountinfo", runtime.GOOS)
		}
	}
}

// V5: the path itself must be absolute, clean, a directory and not a link.
func TestVaultProbeRefusesAnUnsafePath(t *testing.T) {
	f := newVaultProbeFixture(t, "vault", "CRYPT-PLAIN-choir-snapshot-vault")
	link := filepath.Join(filepath.Dir(f.dir), "link")
	if err := os.Symlink(f.dir, link); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(filepath.Dir(f.dir), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{
		"symlink":      link,
		"relative":     "vault",
		"empty":        "",
		"not clean":    f.dir + "/../vault",
		"trailing dot": f.dir + "/.",
		"regular file": file,
		"missing":      f.dir + "-missing",
	} {
		if err := probeVaultAt(path, true, f.env); err == nil {
			t.Fatalf("%s path %q was accepted", name, path)
		}
	}
}

// V5: a subdirectory of an encrypted mount is not the mount itself.
func TestVaultProbeRefusesASubdirectoryOfAMount(t *testing.T) {
	f := newVaultProbeFixture(t, "vault", "CRYPT-PLAIN-choir-snapshot-vault")
	sub := filepath.Join(f.dir, "sub")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := probeVaultAt(sub, false, f.env); err == nil {
		t.Fatal("a subdirectory of the vault mount was accepted")
	}
}

// V6: the mount's root directory must be root's alone.
func TestVaultProbeRefusesALooseOrForeignDirectory(t *testing.T) {
	f := newVaultProbeFixture(t, "vault", "CRYPT-PLAIN-choir-snapshot-vault")
	if err := os.Chmod(f.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := probeVaultAt(f.dir, false, f.env); err == nil {
		t.Fatal("a world-readable vault directory was accepted")
	}
	if err := os.Chmod(f.dir, 0o710); err != nil {
		t.Fatal(err)
	}
	if err := probeVaultAt(f.dir, false, f.env); err == nil {
		t.Fatal("a group-accessible vault directory was accepted")
	}
	if err := os.Chmod(f.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	f.env.OwnerOf = func(os.FileInfo) (uint32, bool) { return 1000, true }
	if err := probeVaultAt(f.dir, false, f.env); err == nil {
		t.Fatal("a vault directory owned by another user was accepted")
	}
	f.env.OwnerOf = func(os.FileInfo) (uint32, bool) { return 0, false }
	if err := probeVaultAt(f.dir, false, f.env); err == nil {
		t.Fatal("a vault directory with an unknown owner was accepted")
	}
}

// Another mount stacked on the vault path hides the encrypted one: the last
// mountinfo entry for the path is the one that serves files.
func TestVaultProbeUsesTheLastMountOnThePath(t *testing.T) {
	f := newVaultProbeFixture(t, "vault", "CRYPT-PLAIN-choir-snapshot-vault")
	f.appendMountinfo(t, fmt.Sprintf("77 22 0:44 / %s rw - tmpfs tmpfs rw", escapeMountinfo(f.dir)))
	if err := probeVaultAt(f.dir, false, f.env); err == nil {
		t.Fatal("a vault shadowed by a later non-crypt mount was accepted")
	}
}

// mountinfo octal escapes in the mount point.
func TestVaultProbeReadsEscapedMountPoints(t *testing.T) {
	f := newVaultProbeFixture(t, "vault dir", "CRYPT-PLAIN-choir-snapshot-vault")
	if err := probeVaultAt(f.dir, false, f.env); err != nil {
		t.Fatalf("a mount point with a space was refused: %v", err)
	}
}

// --- the manager over a fake probe (V7 to V15) ------------------------------

type vaultFixture struct {
	*snapshotFixture
	vault string
	calls *int
}

// newVaultFixture is a snapshot-mode manager whose vault is a temp directory
// and whose probe says yes. Tests replace f.mgr.vaultProbe to refuse.
func newVaultFixture(t *testing.T) *vaultFixture {
	t.Helper()
	// A vault capture resolves its paths through /proc/<pid>/fd (finding
	// B1); only Linux has it. The vault is refused on any other host.
	if runtime.GOOS != "linux" {
		t.Skip("the snapshot vault needs Linux /proc/<pid>/fd")
	}
	f := newSnapshotFixture(t)
	vault := shortStateDir(t)
	if err := os.Chmod(vault, 0o700); err != nil {
		t.Fatal(err)
	}
	f.mgr.cfg.SnapshotVaultDir = vault
	calls := 0
	f.mgr.vaultProbe = func(string, bool) error { calls++; return nil }
	// The device half of the handle check needs a real dm-crypt device; the
	// identity half (the open directory is still the one at the path) is the
	// production code.
	f.mgr.vaultHandleCheck = func(h *os.File, dir string, _ bool) error { return sameOpenDirectory(h, dir) }
	f.serveLaunchAPI(t)
	return &vaultFixture{snapshotFixture: f, vault: vault, calls: &calls}
}

func (f *vaultFixture) vaultSnap() string { return filepath.Join(f.vault, f.vmID, "snapshot") }
func (f *vaultFixture) vaultPartial() string {
	return filepath.Join(f.vault, f.vmID, "snapshot.partial")
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// assertNoPlaintextSnapshot is the property the whole change exists for: no
// committed or partial snapshot of the VM is in the state directory.
func assertNoPlaintextSnapshot(t *testing.T, f *snapshotFixture) {
	t.Helper()
	for _, dir := range []string{f.mgr.snapshotDir(f.vmID), f.mgr.snapshotPartialDir(f.vmID)} {
		if exists(dir) {
			t.Fatalf("a snapshot is in the state directory: %s", dir)
		}
	}
}

func TestVaultHibernateWritesTheSnapshotOnlyInTheVault(t *testing.T) {
	f := newVaultFixture(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatalf("HibernateVM: %v", err)
	}
	meta, err := readSnapshotMeta(f.vaultSnap())
	if err != nil {
		t.Fatalf("no committed snapshot in the vault: %v", err)
	}
	if !meta.Vault {
		t.Fatal("the metadata does not record that the snapshot is vault resident")
	}
	assertNoPlaintextSnapshot(t, f.snapshotFixture)
	for _, name := range []string{snapshotStateName, snapshotMemName, snapshotMetaName} {
		info, err := os.Stat(filepath.Join(f.vaultSnap(), name))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %v, %v; want 0600", name, info, err)
		}
	}
	for _, dir := range []string{filepath.Join(f.vault, f.vmID), f.vaultSnap()} {
		if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
			t.Fatalf("%s mode = %v, %v; want 0700", dir, info, err)
		}
	}
	if !f.mgr.snapshotPortHeldByOther("other-vm", meta.HostPort) {
		t.Fatal("a vault snapshot must reserve its host port")
	}
	if *f.calls < 2 {
		t.Fatalf("the vault was probed %d times; want before the capture and again before the commit", *f.calls)
	}
	tl := readHibernateTimeline(t, f.mgr, f.vmID)
	if tl.Snapshot == nil || tl.Snapshot.Outcome != "snapshot" || !tl.Snapshot.Vault {
		t.Fatalf("hibernate timeline = %+v", tl.Snapshot)
	}
}

func TestVaultRoundTripResumesFromTheVaultAndLeavesNothingBehind(t *testing.T) {
	f := newVaultFixture(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	probedBefore := *f.calls
	restoreAPI := &fakeFirecrackerAPI{}
	var restoreArgs []string
	f.restoreStarter(t, restoreAPI, &restoreArgs)
	if _, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, nil); err != nil {
		t.Fatalf("ResumeVMFromSnapshot: %v", err)
	}
	if restoreAPI.loadState != filepath.Join(f.vaultSnap(), snapshotStateName) || restoreAPI.loadMem != filepath.Join(f.vaultSnap(), snapshotMemName) {
		t.Fatalf("load paths = %s %s, want the vault's", restoreAPI.loadState, restoreAPI.loadMem)
	}
	if *f.calls <= probedBefore {
		t.Fatal("the vault was not probed again before the resume")
	}
	if exists(f.vaultSnap()) || exists(filepath.Join(f.vault, f.vmID)) {
		t.Fatal("the vault must be empty of this VM after a successful resume")
	}
	assertNoPlaintextSnapshot(t, f.snapshotFixture)
}

// V1 to V6 at the manager: a probe that refuses means no fence, no pause, no
// snapshot call, nothing written anywhere, and the kill path.
func TestVaultProbeFailureRefusesTheSnapshotAndWritesNothing(t *testing.T) {
	f := newVaultFixture(t)
	f.mgr.vaultProbe = func(string, bool) error { return errors.New("vault is a plain directory") }
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatalf("HibernateVM: %v", err)
	}
	if f.inst.State != StateHibernated {
		t.Fatalf("state = %s", f.inst.State)
	}
	if f.guest.prepares != 0 {
		t.Fatal("the guest was fenced although the vault was refused")
	}
	for _, call := range f.fc.paths() {
		if call != "GET /" {
			t.Fatalf("Firecracker was driven (%v) although the vault was refused", f.fc.paths())
		}
	}
	entries, err := os.ReadDir(f.vault)
	if err != nil || len(entries) != 0 {
		t.Fatalf("the vault was written to: %v %v", entries, err)
	}
	assertNoPlaintextSnapshot(t, f.snapshotFixture)
	tl := readHibernateTimeline(t, f.mgr, f.vmID)
	if tl.BootKind != "hibernate_kill" || tl.Snapshot == nil || tl.Snapshot.Outcome != "kill" || tl.Snapshot.Reason != snapshotReasonVaultUnavailable {
		t.Fatalf("timeline = %s %+v", tl.BootKind, tl.Snapshot)
	}
}

// V7: the vault disappears between the capture and the commit.
func TestVaultProbeFailureAtCommitDiscardsTheCapture(t *testing.T) {
	f := newVaultFixture(t)
	f.mgr.vaultProbe = func(string, bool) error {
		*f.calls++
		if *f.calls >= 2 {
			return errors.New("vault went away")
		}
		return nil
	}
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(f.fc.paths(), ","), "PUT /snapshot/create") {
		t.Fatalf("the capture never ran: %v", f.fc.paths())
	}
	if exists(f.vaultSnap()) || exists(f.vaultPartial()) {
		t.Fatal("a capture whose commit probe failed must not survive in the vault")
	}
	assertNoPlaintextSnapshot(t, f.snapshotFixture)
	tl := readHibernateTimeline(t, f.mgr, f.vmID)
	if tl.Snapshot == nil || tl.Snapshot.Outcome != "kill" || tl.Snapshot.Reason != snapshotReasonCommitFailed {
		t.Fatalf("timeline = %+v", tl.Snapshot)
	}
}

// V12: a jailed VMM captures inside the jail on the main file system.
func TestVaultJailedCaptureIsRefused(t *testing.T) {
	f := newVaultFixture(t)
	f.inst.confinement = &ConfinementReceipt{Mode: ConfinementJailer, UID: 900001, GID: 900001}
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	if f.guest.prepares != 0 {
		t.Fatal("a jailed VMM was fenced for a vault snapshot")
	}
	entries, _ := os.ReadDir(f.vault)
	if len(entries) != 0 {
		t.Fatalf("the vault was written to: %v", entries)
	}
	assertNoPlaintextSnapshot(t, f.snapshotFixture)
	tl := readHibernateTimeline(t, f.mgr, f.vmID)
	if tl.Snapshot == nil || tl.Snapshot.Reason != snapshotReasonVaultJailed {
		t.Fatalf("timeline = %+v", tl.Snapshot)
	}
}

// V9: computers outside the ONLY list behave exactly as on main.
func TestVaultSelectionByComputerOrVMID(t *testing.T) {
	cases := []struct {
		name      string
		only      []string
		wantVault bool
	}{
		{"empty list selects every computer", nil, true},
		{"computer id", []string{"computer-s3"}, true},
		{"vm id", []string{"vm-s3"}, true},
		{"another computer", []string{"computer-other"}, false},
		{"another vm", []string{"vm-other", "computer-other"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newVaultFixture(t)
			f.mgr.cfg.SnapshotVaultOnly = tc.only
			if err := f.mgr.HibernateVM(f.vmID); err != nil {
				t.Fatal(err)
			}
			inVault := exists(f.vaultSnap())
			inState := exists(f.mgr.snapshotDir(f.vmID))
			if tc.wantVault && (!inVault || inState) {
				t.Fatalf("vault=%v state=%v; want the snapshot in the vault only", inVault, inState)
			}
			if !tc.wantVault {
				if inVault || !inState {
					t.Fatalf("vault=%v state=%v; want the snapshot in the state directory only", inVault, inState)
				}
				if *f.calls != 0 {
					t.Fatal("an unselected computer touched the vault probe")
				}
				meta, err := readSnapshotMeta(f.mgr.snapshotDir(f.vmID))
				if err != nil || meta.Vault {
					t.Fatalf("legacy snapshot meta = %+v, %v", meta, err)
				}
			}
		})
	}
}

// V15: with the vault unset nothing changes and the probe is never called.
func TestVaultUnsetKeepsTheStateDirectoryLayout(t *testing.T) {
	f := newSnapshotFixture(t)
	f.serveLaunchAPI(t)
	f.mgr.vaultProbe = func(string, bool) error { t.Fatal("the vault probe ran with no vault configured"); return nil }
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	if !exists(f.mgr.snapshotDir(f.vmID)) {
		t.Fatal("no snapshot in the state directory")
	}
	restoreAPI := &fakeFirecrackerAPI{}
	var args []string
	f.restoreStarter(t, restoreAPI, &args)
	if _, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, nil); err != nil {
		t.Fatalf("resume: %v", err)
	}
}

// V8: a plaintext snapshot found for a computer that is now a vault computer
// is not loaded: it is discarded and the computer cold-boots.
func TestVaultResumeRefusesAPlaintextSnapshotOfAVaultComputer(t *testing.T) {
	f := newVaultFixture(t)
	f.mgr.cfg.SnapshotVaultDir = "" // capture before the vault is turned on
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	if !exists(f.mgr.snapshotDir(f.vmID)) {
		t.Fatal("setup: no plaintext snapshot")
	}
	f.mgr.cfg.SnapshotVaultDir = f.vault
	started := false
	f.mgr.firecrackerStarter = func(string, []string) error { started = true; return nil }
	_, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, nil)
	var fb *SnapshotFallbackError
	if !errors.As(err, &fb) || fb.Reason != snapshotReasonNotInVault || started {
		t.Fatalf("resume = %v started=%v; want a %s fallback without a start", err, started, snapshotReasonNotInVault)
	}
	assertNoPlaintextSnapshot(t, f.snapshotFixture)
}

// A snapshot in the vault is not loaded when the vault no longer probes as an
// encrypted mount (a swapped device, a remount).
func TestVaultResumeRefusesWhenTheVaultNoLongerProbes(t *testing.T) {
	f := newVaultFixture(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	f.mgr.vaultProbe = func(string, bool) error { return errors.New("not dm-crypt any more") }
	started := false
	f.mgr.firecrackerStarter = func(string, []string) error { started = true; return nil }
	_, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, nil)
	var fb *SnapshotFallbackError
	if !errors.As(err, &fb) || fb.Reason != snapshotReasonVaultUnavailable || started {
		t.Fatalf("resume = %v started=%v; want a %s fallback without a start", err, started, snapshotReasonVaultUnavailable)
	}
	if exists(f.vaultSnap()) {
		t.Fatal("a refused vault snapshot must be discarded")
	}
}

// V13: after a host reboot the vault is a fresh empty volume.
func TestVaultEmptyAfterRebootColdBoots(t *testing.T) {
	f := newVaultFixture(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(f.vault, f.vmID)); err != nil {
		t.Fatal(err)
	}
	started := false
	f.mgr.firecrackerStarter = func(string, []string) error { started = true; return nil }
	_, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, nil)
	var fb *SnapshotFallbackError
	if !errors.As(err, &fb) || fb.Reason != snapshotReasonNoSnapshot || started {
		t.Fatalf("resume = %v started=%v; want no_snapshot without a start", err, started)
	}
}

// V10: vmctl restarts with a snapshot in the vault; its port stays reserved
// and the budget sees it.
func TestVaultSnapshotsSurviveAVmctlRestartInListingAndReservations(t *testing.T) {
	f := newVaultFixture(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	meta, err := readSnapshotMeta(f.vaultSnap())
	if err != nil {
		t.Fatal(err)
	}
	restarted := NewManager(f.mgr.cfg)
	restarted.vaultProbe = func(string, bool) error { return nil }
	restarted.loadSnapshotReservations()
	if !restarted.snapshotPortHeldByOther("other-vm", meta.HostPort) {
		t.Fatal("the vault snapshot's port was not re-reserved after a restart")
	}
	entries := restarted.listSnapshots()
	if len(entries) != 1 || entries[0].vmID != f.vmID || entries[0].dir != f.vaultSnap() {
		t.Fatalf("listSnapshots = %+v", entries)
	}
}

func TestVaultSweepRemovesAnOrphanedVaultSnapshotAndKeepsAHibernatedOne(t *testing.T) {
	f := newVaultFixture(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	if removed := f.mgr.SweepOrphanSnapshots(func(string) bool { return true }); len(removed) != 0 {
		t.Fatalf("a hibernated VM's vault snapshot was swept: %v", removed)
	}
	removed := f.mgr.SweepOrphanSnapshots(func(string) bool { return false })
	if len(removed) != 1 || removed[0] != f.vaultSnap() {
		t.Fatalf("removed = %v, want %s", removed, f.vaultSnap())
	}
	if exists(f.vaultSnap()) {
		t.Fatal("the orphaned vault snapshot is still there")
	}
}

// V8 at boot: a plaintext snapshot left in the state directory for a vault
// computer is removed even though its VM is hibernated; one for a computer
// outside the ONLY list is kept.
func TestVaultSweepRemovesLeftoverPlaintextOfAVaultComputerOnly(t *testing.T) {
	f := newVaultFixture(t)
	f.mgr.cfg.SnapshotVaultDir = ""
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	f.mgr.cfg.SnapshotVaultDir = f.vault

	f.mgr.cfg.SnapshotVaultOnly = []string{"computer-other"}
	if removed := f.mgr.SweepOrphanSnapshots(func(string) bool { return true }); len(removed) != 0 {
		t.Fatalf("a snapshot of a computer outside the vault list was swept: %v", removed)
	}
	if !exists(f.mgr.snapshotDir(f.vmID)) {
		t.Fatal("setup: the plaintext snapshot is gone")
	}

	f.mgr.cfg.SnapshotVaultOnly = []string{"computer-s3"}
	removed := f.mgr.SweepOrphanSnapshots(func(string) bool { return true })
	if len(removed) != 1 || removed[0] != f.mgr.snapshotDir(f.vmID) {
		t.Fatalf("removed = %v, want the plaintext snapshot", removed)
	}
	assertNoPlaintextSnapshot(t, f.snapshotFixture)
}

func TestVaultDiscardRemovesTheSnapshotItsDirectoryAndTheReservation(t *testing.T) {
	f := newVaultFixture(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	meta, _ := readSnapshotMeta(f.vaultSnap())
	if !f.mgr.hasSnapshotFiles(f.vmID) {
		t.Fatal("hasSnapshotFiles missed the vault snapshot")
	}
	if !f.mgr.DiscardSnapshot(f.vmID, "test") {
		t.Fatal("DiscardSnapshot reported nothing removed")
	}
	if exists(filepath.Join(f.vault, f.vmID)) {
		t.Fatal("the VM's vault directory is left behind")
	}
	if f.mgr.snapshotPortHeldByOther("other-vm", meta.HostPort) {
		t.Fatal("the discarded snapshot still holds its port")
	}
}

// V11: the budget counts the vault's snapshots, and the free-space floor is
// measured on the file system the snapshot will occupy.
func TestVaultRoomIsMeasuredOnTheVaultFilesystem(t *testing.T) {
	stateDir := shortStateDir(t)
	vault := shortStateDir(t)
	cfg := DefaultManagerConfig()
	cfg.StateDir = stateDir
	cfg.SnapshotVaultDir = vault
	mgr := NewManager(cfg)
	prev := statfsAvailable
	var asked []string
	need := int64(4) << 30
	avail := need + int64(3)<<30 // 3 GiB to spare
	statfsAvailable = func(path string) (int64, error) { asked = append(asked, path); return avail, nil }
	t.Cleanup(func() { statfsAvailable = prev })

	if !mgr.ensureSnapshotRoomAt(vault, "vm-x", need) {
		t.Fatal("3 GiB to spare on the vault must be enough (its floor is small: only snapshots live there)")
	}
	if mgr.ensureSnapshotRoomAt(stateDir, "vm-x", need) {
		t.Fatal("3 GiB to spare on the state directory is below its 16 GiB floor")
	}
	if len(asked) != 2 || asked[0] != vault || asked[1] != stateDir {
		t.Fatalf("statfs asked about %v; want the vault then the state directory", asked)
	}
	avail = need + snapshotVaultFloorBytes - 1
	if mgr.ensureSnapshotRoomAt(vault, "vm-x", need) {
		t.Fatal("the vault floor must refuse a snapshot that would cross it")
	}
}

func TestVaultConfigFromEnv(t *testing.T) {
	if cfg := LoadConfigFromEnv(); cfg.SnapshotVaultDir != "" || len(cfg.SnapshotVaultOnly) != 0 || cfg.SnapshotVaultAcceptAnyCrypt {
		t.Fatalf("the vault must be off by default: %+v", cfg)
	}
	t.Setenv("VM_SNAPSHOT_VAULT_DIR", "/var/lib/go-choir/snapshot-vault")
	t.Setenv("VM_SNAPSHOT_VAULT_ONLY", "computer-a, vm-b ,,")
	t.Setenv("VM_SNAPSHOT_VAULT_ACCEPT_ANY_CRYPT", "on")
	cfg := LoadConfigFromEnv()
	if cfg.SnapshotVaultDir != "/var/lib/go-choir/snapshot-vault" || !cfg.SnapshotVaultAcceptAnyCrypt ||
		strings.Join(cfg.SnapshotVaultOnly, "|") != "computer-a|vm-b" {
		t.Fatalf("env config = %+v", cfg)
	}
	// V14: an unusable value must not turn the vault off. It is kept as given,
	// so the probe refuses every capture.
	t.Setenv("VM_SNAPSHOT_VAULT_DIR", "relative/vault")
	if cfg := LoadConfigFromEnv(); cfg.SnapshotVaultDir != "relative/vault" {
		t.Fatalf("a relative vault path was dropped or rewritten: %q", cfg.SnapshotVaultDir)
	}
}

// V14: an unusable vault setting refuses captures; it never falls back to the
// state directory.
func TestVaultWithAnUnusableDirRefusesEveryCapture(t *testing.T) {
	f := newSnapshotFixture(t)
	f.serveLaunchAPI(t)
	f.mgr.cfg.SnapshotVaultDir = "relative/vault"
	// The real probe, not a fake: it must refuse a relative path.
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	assertNoPlaintextSnapshot(t, f)
	tl := readHibernateTimeline(t, f.mgr, f.vmID)
	if tl.Snapshot == nil || tl.Snapshot.Reason != snapshotReasonVaultUnavailable {
		t.Fatalf("timeline = %+v", tl.Snapshot)
	}
}
