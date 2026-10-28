package vmmanager

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// L3a, Diff snapshots of cold-booted guests (RES-13;
// docs/design/fp10-snapshot-levers-and-host-slab-2026-10-22.md §2 L3a,
// "Ways it could fail" 1 to 6). Each test names the failure mode it pins.

const diffTestPIDDir = "proc"

// writeSparseMem writes a memory file of length n with 4 KiB of data at
// offset 0 and a hole after it, as Firecracker's mincore Diff writes it.
func writeSparseMem(path string, n int64) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write([]byte(strings.Repeat("g", 4096)))
	if n > 4096 {
		_ = f.Truncate(n)
	}
}

// diffFixture is a snapshot fixture whose VM was cold booted, whose
// Firecracker is a real (sleeping) child process, and whose /proc and
// cgroup trees are fakes that pass every Diff condition as Node B does
// today (reading R2: leaf without a memory controller, service swap.max 0;
// R5: guest mapping THPeligible 0).
type diffFixture struct {
	*snapshotFixture
	pid    int
	proc   string
	cgroup string
}

func newDiffFixture(t *testing.T) *diffFixture {
	t.Helper()
	return armDiff(t, newSnapshotFixture(t))
}

// armDiff turns on every L3a condition for f's VM.
func armDiff(t *testing.T, f *snapshotFixture) *diffFixture {
	t.Helper()
	f.mgr.cfg.SnapshotType = SnapshotTypeConfigDiff
	f.mgr.cfg.SnapshotDiffOnly = []string{"computer-s3"}
	f.fc.diffLen = int64(f.cfg.MachineMemSizeMib) << 20

	sleeper := exec.Command("sleep", "60")
	if err := sleeper.Start(); err != nil {
		t.Fatalf("start sleeper: %v", err)
	}
	t.Cleanup(func() { _ = sleeper.Process.Kill(); _ = sleeper.Wait() })
	pid := sleeper.Process.Pid
	f.mgr.mu.Lock()
	f.inst.PID = pid
	f.inst.coldOrigin = true
	f.mgr.mu.Unlock()
	// The fake process is gone once killed; do not wait for a done signal.
	close(f.inst.done)

	root := f.mgr.cfg.StateDir
	proc := filepath.Join(root, diffTestPIDDir)
	cg := filepath.Join(root, "cgroup")
	f.mgr.procRoot = proc
	f.mgr.cgroupRoot = cg
	d := &diffFixture{snapshotFixture: f, pid: pid, proc: proc, cgroup: cg}
	d.writeProc(t, "status", "Name:\tfirecracker\nVmRSS:\t 1441532 kB\nVmSwap:\t       0 kB\n")
	d.writeProc(t, "cgroup", "0::/system.slice/go-choir-vmctl.service/supervisor\n")
	d.writeProc(t, "smaps", smapsMapping("72f838000000-72fa38000000 rw-p 00000000 00:00 0", 65536, 0)+
		smapsMapping("55d000000000-55d000100000 r-xp 00000000 00:20 1 /nix/store/x-firecracker/bin/firecracker", 1024, 0))
	d.writeCgroup(t, "system.slice/go-choir-vmctl.service/memory.swap.max", "0\n")
	if err := os.MkdirAll(filepath.Join(cg, "system.slice/go-choir-vmctl.service/supervisor"), 0o755); err != nil {
		t.Fatal(err)
	}
	return d
}

func smapsMapping(header string, sizeKB, thp int) string {
	return header + "\n" +
		"Size:           " + itoa(sizeKB) + " kB\n" +
		"Rss:            1024 kB\n" +
		"AnonHugePages:  0 kB\n" +
		"THPeligible:    " + itoa(thp) + "\n"
}

