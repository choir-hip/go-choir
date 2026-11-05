package lease

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
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

// GrantTextureBrief is the Texture custodian's disclosure grant to research
// for one trajectory run: the brief a Texture run Casts (or Asks) research
// carries the owner-private labels of that run's documents, and the grant
// admits exactly those labels at the desk-message sink, for messages from
// that run only. It opens no kind and no service read: research reads nothing
// it could not read before. The host mints it (IssueTextureBrief) when the
// Texture run messages research; a desk never asks for it.
const GrantTextureBrief = "texture.brief"

// GrantMailRead is the Mail custodian's guarded read of one email by ref
// (MA4.7): the Mail desk issues it per task, for the refs it names, and its
// service (internal/maildesk) reads maild as the custodian for exactly those.
const GrantMailRead = "mail.read"

// WriterMail is the Mail desk's writer: the custodian of mail.read.
var WriterMail = objectgraph.Writer("desk:" + deskmanifest.Mail)

type grantRow struct {
	custodian objectgraph.Writer
	// holders are the writers of the desks the table names as holders.
	holders []objectgraph.Writer
	// labels are the owner-private labels a disclosure grant admits at the
	// desk-message sink (AdmitsMessageLabel). A grant with none admits no
	// message.
	labels []string
	// ceiling admits any desk whose manifest ceiling names the grant
	// (deskmanifest.Desk.LeaseGrants): install review covers it.
	ceiling bool
	// byRef scopes the grant by ref (Scope.Refs): a lease serves the refs it
	// names and no other.
	byRef bool
}

var grantTable = map[string]grantRow{
	// Until MA4.6 the research desk's own run holds texture.search; MA4.6
	// moves it to the research_texture child and removes research from this row.
	GrantTextureSearch: {custodian: objectgraph.WriterTexture, holders: []objectgraph.Writer{objectgraph.WriterResearch}},
	// The labels of the data Texture is the custodian of: a document the
	// Texture frame showed (owner_private:document) and the custodian's own
	// search snippets (owner_private:texture, store.textureSearchLabel).
	GrantTextureBrief: {
		custodian: objectgraph.WriterTexture, holders: []objectgraph.Writer{objectgraph.WriterResearch},
		labels: []string{types.OwnerPrivateLabel("document"), types.OwnerPrivateLabel(strings.TrimPrefix(string(objectgraph.WriterTexture), "desk:"))},
	},
	GrantMailRead: {custodian: WriterMail, ceiling: true, byRef: true},
}

// grantAdmitsLabel reports whether the grant table admits label for grant.
func grantAdmitsLabel(grant, label string) bool {
	for _, l := range grantTable[grant].labels {
		if l == label {
			return true
		}
	}
	return false
}

func grantByRef(grant string) bool { return grantTable[grant].byRef }

func grantCustodian(grant string) (objectgraph.Writer, bool) {
	row, ok := grantTable[grant]
	return row.custodian, ok
}

