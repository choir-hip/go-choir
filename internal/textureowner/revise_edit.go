package textureowner

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// ownerBodyEdit is one owner-authored revision that carries a body: the
// editor's save (the edit case) and a take (a published version's body). Links
// are the parent links the revision's provenance carries; NoWake commits it
// without waking the desk, as a take must (no model call).
type ownerBodyEdit struct {
	Body             ReviseBody
	ParentRevisionID string
	IdempotencyKey   string
	Links            []types.ParentLink
	Metadata         map[string]any
	NoWake           bool
	// Verbatim commits Body.Content exactly: the editor's markdown stabilizer
	// repairs text the owner typed, and a take's text is a pinned published
	// body, which the stabilizer must never rewrite.
	Verbatim bool
}

// commitOwnerBodyEdit is the canonical write of an owner-authored body
// revision on an existing document: the code that was the editor save's
// handler, unchanged in what it checks and writes, now reached through
// reviseDocument. Refusals are returned as ownerRevisionRefusal.
func (h *Handler) commitOwnerBodyEdit(ctx context.Context, doc types.Document, ownerID string, in ownerBodyEdit) (ReviseReceipt, error) {
	req := in.Body
	docID := doc.DocID
	lifecycleBound := strings.TrimSpace(doc.TrajectoryID) != ""
	if lifecycleBound && (strings.TrimSpace(in.IdempotencyKey) == "" || req.ExpectedLifecycleVersion <= 0 || strings.TrimSpace(in.ParentRevisionID) == "") {
		return ReviseReceipt{}, refuseOwnerRevision(http.StatusBadRequest, "idempotency_key, expected_lifecycle_version, and parent_revision_id are required for lifecycle revisions")
	}

	now := time.Now().UTC()
	if !lifecycleBound {
		if err := h.canonicalizeAliasedTextureDocumentTitle(asOwner(ctx), ownerID, &doc, now); err != nil {
			log.Printf("texture api: canonicalize aliased document title: %v", err)
			return ReviseReceipt{}, refuseOwnerRevision(http.StatusInternalServerError, "failed to canonicalize document title")
		}
	}

	// If parent_revision_id is not specified, use the document's current head.
	parentID := in.ParentRevisionID
	if parentID == "" {
		parentID = doc.CurrentRevisionID
	}

	citations := req.Citations
	if citations == nil {
		citations = json.RawMessage("[]")
	}
	metadata := req.Metadata
	if metadata == nil {
		metadata = json.RawMessage("{}")
	}
	if len(in.Metadata) > 0 {
		metadata = mergeTextureRevisionMetadata(metadata, in.Metadata)
	}
	content := req.Content
	var parentRev types.Revision
	var hasParentRev bool
	if strings.TrimSpace(parentID) != "" {
		if rev, err := h.getTextureRevision(ctx, ownerID, parentID); err == nil {
			parentRev = rev
			hasParentRev = true
			var stabilized bool
			if !in.Verbatim {
				content, stabilized = stabilizeTextureUserMarkdownStructures(parentRev.Content, content)
			}
			if stabilized {
				metadata = mergeTextureRevisionMetadata(metadata, map[string]any{
					"texture_structure_stabilized":        true,
					"texture_structure_stabilized_reason": "preserved_parent_markdown_table_after_collapsed_draft",
				})
			}
		} else {
			log.Printf("texture api: load parent revision for structure stabilization %s: %v", parentID, err)
		}
	}
	// A bound commit, settled-unbound included, is on doc.TrajectoryID, so its label is too.
	recordTrajectoryID := ""
	if lifecycleBound {
		recordTrajectoryID = doc.TrajectoryID
	}
	metadata = carryForwardTextureMetadataForRecord(metadata, parentRev.Metadata, recordTrajectoryID)
	// Only Texture's evidence path writes an embed (station TE, design E7):
	// the owner may keep, delete or move one, never add or alter one.
	var parentBody json.RawMessage
	if hasParentRev {
		parentBody = parentRev.BodyDoc
	}
	if added := newEmbeds(parentBody, req.BodyDoc); len(added) > 0 {
		return ReviseReceipt{}, refuseOwnerRevision(http.StatusBadRequest, "embeds are written only by Texture (capsule_evidence from a frozen candidate's bundle); this revision adds or alters "+strings.Join(added, ", ")+"; keep, delete or move an embed, do not add or change one")
	}
	if !lifecycleBound {
		if canonicalPath, err := h.ensureCanonicalTextureProjectionPath(ctx, ownerID, doc); err == nil && canonicalPath != "" {
			metadata = mergeTextureRevisionMetadata(metadata, map[string]any{
				canonicalTextureSourcePathMetadataKey: canonicalPath,
			})
		} else if err != nil {
			log.Printf("texture api: ensure canonical texture projection path: %v", err)
		}
	}

	revisionID := uuid.New().String()
	if lifecycleBound {
		revisionID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join([]string{ownerID, doc.ComputerID, doc.TrajectoryID, strings.TrimSpace(in.IdempotencyKey)}, "\x00"))).String()
	}
	rev := types.Revision{
		RevisionID:       revisionID,
		DocID:            docID,
		OwnerID:          ownerID,
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      ownerID,
		Content:          content,
		BodyDoc:          req.BodyDoc,
		SourceEntities:   req.SourceEntities,
		Citations:        citations,
		Metadata:         metadata,
		Provenance:       linksProvenance(in.Links),
		ParentRevisionID: parentID,
		CreatedAt:        now,
	}

	if lifecycleBound {
		rev.ComputerID, rev.TrajectoryID = doc.ComputerID, doc.TrajectoryID
		snapshot, snapshotErr := h.Store.GetLifecycleSnapshot(ctx, ownerID, doc.ComputerID, doc.TrajectoryID)
		if snapshotErr != nil {
			log.Printf("texture api: load lifecycle for revision: %v", snapshotErr)
			return ReviseReceipt{}, refuseOwnerRevision(http.StatusInternalServerError, "failed to load lifecycle revision authority")
		}
		targetAgentID := currentTextureAgentID(doc.DocID)
		targetWorkItemID := ""
		for _, work := range snapshot.WorkItems {
			if work.Status == types.WorkItemOpen && work.AssignedAgentID == targetAgentID && work.AuthorityProfile == deskmanifest.DocumentCustodianName() {
				if targetWorkItemID != "" {
					return ReviseReceipt{}, refuseOwnerRevision(http.StatusConflict, "lifecycle has multiple open Texture target work items")
				}
				targetWorkItemID = work.WorkItemID
			}
		}
		if snapshot.Trajectory.Status == types.TrajectoryLive && targetWorkItemID == "" {
			return ReviseReceipt{}, refuseOwnerRevision(http.StatusConflict, "lifecycle has no open Texture target work item")
		}
		requestID, _ := textureOwnerOccurrenceIdentity(ownerID, doc.ComputerID, doc.DocID, strings.TrimSpace(in.IdempotencyKey))
		command := types.CommitLifecycleArtifactHeadRequest{
			OwnerID: ownerID, ComputerID: doc.ComputerID,
			CommandID:    "public-head:" + strings.TrimSpace(in.IdempotencyKey),
			TrajectoryID: doc.TrajectoryID, ExpectedLifecycleVersion: req.ExpectedLifecycleVersion,
			ExpectedHeadRevisionID: parentID, Unbound: snapshot.Trajectory.Status != types.TrajectoryLive, Revision: rev,
		}
		graph, graphErr := textureToolSourceGraphWriteSet(rev, materializedTextureEdit{BodyDoc: req.BodyDoc, SourceEntities: req.SourceEntities}, &types.RunRecord{RunID: requestID, OwnerID: ownerID, ComputerID: doc.ComputerID})
		if graphErr != nil {
			log.Printf("texture api: lifecycle revision source graph: %v", graphErr)
			return ReviseReceipt{}, refuseOwnerRevision(http.StatusBadRequest, "invalid lifecycle revision source graph")
		}
		commandDigest, digestErr := store.ComputeCommitLifecycleArtifactHeadWithSourceGraphDigest(command, graph)
		if digestErr != nil {
			log.Printf("texture api: digest lifecycle revision command: %v", digestErr)
			return ReviseReceipt{}, refuseOwnerRevision(http.StatusBadRequest, "invalid lifecycle revision payload")
		}
		command.CommandDigest = commandDigest
		// The owner writes the revision, but the source entities and refs it
		// copies in are Texture-custodian kinds with no owner edits: the batch
		// is committed by the kind's registered writer, as a desk revision is.
		commitCtx := asOwner(ctx)
		if len(graph.SourceEntities) > 0 || len(graph.SourceRefs) > 0 {
			commitCtx = asTextureCustodian(ctx)
		}
		result, commitErr := h.Store.CommitLifecycleArtifactHeadWithSourceGraph(commitCtx, command, graph)
		if commitErr != nil {
			if errors.Is(commitErr, store.ErrConcurrentStateChange) || errors.Is(commitErr, store.ErrLifecycleCommandConflict) || errors.Is(commitErr, store.ErrLifecycleInvalidTransition) {
				return ReviseReceipt{}, refuseOwnerRevision(http.StatusConflict, commitErr.Error())
			}
			log.Printf("texture api: commit lifecycle revision: %v", commitErr)
			return ReviseReceipt{}, refuseCommit(commitErr, "failed to commit lifecycle revision")
		}
		if result.Revision == nil {
			return ReviseReceipt{}, refuseOwnerRevision(http.StatusInternalServerError, "lifecycle revision result unavailable")
		}
		if !result.Replay {
			h.recordTextureAudit(ctx, "revision_committed", ownerID, doc.ComputerID, doc.TrajectoryID, doc.DocID, result.Revision.RevisionID, command.CommandID, command.CommandDigest, result.Trajectory.LifecycleVersion)
			h.emitTextureDocumentRevisionEvent(ctx, ownerID, *result.Revision)
			if !command.Unbound && !in.NoWake {
				h.dispatchTextureRevisionWake(ownerID, doc.ComputerID, doc.TrajectoryID, *result.Revision, requestID, result.Events, h.lifecycleDocDeskProfile(ctx, ownerID, doc))
			}
		}
		return ReviseReceipt{DocID: doc.DocID, TrajectoryID: doc.TrajectoryID, RevisionID: result.Revision.RevisionID, RequestID: requestID, Replay: result.Replay, Revision: result.Revision}, nil
	}

	if err := h.Store.CreateRevision(asOwner(ctx), rev); err != nil {
		log.Printf("texture api: create revision: %v", err)
		if errors.Is(err, store.ErrStaleDocumentHead) {
			if req.AllowRebase && parentID != "" && len(strings.TrimSpace(string(req.BodyDoc))) == 0 {
				rebaseReq := textureCreateRevisionRequest{Content: content}
				rebased, rebaseErr := h.createRebasedUserRevision(ctx, docID, ownerID, rebaseReq, parentID, citations, metadata, now)
				if rebaseErr == nil {
					auditComputerID := firstNonEmpty(doc.ComputerID, h.Core.TextureComputerID())
					auditDigest := textureAuditDigest("revision_committed", ownerID, auditComputerID, docID, rebased.RevisionID, rebased.RevisionHash)
					h.recordTextureAudit(ctx, "revision_committed", ownerID, auditComputerID, rebased.TrajectoryID, docID, rebased.RevisionID, "texture-revision:"+docID+":"+rebased.RevisionID, auditDigest, 0)
					h.emitTextureDocumentRevisionEvent(ctx, ownerID, rebased)
					return ReviseReceipt{DocID: docID, RevisionID: rebased.RevisionID, Revision: &rebased}, nil
				}
				log.Printf("texture api: rebase stale user revision: %v", rebaseErr)
			}
			return ReviseReceipt{}, refuseOwnerRevision(http.StatusConflict, "document head changed; reload the latest version before saving")
		}
		if errors.Is(err, store.ErrInvalidTextureRevision) {
			return ReviseReceipt{}, refuseOwnerRevision(http.StatusBadRequest, err.Error())
		}
		return ReviseReceipt{}, refuseOwnerRevision(http.StatusInternalServerError, "failed to create revision")
	}
	storedRev, err := h.getTextureRevision(ctx, ownerID, rev.RevisionID)
	if err != nil {
		log.Printf("texture api: load created revision: %v", err)
		return ReviseReceipt{}, refuseOwnerRevision(http.StatusInternalServerError, "failed to load created revision")
	}
	auditComputerID := firstNonEmpty(doc.ComputerID, h.Core.TextureComputerID())
	auditDigest := textureAuditDigest("revision_committed", ownerID, auditComputerID, docID, storedRev.RevisionID, storedRev.RevisionHash)
	h.recordTextureAudit(ctx, "revision_committed", ownerID, auditComputerID, storedRev.TrajectoryID, docID, storedRev.RevisionID, "texture-revision:"+docID+":"+storedRev.RevisionID, auditDigest, 0)
	h.emitTextureDocumentRevisionEvent(ctx, ownerID, storedRev)
	return ReviseReceipt{DocID: docID, TrajectoryID: storedRev.TrajectoryID, RevisionID: storedRev.RevisionID, Revision: &storedRev}, nil
}
