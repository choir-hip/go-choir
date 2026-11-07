package platform

// A package operation decides, publishes and installs with no owner act
// (docs/design/package-operation-decision-2026-11-06.md, slices P1 and P2).
//
// P1: the capsule-verifier seat of a package decision. corpusd does not build
// a package (the poster's host build refuses package paths); it drafts the
// release itself from the frozen source patch over a platform base it holds,
// runs every host check on the draft, and votes on the draft content digest.
// P2: the publish route that spends that decision on one release, on the
// computer's own credential, with no owner receipt.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/decisionpolicy"
	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/hostbuild"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/packagepath"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// The decision kinds. A request that names none is a platform decision.
const (
	ConsensusKindPlatform = "platform"
	ConsensusKindPackage  = "package"
)

const (
	// packagePublishGrace is how long after a decision's not_after its
	// publish is still taken: the guest drafts, checks and builds the
	// package between the decision and the publish (2.4).
	packagePublishGrace = 30 * time.Minute
	// maxPackagePublishPayload bounds the decoded bytes of one publish: eight
	// screenshots at 2 MiB, one 8 MiB video, three patches and the source
	// patch at 2 MiB each, with the record, rounded up (2.4). The JSON body
	// carries them as base64, so its own limit is a third larger.
	maxPackagePublishPayload = 40 << 20
	maxPackagePublishBody    = maxPackagePublishPayload/3*4 + (1 << 20)

	reasonDecisionConsumed  = "decision_consumed"
	reasonDecisionExpired   = "decision_expired"
	reasonDecisionMismatch  = "decision_mismatch"
	reasonPublisherAddress  = "publisher_address_required"
	reasonPublisherLookup   = "publisher_address_unavailable"
	reasonPublishBusy       = "publish_busy"
	reasonDraftDigest       = "draft_digest_mismatch"
	reasonMediaNotFrozen    = "media_not_frozen"
	reasonPublishAuthority  = "publish_authority_refused"
	reasonBaseUnavailable   = "claimed_base_unavailable"
	reasonPackageNotDecided = "decision_not_a_package_decision"
)

// workspacePlatformPrefix is where a bundle's ordered file effects name the
// platform source tree.
const workspacePlatformPrefix = "workspace/platform/"

// ClassifyOperation is the one classification of an operation by its frozen
// patch, with the bundle's ordered file effects in agreement (2.1). It
// returns ConsensusKindPackage and the package name for a change that is
// entirely under one package root, and ConsensusKindPlatform ("" name) for
// one with no package path. A change that mixes layers or packages, or whose
// effects disagree with its patch (an effect outside the patch's package
// root, or a package effect with a platform patch: a regenerated-identical
// file can appear as an effect with no diff hunk), is an error. An empty
// patch is a platform change.
func ClassifyOperation(patch, bundle []byte) (kind, name string, err error) {
	var fields struct {
		Effects []struct {
			Path string `json:"path"`
		} `json:"ordered_file_effects"`
	}
	if err := json.NewDecoder(bytes.NewReader(bundle)).Decode(&fields); err != nil {
		return "", "", errors.New("the bundle does not decode")
	}
	var effects []string
	for _, effect := range fields.Effects {
		p := strings.TrimLeft(strings.TrimSpace(effect.Path), "/")
		if rel, ok := strings.CutPrefix(p, workspacePlatformPrefix); ok {
			effects = append(effects, rel)
		}
	}
	var paths []string
	if len(patch) > 0 {
		if paths, err = hostbuild.PatchPaths(patch); err != nil {
			return "", "", fmt.Errorf("the source patch is unreadable: %v", err)
		}
	}
	byPatch, byEffects := packagepath.Classify(paths), packagepath.Classify(effects)
	switch {
	case byPatch.Mixed():
		return "", "", fmt.Errorf("%s: %s", packagepath.ReasonPackagePath, byPatch.MixedDetail())
	case byEffects.Mixed():
		return "", "", fmt.Errorf("%s: the bundle's effects mix layers or packages: %s", packagepath.ReasonPackagePath, byEffects.MixedDetail())
	case byPatch.PackageOnly():
		if !byEffects.PackageOnly() || byEffects.Roots[0] != byPatch.Roots[0] {
			// An effect with no hunk, or one in another place, is outside
			// the package root the patch changes.
			return "", "", fmt.Errorf("%s: the bundle's file effects (%s) are not entirely under the patch's package %s", packagepath.ReasonPackagePath, packagepath.Shown(effects, 5), byPatch.Roots[0])
		}
		return ConsensusKindPackage, byPatch.Name(), nil
	case len(byPatch.Package) == 0 && len(byEffects.Package) > 0:
		return "", "", fmt.Errorf("%s: the bundle's file effects carry the package paths %s but its patch changes none", packagepath.ReasonPackagePath, packagepath.Shown(byEffects.Package, 5))
	}
	return ConsensusKindPlatform, "", nil
}

