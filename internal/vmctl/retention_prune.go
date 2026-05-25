package vmctl

import (
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
}

type RetentionPruneInventory struct {
	Ownerships           int   `json:"ownerships"`
	StateDirs            int   `json:"state_dirs"`
	OrphanStateDirs      int   `json:"orphan_state_dirs"`
	EphemeralOwnerships  int   `json:"ephemeral_ownerships"`
	Candidates           int   `json:"candidates"`
	CandidateBytes       int64 `json:"candidate_bytes"`
	ProjectedDeleteCount int   `json:"projected_delete_count"`
	ProjectedDeleteBytes int64 `json:"projected_delete_bytes"`
}

type RetentionPruneCandidate struct {
	VMID           string  `json:"vm_id"`
	UserID         string  `json:"user_id,omitempty"`
	EmailDomain    string  `json:"email_domain,omitempty"`
	Kind           VMKind  `json:"kind,omitempty"`
	State          VMState `json:"state,omitempty"`
	DesktopID      string  `json:"desktop_id,omitempty"`
	Published      bool    `json:"published,omitempty"`
	LastActiveAt   string  `json:"last_active_at,omitempty"`
	AgeSeconds     int64   `json:"age_seconds"`
	SizeBytes      int64   `json:"size_bytes"`
	Reason         string  `json:"reason"`
	ProposedAction string  `json:"proposed_action"`
}

type RetentionPrunePlan struct {
	Mode       string                      `json:"mode"`
	Decision   string                      `json:"decision"`
	Reason     string                      `json:"reason"`
	Inventory  RetentionPruneInventory     `json:"inventory"`
	Candidates []RetentionPruneCandidate   `json:"candidates,omitempty"`
	Config     RetentionPruneConfigSummary `json:"config"`
	Warnings   []string                    `json:"warnings,omitempty"`
}

type RetentionPruneResult struct {
	Status       string                    `json:"status"`
	Deleted      int                       `json:"deleted"`
	BytesDeleted int64                     `json:"bytes_deleted"`
	DeletedVMs   []RetentionPruneCandidate `json:"deleted_vms,omitempty"`
	PlanBefore   RetentionPrunePlan        `json:"plan_before"`
	PlanAfter    RetentionPrunePlan        `json:"plan_after"`
	Warnings     []string                  `json:"warnings,omitempty"`
}

func DefaultRetentionPruneConfig() RetentionPruneConfig {
	return RetentionPruneConfig{
		Mode:            RetentionPruneModeOff,
		StateDir:        "/var/lib/go-choir/vm-state",
		OrphanMinAge:    6 * time.Hour,
		EphemeralMinAge: 24 * time.Hour,
		MaxDeletes:      25,
		MaxBytes:        20 * 1024 * 1024 * 1024,
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
	return cfg
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
	}
}

