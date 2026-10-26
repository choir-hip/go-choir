package vmctl

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Two retention rules beside the ephemeral one, each behind its own switch
// (off, dry-run, active; default off), and the report FP0 reads before
// either deletes anything
// (docs/problems/vm-state-retention-skips-test-computers-and-quarantines-2026-10-22.md):
//
//   - test_class_idle: a test-class computer (live tiers: a test account, or
//     a computer computer-tiers.json names as a test computer), stopped,
//     hibernated or failed, not held, idle at least TestIdleMinAge, and not
//     named by any per-computer selector (selectorEnvKeyRE). Removal is the ephemeral rule's path
//     (DestroyVMState, then the ownership) under a lifecycle claim, with
//     every check repeated at destroy time.
//   - quarantine_age_out: a data.img.quarantine-* of a cold recovery that
//     finished (journal phase done, or status active), older than
//     QuarantineMinAge, of a computer that is not protected, held or
//     selector-named. A pre-swap, failed or journal-less quarantine may be
//     the only copy of a computer's data and is never removed.
//
// Every removal by either rule appends an intent line and a result line to
// the manifest (retention-manifest.jsonl in the state dir).

const (
	retentionReasonTestClassIdle     = "test_class_idle"
	retentionReasonQuarantineAgeOut  = "quarantine_age_out"
	retentionManifestName            = "retention-manifest.jsonl"
	retentionVerdictKeep             = "keep"
	retentionVerdictWouldRemove      = "would_remove"
	retentionVerdictWillRemove       = "will_remove"
	defaultRetentionTestIdleMinAge   = 7 * 24 * time.Hour
	defaultRetentionQuarantineMinAge = 7 * 24 * time.Hour
	lifecycleRetention               = lifecycleHolder("retention removal")
	retentionMinAgeFloor             = 24 * time.Hour
)

// RetentionTestClassEntry is one test-class computer in the plan: what it
// holds, how long it has been idle, and whether the rule takes it or why not.
type RetentionTestClassEntry struct {
	VMID         string   `json:"vm_id"`
	ComputerID   string   `json:"computer_id,omitempty"`
	UserID       string   `json:"user_id,omitempty"`
	DesktopID    string   `json:"desktop_id,omitempty"`
	State        VMState  `json:"state,omitempty"`
	LastActiveAt string   `json:"last_active_at,omitempty"`
	IdleSeconds  int64    `json:"idle_seconds"`
	SizeBytes    int64    `json:"size_bytes"`
	Verdict      string   `json:"verdict"`
	Reasons      []string `json:"reasons,omitempty"`
}

// RetentionQuarantineEntry is one quarantined data image in the plan. The
// why fields come from the cold recovery's journal; the fencing token is
// never read into it.
type RetentionQuarantineEntry struct {
	VMID          string   `json:"vm_id"`
	ComputerID    string   `json:"computer_id,omitempty"`
	Name          string   `json:"name"`
	Path          string   `json:"path"`
	QuarantinedAt string   `json:"quarantined_at"`
	AgeSeconds    int64    `json:"age_seconds"`
	SizeBytes     int64    `json:"size_bytes"`
	RecoveryID    string   `json:"recovery_id,omitempty"`
	JournalPhase  string   `json:"journal_phase,omitempty"`
	JournalStatus string   `json:"journal_status,omitempty"`
	CanonicalHead string   `json:"canonical_head,omitempty"`
	Verdict       string   `json:"verdict"`
	Reasons       []string `json:"reasons,omitempty"`
}

// retentionManifestLine is one line of the removal manifest.
type retentionManifestLine struct {
	Kind          string `json:"kind"`
	Event         string `json:"event"` // removing, removed, failed
	At            string `json:"at"`
	VMID          string `json:"vm_id"`
	ComputerID    string `json:"computer_id,omitempty"`
	UserID        string `json:"user_id,omitempty"`
	Path          string `json:"path,omitempty"`
	SizeBytes     int64  `json:"size_bytes"`
	Why           string `json:"why"`
	QuarantinedAt string `json:"quarantined_at,omitempty"`
	RemovedAt     string `json:"removed_at,omitempty"`
	RecoveryID    string `json:"recovery_id,omitempty"`
	JournalPhase  string `json:"journal_phase,omitempty"`
	CanonicalHead string `json:"canonical_head,omitempty"`
	LastActiveAt  string `json:"last_active_at,omitempty"`
	Error         string `json:"error,omitempty"`
}

