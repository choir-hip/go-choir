// Package committer is the registry of committers (modular architecture,
// app-agent desks design F7): a committer applies a desk's turn for one kind
// with that kind's domain invariants. The desk that may hand a committer its
// turn holds the grant commit.<kind>, which is first-party only; the host
// resolves the committer by the kind and checks the grant against the desk's
// registry row, never against the desk's name.
//
// Texture's document revisions are the first kind: the Texture lifecycle
// owner registers its ApplyTextureTurn transaction for texture_revision.
//
// The package imports only types and the standard library, so the runtime
// that holds the registry and the owners that register into it both import
// it without a cycle.
package committer

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// GrantPrefix prefixes a kind to name the grant a desk holds to have a
// registered committer apply its turn for that kind.
const GrantPrefix = "commit."

// Grant is the grant a desk holds for kind: commit.<kind>.
func Grant(kind string) string { return GrantPrefix + strings.TrimSpace(kind) }

// Committer applies a desk's turn for one kind.
type Committer interface {
	// Commit applies bodyJSON, the turn the desk's activation run rec
	// staged. revisionIdentity is the deterministic idempotency identity
	// the caller derives, so a replayed turn replays the same commit rather
	// than minting a second one. It returns a JSON receipt of the commit.
	Commit(ctx context.Context, rec *types.RunRecord, bodyJSON string, revisionIdentity string) (string, error)
}

// Func adapts a function to a Committer.
type Func func(ctx context.Context, rec *types.RunRecord, bodyJSON string, revisionIdentity string) (string, error)

// Commit calls f.
func (f Func) Commit(ctx context.Context, rec *types.RunRecord, bodyJSON string, revisionIdentity string) (string, error) {
	return f(ctx, rec, bodyJSON, revisionIdentity)
}

// Registry holds at most one committer per kind. The zero value is not
// usable; use New.
type Registry struct {
	mu     sync.RWMutex
	byKind map[string]entry
	next   uint64
}

type entry struct {
	c  Committer
	id uint64
}

// New returns an empty registry.
func New() *Registry { return &Registry{byKind: map[string]entry{}} }

// Register binds c as the committer for kind. It refuses an empty kind, a nil
// committer and a second committer for a kind: one kind has one set of
// invariants. dispose unbinds this registration and only this one; it is safe
// to call more than once.
func (r *Registry) Register(kind string, c Committer) (dispose func(), err error) {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return nil, fmt.Errorf("committer: a committer needs a kind")
	}
	if c == nil {
		return nil, fmt.Errorf("committer: kind %s: nil committer", kind)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, taken := r.byKind[kind]; taken {
		return nil, fmt.Errorf("committer: kind %s already has a committer", kind)
	}
	r.next++
	id := r.next
	r.byKind[kind] = entry{c: c, id: id}
	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if e, ok := r.byKind[kind]; ok && e.id == id {
			delete(r.byKind, kind)
		}
	}, nil
}

// Lookup returns the committer registered for kind.
func (r *Registry) Lookup(kind string) (Committer, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.byKind[strings.TrimSpace(kind)]
	return e.c, ok
}
