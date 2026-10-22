package platform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

const (
	publicTexturePrefix = "/pub/texture/"
	textMediaType       = "text/plain; charset=utf-8"

	defaultPublishedTextureTitle    = "Published Texture"
	defaultUntitledTextureTitle     = "Untitled Texture"
	defaultTextureProposalTitle     = "Texture proposal"
	defaultPublishedTextureSlugBase = "published-texture"
)

type Service struct {
	store         *Store
	artifactsRoot string
	signingKey    *SigningKey
	writeMu       sync.Mutex
	// artifactGCMu serializes artifact GC live-set capture and deletion with
	// explicit projection-base pins only. It must never be writeMu: a sweep
	// lasts minutes and would block credential issuance and VM boots.
	artifactGCMu sync.Mutex
	// artifactGCLogMu serializes GC-log rotation. It is deliberately not
	// artifactGCMu: compressing a large log takes minutes and that lock also
	// serializes projection-base pins.
	artifactGCLogMu sync.Mutex
	// artifactGCPolicy is the configured GC policy; the on-demand endpoint
	// may narrow it but never exceed it.
	artifactGCPolicyMu sync.Mutex
	artifactGCPolicy   *ArtifactGCConfig
	// artifactGCBeforeUnlink is a test hook run before each locked unlink.
	artifactGCBeforeUnlink func(rel string)
	// tapeSweep is the tape sweep's run lock, policy, registry and jobs
	// (tape_sweep.go).
	tapeSweep tapeSweepState
	// tapeReaders caches the segment-first tape reader per artifacts root
	// (event_artifacts.go tapeReader).
	tapeReaderMu  sync.Mutex
	tapeReaderVal *tapestore.Reader
	// tapeAppenderVal is the segment writer (tape_segments_write.go), under
	// the same lock and root rule as the reader. tapeSegMode is
	// PLATFORM_TAPE_SEGMENTS (0 off, 1 write); tapeCount are the diagnostics
	// counters. A zero Service is mode off.
	tapeAppenderVal *tapestore.Appender
	tapeSegMode     atomic.Int32
	tapeCount       tapeCounters
	graphStore      *PublicationGraphStore // graph-native dual-write target
	// captureBodies reads cited captures' blocks at publish, to pin them.
	captureBodies *CaptureBodyStore
	// overlay supplies the live check overlay served beside a version.
	overlayMu sync.RWMutex
	overlay   CheckOverlayProvider
	// autopaperOwners, when non-empty, limits the front list to these
	// owners' stories (CORPUSD_AUTOPAPER_OWNERS).
	frontMu         sync.RWMutex
	autopaperOwners map[string]bool
	// publicBaseURL is the host a published piece's read_url names
	// (CORPUSD_PUBLIC_BASE_URL); empty means DefaultPublicBaseURL.
	publicBaseURL string
	// opinionListing is the opinion kill switch (CORPUSD_OPINION_LISTING);
	// off by default. opinionDailyLimit is the per-user daily cap on new
	// opinion pieces (CORPUSD_OPINION_DAILY_LIMIT); zero means the default.
	opinionListing    bool
	opinionDailyLimit int
	// frontImportance attaches model-judged importance scores to front
	// rows when its scores file is present (publication_front_importance.go).
	frontImportance frontImportanceCache
	// backfillMu admits one cites_capture backfill at a time
	// (publication_related_backfill.go).
	backfillMu sync.Mutex
	// related is the derived state of the related-stories walk: answer
	// cache, memo, story count (publication_related.go).
	related relatedCache
}

type citationInput struct {
	ID       string          `json:"id"`
	URL      string          `json:"url"`
	URI      string          `json:"uri"`
	Ref      string          `json:"ref"`
	Title    string          `json:"title"`
	Selector json.RawMessage `json:"selector"`
	State    string          `json:"state"`
}

// NewService builds a platform Service. signingKeyPath is the platform Ed25519
// signing key (loaded or auto-created); an empty path disables D6 signing
// (signatures omitted, publish still works) for callers that opt out.
func NewService(store *Store, artifactsRoot, signingKeyPath string) *Service {
	svc := &Service{
		store:         store,
		artifactsRoot: filepath.Clean(artifactsRoot),
		captureBodies: &CaptureBodyStore{root: CaptureBodiesRoot(artifactsRoot)},
	}
	svc.related.init()
	store.SetArtifactsRoot(artifactsRoot)
	if signingKeyPath != "" {
		if key, err := LoadOrCreateSigningKey(signingKeyPath); err == nil {
			svc.signingKey = key
		}
	}
	// Wire graph-native publication store.
	ogStore := NewObjectGraphStore(store)
	svc.graphStore = NewPublicationGraphStore(ogStore)
	return svc
}

func (s *Service) Health(ctx context.Context) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("platform service unavailable")
	}
	return s.store.Ping(ctx)
}

