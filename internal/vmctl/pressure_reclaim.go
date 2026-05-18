package vmctl

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	PressureReclaimModeOff    = "off"
	PressureReclaimModeDryRun = "dry-run"
	PressureReclaimModeActive = "active"
)

// PressureReclaimConfig controls pressure-aware reclaim. Dry-run mode samples
// real host pressure and ranks candidate computers without hibernating them.
// Active mode uses the same ranking and protection rules to hibernate a bounded
// number of eligible idle computers when pressure crosses the configured
// threshold.
type PressureReclaimConfig struct {
	Mode                      string
	MinIdle                   time.Duration
	MinMemoryAvailableBytes   uint64
	MinMemoryAvailablePercent float64
	MaxMemorySomeAvg10        float64
	MaxCPUSomeAvg10           float64
	MaxIOSomeAvg10            float64
	StateDir                  string
	MaxCandidates             int
}

// HostPressureSample is a single host resource-pressure observation.
type HostPressureSample struct {
	SampledAt                    string  `json:"sampled_at"`
	MemoryTotalBytes             uint64  `json:"memory_total_bytes,omitempty"`
	MemoryAvailableBytes         uint64  `json:"memory_available_bytes,omitempty"`
	MemoryAvailablePercent       float64 `json:"memory_available_percent,omitempty"`
	MemorySomeAvg10              float64 `json:"memory_some_avg10,omitempty"`
	MemoryFullAvg10              float64 `json:"memory_full_avg10,omitempty"`
	CPUSomeAvg10                 float64 `json:"cpu_some_avg10,omitempty"`
	CPUFullAvg10                 float64 `json:"cpu_full_avg10,omitempty"`
	IOSomeAvg10                  float64 `json:"io_some_avg10,omitempty"`
	IOFullAvg10                  float64 `json:"io_full_avg10,omitempty"`
	StateDirAvailableBytes       uint64  `json:"state_dir_available_bytes,omitempty"`
	StateDirAvailablePercent     float64 `json:"state_dir_available_percent,omitempty"`
	PIDCurrent                   int     `json:"pid_current,omitempty"`
	PIDMax                       int     `json:"pid_max,omitempty"`
	PIDAvailable                 int     `json:"pid_available,omitempty"`
	ObservationError             string  `json:"observation_error,omitempty"`
	MemoryPressure               bool    `json:"memory_pressure"`
	CPUPressure                  bool    `json:"cpu_pressure"`
	IOPressure                   bool    `json:"io_pressure"`
	Pressure                     bool    `json:"pressure"`
	MemoryAvailableThresholdText string  `json:"memory_available_threshold,omitempty"`
	MemoryPSIThresholdText       string  `json:"memory_psi_threshold,omitempty"`
	CPUPSIThresholdText          string  `json:"cpu_psi_threshold,omitempty"`
	IOPSIThresholdText           string  `json:"io_psi_threshold,omitempty"`
}

type PressureReclaimInventory struct {
	TotalOwnerships  int `json:"total_ownerships"`
	Active           int `json:"active"`
	Interactive      int `json:"interactive"`
	Workers          int `json:"workers"`
	Protected        int `json:"protected"`
	Eligible         int `json:"eligible"`
	CandidatesRanked int `json:"candidates_ranked"`
}

type PressureReclaimCandidate struct {
	Rank             int      `json:"rank"`
	Kind             VMKind   `json:"kind"`
	State            VMState  `json:"state"`
	Desktop          string   `json:"desktop,omitempty"`
	WarmnessClass    string   `json:"warmness_class,omitempty"`
	IdleSeconds      int64    `json:"idle_seconds"`
	Protected        bool     `json:"protected"`
	ProtectedReasons []string `json:"protected_reasons,omitempty"`
	ProposedAction   string   `json:"proposed_action"`
}

type PressureReclaimPlan struct {
	Mode       string                       `json:"mode"`
	Decision   string                       `json:"decision"`
	Reason     string                       `json:"reason"`
	Pressure   HostPressureSample           `json:"pressure"`
	Inventory  PressureReclaimInventory     `json:"inventory"`
	Candidates []PressureReclaimCandidate   `json:"candidates,omitempty"`
	Config     PressureReclaimConfigSummary `json:"config"`
}

