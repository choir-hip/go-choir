package platform

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// Updates and the platform's break-glass act (slice F1,
// docs/design/forced-modularity-build-2026-11-04.md section 5).
//
//	POST /internal/platform/packages/updates                   host only: the update facts of an owner's installs
//	POST /internal/platform/packages/releases/<id>/force-requests   host only: a publisher asks for a forced update (a note; executes nothing)
//	POST /internal/platform/operator/packages/yank             the platform operator's yank or forced update
//
// The yank route is the only writer of listing state yanked and of
// forced_release. It is a manual act of a named platform operator, behind
// the same gate as the key-escrow break glass: a host transport, the host
// credential, and an operator token from CHOIR_PLATFORM_OPERATORS compared
// in constant time. No publisher, scanner or threshold reaches it.

const (
	platformOperatorYankRoute = "/internal/platform/operator/packages/yank"
	// PackageForceRequestKind is a publisher's request that the platform
	// force an update: a note for the platform operator, nothing more.
	PackageForceRequestKind objectgraph.ObjectKind = "choir.package_force_request"
	// maxUpdateInstalls bounds one update read.
	maxUpdateInstalls = 256
)

// ConfigurePlatformOperators sets the named platform operators
// (name:token,...). Empty closes the yank route.
func (h *Handler) ConfigurePlatformOperators(raw string) {
	operators, open := parseKeyEscrowOperators(raw)
	h.platformOperators, h.platformOperatorsOpen = operators, open
}

// authorizePlatformOperator returns the operator's name, or writes the
// refusal.
func (h *Handler) authorizePlatformOperator(w http.ResponseWriter, r *http.Request) (string, bool) {
	if h == nil || !h.platformOperatorsOpen {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "platform operator gate is closed"})
		return "", false
	}
	if !trustedInternalTransport(r) || !hostauth.Authorize(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return "", false
	}
	name := r.Header.Get("X-Choir-Operator")
	expected, ok := h.platformOperators[name]
	provided := sha256.Sum256([]byte(r.Header.Get("X-Choir-Operator-Token")))
	if !ok || !hmac.Equal(expected[:], provided[:]) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "operator authorization required"})
		return "", false
	}
	return name, true
}

// PlatformYankRequest is the operator's act: yank a release, and optionally
// force its installers onto another release of the same track.
type PlatformYankRequest struct {
	ReleaseID     string `json:"release_id"`
	Reason        string `json:"reason"`
	ForcedRelease string `json:"forced_release,omitempty"`
}

// HandlePlatformYank serves the operator's yank and forced update.
func (h *Handler) HandlePlatformYank(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	operator, ok := h.authorizePlatformOperator(w, r)
	if !ok {
		return
	}
	var req PlatformYankRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid yank request"})
		return
	}
	releaseID := strings.ToLower(strings.TrimSpace(req.ReleaseID))
	forced := strings.ToLower(strings.TrimSpace(req.ForcedRelease))
	reason := strings.TrimSpace(req.Reason)
	if !computerevent.IsSHA256(releaseID) || (forced != "" && !computerevent.IsSHA256(forced)) {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "release_id (and forced_release) are sha256 ids"})
		return
	}
	if reason == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "a yank names its reason"})
		return
	}
	if len(reason) > 2000 {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "reason is at most 2000 bytes"})
		return
	}
	h.packageMu.Lock()
	defer h.packageMu.Unlock()
	ctx := r.Context()
	_, record, err := h.findPackageRecord(ctx, "", releaseID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errPackageNotPublished)
		return
	}
	ownerID := record.Publisher.OwnerID
	chains, _, err := h.trackChains(ctx, ownerID, record.Package.Name)
	if err != nil {
		writeListingsError(w, err)
		return
	}
	chain := chains[releaseID]
	if len(chain) == 0 {
		writeJSON(w, http.StatusNotFound, errPackageNotPublished)
		return
	}
	current := chain[len(chain)-1]
	if forced != "" {
		target, ok := chains[forced]
		if !ok || len(target) == 0 || forced == releaseID {
			writePackageRefusal(w, &pkgrelease.Refusal{Reason: pkgrelease.ReasonInvalid, Detail: "the forced release is not a release of this package's track"})
			return
		}
		if state := target[len(target)-1].State; state != pkgrelease.StateListed {
			writePackageRefusal(w, &pkgrelease.Refusal{Reason: pkgrelease.ReasonListingState, Detail: "the forced release is " + state + ", not listed"})
			return
		}
	}
	if current.State == pkgrelease.StateYanked && current.ForcedRelease == forced && current.StateReason == reason {
		writePackageJSON(w, http.StatusOK, PackageListingResult{ReleaseID: releaseID, Listing: current})
		return
	}
	prev, err := current.Digest()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "listing digest"})
		return
	}
	next := current.Normalize()
	next.State, next.StateReason, next.StateActor = pkgrelease.StateYanked, reason, pkgrelease.PlatformActorPrefix+operator
	next.ForcedRelease = forced
	next.Version, next.PrevVersionDigest = current.Version+1, prev
	obj, err := listingObject(ownerID, record.Publisher.ComputerID, next, time.Now().UTC())
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
	act := "yanked"
	if forced != "" {
		act = "yanked with a forced update to " + forced
	}
	log.Printf("corpusd: platform operator %s %s release %s (%s): %s", operator, act, releaseID, record.Package.Name, reason)
	writePackageJSON(w, http.StatusOK, PackageListingResult{ReleaseID: releaseID, Changed: true, Listing: next.Normalize()})
}