func (s *Service) SyncTextureDocument(ctx context.Context, req SyncTextureDocumentRequest) (*SyncTextureDocumentResponse, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	req.DocID = strings.TrimSpace(req.DocID)
	req.OwnerID = strings.TrimSpace(req.OwnerID)
	req.Title = strings.TrimSpace(req.Title)
	if req.DocID == "" {
		return nil, fmt.Errorf("doc_id is required")
	}
	if req.OwnerID == "" {
		return nil, fmt.Errorf("owner_id is required")
	}

	if err := s.store.UpsertTextureDocument(ctx, req.DocID, req.OwnerID, req.Title); err != nil {
		return nil, fmt.Errorf("platform sync texture: upsert document: %w", err)
	}

	for _, rev := range req.Revisions {
		platformRev := PlatformTextureRevision{
			RevisionID:       strings.TrimSpace(rev.RevisionID),
			DocID:            req.DocID,
			OwnerID:          req.OwnerID,
			ParentRevisionID: strings.TrimSpace(rev.ParentRevisionID),
			AuthorKind:       strings.TrimSpace(rev.AuthorKind),
			AuthorLabel:      strings.TrimSpace(rev.AuthorLabel),
			Content:          rev.Content,
			BodyDoc:          rev.BodyDoc,
			SourceEntities:   rev.SourceEntities,
			Citations:        rev.Citations,
			Metadata:         rev.Metadata,
			CreatedAt:        rev.CreatedAt,
		}
		if platformRev.RevisionID == "" {
			continue
		}
		if err := s.store.UpsertTextureRevision(ctx, platformRev); err != nil {
			return nil, fmt.Errorf("platform sync texture: upsert revision %s: %w", platformRev.RevisionID, err)
		}
	}

	s.store.markCorpusDirty("sync texture document " + req.DocID + " with " + fmt.Sprintf("%d", len(req.Revisions)) + " revisions")

	return &SyncTextureDocumentResponse{
		DocID:         req.DocID,
		RevisionCount: len(req.Revisions),
	}, nil
}

func (s *Service) GetPlatformTextureDocument(ctx context.Context, docID string) (*PlatformTextureDocument, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	return s.store.GetTextureDocument(ctx, docID)
}

func (s *Service) ListPlatformTextureRevisions(ctx context.Context, docID string) ([]PlatformTextureRevision, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	return s.store.ListTextureRevisions(ctx, docID)
}

func (s *Service) GetPlatformTextureRevision(ctx context.Context, revisionID string) (*PlatformTextureRevision, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	return s.store.GetTextureRevision(ctx, revisionID)
}

