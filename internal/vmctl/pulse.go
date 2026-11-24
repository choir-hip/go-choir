package vmctl

import (
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostroom"
	"github.com/yusefmosiah/go-choir/internal/vmmanager"
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
	// Proved is true when the account's address was proved (users.
	// email_verified_at): only such an account can sign in and open a
	// computer. It is read for the account counts, not for the room.
	Proved bool
	// FirstPasskeyAt is when the first passkey was stored: the moment the
	// sign-up finished (CreatedAt is when it began).
	FirstPasskeyAt time.Time
}

// PulseCapacitySummary is what the owner decides capacity on (the owner,
// 2026-11-23: "we will monitor usage and turn off signup once we hit
// limits"). Aggregate numbers only: no address, id or per-user figure. A
// reading that could not be taken shows no numbers and available false.
type PulseCapacitySummary struct {
	Memory    PulseCapacityMemory    `json:"memory"`
	Disk      PulseCapacityDisk      `json:"disk"`
	Computers PulseCapacityComputers `json:"computers"`
	Room      PulseCapacityRoom      `json:"room"`
}

// PulsePublicCapacity is all the public Pulse summary says of capacity:
// whether sign-up is open. Exact live memory, disk, computer counts and room
// figures are the owner's (the platform operator route), not a stranger's.
type PulsePublicCapacity struct {
	Room PulsePublicRoom `json:"room"`
}

type PulsePublicRoom struct {
	// Available is false when the reading could not be taken (shown as "no
	// reading", and sign-up is not open).
	Available bool `json:"available"`
	Open      bool `json:"open"`
}

// Public is the capacity reading a stranger may read.
func (c PulseCapacitySummary) Public() PulsePublicCapacity {
	return PulsePublicCapacity{Room: PulsePublicRoom{Available: c.Room.Available, Open: c.Room.Available && c.Room.Open}}
}

// PulsePublicStorage is the storage a stranger may read: counts, no bytes and
// no filesystem figures (the host's disk is the owner's reading).
type PulsePublicStorage struct {
	ManualRecoverySnapshotCount  int  `json:"manual_recovery_snapshot_count"`
	ExpensiveNixStoreWalkOmitted bool `json:"expensive_nix_store_walk_omitted"`
}

// PulsePublicSummary is the summary the public route serves: the same as
// PulseSummary with its capacity and storage blocks reduced (the outer fields
// shadow the embedded ones in the JSON).
type PulsePublicSummary struct {
	PulseSummary
	Capacity PulsePublicCapacity `json:"capacity"`
	Storage  PulsePublicStorage  `json:"storage"`
}

// Public is the summary a stranger may read.
func (s PulseSummary) Public() PulsePublicSummary {
	return PulsePublicSummary{
		PulseSummary: s,
		Capacity:     s.Capacity.Public(),
		Storage:      PulsePublicStorage{ManualRecoverySnapshotCount: s.Storage.ManualRecoverySnapshotCount, ExpensiveNixStoreWalkOmitted: s.Storage.ExpensiveNixStoreWalkOmitted},
	}
}

// PulseOperatorReading is what the platform operator reads: the full capacity
// and the storage figures.
type PulseOperatorReading struct {
	PulseCapacitySummary
	Storage PulseStorageSummary `json:"storage"`
}

// Operator is the reading the platform operator may read.
func (s PulseSummary) Operator() PulseOperatorReading {
	return PulseOperatorReading{PulseCapacitySummary: s.Capacity, Storage: s.Storage}
}

type PulseCapacityMemory struct {
	Available      bool   `json:"available"`
	TotalBytes     uint64 `json:"total_bytes"`
	AvailableBytes uint64 `json:"available_bytes"`
	// FloorBytes is what a start must leave free: below it a computer is
	// refused with a plain "host memory" answer, never crashed.
	FloorBytes uint64 `json:"floor_bytes"`
}

type PulseCapacityDisk struct {
	Available      bool   `json:"available"`
	AvailableBytes uint64 `json:"available_bytes"`
	// FloorBytes is the pressure floor: below it a start is refused. The
	// reserve sign-up keeps back is room.reserve_bytes.
	FloorBytes uint64 `json:"floor_bytes"`
}

type PulseCapacityComputers struct {
	Running    int `json:"running"`
	Hibernated int `json:"hibernated"`
	Other      int `json:"other"`
	// RunningGuestMemoryMiB is the guest memory promised to the running
	// computers: the ceiling their resident memory can grow to.
	RunningGuestMemoryMiB int `json:"running_guest_memory_mib"`
}