func (d *diffFixture) writeProc(t *testing.T, name, body string) {
	t.Helper()
	dir := filepath.Join(d.proc, itoa(d.pid))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (d *diffFixture) writeCgroup(t *testing.T, rel, body string) {
	t.Helper()
	path := filepath.Join(d.cgroup, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// createTypes lists the snapshot_type of every /snapshot/create call.
func (f *fakeFirecrackerAPI) createTypes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		if c.Path == "/snapshot/create" {
			typ, _ := c.Body["snapshot_type"].(string)
			out = append(out, typ)
		}
	}
	return out
}

// hibernateAndRead hibernates and returns the committed meta and timeline.
func (d *diffFixture) hibernateAndRead(t *testing.T) (*SnapshotMeta, *BootTimeline) {
	t.Helper()
	d.serveLaunchAPI(t)
	if err := d.mgr.HibernateVM(d.vmID); err != nil {
		t.Fatalf("HibernateVM: %v", err)
	}
	meta, err := readSnapshotMeta(d.mgr.snapshotDir(d.vmID))
	if err != nil {
		t.Fatalf("committed snapshot: %v", err)
	}
	return meta, readHibernateTimeline(t, d.mgr, d.vmID)
}

// The positive case: every condition holds, so the hibernate takes Diff,
// records it, and the snapshot loads.
func TestDiffSnapshotOfColdGuestRoundTrip(t *testing.T) {
	d := newDiffFixture(t)
	meta, tl := d.hibernateAndRead(t)
	if got := strings.Join(d.fc.createTypes(), ","); got != "Diff" {
		t.Fatalf("create types = %s, want Diff", got)
	}
	if meta.SnapshotType != "Diff" || !meta.ColdOrigin || meta.Version != snapshotMetaVersion {
		t.Fatalf("meta type=%q cold=%v version=%d", meta.SnapshotType, meta.ColdOrigin, meta.Version)
	}
	if tl.Snapshot.SnapshotType != "Diff" || tl.Snapshot.DiffRefused != "" {
		t.Fatalf("timeline type=%q refused=%q", tl.Snapshot.SnapshotType, tl.Snapshot.DiffRefused)
	}
	restoreAPI := &fakeFirecrackerAPI{}
	var args []string
	d.restoreStarter(t, restoreAPI, &args)
	inst, err := d.mgr.ResumeVMFromSnapshot(d.vmID, d.expect, nil)
	if err != nil {
		t.Fatalf("resume of a cold-origin Diff: %v", err)
	}
	if inst.coldOrigin {
		t.Fatal("a restored process must not be cold origin")
	}
}

// Failure mode 1: a restored guest never takes Diff, and neither does a
// reattached process, whose origin vmctl did not see.
func TestDiffSnapshotRefusedForRestoredOrReattachedGuest(t *testing.T) {
	t.Run("restored", func(t *testing.T) {
		d := newDiffFixture(t)
		d.mgr.mu.Lock()
		d.inst.coldOrigin = false
		d.mgr.mu.Unlock()
		meta, tl := d.hibernateAndRead(t)
		if got := strings.Join(d.fc.createTypes(), ","); got != "Full" {
			t.Fatalf("create types = %s, want Full", got)
		}
		if meta.SnapshotType != "Full" || meta.ColdOrigin {
			t.Fatalf("meta type=%q cold=%v", meta.SnapshotType, meta.ColdOrigin)
		}
		if tl.Snapshot.DiffRefused != diffRefusedNotColdOrigin {
			t.Fatalf("refusal = %q", tl.Snapshot.DiffRefused)
		}
	})
	t.Run("resumed then hibernated again", func(t *testing.T) {
		d := newDiffFixture(t)
		d.hibernateAndRead(t)
		restoreAPI := &fakeFirecrackerAPI{memBytes: 4096, diffLen: int64(d.cfg.MachineMemSizeMib) << 20}
		var args []string
		d.restoreStarter(t, restoreAPI, &args)
		inst, err := d.mgr.ResumeVMFromSnapshot(d.vmID, d.expect, nil)
		if err != nil {
			t.Fatalf("resume: %v", err)
		}
		if inst.coldOrigin {
			t.Fatal("ResumeVMFromSnapshot must leave coldOrigin false")
		}
		if err := d.mgr.HibernateVM(d.vmID); err != nil {
			t.Fatalf("second HibernateVM: %v", err)
		}
		if got := strings.Join(restoreAPI.createTypes(), ","); got != "Full" {
			t.Fatalf("restored guest create types = %s, want Full", got)
		}
	})
}

// Failure mode 2: guest memory that may swap never takes Diff. The cgroup
// walk takes the guarantee from the nearest ancestor reading 0 (R2), and
// refuses when none does or the tree cannot be read.
func TestDiffSnapshotRefusedWhenGuestMemoryMaySwap(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, d *diffFixture)
	}{
		{"VmSwap nonzero", func(t *testing.T, d *diffFixture) {
			d.writeProc(t, "status", "Name:\tfirecracker\nVmSwap:\t       4 kB\n")
		}},
		{"VmSwap missing", func(t *testing.T, d *diffFixture) {
			d.writeProc(t, "status", "Name:\tfirecracker\n")
		}},
		{"no ancestor sets 0", func(t *testing.T, d *diffFixture) {
			d.writeCgroup(t, "system.slice/go-choir-vmctl.service/memory.swap.max", "max\n")
		}},
		{"leaf reads max, no ancestor 0", func(t *testing.T, d *diffFixture) {
			d.writeCgroup(t, "system.slice/go-choir-vmctl.service/memory.swap.max", "max\n")
			d.writeCgroup(t, "system.slice/go-choir-vmctl.service/supervisor/memory.swap.max", "max\n")
		}},
		{"nonzero limit", func(t *testing.T, d *diffFixture) {
			d.writeCgroup(t, "system.slice/go-choir-vmctl.service/memory.swap.max", "4294967296\n")
		}},
		{"cgroup v1 line only", func(t *testing.T, d *diffFixture) {
			d.writeProc(t, "cgroup", "4:memory:/system.slice\n")
		}},
		{"cgroup file unreadable", func(t *testing.T, d *diffFixture) {
			_ = os.Remove(filepath.Join(d.proc, itoa(d.pid), "cgroup"))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newDiffFixture(t)
			tc.setup(t, d)
			meta, tl := d.hibernateAndRead(t)
			if got := strings.Join(d.fc.createTypes(), ","); got != "Full" || meta.SnapshotType != "Full" {
				t.Fatalf("create types = %s meta=%q, want Full", got, meta.SnapshotType)
			}
			if tl.Snapshot.DiffRefused != diffRefusedMaySwap {
				t.Fatalf("refusal = %q", tl.Snapshot.DiffRefused)
			}
		})
	}
	t.Run("jailer parent reads 0 under a leaf reading max", func(t *testing.T) {
		d := newDiffFixture(t)
		d.writeProc(t, "cgroup", "0::/system.slice/go-choir-vmctl.service/vms/firecracker/vm-s3\n")
		d.writeCgroup(t, "system.slice/go-choir-vmctl.service/vms/firecracker/vm-s3/memory.swap.max", "max\n")
		d.writeCgroup(t, "system.slice/go-choir-vmctl.service/vms/memory.swap.max", "max\n")
		meta, _ := d.hibernateAndRead(t)
		if meta.SnapshotType != "Diff" {
			t.Fatalf("type = %q; the service's 0 bounds the jail leaf", meta.SnapshotType)
		}
	})
}

