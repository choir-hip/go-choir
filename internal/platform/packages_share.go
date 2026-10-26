package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/owneract"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// Library S3 at corpusd (docs/design/library-package-propagation-2026-10-26.md
// 4.1, 4.2, 4.4, 15.1, 17 S3): share, withdraw and reinstate, each an owner
// act written as the listing's next version.
//
//	PUT /internal/platform/listings/<record id>   host only; the proxy's owner act
//
// A share sets the scope set (owner, shared with named owners, public). A
// widening shows every path that would leave and the review diff's size
// and refuses until the owner confirms that digest (--confirm-paths); it
// runs the secret scan again on the stored bytes. A narrowing needs
// neither: the owner can always take readers away. A withdraw takes the
// release off the catalog heads and the track head; it stays in the
// history. A reinstate lists a suspended release again, or a withdrawn one
// while it can still head the track (its parent is the head without it);
// once a release was published past it, reinstating would fork the track
// and refuses track_moved. A yanked release (S11, the host's) takes no
// owner act.

const listingsRoute = "/internal/platform/listings"

// trackPageSize is one page of a track read; reads page to the end. A var
// for the test.
var trackPageSize = 500

// ListingChange is one owner act on a listing: a scope change, or a state.
type ListingChange struct {
	Scopes       []string               `json:"scopes,omitempty"`
	SharedWith   *pkgrelease.SharedWith `json:"shared_with,omitempty"`
	State        string                 `json:"state,omitempty"`
	Reason       string                 `json:"reason,omitempty"`
	ConfirmPaths string                 `json:"confirm_paths,omitempty"`
}

// PackageListingRequest is the proxy's listing act: the change, for the
// owner the proxy authenticated, gated by the computer it names.
type PackageListingRequest struct {
	OwnerID    string `json:"owner_id"`
	ComputerID string `json:"computer_id"`
	ListingChange
}

// PackageListingResult answers a listing act. Changed is false when the
// act asked for what the listing already is (nothing is written).
type PackageListingResult struct {
	ReleaseID string              `json:"release_id"`
	Changed   bool                `json:"changed"`
	Listing   pkgrelease.Listing  `json:"listing"`
	Preview   *pkgrelease.Preview `json:"preview,omitempty"`
}

// PackageListingRefusal is a refused listing act, with the widening's
// preview when it needs a confirmation.
type PackageListingRefusal struct {
	Error   string              `json:"error"`
	Reason  string              `json:"reason,omitempty"`
	Preview *pkgrelease.Preview `json:"preview,omitempty"`
}

// PackageListingSubject is the owner-act subject of a listing act: the
// release, the scope set and the named owners (sorted, comma-joined), and
// the state. The receipt binds exactly the change corpusd writes.
// confirm_paths is not in it: the host checks it against the bytes.
func PackageListingSubject(releaseID string, change ListingChange) map[string]any {
	listing := pkgrelease.Listing{Scopes: change.Scopes}
	if change.SharedWith != nil {
		listing.SharedWith = *change.SharedWith
	}
	listing = listing.Normalize()
	return map[string]any{
		"release_id":       strings.ToLower(strings.TrimSpace(releaseID)),
		"scopes":           strings.Join(listing.Scopes, ","),
		"shared_owners":    strings.Join(listing.SharedWith.Owners, ","),
		"shared_computers": strings.Join(listing.SharedWith.Computers, ","),
		"state":            strings.TrimSpace(change.State),
	}
}

// PackageShareReadsMode is CHOIR_PACKAGE_SHARE_READS, the read side's
// rollback: scoped (the default, also when unset) reads each listing's
// scope set; owner reads every listing at owner scope, as S2 did. Only an
// unset, empty or "scoped" value is scoped: a typo narrows reads, it never
// widens them. Read once at startup.
type PackageShareReadsMode string

const (
	PackageShareReadsScoped PackageShareReadsMode = "scoped"
	PackageShareReadsOwner  PackageShareReadsMode = "owner"

	// EnvPackageShareReads names the switch.
	EnvPackageShareReads = "CHOIR_PACKAGE_SHARE_READS"
)

// ParsePackageShareReads reads the switch value.
func ParsePackageShareReads(raw string) PackageShareReadsMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(PackageShareReadsScoped):
		return PackageShareReadsScoped
	}
	return PackageShareReadsOwner
}

