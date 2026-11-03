package textureowner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Station TE (docs/design/texture-evidence-embeds-te-2026-10-25.md section
// 2), fed by the engineer's own screens
// (docs/problems/selfdev-screens-are-not-files-2026-11-03.md). When a
// candidate freezes with screens (and the computer setting
// texture.evidence_embeds is on), the runtime asks this, Texture's own
// package, to embed them in the document bound to the operation's
// trajectory. Each embed names the screen's file in the computer's tree and
// its sha256; the digests and files come only from the operation's frozen
// bundle on this computer (the screens listing), so no desk and no model can
// present a stored image as a candidate's evidence. The revision commits
// through the store's head-pinned lifecycle commit, the transaction the owner
// revision route also uses. It adds embed blocks and nothing else: the
// desk owns every sentence of the document.

// candidateEvidenceSource is the screens listing as Texture reads it.
type candidateEvidenceSource interface {
	CandidateScreens(ctx context.Context, computerID, operationID string) (agentcore.ScreensListing, error)
}

const (
	// textureEvidenceAuthorLabel names Texture's evidence write in history.
	textureEvidenceAuthorLabel = "texture"
	// textureEvidenceMetadataKey records which operation and digests the
	// revision embedded.
	textureEvidenceMetadataKey = "texture_evidence_embed"
	// textureEmbeddedAssetsMetadataKey lists every digest the revision
	// embeds: the pin set, rebuildable from the tape (design section 4).
	textureEmbeddedAssetsMetadataKey = "texture_embedded_assets"
	// textureEvidenceCommitAttempts bounds the retries on a lost head race.
	textureEvidenceCommitAttempts = 3
)

func (h *Handler) candidateEvidence() candidateEvidenceSource {
	if h.evidenceSource != nil {
		return h.evidenceSource
	}
	if h.Core == nil {
		return nil
	}
	return h.Core
}

// EmbedCandidateEvidence implements agentcore.TextureEvidenceEmbedder.
// computerID scopes the Texture document; selfdevComputerID the operation.
func (h *Handler) EmbedCandidateEvidence(ctx context.Context, ownerID, computerID, selfdevComputerID, operationID string) (string, error) {
	if h == nil || h.Store == nil {
		return "", fmt.Errorf("texture evidence: runtime store unavailable")
	}
	ownerID, computerID, operationID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(operationID)
	source := h.candidateEvidence()
	if source == nil || ownerID == "" || computerID == "" || operationID == "" {
		return "", fmt.Errorf("texture evidence: incomplete request")
	}
	listing, err := source.CandidateScreens(ctx, strings.TrimSpace(selfdevComputerID), operationID)
	if err != nil {
		return "", fmt.Errorf("texture evidence: %s: %w", operationID, err)
	}
	if err := checkEvidenceListing(listing, operationID); err != nil {
		return "", err
	}
	// Only screens whose bytes the asset store holds are embedded, so no
	// figure is ever unresolvable (design E4).
	resolvable := listing.Screens[:0:0]
	for _, screen := range listing.Screens {
		if screen.Resolvable {
			resolvable = append(resolvable, screen)
		}
	}
	if len(resolvable) == 0 {
		return "", fmt.Errorf("texture evidence: %s lists no screen whose bytes are saved", operationID)
	}
	listing.Screens = resolvable
	// The listing must be this computer's: the asset route serves only this
	// computer's store (design E5; review bunch 231).
	if want := firstNonEmpty(strings.TrimSpace(selfdevComputerID), computerID); strings.TrimSpace(listing.ComputerID) != want {
		return "", fmt.Errorf("texture evidence: the listing for %s is computer %q's, not %q's", operationID, listing.ComputerID, want)
	}
	trajectory, err := h.Store.GetLifecycleTrajectory(ctx, ownerID, computerID, listing.TrajectoryID)
	if err != nil {
		return "", fmt.Errorf("texture evidence: trajectory %s: %w", listing.TrajectoryID, err)
	}
	docID := strings.TrimSpace(trajectory.SubjectRefs["doc_id"])
	if docID == "" {
		return "", fmt.Errorf("texture evidence: trajectory %s binds no Texture document", listing.TrajectoryID)
	}
	h.textureEditMu.Lock()
	defer h.textureEditMu.Unlock()
	var lastErr error
	for attempt := 0; attempt < textureEvidenceCommitAttempts; attempt++ {
		receipt, retry, err := h.commitEvidenceEmbeds(ctx, ownerID, computerID, docID, listing)
		if err == nil || !retry {
			return receipt, err
		}
		lastErr = err
	}
	return "", fmt.Errorf("texture evidence: %s: the document kept moving: %w", docID, lastErr)
}

// checkEvidenceListing refuses a listing Texture will not embed: no
// trajectory, no screens or more than a bundle carries, or a screen whose
// file is not the operation's screens file (design E4).
func checkEvidenceListing(listing agentcore.ScreensListing, operationID string) error {
	if strings.TrimSpace(listing.OperationID) != operationID || strings.TrimSpace(listing.TrajectoryID) == "" || strings.TrimSpace(listing.ComputerID) == "" {
		return fmt.Errorf("texture evidence: the listing for %s is not bound to its operation and trajectory", operationID)
	}
	if n := len(listing.Screens); n == 0 || n > capsule.MaxScreens {
		return fmt.Errorf("texture evidence: %s lists %d screens; a candidate carries 1 to %d", operationID, n, capsule.MaxScreens)
	}
	dir := yaegikernel.SelfDevelopmentScreensDir(operationID) + "/"
	for _, screen := range listing.Screens {
		if screen.File != dir+screen.Name || !capsule.ValidScreenName(screen.Name) {
			return fmt.Errorf("texture evidence: %s lists a screen outside its screens directory: %q", operationID, screen.File)
		}
	}
	return nil
}

// commitEvidenceEmbeds writes one revision on the current head. retry is
// true when the head moved under it.
func (h *Handler) commitEvidenceEmbeds(ctx context.Context, ownerID, computerID, docID string, listing agentcore.ScreensListing) (string, bool, error) {
	doc, err := h.Store.GetLifecycleDocument(ctx, ownerID, computerID, docID)
	if err != nil {
		return "", false, fmt.Errorf("texture evidence: document %s: %w", docID, err)
	}
	if doc.TrajectoryID != listing.TrajectoryID {
		// The embed job is async and settlement does not wait for it, so the
		// owner's next revise can continue the document before the capture
		// lands (F1). The operation's trajectory must be one the document's
		// current trajectory continues; the commit is on the current one.
		lineage, lineageErr := h.Store.LifecycleTrajectoryLineage(ctx, ownerID, computerID, doc.TrajectoryID, 0)
		if lineageErr != nil {
			return "", false, fmt.Errorf("texture evidence: lineage of %s: %w", doc.TrajectoryID, lineageErr)
		}
		if !slices.Contains(lineage, listing.TrajectoryID) {
			return "", false, fmt.Errorf("texture evidence: document %s names trajectory %q, not the operation's %q", docID, doc.TrajectoryID, listing.TrajectoryID)
		}
	}
	if doc.ArchivedAt != nil {
		return "", false, fmt.Errorf("texture evidence: document %s is archived", docID)
	}
	head, err := h.Store.GetLifecycleRevision(ctx, ownerID, computerID, doc.CurrentRevisionID)
	if err != nil {
		return "", false, fmt.Errorf("texture evidence: head of %s: %w", docID, err)
	}
	body, entities, err := texturewrite.Base(head)
	if err != nil {
		return "", false, fmt.Errorf("texture evidence: read head of %s: %w", docID, err)
	}
	for _, e := range texturedoc.Embeds(body) {
		if e.Kind == texturedoc.EmbedKindCapsuleEvidence && e.OperationID == listing.OperationID {
			out, _ := json.Marshal(map[string]any{"doc_id": docID, "revision_id": head.RevisionID, "embedded": 0, "reason": "the head already embeds this candidate's evidence"})
			return string(out), false, nil
		}
	}
	nodes, err := evidenceEmbedNodes(body, listing)
	if err != nil {
		return "", false, err
	}
	body.Doc.Content = append(body.Doc.Content, nodes...)
	unused := revisionUnusedSourceEntityIDs(head.Metadata)
	projection, err := texturedoc.Project(body, entities, unused...)
	if err != nil {
		return "", false, fmt.Errorf("texture evidence: the document with its evidence is not valid: %w", err)
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return "", false, err
	}
	var entitiesJSON json.RawMessage
	if len(entities) > 0 {
		if entitiesJSON, err = json.Marshal(entities); err != nil {
			return "", false, err
		}
	}
	snapshot, err := h.Store.GetLifecycleSnapshot(ctx, ownerID, computerID, doc.TrajectoryID)
	if err != nil {
		return "", false, fmt.Errorf("texture evidence: lifecycle of %s: %w", docID, err)
	}
	revisionID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join([]string{
		"choir:texture:evidence-embed", doc.TrajectoryID, listing.OperationID, head.RevisionID, objectgraph.SHA256(bodyJSON),
	}, ":"))).String()
	digests := texturedoc.EmbeddedDigests(body)
	embedded := make([]string, 0, len(nodes))
	for _, n := range nodes {
		e, _ := texturedoc.EmbedOf(n)
		embedded = append(embedded, e.Digest)
	}
	// Pin before the commit (design E17): a pin whose commit then fails is
	// an extra hold; a revision without its pin could be orphaned.
	if err := h.pinEmbeddedAssets(digests, docID, revisionID); err != nil {
		return "", false, err
	}
	extra := map[string]any{
		textureEvidenceMetadataKey:       map[string]any{"operation_id": listing.OperationID, "digests": embedded},
		textureEmbeddedAssetsMetadataKey: digests,
		"texture_write_base_revision_id": head.RevisionID,
	}
	if len(unused) > 0 {
		extra["unused_source_entity_ids"] = unused
	}
	metadata := mergeTextureRevisionMetadata(carryForwardTextureMetadataForRecord(json.RawMessage("{}"), head.Metadata, doc.TrajectoryID), extra)
	now := time.Now().UTC()
	rev := types.Revision{
		RevisionID: revisionID, DocID: docID, OwnerID: ownerID, ComputerID: doc.ComputerID, TrajectoryID: doc.TrajectoryID,
		AuthorKind: types.AuthorAppAgent, AuthorLabel: textureEvidenceAuthorLabel,
		Content: projection.Text, BodyDoc: bodyJSON, SourceEntities: entitiesJSON,
		Citations: json.RawMessage("[]"), Metadata: metadata, ParentRevisionID: head.RevisionID, CreatedAt: now,
	}
	commandID := "texture-evidence:" + listing.OperationID + ":" + head.RevisionID
	command := types.CommitLifecycleArtifactHeadRequest{
		OwnerID: ownerID, ComputerID: doc.ComputerID, CommandID: commandID, TrajectoryID: doc.TrajectoryID,
		ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion, ExpectedHeadRevisionID: head.RevisionID,
		Unbound: snapshot.Trajectory.Status != types.TrajectoryLive, Revision: rev,
	}
	graph, err := textureToolSourceGraphWriteSet(rev, materializedTextureEdit{BodyDoc: bodyJSON, SourceEntities: entitiesJSON},
		&types.RunRecord{RunID: commandID, OwnerID: ownerID, ComputerID: doc.ComputerID})
	if err != nil {
		return "", false, fmt.Errorf("texture evidence: source graph: %w", err)
	}
	if command.CommandDigest, err = store.ComputeCommitLifecycleArtifactHeadWithSourceGraphDigest(command, graph); err != nil {
		return "", false, fmt.Errorf("texture evidence: digest the commit: %w", err)
	}
	result, err := h.Store.CommitLifecycleArtifactHeadWithSourceGraph(ctx, command, graph)
	if err != nil {
		// Each attempt re-reads the document and the lifecycle, so a head or
		// status that moved (live to settled) is retried on the new state.
		retry := errors.Is(err, store.ErrConcurrentStateChange) || errors.Is(err, store.ErrStaleDocumentHead) || errors.Is(err, store.ErrLifecycleInvalidTransition)
		return "", retry, fmt.Errorf("texture evidence: commit on %s: %w", docID, err)
	}
	if result.Revision == nil {
		return "", false, fmt.Errorf("texture evidence: the commit returned no revision")
	}
	if !result.Replay {
		h.recordTextureAudit(ctx, "revision_committed", ownerID, doc.ComputerID, doc.TrajectoryID, docID, result.Revision.RevisionID, command.CommandID, command.CommandDigest, result.Trajectory.LifecycleVersion)
		h.emitTextureDocumentRevisionEvent(ctx, ownerID, *result.Revision)
	}
	out, _ := json.Marshal(map[string]any{"doc_id": docID, "revision_id": result.Revision.RevisionID, "embedded": len(nodes), "operation_id": listing.OperationID})
	return string(out), false, nil
}