type PressureReclaimConfigSummary struct {
	MinIdleSeconds            int64   `json:"min_idle_seconds"`
	MinMemoryAvailableBytes   uint64  `json:"min_memory_available_bytes,omitempty"`
	MinMemoryAvailablePercent float64 `json:"min_memory_available_percent,omitempty"`
	MaxMemorySomeAvg10        float64 `json:"max_memory_some_avg10,omitempty"`
	MaxCPUSomeAvg10           float64 `json:"max_cpu_some_avg10,omitempty"`
	MaxIOSomeAvg10            float64 `json:"max_io_some_avg10,omitempty"`
	MaxCandidates             int     `json:"max_candidates"`
	StateDir                  string  `json:"state_dir,omitempty"`
}

type hostPressureSampler func(PressureReclaimConfig) HostPressureSample

func DefaultPressureReclaimConfig() PressureReclaimConfig {
	return PressureReclaimConfig{
		Mode:                      PressureReclaimModeOff,
		MinIdle:                   30 * time.Minute,
		MinMemoryAvailableBytes:   2 * 1024 * 1024 * 1024,
		MinMemoryAvailablePercent: 15,
		MaxMemorySomeAvg10:        1.0,
		MaxCPUSomeAvg10:           90.0,
		MaxIOSomeAvg10:            5.0,
		StateDir:                  "/var/lib/go-choir/vm-state",
		MaxCandidates:             5,
	}
}

func normalizePressureReclaimConfig(cfg PressureReclaimConfig) PressureReclaimConfig {
	cfg.Mode = strings.TrimSpace(strings.ToLower(cfg.Mode))
	switch cfg.Mode {
	case "", PressureReclaimModeOff:
		cfg.Mode = PressureReclaimModeOff
	case "dryrun", "observe", "observation", PressureReclaimModeDryRun:
		cfg.Mode = PressureReclaimModeDryRun
	case "reclaim", "enforce", PressureReclaimModeActive:
		cfg.Mode = PressureReclaimModeActive
	default:
		cfg.Mode = PressureReclaimModeOff
	}
	if cfg.MinIdle <= 0 {
		cfg.MinIdle = 30 * time.Minute
	}
	if cfg.MaxCandidates <= 0 {
		cfg.MaxCandidates = 5
	}
	if strings.TrimSpace(cfg.StateDir) == "" {
		cfg.StateDir = "/var/lib/go-choir/vm-state"
	}
	return cfg
}

func pressureConfigSummary(cfg PressureReclaimConfig) PressureReclaimConfigSummary {
	return PressureReclaimConfigSummary{
		MinIdleSeconds:            int64(cfg.MinIdle.Seconds()),
		MinMemoryAvailableBytes:   cfg.MinMemoryAvailableBytes,
		MinMemoryAvailablePercent: cfg.MinMemoryAvailablePercent,
		MaxMemorySomeAvg10:        cfg.MaxMemorySomeAvg10,
		MaxCPUSomeAvg10:           cfg.MaxCPUSomeAvg10,
		MaxIOSomeAvg10:            cfg.MaxIOSomeAvg10,
		MaxCandidates:             cfg.MaxCandidates,
		StateDir:                  cfg.StateDir,
	}
}

func sampleHostPressure(cfg PressureReclaimConfig) HostPressureSample {
	sample := HostPressureSample{SampledAt: time.Now().UTC().Format(time.RFC3339)}
	var errs []string

	if total, available, err := readMemInfo(); err != nil {
		errs = append(errs, err.Error())
	} else {
		sample.MemoryTotalBytes = total
		sample.MemoryAvailableBytes = available
		if total > 0 {
			sample.MemoryAvailablePercent = (float64(available) / float64(total)) * 100
		}
	}
	if some, full, err := readPressureAvg10("/proc/pressure/memory"); err != nil {
		errs = append(errs, err.Error())
	} else {
		sample.MemorySomeAvg10 = some
		sample.MemoryFullAvg10 = full
	}
	if some, full, err := readPressureAvg10("/proc/pressure/cpu"); err != nil {
		errs = append(errs, err.Error())
	} else {
		sample.CPUSomeAvg10 = some
		sample.CPUFullAvg10 = full
	}
	if some, full, err := readPressureAvg10("/proc/pressure/io"); err != nil {
		errs = append(errs, err.Error())
	} else {
		sample.IOSomeAvg10 = some
		sample.IOFullAvg10 = full
	}
	if available, total, err := statfsAvailable(cfg.StateDir); err != nil {
		errs = append(errs, err.Error())
	} else {
		sample.StateDirAvailableBytes = available
		if total > 0 {
			sample.StateDirAvailablePercent = (float64(available) / float64(total)) * 100
		}
	}
	if current, max, err := readPIDPressure(); err != nil {
		errs = append(errs, err.Error())
	} else {
		sample.PIDCurrent = current
		sample.PIDMax = max
		sample.PIDAvailable = max - current
	}

	annotatePressure(&sample, cfg)
	if len(errs) > 0 {
		sample.ObservationError = strings.Join(errs, "; ")
	}
	return sample
}