func grantAllowed(desk deskmanifest.Desk, grant string) error {
	row, ok := grantTable[grant]
	if !ok {
		return refuse("grant %q is not a first-party grant", grant)
	}
	if w, ok := objectgraph.DeskWriter(desk.Name); ok && GrantHeldBy(grant, w) {
		return nil
	}
	if row.ceiling {
		for _, g := range desk.LeaseGrants {
			if g == grant {
				return nil
			}
		}
	}
	return refuse("desk %q's ceiling does not name grant %q", desk.Name, grant)
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

// IssueTextureBrief issues the texture.brief grant of a Texture run that
// messages research. The caller is the host's reducer, never a cell: sender
// is the desk of the run record that stages the message, runID that run, and
// holder the desk the message is addressed to. The lease is refused unless the
// sender is the grant's custodian (Texture) and the holder is a desk the grant
// table names, so no other desk can mint it, for itself or for anyone. It is
// bound to the custodian's run (HolderRunID is the run whose messages it
// admits), ends with that run (the store's terminal observer) or at
// MaxDuration, whichever is first, and a crash ends it at boot. A live lease
// of the same holder and run is returned as is.
func (s *Service) IssueTextureBrief(ctx context.Context, ownerID, computerID, sender, runID, agentID, holder string) (Record, error) {
	if strings.TrimSpace(runID) == "" {
		return Record{}, refuse("texture.brief needs the custodian run it is for")
	}
	senderWriter, ok := objectgraph.DeskWriter(strings.TrimSpace(sender))
	if !ok || senderWriter != grantTable[GrantTextureBrief].custodian {
		return Record{}, refuse("texture.brief is issued by its custodian %s, not by %q", grantTable[GrantTextureBrief].custodian, sender)
	}
	holderDesk, ok := lookupDesk(holder)
	if !ok {
		return Record{}, refuse("desk %q is not registered", holder)
	}
	if err := grantAllowed(holderDesk, GrantTextureBrief); err != nil {
		return Record{}, err
	}
	holderWriter, _ := objectgraph.DeskWriter(holderDesk.Name)
	if r, ok := s.liveGrant(holderWriter, runID, GrantTextureBrief); ok {
		return r, nil
	}
	p, w := 0.9, 0.2
	return s.Request(ctx, Request{
		OwnerID: ownerID, ComputerID: computerID, Desk: holderDesk.Name, RunID: runID, AgentID: agentID,
		Origin:   types.OriginSystem,
		Scope:    Scope{Grants: []string{GrantTextureBrief}},
		Purpose:  "Texture's brief to " + holderDesk.Name + " from run " + runID + " carries that run's documents; the custodian admits the message, and nothing is opened to read",
		Duration: MaxDuration,
		Fields:   &types.RequestFields{PSuccess: &p, Weight: &w, ImpactClass: types.ImpactI1, Settlement: types.SettlementRecipient},
		grant:    true,
	})
}

// liveGrant returns a live lease of holder in runID that names grant, without
// counting a use.
func (s *Service) liveGrant(holder objectgraph.Writer, runID, grant string) (Record, bool) {
	for _, r := range s.liveFor(holder, runID) {
		for _, g := range r.Scope.Grants {
			if g == grant {
				return *r, true
			}
		}
	}
	return Record{}, false
}

// AdmitsMessageLabel reports whether holder is admitted for label on a
// message from runID: a live disclosure grant of holder, bound to that run,
// whose grant table row names the label. The grant table and the holder's
// registration are re-checked on every use, as the ceiling is for kinds.
func (s *Service) AdmitsMessageLabel(holder objectgraph.Writer, runID, label string) bool {
	if strings.TrimSpace(runID) == "" {
		return false
	}
	for _, r := range s.liveFor(holder, runID) {
		desk, ok := deskOf(r)
		if !ok {
			continue
		}
		for _, g := range r.Scope.Grants {
			if grantAdmitsLabel(g, label) && grantAllowed(desk, g) == nil {
				s.noteUse(r.LeaseID)
				return true
			}
		}
	}
	return false
}

// IssueGrant is a custodian's answer to a desk's request for a first-party
// grant: the request is a scored precommitment addressed to the custodian, the
// desk's ceiling must name the grant, and the custodian's rule sees the live
// leases the run already holds. custodian must be the grant's custodian in the
// table; the caller is the custodian's own first-party service.
func (s *Service) IssueGrant(ctx context.Context, custodian objectgraph.Writer, req Request, rule func(held []Record) error) (Record, error) {
	if len(req.Scope.Grants) == 0 {
		return Record{}, refuse("a grant request names its grant")
	}
	for _, g := range req.Scope.Grants {
		c, ok := grantCustodian(g)
		if !ok {
			return Record{}, refuse("grant %q is not a first-party grant", g)
		}
		if c != custodian {
			return Record{}, refuse("grant %q is issued by %s, not %s", g, c, custodian)
		}
	}
	req.grant = true
	req.addressee = string(custodian)
	req.rule = rule
	s.issueMu.Lock()
	defer s.issueMu.Unlock()
	return s.Request(ctx, req)
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