// evidenceEmbedNodes builds one capsule_evidence embed per screen, in the
// bundle's order, with Texture's words (design section 2). The target names
// the screen's file in the computer's tree and its sha256.
func evidenceEmbedNodes(body texturedoc.StructuredTextureDoc, listing agentcore.ScreensListing) ([]texturedoc.Node, error) {
	used := map[string]bool{}
	var walk func(texturedoc.Node)
	walk = func(n texturedoc.Node) {
		if id, _ := n.Attrs["id"].(string); id != "" {
			used[id] = true
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(body.Doc)
	nodes := make([]texturedoc.Node, 0, len(listing.Screens))
	for i, screen := range listing.Screens {
		id := "embed-" + strings.TrimPrefix(objectgraph.SHA256([]byte(fmt.Sprintf("%s\x00%d\x00%s", listing.OperationID, i, screen.SHA256))), "sha256:")[:16]
		for used[id] {
			id += "-x"
		}
		used[id] = true
		// The operation id is interpolated with any markup character
		// replaced, so it cannot read as a legacy citation and refuse the
		// write (review bunch 231). Screen names are plain by construction.
		wordsOperation := wordsSafe(listing.OperationID)
		caption := fmt.Sprintf("Screenshot %s from candidate %s (%dx%d)", screen.Name, wordsOperation, screen.Width, screen.Height)
		if screen.Blank {
			caption = fmt.Sprintf("Blank screenshot %s from candidate %s (%dx%d): the page rendered nothing", screen.Name, wordsOperation, screen.Width, screen.Height)
		}
		nodes = append(nodes, texturedoc.Node{Type: "embed", Attrs: map[string]any{
			"id": id, "kind": texturedoc.EmbedKindCapsuleEvidence,
			"target": map[string]any{
				"digest": screen.SHA256, "operation_id": listing.OperationID, "computer_id": listing.ComputerID,
				"file": screen.File, "width": screen.Width, "height": screen.Height, "blank": screen.Blank,
			},
			"caption": caption, "alt": "Screenshot " + screen.Name,
		}})
	}
	// The nodes are checked as a document of their own before they join the
	// body, so a bad capture is named here, not as a store refusal.
	probe := texturedoc.StructuredTextureDoc{Schema: texturedoc.SchemaV1, Doc: texturedoc.Node{Type: "doc", Attrs: map[string]any{"id": "evidence-probe"}, Content: nodes}}
	if err := texturedoc.Validate(normalizeNumbers(probe), nil); err != nil {
		return nil, fmt.Errorf("texture evidence: %s: %w", listing.OperationID, err)
	}
	return nodes, nil
}

// wordsSafe is s for Texture's words: the characters the authoring rules or
// the Write grammar read as markup ([]{}()*`) and control characters become
// '-' (as the frame line's frameSafe does).
func wordsSafe(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || strings.ContainsRune("[]{}()*`", r) {
			return '-'
		}
		return r
	}, s)
}