// SetPackageShareReads sets the mode once at startup, before the handler
// serves.
func (h *Handler) SetPackageShareReads(mode PackageShareReadsMode) {
	if h != nil {
		h.packageShareReads = mode
	}
}

func listingWidened(listing pkgrelease.Listing) bool { return listing.Widened() }

// HandleListings serves the listing acts.
func (h *Handler) HandleListings(w http.ResponseWriter, r *http.Request) {
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "listing routes are host only; a listing changes by the owner's act through the proxy"})
		return
	}
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	releaseID := strings.ToLower(strings.Trim(strings.TrimPrefix(r.URL.Path, listingsRoute), "/"))
	if !computerevent.IsSHA256(releaseID) {
		writeJSON(w, http.StatusNotFound, errPackageNotPublished)
		return
	}
	var request PackageListingRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid listing act"})
		return
	}
	ownerID, computerID := strings.TrimSpace(request.OwnerID), strings.TrimSpace(request.ComputerID)
	change := request.ListingChange
	change.State = strings.TrimSpace(change.State)
	if ownerID == "" || computerID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "owner_id and computer_id are required"})
		return
	}
	scoped, stated := len(change.Scopes) > 0, change.State != ""
	if scoped == stated || (!scoped && change.SharedWith != nil) {
		writePackageRefusal(w, &pkgrelease.Refusal{Reason: pkgrelease.ReasonInvalid, Detail: "a listing act is a scope set (share) or a state (withdraw, reinstate), not both and not neither"})
		return
	}
	if stated && change.State != pkgrelease.StateListed && change.State != pkgrelease.StateWithdrawn {
		writePackageRefusal(w, &pkgrelease.Refusal{Reason: pkgrelease.ReasonInvalid, Detail: fmt.Sprintf(
			"an owner act sets a listing %s (reinstate) or %s (withdraw), not %q", pkgrelease.StateListed, pkgrelease.StateWithdrawn, change.State)})
		return
	}
	actor, status, refusal := h.packageActor(r, ownerID, computerID, owneract.ActPackageListing, PackageListingSubject(releaseID, change), "listing act")
	if status != 0 {
		writeJSON(w, status, refusal)
		return
	}

	// Listing acts and publishes share one lock: each reads the track and
	// writes its next version under it.
	h.packageMu.Lock()
	defer h.packageMu.Unlock()
	ctx := r.Context()
	_, record, err := h.readPackageRecord(ctx, ownerID, releaseID)
	if err != nil {
		// Another owner's release, or none: the same answer as a read.
		writeJSON(w, http.StatusNotFound, errPackageNotPublished)
		return
	}
	listings, _, err := h.trackListings(ctx, ownerID, record.Package.Name)
	if err != nil {
		writeListingsError(w, err)
		return
	}
	current, ok := listings[releaseID]
	if !ok {
		writeJSON(w, http.StatusNotFound, errPackageNotPublished)
		return
	}
	if current.State == pkgrelease.StateYanked {
		writePackageRefusal(w, &pkgrelease.Refusal{Reason: pkgrelease.ReasonListingState, Detail: "the release is yanked by the security authority; no owner act changes its listing"})
		return
	}
	next := current.Normalize()
	var preview *pkgrelease.Preview
	unchanged := false
	if scoped {
		next.Scopes = change.Scopes
		next.SharedWith = pkgrelease.SharedWith{}
		if change.SharedWith != nil {
			next.SharedWith = *change.SharedWith
		}
		next = next.Normalize()
		if err := next.Validate(); err != nil {
			writePackageRefusal(w, err)
			return
		}
		switch {
		case pkgrelease.SameAudience(current, next):
			unchanged = true
		case pkgrelease.Widens(current, next):
			if current.State != pkgrelease.StateListed {
				writePackageRefusal(w, &pkgrelease.Refusal{Reason: pkgrelease.ReasonListingState, Detail: "the release is " + current.State + "; only a listed release is shared wider"})
				return
			}
			p, err := h.wideningPreview(ctx, record)
			if err != nil {
				writePackageRefusal(w, err)
				return
			}
			preview = &p
			if confirm := strings.ToLower(strings.TrimSpace(change.ConfirmPaths)); confirm != p.PathsDigest {
				reason, detail := pkgrelease.ReasonConfirmPathsMismatch, "the confirmation names other paths than this release's"
				if confirm == "" {
					reason, detail = pkgrelease.ReasonConfirmPathsRequired, "a widening shows what leaves: confirm these paths with --confirm-paths "+p.PathsDigest
				}
				writePackageJSON(w, http.StatusConflict, PackageListingRefusal{Error: reason + ": " + detail, Reason: reason, Preview: preview})
				return
			}
			next.StateReason = reasonOr(change.Reason, "shared")
		default:
			next.StateReason = reasonOr(change.Reason, "narrowed")
		}
	} else {
		switch {
		case current.State == change.State:
			unchanged = true
		case change.State == pkgrelease.StateWithdrawn:
			next.State, next.StateReason = pkgrelease.StateWithdrawn, reasonOr(change.Reason, "withdrawn")
		case current.State == pkgrelease.StateWithdrawn:
			if detail, ok := reinstatable(listings, releaseID, record); !ok {
				writePackageRefusal(w, &pkgrelease.Refusal{Reason: pkgrelease.ReasonTrackMoved, Detail: detail})
				return
			}
			next.State, next.StateReason = pkgrelease.StateListed, reasonOr(change.Reason, "reinstated")
		default: // suspended -> listed
			next.State, next.StateReason = pkgrelease.StateListed, reasonOr(change.Reason, "reinstated")
		}
	}
	if unchanged {
		writePackageJSON(w, http.StatusOK, PackageListingResult{ReleaseID: releaseID, Listing: current})
		return
	}
	prev, err := current.Digest()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "listing digest"})
		return
	}
	next.Version, next.PrevVersionDigest, next.StateActor = current.Version+1, prev, actor
	obj, err := listingObject(ownerID, computerID, next, time.Now().UTC())
	if err != nil {
		writePackageRefusal(w, err)
		return
	}
	inserted, err := h.releaseStore().InsertObjectIfAbsent(ctx, obj)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "package listing: " + err.Error()})
		return
	}
	if !inserted {
		writePackageRefusal(w, &pkgrelease.Refusal{Reason: pkgrelease.ReasonListingMoved, Detail: fmt.Sprintf("version %d of this listing is already written; read it and act again", next.Version)})
		return
	}
	writePackageJSON(w, http.StatusOK, PackageListingResult{ReleaseID: releaseID, Changed: true, Listing: next.Normalize(), Preview: preview})
}

