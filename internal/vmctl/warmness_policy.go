package vmctl

import (
	"sort"
	"strings"
	"time"
)

type WarmnessClass string

const (
	WarmnessClassPublicPlatform    WarmnessClass = "public_platform"
	WarmnessClassPrimary           WarmnessClass = "primary"
	WarmnessClassPremiumAlwaysOn   WarmnessClass = "premium_always_on"
	WarmnessClassCriticalProtected WarmnessClass = "critical_protected"
	// WarmnessClassTest is a computer of a test account (the ephemeral
	// accounts retention already recognizes): below the free tier, the
	// first reclaimed under memory pressure.
	WarmnessClassTest WarmnessClass = "test"
)

const (
	PrimaryKeepaliveModeOff           = "off"
	PrimaryKeepaliveModeUnderCapacity = "under-capacity"
)

// WarmnessPolicyConfig carries the computer tiers (internal/computertiers):
// paid accounts, whose computers are always on, and platform computers, a
// tier above every user tier (owner ruling 2026-10-13).
type WarmnessPolicyConfig struct {
	PrimaryKeepaliveMode string
	PaidUserIDs          map[string]bool
	PlatformComputerIDs  map[string]bool
	// TestUserIDs is refreshed from the auth DB by the idle sweep
	// (refreshTestAccounts), not configured.
	TestUserIDs map[string]bool
	// TestComputerIDs names test computers of registered accounts
	// (internal/computertiers test_computers). A platform computer or a
	// paid account's computer listed here keeps its own class.
	TestComputerIDs map[string]bool
}

type WarmnessPolicyConfigSummary struct {
	PrimaryKeepaliveMode  string `json:"primary_keepalive_mode"`
	PaidUserCount         int    `json:"paid_user_count,omitempty"`
	PlatformComputerCount int    `json:"platform_computer_count,omitempty"`
}

type WarmnessHealthSummary struct {
	Policy              WarmnessPolicyConfigSummary `json:"policy"`
	ByClass             map[string]int              `json:"by_class,omitempty"`
	ActiveByClass       map[string]int              `json:"active_by_class,omitempty"`
	IdleEligibleByClass map[string]int              `json:"idle_eligible_by_class,omitempty"`
}

type idleOwnershipCandidate struct {
	own      *VMOwnership
	class    WarmnessClass
	priority int
	idle     time.Duration
}

func DefaultWarmnessPolicyConfig() WarmnessPolicyConfig {
	return WarmnessPolicyConfig{
		PrimaryKeepaliveMode: PrimaryKeepaliveModeOff,
		PaidUserIDs:          map[string]bool{},
		PlatformComputerIDs:  map[string]bool{},
	}
}

func normalizeWarmnessPolicyConfig(cfg WarmnessPolicyConfig) WarmnessPolicyConfig {
	mode := strings.TrimSpace(strings.ToLower(cfg.PrimaryKeepaliveMode))
	switch mode {
	case "", PrimaryKeepaliveModeOff:
		cfg.PrimaryKeepaliveMode = PrimaryKeepaliveModeOff
	case "under_capacity", "under capacity", "keep-primary", "keep_primary", PrimaryKeepaliveModeUnderCapacity:
		cfg.PrimaryKeepaliveMode = PrimaryKeepaliveModeUnderCapacity
	default:
		cfg.PrimaryKeepaliveMode = PrimaryKeepaliveModeOff
	}
	cfg.PaidUserIDs = normalizedIDSet(cfg.PaidUserIDs)
	cfg.PlatformComputerIDs = normalizedIDSet(cfg.PlatformComputerIDs)
	cfg.TestUserIDs = normalizedIDSet(cfg.TestUserIDs)
	cfg.TestComputerIDs = normalizedIDSet(cfg.TestComputerIDs)
	return cfg
}

func normalizedIDSet(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for id, enabled := range in {
		id = strings.TrimSpace(id)
		if id != "" && enabled {
			out[id] = true
		}
	}
	return out
}

func warmnessPolicySummary(cfg WarmnessPolicyConfig) WarmnessPolicyConfigSummary {
	cfg = normalizeWarmnessPolicyConfig(cfg)
	return WarmnessPolicyConfigSummary{
		PrimaryKeepaliveMode:  cfg.PrimaryKeepaliveMode,
		PaidUserCount:         len(cfg.PaidUserIDs),
		PlatformComputerCount: len(cfg.PlatformComputerIDs),
	}
}

