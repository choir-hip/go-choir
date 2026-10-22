package vmctl

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	RetentionPruneModeOff    = "off"
	RetentionPruneModeDryRun = "dry-run"
	RetentionPruneModeActive = "active"
)

// RetentionPruneConfig controls deletion of VM state that is no longer needed
// for product rollback. It is intentionally separate from pressure hibernation:
// this policy deletes only orphan dirs and explicitly ephemeral accounts.
type RetentionPruneConfig struct {
	Mode                    string
	StateDir                string
	AuthDBPath              string
	EphemeralEmailDomains   []string
	EphemeralUserIDPrefixes []string
	OrphanMinAge            time.Duration
	EphemeralMinAge         time.Duration
	MaxDeletes              int
	MaxBytes                int64
	// TestIdleMode and QuarantineMode switch the test_class_idle and
	// quarantine_age_out rules (retention_test_class.go): off, dry-run
	// (reported, never removed) or active (removed when Mode is active too).
	TestIdleMode     string
	TestIdleMinAge   time.Duration
	QuarantineMode   string
	QuarantineMinAge time.Duration
	// SelectorNamed holds the VM and computer ids that per-computer
	// selectors name (VM_*_ONLY, VMCTL_*_ONLY, CHOIR_*_COMPUTERS,
	// CHOIR_*_ONLY; selectorEnvKeyRE);
	// neither new rule touches them.
	SelectorNamed []string
	// ManifestPath is the removal manifest; default <StateDir>/retention-manifest.jsonl.
	ManifestPath string
}

type RetentionPruneConfigSummary struct {
	Mode                    string   `json:"mode"`
	StateDir                string   `json:"state_dir,omitempty"`
	AuthDBPathConfigured    bool     `json:"auth_db_path_configured"`
	EphemeralEmailDomains   []string `json:"ephemeral_email_domains,omitempty"`
	EphemeralUserIDPrefixes []string `json:"ephemeral_user_id_prefixes,omitempty"`
	OrphanMinAgeSeconds     int64    `json:"orphan_min_age_seconds"`
	EphemeralMinAgeSeconds  int64    `json:"ephemeral_min_age_seconds"`
	MaxDeletes              int      `json:"max_deletes"`
	MaxBytes                int64    `json:"max_bytes,omitempty"`
	TestIdleMode            string   `json:"test_idle_mode"`
	TestIdleMinAgeSeconds   int64    `json:"test_idle_min_age_seconds,omitempty"`
	QuarantineMode          string   `json:"quarantine_mode"`
	QuarantineMinAgeSeconds int64    `json:"quarantine_min_age_seconds,omitempty"`
	SelectorNamed           []string `json:"selector_named,omitempty"`
}

type RetentionPruneInventory struct {
	Ownerships                 int   `json:"ownerships"`
	StateDirs                  int   `json:"state_dirs"`
	OrphanStateDirs            int   `json:"orphan_state_dirs"`
	DetachedRollbackStateDirs  int   `json:"detached_rollback_state_dirs"`
	EphemeralOwnerships        int   `json:"ephemeral_ownerships"`
	Candidates                 int   `json:"candidates"`
	CandidateBytes             int64 `json:"candidate_bytes"`
	ProjectedDeleteCount       int   `json:"projected_delete_count"`
	ProjectedDeleteBytes       int64 `json:"projected_delete_bytes"`
	TestClassComputers         int   `json:"test_class_computers,omitempty"`
	TestClassBytes             int64 `json:"test_class_bytes,omitempty"`
	TestClassWouldRemove       int   `json:"test_class_would_remove,omitempty"`
	TestClassWouldRemoveBytes  int64 `json:"test_class_would_remove_bytes,omitempty"`
	Quarantines                int   `json:"quarantines,omitempty"`
	QuarantineBytes            int64 `json:"quarantine_bytes,omitempty"`
	QuarantineWouldRemove      int   `json:"quarantine_would_remove,omitempty"`
	QuarantineWouldRemoveBytes int64 `json:"quarantine_would_remove_bytes,omitempty"`
	KeptStateDirs              int   `json:"kept_state_dirs,omitempty"`
	KeptStateDirBytes          int64 `json:"kept_state_dir_bytes,omitempty"`
}