// normalizeNumbers round-trips a document through JSON, so int attributes
// read as the store reads them.
func normalizeNumbers(doc texturedoc.StructuredTextureDoc) texturedoc.StructuredTextureDoc {
	raw, err := json.Marshal(doc)
	if err != nil {
		return doc
	}
	var out texturedoc.StructuredTextureDoc
	if json.Unmarshal(raw, &out) != nil {
		return doc
	}
	return out
}

// pinEmbeddedAssets pins every digest the revision embeds to it.
func (h *Handler) pinEmbeddedAssets(digests []string, docID, revisionID string) error {
	if h.Core == nil {
		return fmt.Errorf("texture evidence: no asset store to pin in")
	}
	assets := h.Core.AssetStore()
	for _, digest := range digests {
		if err := assets.Pin(digest, "texture-revision:"+docID+":"+revisionID); err != nil {
			return fmt.Errorf("texture evidence: pin %s: %w", digest, err)
		}
	}
	return nil
}

// revisionUnusedSourceEntityIDs reads a revision's unused_source_entity_ids.
func revisionUnusedSourceEntityIDs(raw json.RawMessage) []string {
	var meta struct {
		Unused []string `json:"unused_source_entity_ids"`
	}
	_ = json.Unmarshal(raw, &meta)
	return meta.Unused
}