// selectorEnvKeyRE matches every per-computer selector in vmctl's
// environment (review round 2, R2-4): the staged-feature lists
// VM_*_ONLY (VM_GUEST_RECLAIM_ONLY, VM_CAPSULE_TUNNEL_ENFORCE_ONLY,
// VM_BALLOON_STATS_ONLY, VM_DATA_DISCARD_ONLY,
// VM_FIRECRACKER_NEXT_ONLY, VM_SNAPSHOT_DIFF_ONLY, VM_SNAPSHOT_VAULT_ONLY),
// VMCTL_JAILER_ONLY, and the CHOIR_*_COMPUTERS and CHOIR_*_ONLY allowlists
// (CHOIR_CLAUDE_DELEGATE_COMPUTERS, CHOIR_VISUAL_CAPTURE_COMPUTERS,
// CHOIR_CAPSULE_MIRROR_COMPUTERS, CHOIR_CAPSULE_MIRRORS_ONLY).
var selectorEnvKeyRE = regexp.MustCompile(`^((VM|VMCTL)_[A-Z0-9_]+_ONLY|CHOIR_[A-Z0-9_]+_(COMPUTERS|ONLY))$`)

// RetentionSelectorNamedFromEnviron returns the VM ids and computer ids that
// any per-computer selector names (selectorEnvKeyRE). class: and * tokens
// select by class or everything; they name no computer, so they protect none.
func RetentionSelectorNamedFromEnviron(environ []string) []string {
	seen := map[string]bool{}
	for _, kv := range environ {
		key, value, ok := strings.Cut(kv, "=")
		if !ok || !selectorEnvKeyRE.MatchString(key) {
			continue
		}
		for _, tok := range strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == ' ' || r == '\n' || r == '\t'
		}) {
			tok = strings.TrimSpace(tok)
			if tok == "" || tok == "*" || strings.HasPrefix(tok, "class:") {
				continue
			}
			seen[tok] = true
		}
	}
	out := make([]string, 0, len(seen))
	for tok := range seen {
		out = append(out, tok)
	}
	sort.Strings(out)
	return out
}

func normalizeRetentionRuleMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "dryrun", "observe", "observation", RetentionPruneModeDryRun:
		return RetentionPruneModeDryRun
	case "reclaim", "enforce", "prune", RetentionPruneModeActive:
		return RetentionPruneModeActive
	default:
		return RetentionPruneModeOff
	}
}

// retentionRuleVerdict is a matching entry's verdict: it is removed only
// when both the retention mode and the rule's own switch are active.
func retentionRuleVerdict(cfg RetentionPruneConfig, ruleMode string) string {
	if cfg.Mode == RetentionPruneModeActive && ruleMode == RetentionPruneModeActive {
		return retentionVerdictWillRemove
	}
	return retentionVerdictWouldRemove
}

func retentionSelectorNamed(cfg RetentionPruneConfig, own *VMOwnership) bool {
	if own == nil {
		return false
	}
	for _, tok := range cfg.SelectorNamed {
		if tok == strings.TrimSpace(own.VMID) || (own.ComputerID != "" && tok == strings.TrimSpace(own.ComputerID)) {
			return true
		}
	}
	return false
}