func reasonOr(reason, fallback string) string {
	if reason = strings.TrimSpace(reason); reason != "" {
		return reason
	}
	return fallback
}

// reinstatable reports whether a withdrawn release can head its track
// again without forking it: its parent is the head of the track without it
// (the base when the track has no other head).
func reinstatable(listings map[string]pkgrelease.Listing, releaseID string, record pkgrelease.Record) (string, bool) {
	others := map[string]pkgrelease.Listing{}
	for id, listing := range listings {
		if id != releaseID {
			others[id] = listing
		}
	}
	head, found, _ := trackHead(others)
	switch {
	case record.Parent.Kind == pkgrelease.ParentBase && !found:
		return "", true
	case record.Parent.Kind == pkgrelease.ParentRelease && found && head.RecordID == record.Parent.ReleaseID:
		return "", true
	case found:
		return fmt.Sprintf("the track moved past this release: its head is %s (seq %d), not this release's parent; reinstating it would fork the track, so publish again on the head", head.RecordID, head.Seq), false
	}
	return "this release's parent is no longer on the track; reinstating it would fork the track", false
}

// wideningPreview reads the release's stored patches and computes what a
// widening shows.
func (h *Handler) wideningPreview(ctx context.Context, record pkgrelease.Record) (pkgrelease.Preview, error) {
	treePatch, err := h.readPackageBlob(record.Source.TreePatch)
	if err != nil {
		return pkgrelease.Preview{}, err
	}
	reviewPatch, err := h.readPackageBlob(record.Source.ReviewPatch)
	if err != nil {
		return pkgrelease.Preview{}, err
	}
	return pkgrelease.WideningPreview(ctx, record, treePatch, reviewPatch)
}
