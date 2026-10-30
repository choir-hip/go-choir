package platform

// Per-version permalinks (Cut 2 unit 2b,
// docs/archive/definitions/choir-reeng-cut2-evidence-to-publication-2026-10-12.md).
//
// A story is one publication per (owner, source doc). Each committed
// revision of the doc publishes as version n, n its Texture revision number
// (types.RevisionNumber), at the permanent link <bare>/v<n>; the bare link
// serves the latest version in place. Publishing is idempotent on the
// revision. A revision that fails to publish leaves a gap that nav skips;
// nothing is renumbered or back-filled.
//
// A version is an immutable payload (text, manifest and its hash,
// signatures, resolved_blocks) with two live fields served beside it: nav
// and the check overlay. Only the payload is fixed forever. Each manifest
// names the previous version's manifest hash instead of embedding every past
// body, so the versions form a hash chain.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// publicationManifestSchema names the per-version manifest. v1 chains by
// previous_manifest_hash and carries the version's anchors and the hash of
// its resolved_blocks; it never embeds past versions.
const publicationManifestSchema = "choir.platform.publication_version_manifest.v1"

// resolvedBlocksSchema names a version's pinned capture blocks.
const resolvedBlocksSchema = "choir.platform.resolved_blocks.v1"

// Relation types of a version's anchor rows.
const (
	anchorRelationCites  = "cites"
	anchorRelationEmbeds = "embeds"
)

// PublicationVersionPayload is a version's immutable payload. Its bytes are
// fixed once the version is published: later versions, nav and the check
// overlay never change them.
type PublicationVersionPayload struct {
	Revision       int                    `json:"revision"`
	Text           string                 `json:"text"`
	Manifest       json.RawMessage        `json:"manifest"`
	ManifestHash   string                 `json:"manifest_hash"`
	Signatures     []PublicationSignature `json:"signatures"`
	ResolvedBlocks []ResolvedBlock        `json:"resolved_blocks"`
}

// PublicationSignature is the platform's signature over a version's
// manifest hash (manifestAttestationSchema).
type PublicationSignature struct {
	Schema    string `json:"schema"`
	KeyID     string `json:"key_id"`
	PublicKey string `json:"public_key"`
	Signature string `json:"signature"`
}

// ResolvedBlock is one capture block a version cites or embeds, pinned into
// the version at publish: its text is the version's copy, not a pointer to
// the live capture.
type ResolvedBlock struct {
	Doc     string `json:"doc"`
	Version string `json:"version"`
	CID     string `json:"cid"`
	Hash    string `json:"hash"`
	Kind    string `json:"kind,omitempty"`
	Text    string `json:"text"`
	URL     string `json:"url,omitempty"`
	Title   string `json:"title,omitempty"`
}

// PublicationAnchor is one Write citation or embed in a version's body: the
// source_ref node it sits on, the Ref it carries and the capture blocks it
// resolved to.
type PublicationAnchor struct {
	AnchorID       string    `json:"anchor_id"`
	SourceEntityID string    `json:"source_entity_id"`
	Relation       string    `json:"relation"`
	Ref            types.Ref `json:"ref"`
	CIDs           []string  `json:"cids"`
}

// PublicationNav is a version's live navigation: its own number, the
// published versions before and after it (0 when none) and the latest. Gaps
// left by failed publishes are skipped.
type PublicationNav struct {
	Revision int `json:"revision"`
	Prev     int `json:"prev"`
	Next     int `json:"next"`
	Latest   int `json:"latest"`
}

// Quote-check states a check overlay reports per citation (✓ exact,
// ≈ tolerant, pending, unread source, ✗ failed). Unread means the cited
// source was never captured, so no check can run.
const (
	CheckExact    = "exact"
	CheckTolerant = "tolerant"
	CheckPending  = "pending"
	CheckUnread   = "unread"
	CheckFailed   = "failed"
)

