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

	// PressureReclaimSignalSustained reclaims only when memory is short and
	// stalling for several consecutive samples: PSI memory some avg60 over
	// MaxMemorySomeAvg60 AND MemAvailable under the floor, SustainedSamples
	// in a row, none of them during or just after a hibernate's snapshot
	// write. One reclaim in flight, one computer per reclaim, then a
	// cool-down (docs/problems/pressure-reclaim-wedged-a-user-computer-2026-10-14.md).
	PressureReclaimSignalSustained = "sustained"
	// PressureReclaimSignalInstant is the rollback: one sample decides
	// (avg10 over MaxMemorySomeAvg10 OR MemAvailable under the floor), up to
	// MaxCandidates per pass, no cool-down. The behaviour before 2026-10-14.
	PressureReclaimSignalInstant = "instant"

	defaultMaxMemorySomeAvg60 = 5.0
	defaultSustainedSamples   = 4
	defaultPressureSample     = 15 * time.Second
	defaultReclaimCooldown    = 3 * time.Minute
)

// PressureReclaimConfig controls pressure-aware reclaim. Dry-run mode samples
// real host pressure and ranks candidate computers without hibernating them.
// Active mode uses the same ranking and protection rules to hibernate a bounded
// number of eligible idle computers when pressure crosses the configured
// threshold.
type PressureReclaimConfig struct {
	Mode                        string
	MinIdle                     time.Duration
	MinMemoryAvailableBytes     uint64
	MinMemoryAvailablePercent   float64
	MinStateDirAvailableBytes   uint64
	MinStateDirAvailablePercent float64
	MaxMemorySomeAvg10          float64
	MaxCPUSomeAvg10             float64
	MaxIOSomeAvg10              float64
	StateDir                    string
	MaxCandidates               int
	StaleStateMinAge            time.Duration
	MaxStateDeletes             int

	// Signal selects how pressure becomes a reclaim: sustained (the
	// default, also for an empty value) or instant (the rollback). See the
	// PressureReclaimSignal constants.
	Signal string
	// MaxMemorySomeAvg60 is the sustained signal's PSI threshold.
	MaxMemorySomeAvg60 float64
	// SustainedSamples is how many consecutive qualifying samples the
	// sustained signal needs.
	SustainedSamples int
	// SampleInterval is the pressure monitor's period. A streak whose newest
	// sample is older than twice this is stale.
	SampleInterval time.Duration
	// Cooldown follows every hibernate (any reason): samples inside it do
	// not count, and no reclaim starts. It covers the snapshot write and the
	// PSI decay after it.
	Cooldown time.Duration
}

// HostPressureSample is a single host resource-pressure observation.
type HostPressureSample struct {
	SampledAt              string  `json:"sampled_at"`
	MemoryTotalBytes       uint64  `json:"memory_total_bytes,omitempty"`
	MemoryAvailableBytes   uint64  `json:"memory_available_bytes,omitempty"`
	MemoryAvailablePercent float64 `json:"memory_available_percent,omitempty"`
	MemorySomeAvg10        float64 `json:"memory_some_avg10,omitempty"`
	MemoryFullAvg10        float64 `json:"memory_full_avg10,omitempty"`
	MemorySomeAvg60        float64 `json:"memory_some_avg60,omitempty"`
	// MemoryFloorBytes is the host memory floor this sample was judged
	// against: max(MinMemoryAvailableBytes, MinMemoryAvailablePercent).
	MemoryFloorBytes         uint64  `json:"memory_floor_bytes,omitempty"`
	CPUSomeAvg10             float64 `json:"cpu_some_avg10,omitempty"`
	CPUFullAvg10             float64 `json:"cpu_full_avg10,omitempty"`
	IOSomeAvg10              float64 `json:"io_some_avg10,omitempty"`
	IOFullAvg10              float64 `json:"io_full_avg10,omitempty"`
	StateDirAvailableBytes   uint64  `json:"state_dir_available_bytes,omitempty"`
	StateDirAvailablePercent float64 `json:"state_dir_available_percent,omitempty"`
	PIDCurrent               int     `json:"pid_current,omitempty"`
	PIDMax                   int     `json:"pid_max,omitempty"`
	PIDAvailable             int     `json:"pid_available,omitempty"`
	ObservationError         string  `json:"observation_error,omitempty"`
	MemoryPressure           bool    `json:"memory_pressure"`
	CPUPressure              bool    `json:"cpu_pressure"`
	IOPressure               bool    `json:"io_pressure"`
	StateDirPressure         bool    `json:"state_dir_pressure"`
	// Pressure is any signal over its threshold, reported. ReclaimPressure
	// is the subset a hibernation can relieve (memory, state-dir disk): a
	// CPU or IO stall is reported, never answered by killing a guest
	// (docs/reports/resource-efficiency-memory-assessment-2026-10-13.md, R3).
	//
	// Under the sustained signal ReclaimPressure means this one sample
	// qualifies (avg60 over threshold and memory under the floor, or the
	// state dir under its floor); a reclaim still needs the streak.
	Pressure                       bool     `json:"pressure"`
	ReclaimPressure                bool     `json:"reclaim_pressure"`
	ReclaimSignal                  string   `json:"reclaim_signal,omitempty"`
	Signals                        []string `json:"signals,omitempty"`
	MemoryAvailableThresholdText   string   `json:"memory_available_threshold,omitempty"`
	StateDirAvailableThresholdText string   `json:"state_dir_available_threshold,omitempty"`
	MemoryPSIThresholdText         string   `json:"memory_psi_threshold,omitempty"`
	MemoryPSI60ThresholdText       string   `json:"memory_psi60_threshold,omitempty"`
	CPUPSIThresholdText            string   `json:"cpu_psi_threshold,omitempty"`
	IOPSIThresholdText             string   `json:"io_psi_threshold,omitempty"`
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
	Sustained  *SustainedPressureState      `json:"sustained,omitempty"`
	Inventory  PressureReclaimInventory     `json:"inventory"`
	Candidates []PressureReclaimCandidate   `json:"candidates,omitempty"`
	Config     PressureReclaimConfigSummary `json:"config"`
}