type RetentionPruneCandidate struct {
	VMID           string  `json:"vm_id"`
	UserID         string  `json:"user_id,omitempty"`
	EmailDomain    string  `json:"email_domain,omitempty"`
	Kind           VMKind  `json:"kind,omitempty"`
	State          VMState `json:"state,omitempty"`
	DesktopID      string  `json:"desktop_id,omitempty"`
	LastActiveAt   string  `json:"last_active_at,omitempty"`
	AgeSeconds     int64   `json:"age_seconds"`
	SizeBytes      int64   `json:"size_bytes"`
	Reason         string  `json:"reason"`
	ProposedAction string  `json:"proposed_action"`
}

type RetentionProtectedState struct {
	ReceiptID   string `json:"receipt_id"`
	RouteSlotID string `json:"route_slot_id"`
	VMID        string `json:"vm_id"`
}

// RetentionKeptStateDir is a dir a destroying rule (orphan_state_dir or
// ephemeral_test_primary) would otherwise take but keeps, because it holds
// recovery state that may be the only copy of a computer's data.
type RetentionKeptStateDir struct {
	VMID      string   `json:"vm_id"`
	UserID    string   `json:"user_id,omitempty"`
	Rule      string   `json:"rule"`
	SizeBytes int64    `json:"size_bytes"`
	Reasons   []string `json:"reasons"`
	// Release says what lets the dir go: vmctl never does on its own.
	Release string `json:"release"`
}

const (
	retentionReleaseOrphan    = "an owner act: read the dir (a quarantine may be the only copy of a computer's data), then delete it or restore its ownership; no rule releases it"
	retentionReleaseEphemeral = "the recovery finishing (journal done, quarantine older than the quarantine minimum age), or an owner act"
)

type RetentionPrunePlan struct {
	Mode                    string                    `json:"mode"`
	Decision                string                    `json:"decision"`
	Reason                  string                    `json:"reason"`
	Inventory               RetentionPruneInventory   `json:"inventory"`
	Candidates              []RetentionPruneCandidate `json:"candidates,omitempty"`
	ProtectedDetachedLegacy []RetentionProtectedState `json:"protected_detached_legacy,omitempty"`
	// TestClass and Quarantines list every test-class computer and every
	// quarantined data image while their rule is not off, removable or not.
	TestClass []RetentionTestClassEntry `json:"test_class,omitempty"`
	// KeptStateDirs lists every dir the orphan or ephemeral rule keeps for
	// its recovery state (review round 3, R3-2 and R3-5).
	KeptStateDirs []RetentionKeptStateDir     `json:"kept_state_dirs,omitempty"`
	Quarantines   []RetentionQuarantineEntry  `json:"quarantines,omitempty"`
	Config        RetentionPruneConfigSummary `json:"config"`
	Warnings      []string                    `json:"warnings,omitempty"`
}

type RetentionPruneResult struct {
	Status       string                    `json:"status"`
	Deleted      int                       `json:"deleted"`
	BytesDeleted int64                     `json:"bytes_deleted"`
	DeletedVMs   []RetentionPruneCandidate `json:"deleted_vms,omitempty"`
	PlanBefore   RetentionPrunePlan        `json:"plan_before"`
	PlanAfter    RetentionPrunePlan        `json:"plan_after"`
	Warnings     []string                  `json:"warnings,omitempty"`

	QuarantinesRemoved     int                        `json:"quarantines_removed,omitempty"`
	QuarantineBytesRemoved int64                      `json:"quarantine_bytes_removed,omitempty"`
	RemovedQuarantines     []RetentionQuarantineEntry `json:"removed_quarantines,omitempty"`
}

func DefaultRetentionPruneConfig() RetentionPruneConfig {
	return RetentionPruneConfig{
		Mode:             RetentionPruneModeOff,
		StateDir:         "/var/lib/go-choir/vm-state",
		OrphanMinAge:     6 * time.Hour,
		EphemeralMinAge:  24 * time.Hour,
		MaxDeletes:       25,
		MaxBytes:         20 * 1024 * 1024 * 1024,
		TestIdleMode:     RetentionPruneModeOff,
		TestIdleMinAge:   defaultRetentionTestIdleMinAge,
		QuarantineMode:   RetentionPruneModeOff,
		QuarantineMinAge: defaultRetentionQuarantineMinAge,
	}
}

