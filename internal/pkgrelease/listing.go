package pkgrelease

// ListingKind is the Store B kind of a release listing (design 4.2): one
// object per (record id, version), written by corpusd only. Visibility and
// state live only in the listing, so the record never changes.
const ListingKind = "choir.release_listing"

// Record kinds a listing names.
const (
	RecordKindPackageRelease   = "package_release"
	RecordKindPlatformProposal = "platform_proposal"
)

// Scopes. S2 lists at owner only; the others are S3 and S4.
const (
	ScopeOwner    = "owner"
	ScopeShared   = "shared"
	ScopePlatform = "platform"
	ScopePublic   = "public"
)

// Listing states.
const (
	StateListed    = "listed"
	StateWithdrawn = "withdrawn"
	StateSuspended = "suspended"
	StateYanked    = "yanked"
)

// Listing is one version of a record's listing.
type Listing struct {
	RecordKind        string     `json:"record_kind"`
	RecordID          string     `json:"record_id"`
	Package           string     `json:"package,omitempty"`
	Track             string     `json:"track"`
	Seq               int        `json:"seq,omitempty"`
	Scopes            []string   `json:"scopes"`
	SharedWith        SharedWith `json:"shared_with"`
	State             string     `json:"state"`
	StateReason       string     `json:"state_reason"`
	StateActor        string     `json:"state_actor"`
	Version           int        `json:"version"`
	PrevVersionDigest string     `json:"prev_version_digest,omitempty"`
}

// SharedWith names the owners and computers a shared listing admits (S3).
type SharedWith struct {
	Owners    []string `json:"owners"`
	Computers []string `json:"computers"`
}

// Normalize fills nil slices so a listing has one canonical form.
func (l Listing) Normalize() Listing { return l }

// Canonical returns the listing's canonical JSON.
func (l Listing) Canonical() ([]byte, error) { return nil, errNotBuilt }

// Digest is the sha256 of the canonical listing: the next version's
// prev_version_digest.
func (l Listing) Digest() (string, error) { return "", errNotBuilt }

// Validate checks the listing's shape.
func (l Listing) Validate() error { return errNotBuilt }

// DecodeListing reads exactly one listing, refusing unknown fields.
func DecodeListing(raw []byte) (Listing, error) { return Listing{}, errNotBuilt }

// HeadEligible reports whether a listing in this state can be its track's
// head: a withdrawn or yanked release is never a parent (L23).
func HeadEligible(state string) bool { return false }