// PublishTexture publishes one committed Texture revision as version n of
// the one publication of (owner, source doc), n = req.RevisionNumber (Cut 2
// unit 2b). Publishing the same revision again returns the version already
// published and writes nothing. A revision below the latest published
// version is refused: a version that failed to publish stays a gap.
func (s *Service) PublishTexture(ctx context.Context, req PublishTextureRequest) (*PublishTextureResponse, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	req.OwnerID = strings.TrimSpace(req.OwnerID)
	req.SourceDocID = strings.TrimSpace(req.SourceDocID)
	req.SourceRevisionID = strings.TrimSpace(req.SourceRevisionID)
	req.RevisionHash = strings.TrimSpace(req.RevisionHash)
	req.Title = strings.TrimSpace(req.Title)
	req.RequestedBy = strings.TrimSpace(req.RequestedBy)
	req.SourceTraceID = strings.TrimSpace(req.SourceTraceID)
	req.PublishMode = strings.TrimSpace(req.PublishMode)
	if req.OwnerID == "" {
		return nil, fmt.Errorf("owner_id is required")
	}
	switch req.PublishMode {
	case "":
		req.PublishMode = PublishModeOwner
	case PublishModeAuto, PublishModeOwner, PublishModeOpinion:
	default:
		return nil, fmt.Errorf("publish_mode %q is not %q, %q or %q", req.PublishMode, PublishModeAuto, PublishModeOwner, PublishModeOpinion)
	}
	req.AuthorLabel = strings.TrimSpace(req.AuthorLabel)
	if req.RequestedBy == "" {
		req.RequestedBy = req.OwnerID
	}
	if req.SourceDocID == "" {
		return nil, fmt.Errorf("source_doc_id is required")
	}
	if req.SourceRevisionID == "" {
		return nil, fmt.Errorf("source_revision_id is required")
	}
	if req.RevisionNumber <= 0 {
		return nil, fmt.Errorf("revision_number is required: the source revision's number on its revision chain, counted from 1")
	}
	if req.Title == "" {
		req.Title = defaultUntitledTextureTitle
	}
	if req.Citations == nil {
		req.Citations = json.RawMessage("[]")
	}
	if !json.Valid(req.Citations) {
		return nil, fmt.Errorf("citations must be valid JSON")
	}
	if req.Metadata == nil {
		req.Metadata = json.RawMessage("{}")
	}
	if err := normalizePublishTextureStructuredInput(&req); err != nil {
		return nil, err
	}
	if req.Content == "" {
		return nil, fmt.Errorf("content is required")
	}
	sourceMetadata, err := buildPublicationSourceMetadata(req)
	if err != nil {
		return nil, err
	}

	og := s.ogStore()
	if og == nil {
		return nil, fmt.Errorf("platform service: object graph store unavailable")
	}
	ident, err := publicationIdentityFor(req.OwnerID, req.SourceDocID)
	if err != nil {
		return nil, fmt.Errorf("platform publish: publication id: %w", err)
	}
	newPublication := false
	pubObj, err := og.GetObject(ctx, ident.CanonicalID)
	switch {
	case errors.Is(err, objectgraph.ErrNotFound):
		newPublication = true
	case err != nil:
		return nil, fmt.Errorf("platform publish: read publication: %w", err)
	}
	versions, err := publishedVersions(ctx, og, ident.CanonicalID)
	if err != nil {
		return nil, fmt.Errorf("platform publish: list versions: %w", err)
	}
	revision := req.RevisionNumber
	if prior, ok := findPublishedVersion(versions, revision); ok {
		return replayedPublication(ctx, og, prior, req)
	}
	var previous *versionMetadata
	previousCanonicalID := ""
	if len(versions) > 0 {
		latest := versions[len(versions)-1]
		if revision < latest.Revision {
			return nil, fmt.Errorf("revision %d precedes the latest published version v%d; a version that failed to publish stays a gap", revision, latest.Revision)
		}
		latestObj, err := og.GetObject(ctx, latest.CanonicalID)
		if err != nil {
			return nil, fmt.Errorf("platform publish: read v%d: %w", latest.Revision, err)
		}
		meta, err := decodeVersionMetadata(latestObj)
		if err != nil {
			return nil, err
		}
		previous, previousCanonicalID = &meta, latest.CanonicalID
	}

	// The bare route is fixed by the first version: a story's link never
	// changes, whatever later titles or slugs say.
	// So are its publish mode and owner: a story stays a story, and a hand
	// published Texture stays off the front, whatever later versions say.
	routePath := publicTexturePrefix + slugify(firstNonEmpty(req.Slug, req.Title)) + "-" + shortID(ident.PublicationID)
	publishMode, publicationOwner, authorLabel := req.PublishMode, req.OwnerID, ""
	existingMode := ""
	if !newPublication {
		var pubMeta struct {
			RoutePath   string `json:"route_path"`
			PublishMode string `json:"publish_mode"`
			OwnerID     string `json:"owner_id"`
			AuthorLabel string `json:"author_label"`
		}
		if err := json.Unmarshal(pubObj.Metadata, &pubMeta); err != nil || strings.TrimSpace(pubMeta.RoutePath) == "" {
			return nil, fmt.Errorf("platform publish: publication %s has no route", ident.PublicationID)
		}
		routePath = pubMeta.RoutePath
		// A publication written before modes were recorded takes this
		// version's mode.
		existingMode = strings.TrimSpace(pubMeta.PublishMode)
		publishMode = firstNonEmpty(existingMode, publishMode)
		publicationOwner = firstNonEmpty(strings.TrimSpace(pubMeta.OwnerID), publicationOwner)
		authorLabel = strings.TrimSpace(pubMeta.AuthorLabel)
	}
	// An opinion piece is a public, labelled listing: the switch, the byline,
	// the mode and the user's daily limit are checked before anything is
	// written. Its byline is fixed by the first version.
	if req.PublishMode == PublishModeOpinion {
		if err := s.checkOpinionPublish(ctx, req, newPublication, existingMode); err != nil {
			return nil, err
		}
	}
	if publishMode == PublishModeOpinion && authorLabel == "" {
		authorLabel = req.AuthorLabel
	}
	versionID := publicationVersionID(ident.PublicationID, revision)
	versionCanonicalID, err := publicationVersionCanonicalID(req.OwnerID, versionID)
	if err != nil {
		return nil, fmt.Errorf("platform publish: version id: %w", err)
	}
	permalink := versionPermalink(routePath, revision)

	// Pin every cited and embedded capture block into the version.
	anchors, err := publicationAnchors(req.BodyDoc)
	if err != nil {
		return nil, err
	}
	anchors, resolvedBlocks, err := s.resolveAnchors(ctx, anchors)
	if err != nil {
		return nil, fmt.Errorf("platform publish: %w", err)
	}
	if anchors == nil {
		anchors = []PublicationAnchor{}
	}
	resolvedBlocksJSON, resolvedBlocksHash, err := marshalResolvedBlocks(resolvedBlocks)
	if err != nil {
		return nil, fmt.Errorf("platform publish: marshal resolved blocks: %w", err)
	}
	anchored := map[string]bool{}
	for _, a := range anchors {
		anchored[a.SourceEntityID] = true
	}
	legacyTransclusions := make([]publicationTransclusionInput, 0, len(sourceMetadata.Transclusions))
	for _, tr := range sourceMetadata.Transclusions {
		if !anchored[tr.SourceEntityID] {
			legacyTransclusions = append(legacyTransclusions, tr)
		}
	}

	now := time.Now().UTC()
	proposalID := id("pubprop")
	manifestID := "artman-" + versionID
	sourceID := id("source")
	spanID := id("span")
	retrievalManifestID := id("retman")
	consentID := id("consent")
	reviewID := id("review")
	activityID := id("activity")
	attestationID := id("att")

	contentHash := sha256Hex([]byte(req.Content))
	projectionHash := contentHash
	sourceRevisionHash := sha256Hex([]byte(req.SourceDocID + "\n" + req.SourceRevisionID + "\n" + req.Content + "\n" + string(req.BodyDoc) + "\n" + string(req.SourceEntities) + "\n" + string(req.Citations) + "\n" + string(req.Metadata)))
	storageRef := filepath.Join("sha256", contentHash+".txt")
	if err := s.writeBlob(storageRef, []byte(req.Content)); err != nil {
		return nil, err
	}

	previousManifestHash, previousRevision, supersedesVersionID := "", 0, ""
	if previous != nil {
		previousManifestHash, previousRevision, supersedesVersionID = previous.ManifestHash, previous.Revision, previous.PublicationVersionID
	}
	manifestJSON, err := json.Marshal(publicationManifest{
		Schema:                   publicationManifestSchema,
		SubjectKind:              "publication_version",
		PublicationID:            ident.PublicationID,
		PublicationVersionID:     versionID,
		Revision:                 revision,
		RoutePath:                routePath,
		PermalinkPath:            permalink,
		Title:                    req.Title,
		MediaType:                textMediaType,
		ContentHash:              contentHash,
		ProjectionHash:           projectionHash,
		StorageRef:               storageRef,
		ByteSize:                 len([]byte(req.Content)),
		SourceRevisionHash:       sourceRevisionHash,
		RevisionHash:             req.RevisionHash,
		PreviousManifestHash:     previousManifestHash,
		PreviousRevision:         previousRevision,
		SourceMetadataHash:       sourceMetadata.MetadataHash,
		BodyDoc:                  req.BodyDoc,
		StructuredSourceEntities: req.SourceEntities,
		SourceEntities:           sourceMetadata.SourceEntities,
		Transclusions:            legacyTransclusions,
		Anchors:                  anchors,
		ResolvedBlocksHash:       resolvedBlocksHash,
		AccessPolicy:             sourceMetadata.AccessPolicy,
		ExportPolicy:             sourceMetadata.ExportPolicy,
	})
	if err != nil {
		return nil, fmt.Errorf("platform publish: marshal artifact manifest: %w", err)
	}
	manifestHash := sha256Hex(manifestJSON)
	signatures := []PublicationSignature{}
	if s.signingKey != nil {
		sig, err := s.signingKey.signManifest(manifestHash)
		if err != nil {
			return nil, fmt.Errorf("platform publish: sign manifest: %w", err)
		}
		signatures = append(signatures, PublicationSignature{Schema: manifestAttestationSchema, KeyID: s.signingKey.KeyID, PublicKey: s.signingKey.PublicKeyBase64(), Signature: sig})
	}

	wholeSelector := `{"type":"TextPositionSelector","start":0,"end":` + fmt.Sprintf("%d", len([]rune(req.Content))) + `}`
	retrievalSelectedRefs := mustJSON([]map[string]string{{
		"source_id":  sourceID,
		"span_id":    spanID,
		"version_id": versionID,
	}})
	publicURI := "choir:" + strings.TrimPrefix(permalink, "/")

	citationIDs := []string{}
	sourceCitationID := id("cite")
	externalCitations := parseCitationInputs(req.Citations)

	// Write to the object graph — single path, no relational tables.
	graphParams := PublishGraphParams{
		OwnerID:                    req.OwnerID,
		RequestedBy:                req.RequestedBy,
		SourceDocID:                req.SourceDocID,
		SourceRevisionID:           req.SourceRevisionID,
		SourceRevisionHash:         sourceRevisionHash,
		SourceTraceID:              req.SourceTraceID,
		Title:                      req.Title,
		Slug:                       strings.TrimPrefix(routePath, publicTexturePrefix),
		Content:                    req.Content,
		ContentHash:                contentHash,
		ContentSize:                len([]byte(req.Content)),
		ProjectionHash:             projectionHash,
		WholeSelector:              wholeSelector,
		PublicURI:                  publicURI,
		RoutePath:                  routePath,
		StorageRef:                 storageRef,
		ManifestJSON:               manifestJSON,
		ManifestHash:               manifestHash,
		TokenCount:                 len(strings.Fields(req.Content)),
		SelectedRefsJSON:           json.RawMessage(retrievalSelectedRefs),
		PublicationCanonicalID:     ident.CanonicalID,
		VersionCanonicalID:         versionCanonicalID,
		PreviousVersionCanonicalID: previousCanonicalID,
		NewPublication:             newPublication,
		PublishMode:                publishMode,
		PublicationOwnerID:         publicationOwner,
		AuthorLabel:                authorLabel,
		Revision:                   revision,
		VersionMetadata: versionMetadata{
			EditionLabel:         "v" + strconv.Itoa(revision),
			Revision:             revision,
			PublicationID:        ident.PublicationID,
			PublicationVersionID: versionID,
			SourceDocID:          req.SourceDocID,
			SourceRevisionID:     req.SourceRevisionID,
			SourceRevisionHash:   sourceRevisionHash,
			RevisionHash:         req.RevisionHash,
			ProjectionHash:       projectionHash,
			ArtifactManifestID:   manifestID,
			ManifestHash:         manifestHash,
			PreviousManifestHash: previousManifestHash,
			SupersedesVersionID:  supersedesVersionID,
			RoutePath:            routePath,
			PermalinkPath:        permalink,
			PublishedAt:          now.Format(time.RFC3339Nano),
			Signatures:           signatures,
		},
		ResolvedBlocksJSON:   resolvedBlocksJSON,
		ResolvedBlocksHash:   resolvedBlocksHash,
		PublicationID:        ident.PublicationID,
		ProposalID:           proposalID,
		PublicationVersionID: versionID,
		ArtifactManifestID:   manifestID,
		ConsentID:            consentID,
		ReviewID:             reviewID,
		RetrievalSourceID:    sourceID,
		RetrievalSpanID:      spanID,
		RetrievalManifestID:  retrievalManifestID,
		ActivityID:           activityID,
		AttestationID:        attestationID,
		AttestationEvidenceJSON: json.RawMessage(mustJSON(map[string]string{
			"route_path":           permalink,
			"source_revision_hash": sourceRevisionHash,
			"manifest_hash":        manifestHash,
		})),
		AccessPolicy: sourceMetadata.AccessPolicy,
		ExportPolicy: sourceMetadata.ExportPolicy,
		Now:          now,
	}
	for _, se := range sourceMetadata.SourceEntities {
		graphParams.SourceEntities = append(graphParams.SourceEntities, GraphSourceEntity{
			SourceEntityID: se.SourceEntityID,
			Kind:           se.Kind,
			TargetKind:     se.TargetKind,
			TargetID:       se.TargetID,
			DisplayPolicy:  se.DisplayPolicy,
			OpenSurface:    se.OpenSurface,
			EntityJSON:     se.EntityJSON,
		})
	}
	for _, tr := range legacyTransclusions {
		graphParams.Transclusions = append(graphParams.Transclusions, GraphTransclusion{
			SourceEntityID:     tr.SourceEntityID,
			HostSelector:       tr.HostSelector,
			SourceSelector:     tr.SourceSelector,
			RelationType:       tr.RelationType,
			DefaultDisplayMode: tr.DefaultDisplayMode,
			SnapshotText:       tr.SnapshotText,
			ContentHash:        tr.ContentHash,
			EntityJSON:         tr.EntityJSON,
		})
	}
	for _, a := range anchors {
		ref := a.Ref
		text := anchorText(a, resolvedBlocks)
		display := "collapsed_citation"
		if a.Relation == anchorRelationEmbeds {
			display = "expanded"
		}
		graphParams.Transclusions = append(graphParams.Transclusions, GraphTransclusion{
			AnchorID:           a.AnchorID,
			Ref:                &ref,
			SourceEntityID:     a.SourceEntityID,
			HostSelector:       mustJSONRaw(map[string]any{"type": "source_ref", "node_id": a.AnchorID}),
			SourceSelector:     mustJSONRaw(map[string]any{"selector_kind": "block_ref", "doc": ref.Doc, "version": ref.Version, "cids": a.CIDs}),
			RelationType:       a.Relation,
			DefaultDisplayMode: display,
			SnapshotText:       text,
			ContentHash:        blockTextHash(text),
		})
	}
	for _, cite := range externalCitations {
		toID, ok := publicCitationTarget(cite)
		if !ok {
			continue
		}
		selector := "{}"
		if len(cite.Selector) > 0 && json.Valid(cite.Selector) {
			selector = string(cite.Selector)
		}
		state := strings.TrimSpace(cite.State)
		if state == "" {
			state = "candidate"
		}
		graphParams.Citations = append(graphParams.Citations, GraphCitation{
			ToID:         toID,
			RelationType: "references",
			FromSelector: wholeSelector,
			ToSelector:   selector,
			State:        state,
			EvidenceRef:  firstNonEmpty(cite.Title, toID),
			Confidence:   0.5,
		})
		citationIDs = append(citationIDs, toID)
	}
	// Source citation edge (is_version_of)
	graphParams.Citations = append(graphParams.Citations, GraphCitation{
		ToID:         req.SourceRevisionID,
		RelationType: "is_version_of",
		FromSelector: wholeSelector,
		ToSelector:   wholeSelector,
		State:        "accepted",
		EvidenceRef:  "source_revision_hash:" + sourceRevisionHash,
		Confidence:   1.0,
	})
	citationIDs = append(citationIDs, sourceCitationID)

	if err := s.graphStore.PublishTextureToGraph(ctx, graphParams); err != nil {
		return nil, fmt.Errorf("platform publish: graph write: %w", err)
	}
	// The version's quote checks go beside it in Store B, keyed by version
	// and citation, outside the manifest (Cut 2 2b). A failed write leaves
	// those citations pending in the live overlay; the version stands.
	if err := s.writeVersionChecks(ctx, req.OwnerID, versionID, anchors, req.Metadata, now); err != nil {
		log.Printf("platform publish: %s checks: %v", versionID, err)
	}

	return &PublishTextureResponse{
		PublicationID:        ident.PublicationID,
		ProposalID:           proposalID,
		PublicationVersionID: versionID,
		ArtifactManifestID:   manifestID,
		ContentHash:          contentHash,
		SourceRevisionHash:   sourceRevisionHash,
		ProjectionHash:       projectionHash,
		RoutePath:            routePath,
		RetrievalSourceID:    sourceID,
		RetrievalSpanIDs:     []string{spanID},
		CitationIDs:          citationIDs,
		ConsentID:            consentID,
		ReviewID:             reviewID,
		RollbackID:           "",
		State:                "published",
		Revision:             revision,
		PermalinkPath:        permalink,
		ManifestHash:         manifestHash,
		PreviousManifestHash: previousManifestHash,
		SupersedesVersionID:  supersedesVersionID,
	}, nil
}