func annotatePressure(sample *HostPressureSample, cfg PressureReclaimConfig) {
	if sample == nil {
		return
	}
	sample.MemoryPressure = false
	sample.CPUPressure = false
	sample.IOPressure = false
	sample.Pressure = false
	sample.MemoryAvailableThresholdText = ""
	sample.MemoryPSIThresholdText = ""
	sample.CPUPSIThresholdText = ""
	sample.IOPSIThresholdText = ""
	if cfg.MinMemoryAvailableBytes > 0 {
		sample.MemoryAvailableThresholdText = fmt.Sprintf("available_bytes<%d", cfg.MinMemoryAvailableBytes)
		if sample.MemoryAvailableBytes > 0 && sample.MemoryAvailableBytes < cfg.MinMemoryAvailableBytes {
			sample.MemoryPressure = true
		}
	}
	if cfg.MinMemoryAvailablePercent > 0 {
		threshold := fmt.Sprintf("available_percent<%.2f", cfg.MinMemoryAvailablePercent)
		if sample.MemoryAvailableThresholdText == "" {
			sample.MemoryAvailableThresholdText = threshold
		} else {
			sample.MemoryAvailableThresholdText += "," + threshold
		}
		if sample.MemoryAvailablePercent > 0 && sample.MemoryAvailablePercent < cfg.MinMemoryAvailablePercent {
			sample.MemoryPressure = true
		}
	}
	if cfg.MaxMemorySomeAvg10 > 0 {
		sample.MemoryPSIThresholdText = fmt.Sprintf("memory_some_avg10>%.2f", cfg.MaxMemorySomeAvg10)
		if sample.MemorySomeAvg10 > cfg.MaxMemorySomeAvg10 {
			sample.MemoryPressure = true
		}
	}
	if cfg.MaxCPUSomeAvg10 > 0 {
		sample.CPUPSIThresholdText = fmt.Sprintf("cpu_some_avg10>%.2f", cfg.MaxCPUSomeAvg10)
		if sample.CPUSomeAvg10 > cfg.MaxCPUSomeAvg10 {
			sample.CPUPressure = true
		}
	}
	if cfg.MaxIOSomeAvg10 > 0 {
		sample.IOPSIThresholdText = fmt.Sprintf("io_some_avg10>%.2f", cfg.MaxIOSomeAvg10)
		if sample.IOSomeAvg10 > cfg.MaxIOSomeAvg10 {
			sample.IOPressure = true
		}
	}
	sample.Pressure = sample.MemoryPressure || sample.CPUPressure || sample.IOPressure
}

func readMemInfo() (totalBytes uint64, availableBytes uint64, err error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, fmt.Errorf("read meminfo: %w", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		valueKB, parseErr := strconv.ParseUint(fields[1], 10, 64)
		if parseErr != nil {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			totalBytes = valueKB * 1024
		case "MemAvailable":
			availableBytes = valueKB * 1024
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, fmt.Errorf("scan meminfo: %w", err)
	}
	if totalBytes == 0 || availableBytes == 0 {
		return 0, 0, fmt.Errorf("meminfo missing MemTotal or MemAvailable")
	}
	return totalBytes, availableBytes, nil
}

func readPressureAvg10(path string) (some float64, full float64, err error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, fmt.Errorf("read %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		value := parseAvg10(fields[1:])
		switch fields[0] {
		case "some":
			some = value
		case "full":
			full = value
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, fmt.Errorf("scan %s: %w", path, err)
	}
	return some, full, nil
}

func parseAvg10(fields []string) float64 {
	for _, field := range fields {
		key, value, ok := strings.Cut(field, "=")
		if !ok || key != "avg10" {
			continue
		}
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0
		}
		return parsed
	}
	return 0
}

func statfsAvailable(path string) (availableBytes uint64, totalBytes uint64, err error) {
	if strings.TrimSpace(path) == "" {
		path = "/"
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, fmt.Errorf("statfs %s: %w", path, err)
	}
	return uint64(stat.Bavail) * uint64(stat.Bsize), uint64(stat.Blocks) * uint64(stat.Bsize), nil
}

func readPIDPressure() (current int, max int, err error) {
	data, err := os.ReadFile("/proc/sys/kernel/pid_max")
	if err != nil {
		return 0, 0, fmt.Errorf("read pid_max: %w", err)
	}
	max64, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse pid_max: %w", err)
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, 0, fmt.Errorf("read proc pids: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(entry.Name()); err == nil {
			current++
		}
	}
	return current, int(max64), nil
}

