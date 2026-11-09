package platform

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/owneract"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// Updates and the platform operator's act (slice F1,
// docs/design/forced-modularity-build-2026-11-04.md section 5; operator
// account, docs/design/platform-operator-account-2026-11-06.md).
//
//	POST /internal/platform/packages/updates                   host only: the update facts of an owner's installs
//	POST /internal/platform/packages/releases/<id>/force-requests   host only: a publisher asks for a forced update (a note; executes nothing)
//	POST /internal/platform/operator/packages/yank             the platform operator's yank or forced update
//
// The yank route is the only writer of listing state yanked and of
// forced_release. It is a manual act of a platform operator, reached only
// over a host transport with the host credential, and carrying exactly one
// of two credentials:
//
//   - an owner-act receipt (owneract.ActPlatformYank) the proxy minted for
//     a signed-in session, whose account is in CHOIR_PLATFORM_OPERATOR_USERS
//     (supplied at deploy). The receipt is verified in every owner-act mode,
//     must bind this request, and is recorded single use in
//     owner_act_receipts before the listing is written. state_actor is
//     platform:user:<user id>;
//   - break glass: an operator token from CHOIR_PLATFORM_OPERATORS compared
//     in constant time, sent from the host itself (choir platform yank).
//     state_actor is platform:<operator name>.
//
// No publisher, scanner or threshold reaches it.

const (
	platformOperatorYankRoute = "/internal/platform/operator/packages/yank"
	// PackageForceRequestKind is a publisher's request that the platform
	// force an update: a note for the platform operator, nothing more.
	PackageForceRequestKind objectgraph.ObjectKind = "choir.package_force_request"
	// maxUpdateInstalls bounds one update read.
	maxUpdateInstalls = 256
	// platformOperatorUserActor prefixes the actor of an operator account's
	// act, after pkgrelease.PlatformActorPrefix. Operator token names
	// cannot contain ':', so the two actor forms never collide.
	platformOperatorUserActor = "user:"
	// maxPlatformOperatorUsers bounds the role's roster (the deploy gate
	// carries at most this many).
	maxPlatformOperatorUsers = 8
)

var platformOperatorUserRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// errPlatformActReplayed is a receipt corpusd has already recorded.
var errPlatformActReplayed = errors.New("owner act: receipt already used")

// ConfigurePlatformOperators sets the named break-glass operators
// (name:token,...). Empty closes the token path.
func (h *Handler) ConfigurePlatformOperators(raw string) {
	operators, open := parseKeyEscrowOperators(raw)
	h.platformOperators, h.platformOperatorsOpen = operators, open
}

// ConfigurePlatformOperatorUsers sets the accounts that hold the platform
// operator role: lower-case user ids, comma-separated, or "none". A
// malformed list grants nobody (fail closed) and is returned as an error
// for the start log. It returns the number of accounts granted.
func (h *Handler) ConfigurePlatformOperatorUsers(raw string) (int, error) {
	users, err := ParsePlatformOperatorUsers(raw)
	h.platformOperatorUsers = users
	return len(users), err
}

// ParsePlatformOperatorUsers parses CHOIR_PLATFORM_OPERATOR_USERS.
func ParsePlatformOperatorUsers(raw string) (map[string]bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "none" {
		return nil, nil
	}
	users := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		id := strings.TrimSpace(part)
		if !platformOperatorUserRE.MatchString(id) {
			return nil, fmt.Errorf("platform operator accounts: %q is not a lower-case user id; no account holds the role", id)
		}
		users[id] = true
	}
	if len(users) > maxPlatformOperatorUsers {
		return nil, fmt.Errorf("platform operator accounts: more than %d; no account holds the role", maxPlatformOperatorUsers)
	}
	return users, nil
}

// PlatformYankSubject is the subject an operator's yank receipt binds: the
// exact release, forced target and reason (by digest) corpusd will write.
func PlatformYankSubject(req PlatformYankRequest) map[string]any {
	req = req.normalized()
	return map[string]any{
		"release_id":     req.ReleaseID,
		"forced_release": req.ForcedRelease,
		"reason_sha256":  computerevent.DigestBytes([]byte(req.Reason)),
	}
}

