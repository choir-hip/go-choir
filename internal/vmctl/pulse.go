package vmctl

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	_ "modernc.org/sqlite"
)

const (
	PulseAccountReal             = "real"
	PulseAccountCodexAgenticTest = "codex_agentic_test"
	PulseAccountProtectedTest    = "protected_test"
	PulseAccountInternal         = "internal"
	PulseAccountUnknown          = "unknown"
)

var pulseProtectedTestEmails = map[string]bool{
	"a@b.com": true,
	"b@c.com": true,
}

type PulseSummary struct {
	Status      string                  `json:"status"`
	GeneratedAt string                  `json:"generated_at"`
	Privacy     PulsePrivacyStatement   `json:"privacy"`
	Classifier  PulseClassifierSummary  `json:"classifier"`
	Accounts    PulseAccountSummary     `json:"accounts"`
	Activity    PulseActivitySummary    `json:"activity"`
	Computers   PulseComputerSummary    `json:"computers"`
	Capacity    PulseCapacitySummary    `json:"capacity"`
	Storage     PulseStorageSummary     `json:"storage"`
	Reliability PulseReliabilitySummary `json:"reliability"`
	Freshness   PulseFreshnessSummary   `json:"freshness"`
	Warnings    []string                `json:"warnings,omitempty"`
}

type PulsePrivacyStatement struct {
	Surface              string   `json:"surface"`
	DataMode             string   `json:"data_mode"`
	ExcludedData         []string `json:"excluded_data"`
	NoPrivateSuperset    bool     `json:"no_private_superset"`
	NoRowLevelAnalytics  bool     `json:"no_row_level_analytics"`
	NoUserIdentityOutput bool     `json:"no_user_identity_output"`
}

type PulseClassifierSummary struct {
	Version             string   `json:"version"`
	RealClass           string   `json:"real_class"`
	CodexDomains        []string `json:"codex_domains"`
	ProtectedTestCount  int      `json:"protected_test_count"`
	ProtectedTestPolicy string   `json:"protected_test_policy"`
	UnknownPolicy       string   `json:"unknown_policy"`
}

type PulseAccountSummary struct {
	Total             int            `json:"total"`
	ByClass           map[string]int `json:"by_class"`
	NewRealLast24h    int            `json:"new_real_last_24h"`
	NewRealLast7d     int            `json:"new_real_last_7d"`
	NewRealLast30d    int            `json:"new_real_last_30d"`
	AuthDataAvailable bool           `json:"auth_data_available"`
}

type PulseActivitySummary struct {
	RealActiveLast24h int    `json:"real_active_last_24h"`
	RealActiveLast7d  int    `json:"real_active_last_7d"`
	RealActiveLast30d int    `json:"real_active_last_30d"`
	Source            string `json:"source"`
}

type PulseComputerSummary struct {
	TotalOwnerships       int                                  `json:"total_ownerships"`
	PrimaryComputers      map[string]PulseComputerClassSummary `json:"primary_computers_by_class"`
	RealPrimaryByState    map[string]int                       `json:"real_primary_by_state"`
	RealPrimaryTotal      int                                  `json:"real_primary_total"`
	RealPrimaryUsable     int                                  `json:"real_primary_usable"`
	CodexPrimaryByState   map[string]int                       `json:"codex_primary_by_state"`
	UnknownPrimaryByState map[string]int                       `json:"unknown_primary_by_state"`
}

type PulseComputerClassSummary struct {
	Total   int            `json:"total"`
	ByState map[string]int `json:"by_state"`
}

type PulseStorageSummary struct {
	VMStateBytesByClass          map[string]int64       `json:"vm_state_bytes_by_class"`
	VMStateBytesTotal            int64                  `json:"vm_state_bytes_total"`
	ManualRecoverySnapshotBytes  int64                  `json:"manual_recovery_snapshot_bytes"`
	ManualRecoverySnapshotCount  int                    `json:"manual_recovery_snapshot_count"`
	VMStateFilesystem            PulseFilesystemSummary `json:"vm_state_filesystem"`
	NixStoreFilesystem           PulseFilesystemSummary `json:"nix_store_filesystem"`
	ExpensiveNixStoreWalkOmitted bool                   `json:"expensive_nix_store_walk_omitted"`
}

type PulseFilesystemSummary struct {
	Path             string  `json:"path"`
	TotalBytes       int64   `json:"total_bytes"`
	AvailableBytes   int64   `json:"available_bytes"`
	UsedBytes        int64   `json:"used_bytes"`
	UsedPercent      float64 `json:"used_percent"`
	AvailablePercent float64 `json:"available_percent"`
}

