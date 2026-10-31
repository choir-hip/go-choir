package vmmanager

// Guest releases (D2 slice S3; docs/design/density-d2-snapshots-survive-
// deploys-2026-10-26.md 3 and 4). A release is one guest image kept under
// <VM_GUEST_RELEASES_DIR>/<id>/ (id = the Nix store hash of the image), made
// by the NixOS activation snippet (scripts/node-b-guest-release-materialize)
// and read-only to vmctl. With VM_SNAPSHOT_RELEASE_RETAIN=on a cold boot
// launches from the current release's files, so the hibernated guest's
// memory stays paired with the store disk it was taken against, and a wake
// after a deploy resumes onto that same release when the floor, security
// epoch and ABI are unchanged and the release is inside the support window.
// Off, nothing here runs and launches are byte-identical to before.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// DefaultGuestReleaseSupportWindow is how long a superseded release's
	// snapshots may still resume (owner, 2026-10-25: one week).
	DefaultGuestReleaseSupportWindow = 168 * time.Hour
	releasePolicySchema              = 1
)

// Fallback reasons of the release rules R0 to R6 (design 4.5).
const (
	snapshotReasonReleaseRetainOff      = "release_retain_off"
	snapshotReasonReleaseMissing        = "release_missing"
	snapshotReasonReleaseCurrentUnknown = "release_current_unknown"
	snapshotReasonReleaseFloorMoved     = "release_floor_moved"
	snapshotReasonReleaseSecurityEpoch  = "release_security_epoch"
	snapshotReasonReleaseABI            = "release_abi_unsupported"
	snapshotReasonReleaseOutOfSupport   = "release_out_of_support"
)

var releaseIDPattern = regexp.MustCompile(`^[0-9a-df-np-sv-z]{32}$`)

// releasePolicy is a release's release.json, built into the guest image.
type releasePolicy struct {
	Schema        int    `json:"schema"`
	FloorDigest   string `json:"floor_digest"`
	SecurityEpoch int64  `json:"security_epoch"`
	GuestABI      int    `json:"guest_abi"`
	GuestABIMin   int    `json:"guest_abi_min"`
	BuildCommit   string `json:"build_commit"`
}

type guestRelease struct {
	ID        string
	Dir       string
	StorePath string
	Kernel    string
	Initrd    string
	Rootfs    string
	StoreDisk string
	// Layered releases (docs/design/layered-guest-image-2026-10-31.md): the
	// app layer is StoreDisk (storedisk-app.erofs, deliberately not named
	// storedisk.erofs, so an older vmctl's releaseByID refuses the release),
	// BaseID names the base and BaseDisk is the shared read-only base file.
	Layered  bool
	BaseID   string
	BaseDisk string
	// KernelParams is the trimmed contents of <Dir>/kernel-params.
	KernelParams   string
	Policy         releasePolicy
	FirstCurrentAt time.Time
}

func (m *Manager) releaseRetainOn() bool { return m.cfg.SnapshotReleaseRetain }

func (m *Manager) releasesDir() string {
	if d := strings.TrimSpace(m.cfg.GuestReleasesDir); d != "" {
		return d
	}
	return "/var/lib/go-choir/guest-releases"
}

// basesDir is where layered images' base layers live, one directory per base
// id, written once and never rewritten.
func (m *Manager) basesDir() string {
	if d := strings.TrimSpace(m.cfg.GuestBasesDir); d != "" {
		return d
	}
	return filepath.Join(filepath.Dir(m.releasesDir()), "guest-bases")
}

// releaseLayers is a layered release's layers.json, written by
// node-b-guest-release-materialize from the image's own layers.json.
type releaseLayers struct {
	Schema        int    `json:"schema"`
	Layout        string `json:"layout"`
	BaseID        string `json:"base_id"`
	BaseStorePath string `json:"base_store_path"`
}

const (
	releaseLayersSchema  = 1
	releaseLayoutLayered = "layered"
	baseDiskFile         = "storedisk-base.erofs"
	appDiskFile          = "storedisk-app.erofs"
)

func (m *Manager) releaseSupportWindow() time.Duration {
	if m.cfg.GuestReleaseSupportWindow > 0 {
		return m.cfg.GuestReleaseSupportWindow
	}
	return DefaultGuestReleaseSupportWindow
}