// packageBundleFields is what the seat reads of the frozen bundle bytes.
type packageBundleFields struct {
	SourcePatchSHA256     string `json:"source_patch_sha256"`
	SourcePatchBaseCommit string `json:"source_patch_base_commit"`
	Screens               []struct {
		SHA256 string `json:"sha256"`
	} `json:"screens"`
	Video *struct {
		SHA256 string `json:"sha256"`
	} `json:"video"`
}

func (f packageBundleFields) mediaDigests() []string {
	var out []string
	for _, s := range f.Screens {
		out = append(out, strings.ToLower(strings.TrimSpace(s.SHA256)))
	}
	if f.Video != nil {
		out = append(out, strings.ToLower(strings.TrimSpace(f.Video.SHA256)))
	}
	sort.Strings(out)
	return out
}

// packageSeatResult is what the package seat derived and voted on.
type packageSeatResult struct {
	name              string
	draftDigest       string
	verdictDigest     string
	sourcePatchSHA256 string
	claimedBase       string
	facts             pkgrelease.DraftFacts
	media             []string
}

func (p *packageSeatResult) receiptBlock() map[string]any {
	parent := map[string]any{"kind": p.facts.Parent.Kind, "tree": p.facts.Parent.Tree}
	if p.facts.Parent.Commit != "" {
		parent["commit"] = p.facts.Parent.Commit
	}
	if p.facts.Parent.ReleaseID != "" {
		parent["release_id"] = p.facts.Parent.ReleaseID
	}
	return map[string]any{
		"draft_digest": p.draftDigest, "verdict_digest": p.verdictDigest, "name": p.name,
		"root": p.facts.Root, "tree": p.facts.Tree, "changed_paths": p.facts.ChangedPaths, "parent": parent,
		"manifest_version": p.facts.ManifestVersion, "claimed_platform_base": p.claimedBase,
		"source_patch_sha256": p.sourcePatchSHA256,
	}
}

func (p *packageSeatResult) mediaDigestsJSON() string {
	raw, _ := json.Marshal(append([]string{}, p.media...))
	return string(raw)
}

// id is the decision's job-or-draft identity: a platform decision's host
// build job id, a package decision's draft content digest (2.2).
func (v hostBuildVote) id() string {
	if v.pkg != nil {
		return v.pkg.draftDigest
	}
	return v.job.JobID
}

func (v hostBuildVote) reasonLabel() string {
	if v.pkg != nil {
		return "package draft"
	}
	return "host build"
}

func (v hostBuildVote) seatDescription(keyID string) string {
	if v.pkg != nil {
		return "corpusd package seat " + v.pkg.draftDigest + " (platform-control " + keyID + "): the bundle's source patch drafted as a package release over a platform base corpusd holds, and every host check run on the draft"
	}
	return "corpusd host build " + v.job.JobID + " (platform-control " + keyID + "): the bundle's source patch built at its base commit on the computer's booted base"
}

// bootedBaseCommits is the host build service as the package seat reads it:
// the build commit of the image a computer booted.
type bootedBaseCommits interface {
	BootedBaseCommit(bootedBase string) (string, bool)
}

// maxConcurrentPackagePublishes bounds the by-decision publishes in flight:
// each holds up to a 54 MB body and runs git.
const maxConcurrentPackagePublishes = 2

var packagePublishSlots = make(chan struct{}, maxConcurrentPackagePublishes)

var gitCommitID = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