type PulseReliabilitySummary struct {
	RealPrimaryFailed       int      `json:"real_primary_failed"`
	RealPrimaryBooting      int      `json:"real_primary_booting"`
	RealPrimaryInaccessible int      `json:"real_primary_inaccessible"`
	NotCollected            []string `json:"not_collected"`
}

type PulseFreshnessSummary struct {
	AuthDBReadAt      string `json:"auth_db_read_at,omitempty"`
	OwnershipsReadAt  string `json:"ownerships_read_at"`
	StorageSampledAt  string `json:"storage_sampled_at"`
	GeneratedBy       string `json:"generated_by"`
	DeployedCommitRef string `json:"deployed_commit_ref,omitempty"`
}

type pulseAccountRecord struct {
	UserID     string
	Email      string
	CreatedAt  time.Time
	HasPasskey bool
	// Operator is the operator's own account (CI, drills): not counted
	// against the sign-up cap.
	Operator bool
	// FirstPasskeyAt is when the first passkey was stored: the moment the
	// sign-up finished (CreatedAt is when it began).
	FirstPasskeyAt time.Time
}

// PulseCapacitySummary is what the owner decides capacity on (the owner,
// 2026-11-23: "we will monitor usage and turn off signup once we hit
// limits"). Aggregate numbers only: no address, id or per-user figure.
type PulseCapacitySummary struct {
	Memory       PulseCapacityMemory       `json:"memory"`
	Disk         PulseCapacityDisk         `json:"disk"`
	Computers    PulseCapacityComputers    `json:"computers"`
	Registration PulseCapacityRegistration `json:"registration"`
}

type PulseCapacityMemory struct {
	TotalBytes     uint64 `json:"total_bytes"`
	AvailableBytes uint64 `json:"available_bytes"`
	// FloorBytes is what a start must leave free: below it a computer is
	// refused with a plain "host memory" answer, never crashed.
	FloorBytes uint64 `json:"floor_bytes"`
}

type PulseCapacityDisk struct {
	AvailableBytes uint64 `json:"available_bytes"`
	FloorBytes     uint64 `json:"floor_bytes"`
}

type PulseCapacityComputers struct {
	Running    int `json:"running"`
	Hibernated int `json:"hibernated"`
	Other      int `json:"other"`
	// RunningGuestMemoryMiB is the guest memory promised to the running
	// computers: the ceiling their resident memory can grow to.
	RunningGuestMemoryMiB int `json:"running_guest_memory_mib"`
}

type PulseCapacityRegistration struct {
	// Available is false when the reading cannot be trusted: the auth
	// database could not be read, or the cap's configuration was malformed.
	// Open and Remaining mean nothing then.
	Available bool `json:"available"`
	// Cap is the owner's number of accounts that hold a passkey (the
	// operator's own aside); 0 is no cap.
	Cap int `json:"cap"`
	// Counted is the accounts holding a passkey that count against Cap.
	Counted   int  `json:"counted"`
	Remaining int  `json:"remaining"`
	Open      bool `json:"open"`
	// SignUpsLast24h are accounts that finished a passkey in the last day,
	// the operator's own aside.
	SignUpsLast24h int `json:"sign_ups_last_24h"`
}

// RegistrationCapConfig is the sign-up cap the auth service enforces
// (AUTH_REGISTRATION_CAP), read here from the same nix value so the owner
// sees the number the service uses. Unreadable: the value was malformed.
type RegistrationCapConfig struct {
	Cap        int
	Unreadable bool
	// DiskReserveBytes is the disk the cap keeps back: the deploy floor and
	// the paper's growth reserve (nix/node-b.nix, registrationDiskReserveGiB).
	// The capacity reading shows it as the disk floor, because the cap's
	// room is the free disk above it. Zero: the pressure floor is shown.
	DiskReserveBytes uint64
}

// SetRegistrationCapConfig records the cap for the capacity reading.
func (r *OwnershipRegistry) SetRegistrationCapConfig(cfg RegistrationCapConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registrationCap = cfg
}