// releaseByID loads a release directory. Any unreadable or malformed file is
// an error: the caller refuses (release_missing) or runs legacy.
func (m *Manager) releaseByID(id string) (guestRelease, error) {
	if !releaseIDPattern.MatchString(id) {
		return guestRelease{}, fmt.Errorf("malformed release id %q", id)
	}
	dir := filepath.Join(m.releasesDir(), id)
	if info, err := os.Lstat(dir); err != nil || !info.IsDir() {
		return guestRelease{}, fmt.Errorf("release %s: no directory", id)
	}
	rel := guestRelease{
		ID:        id,
		Dir:       dir,
		Kernel:    filepath.Join(dir, "vmlinux"),
		Initrd:    filepath.Join(dir, "initrd"),
		Rootfs:    filepath.Join(dir, "rootfs.ext4"),
		StoreDisk: filepath.Join(dir, "storedisk.erofs"),
	}
	var installed struct {
		StorePath      string `json:"store_path"`
		FirstCurrentAt string `json:"first_current_at"`
	}
	data, err := os.ReadFile(filepath.Join(dir, "installed.json"))
	if err != nil {
		return guestRelease{}, fmt.Errorf("release %s: %w", id, err)
	}
	if err := json.Unmarshal(data, &installed); err != nil {
		return guestRelease{}, fmt.Errorf("release %s: installed.json: %w", id, err)
	}
	first, err := time.Parse(time.RFC3339, installed.FirstCurrentAt)
	if err != nil {
		return guestRelease{}, fmt.Errorf("release %s: first_current_at: %w", id, err)
	}
	rel.StorePath, rel.FirstCurrentAt = installed.StorePath, first
	data, err = os.ReadFile(filepath.Join(dir, "release.json"))
	if err != nil {
		return guestRelease{}, fmt.Errorf("release %s: %w", id, err)
	}
	if err := json.Unmarshal(data, &rel.Policy); err != nil || rel.Policy.Schema != releasePolicySchema || rel.Policy.FloorDigest == "" {
		return guestRelease{}, fmt.Errorf("release %s: release.json is not a schema %d policy", id, releasePolicySchema)
	}
	data, err = os.ReadFile(filepath.Join(dir, "kernel-params"))
	if err != nil || strings.TrimSpace(string(data)) == "" {
		return guestRelease{}, fmt.Errorf("release %s: kernel-params unreadable or empty", id)
	}
	rel.KernelParams = strings.TrimSpace(string(data))
	layersData, layersErr := os.ReadFile(filepath.Join(dir, "layers.json"))
	if layersErr == nil {
		if err := m.loadReleaseLayers(&rel, layersData); err != nil {
			return guestRelease{}, err
		}
	} else if !os.IsNotExist(layersErr) {
		return guestRelease{}, fmt.Errorf("release %s: layers.json: %w", id, layersErr)
	}
	files := []string{rel.Kernel, rel.Initrd, rel.Rootfs, rel.StoreDisk}
	if rel.Layered {
		files = append(files, rel.BaseDisk)
	}
	for _, p := range files {
		if info, err := os.Lstat(p); err != nil || !info.Mode().IsRegular() {
			return guestRelease{}, fmt.Errorf("release %s: %s is not a regular file", id, filepath.Base(p))
		}
	}
	return rel, nil
}

// loadReleaseLayers turns rel into a layered release: the app layer replaces
// StoreDisk, the base is addressed by its id under the bases directory. A
// release with both storedisk.erofs and layers.json is ambiguous and refused.
func (m *Manager) loadReleaseLayers(rel *guestRelease, data []byte) error {
	var l releaseLayers
	if err := json.Unmarshal(data, &l); err != nil {
		return fmt.Errorf("release %s: layers.json: %w", rel.ID, err)
	}
	if l.Schema != releaseLayersSchema || l.Layout != releaseLayoutLayered || !releaseIDPattern.MatchString(l.BaseID) {
		return fmt.Errorf("release %s: layers.json is not a schema %d layered layout", rel.ID, releaseLayersSchema)
	}
	if _, err := os.Lstat(filepath.Join(rel.Dir, "storedisk.erofs")); err == nil {
		return fmt.Errorf("release %s: both storedisk.erofs and layers.json", rel.ID)
	}
	rel.Layered = true
	rel.BaseID = l.BaseID
	rel.StoreDisk = filepath.Join(rel.Dir, appDiskFile)
	rel.BaseDisk = filepath.Join(m.basesDir(), l.BaseID, baseDiskFile)
	return nil
}

// currentRelease resolves VM_GUEST_RELEASE_LINK once: readlink, take the
// 32-character store hash, load that release directory, and require its
// installed.json to name the link's target. ok=false means legacy.
func (m *Manager) currentRelease() (guestRelease, bool) {
	if !m.releaseRetainOn() {
		return guestRelease{}, false
	}
	target, ok := m.guestImageTarget()
	if !ok {
		return guestRelease{}, false
	}
	base := filepath.Base(target)
	if len(base) < 32 {
		return guestRelease{}, false
	}
	rel, err := m.releaseByID(base[:32])
	if err != nil || rel.StorePath != target {
		return guestRelease{}, false
	}
	return rel, true
}