// authorizePlatformOperatorToken is the break-glass path: it returns the
// operator's name, or writes the refusal.
func (h *Handler) authorizePlatformOperatorToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	name := r.Header.Get("X-Choir-Operator")
	expected, ok := h.platformOperators[name]
	provided := sha256.Sum256([]byte(r.Header.Get("X-Choir-Operator-Token")))
	if !h.platformOperatorsOpen || !ok || !hmac.Equal(expected[:], provided[:]) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "operator authorization required"})
		return "", false
	}
	return name, true
}

// verifyPlatformOperatorAct verifies an operator account's receipt for req,
// in every owner-act mode (a platform act is never admitted on observe).
func (h *Handler) verifyPlatformOperatorAct(r *http.Request, req PlatformYankRequest) (*owneract.Verified, int, error) {
	authority := h.packageOwnerActs
	if authority == nil || authority.verifier == nil {
		return nil, http.StatusServiceUnavailable, fmt.Errorf("no owner-act authority is configured")
	}
	receipt, err := owneract.Decode(r.Header.Get(owneract.Header))
	if err != nil {
		return nil, http.StatusForbidden, err
	}
	verified, err := authority.verifier.Verify(receipt, owneract.PlatformComputerID, owneract.ActPlatformYank, authority.now())
	if err != nil {
		return nil, http.StatusForbidden, err
	}
	if verified.AuthMethod != owneract.AuthMethodSession {
		return nil, http.StatusForbidden, fmt.Errorf("owner act: a platform act is made from a signed-in session")
	}
	for key, want := range PlatformYankSubject(req) {
		if verified.SubjectString(key) != want {
			return nil, http.StatusForbidden, fmt.Errorf("owner act: the receipt does not bind this %s", key)
		}
	}
	if !h.platformOperatorUsers[verified.OwnerUserID] {
		return nil, http.StatusForbidden, fmt.Errorf("account %s is not a platform operator", verified.OwnerUserID)
	}
	return &verified, 0, nil
}

// recordPlatformOperatorAct records the operator's receipt single use, with
// the digest of the listing version the act leaves, before that version is
// written: an act whose write then fails leaves a recorded attempt and no
// effect, never an effect without its authority.
func (h *Handler) recordPlatformOperatorAct(ctx context.Context, verified *owneract.Verified, listingDigest string) error {
	if h.service == nil || h.service.store == nil || h.service.store.db == nil {
		return fmt.Errorf("owner act: no receipt store")
	}
	tx, err := h.service.store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	replayed, err := ownerActReplayed(ctx, tx, verified)
	if err != nil {
		return err
	}
	if replayed {
		return errPlatformActReplayed
	}
	if err := recordOwnerAct(ctx, tx, verified, listingDigest, time.Now().UTC()); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return errPlatformActReplayed
		}
		return err
	}
	return tx.Commit()
}

// PlatformYankRequest is the operator's act: yank a release, and optionally
// force its installers onto another release of the same track.
type PlatformYankRequest struct {
	ReleaseID     string `json:"release_id"`
	Reason        string `json:"reason"`
	ForcedRelease string `json:"forced_release,omitempty"`
}

func (req PlatformYankRequest) normalized() PlatformYankRequest {
	return PlatformYankRequest{
		ReleaseID:     strings.ToLower(strings.TrimSpace(req.ReleaseID)),
		Reason:        strings.TrimSpace(req.Reason),
		ForcedRelease: strings.ToLower(strings.TrimSpace(req.ForcedRelease)),
	}
}

// Validate checks the request's shape (the proxy runs it before it mints a
// receipt; corpusd runs it again).
func (req PlatformYankRequest) Validate() error {
	req = req.normalized()
	if !computerevent.IsSHA256(req.ReleaseID) || (req.ForcedRelease != "" && !computerevent.IsSHA256(req.ForcedRelease)) {
		return fmt.Errorf("release_id (and forced_release) are sha256 ids")
	}
	if req.Reason == "" {
		return fmt.Errorf("a yank names its reason")
	}
	if len(req.Reason) > 2000 {
		return fmt.Errorf("reason is at most 2000 bytes")
	}
	return nil
}