// PublicationCheckOverlay is the live quote-check and judgement overlay for
// a version, served beside its payload and never inside its manifest.
type PublicationCheckOverlay struct {
	// Source names the provider that produced the overlay; "pending" when
	// no provider has results.
	Source    string          `json:"source"`
	Citations []CitationCheck `json:"citations"`
}

// CitationCheck is one citation's live check result.
type CitationCheck struct {
	AnchorID string    `json:"anchor_id"`
	Ref      types.Ref `json:"ref"`
	// State is the badge: exact or tolerant once the quote matched and the
	// support verdict passed, failed when either failed, else pending.
	State     string `json:"state,omitempty"`
	Quote     string `json:"quote"`
	Judgement string `json:"judgement,omitempty"`
	Detail    string `json:"detail,omitempty"`
	CheckedAt string `json:"checked_at,omitempty"`
}

// CheckOverlayVersion keys an overlay by the published version.
type CheckOverlayVersion struct {
	// OwnerID is the publishing owner: a version's checks are stored under
	// it, keyed by version and anchor.
	OwnerID              string
	PublicationID        string
	PublicationVersionID string
	Revision             int
	ManifestHash         string
}

// CheckOverlayCitation keys an overlay entry by citation: the anchor and
// the Ref it cites (Ref.Hash with the quote memoizes a check for an
// unchanged block and quote across versions).
type CheckOverlayCitation struct {
	AnchorID string
	Relation string
	Ref      types.Ref
}

// CheckOverlayProvider is the seam for quote-check and judgement results.
// It is read on every bundle read and its results are served live beside
// the payload, so they may change after publish (a judge finishing, a
// re-check) without touching the version. A provider error degrades the
// overlay to pending; it never fails the read.
type CheckOverlayProvider interface {
	CheckOverlay(ctx context.Context, version CheckOverlayVersion, citations []CheckOverlayCitation) (PublicationCheckOverlay, error)
}

// pendingCheckOverlay reports every citation pending: the overlay before any
// checker is wired.
type pendingCheckOverlay struct{}

func (pendingCheckOverlay) CheckOverlay(_ context.Context, _ CheckOverlayVersion, citations []CheckOverlayCitation) (PublicationCheckOverlay, error) {
	return pendingOverlay(citations), nil
}

func pendingOverlay(citations []CheckOverlayCitation) PublicationCheckOverlay {
	out := PublicationCheckOverlay{Source: CheckPending, Citations: make([]CitationCheck, 0, len(citations))}
	for _, c := range citations {
		out.Citations = append(out.Citations, CitationCheck{AnchorID: c.AnchorID, Ref: c.Ref, State: CheckPending, Quote: CheckPending, Judgement: CheckPending})
	}
	return out
}

// SetCheckOverlayProvider installs the overlay provider; nil restores the
// pending overlay.
func (s *Service) SetCheckOverlayProvider(p CheckOverlayProvider) {
	if s == nil {
		return
	}
	s.overlayMu.Lock()
	defer s.overlayMu.Unlock()
	s.overlay = p
}

func (s *Service) checkOverlay(ctx context.Context, version CheckOverlayVersion, anchors []PublicationAnchor) PublicationCheckOverlay {
	citations := make([]CheckOverlayCitation, 0, len(anchors))
	for _, a := range anchors {
		citations = append(citations, CheckOverlayCitation{AnchorID: a.AnchorID, Relation: a.Relation, Ref: a.Ref})
	}
	s.overlayMu.RLock()
	provider := s.overlay
	s.overlayMu.RUnlock()
	if provider == nil {
		provider = pendingCheckOverlay{}
	}
	out, err := provider.CheckOverlay(ctx, version, citations)
	if err != nil {
		pending := pendingOverlay(citations)
		pending.Source = "unavailable"
		return pending
	}
	if out.Citations == nil {
		out.Citations = []CitationCheck{}
	}
	return out
}

