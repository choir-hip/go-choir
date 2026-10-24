package vmmanager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VM slice tests. The failure list is "Design: the VM slice" in
// docs/problems/vmctl-restart-fails-while-a-jailed-vm-runs-2026-10-24.md;
// each test names the items it pins. A fake cgroup2 root (a temp dir) stands
// in for /sys/fs/cgroup; the reproduction against systemd and the kernel is
// in that record.

const (
	sliceOwn    = "system.slice/go-choir-vmctl.service/supervisor"
	sliceUnit   = "system.slice/go-choir-vmctl.service"
	sliceCgroup = "system.slice/system-choirvms.slice"
	sliceParent = sliceCgroup + "/vms"
)

// sliceHost lays out the Node B shape: vmctl in <unit>/supervisor holding
// pid 1234, a pid 999 left in the unit root (the old layout moved it), the
// unit root's subtree_control empty, and the slice's cgroup present and
// empty, as systemd creates it.
func sliceHost(t *testing.T) (*Manager, string) {
	t.Helper()
	mgr, _ := jailedManager(t, filepath.Join(t.TempDir(), "state"))
	root := filepath.Join(t.TempDir(), "cgroup")
	mgr.cgroupRoot = root
	mgr.cfg.Jailer.CgroupParent = sliceParent
	writeCgroupFile(t, root, sliceOwn+"/cgroup.procs", "1234\n")
	writeCgroupFile(t, root, sliceUnit+"/cgroup.procs", "999\n")
	writeCgroupFile(t, root, sliceUnit+"/cgroup.subtree_control", "")
	writeCgroupFile(t, root, sliceCgroup+"/cgroup.procs", "")
	writeCgroupFile(t, root, sliceCgroup+"/cgroup.subtree_control", "")
	return mgr, root
}

func writeCgroupFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// treeSnapshot is every file under dir with its content, so a test can
// prove prepareJailerCgroups wrote nothing there.
func treeSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			out[path+"/"] = ""
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[path] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func assertTreeUnchanged(t *testing.T, what string, before, after map[string]string) {
	t.Helper()
	for k, v := range before {
		if got, ok := after[k]; !ok || got != v {
			t.Fatalf("%s changed: %s was %q, now %q (present %v)", what, k, v, got, ok)
		}
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			t.Fatalf("%s gained %s", what, k)
		}
	}
}

// Failure item 1: with the parent in the VM slice, vmctl's unit is never
// touched: no process moved out of the unit root or out of supervisor, no
// subtree_control written, nothing created under the unit. Only vms/ is
// made, inside the slice systemd created.
func TestPrepareJailerCgroupsInTheVMSlice(t *testing.T) {
	mgr, root := sliceHost(t)
	unit := filepath.Join(root, sliceUnit)
	before := treeSnapshot(t, unit)
	sliceBefore := treeSnapshot(t, filepath.Join(root, sliceCgroup))

	if err := mgr.prepareJailerCgroups(sliceOwn); err != nil {
		t.Fatalf("prepareJailerCgroups: %v", err)
	}
	assertTreeUnchanged(t, "vmctl's unit cgroup", before, treeSnapshot(t, unit))
	if info, err := os.Stat(filepath.Join(root, sliceParent)); err != nil || !info.IsDir() {
		t.Fatalf("vms/ not created in the slice: %v", err)
	}
	for k, v := range sliceBefore {
		if !strings.HasSuffix(k, "/") {
			if got, _ := os.ReadFile(k); string(got) != v {
				t.Fatalf("slice file %s rewritten: %q -> %q", k, v, got)
			}
		}
	}
	// A second start (vms/ already there, as after a vmctl restart).
	if err := mgr.prepareJailerCgroups(sliceOwn); err != nil {
		t.Fatalf("prepareJailerCgroups with vms/ present: %v", err)
	}
	assertTreeUnchanged(t, "vmctl's unit cgroup after a second start", before, treeSnapshot(t, unit))
}

