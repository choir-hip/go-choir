package platformrelease

import (
	"time"
)

const (
	// CurrentSchemaVersion is the schema version for PlatformRelease.
	CurrentSchemaVersion = "choir.platform_release.v1"

	// DivergenceTrack classifies component lineage tracking.
	TrackPlatform = "platform"
	TrackUser     = "user"

	// DivergenceStatus classifies a computer's divergence from the platform baseline.
	DivergenceTracking  = "tracking"
	DivergenceDivergent = "divergent"
	DivergenceCanary    = "canary"

	// PlatformFollowPolicy controls how a computer handles upstream platform updates.
	FollowPolicyAuto     = "auto"     // auto-fast-forward non-divergent ledgers
	FollowPolicyProposal = "proposal" // generate proposal (PR) for user review
	FollowPolicyPinned   = "pinned"   // ignore platform updates; stay at current version
)

// ComponentRelease represents a single versioned component in the platform release.
type ComponentRelease struct {
	ComponentID        string `json:"component_id"`        // e.g. "desktop_shell", "email", "texture", "settings", "maild", "autoputer"
	Version            string `json:"version"`             // semver or build tag
	ArtifactRef        string `json:"artifact_ref"`        // content-addressed digest (e.g. sha256:...) or git tree SHA
	DivergenceTrack    string `json:"divergence_track"`    // "platform" | "user"
	CompatibilityRange string `json:"compatibility_range"` // semver range, e.g. ">=v1.0.0"
}

// PlatformRelease represents an immutable, content-addressed release
// of the Choir platform baseline (P1).
type PlatformRelease struct {
	SchemaVersion               string             `json:"schema_version"`
	ReleaseID                   string             `json:"release_id"`                     // e.g. "pr-20260908-653f0105"
	PlatformBaseRef             string             `json:"platform_base_ref"`              // canonical git ref (e.g. "main@653f0105")
	CreatedAt                   time.Time          `json:"created_at"`
	GuestKernelDigest           string             `json:"guest_kernel_digest,omitempty"`
	GuestImageDigest            string             `json:"guest_image_digest,omitempty"`
	AutoputerRuntimeDigest      string             `json:"autoputer_runtime_digest,omitempty"`
	FrontendAssetManifestDigest string             `json:"frontend_asset_manifest_digest,omitempty"`
	FrontendAssetsRef           string             `json:"frontend_assets_ref,omitempty"` // commit SHA or tree digest
	MaildSchemaVersion          int                `json:"maild_schema_version,omitempty"`
	EventSchemaVersion          uint64             `json:"event_schema_version"`
	ReducerVersion              uint64             `json:"reducer_version"`
	Components                  []ComponentRelease `json:"components"`
	ContentDigest               string             `json:"content_digest,omitempty"`
}

// ComputerLineage represents a persistent computer's relationship to platform releases.
type ComputerLineage struct {
	ComputerID           string    `json:"computer_id"`
	PlatformBaseRef      string    `json:"platform_base_ref"` // platform release the computer was based on
	DivergenceStatus     string    `json:"divergence_status"` // "tracking" | "divergent" | "canary"
	LastRebasedAt        time.Time `json:"last_rebased_at,omitempty"`
	DivergedComponents   []string  `json:"diverged_components,omitempty"`
	PlatformFollowPolicy string    `json:"platform_follow_policy"` // "auto" | "proposal" | "pinned"
}

// IsTracking returns true if the computer is on the clean tracking path and can
// automatically fast-forward non-divergent platform improvements.
func (l *ComputerLineage) IsTracking() bool {
	if l == nil {
		return false
	}
	return l.DivergenceStatus == DivergenceTracking && len(l.DivergedComponents) == 0 && l.PlatformFollowPolicy == FollowPolicyAuto
}

// CanFastForwardComponent returns true if a specific component has not diverged
// and can be fast-forwarded to a new platform release.
func (l *ComputerLineage) CanFastForwardComponent(componentID string) bool {
	if l == nil {
		return false
	}
	if l.DivergenceStatus == DivergenceCanary || l.PlatformFollowPolicy == FollowPolicyPinned {
		return false
	}
	for _, diverged := range l.DivergedComponents {
		if diverged == componentID {
			return false
		}
	}
	return true
}