func pulseCapacity(now time.Time, users []pulseAccountRecord, usersAvailable bool, ownerships []*VMOwnership, sample HostPressureSample, cfg PressureReclaimConfig, reg RegistrationCapConfig) PulseCapacitySummary {
	out := PulseCapacitySummary{
		Memory: PulseCapacityMemory{TotalBytes: sample.MemoryTotalBytes, AvailableBytes: sample.MemoryAvailableBytes, FloorBytes: sample.MemoryFloorBytes},
		Disk:   PulseCapacityDisk{AvailableBytes: sample.StateDirAvailableBytes, FloorBytes: cfg.MinStateDirAvailableBytes},
	}
	if reg.DiskReserveBytes > 0 {
		out.Disk.FloorBytes = reg.DiskReserveBytes
	}
	for _, own := range ownerships {
		if own == nil {
			continue
		}
		switch own.State {
		case VMStateActive:
			out.Computers.Running++
			_, mem := machineShapeForOwnership(own)
			out.Computers.RunningGuestMemoryMiB += mem
		case VMStateHibernated:
			out.Computers.Hibernated++
		default:
			out.Computers.Other++
		}
	}
	out.Registration.Available = usersAvailable && !reg.Unreadable
	if !out.Registration.Available {
		// No reading: no numbers and not open, never a guess from a partial list.
		return out
	}
	for _, u := range users {
		if !u.HasPasskey || u.Operator {
			continue
		}
		out.Registration.Counted++
		finished := u.FirstPasskeyAt
		if finished.IsZero() {
			finished = u.CreatedAt
		}
		if age := now.Sub(finished); !finished.IsZero() && age >= 0 && age <= 24*time.Hour {
			out.Registration.SignUpsLast24h++
		}
	}
	out.Registration.Cap = reg.Cap
	out.Registration.Open = reg.Cap <= 0 || out.Registration.Counted < reg.Cap
	if reg.Cap > 0 && out.Registration.Counted < reg.Cap {
		out.Registration.Remaining = reg.Cap - out.Registration.Counted
	}
	return out
}

func (r *OwnershipRegistry) PulseSummary() PulseSummary {
	now := time.Now().UTC()
	cfg, ownerships, cachedEmails, _ := r.retentionSnapshot()
	users, warnings := loadPulseAccountsFromAuthDB(cfg.AuthDBPath)
	authDataAvailable := len(warnings) == 0
	userByID := make(map[string]pulseAccountRecord, len(users)+len(cachedEmails))
	for _, user := range users {
		userByID[user.UserID] = user
	}
	for userID, email := range cachedEmails {
		if _, ok := userByID[userID]; !ok {
			userByID[userID] = pulseAccountRecord{UserID: userID, Email: email}
		}
	}
	summary := pulseSummaryFromSnapshot(now, cfg.StateDir, users, userByID, ownerships, authDataAvailable, warnings)
	r.mu.RLock()
	pcfg, sampler, regCap := normalizePressureReclaimConfig(r.pressureReclaim), r.pressureSampler, r.registrationCap
	r.mu.RUnlock()
	if sampler == nil {
		sampler = sampleHostPressure
	}
	summary.Capacity = pulseCapacity(now, users, authDataAvailable, ownerships, sampler(pcfg), pcfg, regCap)
	return summary
}

func ClassifyPulseAccount(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return PulseAccountUnknown
	}
	if pulseProtectedTestEmails[email] {
		return PulseAccountProtectedTest
	}
	switch emailDomain(email) {
	case "example.com", "example.test":
		return PulseAccountCodexAgenticTest
	case "choir.local":
		return PulseAccountInternal
	}
	return PulseAccountReal
}