// testIdleKeepReasons is the test_class_idle rule: nil when it takes the
// computer, else every reason it does not. Used by the plan and again at
// destroy time.
func testIdleKeepReasons(own *VMOwnership, cfg RetentionPruneConfig, warmness WarmnessPolicyConfig, now time.Time) []string {
	if own == nil || strings.TrimSpace(own.VMID) == "" {
		return []string{"no_ownership"}
	}
	var reasons []string
	if class := warmnessClassForOwnership(own, warmness); class != WarmnessClassTest {
		reasons = append(reasons, "class_"+string(class))
	}
	if own.IsHeld() {
		reasons = append(reasons, "held")
	}
	if retentionSelectorNamed(cfg, own) {
		reasons = append(reasons, "selector_named")
	}
	// The ephemeral rule's shape (R2-5): only a test account's primary
	// interactive computer.
	if own.Kind != VMKindInteractive {
		reasons = append(reasons, "not_interactive")
	}
	if normalizeDesktopID(own.DesktopID) != PrimaryDesktopID {
		reasons = append(reasons, "not_primary_desktop")
	}
	switch own.State {
	case VMStateStopped, VMStateHibernated, VMStateFailed:
	default:
		reasons = append(reasons, "state_"+string(own.State))
	}
	if own.StoppedBy == "recover_current" {
		reasons = append(reasons, "stopped_to_recover")
	}
	if own.LastActiveAt.IsZero() {
		reasons = append(reasons, "never_active")
	} else if now.Sub(own.LastActiveAt) < cfg.TestIdleMinAge {
		reasons = append(reasons, "idle_below_min_age")
	}
	// DestroyVMState removes the whole dir: never one that holds a
	// quarantine the age-out rule would keep, or a recovery in flight.
	reasons = append(reasons, vmDirRecoveryKeepReasons(cfg.StateDir, own.VMID, now, cfg.QuarantineMinAge)...)
	return reasons
}

// vmDirRecoveryKeepReasons is why a rule that destroys an owned computer's
// whole dir (test_class_idle, the ephemeral rule) must keep it:
//   - holds_unfinished_recovery: a quarantine whose recovery did not finish
//     or cannot be read (no journal, failed, pre-swap, swapped, booted), a
//     staging image, a journal whose recovery is neither done nor failed,
//     or an unreadable dir; any of these may be the only copy;
//   - holds_young_quarantine: a finished recovery's quarantine younger
//     than minAge, the age-out rule's own minimum (off Linux every
//     quarantine is young: no ctime).
func vmDirRecoveryKeepReasons(stateDir, vmID string, now time.Time, minAge time.Duration) []string {
	root := filepath.Clean(stateDir)
	if root == "." || root == string(os.PathSeparator) || vmID == "" || filepath.Base(vmID) != vmID {
		return []string{"holds_unfinished_recovery"}
	}
	vmDir := filepath.Join(root, vmID)
	files, err := os.ReadDir(vmDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return []string{"holds_unfinished_recovery"}
	}
	unfinished, young := false, false
	for _, file := range files {
		name := file.Name()
		switch {
		case strings.HasPrefix(name, "data.img.staging-"):
			unfinished = true
		case strings.HasPrefix(name, "data.img.quarantine-"):
			generation, operation, ok := parseQuarantineName(name)
			if !ok || !file.Type().IsRegular() {
				unfinished = true
				continue
			}
			journal, ok := readQuarantineJournal(vmDir, generation, operation)
			if !ok || !recoveryFinished(journal) {
				unfinished = true
				continue
			}
			info, err := file.Info()
			if err != nil {
				unfinished = true
				continue
			}
			if now.Sub(inodeChangeTime(info, now)) < minAge {
				young = true
			}
		case strings.HasPrefix(name, "rec-") && strings.HasSuffix(name, ".journal"):
			var j quarantineJournal
			body, err := os.ReadFile(filepath.Join(vmDir, name))
			if err != nil || json.Unmarshal(body, &j) != nil {
				unfinished = true
				continue
			}
			if !recoveryFinished(j) && j.Status != "failed" {
				unfinished = true
			}
		}
	}
	var reasons []string
	if unfinished {
		reasons = append(reasons, "holds_unfinished_recovery")
	}
	if young {
		reasons = append(reasons, "holds_young_quarantine")
	}
	return reasons
}

