package agentcore

import (
	"context"
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