func pulseSummaryFromSnapshot(now time.Time, stateDir string, users []pulseAccountRecord, userByID map[string]pulseAccountRecord, ownerships []*VMOwnership, authDataAvailable bool, warnings []string) PulseSummary {
	stateDir = strings.TrimSpace(stateDir)
	if stateDir == "" {
		stateDir = "/var/lib/go-choir/vm-state"
	}
	accountByClass := emptyPulseIntMap()
	for _, user := range users {
		accountByClass[ClassifyPulseAccount(user.Email)]++
	}
	accounts := PulseAccountSummary{
		Total:             len(users),
		ByClass:           accountByClass,
		NewRealLast24h:    countNewRealUsers(users, now, 24*time.Hour),
		NewRealLast7d:     countNewRealUsers(users, now, 7*24*time.Hour),
		NewRealLast30d:    countNewRealUsers(users, now, 30*24*time.Hour),
		AuthDataAvailable: authDataAvailable,
	}
	computers := PulseComputerSummary{
		TotalOwnerships:       len(ownerships),
		PrimaryComputers:      map[string]PulseComputerClassSummary{},
		RealPrimaryByState:    map[string]int{},
		CodexPrimaryByState:   map[string]int{},
		UnknownPrimaryByState: map[string]int{},
	}
	storage := PulseStorageSummary{
		VMStateBytesByClass:          emptyPulseInt64Map(),
		ExpensiveNixStoreWalkOmitted: true,
	}
	active24h := map[string]bool{}
	active7d := map[string]bool{}
	active30d := map[string]bool{}
	for _, own := range ownerships {
		if own == nil {
			continue
		}
		class := PulseAccountUnknown
		if user, ok := userByID[own.UserID]; ok {
			class = ClassifyPulseAccount(user.Email)
		}
		size := vmStateDirUsageBytes(stateDir, own.VMID)
		storage.VMStateBytesByClass[class] += size
		storage.VMStateBytesTotal += size
		if own.Kind != VMKindInteractive || own.DesktopID != PrimaryDesktopID {
			continue
		}
		addPulseComputerClass(computers.PrimaryComputers, class, string(own.State))
		switch class {
		case PulseAccountReal:
			computers.RealPrimaryTotal++
			computers.RealPrimaryByState[string(own.State)]++
			if pulseUsableComputerState(own.State) {
				computers.RealPrimaryUsable++
			}
			if !own.LastActiveAt.IsZero() {
				if now.Sub(own.LastActiveAt) <= 24*time.Hour {
					active24h[own.UserID] = true
				}
				if now.Sub(own.LastActiveAt) <= 7*24*time.Hour {
					active7d[own.UserID] = true
				}
				if now.Sub(own.LastActiveAt) <= 30*24*time.Hour {
					active30d[own.UserID] = true
				}
			}
		case PulseAccountCodexAgenticTest:
			computers.CodexPrimaryByState[string(own.State)]++
		case PulseAccountUnknown:
			computers.UnknownPrimaryByState[string(own.State)]++
		}
	}
	storage.ManualRecoverySnapshotBytes, storage.ManualRecoverySnapshotCount = pulseManualSnapshotUsage(stateDir)
	storage.VMStateFilesystem = pulseFilesystemSummary(stateDir)
	storage.NixStoreFilesystem = pulseFilesystemSummary("/nix/store")
	authDBReadAt := ""
	if authDataAvailable {
		authDBReadAt = now.Format(time.RFC3339)
	}
	return PulseSummary{
		Status:      "ok",
		GeneratedAt: now.Format(time.RFC3339),
		Privacy: PulsePrivacyStatement{
			Surface:              "public-readonly",
			DataMode:             "aggregate-only",
			ExcludedData:         []string{"prompts", "documents", "traces", "messages", "source histories", "generated artifacts", "per-user timelines", "email lists", "ip addresses", "geolocation", "user agents", "referrers", "device fingerprints", "session replay"},
			NoPrivateSuperset:    true,
			NoRowLevelAnalytics:  true,
			NoUserIdentityOutput: true,
		},
		Classifier: PulseClassifierSummary{
			Version:             "pulse-account-classifier-v1",
			RealClass:           PulseAccountReal,
			CodexDomains:        []string{"example.com", "example.test"},
			ProtectedTestCount:  len(pulseProtectedTestEmails),
			ProtectedTestPolicy: "owner-declared protected test accounts are excluded from real-user counts",
			UnknownPolicy:       "exclude from real-user counts until classified",
		},
		Accounts: accounts,
		Activity: PulseActivitySummary{
			RealActiveLast24h: len(active24h),
			RealActiveLast7d:  len(active7d),
			RealActiveLast30d: len(active30d),
			Source:            "primary computer last_active_at buckets",
		},
		Computers: computers,
		Storage:   storage,
		Reliability: PulseReliabilitySummary{
			RealPrimaryFailed:       computers.RealPrimaryByState[string(VMStateFailed)],
			RealPrimaryBooting:      computers.RealPrimaryByState[string(VMStateBooting)],
			RealPrimaryInaccessible: computers.RealPrimaryByState[string(VMStateDegraded)],
			NotCollected:            []string{"prompt content", "document content", "trace content", "per-user timelines", "ip addresses", "user agents", "referrers", "device fingerprints"},
		},
		Freshness: PulseFreshnessSummary{
			AuthDBReadAt:     authDBReadAt,
			OwnershipsReadAt: now.Format(time.RFC3339),
			StorageSampledAt: now.Format(time.RFC3339),
			GeneratedBy:      "vmctl",
		},
		Warnings: warnings,
	}
}