// publicationManifest is a version's manifest. It is marshaled once at
// publish and stored; its hash is the sha256 of those bytes. Everything a
// reader needs to verify the version is here or hashed here, and nothing
// live (nav, check results) is.
type publicationManifest struct {
	Schema                   string                         `json:"schema"`
	SubjectKind              string                         `json:"subject_kind"`
	PublicationID            string                         `json:"publication_id"`
	PublicationVersionID     string                         `json:"publication_version_id"`
	Revision                 int                            `json:"revision"`
	RoutePath                string                         `json:"route_path"`
	PermalinkPath            string                         `json:"permalink_path"`
	Title                    string                         `json:"title"`
	MediaType                string                         `json:"media_type"`
	ContentHash              string                         `json:"content_hash"`
	ProjectionHash           string                         `json:"projection_hash"`
	StorageRef               string                         `json:"storage_ref"`
	ByteSize                 int                            `json:"byte_size"`
	SourceRevisionHash       string                         `json:"source_revision_hash"`
	RevisionHash             string                         `json:"revision_hash,omitempty"`
	PreviousManifestHash     string                         `json:"previous_manifest_hash"`
	PreviousRevision         int                            `json:"previous_revision"`
	SourceMetadataHash       string                         `json:"source_metadata_hash"`
	BodyDoc                  json.RawMessage                `json:"body_doc"`
	StructuredSourceEntities json.RawMessage                `json:"structured_source_entities"`
	SourceEntities           []publicationSourceEntityInput `json:"source_entities"`
	Transclusions            []publicationTransclusionInput `json:"transclusions"`
	Anchors                  []PublicationAnchor            `json:"anchors"`
	ResolvedBlocksHash       string                         `json:"resolved_blocks_hash"`
	AccessPolicy             json.RawMessage                `json:"access_policy"`
	ExportPolicy             json.RawMessage                `json:"export_policy"`
}

// resolvedBlocksDoc is the stored body of a version's resolved blocks.
type resolvedBlocksDoc struct {
	Schema string          `json:"schema"`
	Blocks []ResolvedBlock `json:"blocks"`
}

func marshalResolvedBlocks(blocks []ResolvedBlock) ([]byte, string, error) {
	if blocks == nil {
		blocks = []ResolvedBlock{}
	}
	raw, err := json.Marshal(resolvedBlocksDoc{Schema: resolvedBlocksSchema, Blocks: blocks})
	if err != nil {
		return nil, "", err
	}
	return raw, sha256Hex(raw), nil
}

// publicationIdentity is the stable identity of the one publication of
// (owner, source doc).
type publicationIdentity struct {
	PublicationID string
	CanonicalID   string
}

func publicationIdentityFor(ownerID, sourceDocID string) (publicationIdentity, error) {
	publicationID := "pub-" + sha256Hex([]byte("texture-publication\x00" + ownerID + "\x00" + sourceDocID))[:32]
	canonicalID, err := objectgraph.BuildCanonicalID("choir.publication", ownerID, objectgraph.StableSuffixFromKey(publicationID))
	if err != nil {
		return publicationIdentity{}, err
	}
	return publicationIdentity{PublicationID: publicationID, CanonicalID: canonicalID}, nil
}

// publicationVersionID is version n's id within its publication.
func publicationVersionID(publicationID string, revision int) string {
	return publicationID + "-v" + strconv.Itoa(revision)
}

func publicationVersionCanonicalID(ownerID, versionID string) (string, error) {
	return objectgraph.BuildCanonicalID("choir.publication_version", ownerID, objectgraph.StableSuffixFromKey(versionID))
}

// versionPermalink is version n's permanent link under a bare route.
func versionPermalink(bare string, revision int) string {
	return strings.TrimRight(bare, "/") + "/v" + strconv.Itoa(revision)
}