// setLaunchRelease points cfg's boot artifacts at rel.
func setLaunchRelease(cfg *VMConfig, rel guestRelease) {
	cfg.KernelImagePath, cfg.InitrdPath, cfg.RootfsPath, cfg.StoreDiskPath = rel.Kernel, rel.Initrd, rel.Rootfs, rel.StoreDisk
	cfg.BaseDiskPath = rel.BaseDisk // "" for a single image: a stale base never rides along
	cfg.KernelParams = rel.KernelParams
	cfg.GuestRelease = rel.ID
}

// bindCurrentRelease makes cfg a launch from the current release when the
// switch is on and the artifacts are not already chosen. When the current
// release cannot be resolved the launch stays legacy (m.cfg paths) and cfg
// carries no release. With the switch off it does nothing.
func (m *Manager) bindCurrentRelease(cfg *VMConfig) {
	if !m.releaseRetainOn() || cfg.GuestRelease != "" || cfg.KernelImagePath != "" || cfg.StoreDiskPath != "" || cfg.BaseDiskPath != "" {
		return
	}
	if rel, ok := m.currentRelease(); ok {
		setLaunchRelease(cfg, rel)
	}
}

// refreshLaunchArtifacts re-chooses the boot artifacts and kernel parameters of
// a computer that boots again from its stored config (a cold ResumeVM, a
// RecoverVM). The stored values were filled at its original launch: after a
// NixOS switch they name the previous image's init= path (H1, review 318
// DeepSeek 2). Artifacts and parameters an explicit override carries are kept;
// the rest are blanked, bound to the current release when the D2 switch is on,
// and filled from the manager (legacy), which is what a fresh boot does. The
// machine shape is untouched. Without a store disk (the legacy rootfs mode)
// nothing is re-chosen, as refreshConfigForCurrentDeploy.
func (m *Manager) refreshLaunchArtifacts(cfg *VMConfig, overrides VMConfig) {
	if m.cfg.StoreDiskPath == "" {
		return
	}
	cfg.GuestRelease = ""
	cfg.BaseDiskPath = ""
	if overrides.KernelImagePath == "" {
		cfg.KernelImagePath = ""
	}
	if overrides.InitrdPath == "" {
		cfg.InitrdPath = ""
	}
	if overrides.RootfsPath == "" {
		cfg.RootfsPath = ""
	}
	if overrides.StoreDiskPath == "" {
		cfg.StoreDiskPath = ""
	}
	if overrides.KernelParams == "" {
		cfg.KernelParams = ""
	}
	m.bindCurrentRelease(cfg)
	m.applyLaunchDefaults(cfg)
}

// reattachConfig is the config of a reattached VM (ReattachVMWithConfig).
// With the switch on, the launch record names the release the process was
// launched from; if its epoch matches and the release directory loads, the
// config names that release, not the current one (H2). Otherwise it is
// today's config, and the VM's next snapshot is legacy.
func (m *Manager) reattachConfig(vmID string, epoch int64, overrides VMConfig) VMConfig {
	cfg := mergeVMConfigOverrides(VMConfig{
		VMID:              vmID,
		KernelImagePath:   m.cfg.KernelImagePath,
		InitrdPath:        m.cfg.InitrdPath,
		RootfsPath:        m.cfg.RootfsPath,
		StoreDiskPath:     m.cfg.StoreDiskPath,
		GuestPort:         m.cfg.GuestPort,
		MachineCPUCount:   m.cfg.MachineCPUCount,
		MachineMemSizeMib: m.cfg.MachineMemSizeMib,
		PersistentDir:     filepath.Join(m.cfg.StateDir, vmID, "persist"),
		Epoch:             epoch,
	}, overrides)
	if m.releaseRetainOn() && overrides.StoreDiskPath == "" {
		if rec, ok := m.readLaunchRecord(vmID, epoch); ok && rec.Release != "" {
			if rel, err := m.releaseByID(rec.Release); err == nil {
				setLaunchRelease(&cfg, rel)
			}
		}
	}
	return cfg
}

// currentReleaseID is the id of the current release, "" when legacy.
func (m *Manager) currentReleaseID() string {
	if rel, ok := m.currentRelease(); ok {
		return rel.ID
	}
	return ""
}

// CurrentGuestRelease is the id of the current release, "" when the switch is
// off or the current release cannot be resolved (vmctl health).
func (m *Manager) CurrentGuestRelease() string { return m.currentReleaseID() }

// CurrentGuestReleaseInfo is the current release's id and the commit its image
// was built from; both "" when the switch is off or the release is unresolved.
func (m *Manager) CurrentGuestReleaseInfo() (id, commit string) {
	if rel, ok := m.currentRelease(); ok {
		return rel.ID, rel.Policy.BuildCommit
	}
	return "", ""
}

