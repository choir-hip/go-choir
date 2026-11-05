package textureowner

import (
	"context"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
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