// HandlePlatformYank serves the operator's yank and forced update.
func (h *Handler) HandlePlatformYank(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if h == nil || (!h.platformOperatorsOpen && len(h.platformOperatorUsers) == 0) {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "platform operator gate is closed"})
		return
	}
	if !trustedInternalTransport(r) || !hostauth.Authorize(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	byReceipt := r.Header.Get(owneract.Header) != ""
	if byReceipt && (r.Header.Get("X-Choir-Operator") != "" || r.Header.Get("X-Choir-Operator-Token") != "") {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "one operator credential per act: a receipt or a token, not both"})
		return
	}
	operator := ""
	if !byReceipt {
		name, ok := h.authorizePlatformOperatorToken(w, r)
		if !ok {
			return
		}
		operator = name
	}
	var req PlatformYankRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid yank request"})
		return
	}
	if err := req.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	req = req.normalized()
	var verified *owneract.Verified
	if byReceipt {
		v, status, err := h.verifyPlatformOperatorAct(r, req)
		if err != nil {
			log.Printf("corpusd: refused platform operator act on release %s: %v", req.ReleaseID, err)
			writeJSON(w, status, apiError{Error: fmt.Sprintf("%v: %v", ErrOwnerActRefused, err)})
			return
		}
		verified = v
		operator = platformOperatorUserActor + v.OwnerUserID
	}
	releaseID, forced, reason := req.ReleaseID, req.ForcedRelease, req.Reason
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
	prev, err := current.Digest()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "listing digest"})
		return
	}
	unchanged := current.State == pkgrelease.StateYanked && current.ForcedRelease == forced && current.StateReason == reason
	next := current.Normalize()
	next.State, next.StateReason, next.StateActor = pkgrelease.StateYanked, reason, pkgrelease.PlatformActorPrefix+operator
	next.ForcedRelease = forced
	next.Version, next.PrevVersionDigest = current.Version+1, prev
	if verified != nil {
		// The receipt is recorded with the listing version the act leaves:
		// the current one when it changes nothing.
		leaves := prev
		if !unchanged {
			if leaves, err = next.Digest(); err != nil {
				writePackageRefusal(w, err)
				return
			}
		}
		if err := h.recordPlatformOperatorAct(ctx, verified, leaves); err != nil {
			if errors.Is(err, errPlatformActReplayed) {
				writeJSON(w, http.StatusConflict, apiError{Error: fmt.Sprintf("%v: %v", ErrOwnerActRefused, err)})
				return
			}
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "owner act record: " + err.Error()})
			return
		}
	}
	if unchanged {
		writePackageJSON(w, http.StatusOK, PackageListingResult{ReleaseID: releaseID, Listing: current})
		return
	}
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
	receiptNote := ""
	if verified != nil {
		receiptNote = " (owner-act receipt " + verified.Digest + ")"
	}
	log.Printf("corpusd: platform operator %s %s release %s (%s)%s: %s", operator, act, releaseID, record.Package.Name, receiptNote, reason)
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
		facts, err := h.updateFacts(ctx, req.OwnerID, install, now)
		if err != nil {
			writeListingsError(w, err)
			return
		}
		out = append(out, facts)
	}
	writePackageJSON(w, http.StatusOK, map[string]any{"updates": out, "soak_seconds": int(pkgrelease.UpdateSoak.Seconds())})
}

// updateFacts is the update read for one install of reader's computer. An
// owner_listed release (a T2 own-track update) is told and offered only to
// its own track's owner; to anyone else it reads as unavailable and is never
// offered.
func (h *Handler) updateFacts(ctx context.Context, reader string, install PackageUpdateInstall, now time.Time) (PackageUpdateFacts, error) {
	facts := PackageUpdateFacts{PackageUpdateInstall: install}
	chains, times, _, err := h.trackChainsTimed(ctx, install.Track, install.Package)
	if err != nil {
		return facts, err
	}
	all := []pkgrelease.TrackRelease{}
	for id, chain := range chains {
		last := chain[len(chain)-1]
		if last.State == pkgrelease.StateOwnerListed && last.Track != reader {
			continue
		}
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
	if told.State != pkgrelease.StateListed && told.State != pkgrelease.StateYanked && told.State != pkgrelease.StateOwnerListed {
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
