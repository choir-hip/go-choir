package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The owner's words reach the engineer verbatim (docs/problems/owner-words-
// restated-away-before-engineering-2026-11-03.md). Texture restates the
// owner's revision in its Cast and management restates the Cast in its
// objective; scope, shot lists, step order, budgets and named tools fell out
// on every arm. The platform now appends the owner revision the Cast chain
// answers to the engineering run's context, read from the tape at every
// render, so no desk writes or edits it. Management's objective stays the
// run's prompt.
const (
	ownerWordsLabel = "Owner's words (verbatim; the operative request)"
	// ownerWordsCarryMax bounds the words carried in the run context; the
	// whole text stays in the Texture request in choir.Records().
	ownerWordsCarryMax = 12 << 10
)

// latestOwnerWords picks the owner revision a Texture Cast answers: the
// newest of the owner directions the runtime attached to its packet (the
// revisions the Cast named and the earlier ones it carried), or, when the
// packet carries none, the newest owner revision of scope's document and
// trajectory written at or before castAt. It says where the words came from.
// ok is false when there are none.
func latestOwnerWords(packet string, castAt time.Time, scope ownerRevisionScope, list func() ([]types.Revision, error)) (castOwnerDirection, string, bool) {
	newest := func(best castOwnerDirection, bestAt time.Time, d castOwnerDirection, at time.Time, found bool) bool {
		return !found || at.After(bestAt) || (at.Equal(bestAt) && d.RevisionID > best.RevisionID)
	}
	var best castOwnerDirection
	var bestAt time.Time
	found := false
	if spec, ok := castPacketObject(packet); ok {
		var attached []castOwnerDirection
		if raw, ok := spec[castOwnerDirectionsKey]; ok && json.Unmarshal(raw, &attached) == nil {
			for _, d := range attached {
				if strings.TrimSpace(d.Text) == "" {
					continue
				}
				at, _ := time.Parse(time.RFC3339Nano, d.At)
				if newest(best, bestAt, d, at, found) {
					best, bestAt, found = d, at, true
				}
			}
		}
	}
	if found {
		return best, "the newest owner revision on Texture's Cast (owner_directions)", true
	}
	if castAt.IsZero() || list == nil {
		return castOwnerDirection{}, "", false
	}
	revisions, err := list()
	if err != nil {
		log.Printf("owner words: list the revisions of %s: %v", scope.DocID, err)
		return castOwnerDirection{}, "", false
	}
	for _, rev := range revisions {
		if rev.AuthorKind != types.AuthorUser || rev.CreatedAt.IsZero() || rev.CreatedAt.After(castAt) {
			continue
		}
		d, err := ownerDirectionFromRevision(strings.TrimSpace(rev.RevisionID), scope, func(string) (types.Revision, error) { return rev, nil })
		if err != nil {
			continue // another trajectory's, or an owner edit without words
		}
		if newest(best, bestAt, d, rev.CreatedAt, found) {
			best, bestAt, found = d, rev.CreatedAt, true
		}
	}
	if !found {
		return castOwnerDirection{}, "", false
	}
	return best, "Texture's Cast carried no owner words; this is the newest owner revision on the document before that Cast", true
}

// ownerWordsBlock renders the owner's words for the run context: the label,
// where they came from, and the text verbatim, cut at ownerWordsCarryMax on a
// rune boundary with a visible marker.
func ownerWordsBlock(d castOwnerDirection, source, textureCastID string) string {
	text := d.Text
	cut := ""
	if len(text) > ownerWordsCarryMax {
		n := ownerWordsCarryMax
		for n > 0 && !utf8.RuneStart(text[n]) {
			n--
		}
		cut = fmt.Sprintf("\n[... truncated: showing %d of %d bytes of the owner's words. The whole text is in the Texture request of record %s in choir.Records() when that Cast carried it; otherwise choir.Report that the owner's words were cut.]", n, len(text), textureCastID)
		text = text[:n]
	}
	var b strings.Builder
	b.WriteString(ownerWordsLabel + ":\n")
	fmt.Fprintf(&b, "Revision %s (%s", d.RevisionID, firstNonEmpty(d.Kind, "direction"))
	if d.At != "" {
		b.WriteString(", written " + d.At)
	}
	b.WriteString("). Source: " + source + ". The platform carries it from the revision; no desk restated it.\n")
	b.WriteString("Management's objective is this round's assignment and the request it serves is these words. Where the objective leaves out scope, steps, their order, budgets, shots or tools named here, follow these words, or report why you did not.\n")
	b.WriteString("<<<owner words\n")
	b.WriteString(text)
	if !strings.HasSuffix(text, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("owner words>>>")
	b.WriteString(cut)
	return b.String()
}

// ownerWordsForRun is the owner's-words block for an engineering assignment
// run, or "" when its chain does not start from a Texture request bound to
// the assignment. The chain is checked as engineeringRequestView checks it:
// management's Cast that opened the assignment, its parent a Texture's Cast to
// that management desk, and the binding's RequestDigest committing to exactly
// that Texture Cast's text.
func (rt *Runtime) ownerWordsForRun(rec *types.RunRecord) string {
	if rt == nil || rt.store == nil || rec == nil || agentProfileForRun(rec) != agentprofile.Engineering {
		return ""
	}
	if metadataStringValue(rec.Metadata, "assignment_id") == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID, computerID := strings.TrimSpace(rec.OwnerID), strings.TrimSpace(rec.ComputerID)
	agentID := strings.TrimSpace(agentIDForRun(rec))
	assignment, err := rt.store.EngineeringAssignmentForInbox(ctx, ownerID, computerID, agentID)
	if err != nil {
		log.Printf("owner words: assignment for %s: %v", agentID, err)
		return ""
	}
	binding := assignment.Binding
	cast, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, binding.ParentControlID)
	if err != nil || cast == nil || !cast.IsCast() || cast.SenderInbox != binding.ParentAgentID || strings.TrimSpace(cast.ParentID) == "" {
		return ""
	}
	parent, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, cast.ParentID)
	if err != nil || !isTextureCastTo(parent, binding.ParentAgentID) || parent.Directive == nil {
		return ""
	}
	digest := parentRequestDigest(parent.SenderInbox, parent.Directive.Objective, parent.Directive.Body)
	if engineeringRequestDigest(strings.TrimSpace(cast.Directive.Objective), cast.ParentID, cast.RecordID, digest) != binding.RequestDigest {
		return ""
	}
	scope := ownerRevisionScope{OwnerID: ownerID, ComputerID: computerID,
		DocID:        strings.TrimPrefix(parent.SenderInbox, agentprofile.Texture+":"),
		TrajectoryID: strings.TrimSpace(binding.TrajectoryID)}
	castAt, _ := time.Parse(time.RFC3339Nano, strings.TrimSpace(parent.Provenance.CommittedAt))
	words, source, ok := latestOwnerWords(parent.Directive.Body, castAt, scope, func() ([]types.Revision, error) {
		return rt.store.ListTextureRevisionsByScopeOG(ctx, ownerID, computerID, scope.DocID, 1<<30)
	})
	if !ok {
		return ""
	}
	return ownerWordsBlock(words, source, parent.RecordID)
}
