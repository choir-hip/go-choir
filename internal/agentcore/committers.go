package agentcore

import (
	"context"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/committer"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// TextureRevisionKind is the kind whose turns the Texture lifecycle owner
// commits through ApplyTextureTurn. A desk hands it a turn under the grant
// commit.texture_revision (deskmanifest), which only the Texture system desk
// holds.
const TextureRevisionKind = "texture_revision"

// Committers is the runtime's committer registry (app-agent desks design
// F7). autoputer registers each kind's committer at boot; the reducer and the
// activation's end resolve one by kind.
func (rt *Runtime) Committers() *committer.Registry {
	rt.committersOnce.Do(func() { rt.committers = committer.New() })
	return rt.committers
}

// deskCommits reports whether the desk registered as role holds the grant to
// have the committer for kind apply its turns. An unknown desk holds nothing.
func deskCommits(role, kind string) bool {
	return deskmanifest.Holds(strings.TrimSpace(role), committer.Grant(kind))
}

// commitDeskTurn applies a desk's turn for kind through the registered
// committer. It refuses, committing nothing, when the desk lacks commit.<kind>
// or no committer is registered for the kind.
func (rt *Runtime) commitDeskTurn(ctx context.Context, role, kind string, rec *types.RunRecord, bodyJSON, revisionIdentity string) (string, error) {
	if !deskCommits(role, kind) {
		return "", fmt.Errorf("reduce: desk %q does not hold %s; its %s turn is not committed", strings.TrimSpace(role), committer.Grant(kind), kind)
	}
	c, ok := rt.Committers().Lookup(kind)
	if !ok {
		return "", fmt.Errorf("reduce: no committer registered for kind %s", kind)
	}
	return c.Commit(ctx, rec, bodyJSON, revisionIdentity)
}