// castPackageSeatBallot is the capsule-verifier seat of a package decision.
// It checks the patch against the bundle, classifies it, binds the claimed
// platform base, drafts the release over that base with the owner's track
// head, runs every host check on the draft and votes on the draft content
// digest. A request the poster will not act on is a refusal; a draft that
// fails a host check is a reject vote with the reason, as a failed host
// build is.
func (a *ConsensusActuator) castPackageSeatBallot(ctx context.Context, computerID string, request ConsensusRequest, policyDigest, subjectDigest string, now time.Time) (hostBuildVote, error) {
	if a.seat == nil {
		return hostBuildVote{}, fmt.Errorf("%w: no package store configured", ErrConsensusUnavailable)
	}
	refuse := func(format string, args ...any) (hostBuildVote, error) {
		return hostBuildVote{}, fmt.Errorf("%w: "+format, append([]any{ErrConsensusRefused}, args...)...)
	}
	if strings.TrimSpace(request.OwnerID) == "" {
		return refuse("a package decision needs the computer's owner, and this caller has none bound")
	}
	var bundle packageBundleFields
	if err := json.Unmarshal(request.Bundle, &bundle); err != nil {
		return refuse("bundle bytes do not decode")
	}
	patchSHA := strings.ToLower(strings.TrimSpace(bundle.SourcePatchSHA256))
	sum := sha256.Sum256(request.SourcePatch)
	if !computerevent.IsSHA256(patchSHA) || len(request.SourcePatch) == 0 || hex.EncodeToString(sum[:]) != patchSHA {
		return refuse("the source patch bytes do not hash to the bundle's source_patch_sha256")
	}
	kind, name, err := ClassifyOperation(request.SourcePatch, request.Bundle)
	if err != nil {
		return refuse("%v", err)
	}
	if kind != ConsensusKindPackage {
		return refuse("the bundle's source patch is not a package-only change; it takes a platform decision")
	}
	claimed := strings.ToLower(strings.TrimSpace(request.ClaimedPlatformBase))
	if !gitCommitID.MatchString(claimed) {
		return refuse("the claimed platform base is not a commit id")
	}
	bundleBase := strings.ToLower(strings.TrimSpace(bundle.SourcePatchBaseCommit))
	if err := a.seat.packageBaseHeld(ctx, claimed); err != nil {
		if errors.Is(err, pkgrelease.ErrBaseUnavailable) && a.seat.packageBase == nil {
			return hostBuildVote{}, fmt.Errorf("%w: %v", ErrConsensusUnavailable, err)
		}
		return refuse("the claimed platform base %s is not in corpusd's store", claimed)
	}
	// The claim is bound to the booted base for every claim (design 2.2): an
	// empty stack (the claim is the bundle's own base) must equal the booted
	// base's build commit; with stack commits it is the stack's base, which
	// must be the booted base or descend from it.
	resolver, ok := a.hostBuilds.(bootedBaseCommits)
	if !ok {
		return hostBuildVote{}, fmt.Errorf("%w: the booted base cannot be resolved here", ErrConsensusUnavailable)
	}
	booted, found := resolver.BootedBaseCommit(strings.TrimSpace(request.BaseImageManifestDigest))
	if !found {
		return refuse("the computer's booted base %s is not an image this host holds", request.BaseImageManifestDigest)
	}
	if claimed != booted {
		if claimed == bundleBase {
			return refuse("the claimed platform base %s is not the booted base's build commit %s", claimed, booted)
		}
		ancestry, ok := a.seat.packageBase.(interface {
			IsAncestor(ctx context.Context, ancestor, descendant string) (bool, error)
		})
		if !ok {
			return hostBuildVote{}, fmt.Errorf("%w: the base object store cannot answer ancestry", ErrConsensusUnavailable)
		}
		if descends, err := ancestry.IsAncestor(ctx, booted, claimed); err != nil || !descends {
			return refuse("the claimed platform base %s does not descend from the booted base's build commit %s", claimed, booted)
		}
	}

	draft, err := a.seat.derivePackageDraft(ctx, packageDraftInput{
		OwnerID: request.OwnerID, ComputerID: computerID, OperationID: request.OperationID, BundleDigest: request.Subject.BundleDigest,
		Name: name, ClaimedBase: claimed, Patch: request.SourcePatch, VerifyHost: true,
	})
	var choice string
	vote := hostBuildVote{pkg: &packageSeatResult{name: name, sourcePatchSHA256: patchSHA, claimedBase: claimed, media: bundle.mediaDigests()}}
	evidence := []string{"package-seat:" + name, "source-patch:sha256:" + patchSHA}
	switch {
	case err == nil:
		vote.pkg.draftDigest, vote.pkg.facts = draft.Digest, draft.Facts
		verdict, verr := computerevent.CanonicalJSON(map[string]any{
			"draft_digest": draft.Digest, "computer_id": computerID, "operation_id": request.OperationID,
			"bundle_digest": request.Subject.BundleDigest, "claimed_platform_base": claimed, "source_patch_sha256": patchSHA,
		})
		if verr != nil {
			return hostBuildVote{}, verr
		}
		vote.pkg.verdictDigest = computerevent.DigestBytes(verdict)
		vote.resultDigest = vote.pkg.verdictDigest
		choice = decisionpolicy.VoteAccept
		evidence = append(evidence, "package-draft:sha256:"+draft.Digest, "package-seat-verdict:sha256:"+vote.pkg.verdictDigest)
	case pkgrelease.ReasonOf(err) != "" && pkgrelease.ReasonOf(err) != pkgrelease.ReasonParentUnverifiable:
		// The draft fails a host check: the seat votes reject with the
		// reason, as a failed host build does.
		choice = decisionpolicy.VoteReject
		vote.reason = err.Error()
		vote.pkg.draftDigest = computerevent.DigestBytes([]byte("rejected:" + request.Subject.BundleDigest + ":" + patchSHA))
		evidence = append(evidence, "package-seat-reason:"+pkgrelease.ReasonOf(err))
	default:
		return hostBuildVote{}, fmt.Errorf("%w: the package draft cannot be made now: %v", ErrConsensusUnavailable, err)
	}
	ballot, err := decisionpolicy.NewBallot(decisionpolicy.SeatCapsuleVerifier, choice, policyDigest, subjectDigest,
		computerID, request.OperationID, "", "", evidence, now, 0)
	if err != nil {
		return hostBuildVote{}, err
	}
	if vote.ballot, err = decisionpolicy.SignBallot(ballot, a.signingKey, decisionpolicy.IssuerHostBuild, now); err != nil {
		return hostBuildVote{}, err
	}
	return vote, nil
}

