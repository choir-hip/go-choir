package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The owner's words reach the engineer verbatim (docs/problems/owner-words-
// restated-away-before-engineering-2026-11-03.md). Texture restates the
// owner's revision in its Cast and management restates the Cast in its
// objective; scope, shot lists, step order, budgets and named tools fell out
// on every arm. The platform now appends the owner revision the Cast chain
// answers to the engineering run's context, read from the data attached to the Cast
// record (X4), so no desk writes or edits it and no render lists the tape. Management's objective stays the
// run's prompt.
const (
	ownerWordsLabel = "Owner's words (verbatim; the operative request)"
	// ownerWordsCarryMax bounds the words carried in the run context; the
	// whole text stays in the Texture request in choir.Records().
	ownerWordsCarryMax = 12 << 10
)

// latestOwnerWords picks the owner revision a Texture Cast answers: the
// newest of the owner directions the runtime attached to its packet (the
// revisions the Cast named and the earlier ones it carried, or, for a Cast
// that named none, the newest owner revision at commit). It reads the Cast
// record only and never lists the tape (X4, MA4.10). It says where the words
// came from. ok is false when the packet carries none.
func latestOwnerWords(packet string) (castOwnerDirection, string, bool) {
	spec, ok := castPacketObject(packet)
	if !ok {
		return castOwnerDirection{}, "", false
	}
	raw, ok := spec[castOwnerDirectionsKey]
	if !ok {
		return castOwnerDirection{}, "", false
	}
	var attached []castOwnerDirection
	if json.Unmarshal(raw, &attached) != nil {
		return castOwnerDirection{}, "", false
	}
	var best castOwnerDirection
	var bestAt time.Time
	found := false
	for _, d := range attached {
		if strings.TrimSpace(d.Text) == "" {
			continue
		}
		at, _ := time.Parse(time.RFC3339Nano, d.At)
		if !found || at.After(bestAt) || (at.Equal(bestAt) && d.RevisionID > best.RevisionID) {
			best, bestAt, found = d, at, true
		}
	}
	if !found {
		return castOwnerDirection{}, "", false
	}
	return best, "the newest owner revision on Texture's Cast (owner_directions)", true
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
	if rt == nil || rt.store == nil || rec == nil || !deskmanifest.IsAssignmentPerformer(agentProfileForRun(rec)) {
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
	words, source, textureCastID, ok := rt.ownerWordsForAssignment(ctx, ownerID, computerID, assignment)
	if !ok {
		return ""
	}
	return ownerWordsBlock(words, source, textureCastID)
}

// ownerWordsForAssignment is the owner revision an engineering assignment's
// Cast chain answers, checked as ownerWordsForRun checks it; the judge's
// per-shot review reads its numbered shot list (selfdev_shot_review.go).
func (rt *Runtime) ownerWordsForAssignment(ctx context.Context, ownerID, computerID string, assignment types.EngineeringAssignment) (castOwnerDirection, string, string, bool) {
	parent, _, _, ok := rt.assignmentCastScope(ctx, ownerID, computerID, assignment)
	if !ok {
		return castOwnerDirection{}, "", "", false
	}
	words, source, ok := latestOwnerWords(parent.Directive.Body)
	return words, source, parent.RecordID, ok
}

func (rt *Runtime) ownerRevisionLister(ctx context.Context, scope ownerRevisionScope) func() ([]types.Revision, error) {
	return func() ([]types.Revision, error) {
		return rt.store.ListTextureRevisionsByScopeOG(ctx, scope.OwnerID, scope.ComputerID, scope.DocID, 1<<30)
	}
}

// assignmentCastScope checks the assignment's Cast chain as ownerWordsForRun
// does and returns Texture's Cast, the document and trajectory scope, and
// the time the Cast committed.
func (rt *Runtime) assignmentCastScope(ctx context.Context, ownerID, computerID string, assignment types.EngineeringAssignment) (*types.CommitmentRecord, ownerRevisionScope, time.Time, bool) {
	binding := assignment.Binding
	cast, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, binding.ParentControlID)
	if err != nil || cast == nil || !cast.IsCast() || cast.SenderInbox != binding.ParentAgentID || strings.TrimSpace(cast.ParentID) == "" {
		return nil, ownerRevisionScope{}, time.Time{}, false
	}
	parent, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, cast.ParentID)
	if err != nil || !isTextureCastTo(parent, binding.ParentAgentID) || parent.Directive == nil {
		return nil, ownerRevisionScope{}, time.Time{}, false
	}
	digest := parentRequestDigest(parent.SenderInbox, parent.Directive.Objective, parent.Directive.Body)
	if engineeringRequestDigest(strings.TrimSpace(cast.Directive.Objective), cast.ParentID, cast.RecordID, digest) != binding.RequestDigest {
		return nil, ownerRevisionScope{}, time.Time{}, false
	}
	scope := ownerRevisionScope{OwnerID: ownerID, ComputerID: computerID,
		DocID:        strings.TrimPrefix(parent.SenderInbox, deskmanifest.DocumentCustodianName()+":"),
		TrajectoryID: strings.TrimSpace(binding.TrajectoryID)}
	castAt, _ := time.Parse(time.RFC3339Nano, strings.TrimSpace(parent.Provenance.CommittedAt))
	return parent, scope, castAt, true
}