// PackageUpdateInstall is one install the update read is asked about.
type PackageUpdateInstall struct {
	Package   string `json:"package"`
	Track     string `json:"track"`
	ReleaseID string `json:"release_id"`
}

// PackageUpdatesRequest is the proxy's update read for one owner.
type PackageUpdatesRequest struct {
	OwnerID  string                 `json:"owner_id"`
	Installs []PackageUpdateInstall `json:"installs"`
}

// PackageUpdateOffer is a release an installer may move to, with its
// listing facts (the soak reads ListedAt).
type PackageUpdateOffer struct {
	Release pkgrelease.TrackRelease `json:"release"`
	View    PackageReleaseView      `json:"view"`
}

// PackageUpdateFacts answers one install: the installed release's listing
// as its installer may know it, the newest listed release of the track and
// the newest one that soaked, and the forced target if the platform named
// one.
type PackageUpdateFacts struct {
	PackageUpdateInstall
	Installed *pkgrelease.TrackRelease `json:"installed,omitempty"`
	Newest    *PackageUpdateOffer      `json:"newest,omitempty"`
	Soaked    *PackageUpdateOffer      `json:"soaked,omitempty"`
	Forced    *PackageUpdateOffer      `json:"forced,omitempty"`
}

// trackRelease is a listing as the update decision reads it.
func trackRelease(listing pkgrelease.Listing, at time.Time) pkgrelease.TrackRelease {
	digest, _ := listing.Digest()
	return pkgrelease.TrackRelease{
		ReleaseID: listing.RecordID, Package: listing.Package, Track: listing.Track, Seq: listing.Seq,
		State: listing.State, StateReason: listing.StateReason, StateActor: listing.StateActor,
		ForcedRelease: listing.ForcedRelease, Version: listing.Version, VersionDigest: digest, ListedAt: at,
	}
}

// packageUpdates serves the update read. The installed release's listing
// is told to its installer when it is listed or yanked (a yank is meant for
// its installers); any other state reads as unavailable with no reason, as
// a withdrawn release is its publisher's alone. Offers are listed releases
// only.
func (h *Handler) packageUpdates(w http.ResponseWriter, r *http.Request) {
	var req PackageUpdatesRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil || strings.TrimSpace(req.OwnerID) == "" || len(req.Installs) > maxUpdateInstalls {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "an update read names its owner and at most 256 installs"})
		return
	}
	ctx := r.Context()
	now := time.Now().UTC()
	out := make([]PackageUpdateFacts, 0, len(req.Installs))
	for _, install := range req.Installs {
		install.ReleaseID = strings.ToLower(strings.TrimSpace(install.ReleaseID))
		if !objectgraph.ValidPackageName(install.Package) || !pkgrelease.ValidTrack(install.Track) || !computerevent.IsSHA256(install.ReleaseID) {
			continue
		}
		facts, err := h.updateFacts(ctx, install, now)
		if err != nil {
			writeListingsError(w, err)
			return
		}
		out = append(out, facts)
	}
	writePackageJSON(w, http.StatusOK, map[string]any{"updates": out, "soak_seconds": int(pkgrelease.UpdateSoak.Seconds())})
}