// packageBaseHeld requires the commit to be in corpusd's object store.
func (h *Handler) packageBaseHeld(ctx context.Context, commit string) error {
	if h == nil || h.packageBase == nil {
		return fmt.Errorf("%w: no base object store configured", pkgrelease.ErrBaseUnavailable)
	}
	_, _, err := h.packageBase.PackageTree(ctx, commit, packagepath.Root+"/_")
	return err
}

type packageDraftInput struct {
	OwnerID, ComputerID, OperationID, BundleDigest string
	Name, ClaimedBase                              string
	Patch                                          []byte
	// VerifyHost runs VerifyHostFacts on the draft, as a record the seat
	// builds; the publish route leaves it to the publish's own host checks.
	VerifyHost bool
}

type packageDraft struct {
	Source pkgrelease.DraftSource
	Facts  pkgrelease.DraftFacts
	Digest string
}

// trackHeadFor is the owner's own track head for a package: its release id
// and tree, or none.
func (h *Handler) trackHeadFor(ctx context.Context, ownerID, name string) (releaseID, tree string, err error) {
	listings, _, err := h.trackListings(ctx, ownerID, name)
	if err != nil {
		return "", "", err
	}
	head, found, _ := trackHead(listings)
	if !found || head.Track != ownerID {
		return "", "", nil
	}
	_, record, err := h.readPackageParent(ctx, ownerID, head.RecordID)
	if err != nil {
		return "", "", fmt.Errorf("the track head's record is unreadable: %w", err)
	}
	return head.RecordID, record.Source.Tree, nil
}

// derivePackageDraft drafts the package release of a frozen patch over the
// claimed platform base with the owner's track head, and digests its
// content. It is the one derivation the seat and the publish route share.
func (h *Handler) derivePackageDraft(ctx context.Context, in packageDraftInput) (packageDraft, error) {
	if h == nil || h.packageBase == nil || strings.TrimSpace(h.packageBaseDir) == "" {
		return packageDraft{}, &pkgrelease.Refusal{Reason: pkgrelease.ReasonParentUnverifiable, Detail: "this host holds no base object store"}
	}
	if !objectgraph.ValidPackageName(in.Name) {
		return packageDraft{}, &pkgrelease.Refusal{Reason: pkgrelease.ReasonInvalid, Detail: "package name " + in.Name}
	}
	headID, headTree, err := h.trackHeadFor(ctx, in.OwnerID, in.Name)
	if err != nil {
		return packageDraft{}, err
	}
	source, err := pkgrelease.DraftPackage(ctx, pkgrelease.DraftInput{
		Repo: h.packageBaseDir, Name: in.Name, Base: in.ClaimedBase, Patch: in.Patch, PlatformBase: in.ClaimedBase,
		HeadReleaseID: headID, HeadTree: headTree,
	})
	if err != nil {
		return packageDraft{}, err
	}
	facts := source.Facts(in.Name)
	digest, err := facts.Digest()
	if err != nil {
		return packageDraft{}, err
	}
	if in.VerifyHost {
		record := pkgrelease.Record{
			Schema: pkgrelease.Schema, Package: pkgrelease.Package{Name: in.Name, ManifestVersion: source.ManifestVersion}, Track: in.OwnerID,
			Parent: source.Parent,
			Source: pkgrelease.Source{
				Root: source.Root, Tree: source.Tree, TreePatch: blobRef(source.Blobs.TreePatch), ReviewPatch: blobRef(source.Blobs.ReviewPatch),
				ChangedPaths: source.Paths, BuiltOn: in.ClaimedBase,
			},
			Manifest:  source.Manifest,
			Publisher: pkgrelease.Publisher{OwnerID: in.OwnerID, ComputerID: in.ComputerID},
			// The decision is not minted yet: its event is the placeholder
			// the shape check needs.
			Operation: pkgrelease.Operation{OperationID: in.OperationID, BundleDigest: in.BundleDigest, DecisionEvent: "pending-package-decision"},
		}
		host := pkgrelease.HostInput{Record: record, TreePatch: source.Blobs.TreePatch, ReviewPatch: source.Blobs.ReviewPatch, Base: h.packageBase}
		if source.Blobs.ParentPatch != nil {
			parent := blobRef(source.Blobs.ParentPatch)
			record.ParentPatch = &parent
			host.Record, host.ParentPatch = record, source.Blobs.ParentPatch
		}
		if source.Parent.Kind == pkgrelease.ParentRelease {
			_, parent, perr := h.readPackageParent(ctx, in.OwnerID, source.Parent.ReleaseID)
			if perr != nil {
				return packageDraft{}, &pkgrelease.Refusal{Reason: pkgrelease.ReasonParentMismatch, Detail: "the parent release is unreadable: " + perr.Error()}
			}
			host.ParentRelease = &parent
			if host.ParentTreePatch, perr = h.readPackageBlob(parent.Source.TreePatch); perr != nil {
				return packageDraft{}, perr
			}
		}
		if _, err := pkgrelease.VerifyHostFacts(ctx, host); err != nil {
			return packageDraft{}, err
		}
	}
	return packageDraft{Source: source, Facts: facts, Digest: digest}, nil
}