// parsePublicationRoute splits a public route into its bare link and the
// version it names: /pub/texture/<slug> names the latest version (0) and
// /pub/texture/<slug>/v<n> names version n, n a positive decimal without a
// leading zero. Anything else under the slug (v0, v01, /latest) is not a
// route. Slugs never contain "/", so a /v<n> segment is never part of one.
func parsePublicationRoute(routePath string) (string, int, bool) {
	path := normalizePublicationRoutePath(routePath)
	if !strings.HasPrefix(path, publicTexturePrefix) {
		return path, 0, true
	}
	slug, version, hasVersion := strings.Cut(strings.TrimPrefix(path, publicTexturePrefix), "/")
	if !hasVersion {
		return path, 0, true
	}
	if slug == "" || len(version) < 2 || version[0] != 'v' || version[1] == '0' {
		return "", 0, false
	}
	n, err := strconv.Atoi(version[1:])
	if err != nil || n <= 0 || strconv.Itoa(n) != version[1:] {
		return "", 0, false
	}
	return publicTexturePrefix + slug, n, true
}

// publishedVersion is one published version of a publication, read from
// its has_version edge.
type publishedVersion struct {
	Revision    int
	CanonicalID string
}

// publishedVersions lists a publication's versions by revision, from the
// has_version edges written with each version. The edges are the one
// authority for which versions exist; nav and the latest version derive
// from them.
func publishedVersions(ctx context.Context, og *ObjectGraphStore, publicationCanonicalID string) ([]publishedVersion, error) {
	edges, err := og.ListEdgesByKind(ctx, publicationCanonicalID, "has_version")
	if err != nil {
		return nil, err
	}
	out := make([]publishedVersion, 0, len(edges))
	seen := map[int]bool{}
	for _, edge := range edges {
		var meta struct {
			Revision int `json:"revision"`
		}
		if err := json.Unmarshal(edge.Metadata, &meta); err != nil || meta.Revision <= 0 || seen[meta.Revision] {
			continue
		}
		seen[meta.Revision] = true
		out = append(out, publishedVersion{Revision: meta.Revision, CanonicalID: edge.ToID})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Revision < out[j].Revision })
	return out, nil
}

func findPublishedVersion(versions []publishedVersion, revision int) (publishedVersion, bool) {
	for _, v := range versions {
		if v.Revision == revision {
			return v, true
		}
	}
	return publishedVersion{}, false
}

// navFor is version n's nav among the published versions.
func navFor(versions []publishedVersion, revision int) PublicationNav {
	nav := PublicationNav{Revision: revision}
	for _, v := range versions {
		if v.Revision < revision {
			nav.Prev = v.Revision
		}
		if v.Revision > revision && nav.Next == 0 {
			nav.Next = v.Revision
		}
		if v.Revision > nav.Latest {
			nav.Latest = v.Revision
		}
	}
	return nav
}

// versionMetadata is the stored metadata of a choir.publication_version.
type versionMetadata struct {
	EditionLabel         string                 `json:"edition_label"`
	Revision             int                    `json:"revision"`
	PublicationID        string                 `json:"publication_id"`
	PublicationVersionID string                 `json:"publication_version_id"`
	SourceDocID          string                 `json:"source_doc_id"`
	SourceRevisionID     string                 `json:"source_revision_id"`
	SourceRevisionHash   string                 `json:"source_revision_hash"`
	RevisionHash         string                 `json:"revision_hash,omitempty"`
	ProjectionHash       string                 `json:"projection_hash"`
	ArtifactManifestID   string                 `json:"artifact_manifest_id"`
	ManifestHash         string                 `json:"manifest_hash"`
	PreviousManifestHash string                 `json:"previous_manifest_hash"`
	SupersedesVersionID  string                 `json:"supersedes_version_id"`
	RoutePath            string                 `json:"route_path"`
	PermalinkPath        string                 `json:"permalink_path"`
	PublishedAt          string                 `json:"published_at"`
	Signatures           []PublicationSignature `json:"signatures"`
}