type pressureCandidateInternal struct {
	own      *VMOwnership
	public   PressureReclaimCandidate
	priority int
	idle     time.Duration
}

func (r *OwnershipRegistry) PressureReclaimPlan() PressureReclaimPlan {
	r.mu.RLock()
	cfg := r.pressureReclaim
	warmnessPolicy := r.warmnessPolicy
	sampler := r.pressureSampler
	ownerships := make([]*VMOwnership, 0, len(r.ownerships)+len(r.workerVMs))
	for _, own := range r.ownerships {
		ownerships = append(ownerships, cloneOwnership(own))
	}
	for _, own := range r.workerVMs {
		ownerships = append(ownerships, cloneOwnership(own))
	}
	r.mu.RUnlock()

	cfg = normalizePressureReclaimConfig(cfg)
	plan := PressureReclaimPlan{
		Mode:     cfg.Mode,
		Decision: "off",
		Reason:   "pressure-aware reclaim disabled",
		Config:   pressureConfigSummary(cfg),
	}
	if cfg.Mode == PressureReclaimModeOff {
		plan.Inventory = pressureInventory(ownerships, nil)
		return plan
	}

	if sampler == nil {
		sampler = sampleHostPressure
	}
	sample := sampler(cfg)
	annotatePressure(&sample, cfg)
	plan.Pressure = sample

	now := time.Now()
	candidates := rankPressureCandidates(ownerships, cfg, warmnessPolicy, now)
	plan.Inventory = pressureInventory(ownerships, candidates)
	limit := cfg.MaxCandidates
	if limit > len(candidates) {
		limit = len(candidates)
	}
	for i := 0; i < limit; i++ {
		candidate := candidates[i].public
		candidate.Rank = i + 1
		plan.Candidates = append(plan.Candidates, candidate)
	}

	if len(candidates) == 0 {
		plan.Decision = "observe"
		plan.Reason = "no active ownerships are eligible for pressure reclaim"
		return plan
	}
	if plan.Inventory.Eligible == 0 {
		plan.Decision = "observe"
		plan.Reason = "no unprotected active ownerships are eligible for pressure reclaim"
		return plan
	}
	if sample.Pressure && cfg.Mode == PressureReclaimModeActive {
		plan.Decision = "reclaim"
		plan.Reason = "host pressure crossed active threshold; eligible VMs may be hibernated"
		return plan
	}
	if sample.Pressure {
		plan.Decision = "would_reclaim"
		plan.Reason = "host pressure crossed dry-run threshold; no VM hibernated"
		return plan
	}
	plan.Decision = "observe"
	plan.Reason = "host pressure below reclaim threshold"
	return plan
}

func (r *OwnershipRegistry) pressureReclaimActionCandidates() []pressureCandidateInternal {
	r.mu.RLock()
	cfg := r.pressureReclaim
	warmnessPolicy := r.warmnessPolicy
	sampler := r.pressureSampler
	ownerships := make([]*VMOwnership, 0, len(r.ownerships)+len(r.workerVMs))
	for _, own := range r.ownerships {
		ownerships = append(ownerships, cloneOwnership(own))
	}
	for _, own := range r.workerVMs {
		ownerships = append(ownerships, cloneOwnership(own))
	}
	r.mu.RUnlock()

	cfg = normalizePressureReclaimConfig(cfg)
	if cfg.Mode != PressureReclaimModeActive {
		return nil
	}
	if sampler == nil {
		sampler = sampleHostPressure
	}
	sample := sampler(cfg)
	annotatePressure(&sample, cfg)
	if !sample.Pressure {
		return nil
	}

	ranked := rankPressureCandidates(ownerships, cfg, warmnessPolicy, time.Now())
	limit := cfg.MaxCandidates
	if limit > len(ranked) {
		limit = len(ranked)
	}
	selected := make([]pressureCandidateInternal, 0, limit)
	for _, candidate := range ranked {
		if candidate.public.Protected {
			continue
		}
		selected = append(selected, candidate)
		if len(selected) >= limit {
			break
		}
	}
	return selected
}

