package platform

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/owneract"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// The listing acts at corpusd (docs/design/library-package-propagation-2026-10-26.md
// 4.2, 15.1): withdraw and reinstate, each an owner act written as the
// listing's next version.
//
//	PUT /internal/platform/listings/<record id>   host only; the proxy's owner act
//
// There is no share (owner, 2026-11-03: "published releases mean published,
// for everyone to see"): a publish lists a release public, and a scope
// change refuses share_retired. A withdraw takes the release off the
// catalog heads, the track head and every other owner's view; it stays in
// its publisher's history. A reinstate lists a suspended release again, or
// a withdrawn one while it can still head the track (its parent is the head
// without it); once a release was published past it, reinstating would
// fork the track and refuses track_moved. A yanked release (S11, the
// host's) takes no owner act.

const listingsRoute = "/internal/platform/listings"

// trackPageSize is one page of a track read; reads page to the end. A var
// for the test.
var trackPageSize = 500

// ListingChange is one owner act on a listing: a state. Scopes, SharedWith
// and ConfirmPaths are what a retired share sent; any of them refuses
// share_retired.
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
	ReleaseID string             `json:"release_id"`
	Changed   bool               `json:"changed"`
	Listing   pkgrelease.Listing `json:"listing"`
}

// PackageListingRefusal is a refused publish or listing act, with the
// publish preview when it needs a confirmation.
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
	if len(change.Scopes) > 0 || change.SharedWith != nil || strings.TrimSpace(change.ConfirmPaths) != "" {
		writePackageJSON(w, http.StatusGone, packageAPIError{Error: pkgrelease.ReasonShareRetired + ": " + pkgrelease.ShareRetiredDetail, Reason: pkgrelease.ReasonShareRetired})
		return
	}
	if change.State == "" {
		writePackageRefusal(w, &pkgrelease.Refusal{Reason: pkgrelease.ReasonInvalid, Detail: "a listing act is a state (withdraw, reinstate)"})
		return
	}
	if change.State != pkgrelease.StateListed && change.State != pkgrelease.StateWithdrawn {
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
	chains, _, err := h.trackChains(ctx, ownerID, record.Package.Name)
	if err != nil {
		writeListingsError(w, err)
		return
	}
	listings := map[string]pkgrelease.Listing{}
	for id, chain := range chains {
		listings[id] = chain[len(chain)-1]
	}
	current, ok := listings[releaseID]
	if !ok {
		writeJSON(w, http.StatusNotFound, errPackageNotPublished)
		return
	}
	// A receipt acts once (review 291 F4): one that already wrote a version
	// of this listing is a replay, for instance of a withdraw after the
	// owner reinstated. A refused act writes nothing, so it spends nothing.
	if actor != "owner_act:unverified" {
		for _, version := range chains[releaseID] {
			if version.StateActor == actor {
				writePackageRefusal(w, &pkgrelease.Refusal{Reason: pkgrelease.ReasonOwnerActReplayed, Detail: fmt.Sprintf(
					"this owner-act receipt already wrote version %d of the listing; each act is a new receipt", version.Version)})
				return
			}
		}
	}
	if current.State == pkgrelease.StateYanked {
		writePackageRefusal(w, &pkgrelease.Refusal{Reason: pkgrelease.ReasonListingState, Detail: "the release is yanked by the security authority; no owner act changes its listing"})
		return
	}
	next := current.Normalize()
	unchanged := false
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
	writePackageJSON(w, http.StatusOK, PackageListingResult{ReleaseID: releaseID, Changed: true, Listing: next.Normalize()})
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
