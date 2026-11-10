package textureowner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The revise call (docs/design/autopaper-step0-contracts-2026-11-10.md,
// contract 1): one entry point for every owner write to a Texture that is not
// the desk's own turn. /revise, the prompt bar's focused submit and the
// editor's body save all reach the canonical write through reviseDocument.

const (
	// reviseMaxTextBytes caps the instruction or comment.
	reviseMaxTextBytes = 16 << 10
	// reviseMaxSources caps the published versions one call names.
	reviseMaxSources = 8
	// reviseMaxBodyBytes caps a /revise request body.
	reviseMaxBodyBytes = 2 << 20
	// reviseFingerprintMetadataKey records, on a revision the call wrote, a
	// hash of the request that wrote it: the same request id with another
	// request is a conflict, the same request a replay.
	reviseFingerprintMetadataKey = "revise_request_fingerprint"
	// reviseClientIDMetadataKey records the request id beside it, so a retry
	// after a take continued a settled document (a new trajectory, so a new
	// revision id) still finds the head it wrote.
	reviseClientIDMetadataKey = "revise_client_request_id"
)

// ReviseCase is which of the call's cases a request is, by its shape.
type ReviseCase string

const (
	// ReviseComment is text alone: an instruction or comment; the desk is woken.
	ReviseComment ReviseCase = "comment"
	// ReviseEdit is a body alone: the owner's direct body edit.
	ReviseEdit ReviseCase = "edit"
	// ReviseTake is exactly one source and no text: a verbatim take, no model call.
	ReviseTake ReviseCase = "take"
	// ReviseMerge is text with one or more sources: the desk is woken to merge.
	ReviseMerge ReviseCase = "merge"
)

// ReviseSource names one published version, pinned by its manifest hash.
type ReviseSource struct {
	PublicationVersionID string `json:"publication_version_id"`
	ManifestHash         string `json:"manifest_hash"`
}

// ReviseBody is the owner's direct body edit: the fields of the editor's save.
type ReviseBody struct {
	Content                  string          `json:"content,omitempty"`
	BodyDoc                  json.RawMessage `json:"body_doc,omitempty"`
	SourceEntities           json.RawMessage `json:"source_entities,omitempty"`
	Citations                json.RawMessage `json:"citations,omitempty"`
	Metadata                 json.RawMessage `json:"metadata,omitempty"`
	AllowRebase              bool            `json:"allow_rebase,omitempty"`
	ExpectedLifecycleVersion int64           `json:"expected_lifecycle_version,omitempty"`
}

// ReviseRequest is the one revise call. OwnerID is the authenticated session's
// owner, never a body field. An empty DocID creates a document from the one
// source (a fork). BaseRevisionID is the revision responded to: empty is the
// head, an older one is accepted for a comment or a merge and recorded as the
// base link, and an edit or a take needs the head. ExpectedHeadRevisionID is
// the legacy strict form: a head that moved is a 409 for every case.
type ReviseRequest struct {
	OwnerID                string
	DocID                  string
	BaseRevisionID         string
	ExpectedHeadRevisionID string
	Text                   string
	Body                   *ReviseBody
	Sources                []ReviseSource
	ClientRequestID        string
	// forkedFrom is set by forkDocument: the revision is a fork's first and
	// records the source it was forked from.
	forkedFrom bool
}

// ReviseReceipt is what a committed (or replayed) revise returns.
type ReviseReceipt struct {
	Case         ReviseCase
	DocID        string
	TrajectoryID string
	RevisionID   string
	RequestID    string
	Replay       bool
	// Revision is the stored revision when the call wrote it itself (edit,
	// take); a comment or merge leaves it nil and the desk follows the
	// document revision event stream.
	Revision *types.Revision
}

func (r ReviseReceipt) ownerRevisionResponse() textureOwnerRevisionResponse {
	return textureOwnerRevisionResponse{
		Schema: textureOwnerRevisionSchemaV1, DocID: r.DocID, TrajectoryID: r.TrajectoryID,
		RevisionID: r.RevisionID, RequestID: r.RequestID, Replay: r.Replay,
	}
}

// reviseWireRequest is the JSON body of POST /api/texture/documents/{id}/revise
// and of the fork route. Text is the contract's name; prompt and intent are the
// names the route has always taken.
type reviseWireRequest struct {
	Text                   string         `json:"text,omitempty"`
	Prompt                 string         `json:"prompt,omitempty"`
	Intent                 string         `json:"intent,omitempty"`
	ClientRequestID        string         `json:"client_request_id,omitempty"`
	BaseRevisionID         string         `json:"base_revision_id,omitempty"`
	ExpectedHeadRevisionID string         `json:"expected_head_revision_id,omitempty"`
	Sources                []ReviseSource `json:"sources,omitempty"`
	Body                   *ReviseBody    `json:"body,omitempty"`
}

func (w reviseWireRequest) request(ownerID, docID, clientRequestID string) ReviseRequest {
	return ReviseRequest{
		OwnerID: ownerID, DocID: docID,
		BaseRevisionID: w.BaseRevisionID, ExpectedHeadRevisionID: w.ExpectedHeadRevisionID,
		Text:    firstNonEmpty(w.Text, w.Prompt, w.Intent),
		Sources: w.Sources, Body: w.Body, ClientRequestID: clientRequestID,
	}
}

// classifyRevise decides the case from the request's shape and refuses the
// combinations the table refuses, before anything is read or written.
func classifyRevise(req ReviseRequest) (ReviseCase, error) {
	hasText := strings.TrimSpace(req.Text) != ""
	hasBody := req.Body != nil
	n := len(req.Sources)
	switch {
	case len(req.Text) > reviseMaxTextBytes:
		return "", refuseOwnerRevision(http.StatusRequestEntityTooLarge, "text is too long")
	case n > reviseMaxSources:
		return "", refuseOwnerRevision(http.StatusBadRequest, "too many sources")
	case hasBody && n > 0:
		return "", refuseOwnerRevision(http.StatusBadRequest, "a body edit cannot name sources")
	case hasBody && hasText:
		return "", refuseOwnerRevision(http.StatusBadRequest, "text and body are separate requests")
	case hasBody:
		return ReviseEdit, nil
	case !hasText && n == 0:
		return "", refuseOwnerRevision(http.StatusBadRequest, "prompt or intent is required")
	case !hasText && n > 1:
		return "", refuseOwnerRevision(http.StatusBadRequest, "taking a version names exactly one source; add text to merge several")
	case !hasText:
		return ReviseTake, nil
	case n > 0:
		return ReviseMerge, nil
	default:
		return ReviseComment, nil
	}
}

// reviseDocument is the one revise call. Validation order: the request's
// shape; the document (another owner's is the same 404 as a missing one); a
// replay of the request id; the sources; the base; then one write.
func (h *Handler) reviseDocument(ctx context.Context, req ReviseRequest) (ReviseReceipt, error) {
	req.Text = strings.TrimSpace(req.Text)
	ownerID := strings.TrimSpace(req.OwnerID)
	if ownerID == "" {
		return ReviseReceipt{}, refuseOwnerRevision(http.StatusUnauthorized, "authentication required")
	}
	kase, err := classifyRevise(req)
	if err != nil {
		return ReviseReceipt{}, err
	}
	if len(req.Sources) > 0 {
		seen := map[string]bool{}
		for _, s := range req.Sources {
			id := strings.TrimSpace(s.PublicationVersionID)
			if id == "" || strings.TrimSpace(s.ManifestHash) == "" {
				return ReviseReceipt{}, refuseOwnerRevision(http.StatusBadRequest, "each source needs a publication_version_id and a manifest_hash")
			}
			if seen[id] {
				return ReviseReceipt{}, refuseOwnerRevision(http.StatusBadRequest, "a source is named twice")
			}
			seen[id] = true
		}
		if strings.TrimSpace(req.ClientRequestID) == "" {
			return ReviseReceipt{}, refuseOwnerRevision(http.StatusBadRequest, "client_request_id is required with sources")
		}
	}
	if strings.TrimSpace(req.DocID) == "" {
		return ReviseReceipt{}, refuseOwnerRevision(http.StatusBadRequest, "document ID is required; a fork is made by forkDocument")
	}
	doc, err := h.getTextureDocument(asOwner(ctx), ownerID, req.DocID)
	if err != nil {
		return ReviseReceipt{}, refuseOwnerRevision(http.StatusNotFound, "document not found")
	}
	if kase == ReviseEdit {
		receipt, err := h.commitOwnerBodyEdit(ctx, doc, ownerID, ownerBodyEdit{
			Body: *req.Body, ParentRevisionID: strings.TrimSpace(req.BaseRevisionID), IdempotencyKey: strings.TrimSpace(req.ClientRequestID),
		})
		receipt.Case = ReviseEdit
		return receipt, err
	}
	ctx = asOwner(ctx)
	if strings.TrimSpace(doc.TrajectoryID) == "" && kase != ReviseTake {
		// Unbound documents have no lifecycle authority: there is no canonical
		// revision event to carry owner input, and the legacy worker-mailbox
		// path was deleted with the owner-instruction channel.
		return ReviseReceipt{}, refuseOwnerRevision(http.StatusConflict, "document is not lifecycle-bound; owner revisions require a lifecycle Texture document")
	}
	fingerprint := reviseFingerprint(kase, req)
	if replay, found, err := h.reviseReplay(ctx, doc, ownerID, req, fingerprint); found || err != nil {
		replay.Case = kase
		return replay, err
	}
	return h.reviseWithSources(ctx, doc, ownerID, req, kase, fingerprint)
}

// reviseFingerprint hashes what a request asks for, so one request id cannot
// name two different requests.
func reviseFingerprint(kase ReviseCase, req ReviseRequest) string {
	sources := make([]ReviseSource, 0, len(req.Sources))
	for _, s := range req.Sources {
		sources = append(sources, ReviseSource{strings.TrimSpace(s.PublicationVersionID), strings.TrimSpace(s.ManifestHash)})
	}
	raw, _ := json.Marshal(struct {
		Case    ReviseCase     `json:"case"`
		Text    string         `json:"text"`
		Base    string         `json:"base"`
		Head    string         `json:"head"`
		Sources []ReviseSource `json:"sources"`
	}{kase, strings.TrimSpace(req.Text), strings.TrimSpace(req.BaseRevisionID), strings.TrimSpace(req.ExpectedHeadRevisionID), sources})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// reviseReplay answers a request id that already wrote a revision on this
// document: the same request is the same receipt, with no second revision and
// no second wake; another request under the id is a conflict.
func (h *Handler) reviseReplay(ctx context.Context, doc types.Document, ownerID string, req ReviseRequest, fingerprint string) (ReviseReceipt, bool, error) {
	clientRequestID := strings.TrimSpace(req.ClientRequestID)
	if clientRequestID == "" || strings.TrimSpace(doc.TrajectoryID) == "" || strings.TrimSpace(doc.ComputerID) == "" {
		return ReviseReceipt{}, false, nil
	}
	revisionID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join([]string{ownerID, doc.ComputerID, doc.TrajectoryID, clientRequestID}, "\x00"))).String()
	stored, err := h.Store.GetLifecycleRevision(ctx, ownerID, doc.ComputerID, revisionID)
	if err != nil {
		if head, headErr := h.Store.GetLifecycleRevision(ctx, ownerID, doc.ComputerID, doc.CurrentRevisionID); headErr == nil {
			meta := decodeRevisionMetadata(head.Metadata)
			if metadataString(meta, reviseClientIDMetadataKey) == clientRequestID && metadataString(meta, reviseFingerprintMetadataKey) == fingerprint {
				requestID, _ := textureOwnerOccurrenceIdentity(ownerID, doc.ComputerID, doc.DocID, clientRequestID)
				return ReviseReceipt{DocID: doc.DocID, TrajectoryID: head.TrajectoryID, RevisionID: head.RevisionID, RequestID: requestID, Replay: true, Revision: &head}, true, nil
			}
		}
		return ReviseReceipt{}, false, nil
	}
	if stored.DocID != doc.DocID {
		return ReviseReceipt{}, false, refuseOwnerRevision(http.StatusConflict, "client_request_id was used for another request")
	}
	if metadataString(decodeRevisionMetadata(stored.Metadata), reviseFingerprintMetadataKey) != fingerprint {
		return ReviseReceipt{}, false, refuseOwnerRevision(http.StatusConflict, "client_request_id was used for another request")
	}
	requestID, _ := textureOwnerOccurrenceIdentity(ownerID, doc.ComputerID, doc.DocID, clientRequestID)
	return ReviseReceipt{DocID: doc.DocID, TrajectoryID: doc.TrajectoryID, RevisionID: stored.RevisionID, RequestID: requestID, Replay: true, Revision: &stored}, true, nil
}