// newEmbeds names the embeds of next, of any kind, whose exact block parent
// does not hold. The owner revision route refuses them: it may keep, delete
// or move an embed, never add or alter one. Only the evidence path above
// writes an embed; owner-uploaded images are a later T1 authority (design
// E7; review bunch 231).
func newEmbeds(parentBody, nextBody json.RawMessage) []string {
	if strings.TrimSpace(string(nextBody)) == "" || strings.TrimSpace(string(nextBody)) == "null" {
		return nil
	}
	var next texturedoc.StructuredTextureDoc
	if err := json.Unmarshal(nextBody, &next); err != nil {
		return nil // the store's validator refuses it with its own reason
	}
	held := map[string]bool{}
	if raw := strings.TrimSpace(string(parentBody)); raw != "" && raw != "null" {
		var parent texturedoc.StructuredTextureDoc
		if err := json.Unmarshal(parentBody, &parent); err == nil {
			for _, n := range parent.Doc.Content {
				if n.Type == "embed" {
					data, _ := json.Marshal(n)
					held[string(data)] = true
				}
			}
		}
	}
	var out []string
	for _, n := range next.Doc.Content {
		if n.Type != "embed" {
			continue
		}
		if data, _ := json.Marshal(n); !held[string(data)] {
			out = append(out, firstNonEmpty(stringAttrOf(n, "id"), "(no id)"))
		}
	}
	return out
}

func stringAttrOf(n texturedoc.Node, key string) string {
	v, _ := n.Attrs[key].(string)
	return strings.TrimSpace(v)
}