type PressureReclaimConfigSummary struct {
	MinIdleSeconds              int64   `json:"min_idle_seconds"`
	MinMemoryAvailableBytes     uint64  `json:"min_memory_available_bytes,omitempty"`
	MinMemoryAvailablePercent   float64 `json:"min_memory_available_percent,omitempty"`
	MinStateDirAvailableBytes   uint64  `json:"min_state_dir_available_bytes,omitempty"`
	MinStateDirAvailablePercent float64 `json:"min_state_dir_available_percent,omitempty"`
	MaxMemorySomeAvg10          float64 `json:"max_memory_some_avg10,omitempty"`
	MaxCPUSomeAvg10             float64 `json:"max_cpu_some_avg10,omitempty"`
	MaxIOSomeAvg10              float64 `json:"max_io_some_avg10,omitempty"`
	MaxCandidates               int     `json:"max_candidates"`
	StaleStateMinAgeSeconds     int64   `json:"stale_state_min_age_seconds,omitempty"`
	MaxStateDeletes             int     `json:"max_state_deletes,omitempty"`
	StateDir                    string  `json:"state_dir,omitempty"`
	Signal                      string  `json:"signal,omitempty"`
	MaxMemorySomeAvg60          float64 `json:"max_memory_some_avg60,omitempty"`
	SustainedSamples            int     `json:"sustained_samples,omitempty"`
	SampleIntervalSeconds       int64   `json:"sample_interval_seconds,omitempty"`
	CooldownSeconds             int64   `json:"cooldown_seconds,omitempty"`
}

type hostPressureSampler func(PressureReclaimConfig) HostPressureSample