// Failure item 5: a parent inside vmctl's own unit (the old layout, a typo,
// an env-only rollback) is refused, naming the record, and nothing moves.
func TestPrepareJailerCgroupsRefusesAParentInsideVmctlsUnit(t *testing.T) {
	for _, parent := range []string{
		sliceUnit + "/vms",
		sliceUnit,
		sliceUnit + "/deeper/vms",
		"/" + sliceUnit + "/vms/",
	} {
		t.Run(parent, func(t *testing.T) {
			mgr, root := sliceHost(t)
			mgr.cfg.Jailer.CgroupParent = parent
			unit := filepath.Join(root, sliceUnit)
			before := treeSnapshot(t, unit)
			err := mgr.prepareJailerCgroups(sliceOwn)
			if err == nil || !strings.Contains(err.Error(), "vmctl-restart-fails-while-a-jailed-vm-runs") {
				t.Fatalf("prepareJailerCgroups(%q) = %v, want the in-unit parent refused", parent, err)
			}
			assertTreeUnchanged(t, "vmctl's unit cgroup", before, treeSnapshot(t, unit))
		})
	}
	// vmctl outside any delegated subgroup: its unit is still its unit.
	mgr, _ := sliceHost(t)
	mgr.cfg.Jailer.CgroupParent = sliceUnit + "/vms"
	if err := mgr.prepareJailerCgroups(sliceUnit); err == nil {
		t.Fatal("a parent inside the unit must be refused when vmctl runs in the unit root too")
	}
}

// Failure item 6: no derivation. An unset parent is refused with the
// variable named, before any cgroup is touched.
func TestPrepareJailerCgroupsRefusesAnUnsetParent(t *testing.T) {
	mgr, root := sliceHost(t)
	mgr.cfg.Jailer.CgroupParent = ""
	before := treeSnapshot(t, root)
	err := mgr.prepareJailerCgroups(sliceOwn)
	if err == nil || !strings.Contains(err.Error(), "VMCTL_JAILER_CGROUP_PARENT") {
		t.Fatalf("prepareJailerCgroups with no parent = %v", err)
	}
	assertTreeUnchanged(t, "cgroup root", before, treeSnapshot(t, root))
}