// recoveryFinished: the journal's phase is done or its status active, and
// the status is not failed. swapped and booted are not finished.
func recoveryFinished(j quarantineJournal) bool {
	return j.Status != "failed" && (j.Phase == "done" || j.Status == "active")
}

// retentionTestClassReport lists every test-class computer with its verdict.
func retentionTestClassReport(cfg RetentionPruneConfig, warmness WarmnessPolicyConfig, ownerships []*VMOwnership, now time.Time) []RetentionTestClassEntry {
	var entries []RetentionTestClassEntry
	for _, own := range ownerships {
		if own == nil || warmnessClassForOwnership(own, warmness) != WarmnessClassTest {
			continue
		}
		entry := RetentionTestClassEntry{
			VMID:       own.VMID,
			ComputerID: own.ComputerID,
			UserID:     own.UserID,
			DesktopID:  own.DesktopID,
			State:      own.State,
			SizeBytes:  vmStateDirUsageBytes(cfg.StateDir, own.VMID),
			Verdict:    retentionVerdictKeep,
		}
		if !own.LastActiveAt.IsZero() {
			entry.LastActiveAt = own.LastActiveAt.UTC().Format(time.RFC3339Nano)
			entry.IdleSeconds = int64(now.Sub(own.LastActiveAt).Seconds())
		}
		entry.Reasons = testIdleKeepReasons(own, cfg, warmness, now)
		if len(entry.Reasons) == 0 {
			entry.Verdict = retentionRuleVerdict(cfg, cfg.TestIdleMode)
		}
		entries = append(entries, entry)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].SizeBytes != entries[j].SizeBytes {
			return entries[i].SizeBytes > entries[j].SizeBytes
		}
		return entries[i].VMID < entries[j].VMID
	})
	return entries
}

type quarantineJournal struct {
	RecoveryID    string `json:"recovery_id"`
	Phase         string `json:"phase"`
	Status        string `json:"status"`
	ComputerID    string `json:"computer_id"`
	CanonicalHead string `json:"canonical_head"`
}

// parseQuarantineName splits data.img.quarantine-<generation>-<operation>.
func parseQuarantineName(name string) (uint64, string, bool) {
	const prefix = "data.img.quarantine-"
	if !strings.HasPrefix(name, prefix) {
		return 0, "", false
	}
	suffix := strings.TrimPrefix(name, prefix)
	sep := strings.IndexByte(suffix, '-')
	if sep <= 0 {
		return 0, "", false
	}
	generation, err := strconv.ParseUint(suffix[:sep], 10, 64)
	operation := suffix[sep+1:]
	if err != nil || generation == 0 || operation == "" || filepath.Base(operation) != operation {
		return 0, "", false
	}
	return generation, operation, true
}

// readQuarantineJournal reads only the why fields of the recovery journal
// beside a quarantine; ok is false when there is no readable journal.
func readQuarantineJournal(vmDir string, generation uint64, operation string) (quarantineJournal, bool) {
	var j quarantineJournal
	body, err := os.ReadFile(filepath.Join(vmDir, fmt.Sprintf("rec-%d-%s.journal", generation, operation)))
	if err != nil || json.Unmarshal(body, &j) != nil {
		return quarantineJournal{}, false
	}
	return j, true
}

// quarantineInspection is one quarantine on disk.
type quarantineInspection struct {
	entry RetentionQuarantineEntry
	at    time.Time
}

// quarantineOwners resolves a quarantine's dir to its ownership and a
// journal's computer id to the ownership that names it.
type quarantineOwners struct {
	byVM       map[string]*VMOwnership
	byComputer map[string]*VMOwnership
}

func newQuarantineOwners(ownerships []*VMOwnership) quarantineOwners {
	owners := quarantineOwners{byVM: map[string]*VMOwnership{}, byComputer: map[string]*VMOwnership{}}
	for _, own := range ownerships {
		if own == nil {
			continue
		}
		owners.byVM[own.VMID] = own
		owners.byComputer[stableComputerID(own.UserID, own.DesktopID, own.ComputerID)] = own
	}
	return owners
}