// PulseCapacityRoom is the reading sign-up is decided on (internal/hostroom):
// free disk over the reserve, less the gap to a footprint for each computer
// that has not grown to one, in whole footprints. An account with no computer
// holds nothing. Available is false when the disk sample, the owners file or
// the plan is missing: then no number is shown and sign-up is not open. The
// auth database is read only for Accounts and SignUpsLast24h.
type PulseCapacityRoom struct {
	Available      bool   `json:"available"`
	ReserveBytes   uint64 `json:"reserve_bytes"`
	FootprintBytes uint64 `json:"footprint_bytes"`
	// Accounts hold a proved passkey (a count to read, not an input: an
	// account with no computer holds no room, owner 2026-11-24). HeldBytes is
	// the disk the computers that exist will still take (the gap to a
	// footprint for each that has not grown to one), which the free reading
	// does not show yet.
	Accounts  int    `json:"accounts"`
	HeldBytes uint64 `json:"held_bytes"`
	// Places is how many more computers the host can hold; sign-up is open
	// when it is at least one.
	Places int  `json:"places"`
	Open   bool `json:"open"`
	// SignUpsLast24h are accounts that finished a passkey in the last day.
	SignUpsLast24h int `json:"sign_ups_last_24h"`
}

// SetRoomPlan records the plan for the capacity reading and the owners file
// (nix/node-b.nix roomFootprintGiB and roomReserveGiB,
// the same values the auth service gets).
func (r *OwnershipRegistry) SetRoomPlan(plan hostroom.Plan) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.roomPlan = plan
}

// SetRoomOwnersPath records where the owners file is published, so the
// capacity reading reads back the file the auth service reads.
func (r *OwnershipRegistry) SetRoomOwnersPath(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.roomOwnersPath = path
}

func pulseCapacity(now time.Time, users []pulseAccountRecord, usersAvailable bool, ownerships []*VMOwnership, owners map[string]uint64, ownersAvailable bool, sample HostPressureSample, cfg PressureReclaimConfig, plan hostroom.Plan) PulseCapacitySummary {
	var out PulseCapacitySummary
	if sample.ObservationError == "" && sample.MemoryTotalBytes > 0 {
		out.Memory = PulseCapacityMemory{Available: true, TotalBytes: sample.MemoryTotalBytes, AvailableBytes: sample.MemoryAvailableBytes, FloorBytes: sample.MemoryFloorBytes}
	}
	diskRead := sample.ObservationError == "" && sample.StateDirAvailableBytes > 0
	if diskRead {
		out.Disk = PulseCapacityDisk{Available: true, AvailableBytes: sample.StateDirAvailableBytes, FloorBytes: cfg.MinStateDirAvailableBytes}
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
	// The same reading the auth service decides on: the owners file, and its
	// age. A file auth cannot use closes sign-up there, so it is no reading here.
	if !diskRead || !plan.Valid() || !ownersAvailable {
		// No reading: no numbers and not open, never a guess.
		return out
	}
	// The auth database gives only the account counts; the room does not
	// depend on it.
	var holders []string
	for _, u := range users {
		if !usersAvailable || !u.HasPasskey || !u.Proved {
			continue
		}
		holders = append(holders, u.UserID)
		finished := u.FirstPasskeyAt
		if finished.IsZero() {
			finished = u.CreatedAt
		}
		if age := now.Sub(finished); !finished.IsZero() && age >= 0 && age <= 24*time.Hour {
			out.Room.SignUpsLast24h++
		}
	}
	out.Room.Available = true
	out.Room.ReserveBytes, out.Room.FootprintBytes = plan.ReserveBytes, plan.FootprintBytes
	out.Room.Accounts = len(holders)
	out.Room.HeldBytes = plan.Held(owners)
	out.Room.Places = plan.Places(sample.StateDirAvailableBytes, out.Room.HeldBytes)
	out.Room.Open = out.Room.Places >= 1
	return out
}

// ComputerOwners are the accounts that have a computer and the disk their
// computers hold now: the data image and, while it sleeps, the memory snapshot.
// Waking deletes the snapshot and the free reading rises by the same bytes the
// computer's disk falls by, so the room does not move.
//
// The disk is counted only as far as it is measured exactly, because the owners
// file is SUBTRACTED from the footprint (Plan.Held): a figure too high opens a
// place. Each data file is measured by vmmanager.FileDiskBytes, the snapshot
// budget's measure; vmctl holds CAP_SYS_ADMIN (nix/node-b.nix), so on Node B
// that is the btrfs extent reading, which sees compress=zstd (st_blocks would
// count a compressed 4 GiB memory file at 4 GiB). A file measured any other
// way (the FIEMAP bound, st_blocks, an error) counts 0, so the computer holds
// its whole footprint back. The snapshot is counted from the disk_bytes its
// writer recorded in meta.json, under the same rule. An account's computers
// are summed. Any ownership holds its disk, whatever its state.
//
// Each VM's figure is cached and recomputed when its state changes, when
// fresh is true (the 30 s tick), or when a disk release ends; a publish
// triggered by a registry change reuses it, so a busy host does not walk every
// computer's extents every two seconds.
func (r *OwnershipRegistry) ComputerOwners(fresh bool) []hostroom.Owner {
	type vm struct {
		id    string
		state VMState
	}
	r.mu.RLock()
	stateDir := normalizeRetentionPruneConfig(r.retentionPrune).StateDir
	measure := r.diskMeasure
	if measure == nil {
		measure = vmmanager.FileDiskBytes
	}
	vmsByUser := map[string][]vm{}
	for _, own := range r.ownerships {
		if own == nil || own.UserID == "" {
			continue
		}
		vmsByUser[own.UserID] = append(vmsByUser[own.UserID], vm{own.VMID, own.State})
	}
	r.mu.RUnlock()
	out := make([]hostroom.Owner, 0, len(vmsByUser))
	for id, vms := range vmsByUser {
		seen := map[string]bool{}
		var total uint64
		inexact := 0
		for _, v := range vms {
			if seen[v.id] {
				continue
			}
			seen[v.id] = true
			n, bad := r.vmDiskBytes(stateDir, v.id, v.state, measure, fresh)
			total += n
			inexact += bad
		}
		out = append(out, hostroom.Owner{UserID: id, DiskBytes: total, InexactFiles: inexact})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })
	return out
}