func loadPulseAccountsFromAuthDB(path string) ([]pulseAccountRecord, []string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, []string{"auth db path is not configured; account counts unavailable"}
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, []string{"auth db unavailable; account counts unavailable"}
	}
	defer db.Close()
	// A passkey's own created_at is when the sign-up finished.
	firstPasskey := map[string]time.Time{}
	credRows, err := db.Query(`SELECT user_id, created_at FROM credentials`)
	if err != nil {
		return nil, []string{"auth credentials unavailable; account counts unavailable"}
	}
	for credRows.Next() {
		var id string
		var at time.Time
		if err := credRows.Scan(&id, &at); err != nil {
			_ = credRows.Close()
			return nil, []string{"auth credentials unavailable; account counts unavailable"}
		}
		if prev, ok := firstPasskey[id]; !ok || at.Before(prev) {
			firstPasskey[id] = at
		}
	}
	if err := credRows.Err(); err != nil {
		_ = credRows.Close()
		return nil, []string{"auth credentials unavailable; account counts unavailable"}
	}
	_ = credRows.Close()
	rows, err := db.Query(`SELECT id, email, created_at, operator FROM users`)
	if err != nil {
		return nil, []string{"auth users unavailable; account counts unavailable"}
	}
	defer rows.Close()
	var out []pulseAccountRecord
	for rows.Next() {
		var rec pulseAccountRecord
		if err := rows.Scan(&rec.UserID, &rec.Email, &rec.CreatedAt, &rec.Operator); err != nil {
			return nil, []string{"auth users unavailable; account counts partially unavailable"}
		}
		rec.FirstPasskeyAt, rec.HasPasskey = firstPasskey[strings.TrimSpace(rec.UserID)]
		rec.UserID = strings.TrimSpace(rec.UserID)
		rec.Email = strings.TrimSpace(rec.Email)
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, []string{"auth users unavailable; account counts partially unavailable"}
	}
	return out, nil
}

func countNewRealUsers(users []pulseAccountRecord, now time.Time, window time.Duration) int {
	count := 0
	for _, user := range users {
		if ClassifyPulseAccount(user.Email) != PulseAccountReal || user.CreatedAt.IsZero() {
			continue
		}
		age := now.Sub(user.CreatedAt)
		if age >= 0 && age <= window {
			count++
		}
	}
	return count
}

func emptyPulseIntMap() map[string]int {
	return map[string]int{
		PulseAccountReal:             0,
		PulseAccountCodexAgenticTest: 0,
		PulseAccountProtectedTest:    0,
		PulseAccountInternal:         0,
		PulseAccountUnknown:          0,
	}
}

func emptyPulseInt64Map() map[string]int64 {
	return map[string]int64{
		PulseAccountReal:             0,
		PulseAccountCodexAgenticTest: 0,
		PulseAccountProtectedTest:    0,
		PulseAccountInternal:         0,
		PulseAccountUnknown:          0,
	}
}

func addPulseComputerClass(classes map[string]PulseComputerClassSummary, class, state string) {
	summary := classes[class]
	if summary.ByState == nil {
		summary.ByState = map[string]int{}
	}
	summary.Total++
	summary.ByState[state]++
	classes[class] = summary
}

func pulseUsableComputerState(state VMState) bool {
	switch state {
	case VMStateActive, VMStateHibernated, VMStateStopped:
		return true
	default:
		return false
	}
}

func pulseFilesystemSummary(path string) PulseFilesystemSummary {
	path = strings.TrimSpace(path)
	if path == "" {
		return PulseFilesystemSummary{}
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return PulseFilesystemSummary{Path: path}
	}
	total := int64(stat.Blocks) * int64(stat.Bsize)
	available := int64(stat.Bavail) * int64(stat.Bsize)
	used := total - available
	usedPercent := 0.0
	availablePercent := 0.0
	if total > 0 {
		usedPercent = float64(used) / float64(total) * 100
		availablePercent = float64(available) / float64(total) * 100
	}
	return PulseFilesystemSummary{
		Path:             path,
		TotalBytes:       total,
		AvailableBytes:   available,
		UsedBytes:        used,
		UsedPercent:      usedPercent,
		AvailablePercent: availablePercent,
	}
}

func pulseManualSnapshotUsage(stateDir string) (int64, int) {
	var bytes int64
	count := 0
	_ = filepath.WalkDir(stateDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasPrefix(name, "data.img.") || strings.HasSuffix(name, ".metadata.json") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		bytes += info.Size()
		count++
		return nil
	})
	return bytes, count
}