// replayedPublication answers a publish of a revision already published:
// the same version, nothing written. A different revision under the same
// number is refused; a revision chain never reuses a number.
func replayedPublication(ctx context.Context, og *ObjectGraphStore, prior publishedVersion, req PublishTextureRequest) (*PublishTextureResponse, error) {
	obj, err := og.GetObject(ctx, prior.CanonicalID)
	if err != nil {
		return nil, fmt.Errorf("platform publish: read v%d: %w", prior.Revision, err)
	}
	meta, err := decodeVersionMetadata(obj)
	if err != nil {
		return nil, err
	}
	if meta.SourceRevisionID != req.SourceRevisionID {
		return nil, fmt.Errorf("version v%d was published from revision %s, not %s", prior.Revision, meta.SourceRevisionID, req.SourceRevisionID)
	}
	return &PublishTextureResponse{
		PublicationID:        meta.PublicationID,
		PublicationVersionID: meta.PublicationVersionID,
		ArtifactManifestID:   meta.ArtifactManifestID,
		ContentHash:          obj.ContentHash,
		SourceRevisionHash:   meta.SourceRevisionHash,
		ProjectionHash:       meta.ProjectionHash,
		RoutePath:            meta.RoutePath,
		RetrievalSpanIDs:     []string{},
		CitationIDs:          []string{},
		State:                "published",
		Revision:             meta.Revision,
		PermalinkPath:        meta.PermalinkPath,
		ManifestHash:         meta.ManifestHash,
		PreviousManifestHash: meta.PreviousManifestHash,
		SupersedesVersionID:  meta.SupersedesVersionID,
		Replayed:             true,
	}, nil
}

