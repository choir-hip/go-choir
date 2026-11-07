package lease

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The capsule store copy is a lease (X6, section 8.5). An engineering
// capsule's /computer/state.db used to be a copy of everything; it is now the
// kinds a lease for the assignment names. The default lease names every
// built-in kind except the ones the owner has not decided to hand to an
// engineer: Texture's bodies (the owner's writing), the mail kinds (the
// owner's emails; the Files route's mail directory is carved out beside it),
// and the lease records themselves. A package's kinds are not built-in and are
// never in the default: they are leased by name.

// capsuleWithheld are the built-in kinds the default capsule lease leaves out.
var capsuleWithheld = map[string]bool{
	"choir.texture_document": true,
	"choir.texture_revision": true,
	"choir.lease":            true,
	"choir.trust_anchor":     true,
	"choir.trust_seed":       true,
}

// CapsuleStoreKinds is the default kind set of a capsule's store lease: every
// built-in kind but the withheld ones and any mail kind.
func CapsuleStoreKinds() []string {
	var kinds []string
	for _, reg := range objectgraph.DefaultRegistry().AllKinds() {
		k := string(reg.Kind)
		if capsuleWithheld[k] || strings.Contains(k, "mail") {
			continue
		}
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return kinds
}

// IssueCapsule issues the lease behind an engineering capsule's store copy and
// returns it. The request is the platform's own (origin system) and a
// precommitment like any other: it states that the copy serves the assignment.
// The lease is held by the engineering desk for the assignment's run and
// ends with it. It is not a read lease: engineering's ceiling names no kinds,
// so the reader gate never admits a read against it; it names what the copy
// holds.
func (s *Service) IssueCapsule(ctx context.Context, ownerID, computerID, assignmentID, runID, agentID string) (Record, error) {
	// The holder is the assigned agent's desk (agent ids are <desk>:<id>).
	desk, _, ok := strings.Cut(agentID, ":")
	if !ok || desk == "" {
		return Record{}, fmt.Errorf("lease: capsule agent id %q names no desk", agentID)
	}
	p, w := 0.9, 0.3
	req := Request{
		OwnerID: ownerID, ComputerID: computerID, Desk: desk, RunID: runID, AgentID: agentID,
		Assignment: assignmentID, Origin: types.OriginSystem,
		Scope:   Scope{Kinds: CapsuleStoreKinds()},
		Purpose: "store copy for the engineering capsule of assignment " + assignmentID,
		Fields:  &types.RequestFields{PSuccess: &p, Weight: &w, ImpactClass: types.ImpactI1, Settlement: types.SettlementRecipient},
		// A capsule lease is inside the platform's own bound, not a desk
		// ceiling: see decideCapsule.
		capsule: true,
	}
	return s.Request(ctx, req)
}