// Failure item 3: the slice not active (its cgroup missing). vmctl refuses
// and does not create the systemd-owned cgroup itself.
func TestPrepareJailerCgroupsRequiresTheSlice(t *testing.T) {
	mgr, root := sliceHost(t)
	if err := os.RemoveAll(filepath.Join(root, sliceCgroup)); err != nil {
		t.Fatal(err)
	}
	err := mgr.prepareJailerCgroups(sliceOwn)
	if err == nil || !strings.Contains(err.Error(), "system-choirvms.slice") {
		t.Fatalf("prepareJailerCgroups without the slice = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, sliceCgroup)); !os.IsNotExist(statErr) {
		t.Fatalf("vmctl created the slice's cgroup: %v", statErr)
	}
}

// Failure item 4: a process in the slice's cgroup or in vms/ makes every
// jailer subtree_control write fail EBUSY; vmctl refuses up front and never
// moves a process it does not own.
func TestPrepareJailerCgroupsRefusesAPopulatedSlice(t *testing.T) {
	for _, rel := range []string{sliceCgroup + "/cgroup.procs", sliceParent + "/cgroup.procs"} {
		t.Run(rel, func(t *testing.T) {
			mgr, root := sliceHost(t)
			writeCgroupFile(t, root, rel, "4242\n")
			before := treeSnapshot(t, root)
			err := mgr.prepareJailerCgroups(sliceOwn)
			if err == nil || !strings.Contains(err.Error(), "4242") {
				t.Fatalf("prepareJailerCgroups with pid 4242 in %s = %v", rel, err)
			}
			assertTreeUnchanged(t, "cgroup root", before, treeSnapshot(t, root))
		})
	}
}

// Failure item 7: leaf cleanup removes the leaf the launch receipt recorded
// (a VM launched under an earlier parent) and the leaf under the current
// parent; a receipt path that is not a clean relative leaf of this VM is
// ignored.
func TestRemoveCgroupLeafUsesTheLaunchReceipt(t *testing.T) {
	mgr, root := sliceHost(t)
	vmID := "vm-a"
	if err := os.MkdirAll(mgr.vmStateDir(vmID), 0o755); err != nil {
		t.Fatal(err)
	}
	oldLeaf := sliceUnit + "/vms/" + vmID
	curLeaf := sliceParent + "/" + vmID
	for _, rel := range []string{oldLeaf, curLeaf} {
		if err := os.MkdirAll(filepath.Join(root, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mgr.persistConfinement(vmID, &ConfinementReceipt{Mode: ConfinementJailer, Cgroup: oldLeaf})
	mgr.removeCgroupLeaf(vmID)
	for _, rel := range []string{oldLeaf, curLeaf} {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Fatalf("leaf %s not removed: %v", rel, err)
		}
	}

	// Receipt paths that are not this VM's leaf, or not clean and relative:
	// each leaves the empty cgroup it would name in place.
	for _, bad := range []struct{ receipt, keep string }{
		{sliceUnit + "/vms/vm-other", sliceUnit + "/vms/vm-other"},
		{"/" + sliceUnit + "/vms/" + vmID, sliceUnit + "/vms/" + vmID},
		{sliceUnit + "/x/../vms/" + vmID, sliceUnit + "/vms/" + vmID},
		{"../" + vmID, "../" + vmID},
		{vmID, vmID},
	} {
		t.Run(bad.receipt, func(t *testing.T) {
			keep := filepath.Join(root, filepath.FromSlash(bad.keep))
			if err := os.MkdirAll(keep, 0o755); err != nil {
				t.Fatal(err)
			}
			mgr.persistConfinement(vmID, &ConfinementReceipt{Mode: ConfinementJailer, Cgroup: bad.receipt})
			mgr.removeCgroupLeaf(vmID)
			if _, err := os.Stat(keep); err != nil {
				t.Fatalf("receipt path %q removed %s: %v", bad.receipt, keep, err)
			}
		})
	}

	// A direct receipt names no cgroup; the current leaf is still removed.
	if err := os.MkdirAll(filepath.Join(root, curLeaf), 0o755); err != nil {
		t.Fatal(err)
	}
	mgr.persistConfinement(vmID, &ConfinementReceipt{Mode: ConfinementDirect})
	mgr.removeCgroupLeaf(vmID)
	if _, err := os.Stat(filepath.Join(root, curLeaf)); !os.IsNotExist(err) {
		t.Fatalf("current leaf not removed with a direct receipt: %v", err)
	}
}

// Failure item 9: a jailed VM in the slice reads the slice's
// memory.swap.max 0, so its diff snapshots are not refused for swap; the
// vmctl unit's value no longer bounds it.
func TestJailedVMInTheSliceCannotSwap(t *testing.T) {
	dir := t.TempDir()
	proc := filepath.Join(dir, "proc")
	cg := filepath.Join(dir, "cgroup")
	writeCgroupFile(t, proc, "4242/status", "Name:\tfirecracker\nVmSwap:\t       0 kB\n")
	writeCgroupFile(t, proc, "4242/cgroup", "0::/"+sliceParent+"/vm-a\n")
	writeCgroupFile(t, cg, sliceParent+"/vm-a/memory.swap.max", "max\n")
	writeCgroupFile(t, cg, sliceParent+"/memory.swap.max", "max\n")
	writeCgroupFile(t, cg, sliceUnit+"/memory.swap.max", "0\n")
	writeCgroupFile(t, cg, "system.slice/memory.swap.max", "max\n")
	if guestMemoryCannotSwap(proc, cg, 4242) {
		t.Fatal("without the slice's MemorySwapMax=0 the VM may swap")
	}
	writeCgroupFile(t, cg, sliceCgroup+"/memory.swap.max", "0\n")
	if !guestMemoryCannotSwap(proc, cg, 4242) {
		t.Fatal("the slice's memory.swap.max 0 bounds the jailed leaf")
	}
}

// Review (bunch 179, Devin and DeepSeek): an own cgroup that cannot be
// determined (no "0::" line, or the cgroup root) skips the in-unit check.
// Fail closed: vmctl cannot prove the parent lies outside its unit, so the
// jailer is unavailable and launches go direct.
func TestPrepareJailerCgroupsRefusesAnUnknownOwnCgroup(t *testing.T) {
	for name, own := range map[string]string{
		"empty":       "",
		"slash":       "/",
		"no_unified":  parseCgroupV2Path("1:name=systemd:/x\n"),
		"cgroup_root": parseCgroupV2Path("0::/\n"),
		"blank":       parseCgroupV2Path("  \n"),
	} {
		t.Run(name, func(t *testing.T) {
			mgr, root := sliceHost(t)
			// The old layout, so only the unknown-unit refusal can stop it.
			mgr.cfg.Jailer.CgroupParent = sliceUnit + "/vms"
			before := treeSnapshot(t, root)
			err := mgr.prepareJailerCgroups(own)
			if err == nil || !strings.Contains(err.Error(), "vmctl-restart-fails-while-a-jailed-vm-runs") {
				t.Fatalf("prepareJailerCgroups(own=%q) = %v, want a refusal naming the record", own, err)
			}
			assertTreeUnchanged(t, "cgroup root", before, treeSnapshot(t, root))
			// Even a parent in the slice is refused: the unit is unknown.
			mgr.cfg.Jailer.CgroupParent = sliceParent
			if err := mgr.prepareJailerCgroups(own); err == nil {
				t.Fatalf("prepareJailerCgroups(own=%q) admitted a parent with the unit unknown", own)
			}
			assertTreeUnchanged(t, "cgroup root", before, treeSnapshot(t, root))
		})
	}
}

// Review (Devin 2): the current leaf path gets the same clean-relative guard
// as the receipt path before any Join or Remove, even with the jailer off.
func TestRemoveCgroupLeafIgnoresAParentThatLeavesTheRoot(t *testing.T) {
	for _, parent := range []string{"../outside", "a/../../outside", "..", "../../outside/vms"} {
		t.Run(parent, func(t *testing.T) {
			mgr, root := sliceHost(t)
			mgr.cfg.Jailer.CgroupParent = parent
			outside := filepath.Join(filepath.Dir(root), "outside")
			victims := []string{filepath.Join(outside, "vm-a"), filepath.Join(outside, "vms", "vm-a"), filepath.Join(filepath.Dir(root), "vm-a")}
			for _, v := range victims {
				if err := os.MkdirAll(v, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if rel := mgr.cgroupLeafRel("vm-a"); rel != "" {
				t.Fatalf("cgroupLeafRel with parent %q = %q, want no leaf", parent, rel)
			}
			mgr.removeCgroupLeaf("vm-a")
			for _, v := range victims {
				if _, err := os.Stat(v); err != nil {
					t.Fatalf("parent %q removed %s: %v", parent, v, err)
				}
			}
		})
	}
	// A clean parent, with slashes around it, still names its leaf.
	mgr, _ := sliceHost(t)
	mgr.cfg.Jailer.CgroupParent = "/" + sliceParent + "/"
	if rel := mgr.cgroupLeafRel("vm-a"); rel != sliceParent+"/vm-a" {
		t.Fatalf("cgroupLeafRel = %q", rel)
	}
}

// Review (DeepSeek 4): the proved staging case. A vmctl restart with jailed
// VMs alive finds vms/ holding live leaves with processes; Start must
// succeed and touch nothing. Only a process directly in the slice's or vms/'s
// own cgroup.procs refuses (a leaf's process does not block the jailer's
// subtree_control write, which is on the ancestors only).
func TestPrepareJailerCgroupsAdmitsAPopulatedLeaf(t *testing.T) {
	mgr, root := sliceHost(t)
	writeCgroupFile(t, root, sliceParent+"/cgroup.procs", "")
	writeCgroupFile(t, root, sliceParent+"/cgroup.subtree_control", "cpu pids\n")
	writeCgroupFile(t, root, sliceParent+"/vm-a/cgroup.procs", "4242\n")
	writeCgroupFile(t, root, sliceParent+"/vm-b/cgroup.procs", "4343\n4344\n")
	before := treeSnapshot(t, root)
	if err := mgr.prepareJailerCgroups(sliceOwn); err != nil {
		t.Fatalf("a vmctl restart with live jailed leaves must start clean: %v", err)
	}
	assertTreeUnchanged(t, "cgroup root", before, treeSnapshot(t, root))
}

// Review (DeepSeek 2): systemd owns the slice's cgroup. After a manual
// `systemctl stop system-choirvms.slice` the jailer's create_dir_all would
// recreate it, so each jailed launch checks the slice and falls back direct
// when it is gone.
func TestDecideConfinementFallsBackDirectWhenTheVMSliceIsGone(t *testing.T) {
	mgr, root := sliceHost(t)
	cfg := VMConfig{VMID: "vm-a"}
	if mode, reason := mgr.decideConfinement(cfg); mode != ConfinementJailer || reason != "" {
		t.Fatalf("with the slice present = %s %q, want the jailer", mode, reason)
	}
	if err := os.RemoveAll(filepath.Join(root, sliceCgroup)); err != nil {
		t.Fatal(err)
	}
	mode, reason := mgr.decideConfinement(cfg)
	if mode != ConfinementDirect || !strings.HasPrefix(reason, confinementReasonVMSliceMissing) {
		t.Fatalf("with the slice gone = %s %q, want direct %s", mode, reason, confinementReasonVMSliceMissing)
	}
	if _, err := os.Stat(filepath.Join(root, sliceCgroup)); !os.IsNotExist(err) {
		t.Fatalf("the decision created the systemd-owned slice cgroup: %v", err)
	}
	// An invalid parent has no leaf and no slice to check: direct as well.
	mgr.cfg.Jailer.CgroupParent = "../x"
	if mode, _ := mgr.decideConfinement(cfg); mode != ConfinementDirect {
		t.Fatalf("an invalid parent must not launch jailed, got %s", mode)
	}
}