func decodeVersionMetadata(obj objectgraph.Object) (versionMetadata, error) {
	var meta versionMetadata
	if err := json.Unmarshal(obj.Metadata, &meta); err != nil {
		return meta, fmt.Errorf("decode publication version %s: %w", obj.CanonicalID, err)
	}
	return meta, nil
}

// publicationAnchors lists the Write citations and embeds in a body_doc, in
// document order: every source_ref node that carries a Ref.
func publicationAnchors(bodyDoc json.RawMessage) ([]PublicationAnchor, error) {
	if len(strings.TrimSpace(string(bodyDoc))) == 0 {
		return nil, nil
	}
	var doc texturedoc.StructuredTextureDoc
	if err := json.Unmarshal(bodyDoc, &doc); err != nil {
		return nil, fmt.Errorf("body_doc must be valid StructuredTextureDoc JSON: %w", err)
	}
	var out []PublicationAnchor
	var walk func(texturedoc.Node)
	walk = func(n texturedoc.Node) {
		if ref, ok := texturewrite.RefOf(n); ok {
			relation := anchorRelationCites
			if mode, _ := n.Attrs["display_mode"].(string); mode == "expanded_ref" {
				relation = anchorRelationEmbeds
			}
			id, _ := n.Attrs["id"].(string)
			entity, _ := n.Attrs["source_entity_id"].(string)
			out = append(out, PublicationAnchor{AnchorID: strings.TrimSpace(id), SourceEntityID: strings.TrimSpace(entity), Relation: relation, Ref: ref})
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(doc.Doc)
	for i, a := range out {
		if a.AnchorID == "" {
			return nil, fmt.Errorf("citation %d of %s has no node id", i+1, a.Ref.Doc)
		}
	}
	return out, nil
}

// errCaptureUnavailable marks a cited capture publish cannot read.
var errCaptureUnavailable = errors.New("cited capture unavailable")

// captureBlockDoc reads a capture's block document from Store B and the
// capture body store.
func (s *Service) captureBlockDoc(ctx context.Context, captureID string) (sourcegraph.CaptureBlocks, objectgraph.WebCaptureMetadata, error) {
	var doc sourcegraph.CaptureBlocks
	kind, _, _, err := objectgraph.ParseCanonicalID(captureID)
	if err != nil || kind != objectgraph.WebCaptureObjectKind {
		return doc, objectgraph.WebCaptureMetadata{}, fmt.Errorf("%w: %s is not a capture id", errCaptureUnavailable, captureID)
	}
	obj, err := s.ogStore().GetObject(ctx, captureID)
	if err != nil {
		return doc, objectgraph.WebCaptureMetadata{}, fmt.Errorf("%w: %s: %v", errCaptureUnavailable, captureID, err)
	}
	meta, err := objectgraph.WebCaptureMetadataFromObject(obj)
	if err != nil {
		return doc, meta, fmt.Errorf("%w: %s: %v", errCaptureUnavailable, captureID, err)
	}
	if meta.BlocksBlobID == "" {
		return doc, meta, fmt.Errorf("%w: %s has no blocks", errCaptureUnavailable, captureID)
	}
	raw, err := s.captureBodies.Body(strings.TrimPrefix(meta.BlocksBlobID, "sha256:"))
	if err != nil {
		return doc, meta, fmt.Errorf("%w: %s blocks: %v", errCaptureUnavailable, captureID, err)
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return doc, meta, fmt.Errorf("%w: %s blocks: %v", errCaptureUnavailable, captureID, err)
	}
	return doc, meta, nil
}

// resolveAnchors pins every anchor's capture blocks: a cite or a quoted
// embed pins the one block its Ref names, an embed without a block pins the
// whole capture. A Ref whose capture version or block hash no longer
// matches what was cited is refused, so a version never pins text its
// author did not cite. Blocks are deduplicated in first-anchor order.
func (s *Service) resolveAnchors(ctx context.Context, anchors []PublicationAnchor) ([]PublicationAnchor, []ResolvedBlock, error) {
	type captureDoc struct {
		blocks sourcegraph.CaptureBlocks
		meta   objectgraph.WebCaptureMetadata
	}
	captures := map[string]captureDoc{}
	var resolved []ResolvedBlock
	seen := map[string]bool{}
	pin := func(doc captureDoc, captureID string, cid, text, kind string) ResolvedBlock {
		rb := ResolvedBlock{Doc: captureID, Version: doc.meta.DocHash, CID: cid, Hash: blockTextHash(text), Kind: kind, Text: text, URL: doc.meta.URL, Title: doc.meta.Title}
		key := captureID + "\x00" + cid
		if !seen[key] {
			seen[key] = true
			resolved = append(resolved, rb)
		}
		return rb
	}
	out := make([]PublicationAnchor, 0, len(anchors))
	for _, a := range anchors {
		doc, ok := captures[a.Ref.Doc]
		if !ok {
			blocks, meta, err := s.captureBlockDoc(ctx, a.Ref.Doc)
			if err != nil {
				return nil, nil, fmt.Errorf("citation %s: %w", a.AnchorID, err)
			}
			doc = captureDoc{blocks: blocks, meta: meta}
			captures[a.Ref.Doc] = doc
		}
		if a.Ref.Version != "" && a.Ref.Version != doc.meta.DocHash {
			return nil, nil, fmt.Errorf("citation %s cites version %s of %s, which is not the capture's version %s", a.AnchorID, a.Ref.Version, a.Ref.Doc, doc.meta.DocHash)
		}
		a.CIDs = []string{}
		if a.Ref.CID == "" {
			if a.Relation != anchorRelationEmbeds {
				return nil, nil, fmt.Errorf("citation %s of %s names no block", a.AnchorID, a.Ref.Doc)
			}
			for _, b := range doc.blocks.Blocks {
				pin(doc, a.Ref.Doc, b.ID, b.Text, b.Kind)
				a.CIDs = append(a.CIDs, b.ID)
			}
			out = append(out, a)
			continue
		}
		found := false
		for _, b := range doc.blocks.Blocks {
			if b.ID != a.Ref.CID {
				continue
			}
			found = true
			if a.Ref.Hash != "" && a.Ref.Hash != blockTextHash(b.Text) {
				return nil, nil, fmt.Errorf("citation %s cites block %s of %s with hash %s, but the block hashes to %s", a.AnchorID, b.ID, a.Ref.Doc, a.Ref.Hash, blockTextHash(b.Text))
			}
			pin(doc, a.Ref.Doc, b.ID, b.Text, b.Kind)
			a.CIDs = append(a.CIDs, b.ID)
			break
		}
		if !found {
			return nil, nil, fmt.Errorf("citation %s cites block %s, which %s does not have", a.AnchorID, a.Ref.CID, a.Ref.Doc)
		}
		out = append(out, a)
	}
	if resolved == nil {
		resolved = []ResolvedBlock{}
	}
	return out, resolved, nil
}

// blockTextHash is a block's content hash as a Ref carries it.
func blockTextHash(text string) string {
	return "sha256:" + sha256Hex([]byte(text))
}

// anchorText is the pinned text an anchor's row snapshots.
func anchorText(a PublicationAnchor, blocks []ResolvedBlock) string {
	byKey := make(map[string]string, len(blocks))
	for _, b := range blocks {
		byKey[b.Doc+"\x00"+b.CID] = b.Text
	}
	parts := make([]string, 0, len(a.CIDs))
	for _, cid := range a.CIDs {
		parts = append(parts, byKey[a.Ref.Doc+"\x00"+cid])
	}
	return strings.Join(parts, "\n\n")
}

// manifestAnchors reads the anchors a stored manifest names.
func manifestAnchors(manifest []byte) []PublicationAnchor {
	var envelope struct {
		Anchors []PublicationAnchor `json:"anchors"`
	}
	if err := json.Unmarshal(manifest, &envelope); err != nil {
		return nil
	}
	return envelope.Anchors
}
