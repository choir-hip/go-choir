package textureowner

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// The writer gate (MA3.2): every write that reaches the object graph declares
// who writes. Texture's documents, revisions and roots admit the owner's own
// edit path, and Texture's custodian for what the platform does on Texture's
// behalf.

// asOwner declares the owner as the writer: the owner's routes (create,
// revise, continue, the prompt bar) write Texture's kinds as the owner.
func asOwner(ctx context.Context) context.Context {
	return objectgraph.WithWriter(ctx, objectgraph.WriterOwner)
}

// ownerCommitContext is the one place an owner-route lifecycle commit picks the
// writer it declares. The owner writes the revision and the document, which
// admit owner edits; the source entities and refs a take or a fork copies in do
// not, and they are written by their own kinds' registered writer. So a commit
// whose write set carries any is made under that custodian (read from the
// registry for the kinds in the set, not assumed from another kind), and a
// commit with none is the owner's. Every owner-route commit that can carry a
// source graph takes its context from here (TestOwnerRouteCommitsPickTheirWriterInOnePlace).
func ownerCommitContext(ctx context.Context, graph store.TextureSourceGraphWriteSet) context.Context {
	var kinds []objectgraph.ObjectKind
	if len(graph.SourceEntities) > 0 {
		kinds = append(kinds, store.TextureSourceEntityObjectKind)
	}
	if len(graph.SourceRefs) > 0 {
		kinds = append(kinds, store.TextureSourceRefObjectKind)
	}
	for _, kind := range kinds {
		if w, ok := objectgraph.CustodianOf(kind); ok {
			return objectgraph.WithWriter(ctx, w)
		}
	}
	if len(kinds) > 0 {
		return asTextureCustodian(ctx)
	}
	return asOwner(ctx)
}

// asTextureCustodian declares the custodian of Texture's documents as the
// writer, for a host path acting for Texture (the root claim and its master
// document).
func asTextureCustodian(ctx context.Context) context.Context {
	w, ok := objectgraph.CustodianOf("choir.texture_document")
	if !ok {
		return ctx
	}
	return objectgraph.WithWriter(ctx, w)
}

// asRequestingDesk declares the desk that asked for a Texture, so a desk
// other than Texture that opens one reads as a crossing in the counters. A
// caller that is no registered desk, or is Texture itself, declares the
// custodian.
func asRequestingDesk(ctx context.Context, callerProfile string) context.Context {
	name := strings.TrimSpace(callerProfile)
	if c, err := agentprofile.Canonical(name); err == nil {
		name = c
	}
	if w, ok := objectgraph.DeskWriter(name); ok {
		return objectgraph.WithWriter(ctx, w)
	}
	return asTextureCustodian(ctx)
}

// asOwnerRoute declares the owner as the reader of a request an owner route
// serves (E1: every owner-authenticated HTTP read stays). A request that
// already names a reader keeps it: a desk's in-process ProductAPI call reaches
// these routes as that desk (MA3.3, X7).
func asOwnerRoute(r *http.Request) *http.Request {
	ctx := objectgraph.WithReaderIfAbsent(r.Context(), objectgraph.WriterOwner)
	if ctx == r.Context() {
		return r
	}
	return r.WithContext(ctx)
}

// asTextureReader declares the custodian of Texture's documents as the reader,
// for Texture's own code the host runs on its behalf (the controller, the
// occurrence resolver, the root).
func asTextureReader(ctx context.Context) context.Context {
	w, ok := objectgraph.CustodianOf("choir.texture_document")
	if !ok {
		return ctx
	}
	return objectgraph.WithReaderIfAbsent(ctx, w)
}

// listedByDesk reports whether the document list is asked for by a desk: a
// desk's in-process ProductAPI call (management's list route, X7). The owner
// and the kernel list the documents; every desk, Texture's own runs included,
// lists the story-index projection Texture publishes (writeStoryIndexList). A
// Texture run works on one story and has no need of the owner's other
// documents, which a public publish would otherwise carry (2026-11-08).
func (h *Handler) listedByDesk(ctx context.Context) bool {
	reader, ok := objectgraph.ReaderFrom(ctx)
	if !ok {
		return false
	}
	switch reader {
	case objectgraph.WriterOwner, objectgraph.WriterKernel, objectgraph.WriterStore:
		return false
	}
	return true
}

// writeStoryIndexList answers the list route for a desk from the story index
// (MA3.7a, X7): the custodian publishes the index first, and the desk reads
// headlines, ledes and ids, never a document or a revision.
func (h *Handler) writeStoryIndexList(w http.ResponseWriter, r *http.Request, ownerID string, limit int, after *textureDocumentCursor) {
	computerID := ""
	if h.Core != nil {
		computerID = strings.TrimSpace(h.Core.TextureComputerID())
	}
	if h.Store == nil || computerID == "" {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "story index unavailable"})
		return
	}
	if err := h.Store.PublishStoryIndex(r.Context(), ownerID, computerID); err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to publish the story index"})
		return
	}
	var before time.Time
	beforeDoc := ""
	if after != nil {
		before, beforeDoc = after.UpdatedAt, after.DocID
	}
	sums, err := h.Store.ListStoryIndex(r.Context(), ownerID, computerID, before, beforeDoc, limit+1)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list stories"})
		return
	}
	resp := textureListDocsResponse{Documents: make([]textureDocumentResponse, 0, len(sums))}
	if len(sums) > limit {
		sums = sums[:limit]
		last := sums[len(sums)-1]
		resp.NextCursor = (&textureDocumentCursor{UpdatedAt: last.UpdatedAt, DocID: last.DocID}).encode()
	}
	for _, sum := range sums {
		resp.Documents = append(resp.Documents, textureDocumentResponse{
			DocID: sum.DocID, OwnerID: ownerID, Title: sum.Headline, ComputerID: computerID, TrajectoryID: sum.TrajectoryID,
			CurrentRevisionID: sum.RevisionID, CurrentVersionNumber: sum.Version,
			UpdatedAt: sum.UpdatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	writeAPIJSON(w, http.StatusOK, resp)
}