func normalizeRetentionPruneConfig(cfg RetentionPruneConfig) RetentionPruneConfig {
	cfg.Mode = strings.TrimSpace(strings.ToLower(cfg.Mode))
	switch cfg.Mode {
	case "", RetentionPruneModeOff:
		cfg.Mode = RetentionPruneModeOff
	case "dryrun", "observe", "observation", RetentionPruneModeDryRun:
		cfg.Mode = RetentionPruneModeDryRun
	case "reclaim", "enforce", "prune", RetentionPruneModeActive:
		cfg.Mode = RetentionPruneModeActive
	default:
		cfg.Mode = RetentionPruneModeOff
	}
	if strings.TrimSpace(cfg.StateDir) == "" {
		cfg.StateDir = "/var/lib/go-choir/vm-state"
	}
	if cfg.OrphanMinAge <= 0 {
		cfg.OrphanMinAge = 6 * time.Hour
	}
	if cfg.EphemeralMinAge <= 0 {
		cfg.EphemeralMinAge = 24 * time.Hour
	}
	if cfg.MaxDeletes <= 0 {
		cfg.MaxDeletes = 25
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = 20 * 1024 * 1024 * 1024
	}
	cfg.EphemeralEmailDomains = normalizeLowerList(cfg.EphemeralEmailDomains)
	cfg.EphemeralUserIDPrefixes = normalizeLowerList(cfg.EphemeralUserIDPrefixes)
	cfg.TestIdleMode = normalizeRetentionRuleMode(cfg.TestIdleMode)
	cfg.QuarantineMode = normalizeRetentionRuleMode(cfg.QuarantineMode)
	if cfg.TestIdleMinAge <= 0 {
		cfg.TestIdleMinAge = defaultRetentionTestIdleMinAge
	}
	if cfg.QuarantineMinAge <= 0 {
		cfg.QuarantineMinAge = defaultRetentionQuarantineMinAge
	}
	// Never faster than the ephemeral rule's day (review round 2, F9).
	if cfg.TestIdleMinAge < retentionMinAgeFloor {
		cfg.TestIdleMinAge = retentionMinAgeFloor
	}
	if cfg.QuarantineMinAge < retentionMinAgeFloor {
		cfg.QuarantineMinAge = retentionMinAgeFloor
	}
	cfg.SelectorNamed = normalizeIDList(cfg.SelectorNamed)
	return cfg
}

