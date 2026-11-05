package agentcore

import (
	"context"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// withDeskWriter declares, for the object graph's writer gate, that desk is
// the writer of every batch made on the returned context (MA3.2). The reducer
// passes the desk of the committing run's record, so the writer is the run's
// desk and never the cell's claim; a host loop that writes a desk's kinds on
// its behalf (the intake loops, as management) passes that desk's profile
// constant. The writer is resolved through the desk manifest. A name that
// is no registered desk declares nothing, and the gate counts the write as
// undeclared.
func withDeskWriter(ctx context.Context, desk string) context.Context {
	w, ok := objectgraph.DeskWriter(canonicalDeskName(strings.TrimSpace(desk)))
	if !ok {
		return ctx
	}
	return objectgraph.WithWriter(ctx, w)
}

// withKindCustodian declares, for a host loop that writes kind on its
// custodian's behalf, the writer the registry holds for kind. A kind the
// registry does not know declares nothing, and the gate counts it undeclared.
func withKindCustodian(ctx context.Context, kind objectgraph.ObjectKind) context.Context {
	w, ok := objectgraph.CustodianOf(kind)
	if !ok {
		// Under test tracking a renamed or mistyped kind fails loudly instead
		// of silently declaring nothing.
		objectgraph.FailOnUnknownCustodian(kind)
		return ctx
	}
	return objectgraph.WithWriter(ctx, w)
}

// withKernelWriter declares the kernel as the writer, for the platform's own
// bookkeeping that belongs to no desk (the crash-boot closure).
func withKernelWriter(ctx context.Context) context.Context {
	return objectgraph.WithWriter(ctx, objectgraph.WriterKernel)
}

// withDeskReader declares, for the object graph's reader gate, that desk is
// the reader of every read made on the returned context (MA3.3). The run's
// execution context passes the desk of the run record, never the cell's
// claim. A name that is no registered desk declares nothing, and the gate
// counts the read as undeclared.
func withDeskReader(ctx context.Context, desk string) context.Context {
	w, ok := objectgraph.DeskWriter(canonicalDeskName(strings.TrimSpace(desk)))
	if !ok {
		return ctx
	}
	return objectgraph.WithReader(ctx, w)
}

// withKindReader declares, for a host loop that reads kind on its custodian's
// behalf, the custodian the registry holds for kind as the reader. It declares
// only where the context names no actor yet.
func withKindReader(ctx context.Context, kind objectgraph.ObjectKind) context.Context {
	w, ok := objectgraph.CustodianOf(kind)
	if !ok {
		objectgraph.FailOnUnknownCustodian(kind)
		return ctx
	}
	return objectgraph.WithReaderIfAbsent(ctx, w)
}

// withKernelReader declares the kernel as the reader, for the platform's own
// bookkeeping that belongs to no desk (boot recovery, warm-up scans).
func withKernelReader(ctx context.Context) context.Context {
	return objectgraph.WithReaderIfAbsent(ctx, objectgraph.WriterKernel)
}

// withOwnerRoute declares the owner as the reader of a request an owner route
// serves (E1), unless the request already names a desk: a desk's in-process
// ProductAPI call reaches the routes as that desk (X7).
func withOwnerRoute(r *http.Request) *http.Request {
	ctx := objectgraph.WithReaderIfAbsent(r.Context(), objectgraph.WriterOwner)
	if ctx == r.Context() {
		return r
	}
	return r.WithContext(ctx)
}

// OwnerRouteReader declares the owner as the reader of every request the
// guest's HTTP routes serve (E1: every owner-authenticated HTTP read stays;
// MA3.3), unless the request already names a desk. The autoputer wraps its
// server with it, so a route's store reads need no declaration of their own.
func OwnerRouteReader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, withOwnerRoute(r))
	})
}