func DefaultPressureReclaimConfig() PressureReclaimConfig {
	return PressureReclaimConfig{
		Mode:                        PressureReclaimModeOff,
		MinIdle:                     30 * time.Minute,
		MinMemoryAvailableBytes:     2 * 1024 * 1024 * 1024,
		MinMemoryAvailablePercent:   15,
		MinStateDirAvailableBytes:   0,
		MinStateDirAvailablePercent: 0,
		MaxMemorySomeAvg10:          1.0,
		MaxCPUSomeAvg10:             90.0,
		MaxIOSomeAvg10:              5.0,
		StateDir:                    "/var/lib/go-choir/vm-state",
		MaxCandidates:               5,
		StaleStateMinAge:            6 * time.Hour,
		MaxStateDeletes:             5,
		Signal:                      PressureReclaimSignalSustained,
		MaxMemorySomeAvg60:          defaultMaxMemorySomeAvg60,
		SustainedSamples:            defaultSustainedSamples,
		SampleInterval:              defaultPressureSample,
		Cooldown:                    defaultReclaimCooldown,
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
	if cfg.StaleStateMinAge <= 0 {
		cfg.StaleStateMinAge = 6 * time.Hour
	}
	if cfg.MaxStateDeletes <= 0 {
		cfg.MaxStateDeletes = cfg.MaxCandidates
	}
	if strings.TrimSpace(cfg.StateDir) == "" {
		cfg.StateDir = "/var/lib/go-choir/vm-state"
	}
	cfg.Signal = strings.TrimSpace(strings.ToLower(cfg.Signal))
	switch cfg.Signal {
	case PressureReclaimSignalInstant, "legacy", "avg10":
		cfg.Signal = PressureReclaimSignalInstant
	default:
		cfg.Signal = PressureReclaimSignalSustained
	}
	if cfg.MaxMemorySomeAvg60 <= 0 {
		cfg.MaxMemorySomeAvg60 = defaultMaxMemorySomeAvg60
	}
	if cfg.SustainedSamples <= 0 {
		cfg.SustainedSamples = defaultSustainedSamples
	}
	if cfg.SampleInterval <= 0 {
		cfg.SampleInterval = defaultPressureSample
	}
	if cfg.Cooldown <= 0 {
		cfg.Cooldown = defaultReclaimCooldown
	}
	return cfg
}

// memoryFloorBytes is the host memory floor: the larger of the byte and
// percent thresholds. Reclaim and admission share it.
func memoryFloorBytes(cfg PressureReclaimConfig, totalBytes uint64) uint64 {
	floor := cfg.MinMemoryAvailableBytes
	if cfg.MinMemoryAvailablePercent > 0 && totalBytes > 0 {
		if pct := uint64(float64(totalBytes) * cfg.MinMemoryAvailablePercent / 100); pct > floor {
			floor = pct
		}
	}
	return floor
}

func pressureConfigSummary(cfg PressureReclaimConfig) PressureReclaimConfigSummary {
	return PressureReclaimConfigSummary{
		MinIdleSeconds:              int64(cfg.MinIdle.Seconds()),
		MinMemoryAvailableBytes:     cfg.MinMemoryAvailableBytes,
		MinMemoryAvailablePercent:   cfg.MinMemoryAvailablePercent,
		MinStateDirAvailableBytes:   cfg.MinStateDirAvailableBytes,
		MinStateDirAvailablePercent: cfg.MinStateDirAvailablePercent,
		MaxMemorySomeAvg10:          cfg.MaxMemorySomeAvg10,
		MaxCPUSomeAvg10:             cfg.MaxCPUSomeAvg10,
		MaxIOSomeAvg10:              cfg.MaxIOSomeAvg10,
		MaxCandidates:               cfg.MaxCandidates,
		StaleStateMinAgeSeconds:     int64(cfg.StaleStateMinAge.Seconds()),
		MaxStateDeletes:             cfg.MaxStateDeletes,
		StateDir:                    cfg.StateDir,
		Signal:                      cfg.Signal,
		MaxMemorySomeAvg60:          cfg.MaxMemorySomeAvg60,
		SustainedSamples:            cfg.SustainedSamples,
		SampleIntervalSeconds:       int64(cfg.SampleInterval.Seconds()),
		CooldownSeconds:             int64(cfg.Cooldown.Seconds()),
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
	if avg, err := readPressureAverages("/proc/pressure/memory"); err != nil {
		errs = append(errs, err.Error())
	} else {
		sample.MemorySomeAvg10 = avg.some10
		sample.MemoryFullAvg10 = avg.full10
		sample.MemorySomeAvg60 = avg.some60
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
	sample.StateDirPressure = false
	sample.Pressure = false
	sample.ReclaimPressure = false
	sample.Signals = nil
	sample.MemoryAvailableThresholdText = ""
	sample.StateDirAvailableThresholdText = ""
	sample.MemoryPSIThresholdText = ""
	sample.MemoryPSI60ThresholdText = ""
	sample.ReclaimSignal = ""
	sample.MemoryFloorBytes = memoryFloorBytes(cfg, sample.MemoryTotalBytes)
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
	if cfg.MinStateDirAvailableBytes > 0 {
		sample.StateDirAvailableThresholdText = fmt.Sprintf("available_bytes<%d", cfg.MinStateDirAvailableBytes)
		if sample.StateDirAvailableBytes > 0 && sample.StateDirAvailableBytes < cfg.MinStateDirAvailableBytes {
			sample.StateDirPressure = true
		}
	}
	if cfg.MinStateDirAvailablePercent > 0 {
		threshold := fmt.Sprintf("available_percent<%.2f", cfg.MinStateDirAvailablePercent)
		if sample.StateDirAvailableThresholdText == "" {
			sample.StateDirAvailableThresholdText = threshold
		} else {
			sample.StateDirAvailableThresholdText += "," + threshold
		}
		if sample.StateDirAvailablePercent > 0 && sample.StateDirAvailablePercent < cfg.MinStateDirAvailablePercent {
			sample.StateDirPressure = true
		}
	}
	for _, signal := range []struct {
		name string
		on   bool
	}{{"memory", sample.MemoryPressure}, {"state_dir", sample.StateDirPressure}, {"cpu", sample.CPUPressure}, {"io", sample.IOPressure}} {
		if signal.on {
			sample.Signals = append(sample.Signals, signal.name)
		}
	}
	sample.Pressure = len(sample.Signals) > 0
	sample.ReclaimPressure = sample.MemoryPressure || sample.StateDirPressure
	switch cfg.Signal {
	case PressureReclaimSignalSustained:
		// Only a shortage that stalls counts: memory under the floor AND
		// avg60 over its threshold. An avg10 burst with memory to spare
		// (a du, a nix build, our own snapshot write) never does.
		sample.ReclaimSignal = PressureReclaimSignalSustained
		sample.MemoryPSI60ThresholdText = fmt.Sprintf("memory_some_avg60>%.2f&&available_bytes<%d", cfg.MaxMemorySomeAvg60, sample.MemoryFloorBytes)
		belowFloor := sample.MemoryAvailableBytes > 0 && sample.MemoryAvailableBytes < sample.MemoryFloorBytes
		memoryQualifies := belowFloor && sample.MemorySomeAvg60 > cfg.MaxMemorySomeAvg60
		sample.ReclaimPressure = memoryQualifies || sample.StateDirPressure
	case PressureReclaimSignalInstant:
		sample.ReclaimSignal = PressureReclaimSignalInstant
	}
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
	avg, err := readPressureAverages(path)
	return avg.some10, avg.full10, err
}

type pressureAverages struct {
	some10, full10, some60 float64
}

func readPressureAverages(path string) (pressureAverages, error) {
	var avg pressureAverages
	file, err := os.Open(path)
	if err != nil {
		return avg, fmt.Errorf("read %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "some":
			avg.some10 = parseAvg(fields[1:], "avg10")
			avg.some60 = parseAvg(fields[1:], "avg60")
		case "full":
			avg.full10 = parseAvg(fields[1:], "avg10")
		}
	}
	if err := scanner.Err(); err != nil {
		return avg, fmt.Errorf("scan %s: %w", path, err)
	}
	return avg, nil
}

func parseAvg10(fields []string) float64 { return parseAvg(fields, "avg10") }

func parseAvg(fields []string, name string) float64 {
	for _, field := range fields {
		key, value, ok := strings.Cut(field, "=")
		if !ok || key != name {
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
	return r.pressureReclaimPlan(r.hibernationHold(r.guestBusy(), r.now))
}

// PressureReclaimPlanSeen is the reclaim plan from the guests' last recorded
// busy answers, without probing any guest. For health only.
func (r *OwnershipRegistry) PressureReclaimPlanSeen() PressureReclaimPlan {
	return r.pressureReclaimPlan(r.hibernationHold(r.guestBusySeenAnswer(), r.now))
}

func (r *OwnershipRegistry) pressureReclaimPlan(busy func(*VMOwnership) bool) PressureReclaimPlan {
	r.mu.RLock()
	cfg := r.pressureReclaim
	warmnessPolicy := r.warmnessPolicy
	sampler := r.pressureSampler
	ownerships := make([]*VMOwnership, 0, len(r.ownerships))
	for _, own := range r.ownerships {
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

	now := r.now()
	candidates := rankPressureCandidates(ownerships, cfg, warmnessPolicy, now, busy)
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
	reclaimNow := sample.ReclaimPressure
	if cfg.Signal == PressureReclaimSignalSustained {
		// The decision is the monitor's window, not this one sample.
		st := r.SustainedPressureState()
		plan.Sustained = &st
		reclaimNow = st.Sustained
	}
	if reclaimNow && cfg.Mode == PressureReclaimModeActive {
		if plan.Sustained != nil && plan.Sustained.Hold != "" {
			plan.Decision = "hold"
			plan.Reason = "sustained host pressure; reclaim waits: " + plan.Sustained.Hold
			return plan
		}
		plan.Decision = "reclaim"
		plan.Reason = "host pressure crossed active threshold; eligible VMs may be hibernated"
		return plan
	}
	if reclaimNow {
		plan.Decision = "would_reclaim"
		plan.Reason = "host pressure crossed dry-run threshold; no VM hibernated"
		return plan
	}
	plan.Decision = "observe"
	plan.Reason = "host pressure below reclaim threshold"
	if sample.Pressure {
		plan.Reason = "cpu or io stall reported; reclaim answers memory and disk pressure only"
	}
	if plan.Sustained != nil && plan.Sustained.Streak > 0 {
		plan.Reason = fmt.Sprintf("memory short and stalling, not yet sustained (streak %d/%d)", plan.Sustained.Streak, plan.Sustained.RequiredSamples)
	}
	return plan
}

func (r *OwnershipRegistry) pressureReclaimActionCandidates() []pressureCandidateInternal {
	r.mu.RLock()
	cfg := r.pressureReclaim
	warmnessPolicy := r.warmnessPolicy
	sampler := r.pressureSampler
	ownerships := make([]*VMOwnership, 0, len(r.ownerships))
	for _, own := range r.ownerships {
		ownerships = append(ownerships, cloneOwnership(own))
	}
	r.mu.RUnlock()

	cfg = normalizePressureReclaimConfig(cfg)
	if cfg.Mode != PressureReclaimModeActive {
		return nil
	}
	limit := cfg.MaxCandidates
	if cfg.Signal == PressureReclaimSignalSustained {
		// The monitor's window decides, and one computer per reclaim.
		if !r.SustainedPressureState().Sustained {
			return nil
		}
		limit = 1
	} else {
		if sampler == nil {
			sampler = sampleHostPressure
		}
		sample := sampler(cfg)
		annotatePressure(&sample, cfg)
		if !sample.ReclaimPressure {
			return nil
		}
	}

	ranked := rankPressureCandidates(ownerships, cfg, warmnessPolicy, r.now(), r.hibernationHold(r.guestBusy(), r.now))
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
		inv.Interactive++
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

func rankPressureCandidates(ownerships []*VMOwnership, cfg PressureReclaimConfig, warmnessPolicy WarmnessPolicyConfig, now time.Time, busyProbe func(*VMOwnership) bool) []pressureCandidateInternal {
	candidates := make([]pressureCandidateInternal, 0, len(ownerships))
	for _, own := range ownerships {
		if own == nil || own.State != VMStateActive {
			continue
		}
		candidates = append(candidates, pressureCandidateForOwnership(own, cfg, warmnessPolicy, now, busyProbe))
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

func pressureCandidateForOwnership(own *VMOwnership, cfg PressureReclaimConfig, warmnessPolicy WarmnessPolicyConfig, now time.Time, busyProbe func(*VMOwnership) bool) pressureCandidateInternal {
	idle := time.Duration(0)
	if !own.LastActiveAt.IsZero() {
		idle = now.Sub(own.LastActiveAt)
	}
	warmnessClass := warmnessClassForOwnership(own, warmnessPolicy)
	reasons := protectedReclaimReasons(own, cfg, warmnessPolicy, idle, busyProbe)
	priority := warmnessPriority(warmnessClass)
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

func protectedReclaimReasons(own *VMOwnership, cfg PressureReclaimConfig, warmnessPolicy WarmnessPolicyConfig, idle time.Duration, busyProbe func(*VMOwnership) bool) []string {
	var reasons []string
	if own == nil {
		return []string{"missing_ownership"}
	}
	if own.IsHeld() {
		// A held computer is never reclaimed under memory pressure.
		return []string{"maintenance_hold"}
	}
	switch warmnessClassForOwnership(own, warmnessPolicy) {
	case WarmnessClassPremiumAlwaysOn:
		reasons = append(reasons, "premium_always_on")
	case WarmnessClassCriticalProtected:
		reasons = append(reasons, "critical_protected")
	}
	if own.LastActiveAt.IsZero() {
		reasons = append(reasons, "unknown_last_active")
	}
	// A test computer yields to real ones: recent activity does not
	// protect it, only a busy guest does.
	if cfg.MinIdle > 0 && idle < cfg.MinIdle && warmnessClassForOwnership(own, warmnessPolicy) != WarmnessClassTest {
		reasons = append(reasons, "recent_activity")
	}
	if busyProbe != nil && busyProbe(own) {
		// A guest with in-flight runs is doing work — LastActiveAt under-reads
		// it. Never reclaim a live-desk computer under host pressure.
		reasons = append(reasons, "guest_busy")
	}
	return reasons
}