// warmnessClassForOwnership derives a computer's class. A platform
// computer (by computer id, or a computer-level class persisted on the
// ownership) is critical_protected or public_platform. A paid account's
// computer is premium_always_on, read live from the tiers so a downgrade
// takes effect; an account-derived class persisted on an older ownership is
// ignored. A test account's computer, or a computer computer-tiers.json
// names as a test computer, is test. Everything else is primary.
func warmnessClassForOwnership(own *VMOwnership, cfg WarmnessPolicyConfig) WarmnessClass {
	if own == nil {
		return ""
	}
	cfg = normalizeWarmnessPolicyConfig(cfg)
	if cfg.PlatformComputerIDs[strings.TrimSpace(own.ComputerID)] {
		return WarmnessClassCriticalProtected
	}
	if class := computerLevelClass(own.WarmnessClass); class != "" {
		return class
	}
	if cfg.PaidUserIDs[strings.TrimSpace(own.UserID)] {
		return WarmnessClassPremiumAlwaysOn
	}
	if cfg.TestUserIDs[strings.TrimSpace(own.UserID)] || cfg.TestComputerIDs[strings.TrimSpace(own.ComputerID)] {
		return WarmnessClassTest
	}
	return WarmnessClassPrimary
}

// computerLevelClass returns class when it belongs to the computer itself
// (a platform computer), and "" when it would be derived from the account.
func computerLevelClass(class WarmnessClass) WarmnessClass {
	switch class {
	case WarmnessClassCriticalProtected, WarmnessClassPublicPlatform:
		return class
	}
	return ""
}

func warmnessPriority(class WarmnessClass) int {
	switch class {
	case WarmnessClassTest:
		return 10
	case WarmnessClassPrimary:
		return 20
	case WarmnessClassPublicPlatform:
		return 30
	case WarmnessClassPremiumAlwaysOn:
		return 90
	case WarmnessClassCriticalProtected:
		return 100
	default:
		return 50
	}
}

func warmnessClassProtected(class WarmnessClass) bool {
	return class == WarmnessClassPublicPlatform ||
		class == WarmnessClassPremiumAlwaysOn ||
		class == WarmnessClassCriticalProtected
}

func warmnessSummary(cfg WarmnessPolicyConfig, ownerships []*VMOwnership, idleEligible []*VMOwnership) WarmnessHealthSummary {
	cfg = normalizeWarmnessPolicyConfig(cfg)
	summary := WarmnessHealthSummary{
		Policy:              warmnessPolicySummary(cfg),
		ByClass:             map[string]int{},
		ActiveByClass:       map[string]int{},
		IdleEligibleByClass: map[string]int{},
	}
	for _, own := range ownerships {
		if own == nil {
			continue
		}
		class := string(warmnessClassForOwnership(own, cfg))
		if class == "" {
			continue
		}
		summary.ByClass[class]++
		if own.State == VMStateActive {
			summary.ActiveByClass[class]++
		}
	}
	for _, own := range idleEligible {
		if own == nil {
			continue
		}
		class := string(warmnessClassForOwnership(own, cfg))
		if class == "" {
			continue
		}
		summary.IdleEligibleByClass[class]++
	}
	return summary
}

func idleOwnershipCandidates(ownerships []*VMOwnership, cfg WarmnessPolicyConfig, pressure HostPressureSample, idleTimeout time.Duration, now time.Time, busyProbe func(*VMOwnership) bool) []idleOwnershipCandidate {
	cfg = normalizeWarmnessPolicyConfig(cfg)
	candidates := make([]idleOwnershipCandidate, 0, len(ownerships))
	for _, own := range ownerships {
		if own == nil || own.State != VMStateActive {
			continue
		}
		if own.IsHeld() {
			// A held computer is never idle-stopped/hibernated by the sweeper.
			continue
		}
		if own.LastActiveAt.IsZero() {
			continue
		}
		idle := now.Sub(own.LastActiveAt)
		if idle <= idleTimeout {
			continue
		}
		// A guest with in-flight runs is not idle — LastActiveAt tracks
		// ownership-resolution traffic only, so a computer mid-desk-run reads
		// stale here. Consult the guest's own busy signal before candidacy;
		// receipt: docs/problems/vmctl-idle-sweep-hibernates-busy-guest-2026-09-28.md
		if busyProbe != nil && busyProbe(own) {
			continue
		}
		class := warmnessClassForOwnership(own, cfg)
		if warmnessClassProtected(class) {
			continue
		}
		if class == WarmnessClassPrimary && cfg.PrimaryKeepaliveMode == PrimaryKeepaliveModeUnderCapacity && !pressure.ReclaimPressure {
			continue
		}
		candidates = append(candidates, idleOwnershipCandidate{
			own:      own,
			class:    class,
			priority: warmnessPriority(class),
			idle:     idle,
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if left.priority != right.priority {
			return left.priority < right.priority
		}
		if left.idle != right.idle {
			return left.idle > right.idle
		}
		return left.own.VMID < right.own.VMID
	})
	if cfg.PrimaryKeepaliveMode == PrimaryKeepaliveModeUnderCapacity && pressure.ReclaimPressure {
		hasLowerPriority := false
		for _, candidate := range candidates {
			if candidate.class != WarmnessClassPrimary {
				hasLowerPriority = true
				break
			}
		}
		if hasLowerPriority {
			filtered := candidates[:0]
			for _, candidate := range candidates {
				if candidate.class != WarmnessClassPrimary {
					filtered = append(filtered, candidate)
				}
			}
			candidates = filtered
		}
	}
	return candidates
}