// retentionStateDirName reports whether a dir under vm-state is vmctl's VM
// state (vm-* or candidate-*), as the orphan scan reads it. Anything else is
// host bookkeeping or an operator's saved copy and is never scanned.
func retentionStateDirName(name string) bool {
	return strings.HasPrefix(name, "vm-") || strings.HasPrefix(name, "candidate-")
}

// journalComputerProtected reports whether a journal's computer id is the
// paper or another platform computer, a paid account's computer (the
// owner's), or the computer of any ownership whose class is protected.
func journalComputerProtected(computerID string, warmness WarmnessPolicyConfig, owners quarantineOwners) bool {
	if warmness.PlatformComputerIDs[computerID] {
		return true
	}
	for userID := range warmness.PaidUserIDs {
		if stableComputerID(userID, PrimaryDesktopID, "") == computerID {
			return true
		}
	}
	if own := owners.byComputer[computerID]; own != nil && warmnessClassProtected(warmnessClassForOwnership(own, warmness)) {
		return true
	}
	return false
}

// inspectQuarantine reads one quarantine file and decides the age-out rule
// for it; the plan and the removal both use it. Only a quarantine of a
// test-class computer (R2-3), in a dir with an ownership record (R2-2),
// whose journal names that same computer and is finished, ages out.
func inspectQuarantine(cfg RetentionPruneConfig, warmness WarmnessPolicyConfig, owners quarantineOwners, vmDir, vmID, name string, now time.Time) quarantineInspection {
	path := filepath.Join(vmDir, name)
	entry := RetentionQuarantineEntry{VMID: vmID, Name: name, Path: path, Verdict: retentionVerdictKeep}
	owner := owners.byVM[vmID]
	if owner != nil {
		entry.ComputerID = stableComputerID(owner.UserID, owner.DesktopID, owner.ComputerID)
	}
	generation, operation, parsed := parseQuarantineName(name)
	info, err := os.Lstat(path)
	switch {
	case !parsed:
		entry.Reasons = []string{"unparsed_name"}
		return quarantineInspection{entry: entry}
	case err != nil:
		entry.Reasons = []string{"unreadable"}
		return quarantineInspection{entry: entry}
	case !info.Mode().IsRegular():
		entry.Reasons = []string{"not_regular"}
		return quarantineInspection{entry: entry}
	}
	// Off Linux there is no ctime: the age is zero and nothing ages out.
	at := inodeChangeTime(info, now)
	entry.QuarantinedAt = at.UTC().Format(time.RFC3339Nano)
	entry.AgeSeconds = int64(now.Sub(at).Seconds())
	entry.SizeBytes = allocatedBytes(info)

	var reasons []string
	journal, hasJournal := readQuarantineJournal(vmDir, generation, operation)
	if hasJournal {
		entry.RecoveryID = journal.RecoveryID
		entry.JournalPhase = journal.Phase
		entry.JournalStatus = journal.Status
		entry.CanonicalHead = journal.CanonicalHead
		if entry.ComputerID == "" {
			entry.ComputerID = journal.ComputerID
		}
	}
	switch {
	case !hasJournal:
		reasons = append(reasons, "journal_missing")
	case journal.Status == "failed":
		reasons = append(reasons, "recovery_failed")
	case !recoveryFinished(journal):
		reasons = append(reasons, "journal_phase_"+journal.Phase)
	}
	if hasJournal {
		jc := strings.TrimSpace(journal.ComputerID)
		switch {
		case jc == "":
			reasons = append(reasons, "journal_computer_missing")
		case journalComputerProtected(jc, warmness, owners):
			reasons = append(reasons, "journal_computer_protected")
		case owner != nil && jc != entry.ComputerID:
			reasons = append(reasons, "journal_computer_mismatch")
		}
	}
	if now.Sub(at) < cfg.QuarantineMinAge {
		reasons = append(reasons, "age_below_min_age")
	}
	if owner == nil {
		reasons = append(reasons, "no_ownership")
	} else {
		switch class := warmnessClassForOwnership(owner, warmness); {
		case warmnessClassProtected(class):
			reasons = append(reasons, "owner_protected_class")
		case class != WarmnessClassTest:
			reasons = append(reasons, "owner_not_test_class")
		}
		if owner.IsHeld() {
			reasons = append(reasons, "owner_held")
		}
		if owner.StoppedBy == "recover_current" {
			reasons = append(reasons, "owner_recovering")
		}
	}
	for _, tok := range cfg.SelectorNamed {
		if tok == vmID || (entry.ComputerID != "" && tok == entry.ComputerID) ||
			(hasJournal && journal.ComputerID != "" && tok == journal.ComputerID) {
			reasons = append(reasons, "selector_named")
			break
		}
	}
	entry.Reasons = reasons
	if len(reasons) == 0 {
		entry.Verdict = retentionRuleVerdict(cfg, cfg.QuarantineMode)
	}
	return quarantineInspection{entry: entry, at: at}
}