func blobRef(raw []byte) pkgrelease.Blob {
	return pkgrelease.Blob{SHA256: computerevent.DigestBytes(raw), Bytes: len(raw)}
}

// --- publisher address (the proxy's lookup, as an internal call) ---

const publicAddressesPath = "/internal/maild/public-addresses"

type publisherAddressLookup func(ctx context.Context, ownerID string) (string, error)

// ConfigurePublisherAddresses names maild, the owner of the numbered Choir
// addresses, for the publish-by-decision route. It asks the same host-internal
// lookup the proxy asks (proxy/packages_publisher.go). Unconfigured, every
// publish by decision refuses publisher_address_unavailable.
func (h *Handler) ConfigurePublisherAddresses(maildURL string, client *http.Client) {
	if h == nil {
		return
	}
	maildURL = strings.TrimRight(strings.TrimSpace(maildURL), "/")
	if maildURL == "" {
		h.publisherAddress = nil
		return
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	h.publisherAddress = func(ctx context.Context, ownerID string) (string, error) {
		body, _ := json.Marshal(map[string]any{"owner_ids": []string{ownerID}})
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, maildURL+publicAddressesPath, bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		request.Header.Set("Content-Type", "application/json")
		hostauth.MarkInternal(request)
		response, err := client.Do(request)
		if err != nil {
			return "", err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return "", fmt.Errorf("maild answered status %d", response.StatusCode)
		}
		var answer struct {
			Addresses map[string]string `json:"addresses"`
		}
		if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&answer); err != nil {
			return "", err
		}
		return strings.TrimSpace(answer.Addresses[ownerID]), nil
	}
}

// --- publish by decision ---

// PackagePublishByDecisionRequest is the guest's request to publish the
// release of one admitted package decision. The owner and computer are the
// guest credential's binding (vmctl), never these fields: OwnerID and
// ComputerID are read only from a host-internal caller.
type PackagePublishByDecisionRequest struct {
	DecisionDigest string          `json:"decision_digest"`
	OperationID    string          `json:"operation_id"`
	BundleDigest   string          `json:"bundle_digest"`
	ReleaseID      string          `json:"release_id"`
	Record         json.RawMessage `json:"record"`
	// The draft's patches, the frozen source patch the draft is re-derived
	// from, and the base it is derived over.
	TreePatch           []byte `json:"tree_patch"`
	ReviewPatch         []byte `json:"review_patch"`
	ParentPatch         []byte `json:"parent_patch,omitempty"`
	SourcePatch         []byte `json:"source_patch"`
	ClaimedPlatformBase string `json:"claimed_platform_base"`
	// Media are the bytes of the record's screenshots and video.
	Media      [][]byte `json:"media,omitempty"`
	OwnerID    string   `json:"owner_id,omitempty"`
	ComputerID string   `json:"computer_id,omitempty"`
}

func (r PackagePublishByDecisionRequest) payloadBytes() int {
	n := len(r.Record) + len(r.TreePatch) + len(r.ReviewPatch) + len(r.ParentPatch) + len(r.SourcePatch)
	for _, m := range r.Media {
		n += len(m)
	}
	return n
}

// packageDecisionRow is the decision as the publish route reads it.
type packageDecisionRow struct {
	computerID, operationID, bundle, kind, standingDigest, admitted string
	draftDigest, patchSHA, mediaDigests, published                  string
	generation                                                      uint64
	notAfter                                                        time.Time
}

func (a *ConsensusActuator) readPackageDecision(ctx context.Context, digest string) (packageDecisionRow, bool, error) {
	var row packageDecisionRow
	err := a.store.db.QueryRowContext(ctx, `SELECT computer_id, operation_id, bundle_digest, kind, standing_mode_receipt_digest, COALESCE(admitted_event_digest, ''), host_build_job_id, COALESCE(package_source_patch_sha256, ''), COALESCE(package_media_digests, ''), COALESCE(published_release_id, ''), standing_generation, not_after FROM consensus_decisions WHERE decision_digest=?`, digest).
		Scan(&row.computerID, &row.operationID, &row.bundle, &row.kind, &row.standingDigest, &row.admitted, &row.draftDigest, &row.patchSHA, &row.mediaDigests, &row.published, &row.generation, &row.notAfter)
	if errors.Is(err, sql.ErrNoRows) {
		return packageDecisionRow{}, false, nil
	}
	return row, err == nil, err
}