func (s *Service) SubmitPublicationProposal(ctx context.Context, req SubmitPublicationProposalRequest) (*SubmitPublicationProposalResponse, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	req.PublicationID = strings.TrimSpace(req.PublicationID)
	req.PublicationVersionID = strings.TrimSpace(req.PublicationVersionID)
	req.SubmitterID = strings.TrimSpace(req.SubmitterID)
	req.SubmitterDocID = strings.TrimSpace(req.SubmitterDocID)
	req.SubmitterRevisionID = strings.TrimSpace(req.SubmitterRevisionID)
	req.Title = strings.TrimSpace(req.Title)
	req.RequestedBy = strings.TrimSpace(req.RequestedBy)
	if req.PublicationID == "" {
		return nil, fmt.Errorf("publication_id is required")
	}
	if req.SubmitterID == "" {
		return nil, fmt.Errorf("submitter_id is required")
	}
	if req.RequestedBy == "" {
		req.RequestedBy = req.SubmitterID
	}
	if req.RequestedBy != req.SubmitterID {
		return nil, fmt.Errorf("requested_by must match submitter_id")
	}
	if req.SubmitterDocID == "" {
		return nil, fmt.Errorf("submitter_doc_id is required")
	}
	if req.SubmitterRevisionID == "" {
		return nil, fmt.Errorf("submitter_revision_id is required")
	}
	if req.Content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if req.Title == "" {
		req.Title = defaultTextureProposalTitle
	}
	if req.Citations == nil {
		req.Citations = json.RawMessage("[]")
	}
	if !json.Valid(req.Citations) {
		return nil, fmt.Errorf("citations must be valid JSON")
	}

	var sourceOwnerID string
	// Look up the publication in the object graph by publication_id metadata.
	og := s.ogStore()
	if og == nil {
		return nil, fmt.Errorf("platform service: object graph store unavailable")
	}
	pubObjs, err := og.ListObjectsByMetadata(ctx, "choir.publication", "$.publication_id", req.PublicationID, 1)
	if err != nil || len(pubObjs) == 0 {
		return nil, fmt.Errorf("publication not found")
	}
	pubObj := pubObjs[0]
	var pubMeta struct {
		State string `json:"state"`
	}
	_ = json.Unmarshal(pubObj.Metadata, &pubMeta)
	sourceOwnerID = pubObj.OwnerID
	if pubMeta.State != "published" {
		return nil, fmt.Errorf("publication not found")
	}
	if req.PublicationVersionID == "" {
		versions, err := publishedVersions(ctx, og, pubObj.CanonicalID)
		if err != nil || len(versions) == 0 {
			return nil, fmt.Errorf("publication version not found")
		}
		latestObj, err := og.GetObject(ctx, versions[len(versions)-1].CanonicalID)
		if err != nil {
			return nil, fmt.Errorf("publication version not found")
		}
		latestMeta, err := decodeVersionMetadata(latestObj)
		if err != nil {
			return nil, err
		}
		req.PublicationVersionID = latestMeta.PublicationVersionID
	}
	// Look up the version in the object graph by publication_version_id metadata.
	versionObjs, err := og.ListObjectsByMetadata(ctx, "choir.publication_version", "$.publication_version_id", req.PublicationVersionID, 1)
	if err != nil || len(versionObjs) == 0 {
		return nil, fmt.Errorf("publication version not found")
	}
	versionContentHash := versionObjs[0].ContentHash

	now := time.Now().UTC()
	proposalID := id("readerprop")
	manifestID := id("artman")
	deliveryID := id("delivery")
	attestationID := id("att")
	activityID := id("activity")
	contentHash := sha256Hex([]byte(req.Content))
	transclusionsJSON, err := json.Marshal(req.Transclusions)
	if err != nil {
		return nil, fmt.Errorf("platform proposal: marshal transclusions: %w", err)
	}
	proposalRevisionHash := sha256Hex([]byte(req.SubmitterDocID + "\n" + req.SubmitterRevisionID + "\n" + req.Content + "\n" + string(transclusionsJSON) + "\n" + string(req.Citations)))
	projectionHash := contentHash
	storageRef := filepath.Join("sha256", "proposals", contentHash+".txt")
	if err := s.writeBlob(storageRef, []byte(req.Content)); err != nil {
		return nil, err
	}
	manifest := map[string]any{
		"schema":                        "choir.platform.proposal_artifact_manifest.v0",
		"subject_kind":                  "publication_version_proposal",
		"subject_id":                    proposalID,
		"media_type":                    textMediaType,
		"content_hash":                  contentHash,
		"proposal_revision_hash":        proposalRevisionHash,
		"projection_hash":               projectionHash,
		"storage_ref":                   storageRef,
		"byte_size":                     len([]byte(req.Content)),
		"source_publication_id":         req.PublicationID,
		"source_publication_version_id": req.PublicationVersionID,
		"submitter_doc_id":              req.SubmitterDocID,
		"submitter_revision_id":         req.SubmitterRevisionID,
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("platform proposal: marshal artifact manifest: %w", err)
	}
	manifestHash := sha256Hex(manifestJSON)

	// Build transclusion edges for graph write.
	var transclusionEdges []ProposalTransclusionEdge
	transclusionIDs := []string{}
	for _, ref := range req.Transclusions {
		selector := "{}"
		if len(ref.Selector) > 0 && json.Valid(ref.Selector) {
			selector = string(ref.Selector)
		}
		toID := firstNonEmpty(ref.SpanID, ref.PublicationVersionID, req.PublicationVersionID)
		toKind := "published_texture_span"
		if ref.SpanID == "" {
			toKind = "publication_version"
		}
		transclusionEdges = append(transclusionEdges, ProposalTransclusionEdge{
			ToID:        toID,
			ToKind:      toKind,
			Selector:    selector,
			EvidenceRef: "source_content_hash:" + firstNonEmpty(ref.ContentHash, versionContentHash),
		})
		transclusionIDs = append(transclusionIDs, toID)
	}

	// Build citation edges for graph write.
	var citationEdges []ProposalCitationEdge
	citationIDs := []string{}
	for _, citation := range parseCitationInputs(req.Citations) {
		toID, ok := publicCitationTarget(citation)
		if !ok {
			continue
		}
		citationEdges = append(citationEdges, ProposalCitationEdge{
			ToID:        toID,
			EvidenceRef: firstNonEmpty(citation.Title, toID),
		})
		citationIDs = append(citationIDs, toID)
	}

	// Write to the object graph — single path, no relational tables.
	proposalParams := ProposalGraphParams{
		ProposalID:               proposalID,
		PublicationID:            req.PublicationID,
		PublicationVersionID:     req.PublicationVersionID,
		SourceOwnerID:            sourceOwnerID,
		SubmitterID:              req.SubmitterID,
		SubmitterDocID:           req.SubmitterDocID,
		SubmitterRevisionID:      req.SubmitterRevisionID,
		ProposalRevisionHash:     proposalRevisionHash,
		Content:                  req.Content,
		ContentHash:              contentHash,
		ContentSize:              len([]byte(req.Content)),
		ProjectionHash:           projectionHash,
		Title:                    req.Title,
		ArtifactManifestID:       manifestID,
		ManifestJSON:             manifestJSON,
		ManifestHash:             manifestHash,
		StorageRef:               storageRef,
		DeliveryID:               deliveryID,
		DeliveryRef:              "platform-dolt:publication_version_proposals/" + proposalID,
		ActivityID:               activityID,
		AttestationID:            attestationID,
		SourceVersionContentHash: versionContentHash,
		TransclusionEdges:        transclusionEdges,
		CitationEdges:            citationEdges,
		Now:                      now,
	}
	if err := s.graphStore.SubmitProposalToGraph(ctx, proposalParams); err != nil {
		return nil, fmt.Errorf("platform proposal: graph write: %w", err)
	}

	return &SubmitPublicationProposalResponse{
		ProposalID:           proposalID,
		PublicationID:        req.PublicationID,
		PublicationVersionID: req.PublicationVersionID,
		SourceOwnerID:        sourceOwnerID,
		SubmitterID:          req.SubmitterID,
		ContentHash:          contentHash,
		ProposalRevisionHash: proposalRevisionHash,
		ArtifactManifestID:   manifestID,
		TransclusionIDs:      transclusionIDs,
		CitationIDs:          citationIDs,
		DeliveryID:           deliveryID,
		DeliveryState:        "recorded_for_author",
		State:                "proposed",
	}, nil
}