func (r *OwnershipRegistry) RetentionPrunePlan() RetentionPrunePlan {
	cfg, ownerships, emails := r.retentionSnapshot()
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
	ownedVMs := make(map[string]bool, len(ownerships))
	for _, own := range ownerships {
		if own != nil && strings.TrimSpace(own.VMID) != "" {
			ownedVMs[own.VMID] = true
		}
	}
	plan.Inventory.Ownerships = len(ownerships)
	now := time.Now()
	candidates := make([]RetentionPruneCandidate, 0)
	orphans, stateDirCount, orphanCount, orphanWarnings := retentionOrphanCandidates(cfg, ownedVMs, now)
	plan.Inventory.StateDirs = stateDirCount
	plan.Inventory.OrphanStateDirs = orphanCount
	plan.Warnings = append(plan.Warnings, orphanWarnings...)
	candidates = append(candidates, orphans...)
	for _, own := range ownerships {
		email := strings.TrimSpace(emails[own.UserID])
		if retentionOwnershipEphemeral(own, email, cfg) {
			plan.Inventory.EphemeralOwnerships++
		}
		if !retentionOwnershipReclaimable(own, email, cfg, now) {
			continue
		}
		size := vmStateDirUsageBytes(cfg.StateDir, own.VMID)
		candidates = append(candidates, RetentionPruneCandidate{
			VMID:           own.VMID,
			UserID:         own.UserID,
			EmailDomain:    emailDomain(email),
			Kind:           own.Kind,
			State:          own.State,
			DesktopID:      own.DesktopID,
			Published:      own.Published,
			LastActiveAt:   own.LastActiveAt.UTC().Format(time.RFC3339Nano),
			AgeSeconds:     int64(now.Sub(own.LastActiveAt).Seconds()),
			SizeBytes:      size,
			Reason:         "ephemeral_test_primary",
			ProposedAction: "destroy_vm_state_and_remove_ownership",
		})
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
	if len(limited) == 0 {
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

func (r *OwnershipRegistry) retentionSnapshot() (RetentionPruneConfig, []*VMOwnership, map[string]string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cfg := normalizeRetentionPruneConfig(r.retentionPrune)
	ownerships := make([]*VMOwnership, 0, len(r.ownerships)+len(r.workerVMs))
	for _, own := range r.ownerships {
		ownerships = append(ownerships, cloneOwnership(own))
	}
	for _, own := range r.workerVMs {
		ownerships = append(ownerships, cloneOwnership(own))
	}
	emails := make(map[string]string, len(r.retentionUserEmails))
	for userID, email := range r.retentionUserEmails {
		emails[userID] = email
	}
	return cfg, ownerships, emails
}

func retentionOrphanCandidates(cfg RetentionPruneConfig, ownedVMs map[string]bool, now time.Time) ([]RetentionPruneCandidate, int, int, []string) {
	root := filepath.Clean(cfg.StateDir)
	if root == "." || root == string(os.PathSeparator) {
		return nil, 0, 0, []string{"retention prune refused unsafe state dir"}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, 0, nil
		}
		return nil, 0, 0, []string{fmt.Sprintf("read state dir: %v", err)}
	}
	var candidates []RetentionPruneCandidate
	stateDirCount := 0
	orphanCount := 0
	for _, entry := range entries {
		if entry == nil || !entry.IsDir() {
			continue
		}
		stateDirCount++
		vmID := entry.Name()
		if !strings.HasPrefix(vmID, "vm-") || ownedVMs[vmID] {
			continue
		}
		orphanCount++
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
	return candidates, stateDirCount, orphanCount, nil
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
	if !retentionOwnershipEphemeral(own, email, cfg) {
		return false
	}
	if own.Kind != VMKindInteractive || own.DesktopID != PrimaryDesktopID || !own.Published {
		return false
	}
	switch own.State {
	case VMStateStopped, VMStateHibernated, VMStateFailed:
	default:
		return false
	}
	return now.Sub(own.LastActiveAt) >= cfg.EphemeralMinAge
}

func sortRetentionCandidates(candidates []RetentionPruneCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
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
		if left.SizeBytes != right.SizeBytes {
			return left.SizeBytes > right.SizeBytes
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

func (r *OwnershipRegistry) PruneRetention() RetentionPruneResult {
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
		if r.destroyRetentionCandidate(candidate) {
			result.Deleted++
			result.BytesDeleted += candidate.SizeBytes
			result.DeletedVMs = append(result.DeletedVMs, candidate)
		}
	}
	result.PlanAfter = r.RetentionPrunePlan()
	return result
}

func (r *OwnershipRegistry) destroyRetentionCandidate(candidate RetentionPruneCandidate) bool {
	if strings.TrimSpace(candidate.VMID) == "" {
		return false
	}
	r.mu.RLock()
	mgr := r.vmManager
	cfg := normalizeRetentionPruneConfig(r.retentionPrune)
	current := cloneOwnership(r.vmByID[candidate.VMID])
	email := strings.TrimSpace(r.retentionUserEmails[candidate.UserID])
	r.mu.RUnlock()
	if mgr == nil {
		return false
	}
	if current != nil {
		loaded, warnings := loadRetentionEmailsFromAuthDB(cfg.AuthDBPath)
		for _, warning := range warnings {
			log.Printf("vmctl: retention prune warning: %s", warning)
		}
		if loadedEmail := strings.TrimSpace(loaded[current.UserID]); loadedEmail != "" {
			email = loadedEmail
		}
		if !retentionOwnershipReclaimable(cloneOwnership(current), email, cfg, time.Now()) {
			return false
		}
	} else if candidate.Reason != "orphan_state_dir" {
		return false
	}
	if err := mgr.DestroyVMState(candidate.VMID); err != nil {
		log.Printf("vmctl: retention prune skipped %s: %v", candidate.VMID, err)
		return false
	}
	if current == nil {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	current = r.vmByID[candidate.VMID]
	if current == nil {
		return true
	}
	if current.Kind == VMKindWorker {
		delete(r.workerVMs, strings.TrimSpace(current.WorkerID))
	} else {
		delete(r.ownerships, ownershipKey(current.UserID, current.DesktopID))
	}
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