// normalizeIDList trims, dedups and sorts ids; ids keep their case.
func normalizeIDList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, raw := range values {
		v := strings.TrimSpace(raw)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func normalizeLowerList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, raw := range values {
		v := strings.ToLower(strings.TrimSpace(raw))
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func retentionPruneConfigSummary(cfg RetentionPruneConfig) RetentionPruneConfigSummary {
	return RetentionPruneConfigSummary{
		Mode:                    cfg.Mode,
		StateDir:                cfg.StateDir,
		AuthDBPathConfigured:    strings.TrimSpace(cfg.AuthDBPath) != "",
		EphemeralEmailDomains:   append([]string(nil), cfg.EphemeralEmailDomains...),
		EphemeralUserIDPrefixes: append([]string(nil), cfg.EphemeralUserIDPrefixes...),
		OrphanMinAgeSeconds:     int64(cfg.OrphanMinAge.Seconds()),
		EphemeralMinAgeSeconds:  int64(cfg.EphemeralMinAge.Seconds()),
		MaxDeletes:              cfg.MaxDeletes,
		MaxBytes:                cfg.MaxBytes,
		TestIdleMode:            cfg.TestIdleMode,
		TestIdleMinAgeSeconds:   int64(cfg.TestIdleMinAge.Seconds()),
		QuarantineMode:          cfg.QuarantineMode,
		QuarantineMinAgeSeconds: int64(cfg.QuarantineMinAge.Seconds()),
		SelectorNamed:           append([]string(nil), cfg.SelectorNamed...),
	}
}

func (r *OwnershipRegistry) RetentionPrunePlan() RetentionPrunePlan {
	cfg, ownerships, emails, detached := r.retentionSnapshot()
	return retentionPrunePlanFromSnapshot(cfg, r.warmnessPolicySnapshot(), ownerships, emails, detached, r.retentionNow())
}

func (r *OwnershipRegistry) RetentionShadowPlan() RetentionPrunePlan {
	cfg, ownerships, emails, detached, ok := r.retentionShadowSnapshot()
	if !ok {
		cfg := normalizeRetentionPruneConfig(RetentionPruneConfig{Mode: RetentionPruneModeOff})
		return RetentionPrunePlan{
			Mode:     cfg.Mode,
			Decision: "disabled",
			Reason:   "retention shadow plan is disabled",
			Config:   retentionPruneConfigSummary(cfg),
		}
	}
	return retentionPrunePlanFromSnapshot(cfg, r.warmnessPolicySnapshot(), ownerships, emails, detached, r.retentionNow())
}

func retentionPrunePlanFromSnapshot(cfg RetentionPruneConfig, warmness WarmnessPolicyConfig, ownerships []*VMOwnership, emails map[string]string, detached map[string]RetentionProtectedState, now time.Time) RetentionPrunePlan {
	plan := RetentionPrunePlan{
		Mode:     cfg.Mode,
		Decision: "disabled",
		Reason:   "retention prune is disabled",
		Config:   retentionPruneConfigSummary(cfg),
	}
	if cfg.Mode == RetentionPruneModeOff {
		return plan
	}
	loaded, warnings := loadRetentionEmailsFromAuthDB(cfg.AuthDBPath)
	for k, v := range loaded {
		emails[k] = v
	}
	plan.Warnings = append(plan.Warnings, warnings...)
	ownedVMs := make(map[string]bool, len(ownerships)+len(detached))
	for _, own := range ownerships {
		if own != nil && strings.TrimSpace(own.VMID) != "" {
			ownedVMs[own.VMID] = true
		}
	}
	for vmID, protected := range detached {
		ownedVMs[vmID] = true
		plan.ProtectedDetachedLegacy = append(plan.ProtectedDetachedLegacy, protected)
	}
	sort.Slice(plan.ProtectedDetachedLegacy, func(i, j int) bool {
		return plan.ProtectedDetachedLegacy[i].VMID < plan.ProtectedDetachedLegacy[j].VMID
	})
	plan.Inventory.Ownerships = len(ownerships)
	plan.Inventory.DetachedRollbackStateDirs = len(detached)
	candidates := make([]RetentionPruneCandidate, 0)
	orphans, kept, stateDirCount, orphanCount, orphanWarnings := retentionOrphanCandidates(cfg, ownedVMs, now)
	plan.Inventory.StateDirs = stateDirCount
	plan.Inventory.OrphanStateDirs = orphanCount
	plan.Warnings = append(plan.Warnings, orphanWarnings...)
	candidates = append(candidates, orphans...)
	plan.KeptStateDirs = append(plan.KeptStateDirs, kept...)
	for _, own := range ownerships {
		email := strings.TrimSpace(emails[own.UserID])
		if retentionOwnershipEphemeral(own, email, cfg) {
			plan.Inventory.EphemeralOwnerships++
		}
		if !retentionOwnershipReclaimable(own, email, cfg, now) || retentionOwnershipProtected(own, warmness) {
			continue
		}
		size := vmStateDirUsageBytes(cfg.StateDir, own.VMID)
		// R3-3: the ephemeral rule destroys the whole dir; never one holding
		// recovery state the age-out rule would keep.
		if reasons := vmDirRecoveryKeepReasons(cfg.StateDir, own.VMID, now, cfg.QuarantineMinAge); len(reasons) > 0 {
			plan.KeptStateDirs = append(plan.KeptStateDirs, RetentionKeptStateDir{
				VMID: own.VMID, UserID: own.UserID, Rule: "ephemeral_test_primary",
				SizeBytes: size, Reasons: reasons, Release: retentionReleaseEphemeral,
			})
			continue
		}
		candidates = append(candidates, RetentionPruneCandidate{
			VMID:           own.VMID,
			UserID:         own.UserID,
			EmailDomain:    emailDomain(email),
			Kind:           own.Kind,
			State:          own.State,
			DesktopID:      own.DesktopID,
			LastActiveAt:   own.LastActiveAt.UTC().Format(time.RFC3339Nano),
			AgeSeconds:     int64(now.Sub(own.LastActiveAt).Seconds()),
			SizeBytes:      size,
			Reason:         "ephemeral_test_primary",
			ProposedAction: "destroy_vm_state_and_remove_ownership",
		})
	}
	if cfg.TestIdleMode != RetentionPruneModeOff {
		planned := make(map[string]bool, len(candidates))
		for _, c := range candidates {
			planned[c.VMID] = true
		}
		byVM := make(map[string]*VMOwnership, len(ownerships))
		for _, own := range ownerships {
			byVM[own.VMID] = own
		}
		plan.TestClass = retentionTestClassReport(cfg, warmness, ownerships, now)
		for _, entry := range plan.TestClass {
			plan.Inventory.TestClassComputers++
			plan.Inventory.TestClassBytes += entry.SizeBytes
			if entry.Verdict == retentionVerdictKeep {
				continue
			}
			plan.Inventory.TestClassWouldRemove++
			plan.Inventory.TestClassWouldRemoveBytes += entry.SizeBytes
			if entry.Verdict != retentionVerdictWillRemove || planned[entry.VMID] {
				continue
			}
			own := byVM[entry.VMID]
			candidates = append(candidates, RetentionPruneCandidate{
				VMID:           entry.VMID,
				UserID:         entry.UserID,
				EmailDomain:    emailDomain(strings.TrimSpace(emails[own.UserID])),
				Kind:           own.Kind,
				State:          entry.State,
				DesktopID:      entry.DesktopID,
				LastActiveAt:   entry.LastActiveAt,
				AgeSeconds:     entry.IdleSeconds,
				SizeBytes:      entry.SizeBytes,
				Reason:         retentionReasonTestClassIdle,
				ProposedAction: "destroy_vm_state_and_remove_ownership",
			})
		}
	}
	quarantineRemovals := 0
	if cfg.QuarantineMode != RetentionPruneModeOff {
		entries, warnings := retentionQuarantineReport(cfg, warmness, ownerships, now)
		plan.Quarantines = entries
		plan.Warnings = append(plan.Warnings, warnings...)
		for _, entry := range entries {
			plan.Inventory.Quarantines++
			plan.Inventory.QuarantineBytes += entry.SizeBytes
			if entry.Verdict == retentionVerdictKeep {
				continue
			}
			plan.Inventory.QuarantineWouldRemove++
			plan.Inventory.QuarantineWouldRemoveBytes += entry.SizeBytes
			if entry.Verdict == retentionVerdictWillRemove {
				quarantineRemovals++
			}
		}
	}
	for _, k := range plan.KeptStateDirs {
		plan.Inventory.KeptStateDirs++
		plan.Inventory.KeptStateDirBytes += k.SizeBytes
	}
	sortRetentionCandidates(candidates)
	plan.Inventory.Candidates = len(candidates)
	for _, candidate := range candidates {
		plan.Inventory.CandidateBytes += candidate.SizeBytes
	}
	limited := limitRetentionCandidates(candidates, cfg)
	plan.Candidates = limited
	plan.Inventory.ProjectedDeleteCount = len(limited)
	for _, candidate := range limited {
		plan.Inventory.ProjectedDeleteBytes += candidate.SizeBytes
	}
	if len(limited) == 0 && quarantineRemovals == 0 {
		plan.Decision = "observe"
		plan.Reason = "no VM state matched the retention prune policy"
		return plan
	}
	if cfg.Mode == RetentionPruneModeDryRun {
		plan.Decision = "would_prune"
		plan.Reason = "matching disposable VM state found; dry-run mode will not delete it"
		return plan
	}
	plan.Decision = "will_prune"
	plan.Reason = "matching disposable VM state found"
	return plan
}

func (r *OwnershipRegistry) retentionSnapshot() (RetentionPruneConfig, []*VMOwnership, map[string]string, map[string]RetentionProtectedState) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cfg := normalizeRetentionPruneConfig(r.retentionPrune)
	ownerships := make([]*VMOwnership, 0, len(r.ownerships))
	for _, own := range r.ownerships {
		ownerships = append(ownerships, cloneOwnership(own))
	}
	emails := make(map[string]string, len(r.retentionUserEmails))
	for userID, email := range r.retentionUserEmails {
		emails[userID] = email
	}
	detached := map[string]RetentionProtectedState{}
	return cfg, ownerships, emails, detached
}

func (r *OwnershipRegistry) retentionShadowSnapshot() (RetentionPruneConfig, []*VMOwnership, map[string]string, map[string]RetentionProtectedState, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !r.retentionShadowPruneEnabled {
		return RetentionPruneConfig{}, nil, nil, nil, false
	}
	cfg := normalizeRetentionPruneConfig(r.retentionShadowPrune)
	if cfg.Mode != RetentionPruneModeOff {
		cfg.Mode = RetentionPruneModeDryRun
	}
	ownerships := make([]*VMOwnership, 0, len(r.ownerships))
	for _, own := range r.ownerships {
		ownerships = append(ownerships, cloneOwnership(own))
	}
	emails := make(map[string]string, len(r.retentionUserEmails))
	for userID, email := range r.retentionUserEmails {
		emails[userID] = email
	}
	detached := map[string]RetentionProtectedState{}
	return cfg, ownerships, emails, detached, true
}

func retentionOrphanCandidates(cfg RetentionPruneConfig, ownedVMs map[string]bool, now time.Time) ([]RetentionPruneCandidate, []RetentionKeptStateDir, int, int, []string) {
	root := filepath.Clean(cfg.StateDir)
	if root == "." || root == string(os.PathSeparator) {
		return nil, nil, 0, 0, []string{"retention prune refused unsafe state dir"}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, 0, 0, nil
		}
		return nil, nil, 0, 0, []string{fmt.Sprintf("read state dir: %v", err)}
	}
	var candidates []RetentionPruneCandidate
	var kept []RetentionKeptStateDir
	stateDirCount := 0
	orphanCount := 0
	for _, entry := range entries {
		if entry == nil || !entry.IsDir() {
			continue
		}
		vmID := entry.Name()
		if strings.HasPrefix(vmID, "vm-") || strings.HasPrefix(vmID, "candidate-") {
			stateDirCount++
		}
		// vmctl-managed state dirs are named vm-* (generated) or candidate-*
		// (named candidate/fleet computers). Anything else under vm-state is
		// host bookkeeping (ownerships.json, backups), not a guest disk.
		if (!strings.HasPrefix(vmID, "vm-") && !strings.HasPrefix(vmID, "candidate-")) || ownedVMs[vmID] {
			continue
		}
		orphanCount++
		// R2-8: a dir holding recovery state (a quarantine may be the only
		// copy of a computer's data) is never an orphan to destroy.
		if reasons := retentionOrphanKeepReasons(root, vmID); len(reasons) > 0 {
			kept = append(kept, RetentionKeptStateDir{
				VMID: vmID, Rule: "orphan_state_dir", SizeBytes: vmStateDirUsageBytes(root, vmID),
				Reasons: reasons, Release: retentionReleaseOrphan,
			})
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		age := now.Sub(info.ModTime())
		if age < cfg.OrphanMinAge {
			continue
		}
		candidates = append(candidates, RetentionPruneCandidate{
			VMID:           vmID,
			AgeSeconds:     int64(age.Seconds()),
			SizeBytes:      vmStateDirUsageBytes(root, vmID),
			Reason:         "orphan_state_dir",
			ProposedAction: "destroy_unowned_vm_state",
		})
	}
	return candidates, kept, stateDirCount, orphanCount, nil
}

func retentionOwnershipEphemeral(own *VMOwnership, email string, cfg RetentionPruneConfig) bool {
	if own == nil {
		return false
	}
	userID := strings.ToLower(strings.TrimSpace(own.UserID))
	for _, prefix := range cfg.EphemeralUserIDPrefixes {
		if strings.HasPrefix(userID, prefix) {
			return true
		}
	}
	domain := emailDomain(email)
	if domain == "" {
		return false
	}
	for _, allowed := range cfg.EphemeralEmailDomains {
		if domain == allowed {
			return true
		}
	}
	return false
}

func retentionOwnershipReclaimable(own *VMOwnership, email string, cfg RetentionPruneConfig, now time.Time) bool {
	if own == nil || strings.TrimSpace(own.VMID) == "" || own.LastActiveAt.IsZero() {
		return false
	}
	if own.IsHeld() {
		// A held computer is never reclaimable/prunable.
		return false
	}
	if !retentionOwnershipEphemeral(own, email, cfg) {
		return false
	}
	if own.Kind != VMKindInteractive || own.DesktopID != PrimaryDesktopID {
		return false
	}
	switch own.State {
	case VMStateStopped, VMStateHibernated, VMStateFailed:
	default:
		return false
	}
	return now.Sub(own.LastActiveAt) >= cfg.EphemeralMinAge
}

// retentionOwnershipProtected reports whether a computer's warmness class
// keeps it out of retention entirely. An account on an ephemeral domain says
// nothing about whether the computer is disposable: the paper runs under such
// an account and is critical_protected
// (docs/problems/retention-prune-would-destroy-the-paper-2026-10-16.md).
func retentionOwnershipProtected(own *VMOwnership, warmness WarmnessPolicyConfig) bool {
	return warmnessClassProtected(warmnessClassForOwnership(own, warmness))
}

func sortRetentionCandidates(candidates []RetentionPruneCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if left.SizeBytes != right.SizeBytes {
			return left.SizeBytes > right.SizeBytes
		}
		leftRank := retentionReasonRank(left.Reason)
		rightRank := retentionReasonRank(right.Reason)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		leftState := retentionStateRank(left.State)
		rightState := retentionStateRank(right.State)
		if leftState != rightState {
			return leftState < rightState
		}
		if left.AgeSeconds != right.AgeSeconds {
			return left.AgeSeconds > right.AgeSeconds
		}
		return left.VMID < right.VMID
	})
}