// releaseRefusal applies R0 to R6 to a snapshot meta. "" means the release
// rules pass (or the meta is legacy). cur is the current release when it
// resolved.
func (m *Manager) releaseRefusal(meta *SnapshotMeta, now time.Time) string {
	if meta.GuestRelease == "" {
		return ""
	}
	if !m.releaseRetainOn() {
		return snapshotReasonReleaseRetainOff // R0
	}
	rel, err := m.releaseByID(meta.GuestRelease)
	if err != nil {
		return snapshotReasonReleaseMissing // R1
	}
	cur, ok := m.currentRelease()
	if !ok {
		return snapshotReasonReleaseCurrentUnknown // R2
	}
	if rel.Policy.FloorDigest != cur.Policy.FloorDigest {
		return snapshotReasonReleaseFloorMoved // R3
	}
	if rel.Policy.SecurityEpoch != cur.Policy.SecurityEpoch {
		return snapshotReasonReleaseSecurityEpoch // R4: lower (a fix landed) and higher (a host rollback) both refuse
	}
	if rel.Policy.GuestABI < cur.Policy.GuestABIMin || rel.Policy.GuestABI > cur.Policy.GuestABI {
		return snapshotReasonReleaseABI // R5
	}
	if rel.ID != cur.ID && now.Sub(rel.FirstCurrentAt) > m.releaseSupportWindow() {
		return snapshotReasonReleaseOutOfSupport // R6; the current release never ages out
	}
	return ""
}

// GuestReleaseInUse is one release in the in-use answer.
type GuestReleaseInUse struct {
	ID string `json:"id"`
	// BaseID is the layered base the release names, "" for a single image.
	// Retention keeps a base while any retained release names it; this lets
	// it also keep the base of a release that is only in use.
	BaseID    string   `json:"base_id,omitempty"`
	Running   []string `json:"running"`
	Snapshots []string `json:"snapshots"`
}

// GuestReleaseUsage is GET /internal/vmctl/guest-releases: what retention may
// not delete. Complete is false when a snapshot root that is configured could
// not be read.
type GuestReleaseUsage struct {
	Retain   bool                `json:"retain"`
	Current  string              `json:"current"`
	Complete bool                `json:"complete"`
	InUse    []GuestReleaseInUse `json:"in_use"`
}

// GuestReleaseUsage lists the releases live VMs run and unconsumed snapshots
// (younger than the max age, every root including the vault) name.
func (m *Manager) GuestReleaseUsage() GuestReleaseUsage {
	use := GuestReleaseUsage{Retain: m.releaseRetainOn(), Current: m.currentReleaseID(), Complete: true, InUse: []GuestReleaseInUse{}}
	byID := map[string]*GuestReleaseInUse{}
	entry := func(id string) *GuestReleaseInUse {
		if e, ok := byID[id]; ok {
			return e
		}
		e := &GuestReleaseInUse{ID: id, Running: []string{}, Snapshots: []string{}}
		if rel, err := m.releaseByID(id); err == nil {
			e.BaseID = rel.BaseID
		}
		byID[id] = e
		return e
	}
	m.mu.RLock()
	for vmID, inst := range m.vms {
		if inst.Config.GuestRelease != "" && (inst.State == StateRunning || inst.State == StatePending) {
			e := entry(inst.Config.GuestRelease)
			e.Running = append(e.Running, vmID)
		}
	}
	m.mu.RUnlock()
	now := time.Now()
	for _, root := range m.snapshotRoots() {
		entries, err := os.ReadDir(root)
		if err != nil {
			use.Complete = false
			continue
		}
		for _, de := range entries {
			if !de.IsDir() {
				continue
			}
			meta, err := readSnapshotMeta(snapshotDirIn(root, de.Name()))
			if err != nil || meta.Consumed || meta.GuestRelease == "" || !now.Before(meta.CreatedAt.Add(m.snapshotMaxAge())) {
				continue
			}
			e := entry(meta.GuestRelease)
			e.Snapshots = append(e.Snapshots, de.Name())
		}
	}
	for _, e := range byID {
		use.InUse = append(use.InUse, *e)
	}
	return use
}

// GuestReleaseVMState is the release a live VM was launched from and whether
// it is behind the current release.
type GuestReleaseVMState struct {
	GuestRelease string
	Behind       bool
}

// GuestReleaseHealth is the current release and, for each running VM launched
// from a release, that release. The map is built under the manager lock: the
// instances' state and config are mutated under it by every lifecycle (review
// 331 DeepSeek 3).
func (m *Manager) GuestReleaseHealth() (string, map[string]GuestReleaseVMState) {
	current := m.currentReleaseID()
	out := map[string]GuestReleaseVMState{}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for vmID, inst := range m.vms {
		rel := inst.Config.GuestRelease
		if rel == "" || inst.State != StateRunning {
			continue
		}
		out[vmID] = GuestReleaseVMState{GuestRelease: rel, Behind: current != "" && current != rel}
	}
	return current, out
}