// HandlePackagePublishByDecision is POST
// /internal/computers/self-development/package-publish: the guest of a
// computer publishes the release of its admitted package decision (2.4). The
// decision replaces the owner's publish receipt and confirm_paths: it binds
// the draft content digest corpusd derived itself, and the screens the
// capsule rendered and the judge reviewed. The caller is the guest
// credential: vmctl's binding of the tap names the owner and the computer.
func (h *Handler) HandlePackagePublishByDecision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if h == nil || h.consensus == nil || h.releaseStore() == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "package publish by decision unavailable"})
		return
	}
	// The credential binds before any body byte is read, and at most
	// maxConcurrentPackagePublishes run at once, so the decode is bounded in
	// bytes and in number.
	ownerID, computerID, internal, ok := h.packagePublishCaller(w, r)
	if !ok {
		return
	}
	select {
	case packagePublishSlots <- struct{}{}:
		defer func() { <-packagePublishSlots }()
	default:
		w.Header().Set("Retry-After", "5")
		writeJSON(w, http.StatusTooManyRequests, packageAPIError{Error: "too many package publishes are running; retry shortly", Reason: reasonPublishBusy})
		return
	}
	var request PackagePublishByDecisionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxPackagePublishBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeJSON(w, http.StatusRequestEntityTooLarge, packageAPIError{Error: "the publish body exceeds the cap", Reason: pkgrelease.ReasonBlobTooLarge})
			return
		}
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid package publish request"})
		return
	}
	if request.payloadBytes() > maxPackagePublishPayload {
		writeJSON(w, http.StatusRequestEntityTooLarge, packageAPIError{Error: "the publish payload exceeds 40 MiB", Reason: pkgrelease.ReasonBlobTooLarge})
		return
	}
	if internal {
		ownerID, computerID = strings.TrimSpace(request.OwnerID), strings.TrimSpace(request.ComputerID)
		if ownerID == "" || computerID == "" {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "owner_id and computer_id are required of an internal caller"})
			return
		}
	} else if (strings.TrimSpace(request.OwnerID) != "" && strings.TrimSpace(request.OwnerID) != ownerID) ||
		(strings.TrimSpace(request.ComputerID) != "" && strings.TrimSpace(request.ComputerID) != computerID) {
		writeJSON(w, http.StatusForbidden, apiError{Error: errInternalCallerRequired.Error()})
		return
	}
	h.publishByDecision(w, r, ownerID, computerID, request)
}

// packagePublishCaller names the credential's owner and computer before the
// body is read. A host-internal caller states them in the body (internal is
// true and the names are empty here); a guest is bound by vmctl to exactly
// one of each, must hold the event:append capability the consensus route
// requires, and the request's own claim, read later, must agree.
func (h *Handler) packagePublishCaller(w http.ResponseWriter, r *http.Request) (ownerID, computerID string, internal, ok bool) {
	if trustedInternalCaller(r) {
		return "", "", true, true
	}
	ownership, err := bindTapCaller(r.Context(), h.guestLookup, r)
	if err != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: errInternalCallerRequired.Error()})
		return "", "", false, false
	}
	ownerID, computerID = strings.TrimSpace(ownership.UserID), strings.TrimSpace(ownership.ComputerID)
	if ownerID == "" || computerID == "" ||
		(strings.TrimSpace(r.URL.Query().Get("computer_id")) != "" && strings.TrimSpace(r.URL.Query().Get("computer_id")) != computerID) {
		writeJSON(w, http.StatusForbidden, apiError{Error: errInternalCallerRequired.Error()})
		return "", "", false, false
	}
	if h.eventAuth == nil || h.eventAuth.Authorize(r, computerID, "event:append") != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: "package publish capability refused"})
		return "", "", false, false
	}
	return ownerID, computerID, false, true
}