// The other conditions: off by default and with no computer selected
// (inert), the version floor, THP on the guest mapping.
func TestDiffSnapshotConditions(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(t *testing.T, d *diffFixture)
		refusal string
	}{
		{"default full", func(t *testing.T, d *diffFixture) { d.mgr.cfg.SnapshotType = "" }, diffRefusedOff},
		{"explicit full", func(t *testing.T, d *diffFixture) { d.mgr.cfg.SnapshotType = SnapshotTypeConfigFull }, diffRefusedOff},
		{"diff with no computer selected", func(t *testing.T, d *diffFixture) { d.mgr.cfg.SnapshotDiffOnly = nil }, diffRefusedNotSelected},
		{"another computer selected", func(t *testing.T, d *diffFixture) {
			d.mgr.cfg.SnapshotDiffOnly = []string{"computer-other"}
		}, diffRefusedNotSelected},
		{"firecracker below 1.15.0", func(t *testing.T, d *diffFixture) {
			if err := os.WriteFile(d.mgr.cfg.FirecrackerBinPath, []byte("#!/bin/sh\necho 'Firecracker v1.14.2'\n"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, diffRefusedFirecracker},
		{"firecracker version unreadable", func(t *testing.T, d *diffFixture) {
			if err := os.WriteFile(d.mgr.cfg.FirecrackerBinPath, []byte("#!/bin/sh\necho 'weird'\n"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, diffRefusedFirecracker},
		{"guest mapping THP eligible", func(t *testing.T, d *diffFixture) {
			d.writeProc(t, "smaps", smapsMapping("72f838000000-72fa38000000 rw-p 00000000 00:00 0", 65536, 1))
		}, diffRefusedTHP},
		{"no guest mapping found", func(t *testing.T, d *diffFixture) {
			d.writeProc(t, "smaps", smapsMapping("55d000000000-55d000100000 r-xp 00000000 00:20 1 /bin/fc", 1024, 0))
		}, diffRefusedTHP},
		{"no pid", func(t *testing.T, d *diffFixture) {
			d.mgr.mu.Lock()
			d.inst.PID = 0
			d.mgr.mu.Unlock()
		}, diffRefusedNoPID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newDiffFixture(t)
			tc.setup(t, d)
			meta, tl := d.hibernateAndRead(t)
			if got := strings.Join(d.fc.createTypes(), ","); got != "Full" || meta.SnapshotType != "Full" {
				t.Fatalf("create types = %s meta=%q, want Full", got, meta.SnapshotType)
			}
			if tl.Snapshot.DiffRefused != tc.refusal {
				t.Fatalf("refusal = %q, want %q", tl.Snapshot.DiffRefused, tc.refusal)
			}
		})
	}
	t.Run("class:test selects a test computer", func(t *testing.T) {
		d := newDiffFixture(t)
		d.mgr.cfg.SnapshotDiffOnly = []string{"class:test"}
		d.mgr.mu.Lock()
		d.inst.Config.WarmnessClass = "test"
		d.mgr.mu.Unlock()
		if meta, _ := d.hibernateAndRead(t); meta.SnapshotType != "Diff" {
			t.Fatalf("type = %q", meta.SnapshotType)
		}
	})
}

// D5 step 3 (S6): the list reaches every non-protected class by name, and a
// protected computer takes Full whatever the list holds ("*" included) and
// whether it is named by class or by id. Both the hibernate and the load
// check use the same decision.
func TestDiffSnapshotNeverSelectsProtectedClasses(t *testing.T) {
	list := []string{"class:test", "class:primary", "class:premium_always_on"}
	for _, class := range []string{"test", "primary", "premium_always_on"} {
		t.Run("selected "+class, func(t *testing.T) {
			d := newDiffFixture(t)
			d.mgr.cfg.SnapshotDiffOnly = list
			d.mgr.mu.Lock()
			d.inst.Config.WarmnessClass = class
			d.mgr.mu.Unlock()
			if meta, _ := d.hibernateAndRead(t); meta.SnapshotType != "Diff" {
				t.Fatalf("type = %q", meta.SnapshotType)
			}
		})
	}
	for _, class := range []string{"critical_protected", "public_platform"} {
		for name, only := range map[string][]string{
			"star":      {"*"},
			"by id":     {"computer-s3"},
			"all names": append([]string{"*", "computer-s3", "class:" + class}, list...),
		} {
			t.Run(class+" "+name, func(t *testing.T) {
				d := newDiffFixture(t)
				d.mgr.cfg.SnapshotDiffOnly = only
				d.mgr.mu.Lock()
				d.inst.Config.WarmnessClass = class
				d.mgr.mu.Unlock()
				meta, tl := d.hibernateAndRead(t)
				if meta.SnapshotType != "Full" || tl.Snapshot.DiffRefused != diffRefusedProtected {
					t.Fatalf("type=%q refused=%q, want Full and %q", meta.SnapshotType, tl.Snapshot.DiffRefused, diffRefusedProtected)
				}
				// A Diff written before the list changed is not loaded for it.
				if r := d.mgr.diffSnapshotRefusalReason("", &SnapshotMeta{SnapshotType: snapshotTypeDiff, ColdOrigin: true, Config: VMConfig{VMID: "x", WarmnessClass: class}}); r != snapshotReasonDiffLeverOff {
					t.Fatalf("load refusal = %q", r)
				}
			})
		}
	}
}

// Failure mode 3: a Diff create error falls back to Full inside the same
// pause (one PATCH /vm), and no Diff file is left behind.
func TestDiffSnapshotCreateErrorFallsBackToFullInSamePause(t *testing.T) {
	d := newDiffFixture(t)
	d.fc.failDiff = true
	meta, tl := d.hibernateAndRead(t)
	if got := strings.Join(d.fc.paths(), ","); got != "GET /,PATCH /vm,PUT /snapshot/create,PUT /snapshot/create" {
		t.Fatalf("API calls = %s", got)
	}
	if got := strings.Join(d.fc.createTypes(), ","); got != "Diff,Full" {
		t.Fatalf("create types = %s", got)
	}
	if meta.SnapshotType != "Full" || !meta.ColdOrigin {
		t.Fatalf("meta type=%q cold=%v; the Full fallback still records the cold origin", meta.SnapshotType, meta.ColdOrigin)
	}
	if tl.Snapshot.DiffRefused != diffRefusedCreateFailed || tl.Snapshot.Outcome != "snapshot" {
		t.Fatalf("timeline refused=%q outcome=%q", tl.Snapshot.DiffRefused, tl.Snapshot.Outcome)
	}
	info, err := os.Stat(filepath.Join(d.mgr.snapshotDir(d.vmID), snapshotMemName))
	if err != nil || info.Size() != 4096 {
		t.Fatalf("mem file after fallback = %v %v; want the Full file", info, err)
	}
}

// Failure mode 4: a short Diff memory file is refused (the computer cold
// boots), and so is a Diff that does not claim cold origin or an unknown
// type. The length counts the virtio-mem region (R3: 8 + 4 GiB).
func TestValidateSnapshotDiffRules(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(t *testing.T, dir string, meta *SnapshotMeta)
		want   string
	}{
		{"short Diff file", func(t *testing.T, dir string, meta *SnapshotMeta) {
			writeSparseMem(filepath.Join(dir, snapshotMemName), int64(meta.MemSizeMiB)<<20-4096)
		}, snapshotReasonDiffLength},
		{"long Diff file", func(t *testing.T, dir string, meta *SnapshotMeta) {
			writeSparseMem(filepath.Join(dir, snapshotMemName), int64(meta.MemSizeMiB)<<20+4096)
		}, snapshotReasonDiffLength},
		{"Diff without cold origin", func(t *testing.T, dir string, meta *SnapshotMeta) {
			meta.ColdOrigin = false
		}, snapshotReasonDiffNotCold},
		{"unknown type", func(t *testing.T, dir string, meta *SnapshotMeta) {
			meta.SnapshotType = "Sparse"
		}, snapshotReasonSnapshotType},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newDiffFixture(t)
			meta, _ := d.hibernateAndRead(t)
			dir := d.mgr.snapshotDir(d.vmID)
			tc.mutate(t, dir, meta)
			if err := writeSnapshotMeta(dir, meta); err != nil {
				t.Fatal(err)
			}
			started := false
			d.mgr.firecrackerStarter = func(string, []string) error { started = true; return nil }
			_, err := d.mgr.ResumeVMFromSnapshot(d.vmID, d.expect, nil)
			var fb *SnapshotFallbackError
			if !errors.As(err, &fb) || fb.Reason != tc.want {
				t.Fatalf("resume = %v, want %s", err, tc.want)
			}
			if started {
				t.Fatal("a refused Diff must never start Firecracker")
			}
		})
	}
}

// Failure mode 5: no version bump, and a meta from before L3a (no
// snapshot_type) is a Full snapshot that still validates; a Diff meta
// written by this code decodes into the old struct shape unchanged.
func TestDiffSnapshotMetaIsBackwardCompatible(t *testing.T) {
	d := newDiffFixture(t)
	d.mgr.cfg.SnapshotType = SnapshotTypeConfigFull
	meta, _ := d.hibernateAndRead(t)
	dir := d.mgr.snapshotDir(d.vmID)
	raw, err := os.ReadFile(filepath.Join(dir, snapshotMetaName))
	if err != nil {
		t.Fatal(err)
	}
	var generic map[string]any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber() // inode and mtime_ns do not fit a float64
	if err := dec.Decode(&generic); err != nil {
		t.Fatal(err)
	}
	delete(generic, "snapshot_type")
	delete(generic, "cold_origin")
	old, _ := json.Marshal(generic)
	if err := os.WriteFile(filepath.Join(dir, snapshotMetaName), old, 0o600); err != nil {
		t.Fatal(err)
	}
	reread, err := readSnapshotMeta(dir)
	if err != nil || reread.Version != meta.Version {
		t.Fatalf("old meta = %+v %v", reread, err)
	}
	if got := d.mgr.diffSnapshotRefusalReason(dir, reread); got != "" {
		t.Fatalf("an old (Full) meta is refused by the Diff rules: %q", got)
	}
	restoreAPI := &fakeFirecrackerAPI{}
	var args []string
	d.restoreStarter(t, restoreAPI, &args)
	if _, err := d.mgr.ResumeVMFromSnapshot(d.vmID, d.expect, nil); err != nil {
		t.Fatalf("resume of a pre-L3a meta: %v", err)
	}
}

// Failure mode 6: a Diff file's allocation is its real size, so the budget
// counts the resident pages, not the guest's memory size.
func TestDiffSnapshotBudgetCountsAllocatedBytes(t *testing.T) {
	d := newDiffFixture(t)
	meta, _ := d.hibernateAndRead(t)
	logical := int64(d.cfg.MachineMemSizeMib) << 20
	if meta.Bytes < logical {
		t.Fatalf("logical bytes = %d, want at least %d", meta.Bytes, logical)
	}
	if meta.DiskBytes <= 0 || meta.DiskBytes > 1<<20 {
		t.Fatalf("disk bytes = %d (%s); a 4 KiB Diff must count about 4 KiB", meta.DiskBytes, meta.DiskBytesMethod)
	}
}

// The flag is read from the environment like the other *_ONLY selectors.
func TestDiffSnapshotConfigFromEnv(t *testing.T) {
	t.Setenv("VM_SNAPSHOT_TYPE", "")
	t.Setenv("VM_SNAPSHOT_DIFF_ONLY", "")
	cfg := LoadConfigFromEnv()
	if cfg.SnapshotType != SnapshotTypeConfigFull || len(cfg.SnapshotDiffOnly) != 0 {
		t.Fatalf("default = %q %v", cfg.SnapshotType, cfg.SnapshotDiffOnly)
	}
	t.Setenv("VM_SNAPSHOT_TYPE", " Diff ")
	t.Setenv("VM_SNAPSHOT_DIFF_ONLY", "computer-a, class:test")
	cfg = LoadConfigFromEnv()
	if cfg.SnapshotType != SnapshotTypeConfigDiff || strings.Join(cfg.SnapshotDiffOnly, ",") != "computer-a,class:test" {
		t.Fatalf("diff = %q %v", cfg.SnapshotType, cfg.SnapshotDiffOnly)
	}
	t.Setenv("VM_SNAPSHOT_TYPE", "sparse")
	if cfg = LoadConfigFromEnv(); cfg.SnapshotType != SnapshotTypeConfigFull {
		t.Fatalf("an unknown value must read as full, got %q", cfg.SnapshotType)
	}
}

// countVersionExecs makes the fake Firecracker binary count its --version
// runs, and returns a reader of the count.
func countVersionExecs(t *testing.T, mgr *Manager) func() int {
	t.Helper()
	counter := filepath.Join(mgr.cfg.StateDir, "version-execs")
	script := "#!/bin/sh\necho x >> '" + counter + "'\necho '" + testFCVersion + "'\n"
	if err := os.WriteFile(mgr.cfg.FirecrackerBinPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return func() int {
		raw, _ := os.ReadFile(counter)
		return strings.Count(string(raw), "x")
	}
}

// Round 2, F2: no hibernate runs `firecracker --version` while the guest is
// paused: never in full mode (the paper's case), and in diff mode the
// decision is taken before the pause.
func TestHibernateNeverExecsVersionInsideThePause(t *testing.T) {
	for _, mode := range []string{SnapshotTypeConfigFull, SnapshotTypeConfigDiff} {
		t.Run(mode, func(t *testing.T) {
			d := newDiffFixture(t)
			d.mgr.cfg.SnapshotType = mode
			execs := countVersionExecs(t, d.mgr)
			atPause, atCreate := -1, -1
			d.fc.onCall = func(method, path string) {
				switch method + " " + path {
				case "PATCH /vm":
					atPause = execs()
				case "PUT /snapshot/create":
					if atCreate < 0 {
						atCreate = execs()
					}
				}
			}
			meta, _ := d.hibernateAndRead(t)
			if atPause < 0 || atCreate < 0 {
				t.Fatalf("pause or create not seen: %d %d", atPause, atCreate)
			}
			if atCreate != atPause {
				t.Fatalf("%d version exec(s) ran inside the pause", atCreate-atPause)
			}
			want := map[string]string{SnapshotTypeConfigFull: "Full", SnapshotTypeConfigDiff: "Diff"}[mode]
			if meta.SnapshotType != want {
				t.Fatalf("type = %q, want %q", meta.SnapshotType, want)
			}
		})
	}
	t.Run("full mode never execs for the Diff decision", func(t *testing.T) {
		d := newDiffFixture(t)
		d.mgr.cfg.SnapshotType = SnapshotTypeConfigFull
		execs := countVersionExecs(t, d.mgr)
		d.hibernateAndRead(t)
		// One exec only: the meta's firecracker_version, after the stop.
		if n := execs(); n != 1 {
			t.Fatalf("version execs = %d, want 1 (the meta's)", n)
		}
	})
}

// Round 2, F1: Firecracker set_len()s a Diff file, so a length mismatch is
// checked after the create and falls back to Full in the same pause.
func TestDiffSnapshotLengthCheckedAtCreate(t *testing.T) {
	t.Run("length mismatch after create", func(t *testing.T) {
		d := newDiffFixture(t)
		d.fc.diffLen = int64(d.cfg.MachineMemSizeMib) << 19
		meta, tl := d.hibernateAndRead(t)
		if got := strings.Join(d.fc.createTypes(), ","); got != "Diff,Full" {
			t.Fatalf("create types = %s, want Diff,Full", got)
		}
		if got := strings.Count(strings.Join(d.fc.paths(), ","), "PATCH /vm"); got != 1 {
			t.Fatalf("pauses = %d, want 1", got)
		}
		if meta.SnapshotType != "Full" || tl.Snapshot.DiffRefused != diffRefusedLengthAtCreate {
			t.Fatalf("meta type=%q refused=%q", meta.SnapshotType, tl.Snapshot.DiffRefused)
		}
	})
}

// Round 2, F4: a Diff snapshot loads only while the lever still selects its
// computer. Turning the lever off (the rollback) cold boots a pending Diff.
func TestDiffSnapshotRefusedAtLoadWhenLeverOff(t *testing.T) {
	cases := map[string]func(d *diffFixture){
		"type full":      func(d *diffFixture) { d.mgr.cfg.SnapshotType = SnapshotTypeConfigFull },
		"list emptied":   func(d *diffFixture) { d.mgr.cfg.SnapshotDiffOnly = nil },
		"other computer": func(d *diffFixture) { d.mgr.cfg.SnapshotDiffOnly = []string{"computer-other"} },
	}
	for name, off := range cases {
		t.Run(name, func(t *testing.T) {
			d := newDiffFixture(t)
			if meta, _ := d.hibernateAndRead(t); meta.SnapshotType != "Diff" {
				t.Fatalf("type = %q", meta.SnapshotType)
			}
			off(d)
			started := false
			d.mgr.firecrackerStarter = func(string, []string) error { started = true; return nil }
			_, err := d.mgr.ResumeVMFromSnapshot(d.vmID, d.expect, nil)
			var fb *SnapshotFallbackError
			if !errors.As(err, &fb) || fb.Reason != snapshotReasonDiffLeverOff {
				t.Fatalf("resume = %v, want %s", err, snapshotReasonDiffLeverOff)
			}
			if started {
				t.Fatal("a Diff with the lever off must never start Firecracker")
			}
		})
	}
}

// Round 2, F5: a jailed Diff create error removes what Firecracker wrote in
// the jail before the Full create.
func TestJailedDiffCreateErrorClearsJailPartial(t *testing.T) {
	jf := newJailedSnapshotFixture(t)
	d := armDiff(t, jf.snapshotFixture)
	d.fc.failDiff = true
	jailMem := filepath.Join(d.mgr.jailRoot(d.vmID), snapshotPartialDirName, snapshotMemName)
	creates, leftover := 0, false
	d.fc.onCall = func(method, path string) {
		if path != "/snapshot/create" {
			return
		}
		creates++
		if creates == 2 {
			if _, err := os.Stat(jailMem); err == nil {
				leftover = true
			}
		}
	}
	if err := d.mgr.HibernateVM(d.vmID); err != nil {
		t.Fatalf("HibernateVM: %v", err)
	}
	if got := strings.Join(d.fc.createTypes(), ","); got != "Diff,Full" {
		t.Fatalf("create types = %s", got)
	}
	if leftover {
		t.Fatal("the failed Diff's memory file was still in the jail at the Full create")
	}
	meta, err := readSnapshotMeta(d.mgr.snapshotDir(d.vmID))
	if err != nil || meta.SnapshotType != "Full" || meta.Confinement != ConfinementJailer {
		t.Fatalf("meta = %+v, %v", meta, err)
	}
}

// Round 2, F6: the wake's timeline names the image it loaded.
func TestDiffResumeTimelineRecordsImageType(t *testing.T) {
	d := newDiffFixture(t)
	d.hibernateAndRead(t)
	restoreAPI := &fakeFirecrackerAPI{}
	var args []string
	d.restoreStarter(t, restoreAPI, &args)
	if _, err := d.mgr.ResumeVMFromSnapshot(d.vmID, d.expect, nil); err != nil {
		t.Fatalf("resume: %v", err)
	}
	tl, err := d.mgr.BootTimelineForVM(d.vmID)
	if err != nil {
		t.Fatal(err)
	}
	if tl.Snapshot == nil || tl.Snapshot.SnapshotType != "Diff" || !tl.Snapshot.ColdOrigin {
		t.Fatalf("resume timeline snapshot = %+v", tl.Snapshot)
	}
}

// Round 2 follow-up, N2: a jailed Diff succeeds. The create-time length
// check must read the jail's capture dir, where the jailed VMM wrote the
// file; reading the host partial dir would turn every jailed Diff into a
// silent Full. Also the jailed wrong-length path (DeepSeek round 2, low 3).
func TestJailedDiffSnapshot(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		jf := newJailedSnapshotFixture(t)
		d := armDiff(t, jf.snapshotFixture)
		if err := d.mgr.HibernateVM(d.vmID); err != nil {
			t.Fatalf("HibernateVM: %v", err)
		}
		tl := readHibernateTimeline(t, d.mgr, d.vmID)
		if got := strings.Join(d.fc.createTypes(), ","); got != "Diff" {
			t.Fatalf("create types = %s refused=%q, want Diff", got, tl.Snapshot.DiffRefused)
		}
		meta, err := readSnapshotMeta(d.mgr.snapshotDir(d.vmID))
		if err != nil || meta.SnapshotType != "Diff" || !meta.ColdOrigin || meta.Confinement != ConfinementJailer {
			t.Fatalf("meta = %+v, %v", meta, err)
		}
		info, err := os.Stat(filepath.Join(d.mgr.snapshotDir(d.vmID), snapshotMemName))
		if err != nil || info.Size() != int64(d.cfg.MachineMemSizeMib)<<20 {
			t.Fatalf("committed Diff mem = %v, %v", info, err)
		}
	})
	t.Run("wrong length", func(t *testing.T) {
		jf := newJailedSnapshotFixture(t)
		d := armDiff(t, jf.snapshotFixture)
		d.fc.diffLen = int64(d.cfg.MachineMemSizeMib) << 19
		if err := d.mgr.HibernateVM(d.vmID); err != nil {
			t.Fatalf("HibernateVM: %v", err)
		}
		if got := strings.Join(d.fc.createTypes(), ","); got != "Diff,Full" {
			t.Fatalf("create types = %s, want Diff,Full", got)
		}
		tl := readHibernateTimeline(t, d.mgr, d.vmID)
		meta, err := readSnapshotMeta(d.mgr.snapshotDir(d.vmID))
		if err != nil || meta.SnapshotType != "Full" || tl.Snapshot.DiffRefused != diffRefusedLengthAtCreate {
			t.Fatalf("meta = %+v refused=%q, %v", meta, tl.Snapshot.DiffRefused, err)
		}
	})
}