// retentionQuarantineReport lists every quarantined data image in vmctl's
// VM state dirs (vm-* and candidate-*). It reads directory entries and one
// journal per quarantine; it never opens an image.
func retentionQuarantineReport(cfg RetentionPruneConfig, warmness WarmnessPolicyConfig, ownerships []*VMOwnership, now time.Time) ([]RetentionQuarantineEntry, []string) {
	root := filepath.Clean(cfg.StateDir)
	if root == "." || root == string(os.PathSeparator) {
		return nil, []string{"quarantine report refused unsafe state dir"}
	}
	dirs, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []string{fmt.Sprintf("read state dir: %v", err)}
	}
	owners := newQuarantineOwners(ownerships)
	var entries []RetentionQuarantineEntry
	var warnings []string
	for _, dir := range dirs {
		if dir == nil || !dir.IsDir() || !retentionStateDirName(dir.Name()) {
			continue
		}
		vmID := dir.Name()
		vmDir := filepath.Join(root, vmID)
		files, err := os.ReadDir(vmDir)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("read %s: %v", vmID, err))
			continue
		}
		for _, file := range files {
			if !strings.HasPrefix(file.Name(), "data.img.quarantine-") {
				continue
			}
			entries = append(entries, inspectQuarantine(cfg, warmness, owners, vmDir, vmID, file.Name(), now).entry)
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].SizeBytes != entries[j].SizeBytes {
			return entries[i].SizeBytes > entries[j].SizeBytes
		}
		return entries[i].Path < entries[j].Path
	})
	return entries, warnings
}

func retentionManifestPath(cfg RetentionPruneConfig) string {
	if p := strings.TrimSpace(cfg.ManifestPath); p != "" {
		return p
	}
	return filepath.Join(cfg.StateDir, retentionManifestName)
}

