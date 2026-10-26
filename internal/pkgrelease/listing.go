package pkgrelease

import (
	"sort"
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

// Scopes. S2 lists at owner only; S3 adds shared and public; platform is
// S4's.
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

// Normalize fills nil slices and sorts the scope set and the named owners
// and computers, so a listing has one canonical form. It never drops a
// duplicate: Validate refuses one.
func (l Listing) Normalize() Listing {
	sorted := func(in []string) []string {
		out := append([]string{}, in...)
		sort.Strings(out)
		return out
	}
	l.Scopes = sorted(l.Scopes)
	l.SharedWith.Owners = sorted(l.SharedWith.Owners)
	l.SharedWith.Computers = sorted(l.SharedWith.Computers)
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

// Validate checks the listing's shape. Every scope set holds owner; shared
// names at least one owner other than the track's, and only shared names
// any; public admits every owner. The platform scope (S4) and named
// computers (no computer reads packages before S5) refuse
// scope_not_built: a scope this build cannot enforce is refused, never
// stored and ignored. Platform proposals arrive with S4b.
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
	if err := l.validateScopes(); err != nil {
		return err
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

func (l Listing) validateScopes() error {
	fail := func(format string, args ...any) error { return refuse(ReasonInvalid, "listing: "+format, args...) }
	seen := map[string]bool{}
	for _, scope := range l.Scopes {
		switch scope {
		case ScopeOwner, ScopeShared, ScopePublic:
		case ScopePlatform:
			return refuse(ReasonScopeNotBuilt, "listing: the platform scope arrives with S4")
		default:
			return fail("scope %q is not a scope", scope)
		}
		if seen[scope] {
			return fail("scope %q is named twice", scope)
		}
		seen[scope] = true
	}
	if !seen[ScopeOwner] {
		return fail("every scope set holds owner")
	}
	if len(l.SharedWith.Computers) != 0 {
		return refuse(ReasonScopeNotBuilt, "listing: sharing with named computers arrives when a computer reads packages (S5); share with their owners")
	}
	owners := map[string]bool{}
	for _, owner := range l.SharedWith.Owners {
		if !safeIdentity.MatchString(owner) {
			return fail("shared owner %q is not an owner id", owner)
		}
		if owner == l.Track {
			return fail("the track's owner sees its own listing; it is not shared with itself")
		}
		if owners[owner] {
			return fail("shared owner %q is named twice", owner)
		}
		owners[owner] = true
	}
	if seen[ScopeShared] != (len(owners) > 0) {
		return fail("the shared scope and the named owners go together")
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
