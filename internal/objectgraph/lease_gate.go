package objectgraph

import (
	"context"
	"log"
	"sync"
)

// The lease half of the reader gate (modular architecture plan MA3.4, R20,
// section 8.5).
//
// Authority has two layers. The desk manifest's ceiling is the most a desk may
// ever request; a custodian-issued lease is what it holds now (internal/lease).
// The reader gate asks the lease authority only when the custody table did not
// already admit the read, so a lease adds a reader and never removes one. The
// authority is attached to the context by trusted in-process code with the
// run's id, from the run record: a cell can neither name a lease nor present
// one.

// LeaseAuthority answers the reader gate. holder is the writer the read
// declares, runID the run the read is made for.
type LeaseAuthority interface {
	// AdmitsKind reports whether a live lease of holder in runID covers kind.
	AdmitsKind(holder Writer, runID string, kind ObjectKind) bool
	// AdmitsCapture reports whether a live lease of holder in runID covers the
	// capture, by its id or by any label its record carries.
	AdmitsCapture(holder Writer, runID, captureID string, labels []string) bool
	// AdmitsGrant reports the id of a live lease of holder in runID that names
	// the first-party grant (a guarded custodian service, not a kind).
	AdmitsGrant(holder Writer, runID, grant string) (leaseID string, ok bool)
	// AdmitsGrantRef is AdmitsGrant for a grant scoped by ref (mail.read): the
	// lease must name the grant and the one ref the use reads (MA4.7).
	AdmitsGrantRef(holder Writer, runID, grant, ref string) (leaseID string, ok bool)
}

type leaseHolderKey struct{}

type leaseHolder struct {
	auth  LeaseAuthority
	runID string
}

// WithLeases attaches the lease authority and the run's id to ctx, for the
// reads made with the returned context. Call it only from trusted in-process
// code that holds the run record.
func WithLeases(ctx context.Context, auth LeaseAuthority, runID string) context.Context {
	if auth == nil {
		return ctx
	}
	return context.WithValue(ctx, leaseHolderKey{}, leaseHolder{auth: auth, runID: runID})
}

// WithoutLeases returns ctx with no lease authority: a read made on it is
// admitted by the custody table alone. A research child shares its parent's run
// id, so it must not inherit the parent's leases (MA3.7c).
func WithoutLeases(ctx context.Context) context.Context {
	return context.WithValue(ctx, leaseHolderKey{}, nil)
}

func leasesFrom(ctx context.Context) (leaseHolder, bool) {
	if ctx == nil {
		return leaseHolder{}, false
	}
	h, ok := ctx.Value(leaseHolderKey{}).(leaseHolder)
	return h, ok
}

// LeaseAdmitsKind reports whether the lease attached to ctx admits holder to
// kind.
func LeaseAdmitsKind(ctx context.Context, holder Writer, kind ObjectKind) bool {
	h, ok := leasesFrom(ctx)
	return ok && h.auth.AdmitsKind(holder, h.runID, kind)
}

// LeaseAdmitsCapture reports whether the lease attached to ctx admits holder
// to a capture with the given id and labels.
func LeaseAdmitsCapture(ctx context.Context, holder Writer, captureID string, labels []string) bool {
	h, ok := leasesFrom(ctx)
	return ok && h.auth.AdmitsCapture(holder, h.runID, captureID, labels)
}

// LeaseAdmitsGrant reports the id of the lease attached to ctx that gives
// holder the grant. The custodian's service calls it before it reads as the
// custodian.
func LeaseAdmitsGrant(ctx context.Context, holder Writer, grant string) (string, bool) {
	h, ok := leasesFrom(ctx)
	if !ok {
		return "", false
	}
	return h.auth.AdmitsGrant(holder, h.runID, grant)
}

// LeaseAdmitsGrantRef reports the id of the lease attached to ctx that gives
// holder the grant for ref. The custodian's service calls it before it reads
// that one ref as the custodian.
func LeaseAdmitsGrantRef(ctx context.Context, holder Writer, grant, ref string) (string, bool) {
	h, ok := leasesFrom(ctx)
	if !ok || ref == "" {
		return "", false
	}
	return h.auth.AdmitsGrantRef(holder, h.runID, grant, ref)
}

// LeaseRunID is the run the lease context was attached for.
func LeaseRunID(ctx context.Context) string {
	h, _ := leasesFrom(ctx)
	return h.runID
}

var (
	leaseUseMu     sync.Mutex
	leaseUseLogged = map[string]bool{}
)

// NoteLeaseRead logs, once per (holder, subject), that a read was served
// against a lease: the frame record of a use (the service counts every one).
func NoteLeaseRead(holder Writer, subject string) {
	key := string(holder) + "|" + subject
	leaseUseMu.Lock()
	first := !leaseUseLogged[key]
	leaseUseLogged[key] = true
	leaseUseMu.Unlock()
	if first {
		log.Printf("objectgraph: lease read: %s read %s against a lease", holder, subject)
	}
}