// appendRetentionManifest appends one line and syncs it. A removal whose
// intent line cannot be written does not happen.
func appendRetentionManifest(cfg RetentionPruneConfig, line retentionManifestLine) error {
	body, err := json.Marshal(line)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(retentionManifestPath(cfg), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(body, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func (r *OwnershipRegistry) retentionNow() time.Time {
	r.mu.RLock()
	clock := r.retentionClock
	r.mu.RUnlock()
	if clock == nil {
		return time.Now()
	}
	return clock()
}

func (r *OwnershipRegistry) setRetentionClockForTest(clock func() time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.retentionClock = clock
}

// destroyTestIdleCandidate removes a test_class_idle candidate. Under the
// registry lock it repeats the rule against the live ownership, tiers and
// selectors and claims the computer's lifecycle, so a visit, resume or
// refresh cannot start it while its state is removed; then it writes the
// manifest intent, destroys the state and drops the ownership.
func (r *OwnershipRegistry) destroyTestIdleCandidate(candidate RetentionPruneCandidate) bool {
	now := r.retentionNow()
	r.mu.Lock()
	mgr := r.vmManager
	cfg := normalizeRetentionPruneConfig(r.retentionPrune)
	warmness := normalizeWarmnessPolicyConfig(r.warmnessPolicy)
	current := r.vmByID[candidate.VMID]
	if mgr == nil || current == nil || current.UserID != candidate.UserID ||
		cfg.Mode != RetentionPruneModeActive || cfg.TestIdleMode != RetentionPruneModeActive {
		r.mu.Unlock()
		return false
	}
	if reasons := testIdleKeepReasons(current, cfg, warmness, now); len(reasons) > 0 {
		r.mu.Unlock()
		log.Printf("vmctl: retention test_class_idle kept %s at destroy time: %s", candidate.VMID, strings.Join(reasons, ","))
		return false
	}
	key := ownershipKey(current.UserID, current.DesktopID)
	if r.lifecycleConflictLocked(key) != nil {
		r.mu.Unlock()
		return false
	}
	r.lifecycle[key] = lifecycleRetention
	snapshot := *current
	r.mu.Unlock()
	defer r.releaseLifecycle(key, lifecycleRetention)

	line := retentionManifestLine{
		Kind:         retentionReasonTestClassIdle,
		Event:        "removing",
		At:           now.UTC().Format(time.RFC3339Nano),
		VMID:         snapshot.VMID,
		ComputerID:   snapshot.ComputerID,
		UserID:       snapshot.UserID,
		Path:         filepath.Join(cfg.StateDir, snapshot.VMID),
		SizeBytes:    candidate.SizeBytes,
		Why:          fmt.Sprintf("test-class computer %s, idle since %s (min %s)", snapshot.State, snapshot.LastActiveAt.UTC().Format(time.RFC3339), cfg.TestIdleMinAge),
		LastActiveAt: snapshot.LastActiveAt.UTC().Format(time.RFC3339Nano),
	}
	if err := appendRetentionManifest(cfg, line); err != nil {
		log.Printf("vmctl: retention test_class_idle kept %s: manifest: %v", snapshot.VMID, err)
		return false
	}
	if err := mgr.DestroyVMState(snapshot.VMID); err != nil {
		line.Event, line.Error = "failed", err.Error()
		_ = appendRetentionManifest(cfg, line)
		log.Printf("vmctl: retention test_class_idle skipped %s: %v", snapshot.VMID, err)
		return false
	}
	r.mu.Lock()
	if cur := r.vmByID[snapshot.VMID]; cur != nil {
		delete(r.ownerships, ownershipKey(cur.UserID, cur.DesktopID))
		delete(r.vmByID, cur.VMID)
		r.saveLocked()
	}
	r.mu.Unlock()
	line.Event = "removed"
	line.RemovedAt = r.retentionNow().UTC().Format(time.RFC3339Nano)
	if err := appendRetentionManifest(cfg, line); err != nil {
		log.Printf("vmctl: retention test_class_idle removed %s; manifest result line: %v", snapshot.VMID, err)
	}
	log.Printf("vmctl: retention test_class_idle removed %s (computer %s, %d bytes)", snapshot.VMID, snapshot.ComputerID, candidate.SizeBytes)
	return true
}

// removeRetentionQuarantine ages out one quarantine: it inspects the file
// again against the live config, tiers and owner, writes the manifest
// intent, unlinks it and syncs the directory.
func (r *OwnershipRegistry) removeRetentionQuarantine(planned RetentionQuarantineEntry) (int64, bool) {
	now := r.retentionNow()
	r.mu.RLock()
	cfg := normalizeRetentionPruneConfig(r.retentionPrune)
	warmness := normalizeWarmnessPolicyConfig(r.warmnessPolicy)
	ownerships := make([]*VMOwnership, 0, len(r.ownerships))
	for _, own := range r.ownerships {
		ownerships = append(ownerships, cloneOwnership(own))
	}
	r.mu.RUnlock()
	if cfg.Mode != RetentionPruneModeActive || cfg.QuarantineMode != RetentionPruneModeActive {
		return 0, false
	}
	root := filepath.Clean(cfg.StateDir)
	vmDir := filepath.Join(root, planned.VMID)
	if root == "." || root == string(os.PathSeparator) || filepath.Base(planned.VMID) != planned.VMID ||
		!retentionStateDirName(planned.VMID) ||
		filepath.Base(planned.Name) != planned.Name || filepath.Join(vmDir, planned.Name) != planned.Path {
		return 0, false
	}
	inspected := inspectQuarantine(cfg, warmness, newQuarantineOwners(ownerships), vmDir, planned.VMID, planned.Name, now)
	if inspected.entry.Verdict != retentionVerdictWillRemove {
		log.Printf("vmctl: retention quarantine_age_out kept %s at removal time: %s", planned.Path, strings.Join(inspected.entry.Reasons, ","))
		return 0, false
	}
	entry := inspected.entry
	line := retentionManifestLine{
		Kind:          retentionReasonQuarantineAgeOut,
		Event:         "removing",
		At:            now.UTC().Format(time.RFC3339Nano),
		VMID:          entry.VMID,
		ComputerID:    entry.ComputerID,
		Path:          entry.Path,
		SizeBytes:     entry.SizeBytes,
		Why:           fmt.Sprintf("cold recovery %s quarantined the data image (journal phase %s, status %s, canonical head %s); aged out after %s", entry.RecoveryID, entry.JournalPhase, entry.JournalStatus, entry.CanonicalHead, cfg.QuarantineMinAge),
		QuarantinedAt: entry.QuarantinedAt,
		RecoveryID:    entry.RecoveryID,
		JournalPhase:  entry.JournalPhase,
		CanonicalHead: entry.CanonicalHead,
	}
	if err := appendRetentionManifest(cfg, line); err != nil {
		log.Printf("vmctl: retention quarantine_age_out kept %s: manifest: %v", entry.Path, err)
		return 0, false
	}
	if err := os.Remove(entry.Path); err != nil {
		line.Event, line.Error = "failed", err.Error()
		_ = appendRetentionManifest(cfg, line)
		log.Printf("vmctl: retention quarantine_age_out skipped %s: %v", entry.Path, err)
		return 0, false
	}
	if err := fsyncDirectory(vmDir); err != nil {
		log.Printf("vmctl: retention quarantine_age_out removed %s; directory sync: %v", entry.Path, err)
	}
	line.Event = "removed"
	line.RemovedAt = r.retentionNow().UTC().Format(time.RFC3339Nano)
	if err := appendRetentionManifest(cfg, line); err != nil {
		log.Printf("vmctl: retention quarantine_age_out removed %s; manifest result line: %v", entry.Path, err)
	}
	log.Printf("vmctl: retention quarantine_age_out removed %s (%d bytes, recovery %s)", entry.Path, entry.SizeBytes, entry.RecoveryID)
	return entry.SizeBytes, true
}

// ageOutRetentionQuarantines removes the plan's will_remove quarantines,
// within what is left of the pass's delete count and byte bounds.
func (r *OwnershipRegistry) ageOutRetentionQuarantines(plan RetentionPrunePlan, result *RetentionPruneResult) {
	cfg := normalizeRetentionPruneConfig(r.retentionPruneSnapshot())
	deletes, bytes := result.Deleted, result.BytesDeleted
	for _, entry := range plan.Quarantines {
		if entry.Verdict != retentionVerdictWillRemove {
			continue
		}
		if deletes >= cfg.MaxDeletes || (deletes > 0 && bytes+entry.SizeBytes > cfg.MaxBytes) {
			break
		}
		size, ok := r.removeRetentionQuarantine(entry)
		if !ok {
			continue
		}
		deletes++
		bytes += size
		result.QuarantinesRemoved++
		result.QuarantineBytesRemoved += size
		result.RemovedQuarantines = append(result.RemovedQuarantines, entry)
	}
}

func (r *OwnershipRegistry) retentionPruneSnapshot() RetentionPruneConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.retentionPrune
}