func (h *Handler) publishByDecision(w http.ResponseWriter, r *http.Request, ownerID, computerID string, request PackagePublishByDecisionRequest) {
	ctx := r.Context()
	refuse := func(status int, reason, format string, args ...any) {
		writeJSON(w, status, packageAPIError{Error: fmt.Sprintf(format, args...), Reason: reason})
	}
	digest := strings.ToLower(strings.TrimSpace(request.DecisionDigest))
	if !computerevent.IsSHA256(digest) {
		refuse(http.StatusBadRequest, reasonDecisionMismatch, "decision_digest must be a sha256")
		return
	}
	row, found, err := h.consensus.readPackageDecision(ctx, digest)
	switch {
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "package publish: decision unreadable"})
		return
	case !found || row.computerID != computerID:
		refuse(http.StatusNotFound, reasonDecisionMismatch, "no such decision for this computer")
		return
	case row.kind != ConsensusKindPackage:
		refuse(http.StatusForbidden, reasonPackageNotDecided, "the decision is not a package decision")
		return
	case row.admitted == "":
		refuse(http.StatusForbidden, reasonDecisionMismatch, "the decision was never admitted on the computer's tape")
		return
	case row.operationID != strings.TrimSpace(request.OperationID) || row.bundle != strings.ToLower(strings.TrimSpace(request.BundleDigest)):
		refuse(http.StatusForbidden, reasonDecisionMismatch, "the decision is for another operation or bundle")
		return
	}
	if platform, perr := platformComputer(computerID); platform || perr != nil {
		refuse(http.StatusForbidden, reasonPublishAuthority, "a platform computer does not self-develop; it publishes no package by decision")
		return
	}
	authority, err := h.consensus.currentApplyAuthority(ctx, computerID)
	if err != nil || authority.generation != row.generation || authority.receiptDigest != row.standingDigest {
		refuse(http.StatusForbidden, reasonPublishAuthority, "the computer's self-development mode no longer authorizes the mode the decision was made under")
		return
	}
	if time.Now().UTC().After(row.notAfter.UTC().Add(packagePublishGrace)) {
		refuse(http.StatusGone, reasonDecisionExpired, "the decision expired before the publish")
		return
	}
	sum := sha256.Sum256(request.SourcePatch)
	if row.patchSHA == "" || hex.EncodeToString(sum[:]) != row.patchSHA {
		refuse(http.StatusForbidden, reasonDecisionMismatch, "the source patch is not the one the decision was made on")
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
	if row.published != "" && row.published != releaseID {
		refuse(http.StatusConflict, reasonDecisionConsumed, "the decision already published release %s; it publishes one release", row.published)
		return
	}
	if record.Publisher != (pkgrelease.Publisher{OwnerID: ownerID, ComputerID: computerID}) || record.Track != ownerID {
		refuse(http.StatusForbidden, reasonPublishAuthority, "the record's publisher or track is not the owner and computer bound to this credential")
		return
	}
	if record.Operation.OperationID != row.operationID || record.Operation.BundleDigest != row.bundle || record.Operation.DecisionEvent != row.admitted {
		refuse(http.StatusForbidden, reasonDecisionMismatch, "the record is of another operation, bundle or decision event than the decision")
		return
	}

	// The publisher must have a numbered Choir address, as for an owner's
	// publish through the proxy.
	if h.publisherAddress == nil {
		refuse(http.StatusBadGateway, reasonPublisherLookup, "the address service is not configured, so this release cannot be published")
		return
	}
	address, aerr := h.publisherAddress(ctx, ownerID)
	switch {
	case aerr != nil:
		log.Printf("corpusd: publisher address lookup for %s: %v", ownerID, aerr)
		refuse(http.StatusBadGateway, reasonPublisherLookup, "the address service did not answer, so this release cannot be published yet")
		return
	case address == "":
		refuse(http.StatusUnprocessableEntity, reasonPublisherAddress, "this account has no Choir address, and a published app shows its maker by that address")
		return
	}

	// The draft, re-derived from the frozen patch over the claimed base, is
	// the draft the seat voted on and the record the guest posts.
	claimed := strings.ToLower(strings.TrimSpace(request.ClaimedPlatformBase))
	if !gitCommitID.MatchString(claimed) {
		refuse(http.StatusForbidden, reasonDraftDigest, "the claimed platform base is not a commit id")
		return
	}
	recordDigest, err := record.DraftFacts().Digest()
	if err != nil {
		writePackageRefusal(w, err)
		return
	}
	if recordDigest != row.draftDigest {
		refuse(http.StatusForbidden, reasonDraftDigest, "the posted record is not the draft the decision binds (decision %s, record %s)", row.draftDigest, recordDigest)
		return
	}
	// The first publish re-derives the draft from the frozen patch over the
	// claimed base, on the owner's track head now, and requires it to be the
	// draft the seat voted on. The repeat of a publish that already took the
	// decision (same release) skips it: the track head is that release by now.
	if row.published != releaseID {
		if err := h.packageBaseHeld(ctx, claimed); err != nil {
			refuse(http.StatusUnprocessableEntity, reasonBaseUnavailable, "the claimed platform base %s is not in corpusd's store", claimed)
			return
		}
		draft, err := h.derivePackageDraft(ctx, packageDraftInput{
			OwnerID: ownerID, ComputerID: computerID, OperationID: row.operationID, BundleDigest: row.bundle,
			Name: record.Package.Name, ClaimedBase: claimed, Patch: request.SourcePatch,
		})
		if err != nil {
			if pkgrelease.ReasonOf(err) != "" {
				writePackageRefusal(w, err)
				return
			}
			log.Printf("corpusd: package publish by decision %s: draft: %v", digest, err)
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "package publish: the draft cannot be derived"})
			return
		}
		if draft.Digest != row.draftDigest || draft.Digest != recordDigest ||
			record.Source.TreePatch.SHA256 != computerevent.DigestBytes(draft.Source.Blobs.TreePatch) ||
			record.Source.ReviewPatch.SHA256 != computerevent.DigestBytes(draft.Source.Blobs.ReviewPatch) {
			refuse(http.StatusForbidden, reasonDraftDigest, "the posted record is not the draft the decision binds (derived %s, decision %s, record %s)", draft.Digest, row.draftDigest, recordDigest)
			return
		}
	}

	// Media: only items the decision's frozen set names, each with its bytes.
	var frozen []string
	_ = json.Unmarshal([]byte(row.mediaDigests), &frozen)
	frozenSet := map[string]bool{}
	for _, d := range frozen {
		frozenSet[d] = true
	}
	posted := map[string][]byte{}
	for _, raw := range request.Media {
		posted[computerevent.DigestBytes(raw)] = raw
	}
	named := map[string]bool{}
	for _, item := range record.Media {
		if !frozenSet[item.SHA256] {
			refuse(http.StatusForbidden, reasonMediaNotFrozen, "media %s (%s) is not among the screens and video the decision froze", item.Name, item.SHA256)
			return
		}
		raw, ok := posted[item.SHA256]
		if !ok {
			refuse(http.StatusUnprocessableEntity, pkgrelease.ReasonBlobMismatch, "media %s (%s) was not posted", item.Name, item.SHA256)
			return
		}
		if err := pkgrelease.VerifyMedia(item, raw); err != nil {
			writePackageRefusal(w, err)
			return
		}
		named[item.SHA256] = true
	}
	for d := range posted {
		if !named[d] {
			refuse(http.StatusForbidden, reasonMediaNotFrozen, "a posted media item (%s) is not named by the record", d)
			return
		}
	}

	// Everything the publish reads is stored from bytes checked against the
	// record, then the one publish path runs every host check on them.
	if err := h.storePackageBytes(record, request, posted); err != nil {
		writePackageRefusal(w, err)
		return
	}
	db := h.consensus.store.db
	h.publishVerifiedRecord(w, r, publishParams{
		record: record, releaseID: releaseID, ownerID: ownerID, computerID: computerID,
		actor: "package_decision:" + digest, byDecision: true,
		claim: func(ctx context.Context) error {
			// The decision is consumed once: the release id is set if none is,
			// and only the same release may hold it.
			if _, err := db.ExecContext(ctx, `UPDATE consensus_decisions SET published_release_id=? WHERE decision_digest=? AND (published_release_id IS NULL OR published_release_id='')`, releaseID, digest); err != nil {
				return err
			}
			var holder string
			if err := db.QueryRowContext(ctx, `SELECT COALESCE(published_release_id, '') FROM consensus_decisions WHERE decision_digest=?`, digest).Scan(&holder); err != nil {
				return err
			}
			if strings.TrimSpace(holder) != releaseID {
				return &pkgrelease.Refusal{Reason: reasonDecisionConsumed, Detail: "the decision already published release " + strings.TrimSpace(holder)}
			}
			return nil
		},
	})
}

