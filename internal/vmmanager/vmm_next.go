package vmmanager

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// A second Firecracker build, chosen per computer at each launch (NET-08,
// RES-16; P3 of docs/definitions/choir-launch-fp8-image-jailer-2026-10-21.md).
//
// VM_FIRECRACKER_BIN is one binary for the host, so a Firecracker bump
// changes every computer in one deploy and refuses every hibernate snapshot
// at once. The next build is launched only for the computers its selector
// names, so a new version can be proven on one test computer first. 1.17
// (and data-drive discard) was proven that way and is now the pinned build
// (docs/problems/firecracker-1-17-for-every-computer-2026-10-22.md); with
// VM_FIRECRACKER_NEXT_BIN unset this seam is inert until the next bump:
//
//	VM_FIRECRACKER_NEXT_BIN    the next firecracker (same file name as the
//	                           pinned one: the jail directory is named after it)
//	VM_JAILER_NEXT_BIN         its jailer (default: beside it)
//	VM_FIRECRACKER_NEXT_ONLY   VM ids, computer ids, class:test or *
//	                           (computer_select.go); empty: next is off
//
// A next binary that is missing, not executable, or named differently is
// refused at load (logged) and every launch uses the pinned build. The build
// a VM launched with is recorded in its state dir, so a snapshot taken from
// it names the version it really ran (a restore under the other build is
// refused as firecracker_version_mismatch and cold boots) and a restarted
// vmctl still knows it.

// FirecrackerNextConfig is the next build and who launches it.
type FirecrackerNextConfig struct {
	BinPath       string
	JailerBinPath string
	Only          []string
	// JailExec is the next binary when it is static (it can run in the
	// jail); empty means a computer on the next build launches direct.
	JailExec string
	// DataDiscard is VM_DATA_DISCARD for the next build: on only if the
	// flag is on and the next binary reports 1.17.0 or newer.
	DataDiscard bool
}

// vmmBuild is the Firecracker (and jailer) one launch uses.
type vmmBuild struct {
	Firecracker string `json:"firecracker"`
	Jailer      string `json:"jailer,omitempty"`
	// JailedFirecracker is the jailer's --exec-file: a static build of the
	// same version (VM_JAILER_FIRECRACKER_BIN for the pinned build; the
	// next binary itself when it is static). Empty on a next build that is
	// not static: that computer launches direct.
	JailedFirecracker string `json:"jailed_firecracker,omitempty"`
	Next              bool   `json:"next,omitempty"`
}

const launchedVMMFileName = "vmm-build.json"

func loadFirecrackerNextFromEnv(pinnedBin string, discardFlag bool) FirecrackerNextConfig {
	bin := strings.TrimSpace(os.Getenv("VM_FIRECRACKER_NEXT_BIN"))
	if bin == "" {
		return FirecrackerNextConfig{}
	}
	only := parseComputerSelector(os.Getenv("VM_FIRECRACKER_NEXT_ONLY"))
	if len(only) == 0 {
		log.Printf("vmmanager: VM_FIRECRACKER_NEXT_BIN is set but VM_FIRECRACKER_NEXT_ONLY is empty; the next build is off")
		return FirecrackerNextConfig{}
	}
	if err := checkExecutable(bin); err != nil {
		log.Printf("vmmanager: VM_FIRECRACKER_NEXT_BIN refused (%v); every launch uses %s", err, pinnedBin)
		return FirecrackerNextConfig{}
	}
	if canonicalBase(bin) != canonicalBase(pinnedBin) {
		log.Printf("vmmanager: VM_FIRECRACKER_NEXT_BIN %s is not named like %s (the jail dir is named after the binary); the next build is off", bin, pinnedBin)
		return FirecrackerNextConfig{}
	}
	jailer := strings.TrimSpace(os.Getenv("VM_JAILER_NEXT_BIN"))
	if jailer == "" {
		jailer = filepath.Join(filepath.Dir(bin), "jailer")
	}
	if err := checkExecutable(jailer); err != nil {
		log.Printf("vmmanager: next jailer %s refused (%v); the next build is off", jailer, err)
		return FirecrackerNextConfig{}
	}
	n := FirecrackerNextConfig{BinPath: bin, JailerBinPath: jailer, Only: only}
	// A jailed launch on the next build runs the next binary itself in the
	// jail, so only a static one can be jailed; otherwise its computers
	// launch direct (confinementReasonNextNotStatic).
	if err := staticELFError(bin); err != nil {
		log.Printf("vmmanager: next Firecracker build cannot be jailed (%v); its computers launch direct", err)
	} else {
		n.JailExec = bin
	}
	n.DataDiscard = dataDiscardEnabledFromEnv(discardFlag, bin)
	log.Printf("vmmanager: next Firecracker build %s (%s) for %s; data discard allowed=%v",
		bin, firecrackerBuildVersion(bin), strings.Join(only, ","), n.DataDiscard)
	return n
}

func checkExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return os.ErrPermission
	}
	return nil
}

func canonicalBase(bin string) string {
	if bin == "" {
		bin = "firecracker"
	}
	if real, err := filepath.EvalSymlinks(bin); err == nil {
		bin = real
	}
	return filepath.Base(bin)
}

func (m *Manager) pinnedVMM() vmmBuild {
	return vmmBuild{Firecracker: m.cfg.FirecrackerBinPath, Jailer: m.cfg.Jailer.BinPath, JailedFirecracker: m.jailedExec()}
}

// vmmFor is the build cfg's next launch uses.
func (m *Manager) vmmFor(cfg VMConfig) vmmBuild {
	n := m.cfg.FirecrackerNext
	if n.BinPath != "" && selectsComputer(n.Only, cfg) {
		return vmmBuild{Firecracker: n.BinPath, Jailer: n.JailerBinPath, JailedFirecracker: n.JailExec, Next: true}
	}
	return m.pinnedVMM()
}

// dataDiscardFor is the data drive's discard decision for cfg's launch: the
// build it launches must allow it, and VM_DATA_DISCARD_ONLY (if set) must
// name it.
func (m *Manager) dataDiscardFor(cfg VMConfig) bool {
	allowed := m.cfg.DataDiscard
	if m.vmmFor(cfg).Next {
		allowed = m.cfg.FirecrackerNext.DataDiscard
	}
	if !allowed {
		return false
	}
	return len(m.cfg.DataDiscardOnly) == 0 || selectsComputer(m.cfg.DataDiscardOnly, cfg)
}

// recordLaunchedVMM notes the build vmID is launching, in memory and in its
// state dir (best effort: a missing record reads as the pinned build).
func (m *Manager) recordLaunchedVMM(vmID string, b vmmBuild) {
	m.launchedMu.Lock()
	if m.launchedVMMs == nil {
		m.launchedVMMs = map[string]vmmBuild{}
	}
	m.launchedVMMs[vmID] = b
	m.launchedMu.Unlock()
	data, err := json.Marshal(b)
	if err != nil {
		return
	}
	// A new VM's state dir may not exist yet; bootVM creates it with the
	// same mode moments later.
	dir := m.vmStateDir(vmID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		log.Printf("vmmanager: record VMM build of VM %s: %v", vmID, err)
		return
	}
	if err := writeFileDurably(dir, launchedVMMFileName, append(data, '\n'), 0o600); err != nil {
		log.Printf("vmmanager: record VMM build of VM %s: %v", vmID, err)
	}
}

// launchedVMM is the build vmID last launched with; the pinned build when
// nothing is recorded (a VM launched before the record existed).
func (m *Manager) launchedVMM(vmID string) vmmBuild {
	m.launchedMu.Lock()
	b, ok := m.launchedVMMs[vmID]
	m.launchedMu.Unlock()
	if ok {
		return b
	}
	if data, err := os.ReadFile(filepath.Join(m.vmStateDir(vmID), launchedVMMFileName)); err == nil {
		var rec vmmBuild
		if json.Unmarshal(data, &rec) == nil && rec.Firecracker != "" {
			if rec.Jailer == "" {
				rec.Jailer = m.cfg.Jailer.BinPath
			}
			// A record from before the jailed exec file existed: the pinned
			// build's (a next build with no record of it launches direct).
			if rec.JailedFirecracker == "" && !rec.Next {
				rec.JailedFirecracker = m.jailedExec()
			}
			return rec
		}
	}
	return m.pinnedVMM()
}
