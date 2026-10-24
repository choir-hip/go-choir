package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/vocabmigrate"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Engineering reads the Texture request its assignment serves
// (docs/problems/claude-delegate-resume-sonnet-2026-10-20.md, "Fix design";
// P5 cause B). Management's Cast to engineering names the Texture Cast it
// serves by id, but engineering cannot read management's inbox, so before
// this the owner's request reached engineering only as far as management's
// one-line objective. The parent's text is now joined into engineering's
// Records() view of the opening Cast, from the tape, at read time, and the
// parent's content digest is folded into the assignment binding's
// RequestDigest so the join is checked against what the Cast committed.
const (
	// engineeringRequestDigestSeed domain-separates the parent-text digest.
	engineeringRequestDigestSeed = "choir:delegated-cast-parent-request:v1"
	// The view cuts the parent's text at these bounds; the digest covers all
	// of it. The parent record stays on the tape, but no in-cell verb reads
	// a record by ref, so what is cut is not readable from the desk. The
	// objective bound equals the body's: a 4 KiB objective cut a 6,338 B
	// request on 2026-10-24
	// (docs/problems/engineering-cast-objective-cut-at-4kib-2026-10-24.md).
	engineeringRequestObjectiveMax = 16 << 10
	engineeringRequestBodyMax      = 16 << 10
)

// isTextureCastTo reports whether rec is a Texture's Cast in the inbox of
// the management desk: the only kind of record an engineering Cast serves.
func isTextureCastTo(rec *types.CommitmentRecord, management string) bool {
	return rec != nil && rec.InboxSeq > 0 && rec.Inbox == management && rec.IsCast() &&
		strings.HasPrefix(rec.SenderInbox, agentprofile.Texture+":")
}

// parentRequestDigest is the content digest of a Texture Cast's request: its
// sender, objective and body in full, length-prefixed so no two texts collide
// across a field boundary.
func parentRequestDigest(from, objective, body string) string {
	var b strings.Builder
	b.WriteString(engineeringRequestDigestSeed)
	for _, part := range []string{from, objective, body} {
		fmt.Fprintf(&b, "\x00%d:%s", len(part), part)
	}
	return objectgraph.SHA256([]byte(b.String()))
}

// engineeringRequestDigest is the binding's RequestDigest: the Cast's own
// objective, parent id and record id, plus the parent's content digest.
func engineeringRequestDigest(objective, parentID, recordID, parentDigest string) string {
	return objectgraph.SHA256([]byte(strings.Join([]string{
		vocabmigrate.IdentitySeedDelegatedCastRequestV1, objective, parentID, recordID, parentDigest,
	}, "\x00")))
}

// legacyEngineeringRequestDigest is what bindings minted before the parent
// was folded in committed to. It is only recognised, never produced.
func legacyEngineeringRequestDigest(objective, parentID, recordID string) string {
	return objectgraph.SHA256([]byte(strings.Join([]string{
		vocabmigrate.IdentitySeedDelegatedCastRequestV1, objective, parentID, recordID,
	}, "\x00")))
}

// truncationMarker says plainly that the rest is not readable from the desk
// (no verb reads a record by ref) and names the route that works: management
// holds the Texture Cast in its inbox and can send the omitted part.
func truncationMarker(total int, kept int, parentRecordID string) string {
	return fmt.Sprintf("\n[... truncated: showing %d of %d bytes of the Texture request (record %s). The rest is not readable from your desk; do not guess it. Ask for it with choir.Ask(\"management\", \"send bytes %d-%d of the request in %s\") before acting on what is cut]", kept, total, parentRecordID, kept, total, parentRecordID)
}

// boundRequestText cuts text to at most max bytes on a rune boundary and
// appends a visible marker naming the parent record when it cut.
func boundRequestText(text string, max int, parentRecordID string) (string, bool) {
	if len(text) <= max {
		return text, false
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut] + truncationMarker(len(text), cut, parentRecordID), true
}

// engineeringRequestView joins the Texture request into engineering's view of
// the Cast that opened its assignment. It shows the request only when the
// parent is a Texture's Cast to the binding's management desk and the
// binding's RequestDigest commits to exactly the text on the tape; otherwise
// it leaves the request out and says why in RequestError. Nothing here is
// process-local: the view is derived from the tape at every read.
func (rt *Runtime) engineeringRequestView(ctx context.Context, ownerID, computerID string, rec types.CommitmentRecord, view *yaegikernel.InboxRecord) {
	inbox := strings.TrimSpace(rec.Inbox)
	if !strings.HasPrefix(inbox, agentprofile.Engineering+":") || !rec.IsCast() || strings.TrimSpace(rec.ParentID) == "" {
		return
	}
	assignment, err := rt.store.EngineeringAssignmentForInbox(ctx, ownerID, computerID, inbox)
	if err != nil {
		view.RequestError = fmt.Sprintf("the assignment for %s could not be read: %v", inbox, err)
		requestIntoBody(view)
		return
	}
	binding := assignment.Binding
	if rec.RecordID != binding.ParentControlID || rec.SenderInbox != binding.ParentAgentID {
		return // only management's Cast that opened this assignment
	}
	// Desks print r.Body, not r.Request: the spec JSON shown as Body carries
	// the request, or why it is missing.
	defer requestIntoBody(view)
	parent, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, rec.ParentID)
	if err != nil || !isTextureCastTo(parent, binding.ParentAgentID) {
		view.RequestError = fmt.Sprintf("the request %s is not a Texture's Cast to %s on the tape", rec.ParentID, binding.ParentAgentID)
		return
	}
	objective := strings.TrimSpace(rec.Directive.Objective)
	digest := parentRequestDigest(parent.SenderInbox, parent.Directive.Objective, parent.Directive.Body)
	if engineeringRequestDigest(objective, rec.ParentID, rec.RecordID, digest) != binding.RequestDigest {
		if legacyEngineeringRequestDigest(objective, rec.ParentID, rec.RecordID) == binding.RequestDigest {
			view.RequestError = "this assignment predates request binding: the Texture request is not bound to it"
		} else {
			view.RequestError = "the Texture request on the tape does not match the digest the assignment committed to"
		}
		return
	}
	request := &yaegikernel.InboxRequest{RecordID: parent.RecordID, From: parent.SenderInbox, Digest: digest}
	var cutObjective, cutBody bool
	request.Objective, cutObjective = boundRequestText(parent.Directive.Objective, engineeringRequestObjectiveMax, parent.RecordID)
	request.Body, cutBody = boundRequestText(parent.Directive.Body, engineeringRequestBodyMax, parent.RecordID)
	request.Truncated = cutObjective || cutBody
	view.Request = request
}

// requestIntoBody folds view.Request (or view.RequestError) into the spec JSON
// the desk reads as Body, keeping the spec's own keys. A Body that is not a
// JSON object is kept under "spec".
func requestIntoBody(view *yaegikernel.InboxRecord) {
	if view.Request == nil && view.RequestError == "" {
		return
	}
	spec := map[string]any{}
	if raw := strings.TrimSpace(view.Body); raw != "" {
		if err := json.Unmarshal([]byte(raw), &spec); err != nil || spec == nil {
			spec = map[string]any{"spec": view.Body}
		}
	}
	if r := view.Request; r != nil {
		spec["request"] = map[string]any{
			"record_id": r.RecordID, "from": r.From, "objective": r.Objective, "body": r.Body, "truncated": r.Truncated,
		}
	} else {
		spec["request_error"] = view.RequestError
	}
	if out, err := json.Marshal(spec); err == nil {
		view.Body = string(out)
	}
}