func (h *Handler) updateFacts(ctx context.Context, install PackageUpdateInstall, now time.Time) (PackageUpdateFacts, error) {
	facts := PackageUpdateFacts{PackageUpdateInstall: install}
	chains, times, _, err := h.trackChainsTimed(ctx, install.Track, install.Package)
	if err != nil {
		return facts, err
	}
	all := []pkgrelease.TrackRelease{}
	for id, chain := range chains {
		last := chain[len(chain)-1]
		at := times[listedAtKey{id, last.Version}]
		all = append(all, trackRelease(last, at))
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Seq > all[j].Seq })
	var installed *pkgrelease.TrackRelease
	for i := range all {
		if all[i].ReleaseID == install.ReleaseID {
			tr := all[i]
			installed = &tr
		}
	}
	if installed == nil {
		return facts, nil
	}
	told := *installed
	if told.State != pkgrelease.StateListed && told.State != pkgrelease.StateYanked {
		told = pkgrelease.TrackRelease{ReleaseID: told.ReleaseID, Package: told.Package, Track: told.Track, Seq: told.Seq, State: "unavailable", Version: told.Version}
	}
	facts.Installed = &told
	offer := func(tr pkgrelease.TrackRelease) *PackageUpdateOffer {
		obj, _, err := h.readPackageRecord(ctx, install.Track, tr.ReleaseID)
		if err != nil {
			return nil
		}
		listing := chains[tr.ReleaseID][len(chains[tr.ReleaseID])-1]
		return &PackageUpdateOffer{Release: tr, View: packageView(obj, tr.ReleaseID, listingFor(listing, ""))}
	}
	if yanked, forced := installed.PlatformAct(); yanked && forced != "" {
		for _, tr := range all {
			if tr.ReleaseID == forced && tr.State == pkgrelease.StateListed {
				facts.Forced = offer(tr)
			}
		}
	}
	newer := pkgrelease.NewerReleases(*installed, all)
	if len(newer) > 0 {
		facts.Newest = offer(newer[0])
	}
	if soaked, ok := pkgrelease.NewestSoaked(newer, now); ok {
		if facts.Newest != nil && facts.Newest.Release.ReleaseID == soaked.ReleaseID {
			facts.Soaked = facts.Newest
		} else {
			facts.Soaked = offer(soaked)
		}
	}
	return facts, nil
}

// PackageForceRequest is a publisher's request that the platform force an
// update of one of its releases. It is a note: it notifies the platform
// operator and changes no listing and no install.
type PackageForceRequest struct {
	OwnerID    string `json:"owner_id"`
	ComputerID string `json:"computer_id"`
	Note       string `json:"note"`
}

// packageForceRequest writes the note. Only the release's publisher may
// write one; another owner's answers as a missing release.
func (h *Handler) packageForceRequest(w http.ResponseWriter, r *http.Request, releaseID string) {
	var req PackageForceRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil || strings.TrimSpace(req.OwnerID) == "" || strings.TrimSpace(req.Note) == "" || len(req.Note) > 4000 {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "a force request names its owner and a note of at most 4000 bytes"})
		return
	}
	ctx := r.Context()
	_, record, err := h.readPackageRecord(ctx, req.OwnerID, releaseID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errPackageNotPublished)
		return
	}
	now := time.Now().UTC()
	id, err := objectgraph.BuildCanonicalID(PackageForceRequestKind, req.OwnerID, releaseID+"-"+strconv.FormatInt(now.UnixNano(), 10))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	body, err := computerevent.CanonicalJSON(map[string]any{
		"release_id": releaseID, "package": record.Package.Name, "track": record.Track,
		"computer_id": strings.TrimSpace(req.ComputerID), "note": strings.TrimSpace(req.Note),
		"requested_at": now.Format(time.RFC3339Nano),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "force request"})
		return
	}
	metadata, err := objectgraph.NormalizeMetadata(map[string]any{"release_id": releaseID, "package": record.Package.Name})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "force request"})
		return
	}
	store := h.releaseStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "package store unavailable"})
		return
	}
	obj := objectgraph.Object{
		CanonicalID: id, ObjectKind: PackageForceRequestKind, OwnerID: req.OwnerID, ComputerID: strings.TrimSpace(req.ComputerID),
		VersionID: "1", Body: body, Metadata: metadata,
		ContentHash: objectgraph.ContentHash(PackageForceRequestKind, body, metadata), CreatedAt: now, UpdatedAt: now,
	}
	if _, err := store.InsertObjectIfAbsent(ctx, obj); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "force request: " + err.Error()})
		return
	}
	// The notification: the platform operator reads corpusd's log and the
	// requests. Nothing else happens; the yank route is the only act.
	log.Printf("corpusd: publisher %s requests a forced update of release %s (%s): %s", req.OwnerID, releaseID, record.Package.Name, strings.TrimSpace(req.Note))
	writeJSON(w, http.StatusAccepted, map[string]any{"request_id": id, "release_id": releaseID, "executed": false,
		"detail": "the platform operator is notified; a forced update or yank is the platform's manual act and nothing changed"})
}