// ownerRevisionExtra is what a comment or merge adds to the owner revision it
// commits: the request fingerprint, the base the owner responded to and the
// source links.
type ownerRevisionExtra struct {
	Fingerprint    string
	BaseRevisionID string
	SourceLinks    []types.ParentLink
}

// parentLinks are the links the revision's provenance carries: the sources,
// and the base when the request has sources or names a base that is not the
// head. A plain instruction on the head carries none, so its bytes are what
// they always were.
func (e ownerRevisionExtra) parentLinks(headRevisionID string) []types.ParentLink {
	base := strings.TrimSpace(e.BaseRevisionID)
	if len(e.SourceLinks) == 0 && (base == "" || base == headRevisionID) {
		return nil
	}
	if base == "" {
		base = headRevisionID
	}
	links := []types.ParentLink{{Role: types.ParentLinkBase, RevisionID: base}}
	return append(links, e.SourceLinks...)
}

// linksProvenance is the provenance a revision carries when it has parent
// links, nil when it has none. AuthoredAt stays zero: the command digest
// covers provenance and a replay must digest the same.
func linksProvenance(links []types.ParentLink) json.RawMessage {
	if len(links) == 0 {
		return nil
	}
	raw, err := types.Provenance{SchemaVersion: types.ProvenanceSchemaVersion, ParentLinks: links}.CanonicalJSON()
	if err != nil {
		log.Printf("texture api: encode parent links: %v", err)
		return nil
	}
	return raw
}