// ownerNotesForAssignment is the judge's view of the owner's notes written
// after the assignment's Cast ("" when none or the chain does not check).
func (rt *Runtime) ownerNotesForAssignment(ctx context.Context, ownerID, computerID string, assignment types.EngineeringAssignment) string {
	_, scope, castAt, ok := rt.assignmentCastScope(ctx, ownerID, computerID, assignment)
	if !ok {
		return ""
	}
	return ownerNotesSince(castAt, scope, rt.ownerRevisionLister(ctx, scope))
}

// Failure modes of the judge's owner notes, decided before the code:
//   - ON1 a non-owner revision (Texture's, an agent's, an owner edit with no
//     words) counted as the owner's: the same predicate as latestOwnerWords,
//     ownerDirectionFromRevision, which requires AuthorUser, the computer's
//     owner, the document, the trajectory and words.
//   - ON2 a note from another trajectory or document: the same predicate.
//   - ON3 unbounded size: at most judgeOwnerNotesMax notes, the newest, each
//     cut at judgeOwnerNoteRunes runes.
//   - ON4 a note changes the required shot list: notes never reach
//     requestedShots; they go only into the prompt text the model reads.
//   - ON5 no notes, or no Cast time, or a listing error: "" and the prompt is
//     byte for byte what it was.
//   - ON6 a note at or before the Cast is repeated: only revisions strictly
//     after castAt count; earlier ones are in the Cast's words already.
const (
	judgeOwnerNotesMax  = 5
	judgeOwnerNoteRunes = 1500
	judgeOwnerNotesHead = "Owner notes since the request (newest last; they clarify or amend it):"
)

// ownerNotesSince renders the owner's own revisions of scope's document and
// trajectory written after castAt, oldest to newest, the last
// judgeOwnerNotesMax of them. It reads them as latestOwnerWords does.
func ownerNotesSince(castAt time.Time, scope ownerRevisionScope, list func() ([]types.Revision, error)) string {
	if castAt.IsZero() || list == nil {
		return ""
	}
	revisions, err := list()
	if err != nil {
		log.Printf("owner notes: list the revisions of %s: %v", scope.DocID, err)
		return ""
	}
	type note struct {
		at   time.Time
		id   string
		text string
	}
	var notes []note
	for _, rev := range revisions {
		if rev.AuthorKind != types.AuthorUser || rev.CreatedAt.IsZero() || !rev.CreatedAt.After(castAt) {
			continue
		}
		d, err := ownerDirectionFromRevision(strings.TrimSpace(rev.RevisionID), scope, func(string) (types.Revision, error) { return rev, nil })
		if err != nil || strings.TrimSpace(d.Text) == "" {
			continue
		}
		notes = append(notes, note{at: rev.CreatedAt, id: d.RevisionID, text: d.Text})
	}
	if len(notes) == 0 {
		return ""
	}
	sort.SliceStable(notes, func(i, j int) bool {
		if !notes[i].at.Equal(notes[j].at) {
			return notes[i].at.Before(notes[j].at)
		}
		return notes[i].id < notes[j].id
	})
	if len(notes) > judgeOwnerNotesMax {
		notes = notes[len(notes)-judgeOwnerNotesMax:]
	}
	var b strings.Builder
	b.WriteString(judgeOwnerNotesHead + "\n")
	for _, n := range notes {
		fmt.Fprintf(&b, "- (%s) %s\n", n.at.UTC().Format(time.RFC3339), truncateRunesShot(strings.TrimSpace(n.text), judgeOwnerNoteRunes))
	}
	return b.String()
}