type diskCacheEntry struct {
	state   VMState
	bytes   uint64
	inexact int
}

const (
	diskReleaseSnapshot = 1 // the snapshot is about to be consumed
	diskReleaseWhole    = 2 // the computer's state is about to be destroyed
)

func (r *OwnershipRegistry) vmDiskBytes(stateDir, vmID string, state VMState, measure func(string) (int64, string, error), fresh bool) (uint64, int) {
	r.diskMu.Lock()
	defer r.diskMu.Unlock()
	switch r.diskReleasing[vmID] {
	case diskReleaseWhole:
		return 0, 0
	case diskReleaseSnapshot:
		n, bad := vmStateDirDiskBytes(stateDir, vmID, measure, true)
		return uint64(n), bad
	}
	if e, ok := r.diskCache[vmID]; ok && !fresh && e.state == state {
		return e.bytes, e.inexact
	}
	n, bad := vmStateDirDiskBytes(stateDir, vmID, measure, false)
	if r.diskCache == nil {
		r.diskCache = map[string]diskCacheEntry{}
	}
	r.diskCache[vmID] = diskCacheEntry{state: state, bytes: uint64(n), inexact: bad}
	return uint64(n), bad
}

// beginDiskRelease is called, with no registry lock held, before something
// deletes a computer's disk (a snapshot resume consumes the snapshot; a prune
// destroys the state). The owners file is rewritten first with that disk
// already out of the computer's figure, so the room holds the bytes back
// while the free reading still has them, and gives nothing back when the
// free reading rises. The returned function ends the release: the figure is
// recomputed and published again. Without it the file would lag the delete and
// open a place for the length of the operation.
func (r *OwnershipRegistry) beginDiskRelease(vmID string, kind int) func() {
	r.diskMu.Lock()
	if r.diskReleasing == nil {
		r.diskReleasing = map[string]int{}
	}
	r.diskReleasing[vmID] = kind
	r.diskMu.Unlock()
	r.publishOwnersNow()
	return func() {
		r.diskMu.Lock()
		delete(r.diskReleasing, vmID)
		delete(r.diskCache, vmID)
		r.diskMu.Unlock()
		r.noteOwnersChanged()
	}
}