// reviseBaseRevision checks that base names a revision of this document.
func (h *Handler) reviseBaseRevision(ctx context.Context, doc types.Document, ownerID, base string) error {
	rev, err := h.Store.GetLifecycleRevision(ctx, ownerID, doc.ComputerID, base)
	if err != nil || rev.DocID != doc.DocID {
		return refuseOwnerRevision(http.StatusUnprocessableEntity, "base_revision_id is not a revision of this document")
	}
	return nil
}

// reviseComment is the comment case: the owner's instruction on the head,
// committed as an owner revision that wakes the desk (today's /revise).
func (h *Handler) reviseComment(ctx context.Context, doc types.Document, ownerID string, req ReviseRequest, extra ownerRevisionExtra) (ReviseReceipt, error) {
	base := strings.TrimSpace(req.BaseRevisionID)
	if base != "" && base != doc.CurrentRevisionID {
		if err := h.reviseBaseRevision(ctx, doc, ownerID, base); err != nil {
			return ReviseReceipt{}, err
		}
	}
	extra.BaseRevisionID = base
	resp, err := h.commitLifecycleOwnerRevision(ctx, doc, ownerID, textureAgentRevisionRequest{
		Prompt: req.Text, ClientRequestID: strings.TrimSpace(req.ClientRequestID),
		ExpectedHeadRevisionID: strings.TrimSpace(req.ExpectedHeadRevisionID),
	}, extra)
	if err != nil {
		return ReviseReceipt{}, err
	}
	return ReviseReceipt{
		Case: ReviseComment, DocID: resp.DocID, TrajectoryID: resp.TrajectoryID,
		RevisionID: resp.RevisionID, RequestID: resp.RequestID, Replay: resp.Replay,
	}, nil
}

