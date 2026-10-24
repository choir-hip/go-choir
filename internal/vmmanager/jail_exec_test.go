package vmmanager

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The jailed exec file must run inside the chroot, which holds no loader
// and no libraries (docs/problems/jailed-firecracker-cannot-exec-dynamic-
// binary-2026-10-23.md). Failure modes guarded:
//  1. a dynamically linked exec file (PT_INTERP) passing the startup check,
//     so vmctl logs "jailer ready" and the first jailed launch fails with
//     "Failed to exec into Firecracker: No such file or directory";
//  2. an exec file that is not ELF at all (a wrapper script) passing it;
//  3. the jailed exec file reporting another version than the direct one, so
//     a snapshot's recorded version (read from the direct binary) lies;
//  4. a jailed exec file named differently from the direct one, so the jail
//     dir vmctl computes is not the one the jailer makes;
//  5. --exec-file still naming the direct (dynamic) binary;
//  6. a computer on the next build jailed with a next binary that is not
//     static (it falls back to a direct launch, with a reason).

// writeELFFixture writes a minimal ELF64 executable: a PT_LOAD, and a
// PT_INTERP naming interp when interp is non-empty. Its machine is SPARC V9,
// so no test host's kernel runs it (exec fails with ENOEXEC).
func writeELFFixture(t *testing.T, path, interp string) string {
	t.Helper()
	nph := 1
	if interp != "" {
		nph = 2
	}
	var buf bytes.Buffer
	var ident [elf.EI_NIDENT]byte
	copy(ident[:], elf.ELFMAG)
	ident[elf.EI_CLASS] = byte(elf.ELFCLASS64)
	ident[elf.EI_DATA] = byte(elf.ELFDATA2LSB)
	ident[elf.EI_VERSION] = byte(elf.EV_CURRENT)
	dataOff := uint64(64 + 56*nph)
	write := func(v interface{}) {
		if err := binary.Write(&buf, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	write(elf.Header64{
		Ident: ident, Type: uint16(elf.ET_EXEC), Machine: uint16(elf.EM_SPARCV9),
		Version: uint32(elf.EV_CURRENT), Entry: 0x400000, Phoff: 64,
		Ehsize: 64, Phentsize: 56, Phnum: uint16(nph),
	})
	s := interp + "\x00"
	size := dataOff
	if interp != "" {
		size += uint64(len(s))
	}
	write(elf.Prog64{Type: uint32(elf.PT_LOAD), Flags: uint32(elf.PF_R | elf.PF_X),
		Vaddr: 0x400000, Paddr: 0x400000, Filesz: size, Memsz: size, Align: 0x1000})
	if interp != "" {
		write(elf.Prog64{Type: uint32(elf.PT_INTERP), Flags: uint32(elf.PF_R), Off: dataOff,
			Vaddr: 0x400000 + dataOff, Paddr: 0x400000 + dataOff, Filesz: uint64(len(s)), Memsz: uint64(len(s)), Align: 1})
		buf.WriteString(s)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

const nixGlibcInterp = "/nix/store/h4wfwic1-glibc-2.44-25/lib/ld-linux-x86-64.so.2"

func TestStaticELFCheck(t *testing.T) {
	dir := t.TempDir()
	static := writeELFFixture(t, filepath.Join(dir, "static", "firecracker"), "")
	dynamic := writeELFFixture(t, filepath.Join(dir, "dynamic", "firecracker"), nixGlibcInterp)
	script := filepath.Join(dir, "script", "firecracker")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho 'Firecracker v1.17.0'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := staticELFError(static); err != nil {
		t.Fatalf("a static ELF must pass: %v", err)
	}
	// Failure mode 1.
	err := staticELFError(dynamic)
	if err == nil || !strings.Contains(err.Error(), "dynamically linked") || !strings.Contains(err.Error(), nixGlibcInterp) {
		t.Fatalf("a PT_INTERP exec file must be refused, naming its interpreter: %v", err)
	}
	// Failure mode 2.
	if err := staticELFError(script); err == nil || !strings.Contains(err.Error(), "not an ELF") {
		t.Fatalf("a script must be refused as not ELF: %v", err)
	}
	if err := staticELFError(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("a missing exec file must be refused")
	}
}

func TestJailedExecFileCheck(t *testing.T) {
	dir := t.TempDir()
	direct := filepath.Join(dir, "glibc", "bin", "firecracker")
	static := writeELFFixture(t, filepath.Join(dir, "musl", "bin", "firecracker"), "")
	dynamic := writeELFFixture(t, filepath.Join(dir, "dyn", "bin", "firecracker"), nixGlibcInterp)
	renamed := writeELFFixture(t, filepath.Join(dir, "odd", "bin", "fc"), "")
	versions := map[string]string{direct: "Firecracker v1.17.0", static: "Firecracker v1.17.0", renamed: "Firecracker v1.17.0"}
	version := func(bin string) string { return versions[bin] }

	if err := jailedExecError(direct, static, version); err != nil {
		t.Fatalf("a static exec file of the same version and name must pass: %v", err)
	}
	if err := jailedExecError(direct, dynamic, version); err == nil || !strings.Contains(err.Error(), "dynamically linked") {
		t.Fatalf("failure mode 1: %v", err)
	}
	// Failure mode 4.
	if err := jailedExecError(direct, renamed, version); err == nil || !strings.Contains(err.Error(), "named") {
		t.Fatalf("a differently named exec file must be refused: %v", err)
	}
	// Failure mode 3.
	versions[static] = "Firecracker v1.17.1"
	if err := jailedExecError(direct, static, version); err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("a version mismatch must be refused: %v", err)
	}
	versions[static] = ""
	if err := jailedExecError(direct, static, version); err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("an exec file that reports no version must be refused: %v", err)
	}
}

// Failure mode 1 at Start: prepareJailerHost refuses a dynamic exec file
// before it touches the jail dirs or the cgroups, so vmctl logs JAILER
// UNAVAILABLE and every launch goes direct.
func TestPrepareJailerHostRefusesDynamicExecFile(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := jailedManager(t, filepath.Join(dir, "state"))
	direct := fakeFirecrackerAt(t, filepath.Join(dir, "glibc", "bin"), "Firecracker v1.17.0")
	mgr.cfg.FirecrackerBinPath = direct
	mgr.cfg.Jailer.BinPath = filepath.Join(filepath.Dir(direct), "jailer")
	mgr.cfg.Jailer.FirecrackerBinPath = writeELFFixture(t, filepath.Join(dir, "dyn", "bin", "firecracker"), nixGlibcInterp)

	err := mgr.prepareJailerHost()
	if err == nil || !strings.Contains(err.Error(), "dynamically linked") {
		t.Fatalf("prepareJailerHost = %v, want the dynamic exec file refused", err)
	}
	if _, statErr := os.Stat(mgr.jailChrootBase()); !os.IsNotExist(statErr) {
		t.Fatalf("the refusal must come before the jail dirs are made: %v", statErr)
	}

	// The default exec file is the direct binary (today's behavior): the
	// fake here is a script, so it is refused as not ELF.
	mgr.cfg.Jailer.FirecrackerBinPath = ""
	if err := mgr.prepareJailerHost(); err == nil || !strings.Contains(err.Error(), "not an ELF") {
		t.Fatalf("prepareJailerHost with the default exec file = %v", err)
	}
}

// Failure mode 5.
func TestJailerArgsUseTheJailedExecFile(t *testing.T) {
	mgr, _ := jailedManager(t, "/var/lib/go-choir/vm-state")
	mgr.cfg.Jailer.FirecrackerBinPath = "/nix/store/def-firecracker-static-x86_64-unknown-linux-musl-1.17.0/bin/firecracker"
	got := strings.Join(mgr.jailerArgs("vm-a", 900123, jailFirecrackerLaunchArgs(true)), " ")
	if !strings.Contains(got, "--exec-file /nix/store/def-firecracker-static-x86_64-unknown-linux-musl-1.17.0/bin/firecracker ") {
		t.Fatalf("jailer args = %s, want the static exec file", got)
	}
	// Direct launches keep the direct binary.
	if b := mgr.vmmFor(VMConfig{VMID: "vm-a"}); b.Firecracker != mgr.cfg.FirecrackerBinPath {
		t.Fatalf("direct build = %+v", b)
	}
}

// Failure mode 6: the next build jails only with a static next binary.
func TestNextBuildJailsOnlyWhenStatic(t *testing.T) {
	jailedNext := func(jailExec string) *Manager {
		mgr := nextTestManager(t, func(c *ManagerConfig) {
			c.Jailer.Enabled = true
			c.Jailer.FirecrackerBinPath = "/nix/store/s-firecracker-static-1.16.2/bin/firecracker"
			c.FirecrackerNext.JailExec = jailExec
		})
		withVMSlice(t, mgr)
		mgr.jailerHostErr = nil
		return mgr
	}
	test := balloonTestVMConfig()
	test.WarmnessClass = "test"
	primary := balloonTestVMConfig()
	primary.VMID = "vm-primary"
	primary.WarmnessClass = "primary"

	mgr := jailedNext("")
	if mode, reason := mgr.decideConfinement(test); mode != ConfinementDirect || reason != confinementReasonNextNotStatic {
		t.Fatalf("next build without a static binary = %s %s, want direct %s", mode, reason, confinementReasonNextNotStatic)
	}
	if mode, _ := mgr.decideConfinement(primary); mode != ConfinementJailer {
		t.Fatalf("a pinned computer must still jail: %s", mode)
	}
	mgr.recordLaunchedVMM(primary.VMID, mgr.vmmFor(primary))
	if got := strings.Join(mgr.jailerArgs(primary.VMID, 900001, nil), " "); !strings.Contains(got, "--exec-file "+mgr.cfg.Jailer.FirecrackerBinPath+" ") {
		t.Fatalf("pinned jailer args = %s", got)
	}

	mgr = jailedNext(mgr.cfg.FirecrackerNext.BinPath)
	if mode, reason := mgr.decideConfinement(test); mode != ConfinementJailer {
		t.Fatalf("next build with a static binary = %s %s, want jailed", mode, reason)
	}
	mgr.recordLaunchedVMM(test.VMID, mgr.vmmFor(test))
	if got := strings.Join(mgr.jailerArgs(test.VMID, 900002, nil), " "); !strings.Contains(got, "--exec-file "+mgr.cfg.FirecrackerNext.BinPath+" ") {
		t.Fatalf("next jailer args = %s, want the next binary", got)
	}
}

func TestLoadConfigFromEnv_JailedExecFile(t *testing.T) {
	root := t.TempDir()
	direct := fakeFirecrackerAt(t, filepath.Join(root, "fc-1.17.0", "bin"), "Firecracker v1.17.0")
	t.Setenv("VM_FIRECRACKER_BIN", direct)
	t.Setenv("VM_STATE_DIR", filepath.Join(root, "state"))
	if got := LoadConfigFromEnv().Jailer.FirecrackerBinPath; got != direct {
		t.Fatalf("default jailed exec file = %q, want the direct binary %q", got, direct)
	}
	t.Setenv("VM_JAILER_FIRECRACKER_BIN", " /nix/store/m-firecracker-static-1.17.0/bin/firecracker ")
	if got := LoadConfigFromEnv().Jailer.FirecrackerBinPath; got != "/nix/store/m-firecracker-static-1.17.0/bin/firecracker" {
		t.Fatalf("VM_JAILER_FIRECRACKER_BIN = %q", got)
	}

	// A next binary that is a script launches, but never jailed; a static
	// one jails.
	next := fakeFirecrackerAt(t, filepath.Join(root, "fc-next", "bin"), "Firecracker v1.18.0")
	t.Setenv("VM_FIRECRACKER_NEXT_BIN", next)
	t.Setenv("VM_FIRECRACKER_NEXT_ONLY", "class:test")
	if n := LoadConfigFromEnv().FirecrackerNext; n.BinPath != next || n.JailExec != "" {
		t.Fatalf("script next build = %+v, want launched but not jailable", n)
	}
	staticNext := writeELFFixture(t, filepath.Join(root, "fc-next-static", "bin", "firecracker"), "")
	if err := os.WriteFile(filepath.Join(filepath.Dir(staticNext), "jailer"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VM_FIRECRACKER_NEXT_BIN", staticNext)
	if n := LoadConfigFromEnv().FirecrackerNext; n.BinPath != staticNext || n.JailExec != staticNext {
		t.Fatalf("static next build = %+v, want jailable", n)
	}
}