func retentionReasonRank(reason string) int {
	switch reason {
	case "orphan_state_dir":
		return 0
	case "ephemeral_test_primary":
		return 1
	case retentionReasonTestClassIdle:
		return 2
	default:
		return 9
	}
}

func retentionStateRank(state VMState) int {
	switch state {
	case VMStateFailed:
		return 0
	case VMStateStopped:
		return 1
	case VMStateHibernated:
		return 2
	default:
		return 9
	}
}

func limitRetentionCandidates(candidates []RetentionPruneCandidate, cfg RetentionPruneConfig) []RetentionPruneCandidate {
	limited := make([]RetentionPruneCandidate, 0, len(candidates))
	var bytes int64
	for _, candidate := range candidates {
		if cfg.MaxDeletes > 0 && len(limited) >= cfg.MaxDeletes {
			break
		}
		if cfg.MaxBytes > 0 && len(limited) > 0 && bytes+candidate.SizeBytes > cfg.MaxBytes {
			break
		}
		limited = append(limited, candidate)
		bytes += candidate.SizeBytes
	}
	return limited
}

func (r *OwnershipRegistry) PruneRetention(ctx context.Context, guard ComputerVersionRouteGuard) RetentionPruneResult {
	before := r.RetentionPrunePlan()
	result := RetentionPruneResult{
		Status:     "disabled",
		PlanBefore: before,
	}
	if before.Mode != RetentionPruneModeActive {
		if before.Mode == RetentionPruneModeDryRun {
			result.Status = "dry_run"
		}
		result.PlanAfter = before
		return result
	}
	result.Status = "ok"
	for _, candidate := range before.Candidates {
		// Orphan state dirs have no ownership record, so there is no D-ROUTE
		// join to authorize against — requiring one would make orphans
		// unprunable forever (empty user/desktop always fails the guard).
		// Their safety is the directory-name match + OrphanMinAge.
		owned := candidate.UserID != "" || candidate.DesktopID != ""
		if owned && !authorizeLifecycleRoute(ctx, guard, candidate.UserID, candidate.DesktopID) {
			continue
		}
		if r.destroyRetentionCandidate(candidate) {
			result.Deleted++
			result.BytesDeleted += candidate.SizeBytes
			result.DeletedVMs = append(result.DeletedVMs, candidate)
		}
	}
	r.ageOutRetentionQuarantines(before, &result)
	result.PlanAfter = r.RetentionPrunePlan()
	return result
}

