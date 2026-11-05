package lease

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// First-party typed grants (modular architecture plan MA3.7c, section 8.6).
//
// A grant is a lease on a guarded service API the custodian owns, not on a
// kind. The reader gate never admits a read against it; the custodian's service
// asks for the grant (objectgraph.LeaseAdmitsGrant), and only then reads as the
// custodian, so the read is the custodian's own and nothing wider.

// GrantTextureSearch is the Texture custodian's guarded corpus search: ranked
// refs plus bounded snippets labelled owner_private:texture, never a document
// body (store.TextureSearchUnderLease).
const GrantTextureSearch = "texture.search"

type grantRow struct {
	custodian objectgraph.Writer
	// holders are the writers of the desks whose ceiling may name the grant.
	holders []objectgraph.Writer
}

var grantTable = map[string]grantRow{
	// Until MA4.6 the research desk's own run holds texture.search; MA4.6
	// moves it to the research_texture child and removes research from this row.
	GrantTextureSearch: {custodian: objectgraph.WriterTexture, holders: []objectgraph.Writer{objectgraph.WriterResearch}},
}

func grantCustodian(grant string) (objectgraph.Writer, bool) {
	row, ok := grantTable[grant]
	return row.custodian, ok
}

func grantAllowed(desk, grant string) error {
	if _, ok := grantTable[grant]; !ok {
		return refuse("grant %q is not a first-party grant", grant)
	}
	if w, ok := objectgraph.DeskWriter(desk); ok && GrantHeldBy(grant, w) {
		return nil
	}
	return refuse("desk %q's ceiling does not name grant %q", desk, grant)
}

// GrantHeldBy reports whether the grant table names writer as a holder.
func GrantHeldBy(grant string, w objectgraph.Writer) bool {
	for _, h := range grantTable[grant].holders {
		if h == w {
			return true
		}
	}
	return false
}

// IssueTextureSearch issues the texture.search lease of a research run. The
// Texture custodian's service issues it (the platform's own request, origin
// system, stated as a precommitment like any lease); it is held by the research
// desk for the run and ends with it.
func (s *Service) IssueTextureSearch(ctx context.Context, ownerID, computerID, runID, agentID string) (Record, error) {
	if strings.TrimSpace(runID) == "" {
		return Record{}, fmt.Errorf("lease: texture.search needs the run it is for")
	}
	// The holder is the run's own desk (agent ids are <desk>:<id>), checked
	// against the grant table when the lease is decided.
	desk, _, ok := strings.Cut(agentID, ":")
	if !ok || desk == "" {
		return Record{}, fmt.Errorf("lease: texture.search agent id %q names no desk", agentID)
	}
	p, w := 0.9, 0.2
	return s.Request(ctx, Request{
		OwnerID: ownerID, ComputerID: computerID, Desk: desk, RunID: runID, AgentID: agentID,
		Origin:  types.OriginSystem,
		Scope:   Scope{Grants: []string{GrantTextureSearch}},
		Purpose: "research's corpus search over Texture: ranked refs and bounded snippets, never a document body",
		Fields:  &types.RequestFields{PSuccess: &p, Weight: &w, ImpactClass: types.ImpactI1, Settlement: types.SettlementRecipient},
		grant:   true,
	})
}

// GrantNames lists the first-party typed grants, sorted.
func GrantNames() []string {
	out := make([]string, 0, len(grantTable))
	for name := range grantTable {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