// storePackageBytes puts the record's patches and media in the host content
// store, each verified against the blob or item the record names.
func (h *Handler) storePackageBytes(record pkgrelease.Record, request PackagePublishByDecisionRequest, media map[string][]byte) error {
	patches := []struct {
		blob pkgrelease.Blob
		raw  []byte
	}{{record.Source.TreePatch, request.TreePatch}, {record.Source.ReviewPatch, request.ReviewPatch}}
	if record.ParentPatch != nil {
		patches = append(patches, struct {
			blob pkgrelease.Blob
			raw  []byte
		}{*record.ParentPatch, request.ParentPatch})
	} else if len(request.ParentPatch) != 0 {
		return &pkgrelease.Refusal{Reason: pkgrelease.ReasonInvalid, Detail: "parent_patch bytes for a record that names none"}
	}
	for _, p := range patches {
		if err := pkgrelease.VerifyBlob(p.blob, p.raw); err != nil {
			return err
		}
		store := h.contentStore(pkgrelease.ContentNamespace)
		store.maxBytes = pkgrelease.MaxBlobBytes
		if err := store.Put(p.blob.SHA256, bytes.NewReader(p.raw)); err != nil {
			return fmt.Errorf("package blob store: %w", err)
		}
	}
	for digest, raw := range media {
		store := h.contentStore(pkgrelease.MediaNamespace)
		store.maxBytes = pkgrelease.MaxMediaBytes
		if err := store.Put(digest, bytes.NewReader(raw)); err != nil {
			return fmt.Errorf("package media store: %w", err)
		}
	}
	return nil
}
