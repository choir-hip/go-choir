package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// maxTrackListings bounds one (owner, package) listing read. S2 lists one
// version per release; S3's state changes add versions.
const maxTrackListings = 1000

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
//	GET  /internal/platform/packages/catalog/<name>/releases?owner_id=   a package's history
//	GET  /internal/platform/packages/tracks/<name>/head?owner_id=        the owner's track head
//
// A guest never reaches them: publish is the owner's act through the proxy
// (L6), and S2's reads are the owner's own, named by the proxy. Every read
// names the owner it is for; a read that names none is refused, never
// unscoped. Visibility is the listing's scope set, which S2 holds at owner.
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
	case pkgrelease.ReasonTrackMoved, pkgrelease.ReasonParentUnverifiable, pkgrelease.ReasonBlobMismatch:
		status = http.StatusConflict
	case pkgrelease.ReasonBlobTooLarge:
		status = http.StatusRequestEntityTooLarge
	case "":
		writeJSON(w, http.StatusInternalServerError, packageAPIError{Error: "package publish: " + err.Error()})
		return
	}
	writeJSON(w, status, packageAPIError{Error: err.Error(), Reason: reason})
}

// PackagePublishSubject is the owner-act subject of a package publish: the
// release id and the one scope S2 lists at.
func PackagePublishSubject(releaseID string) map[string]any {
	return map[string]any{"release_id": strings.ToLower(strings.TrimSpace(releaseID)), "scopes": pkgrelease.ScopeOwner}
}