func (s *Service) UpdateProposalDeliveryState(ctx context.Context, req UpdateProposalDeliveryStateRequest) (*UpdateProposalDeliveryStateResponse, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service is not initialized")
	}
	req.ProposalID = strings.TrimSpace(req.ProposalID)
	req.DeliveryID = strings.TrimSpace(req.DeliveryID)
	req.DeliveryState = normalizeProposalDeliveryState(req.DeliveryState)
	req.DeliveryRef = strings.TrimSpace(req.DeliveryRef)
	if req.ProposalID == "" {
		return nil, fmt.Errorf("proposal_id is required")
	}
	if req.DeliveryID == "" {
		return nil, fmt.Errorf("delivery_id is required")
	}
	if req.DeliveryState == "" {
		return nil, fmt.Errorf("delivery_state is required")
	}
	if req.DeliveryRef == "" {
		req.DeliveryRef = "platform-dolt:proposal_delivery_records/" + req.DeliveryID
	}
	now := time.Now().UTC()
	if s.graphStore != nil {
		if err := s.graphStore.UpdateProposalDeliveryState(ctx, req.ProposalID, req.DeliveryID, req.DeliveryState, req.DeliveryRef, now); err != nil {
			return nil, fmt.Errorf("platform proposal delivery: update graph: %w", err)
		}
	}
	return &UpdateProposalDeliveryStateResponse{
		ProposalID:    req.ProposalID,
		DeliveryID:    req.DeliveryID,
		DeliveryState: req.DeliveryState,
	}, nil
}

