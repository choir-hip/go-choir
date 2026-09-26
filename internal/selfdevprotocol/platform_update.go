package selfdevprotocol

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/platformrelease"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// Platform-update kinds ride the existing AuthorityReceipt wire shape: the
// platform-control signer signs a commitment to the offer artifact, and the
// guest counter-verifies. The evidence class stays distinct from
// checkpoint/route_projection so a forged selfdev receipt cannot mint an update.
const (
	ReceiptKindPlatformUpdate = "platform_update"

	// PlatformUpdateFollowActor/Scope bind the platform-follow evidence class
	// in route-projection requests. The owner self-dev path uses owner actor +
	// "computer:self_development:*" scopes; platform updates carry neither.
	PlatformUpdateFollowActor = "platform"
	PlatformUpdateFollowScope = "computer:platform_follow:update"
)

// PlatformUpdateFile is one signed file payload in an update offer. Bytes are
// base64 for transport; SHA256 is the plaintext digest and must match the
// manifest entry for Path.
type PlatformUpdateFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode"`
	Bytes  string `json:"bytes"` // base64
}

// PlatformUpdateOffer is the platform's signed instruction to a tracking
// computer: apply this release under a platform-attested lineage policy, bound
// to a canonical head and an expiry. The signature is the platform-control
// domain's receipt over the offer digest; no owner decision is minted and none
// is implied.
type PlatformUpdateOffer struct {
	Version     int    `json:"version"`
	ComputerID  string `json:"computer_id"`
	UpdateID    string `json:"update_id"` // idempotency key + canonical event identity
	Realization string `json:"realization_id"`

	// The release to apply. Manifest is the updater manifest *before* the
	// accepted-head finalize (AcceptedEventHead may be empty; the guest fills
	// it after committing the accepted event). ContentDigest must be present —
	// it is the release identity the updater's trusted store binds.
	Manifest updater.ReleaseManifest `json:"manifest"`
	Files    []PlatformUpdateFile    `json:"files"`

	// CodeClosure/ArtifactProgram are the route-slot versions promoted to.
	CodeClosure     computerversion.CodeClosure     `json:"code_closure"`
	ArtifactProgram computerversion.ArtifactProgram `json:"artifact_program"`

	// VerifierRefs are the platform-attached verification evidence for the
	// update; bound into the accepted event + the verifier certificate.
	VerifierRefs []string `json:"verifier_refs"`

	// LineageAssertion is the platform's signed claim about this computer's
	// tracking posture. The guest enforces it: only auto/tracking/non-divergent
	// computers may fast-forward.
	DivergenceStatus     string   `json:"divergence_status"`
	PlatformFollowPolicy string   `json:"platform_follow_policy"`
	DivergedComponents   []string `json:"diverged_components,omitempty"`

	// BaseEventHead binds the offer to a canonical head; a guest that has
	// moved past it refuses rather than apply over a stale base.
	BaseEventHead string `json:"base_event_head"`
	ExpiresAt     string `json:"expires_at"`

	Authorization AuthorityReceipt `json:"authorization"`
}

// PlatformUpdateOfferFromRequest validates and binds an incoming signed offer.
// It does not verify the signature — the caller does that against the
// platform-control public key — it checks structure, expiry, and internal
// joins.
func PlatformUpdateOfferFromRequest(offer PlatformUpdateOffer, now time.Time) error {
	if offer.Version != 1 || strings.TrimSpace(offer.ComputerID) == "" || strings.TrimSpace(offer.UpdateID) == "" ||
		strings.TrimSpace(offer.Realization) == "" {
		return fmt.Errorf("platform update offer: version, computer, update id, and realization are required")
	}
	if offer.Manifest.ComputerID != offer.ComputerID {
		return fmt.Errorf("platform update offer: manifest computer mismatch")
	}
	if !computerevent.IsSHA256(offer.Manifest.ContentDigest) {
		return fmt.Errorf("platform update offer: manifest content digest is required")
	}
	if offer.Manifest.CodeRef == "" || offer.Manifest.ArtifactProgramRef == "" {
		return fmt.Errorf("platform update offer: manifest version refs are required")
	}
	if string(offer.CodeClosure.Ref) != offer.Manifest.CodeRef || string(offer.ArtifactProgram.Ref) != offer.Manifest.ArtifactProgramRef {
		return fmt.Errorf("platform update offer: closure/program join mismatch")
	}
	if offer.CodeClosure.Verify() != nil || offer.ArtifactProgram.Verify() != nil {
		return fmt.Errorf("platform update offer: code closure or artifact program invalid")
	}
	if len(offer.VerifierRefs) == 0 {
		return fmt.Errorf("platform update offer: verifier evidence required")
	}
	// Platform-follow policy gate: only auto + tracking + non-divergent.
	if offer.PlatformFollowPolicy != platformrelease.FollowPolicyAuto ||
		offer.DivergenceStatus != platformrelease.DivergenceTracking ||
		len(offer.DivergedComponents) != 0 {
		return fmt.Errorf("platform update offer: lineage is not a clean auto-tracking computer")
	}
	if !computerevent.IsSHA256(offer.BaseEventHead) {
		return fmt.Errorf("platform update offer: base event head is required")
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, offer.ExpiresAt)
	if err != nil || expiresAt.Location() != time.UTC || !expiresAt.After(now.UTC()) || expiresAt.After(now.UTC().Add(5*time.Minute)) {
		return fmt.Errorf("platform update offer: short canonical expiry is required")
	}
	// Manifest files must be covered by the signed payload — no silent subset.
	manifestFiles := make(map[string]updater.ManifestFile, len(offer.Manifest.Files))
	for _, f := range offer.Manifest.Files {
		manifestFiles[f.Path] = f
	}
	if len(offer.Files) != len(manifestFiles) {
		return fmt.Errorf("platform update offer: payload must cover every manifest file")
	}
	seen := make(map[string]bool, len(offer.Files))
	for _, file := range offer.Files {
		entry, ok := manifestFiles[file.Path]
		if !ok || seen[file.Path] {
			return fmt.Errorf("platform update offer: payload path %q is not in the manifest", file.Path)
		}
		seen[file.Path] = true
		if entry.SHA256 != file.SHA256 || entry.Mode != file.Mode {
			return fmt.Errorf("platform update offer: payload file %q does not match the manifest", file.Path)
		}
		raw, err := base64.StdEncoding.DecodeString(file.Bytes)
		if err != nil {
			return fmt.Errorf("platform update offer: payload file %q does not decode", file.Path)
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != file.SHA256 {
			return fmt.Errorf("platform update offer: payload file %q digest mismatch", file.Path)
		}
	}
	return nil
}

// OfferDigest computes the canonical digest of the unsigned offer artifact —
// the commitment the platform-control signature covers.
func (offer PlatformUpdateOffer) Digest() (string, error) {
	unsigned := offer
	unsigned.Authorization = AuthorityReceipt{}
	canonical, err := computerevent.CanonicalJSON(unsigned)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

// SortedManifestPaths returns the offer's manifest paths for deterministic
// staging order.
func (offer PlatformUpdateOffer) SortedManifestPaths() []string {
	paths := make([]string, 0, len(offer.Manifest.Files))
	for _, f := range offer.Manifest.Files {
		paths = append(paths, f.Path)
	}
	sort.Strings(paths)
	return paths
}