func (r *OwnershipRegistry) destroyRetentionCandidate(candidate RetentionPruneCandidate) bool {
	if strings.TrimSpace(candidate.VMID) == "" {
		return false
	}
	if candidate.Reason == retentionReasonTestClassIdle {
		return r.destroyTestIdleCandidate(candidate)
	}
	r.mu.RLock()
	mgr := r.vmManager
	cfg := normalizeRetentionPruneConfig(r.retentionPrune)
	current := cloneOwnership(r.vmByID[candidate.VMID])
	detached := false
	email := strings.TrimSpace(r.retentionUserEmails[candidate.UserID])
	r.mu.RUnlock()
	if mgr == nil || detached {
		return false
	}
	if current == nil {
		if candidate.Reason != "orphan_state_dir" {
			return false
		}
		r.mu.RLock()
		defer r.mu.RUnlock()
		// R2-8 and R3-1: re-read under the lock; recovery state that
		// appeared after the plan keeps the dir.
		if r.vmByID[candidate.VMID] != nil || len(retentionOrphanKeepReasons(cfg.StateDir, candidate.VMID)) > 0 {
			return false
		}
		if err := mgr.DestroyVMState(candidate.VMID); err != nil {
			log.Printf("vmctl: retention prune skipped %s: %v", candidate.VMID, err)
			return false
		}
		return true
	}
	loaded, warnings := loadRetentionEmailsFromAuthDB(cfg.AuthDBPath)
	for _, warning := range warnings {
		log.Printf("vmctl: retention prune warning: %s", warning)
	}
	if loadedEmail := strings.TrimSpace(loaded[current.UserID]); loadedEmail != "" {
		email = loadedEmail
	}
	// R3-3: every check is re-read under the registry lock, the dir's
	// recovery state included, and the computer's lifecycle is claimed, so
	// a visit, resume or refresh cannot start it while its state goes.
	now := r.retentionNow()
	r.mu.Lock()
	cur := r.vmByID[candidate.VMID]
	warmness := normalizeWarmnessPolicyConfig(r.warmnessPolicy)
	if cur == nil || !retentionOwnershipReclaimable(cur, email, cfg, now) || retentionOwnershipProtected(cur, warmness) ||
		len(vmDirRecoveryKeepReasons(cfg.StateDir, cur.VMID, now, cfg.QuarantineMinAge)) > 0 {
		r.mu.Unlock()
		return false
	}
	key := ownershipKey(cur.UserID, cur.DesktopID)
	if r.lifecycleConflictLocked(key) != nil {
		r.mu.Unlock()
		return false
	}
	r.lifecycle[key] = lifecycleRetention
	r.mu.Unlock()
	defer r.releaseLifecycle(key, lifecycleRetention)
	if err := mgr.DestroyVMState(candidate.VMID); err != nil {
		log.Printf("vmctl: retention prune skipped %s: %v", candidate.VMID, err)
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	current = r.vmByID[candidate.VMID]
	if current == nil {
		return true
	}
	delete(r.ownerships, ownershipKey(current.UserID, current.DesktopID))
	delete(r.vmByID, current.VMID)
	r.saveLocked()
	return true
}

func loadRetentionEmailsFromAuthDB(path string) (map[string]string, []string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, []string{fmt.Sprintf("open auth db: %v", err)}
	}
	defer db.Close()
	rows, err := db.Query(`SELECT id, email FROM users`)
	if err != nil {
		return nil, []string{fmt.Sprintf("query auth users: %v", err)}
	}
	defer rows.Close()
	emails := map[string]string{}
	for rows.Next() {
		var id, email string
		if err := rows.Scan(&id, &email); err != nil {
			return emails, []string{fmt.Sprintf("scan auth user: %v", err)}
		}
		emails[strings.TrimSpace(id)] = strings.TrimSpace(email)
	}
	if err := rows.Err(); err != nil {
		return emails, []string{fmt.Sprintf("iterate auth users: %v", err)}
	}
	return emails, nil
}