// packagePublishActor verifies the publish's owner-act receipt and names
// the listing's state actor. status is nonzero for a refusal.
func (h *Handler) packagePublishActor(r *http.Request, ownerID, computerID, releaseID string) (actor string, status int, refusal string) {
	authority := h.packageOwnerActs
	if authority == nil {
		return "owner_act:unverified", 0, ""
	}
	verified, err := func() (owneract.Verified, error) {
		receipt, err := owneract.Decode(r.Header.Get(owneract.Header))
		if err != nil {
			return owneract.Verified{}, err
		}
		verified, err := authority.verifier.Verify(receipt, computerID, owneract.ActPackagePublish, authority.now())
		if err != nil {
			return owneract.Verified{}, err
		}
		want := PackagePublishSubject(releaseID)
		if verified.OwnerUserID != ownerID || verified.SubjectString("release_id") != want["release_id"] || verified.SubjectString("scopes") != want["scopes"] {
			return owneract.Verified{}, fmt.Errorf("owner act: the receipt does not bind this owner, release and scope")
		}
		return verified, nil
	}()
	if err != nil {
		if refused := authority.refuse(ErrOwnerActRefused, "package publish", computerID, err.Error()); refused != nil {
			return "", http.StatusForbidden, refused.Error()
		}
		return "owner_act:unverified", 0, ""
	}
	// A receipt replayed inside its five minutes can only republish the
	// release it names, which is idempotent.
	return "owner_act:" + verified.Digest, 0, ""
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
	metadata, err := objectgraph.NormalizeMetadata(map[string]any{
		"record_id": listing.RecordID, "record_kind": listing.RecordKind, "package": listing.Package,
		"track": listing.Track, "seq": listing.Seq, "state": listing.State, "version": listing.Version,
		"listed_at": now.UTC().Format(time.RFC3339Nano),
	})
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
func (h *Handler) trackListings(ctx context.Context, ownerID, pkg string) (map[string]pkgrelease.Listing, error) {
	store := h.releaseStore()
	if store == nil {
		return nil, errors.New("package store unavailable")
	}
	objects, err := store.ListObjectsByMetadataForOwner(ctx, string(ReleaseListingKind), "$.package", pkg, ownerID, maxTrackListings)
	if err != nil {
		return nil, err
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
	current := map[string]pkgrelease.Listing{}
	for recordID, byVersion := range versions {
		listing, ok := byVersion[1]
		if !ok {
			continue
		}
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
		}
		current[recordID] = listing
	}
	return current, nil
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

// listingVisible is the listing's scope set for a reader. S2 lists at owner
// only: the owner sees it, and nobody else, an admin included (L4).
func listingVisible(listing pkgrelease.Listing, readerOwnerID string) bool {
	for _, scope := range listing.Scopes {
		if scope == pkgrelease.ScopeOwner && listing.Track == readerOwnerID {
			return true
		}
	}
	return false
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
		writeJSON(w, status, apiError{Error: refusal})
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
	// their parent (corpusd is one process).
	h.packageMu.Lock()
	defer h.packageMu.Unlock()
	ctx := r.Context()
	listings, err := h.trackListings(ctx, ownerID, record.Package.Name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "package listings unavailable"})
		return
	}
	existing, _, existingErr := h.readPackageRecord(ctx, ownerID, releaseID)
	if existingErr == nil {
		if listing, ok := listings[releaseID]; ok {
			// A republish names the first object and its listing, and never
			// takes a new seq.
			writeJSON(w, http.StatusOK, packageView(existing, releaseID, listing))
			return
		}
		// The record was stored and its listing was not (a crash between the
		// two writes): list it now, under the same checks.
	}
	head, hasHead, maxSeq := trackHead(listings)
	switch record.Parent.Kind {
	case pkgrelease.ParentBase:
		if hasHead {
			writePackageRefusal(w, &pkgrelease.Refusal{Reason: pkgrelease.ReasonTrackMoved, Detail: fmt.Sprintf(
				"the track already has release %s (seq %d) as its head; a new release's parent is that head, not the base", head.RecordID, head.Seq)})
			return
		}
	case pkgrelease.ParentRelease:
		if !hasHead || head.RecordID != record.Parent.ReleaseID {
			detail := "the track has no listed release, so a first release's parent is the base"
			if hasHead {
				detail = fmt.Sprintf("the parent %s is not the track head; the head is %s (seq %d)", record.Parent.ReleaseID, head.RecordID, head.Seq)
			}
			writePackageRefusal(w, &pkgrelease.Refusal{Reason: pkgrelease.ReasonTrackMoved, Detail: detail})
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
	if _, err := store.InsertObjectIfAbsent(ctx, listingObj); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "package listing: " + err.Error()})
		return
	}
	view := packageView(obj, releaseID, listing.Normalize())
	view.Published = true
	writeJSON(w, http.StatusCreated, view)
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
	obj, record, err := h.readPackageRecord(r.Context(), reader, releaseID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errPackageNotPublished)
		return
	}
	listings, err := h.trackListings(r.Context(), reader, record.Package.Name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "package listings unavailable"})
		return
	}
	listing, ok := listings[releaseID]
	if !ok || !listingVisible(listing, reader) {
		writeJSON(w, http.StatusNotFound, errPackageNotPublished)
		return
	}
	writeJSON(w, http.StatusOK, packageView(obj, releaseID, listing))
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
	store := h.releaseStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "package store unavailable"})
		return
	}
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	objects, err := store.ListObjects(r.Context(), objectgraph.ListFilter{Kind: ReleaseListingKind, OwnerID: reader, Limit: maxTrackListings})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "package catalog unavailable"})
		return
	}
	names := map[string]bool{}
	for _, obj := range objects {
		if listing, err := verifiedListing(obj, reader); err == nil && (query == "" || strings.Contains(listing.Package, query)) {
			names[listing.Package] = true
		}
	}
	entries := []PackageCatalogEntry{}
	for name := range names {
		listings, err := h.trackListings(r.Context(), reader, name)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "package catalog unavailable"})
			return
		}
		visible := map[string]pkgrelease.Listing{}
		for id, listing := range listings {
			if listingVisible(listing, reader) {
				visible[id] = listing
			}
		}
		if len(visible) == 0 {
			continue
		}
		head, found, _ := trackHead(visible)
		entry := PackageCatalogEntry{Package: name, Track: reader, Releases: len(visible)}
		if found {
			entry.HeadReleaseID, entry.HeadSeq = head.RecordID, head.Seq
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Package < entries[j].Package })
	writeJSON(w, http.StatusOK, map[string]any{"packages": entries, "count": len(entries)})
}

func (h *Handler) packageHistory(w http.ResponseWriter, r *http.Request, name string) {
	reader, ok := packageReader(w, r)
	if !ok {
		return
	}
	views := []PackageReleaseView{}
	if objectgraph.ValidPackageName(name) {
		listings, err := h.trackListings(r.Context(), reader, name)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "package history unavailable"})
			return
		}
		for id, listing := range listings {
			if !listingVisible(listing, reader) {
				continue
			}
			obj, _, err := h.readPackageRecord(r.Context(), reader, id)
			if err != nil {
				continue // a record that no longer verifies is not served
			}
			views = append(views, packageView(obj, id, listing))
		}
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Listing.Seq > views[j].Listing.Seq })
	writeJSON(w, http.StatusOK, map[string]any{"package": name, "releases": views, "count": len(views)})
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
	listings, err := h.trackListings(r.Context(), reader, name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "package listings unavailable"})
		return
	}
	head, found, _ := trackHead(listings)
	if !found || !listingVisible(head, reader) {
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
