package pkgrelease

import (
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

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
func (l Listing) Normalize() Listing {
	if l.Scopes == nil {
		l.Scopes = []string{}
	}
	if l.SharedWith.Owners == nil {
		l.SharedWith.Owners = []string{}
	}
	if l.SharedWith.Computers == nil {
		l.SharedWith.Computers = []string{}
	}
	return l
}

// Canonical returns the listing's canonical JSON.
func (l Listing) Canonical() ([]byte, error) {
	raw, err := computerevent.CanonicalJSON(l.Normalize())
	if err != nil {
		return nil, refuse(ReasonInvalid, "listing: %v", err)
	}
	return raw, nil
}

// Digest is the sha256 of the canonical listing: the next version's
// prev_version_digest.
func (l Listing) Digest() (string, error) {
	raw, err := l.Canonical()
	if err != nil {
		return "", err
	}
	return computerevent.DigestBytes(raw), nil
}

// Validate checks the listing's shape. S2 lists package releases at scope
// owner only; the other scopes and record kinds arrive with S3 and S4, and
// a listing that names them before then is refused, not ignored.
func (l Listing) Validate() error {
	l = l.Normalize()
	fail := func(format string, args ...any) error { return refuse(ReasonInvalid, "listing: "+format, args...) }
	if l.RecordKind != RecordKindPackageRelease {
		return fail("record_kind %q is not %s", l.RecordKind, RecordKindPackageRelease)
	}
	if !computerevent.IsSHA256(l.RecordID) || !objectgraph.ValidPackageName(l.Package) || !safeIdentity.MatchString(l.Track) {
		return fail("record_id, package and track are required")
	}
	if l.Seq < 1 || l.Version < 1 {
		return fail("seq and version start at 1")
	}
	if len(l.Scopes) != 1 || l.Scopes[0] != ScopeOwner || len(l.SharedWith.Owners) != 0 || len(l.SharedWith.Computers) != 0 {
		return fail("scopes must be [owner] until S3")
	}
	switch l.State {
	case StateListed, StateWithdrawn, StateSuspended, StateYanked:
	default:
		return fail("state %q is not a listing state", l.State)
	}
	if strings.TrimSpace(l.StateActor) == "" {
		return fail("state_actor is required")
	}
	if (l.Version == 1) != (l.PrevVersionDigest == "") || (l.PrevVersionDigest != "" && !computerevent.IsSHA256(l.PrevVersionDigest)) {
		return fail("version 1 names no predecessor; every later version names its predecessor's digest")
	}
	return nil
}

// DecodeListing reads exactly one listing, refusing unknown fields.
func DecodeListing(raw []byte) (Listing, error) {
	var listing Listing
	if err := decodeStrict(raw, &listing); err != nil {
		return Listing{}, err
	}
	return listing.Normalize(), nil
}

// HeadEligible reports whether a listing in this state can be its track's
// head. A withdrawn release is never a parent (L23); neither is a yanked
// one, whose content the security authority set aside. A suspended release
// stays on the track: its fix is published on top of it.
func HeadEligible(state string) bool {
	return state == StateListed || state == StateSuspended
}
