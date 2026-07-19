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
)

const (
	PrimaryKeepaliveModeOff           = "off"
	PrimaryKeepaliveModeUnderCapacity = "under-capacity"
)

type WarmnessPolicyConfig struct {
	PrimaryKeepaliveMode string
	AlwaysOnUserIDs      map[string]bool
}

type WarmnessPolicyConfigSummary struct {
	PrimaryKeepaliveMode string `json:"primary_keepalive_mode"`
	AlwaysOnUserCount    int    `json:"always_on_user_count,omitempty"`
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
		AlwaysOnUserIDs:      map[string]bool{},
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
	if cfg.AlwaysOnUserIDs == nil {
		cfg.AlwaysOnUserIDs = map[string]bool{}
	}
	normalized := make(map[string]bool, len(cfg.AlwaysOnUserIDs))
	for userID, enabled := range cfg.AlwaysOnUserIDs {
		userID = strings.TrimSpace(userID)
		if userID != "" && enabled {
			normalized[userID] = true
		}
	}
	cfg.AlwaysOnUserIDs = normalized
	return cfg
}

func warmnessPolicySummary(cfg WarmnessPolicyConfig) WarmnessPolicyConfigSummary {
	cfg = normalizeWarmnessPolicyConfig(cfg)
	return WarmnessPolicyConfigSummary{
		PrimaryKeepaliveMode: cfg.PrimaryKeepaliveMode,
		AlwaysOnUserCount:    len(cfg.AlwaysOnUserIDs),
	}
}

func warmnessClassForOwnership(own *VMOwnership, cfg WarmnessPolicyConfig) WarmnessClass {
	if own == nil {
		return ""
	}
	cfg = normalizeWarmnessPolicyConfig(cfg)
	if cfg.AlwaysOnUserIDs[strings.TrimSpace(own.UserID)] {
		return WarmnessClassPremiumAlwaysOn
	}
	if own.WarmnessClass != "" {
		switch own.WarmnessClass {
		case WarmnessClassPremiumAlwaysOn, WarmnessClassPrimary, WarmnessClassCriticalProtected, WarmnessClassPublicPlatform:
			return own.WarmnessClass
		}
	}
	return WarmnessClassPrimary
}

func warmnessPriority(class WarmnessClass) int {
	switch class {
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

func idleOwnershipCandidates(ownerships []*VMOwnership, cfg WarmnessPolicyConfig, pressure HostPressureSample, idleTimeout time.Duration, now time.Time) []idleOwnershipCandidate {
	cfg = normalizeWarmnessPolicyConfig(cfg)
	candidates := make([]idleOwnershipCandidate, 0, len(ownerships))
	for _, own := range ownerships {
		if own == nil || own.State != VMStateActive {
			continue
		}
		if own.LastActiveAt.IsZero() {
			continue
		}
		idle := now.Sub(own.LastActiveAt)
		if idle <= idleTimeout {
			continue
		}
		class := warmnessClassForOwnership(own, cfg)
		if warmnessClassProtected(class) {
			continue
		}
		if class == WarmnessClassPrimary && cfg.PrimaryKeepaliveMode == PrimaryKeepaliveModeUnderCapacity && !pressure.Pressure {
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
	if cfg.PrimaryKeepaliveMode == PrimaryKeepaliveModeUnderCapacity && pressure.Pressure {
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