// reviseWithSources runs the cases that follow the replay check.
func (h *Handler) reviseWithSources(ctx context.Context, doc types.Document, ownerID string, req ReviseRequest, kase ReviseCase, fingerprint string) (ReviseReceipt, error) {
	switch kase {
	case ReviseComment:
		return h.reviseComment(ctx, doc, ownerID, req, ownerRevisionExtra{Fingerprint: fingerprint})
	case ReviseTake:
		receipt, err := h.reviseTake(ctx, doc, ownerID, req, fingerprint)
		if err == nil && !receipt.Replay {
			h.emitRevisionCommitted(ctx, ownerID, doc, receipt.RevisionID, ReviseTake, req.forkedFrom)
		}
		return receipt, err
	case ReviseMerge:
		// Sources are validated before the request revision is written; the
		// desk reads each again at request time, pinned by the hash.
		resolved, err := h.resolveReviseSources(ctx, req.Sources)
		if err != nil {
			return ReviseReceipt{}, err
		}
		receipt, err := h.reviseComment(ctx, doc, ownerID, req, ownerRevisionExtra{Fingerprint: fingerprint, SourceLinks: sourceLinks(resolved)})
		receipt.Case = ReviseMerge
		if err == nil && !receipt.Replay {
			h.emitRevisionCommitted(ctx, ownerID, doc, receipt.RevisionID, ReviseMerge, req.forkedFrom)
		}
		return receipt, err
	default:
		return ReviseReceipt{}, refuseOwnerRevision(http.StatusNotImplemented, "sources are not accepted by this build")
	}
}
