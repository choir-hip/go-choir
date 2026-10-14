package vmbackend

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Per-computer backend record and the rules for choosing and changing it.
//
// Authority: vmctl's ownership record will carry the requested backend (a
// per-computer setting, test computers only at first). vmmanager writes the
// backend that actually launched into the VM state dir at every cold boot
// (LaunchRecord), and snapshot metadata carries it too. A computer with no
// record ran Firecracker, because Firecracker is the only backend that has
// ever launched a computer.

// LaunchRecordName is the per-VM file under <VM_STATE_DIR>/<vmID>/.
const LaunchRecordName = "vm-backend.json"

const launchRecordVersion = 1

// LaunchRecord is the durable fact "this computer last cold booted on this
// backend".
type LaunchRecord struct {
	Version        int       `json:"version"`
	Backend        Name      `json:"backend"`
	BackendVersion string    `json:"backend_version,omitempty"`
	Epoch          int64     `json:"epoch,omitempty"`
	RecordedAt     time.Time `json:"recorded_at"`
	// Implicit is set on a record synthesised for a computer that has no
	// file (every computer before this seam). It is never written.
	Implicit bool `json:"-"`
}

// LaunchRecordPath is the record's path for a VM state dir.
func LaunchRecordPath(vmStateDir string) string {
	return filepath.Join(vmStateDir, LaunchRecordName)
}

// ReadLaunchRecord returns the VM's record. A missing file is the implicit
// Firecracker record, not an error. A file that exists but cannot be read
// or names an unknown backend is an error: guessing would boot a computer
// on a backend it never ran on.
func ReadLaunchRecord(vmStateDir string) (LaunchRecord, error) {
	data, err := os.ReadFile(LaunchRecordPath(vmStateDir))
	if errors.Is(err, os.ErrNotExist) {
		return LaunchRecord{Version: launchRecordVersion, Backend: Default, Implicit: true}, nil
	}
	if err != nil {
		return LaunchRecord{}, fmt.Errorf("vmbackend: read launch record: %w", err)
	}
	var rec LaunchRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return LaunchRecord{}, fmt.Errorf("vmbackend: parse launch record: %w", err)
	}
	if rec.Version != launchRecordVersion {
		return LaunchRecord{}, fmt.Errorf("vmbackend: launch record version %d, want %d", rec.Version, launchRecordVersion)
	}
	n, err := ParseName(string(rec.Backend))
	if err != nil || strings.TrimSpace(string(rec.Backend)) == "" {
		return LaunchRecord{}, fmt.Errorf("vmbackend: launch record names backend %q", rec.Backend)
	}
	rec.Backend = n
	return rec, nil
}

// WriteLaunchRecord writes the record atomically and durably (temp file,
// fsync, rename, fsync dir), the same discipline as snapshot metadata.
func WriteLaunchRecord(vmStateDir string, rec LaunchRecord) error {
	if _, err := ParseName(string(rec.Backend)); err != nil || rec.Backend == "" {
		return fmt.Errorf("vmbackend: refuse to record backend %q", rec.Backend)
	}
	rec.Version = launchRecordVersion
	if rec.RecordedAt.IsZero() {
		rec.RecordedAt = time.Now().UTC()
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	final := LaunchRecordPath(vmStateDir)
	tmp := final + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, final); err != nil {
		return err
	}
	dir, err := os.Open(vmStateDir)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

// Phase is the computer's lifecycle phase as far as a backend change cares.
type Phase string

const (
	PhaseStopped Phase = "stopped"
	PhasePending Phase = "pending"
	PhaseRunning Phase = "running"
	// PhaseHibernated covers both hibernate modes: a snapshot is not
	// portable, and a kill-hibernated computer must be stopped (an owner or
	// vmctl act) before its backend may change.
	PhaseHibernated Phase = "hibernated"
	PhaseFailed     Phase = "failed"
)

// ErrSwitchNotStopped refuses a backend change outside the stopped phase.
var ErrSwitchNotStopped = errors.New("vmbackend: a computer changes backend only while stopped")

// CheckSwitch says whether a computer in phase may move from one backend to
// another. Staying on the same backend is always allowed. A change is
// allowed only from PhaseStopped, so the next launch is a cold boot on the
// new backend: no process to adopt, no snapshot to load. Hibernated is
// refused even though a kill-hibernate keeps no memory: the snapshot-mode
// case would silently become a crash boot, and the restart rule allows only
// explicit stops to end work.
func CheckSwitch(from, to Name, phase Phase) error {
	if from == "" {
		from = Default
	}
	if to == "" {
		to = Default
	}
	if _, err := ParseName(string(from)); err != nil {
		return err
	}
	if _, err := ParseName(string(to)); err != nil {
		return err
	}
	if from == to {
		return nil
	}
	if phase != PhaseStopped {
		return fmt.Errorf("%w: %s -> %s requested in phase %q", ErrSwitchNotStopped, from, to, phase)
	}
	return nil
}

// Policy is the host's backend gate (VM_BACKENDS_ENABLED). The default
// enables Firecracker only, so nothing changes until a host opts in.
type Policy struct {
	enabled map[Name]bool
}

// ErrBackendNotEnabled refuses a backend the host has not enabled.
var ErrBackendNotEnabled = errors.New("vmbackend: backend not enabled on this host")

// ErrBackendTestOnly refuses a non-default backend for a computer that is
// not a test computer.
var ErrBackendTestOnly = errors.New("vmbackend: non-default backends are for test computers only")

// ParsePolicy reads a comma-separated list of backend names. Empty means
// Firecracker only. Unknown names are an error: a typo must not silently
// leave a host without the backend an operator meant to enable.
func ParsePolicy(value string) (Policy, error) {
	p := Policy{enabled: map[Name]bool{}}
	value = strings.TrimSpace(value)
	if value == "" {
		p.enabled[Default] = true
		return p, nil
	}
	for _, part := range strings.Split(value, ",") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		n, err := ParseName(part)
		if err != nil {
			return Policy{}, err
		}
		p.enabled[n] = true
	}
	if len(p.enabled) == 0 {
		return Policy{}, fmt.Errorf("vmbackend: VM_BACKENDS_ENABLED=%q enables nothing", value)
	}
	return p, nil
}

// Enabled reports whether the host may launch n.
func (p Policy) Enabled(n Name) bool { return p.enabled[n] }

// Select returns the backend a computer's next cold boot uses. requested
// is the computer's setting (empty = Default). A non-default backend needs
// both the host gate and a test computer.
func (p Policy) Select(requested Name, testComputer bool) (Name, error) {
	n, err := ParseName(string(requested))
	if err != nil {
		return "", err
	}
	if !p.Enabled(n) {
		return "", fmt.Errorf("%w: %s", ErrBackendNotEnabled, n)
	}
	if n != Default && !testComputer {
		return "", fmt.Errorf("%w: %s", ErrBackendTestOnly, n)
	}
	return n, nil
}