func normalizeProposalDeliveryState(state string) string {
	switch strings.TrimSpace(strings.ToLower(state)) {
	case "recorded_for_author", "queued", "delivered", "failed":
		return strings.TrimSpace(strings.ToLower(state))
	default:
		return ""
	}
}

func (s *Service) writeBlob(storageRef string, data []byte) error {
	path, err := s.artifactPath(storageRef)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("platform artifact: create dir: %w", err)
	}
	tmp := path + ".tmp-" + shortID(id("write"))
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		return fmt.Errorf("platform artifact: write blob: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("platform artifact: install blob: %w", err)
	}
	s.tapeCount.blobFiles.Add(1)
	return nil
}

func publicCitationTarget(c citationInput) (string, bool) {
	for _, candidate := range []string{c.URI, c.URL, c.Ref} {
		target := strings.TrimSpace(candidate)
		lower := strings.ToLower(target)
		if strings.HasPrefix(lower, "https://") ||
			strings.HasPrefix(lower, "http://") ||
			strings.HasPrefix(lower, "doi:") ||
			strings.HasPrefix(lower, "urn:") ||
			strings.HasPrefix(lower, "ipfs://") ||
			strings.HasPrefix(lower, "ar://") ||
			strings.HasPrefix(lower, "choir:pub/") {
			return target, true
		}
	}
	return "", false
}