func cloneOwnership(own *VMOwnership) *VMOwnership {
	if own == nil {
		return nil
	}
	cp := *own
	return &cp
}

func pressureInventory(ownerships []*VMOwnership, candidates []pressureCandidateInternal) PressureReclaimInventory {
	inv := PressureReclaimInventory{TotalOwnerships: len(ownerships)}
	for _, own := range ownerships {
		if own == nil {
			continue
		}
		if own.State == VMStateActive {
			inv.Active++
		}
		if own.Kind == VMKindWorker {
			inv.Workers++
		} else {
			inv.Interactive++
		}
	}
	for _, candidate := range candidates {
		if candidate.public.Protected {
			inv.Protected++
		} else {
			inv.Eligible++
		}
	}
	inv.CandidatesRanked = len(candidates)
	return inv
}

func rankPressureCandidates(ownerships []*VMOwnership, cfg PressureReclaimConfig, warmnessPolicy WarmnessPolicyConfig, now time.Time) []pressureCandidateInternal {
	candidates := make([]pressureCandidateInternal, 0, len(ownerships))
	for _, own := range ownerships {
		if own == nil || own.State != VMStateActive {
			continue
		}
		candidates = append(candidates, pressureCandidateForOwnership(own, cfg, warmnessPolicy, now))
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if left.public.Protected != right.public.Protected {
			return !left.public.Protected
		}
		if left.priority != right.priority {
			return left.priority < right.priority
		}
		if left.idle != right.idle {
			return left.idle > right.idle
		}
		return left.own.VMID < right.own.VMID
	})
	return candidates
}

func pressureCandidateForOwnership(own *VMOwnership, cfg PressureReclaimConfig, warmnessPolicy WarmnessPolicyConfig, now time.Time) pressureCandidateInternal {
	idle := time.Duration(0)
	if !own.LastActiveAt.IsZero() {
		idle = now.Sub(own.LastActiveAt)
	}
	warmnessClass := warmnessClassForOwnership(own, warmnessPolicy)
	reasons := protectedReclaimReasons(own, cfg, warmnessPolicy, idle)
	priority := warmnessPriority(warmnessClass)
	if warmnessClass == WarmnessClassCriticalProtected && staleCriticalWorkerIdle(idle) {
		priority = warmnessPriority(WarmnessClassWorker)
	}
	desktop := "primary"
	if own.Kind == VMKindInteractive && own.DesktopID != PrimaryDesktopID {
		desktop = "published"
	}
	return pressureCandidateInternal{
		own:      own,
		priority: priority,
		idle:     idle,
		public: PressureReclaimCandidate{
			Kind:             own.Kind,
			State:            own.State,
			Desktop:          desktop,
			WarmnessClass:    string(warmnessClass),
			IdleSeconds:      int64(idle.Seconds()),
			Protected:        len(reasons) > 0,
			ProtectedReasons: reasons,
			ProposedAction:   "hibernate",
		},
	}
}

func protectedReclaimReasons(own *VMOwnership, cfg PressureReclaimConfig, warmnessPolicy WarmnessPolicyConfig, idle time.Duration) []string {
	var reasons []string
	if own == nil {
		return []string{"missing_ownership"}
	}
	switch warmnessClassForOwnership(own, warmnessPolicy) {
	case WarmnessClassPremiumAlwaysOn:
		reasons = append(reasons, "premium_always_on")
	case WarmnessClassCriticalProtected:
		if !staleCriticalWorkerIdle(idle) {
			reasons = append(reasons, "critical_protected")
		}
	}
	if own.LastActiveAt.IsZero() {
		reasons = append(reasons, "unknown_last_active")
	}
	if cfg.MinIdle > 0 && idle < cfg.MinIdle {
		reasons = append(reasons, "recent_activity")
	}
	if own.Kind == VMKindWorker && criticalWorkerPurpose(own) && !staleCriticalWorkerIdle(idle) {
		reasons = append(reasons, "critical_worker_purpose")
	}
	return reasons
}

func staleCriticalWorkerIdle(idle time.Duration) bool {
	return idle >= criticalWorkerProtectionMaxIdle
}

func criticalWorkerPurpose(own *VMOwnership) bool {
	if own == nil {
		return false
	}
	text := strings.ToLower(strings.Join([]string{
		own.Purpose,
		own.TrajectoryID,
		own.ObjectiveFingerprint,
		own.ParentAgentID,
	}, " "))
	for _, marker := range []string{"verifier", "verify", "promotion", "rollback", "publication"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