func emailDomain(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return ""
	}
	return strings.TrimSpace(email[at+1:])
}

// retentionOrphanKeepReasons is why the orphan rule keeps an ownerless
// dir: anything a cold recovery left (a quarantined or staging data image,
// a recovery journal) or an unreadable dir. A journal is never deleted, so a
// dir that ever cold-recovered stays until an owner act
// (retentionReleaseOrphan).
func retentionOrphanKeepReasons(stateDir, vmID string) []string {
	root := filepath.Clean(stateDir)
	if root == "." || root == string(os.PathSeparator) || vmID == "" || filepath.Base(vmID) != vmID {
		return []string{"unreadable"}
	}
	files, err := os.ReadDir(filepath.Join(root, vmID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return []string{"unreadable"}
	}
	seen := map[string]bool{}
	var reasons []string
	add := func(reason string) {
		if !seen[reason] {
			seen[reason] = true
			reasons = append(reasons, reason)
		}
	}
	for _, file := range files {
		name := file.Name()
		switch {
		case strings.HasPrefix(name, "data.img.quarantine-"):
			add("holds_quarantine")
		case strings.HasPrefix(name, "data.img.staging-"):
			add("holds_staging_image")
		case strings.HasPrefix(name, "rec-") && strings.HasSuffix(name, ".journal"):
			add("holds_recovery_journal")
		}
	}
	return reasons
}