func (s *Service) readBlob(storageRef string) ([]byte, error) {
	path, err := s.artifactPath(storageRef)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("platform artifact: read blob: %w", err)
	}
	return data, nil
}

func (s *Service) artifactPath(storageRef string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("platform artifact: service unavailable")
	}
	root := filepath.Clean(s.artifactsRoot)
	if root == "" || root == "." {
		return "", fmt.Errorf("platform artifact: root is not configured")
	}
	cleaned := filepath.Clean(strings.TrimLeft(storageRef, string(filepath.Separator)))
	path := filepath.Join(root, cleaned)
	rel, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("platform artifact: invalid storage ref")
	}
	return path, nil
}

// ArtifactPath returns the absolute filesystem path for storageRef within the artifacts root.
func (s *Service) ArtifactPath(storageRef string) (string, error) {
	return s.artifactPath(storageRef)
}

// ArtifactsRoot returns the configured artifacts root directory.
func (s *Service) ArtifactsRoot() string {
	if s == nil {
		return ""
	}
	return s.artifactsRoot
}

func id(prefix string) string {
	return prefix + "-" + uuid.NewString()
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func normalizePublicationRoutePath(routePath string) string {
	normalized := "/" + strings.TrimLeft(strings.TrimSpace(routePath), "/")
	if normalized != "/" && strings.HasPrefix(normalized, publicTexturePrefix) {
		normalized = strings.TrimRight(normalized, "/")
	}
	return normalized
}

func slugify(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return "texture"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range raw {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			b.WriteRune(r)
			lastDash = false
		case unicode.IsSpace(r) || r == '-' || r == '_' || r == '.':
			if b.Len() > 0 && !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "texture"
	}
	if len(slug) > 96 {
		slug = strings.Trim(slug[:96], "-")
	}
	if slug == "" {
		return "texture"
	}
	return slug
}

var idShortener = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func shortID(value string) string {
	short := idShortener.ReplaceAllString(value, "")
	if len(short) > 12 {
		short = short[:12]
	}
	if short == "" {
		return "id"
	}
	return short
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func parseCitationInputs(raw json.RawMessage) []citationInput {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var citations []citationInput
	if err := json.Unmarshal(raw, &citations); err == nil {
		return citations
	}
	var wrapper struct {
		Citations []citationInput `json:"citations"`
	}
	if err := json.Unmarshal(raw, &wrapper); err == nil {
		return wrapper.Citations
	}
	return nil
}
