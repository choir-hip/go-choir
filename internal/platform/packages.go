package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/owneract"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// PackageReleaseKind is the Store B object a published package release is
// (Library S2, docs/design/library-package-propagation-2026-10-26.md 3.1):
// immutable, insert-if-absent, filed under the publisher's owner and the
// release id.
const PackageReleaseKind objectgraph.ObjectKind = "choir.package_release"

// ReleaseListingKind is one version of a record's listing (design 4.2):
// its scopes, its state and its sequence number on the track. Each version
// is its own object, written insert-if-absent, naming the digest of the
// version before it.
const ReleaseListingKind objectgraph.ObjectKind = "choir.release_listing"

const packagesRoute = "/internal/platform/packages"

// PackageBaseStatus is the package base store in corpusd's health detail:
// FP0 reads the shallow flag without SSH before S2's proof (design 3.2,
// check 3).
type PackageBaseStatus struct {
	Store   string `json:"store,omitempty"`
	Shallow *bool  `json:"shallow,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ConfigurePackageBase names the host's object store for base-parent
// checks and reads its shallow flag once. An empty dir leaves base parents
// unverifiable (every one refuses). It is the directory the host build
// already reads, so this needs no new host capability: one more read-only
// git command in a process that already runs git there.
func (h *Handler) ConfigurePackageBase(ctx context.Context, dir string) {
	if h == nil {
		return
	}
	dir = strings.TrimSpace(dir)
	if dir == "" {
		h.packageBase = nil
		h.packageBaseStatus = &PackageBaseStatus{Error: "no base object store configured (CORPUSD_HOST_BUILD_SOURCE_DIR); every base parent refuses package_parent_unverifiable"}
		return
	}
	store := pkgrelease.GitStore{Dir: dir}
	h.packageBase = store
	status := &PackageBaseStatus{Store: dir}
	if shallow, err := store.Shallow(ctx); err != nil {
		status.Error = err.Error()
	} else {
		status.Shallow = &shallow
	}
	h.packageBaseStatus = status
}

// ConfigurePackageOwnerActs installs the owner-act policy package publish
// verifies its receipt under (CHOIR_OWNER_ACT_MODE): enforce refuses a
// publish without a receipt binding the release; observe logs and lists it
// as unverified. Unset, the proxy's owner-act gate is the act, as for the
// software release publish.
func (h *Handler) ConfigurePackageOwnerActs(authority *OwnerActAuthority) {
	if h != nil {
		h.packageOwnerActs = authority
	}
}

// PackageReleaseView is one published package release with its current
// listing.
type PackageReleaseView struct {
	ReleaseID   string             `json:"release_id"`
	CanonicalID string             `json:"canonical_id"`
	PublishedAt string             `json:"published_at"`
	Published   bool               `json:"published,omitempty"`
	Record      json.RawMessage    `json:"record"`
	Listing     pkgrelease.Listing `json:"listing"`
}

type packageAPIError struct {
	Error  string `json:"error"`
	Reason string `json:"reason,omitempty"`
}

type packagePublishRequest struct {
	ReleaseID  string          `json:"release_id"`
	Record     json.RawMessage `json:"record"`
	OwnerID    string          `json:"owner_id"`
	ComputerID string          `json:"computer_id"`
}

// HandlePackages serves the package release routes, every one host only:
//
//	PUT  /internal/platform/packages/blobs/<sha>                 upload a patch
//	POST /internal/platform/packages/releases                    publish (owner act, through the proxy)
//	GET  /internal/platform/packages/releases/<id>?owner_id=     one release and its listing
//	GET  /internal/platform/packages/catalog?owner_id=&q=        the packages the owner may see
//	GET  /internal/platform/packages/catalog/<name>/releases?owner_id=[&track=&limit=&before_seq=]   one track's history
//	GET  /internal/platform/packages/tracks/<name>/head?owner_id=        the owner's track head
//
// A guest never reaches them: publish is the owner's act through the proxy
// (L6). Every read names the owner it is for, as the proxy authenticated
// it; a read that names none is refused, never unscoped. Visibility is the
// listing's scope set (S3: owner, shared, public; listingVisible), and an
// unseen release answers exactly as a missing one.
func (h *Handler) HandlePackages(w http.ResponseWriter, r *http.Request) {
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "package routes are host only; publish is the owner's act through the proxy"})
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, packagesRoute), "/")
	parts := strings.Split(rest, "/")
	switch {
	case len(parts) == 2 && parts[0] == "blobs" && r.Method == http.MethodPut:
		h.putPackageBlob(w, r, parts[1])
	case rest == "releases" && r.Method == http.MethodPost:
		h.publishPackage(w, r)
	case len(parts) == 2 && parts[0] == "releases" && r.Method == http.MethodGet:
		h.readPackageRelease(w, r, strings.ToLower(parts[1]))
	case rest == "catalog" && r.Method == http.MethodGet:
		h.packageCatalog(w, r)
	case len(parts) == 3 && parts[0] == "catalog" && parts[2] == "releases" && r.Method == http.MethodGet:
		h.packageHistory(w, r, parts[1])
	case len(parts) == 3 && parts[0] == "tracks" && parts[2] == "head" && r.Method == http.MethodGet:
		h.packageTrackHead(w, r, parts[1])
	default:
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

func (h *Handler) putPackageBlob(w http.ResponseWriter, r *http.Request, digest string) {
	if !computerevent.IsSHA256(digest) {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "package blob: sha256 digest required"})
		return
	}
	store := h.contentStore(pkgrelease.ContentNamespace)
	// The cap holds at upload, so an oversize patch never lands (L27); the
	// store streams and hashes, never buffering.
	store.maxBytes = pkgrelease.MaxBlobBytes
	switch err := store.Put(digest, r.Body); {
	case err == nil:
		writeJSON(w, http.StatusCreated, map[string]string{"digest": digest})
	case errors.Is(err, errContentDigestMismatch):
		writeJSON(w, http.StatusBadRequest, apiError{Error: "package blob: digest mismatch"})
	case errors.Is(err, errContentTooLarge):
		size := "more than " + strconv.Itoa(pkgrelease.MaxBlobBytes)
		if r.ContentLength > 0 {
			size = strconv.FormatInt(r.ContentLength, 10)
		}
		writeJSON(w, http.StatusRequestEntityTooLarge, packageAPIError{
			Error:  fmt.Sprintf("package blob: %s bytes exceeds the %d byte (2 MiB) cap; the cap is raised only after S2's measurement, in a reviewed change", size, pkgrelease.MaxBlobBytes),
			Reason: pkgrelease.ReasonBlobTooLarge,
		})
	default:
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "package blob: write"})
	}
}

// writePackageRefusal maps a pkgrelease refusal to its status: what the
// host cannot verify, or a track that moved, is a conflict; a record or
// blob that fails a check is unprocessable.
func writePackageRefusal(w http.ResponseWriter, err error) {
	reason := pkgrelease.ReasonOf(err)
	status := http.StatusUnprocessableEntity
	switch reason {
	case pkgrelease.ReasonTrackMoved, pkgrelease.ReasonParentUnverifiable, pkgrelease.ReasonBlobMismatch,
		pkgrelease.ReasonRecordUnlisted, pkgrelease.ReasonListingConflict, pkgrelease.ReasonConfirmPathsRequired,
		pkgrelease.ReasonConfirmPathsMismatch, pkgrelease.ReasonListingState, pkgrelease.ReasonListingMoved,
		pkgrelease.ReasonOwnerActReplayed:
		status = http.StatusConflict
	case pkgrelease.ReasonBlobTooLarge:
		status = http.StatusRequestEntityTooLarge
	case "":
		writeJSON(w, http.StatusInternalServerError, packageAPIError{Error: "package publish: " + err.Error()})
		return
	}
	writeJSON(w, status, packageAPIError{Error: err.Error(), Reason: reason})
}

// writeListingsError answers a failed track read: a refusal by its reason,
// anything else as the store being unavailable.
func writeListingsError(w http.ResponseWriter, err error) {
	if pkgrelease.ReasonOf(err) != "" {
		writePackageRefusal(w, err)
		return
	}
	writeJSON(w, http.StatusInternalServerError, apiError{Error: "package listings unavailable"})
}

// PackagePublishSubject is the owner-act subject of a package publish: the
// release id and the one scope S2 lists at.
func PackagePublishSubject(releaseID string) map[string]any {
	return map[string]any{"release_id": strings.ToLower(strings.TrimSpace(releaseID)), "scopes": pkgrelease.ScopeOwner}
}

// packagePublishActor verifies the publish's owner-act receipt and names
// the listing's state actor. status is nonzero for a refusal.
func (h *Handler) packagePublishActor(r *http.Request, ownerID, computerID, releaseID string) (actor string, status int, refusal packageAPIError) {
	return h.packageActor(r, ownerID, computerID, owneract.ActPackagePublish, PackagePublishSubject(releaseID), "package publish")
}

// packageActor verifies an owner-act receipt of kind for ownerID on
// computerID whose subject is exactly subject, and names the listing's
// state actor. status is nonzero for a refusal.
func (h *Handler) packageActor(r *http.Request, ownerID, computerID, kind string, subject map[string]any, what string) (actor string, status int, refusal packageAPIError) {
	authority := h.packageOwnerActs
	if authority == nil {
		// No authority is a wiring fault, not observe: a red route's
		// unconfigured default refuses (review 274 F3). cmd/corpusd always
		// configures one, in the mode CHOIR_OWNER_ACT_MODE names.
		return "", http.StatusServiceUnavailable, packageAPIError{Error: what + ": no owner-act authority is configured", Reason: "owner_act_unconfigured"}
	}
	verified, err := func() (owneract.Verified, error) {
		receipt, err := owneract.Decode(r.Header.Get(owneract.Header))
		if err != nil {
			return owneract.Verified{}, err
		}
		verified, err := authority.verifier.Verify(receipt, computerID, kind, authority.now())
		if err != nil {
			return owneract.Verified{}, err
		}
		if verified.OwnerUserID != ownerID {
			return owneract.Verified{}, fmt.Errorf("owner act: the receipt does not bind this owner")
		}
		for key, want := range subject {
			if verified.SubjectString(key) != want {
				return owneract.Verified{}, fmt.Errorf("owner act: the receipt does not bind this %s", key)
			}
		}
		return verified, nil
	}()
	if err != nil {
		if refused := authority.refuse(ErrOwnerActRefused, what, computerID, err.Error()); refused != nil {
			return "", http.StatusForbidden, packageAPIError{Error: refused.Error()}
		}
		return "owner_act:unverified", 0, packageAPIError{}
	}
	// A receipt replayed inside its five minutes can only repeat the act it
	// names, and only a host-internal caller carries one: a publish is
	// idempotent on the release id; a listing act asks for one scope set or
	// state, which the owner set or will set again by a newer act.
	return "owner_act:" + verified.Digest, 0, packageAPIError{}
}

func packageReleaseCanonicalID(ownerID, releaseID string) (string, error) {
	return objectgraph.BuildCanonicalID(PackageReleaseKind, ownerID, releaseID)
}

func listingCanonicalID(ownerID, recordID string, version int) (string, error) {
	return objectgraph.BuildCanonicalID(ReleaseListingKind, ownerID, recordID+"-v"+strconv.Itoa(version))
}

// listingObject files one listing version as a Store B object.
func listingObject(ownerID, computerID string, listing pkgrelease.Listing, now time.Time) (objectgraph.Object, error) {
	listing = listing.Normalize()
	if err := listing.Validate(); err != nil {
		return objectgraph.Object{}, err
	}
	body, err := listing.Canonical()
	if err != nil {
		return objectgraph.Object{}, err
	}
	id, err := listingCanonicalID(ownerID, listing.RecordID, listing.Version)
	if err != nil {
		return objectgraph.Object{}, err
	}
	fields := map[string]any{
		"record_id": listing.RecordID, "record_kind": listing.RecordKind, "package": listing.Package,
		"track": listing.Track, "seq": listing.Seq, "state": listing.State, "version": listing.Version,
		"listed_at": now.UTC().Format(time.RFC3339Nano),
	}
	if listingWidened(listing) {
		// The index another owner's catalog finds this track by (S3): a
		// version that admits any reader beyond the owner. Whether the
		// track's current version still does is decided from the chain.
		fields["widened"] = "true"
	}
	metadata, err := objectgraph.NormalizeMetadata(fields)
	if err != nil {
		return objectgraph.Object{}, err
	}
	return objectgraph.Object{
		CanonicalID: id, ObjectKind: ReleaseListingKind, OwnerID: ownerID, ComputerID: computerID,
		VersionID: strconv.Itoa(listing.Version), Body: body, Metadata: metadata,
		ContentHash: objectgraph.ContentHash(ReleaseListingKind, body, metadata), CreatedAt: now, UpdatedAt: now,
	}, nil
}

// verifiedListing trusts nothing about an object but its body and its id:
// the body is a valid listing on ownerID's track, filed under the canonical
// id its record and version name.
func verifiedListing(obj objectgraph.Object, ownerID string) (pkgrelease.Listing, error) {
	if obj.ObjectKind != ReleaseListingKind || obj.Tombstone || obj.OwnerID != ownerID {
		return pkgrelease.Listing{}, objectgraph.ErrNotFound
	}
	listing, err := pkgrelease.DecodeListing(obj.Body)
	if err == nil {
		err = listing.Validate()
	}
	if err != nil {
		return pkgrelease.Listing{}, err
	}
	id, err := listingCanonicalID(ownerID, listing.RecordID, listing.Version)
	if err != nil || id != obj.CanonicalID || listing.Track != ownerID {
		return pkgrelease.Listing{}, objectgraph.ErrNotFound
	}
	return listing, nil
}

// trackListings is each record's current listing on ownerID's track of a
// package: the newest version whose chain of predecessor digests is intact
// back to version 1.
//
// It reads the track to its end, a page of trackPageSize at a time, so
// seq and the head are never counted from part of a track (review 274 F7)
// and a long track never stops taking releases (bunch279-I2).
func (h *Handler) trackListings(ctx context.Context, ownerID, pkg string) (map[string]pkgrelease.Listing, int, error) {
	chains, count, err := h.trackChains(ctx, ownerID, pkg)
	if err != nil {
		return nil, count, err
	}
	current := map[string]pkgrelease.Listing{}
	for recordID, chain := range chains {
		current[recordID] = chain[len(chain)-1]
	}
	return current, count, nil
}

// trackChains is each record's chain of listing versions on the track,
// version 1 first, up to the newest one whose predecessor digests are
// intact.
func (h *Handler) trackChains(ctx context.Context, ownerID, pkg string) (map[string][]pkgrelease.Listing, int, error) {
	objects, err := h.pagedListings(ctx, "$.package", pkg, ownerID)
	if err != nil {
		return nil, 0, err
	}
	versions := map[string]map[int]pkgrelease.Listing{}
	for _, obj := range objects {
		listing, err := verifiedListing(obj, ownerID)
		if err != nil || listing.Package != pkg {
			continue
		}
		if versions[listing.RecordID] == nil {
			versions[listing.RecordID] = map[int]pkgrelease.Listing{}
		}
		versions[listing.RecordID][listing.Version] = listing
	}
	chains := map[string][]pkgrelease.Listing{}
	for recordID, byVersion := range versions {
		listing, ok := byVersion[1]
		if !ok {
			continue
		}
		chain := []pkgrelease.Listing{listing}
		for next := 2; ; next++ {
			candidate, ok := byVersion[next]
			if !ok {
				break
			}
			digest, err := listing.Digest()
			if err != nil || candidate.PrevVersionDigest != digest || candidate.Seq != listing.Seq {
				break
			}
			listing = candidate
			chain = append(chain, listing)
		}
		chains[recordID] = chain
	}
	return chains, len(objects), nil
}

// pagedListings reads every listing object of ownerID matching
// (jsonPath = value; every one when jsonPath is empty), page by page. It
// ends on an empty page or one shorter than the limit the store applies
// (objectgraph.NormalizedLimit), never on the size it asked for, so a
// zero page size or a store that caps a page cannot end it early or never
// (review 291 F5). The query is the owner's: the (object_kind, owner_id)
// index bounds it by the owner's listings.
func (h *Handler) pagedListings(ctx context.Context, jsonPath, value, ownerID string) ([]objectgraph.Object, error) {
	store := h.releaseStore()
	if store == nil {
		return nil, errors.New("package store unavailable")
	}
	if ownerID == "" {
		return nil, errors.New("pagedListings: a listing read names its owner")
	}
	limit := objectgraph.NormalizedLimit(trackPageSize)
	var out []objectgraph.Object
	after := ""
	for {
		page, err := store.ListObjectsByMetadataPage(ctx, string(ReleaseListingKind), jsonPath, value, ownerID, after, limit)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		if len(page) == 0 || len(page) < limit {
			return out, nil
		}
		after = page[len(page)-1].CanonicalID
	}
}

// widenedListings reads every listing version on the cloud that admits
// readers beyond its owner, through the registered lookup key (review 291
// F1): with nothing widened it is one key seek that finds nothing. Before
// the keys are live it reads nothing (live is false): another owner's
// tracks are then missing from a catalog until the backfill lands, which
// is the narrower answer, never a scan.
func (h *Handler) widenedListings(ctx context.Context) (objects []objectgraph.Object, live bool, err error) {
	store := h.releaseStore()
	if store == nil {
		return nil, false, errors.New("package store unavailable")
	}
	if !store.LookupKeysLive() {
		return nil, false, nil
	}
	after := ""
	for {
		page, next, err := store.ListKeyedObjectsPage(ctx, string(ReleaseListingKind), "$.widened", "true", after, objectgraph.NormalizedLimit(trackPageSize))
		if err != nil {
			return nil, true, err
		}
		h.widenedListingReads.Add(int64(len(page)))
		objects = append(objects, page...)
		if next == "" {
			return objects, true, nil
		}
		after = next
	}
}

// trackHead is the latest listing on the track that can be a parent (not
// withdrawn, not yanked), and the highest seq the track has given.
func trackHead(listings map[string]pkgrelease.Listing) (head pkgrelease.Listing, found bool, maxSeq int) {
	for _, listing := range listings {
		if listing.Seq > maxSeq {
			maxSeq = listing.Seq
		}
		if pkgrelease.HeadEligible(listing.State) && (!found || listing.Seq > head.Seq) {
			head, found = listing, true
		}
	}
	return head, found, maxSeq
}

// listingVisible is the listing's scope set for a reader (L4): the owner;
// a named owner of a shared listing; anyone of a public one. Nobody else,
// an admin included. CHOIR_PACKAGE_SHARE_READS=owner reads every listing at
// owner scope, whatever its versions say.
func (h *Handler) listingVisible(listing pkgrelease.Listing, readerOwnerID string) bool {
	if h.packageShareReads == PackageShareReadsOwner {
		return readerOwnerID != "" && listing.Track == readerOwnerID
	}
	return listing.VisibleTo(readerOwnerID)
}

// listingFor is the listing as reader may see it: another owner never
// learns who else a shared listing names.
func listingFor(listing pkgrelease.Listing, reader string) pkgrelease.Listing {
	listing = listing.Normalize()
	if listing.Track == reader {
		return listing
	}
	named := []string{}
	for _, owner := range listing.SharedWith.Owners {
		if owner == reader {
			named = append(named, owner)
		}
	}
	listing.SharedWith = pkgrelease.SharedWith{Owners: named, Computers: []string{}}
	return listing
}

// readPackageRecord reads ownerID's release by id and re-verifies it from
// its body.
func (h *Handler) readPackageRecord(ctx context.Context, ownerID, releaseID string) (objectgraph.Object, pkgrelease.Record, error) {
	store := h.releaseStore()
	if store == nil {
		return objectgraph.Object{}, pkgrelease.Record{}, errors.New("package store unavailable")
	}
	id, err := packageReleaseCanonicalID(ownerID, releaseID)
	if err != nil {
		return objectgraph.Object{}, pkgrelease.Record{}, objectgraph.ErrNotFound
	}
	obj, err := store.GetObject(ctx, id)
	if err != nil {
		return objectgraph.Object{}, pkgrelease.Record{}, err
	}
	if obj.ObjectKind != PackageReleaseKind || obj.Tombstone || obj.OwnerID != ownerID {
		return objectgraph.Object{}, pkgrelease.Record{}, objectgraph.ErrNotFound
	}
	record, err := pkgrelease.Decode(obj.Body)
	if err == nil {
		err = pkgrelease.VerifyID(record, releaseID)
	}
	if err != nil {
		return objectgraph.Object{}, pkgrelease.Record{}, err
	}
	if record.Publisher.OwnerID != ownerID {
		return objectgraph.Object{}, pkgrelease.Record{}, objectgraph.ErrNotFound
	}
	return obj, record, nil
}

// writePackageJSON writes a response that carries stored records: without
// HTML escaping, so a record travels as the canonical bytes its id hashes
// (an operation id or a declaration may hold < > or &).
func writePackageJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(v); err != nil {
		log.Printf("corpusd: json encode: %v", err)
	}
}

func packageView(obj objectgraph.Object, releaseID string, listing pkgrelease.Listing) PackageReleaseView {
	var metadata struct {
		PublishedAt string `json:"published_at"`
	}
	_ = json.Unmarshal(obj.Metadata, &metadata)
	return PackageReleaseView{ReleaseID: releaseID, CanonicalID: obj.CanonicalID, PublishedAt: metadata.PublishedAt, Record: json.RawMessage(obj.Body), Listing: listing}
}

func (h *Handler) readPackageBlob(blob pkgrelease.Blob) ([]byte, error) {
	raw, err := h.contentStore(pkgrelease.ContentNamespace).ReadVerified(blob.SHA256, pkgrelease.MaxBlobBytes)
	if err != nil {
		return nil, &pkgrelease.Refusal{Reason: pkgrelease.ReasonBlobMismatch, Detail: "blob " + blob.SHA256 + " is not in the host content store as the record names it"}
	}
	if err := pkgrelease.VerifyBlob(blob, raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (h *Handler) publishPackage(w http.ResponseWriter, r *http.Request) {
	store := h.releaseStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "package store unavailable"})
		return
	}
	var request packagePublishRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, pkgrelease.MaxRecordBytes+(64<<10)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid package publish request"})
		return
	}
	record, err := pkgrelease.Decode(request.Record)
	if err == nil {
		err = pkgrelease.VerifyID(record, request.ReleaseID)
	}
	if err != nil {
		writePackageRefusal(w, err)
		return
	}
	releaseID := strings.ToLower(strings.TrimSpace(request.ReleaseID))
	ownerID, computerID := strings.TrimSpace(request.OwnerID), strings.TrimSpace(request.ComputerID)
	// The proxy names the owner and computer it authorized; the record must
	// be theirs, and its track is the owner's (3.4).
	if ownerID == "" || computerID == "" || record.Publisher != (pkgrelease.Publisher{OwnerID: ownerID, ComputerID: computerID}) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "the record's publisher is not the owner and computer that published it"})
		return
	}
	actor, status, refusal := h.packagePublishActor(r, ownerID, computerID, releaseID)
	if status != 0 {
		writeJSON(w, status, refusal)
		return
	}
	in := pkgrelease.HostInput{Record: record, Base: h.packageBase}
	if in.TreePatch, err = h.readPackageBlob(record.Source.TreePatch); err == nil {
		in.ReviewPatch, err = h.readPackageBlob(record.Source.ReviewPatch)
	}
	if err == nil && record.ParentPatch != nil {
		in.ParentPatch, err = h.readPackageBlob(*record.ParentPatch)
	}
	if err != nil {
		writePackageRefusal(w, err)
		return
	}

	// One publish at a time: the track head and the next seq are read and
	// written under this lock, so two publishes cannot both take the head as
	// their parent. The lock is the whole guarantee: it assumes one corpusd
	// writes the store (it does; see packageMu). The store has no unique
	// (owner, package, seq) constraint.
	h.packageMu.Lock()
	defer h.packageMu.Unlock()
	ctx := r.Context()
	listings, _, err := h.trackListings(ctx, ownerID, record.Package.Name)
	if err != nil {
		writeListingsError(w, err)
		return
	}
	existing, _, existingErr := h.readPackageRecord(ctx, ownerID, releaseID)
	if existingErr == nil {
		if listing, ok := listings[releaseID]; ok {
			// A republish names the first object and its listing, and never
			// takes a new seq.
			writePackageJSON(w, http.StatusOK, packageView(existing, releaseID, listing))
			return
		}
		// The record was stored and its listing was not (a crash between the
		// two writes): list it now, under the same checks.
	}
	head, hasHead, maxSeq := trackHead(listings)
	// moved refuses a parent that is not the head. A record stored without
	// its listing gets its own reason: it stays stored and unlisted, inert,
	// and the owner publishes again on the head (review 274 F6).
	moved := func(detail string) {
		reason := pkgrelease.ReasonTrackMoved
		if existingErr == nil {
			reason = pkgrelease.ReasonRecordUnlisted
			detail = "this release's record was stored without its listing (a publish interrupted between the two writes), and the track has moved since: " +
				detail + "; the record stays unlisted and is no release, so publish again on the head"
		}
		writePackageRefusal(w, &pkgrelease.Refusal{Reason: reason, Detail: detail})
	}
	switch record.Parent.Kind {
	case pkgrelease.ParentBase:
		if hasHead {
			moved(fmt.Sprintf("the track already has release %s (seq %d) as its head; a new release's parent is that head, not the base", head.RecordID, head.Seq))
			return
		}
	case pkgrelease.ParentRelease:
		if !hasHead || head.RecordID != record.Parent.ReleaseID {
			detail := "the track has no listed release, so a first release's parent is the base"
			if hasHead {
				detail = fmt.Sprintf("the parent %s is not the track head; the head is %s (seq %d)", record.Parent.ReleaseID, head.RecordID, head.Seq)
			}
			moved(detail)
			return
		}
		_, parent, err := h.readPackageRecord(ctx, ownerID, record.Parent.ReleaseID)
		if err != nil {
			writePackageRefusal(w, &pkgrelease.Refusal{Reason: pkgrelease.ReasonParentMismatch, Detail: "the parent release is unreadable: " + err.Error()})
			return
		}
		in.ParentRelease = &parent
		if in.ParentTreePatch, err = h.readPackageBlob(parent.Source.TreePatch); err != nil {
			writePackageRefusal(w, err)
			return
		}
	}
	if err := pkgrelease.VerifyHost(ctx, in); err != nil {
		writePackageRefusal(w, err)
		return
	}

	now := time.Now().UTC()
	obj := existing
	inserted := false
	if existingErr != nil {
		body, err := record.Canonical()
		if err != nil {
			writePackageRefusal(w, err)
			return
		}
		canonicalID, err := packageReleaseCanonicalID(ownerID, releaseID)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: err.Error()})
			return
		}
		metadata, err := objectgraph.NormalizeMetadata(map[string]any{
			"release_id": releaseID, "package": record.Package.Name, "track": record.Track,
			"publisher_computer_id": computerID, "operation_id": record.Operation.OperationID,
			"tree": record.Source.Tree, "parent_kind": record.Parent.Kind, "published_at": now.Format(time.RFC3339Nano),
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "package metadata"})
			return
		}
		obj = objectgraph.Object{
			CanonicalID: canonicalID, ObjectKind: PackageReleaseKind, OwnerID: ownerID, ComputerID: computerID,
			VersionID: releaseID, Body: body, Metadata: metadata,
			ContentHash: objectgraph.ContentHash(PackageReleaseKind, body, metadata), CreatedAt: now, UpdatedAt: now,
		}
		if inserted, err = store.InsertObjectIfAbsent(ctx, obj); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "package publish: " + err.Error()})
			return
		}
		if !inserted {
			if obj, _, err = h.readPackageRecord(ctx, ownerID, releaseID); err != nil {
				writeJSON(w, http.StatusInternalServerError, apiError{Error: "package publish: read back"})
				return
			}
		}
	}
	listing := pkgrelease.Listing{
		RecordKind: pkgrelease.RecordKindPackageRelease, RecordID: releaseID, Package: record.Package.Name,
		Track: record.Track, Seq: maxSeq + 1, Scopes: []string{pkgrelease.ScopeOwner},
		State: pkgrelease.StateListed, StateReason: "published", StateActor: actor, Version: 1,
	}
	listingObj, err := listingObject(ownerID, computerID, listing, now)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "package listing: " + err.Error()})
		return
	}
	listed, err := store.InsertObjectIfAbsent(ctx, listingObj)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "package listing: " + err.Error()})
		return
	}
	if !listed {
		// The id is held by an object the track read did not take for this
		// release's listing (it does not verify): this publish listed
		// nothing, and says so rather than describe a listing it did not
		// store.
		writePackageRefusal(w, &pkgrelease.Refusal{Reason: pkgrelease.ReasonListingConflict, Detail: "the listing id " + listingObj.CanonicalID +
			" is held by an object that is not this release's listing; the record is stored and unlisted"})
		return
	}
	view := packageView(obj, releaseID, listing.Normalize())
	view.Published = true
	writePackageJSON(w, http.StatusCreated, view)
}

// packageReader is the owner a host read is for. A read that names none is
// refused: S2 has no unscoped package read.
func packageReader(w http.ResponseWriter, r *http.Request) (string, bool) {
	owner := strings.TrimSpace(r.URL.Query().Get("owner_id"))
	if owner == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "owner_id is required: every package read is for one owner"})
		return "", false
	}
	return owner, true
}

var errPackageNotPublished = apiError{Error: "package release not published"}

func (h *Handler) readPackageRelease(w http.ResponseWriter, r *http.Request, releaseID string) {
	reader, ok := packageReader(w, r)
	if !ok {
		return
	}
	if !computerevent.IsSHA256(releaseID) {
		writeJSON(w, http.StatusNotFound, errPackageNotPublished)
		return
	}
	// A release the reader may not see answers exactly as a missing one.
	obj, record, err := h.findPackageRecord(r.Context(), reader, releaseID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errPackageNotPublished)
		return
	}
	listings, _, err := h.trackListings(r.Context(), record.Publisher.OwnerID, record.Package.Name)
	if err != nil {
		writeListingsError(w, err)
		return
	}
	listing, ok := listings[releaseID]
	if !ok || !h.listingVisible(listing, reader) {
		writeJSON(w, http.StatusNotFound, errPackageNotPublished)
		return
	}
	writePackageJSON(w, http.StatusOK, packageView(obj, releaseID, listingFor(listing, reader)))
}

// findPackageRecord is the release by id: the reader's own, or another
// owner's found by its id. Visibility is the caller's to decide from the
// listing; under CHOIR_PACKAGE_SHARE_READS=owner only the reader's own is
// looked for.
func (h *Handler) findPackageRecord(ctx context.Context, reader, releaseID string) (objectgraph.Object, pkgrelease.Record, error) {
	obj, record, err := h.readPackageRecord(ctx, reader, releaseID)
	if err == nil || h.packageShareReads == PackageShareReadsOwner {
		return obj, record, err
	}
	store := h.releaseStore()
	if store == nil {
		return objectgraph.Object{}, pkgrelease.Record{}, errors.New("package store unavailable")
	}
	// release_id is a registered lookup key (review 291 F2): the read
	// seeks it. Before the keys are live it would scan the kind, so the
	// read answers as missing instead, the narrower answer.
	if !store.LookupKeysLive() {
		return objectgraph.Object{}, pkgrelease.Record{}, objectgraph.ErrNotFound
	}
	found, err := store.GetObjectByMetadata(ctx, string(PackageReleaseKind), "$.release_id", releaseID)
	if err != nil {
		return objectgraph.Object{}, pkgrelease.Record{}, err
	}
	// Re-read under the owner the object is filed under, so the id, the
	// owner and the body verify as for the owner's own read.
	return h.readPackageRecord(ctx, found.OwnerID, releaseID)
}

// PackageCatalogEntry is one package on one track the reader may see.
type PackageCatalogEntry struct {
	Package       string `json:"package"`
	Track         string `json:"track"`
	HeadReleaseID string `json:"head_release_id,omitempty"`
	HeadSeq       int    `json:"head_seq,omitempty"`
	Releases      int    `json:"releases"`
}

func (h *Handler) packageCatalog(w http.ResponseWriter, r *http.Request) {
	reader, ok := packageReader(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	matches := func(name string) bool { return query == "" || strings.Contains(name, query) }
	type trackKey struct{ track, pkg string }
	tracks := map[trackKey]bool{}
	// The reader's own tracks.
	own, err := h.pagedListings(ctx, "", "", reader)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "package catalog unavailable"})
		return
	}
	for _, obj := range own {
		if listing, err := verifiedListing(obj, reader); err == nil && matches(listing.Package) {
			tracks[trackKey{reader, listing.Package}] = true
		}
	}
	// Other owners' tracks with a version that ever admitted this reader;
	// whether the current version still does is decided below.
	if h.packageShareReads != PackageShareReadsOwner {
		widened, _, err := h.widenedListings(ctx)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "package catalog unavailable"})
			return
		}
		for _, obj := range widened {
			if obj.OwnerID == reader {
				continue
			}
			if listing, err := verifiedListing(obj, obj.OwnerID); err == nil && listing.VisibleTo(reader) && matches(listing.Package) {
				tracks[trackKey{obj.OwnerID, listing.Package}] = true
			}
		}
	}
	entries := []PackageCatalogEntry{}
	for key := range tracks {
		listings, _, err := h.trackListings(ctx, key.track, key.pkg)
		if err != nil {
			writeListingsError(w, err)
			return
		}
		visible := map[string]pkgrelease.Listing{}
		for id, listing := range listings {
			if h.listingVisible(listing, reader) {
				visible[id] = listing
			}
		}
		if len(visible) == 0 {
			continue
		}
		head, found, _ := trackHead(visible)
		entry := PackageCatalogEntry{Package: key.pkg, Track: key.track, Releases: len(visible)}
		if found {
			entry.HeadReleaseID, entry.HeadSeq = head.RecordID, head.Seq
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Package != entries[j].Package {
			return entries[i].Package < entries[j].Package
		}
		return entries[i].Track < entries[j].Track
	})
	// reader is the caller's own owner id: what another owner names to
	// share with it.
	writeJSON(w, http.StatusOK, map[string]any{"packages": entries, "count": len(entries), "reader": reader})
}

// History pages: newest first, at most maxHistoryPage releases a page.
const (
	defaultHistoryPage = 100
	maxHistoryPage     = 500
)

// packageHistory is one track's version history, newest first, as reader
// may see it: the reader's own track unless track names another. A track
// that shows the reader nothing is 200 with nothing (L4). before_seq pages:
// the page holds releases below it, and next_before_seq names the next
// page when there is one.
func (h *Handler) packageHistory(w http.ResponseWriter, r *http.Request, name string) {
	reader, ok := packageReader(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	track := strings.TrimSpace(q.Get("track"))
	if track == "" {
		track = reader
	}
	limit := defaultHistoryPage
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "limit is a positive number"})
			return
		}
		limit = min(n, maxHistoryPage)
	}
	before := 0
	if raw := strings.TrimSpace(q.Get("before_seq")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "before_seq is a positive number"})
			return
		}
		before = n
	}
	views := []PackageReleaseView{}
	next := 0
	if objectgraph.ValidPackageName(name) && pkgrelease.ValidTrack(track) {
		listings, _, err := h.trackListings(r.Context(), track, name)
		if err != nil {
			writeListingsError(w, err)
			return
		}
		visible := []pkgrelease.Listing{}
		for _, listing := range listings {
			if h.listingVisible(listing, reader) && (before == 0 || listing.Seq < before) {
				visible = append(visible, listing)
			}
		}
		sort.Slice(visible, func(i, j int) bool { return visible[i].Seq > visible[j].Seq })
		for _, listing := range visible {
			if len(views) == limit {
				// More remain: the next page starts below the last one served.
				next = views[len(views)-1].Listing.Seq
				break
			}
			obj, _, err := h.readPackageRecord(r.Context(), track, listing.RecordID)
			if err != nil {
				continue // a record that no longer verifies is not served
			}
			views = append(views, packageView(obj, listing.RecordID, listingFor(listing, reader)))
		}
	}
	out := map[string]any{"package": name, "track": track, "releases": views, "count": len(views)}
	if next > 0 {
		out["next_before_seq"] = next
	}
	writePackageJSON(w, http.StatusOK, out)
}

// PackageTrackHead is a track's head: the parent the next release names.
type PackageTrackHead struct {
	ReleaseID string `json:"release_id"`
	Tree      string `json:"tree"`
	Seq       int    `json:"seq"`
}

func (h *Handler) packageTrackHead(w http.ResponseWriter, r *http.Request, name string) {
	reader, ok := packageReader(w, r)
	if !ok {
		return
	}
	noHead := packageAPIError{Error: "no listed release on this track; a first release's parent is the base", Reason: "no_track_head"}
	if !objectgraph.ValidPackageName(name) {
		writeJSON(w, http.StatusNotFound, noHead)
		return
	}
	listings, _, err := h.trackListings(r.Context(), reader, name)
	if err != nil {
		writeListingsError(w, err)
		return
	}
	head, found, _ := trackHead(listings)
	// The head a publish drafts on is the reader's own track's, never a
	// track shared with it (L24).
	if !found || head.Track != reader {
		writeJSON(w, http.StatusNotFound, noHead)
		return
	}
	_, record, err := h.readPackageRecord(r.Context(), reader, head.RecordID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "the track head's record is unreadable"})
		return
	}
	writeJSON(w, http.StatusOK, PackageTrackHead{ReleaseID: head.RecordID, Tree: record.Source.Tree, Seq: head.Seq})
}