// publishOwnersNow writes the owners file now, from the cache, when a
// publisher is running; otherwise it does nothing.
func (r *OwnershipRegistry) publishOwnersNow() {
	r.mu.RLock()
	path := r.ownersPublishPath
	r.mu.RUnlock()
	if path == "" {
		return
	}
	r.ownersWriteMu.Lock()
	defer r.ownersWriteMu.Unlock()
	if err := hostroom.WriteOwners(path, time.Now(), r.ComputerOwners(false)); err != nil {
		log.Printf("vmctl: publish computer owners: %v", err)
	}
}

// roomOwnersReading reads the file the auth service reads, with the age it
// accepts. Unavailable when no path is set or the file is missing, torn,
// stale or from the future.
func (r *OwnershipRegistry) roomOwnersReading(now time.Time) (map[string]uint64, bool) {
	r.mu.RLock()
	path := r.roomOwnersPath
	r.mu.RUnlock()
	if strings.TrimSpace(path) == "" {
		return nil, false
	}
	owners, err := hostroom.ReadOwners(path, now, hostroom.MaxOwnersAge)
	if err != nil {
		return nil, false
	}
	return owners, true
}

// PublishComputerOwners writes the accounts that have a computer where the
// auth service reads them (internal/hostroom), every figure measured afresh.
// The auth service closes sign-up when the file is missing or older than
// hostroom.MaxOwnersAge.
func (r *OwnershipRegistry) PublishComputerOwners(path string, at time.Time) error {
	return r.publishComputerOwners(path, at, true)
}

func (r *OwnershipRegistry) publishComputerOwners(path string, at time.Time, fresh bool) error {
	r.ownersWriteMu.Lock()
	defer r.ownersWriteMu.Unlock()
	return hostroom.WriteOwners(path, at, r.ComputerOwners(fresh))
}

// ComputerOwnersInterval is how often the owners file is rewritten: well
// inside hostroom.MaxOwnersAge, so one missed write does not close sign-up.
const ComputerOwnersInterval = 30 * time.Second

// ownersPublishGap is the least time between two publishes triggered by
// changes, so a burst of ownership saves is one write.
const ownersPublishGap = 2 * time.Second

// noteOwnersChanged asks the publisher for a write now: called when the
// ownership registry is saved (a computer made, removed, pruned, woken,
// hibernated). It never blocks.
func (r *OwnershipRegistry) noteOwnersChanged() {
	select {
	case r.ownersChanged <- struct{}{}:
	default:
	}
}

// StartComputerOwnersPublisher writes the owners file now, on every change of
// the registry (so a wake or a removal is in the file without waiting for the
// tick: a stale file would hold back room that has gone, or release room that
// has not), and every interval until ctx ends. A write that fails is logged;
// the file then ages and the auth service closes sign-up, which is the safe
// direction. The returned channel is closed when the publisher has stopped.
func (r *OwnershipRegistry) StartComputerOwnersPublisher(ctx context.Context, path string, interval time.Duration, logf func(string, ...any)) <-chan struct{} {
	r.mu.Lock()
	r.ownersPublishPath = path
	r.mu.Unlock()
	publish := func(fresh bool) {
		if err := r.publishComputerOwners(path, time.Now(), fresh); err != nil && logf != nil {
			logf("vmctl: publish computer owners: %v", err)
		}
	}
	publish(true)
	done := make(chan struct{})
	go func() {
		defer close(done)
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				publish(true)
			case <-r.ownersChanged:
				publish(false)
				select {
				case <-ctx.Done():
					return
				case <-time.After(ownersPublishGap):
				}
			}
		}
	}()
	return done
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
	pcfg, sampler, plan := normalizePressureReclaimConfig(r.pressureReclaim), r.pressureSampler, r.roomPlan
	r.mu.RUnlock()
	if sampler == nil {
		sampler = sampleHostPressure
	}
	owners, ownersAvailable := r.roomOwnersReading(now)
	summary.Capacity = pulseCapacity(now, users, authDataAvailable, ownerships, owners, ownersAvailable, sampler(pcfg), pcfg, plan)
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
	rows, err := db.Query(`SELECT id, email, created_at, email_verified_at FROM users`)
	if err != nil {
		return nil, []string{"auth users unavailable; account counts unavailable"}
	}
	defer rows.Close()
	var out []pulseAccountRecord
	for rows.Next() {
		var rec pulseAccountRecord
		var verified sql.NullTime
		if err := rows.Scan(&rec.UserID, &rec.Email, &rec.CreatedAt, &verified); err != nil {
			return nil, []string{"auth users unavailable; account counts partially unavailable"}
		}
		rec.Proved = verified.Valid
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
