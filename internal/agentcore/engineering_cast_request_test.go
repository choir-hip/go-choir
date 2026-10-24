package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Engineering reads the Texture request (docs/problems/
// claude-delegate-resume-sonnet-2026-10-20.md, "Fix design"). Ways this could
// fail, written first: the request is lost on a fresh runtime (process-local
// state); a legitimate replay computes a different digest; an oversized
// parent floods the view or is cut silently; a parent that is not a Texture
// Cast is shown anyway; a binding minted before the fix shows unbound text; a
// later Cast carries the request; the request leaks into the run objective.

type requestFixture struct {
	rt   *Runtime
	s    *store.Store
	seed store.EngineeringAssignmentSeed
}

func newRequestFixture(t *testing.T) requestFixture {
	t.Helper()
	rt, s := testRuntime(t)
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "user-request", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return requestFixture{rt: rt, s: s, seed: seed}
}

// textureCast commits a Texture Cast to management from texture:<key> with
// the given text and returns it as stored.
func (f requestFixture) textureCast(t *testing.T, sender, key, objective, body string) types.CommitmentRecord {
	t.Helper()
	rec := store.TextureCastRecord(key+":Cast", sender, f.seed.ParentAgentID, objective)
	rec.Directive.Body = body
	commit := types.CommitInboxRecordsRequest{OwnerID: f.seed.OwnerID, ComputerID: f.seed.ComputerID, CommandID: "seed-" + key, Records: []types.CommitmentRecord{rec}}
	commit.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(commit)
	if _, err := f.s.CommitInboxRecords(context.Background(), commit); err != nil {
		t.Fatalf("seed %s: %v", key, err)
	}
	got, err := f.s.GetCommitmentRecord(context.Background(), f.seed.OwnerID, f.seed.ComputerID, rec.RecordID)
	if err != nil {
		t.Fatal(err)
	}
	return *got
}

// open opens assignment 0 whose management Cast names parent. digest picks
// the binding's RequestDigest from the opening Cast's (objective, parent id,
// record id).
func (f requestFixture) open(t *testing.T, parent *types.CommitmentRecord, capability string, digest func(objective, parentID, recordID string) string) string {
	t.Helper()
	open := store.SeedEngineeringOpenRequest(f.seed, 0, capability)
	cast := &open.Records.Records[0]
	cast.ParentID = parent.RecordID
	open.Records.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(open.Records)
	open.Binding.ParentConversationID, open.Binding.ParentRound = parent.ConversationID, parent.Round
	open.Binding.RequestDigest = digest(strings.TrimSpace(cast.Directive.Objective), cast.ParentID, cast.RecordID)
	open.CommandDigest, _ = store.ComputeOpenEngineeringAssignmentDigest(open)
	if _, err := f.s.OpenEngineeringAssignment(context.Background(), open); err != nil {
		t.Fatalf("open: %v", err)
	}
	return open.Binding.AssignedAgentID
}

func boundDigest(parent types.CommitmentRecord) func(string, string, string) string {
	return func(objective, parentID, recordID string) string {
		return engineeringRequestDigest(objective, parentID, recordID,
			parentRequestDigest(parent.SenderInbox, parent.Directive.Objective, parent.Directive.Body))
	}
}

func (f requestFixture) view(t *testing.T, rt *Runtime, inbox string) []yaegikernel.InboxRecord {
	t.Helper()
	cr, err := rt.loadCellRecords(context.Background(), f.seed.OwnerID, f.seed.ComputerID, inbox, "")
	if err != nil {
		t.Fatalf("loadCellRecords: %v", err)
	}
	out := make([]yaegikernel.InboxRecord, 0, len(cr.view))
	for _, v := range cr.view {
		out = append(out, v)
	}
	return out
}

func TestEngineeringRequestDigestBindsParentText(t *testing.T) {
	base := parentRequestDigest("texture:doc-a", "ship the page", `{"summary":"x"}`)
	if base != parentRequestDigest("texture:doc-a", "ship the page", `{"summary":"x"}`) {
		t.Fatal("same parent text gave different digests: a legitimate replay would be refused")
	}
	for name, other := range map[string]string{
		"from":      parentRequestDigest("texture:doc-b", "ship the page", `{"summary":"x"}`),
		"objective": parentRequestDigest("texture:doc-a", "ship another page", `{"summary":"x"}`),
		"body":      parentRequestDigest("texture:doc-a", "ship the page", `{"summary":"y"}`),
		"boundary":  parentRequestDigest("texture:doc-a", "ship the pag", `e{"summary":"x"}`),
	} {
		if other == base {
			t.Errorf("digest ignores a change in %s", name)
		}
	}
	a := engineeringRequestDigest("do it", "parent-1", "cast-1", base)
	if a != engineeringRequestDigest("do it", "parent-1", "cast-1", base) {
		t.Fatal("binding request digest is not deterministic")
	}
	if a == engineeringRequestDigest("do it", "parent-1", "cast-1", parentRequestDigest("texture:doc-a", "ship the page", `{"summary":"CHANGED"}`)) {
		t.Fatal("a replay with different parent text keeps the same binding RequestDigest: it would not be refused")
	}
	if legacyEngineeringRequestDigest("do it", "parent-1", "cast-1") == a {
		t.Fatal("the new binding digest equals the legacy one: the parent is not folded in")
	}
}

func TestBoundRequestTextIsBoundedAndMarked(t *testing.T) {
	if got, cut := boundRequestText("objective", "short", 64, "r1"); got != "short" || cut {
		t.Fatalf("a small text was changed: %q %v", got, cut)
	}
	big := strings.Repeat("é", 100) // 2 bytes per rune, 200 bytes
	got, cut := boundRequestText("objective", big, 51, "texture-cell:Cast:r1")
	if !cut || !utf8.ValidString(got) {
		t.Fatalf("valid=%v truncated=%v", utf8.ValidString(got), cut)
	}
	if !strings.Contains(got, "truncated") || !strings.Contains(got, "200 bytes") || !strings.Contains(got, "texture-cell:Cast:r1") {
		t.Fatalf("no visible marker naming the size and the record: %q", got)
	}
}

func TestEngineeringCastShowsTheTextureRequest(t *testing.T) {
	f := newRequestFixture(t)
	ctx := context.Background()
	parent := f.textureCast(t, f.seed.TextureInbox, "real", "put the probe line in docs/probes/x.md", `{"path":"docs/probes/x.md","line":"hello","test":"go vet ./internal/x"}`)
	inbox := f.open(t, &parent, "capability-real", boundDigest(parent))

	// A fresh Runtime over the same store (a restarted process) reads the
	// same request: nothing is process-local.
	for name, rt := range map[string]*Runtime{"running": f.rt, "fresh": {store: f.s}} {
		views := f.view(t, rt, inbox)
		if len(views) != 1 {
			t.Fatalf("%s: %d records", name, len(views))
		}
		got := views[0]
		if got.RequestError != "" || got.Request == nil {
			t.Fatalf("%s: no request (error %q)", name, got.RequestError)
		}
		r := got.Request
		if r.From != parent.SenderInbox || r.Objective != parent.Directive.Objective || r.Body != parent.Directive.Body ||
			r.RecordID != parent.RecordID || r.Truncated {
			t.Fatalf("%s: request is not the Texture Cast verbatim: %+v", name, r)
		}
		if r.Digest != parentRequestDigest(parent.SenderInbox, parent.Directive.Objective, parent.Directive.Body) {
			t.Fatalf("%s: request digest %q", name, r.Digest)
		}
		if !strings.Contains(got.String(), "docs/probes/x.md") {
			t.Fatalf("%s: a printed record does not show the request: %s", name, got.String())
		}
	}

	// The run objective is management's objective only: the request reaches
	// engineering through Records(), never the prompt.
	assignment, err := f.s.GetEngineeringAssignment(ctx, f.seed.OwnerID, f.seed.ComputerID, f.seed.AssignmentIDs[0], 1)
	if err != nil {
		t.Fatal(err)
	}
	objective, err := f.rt.assignmentObjective(ctx, assignment)
	if err != nil || strings.Contains(objective, "docs/probes/x.md") {
		t.Fatalf("run objective %q (err %v) carries the parent text", objective, err)
	}
}

func TestEngineeringRequestRefusedWhenBindingDoesNotMatch(t *testing.T) {
	// Legacy: a binding minted before the fix, digest without the parent.
	f := newRequestFixture(t)
	parent := f.textureCast(t, f.seed.TextureInbox, "legacy", "legacy ask", "legacy body")
	inbox := f.open(t, &parent, "capability-legacy", func(objective, parentID, recordID string) string {
		return legacyEngineeringRequestDigest(objective, parentID, recordID)
	})
	v := f.view(t, f.rt, inbox)
	if len(v) != 1 || v[0].Request != nil || !strings.Contains(v[0].RequestError, "predates") || strings.Contains(v[0].String(), "legacy body") {
		t.Fatalf("legacy binding showed unbound text or no reason: %+v", v)
	}

	// Drift: the binding commits to different parent text than the tape holds.
	g := newRequestFixture(t)
	parent = g.textureCast(t, g.seed.TextureInbox, "drift", "drift ask", "drift body")
	inbox = g.open(t, &parent, "capability-drift", func(string, string, string) string {
		return objectgraph.SHA256([]byte("a different parent"))
	})
	v = g.view(t, g.rt, inbox)
	if len(v) != 1 || v[0].Request != nil || !strings.Contains(v[0].RequestError, "does not match") || strings.Contains(v[0].String(), "drift body") {
		t.Fatalf("drifted binding showed text or no reason: %+v", v)
	}
}

// The store's open already admits only a Texture's Cast as the parent; the
// view re-checks, so a record whose parent is not one (here the Cast record
// handed to the view is pointed at a research Cast, as a corrupted tape
// would be) shows nothing even when the binding's digest matches that text.
func TestEngineeringRequestRefusedForNonTextureParent(t *testing.T) {
	f := newRequestFixture(t)
	good := f.textureCast(t, f.seed.TextureInbox, "ok", "real ask", "real body")
	inbox := f.open(t, &good, "capability-nt", boundDigest(good))
	notTexture := f.textureCast(t, "research:document-x", "notx", "secret research ask", "secret research body")

	cr, err := f.rt.loadCellRecords(context.Background(), f.seed.OwnerID, f.seed.ComputerID, inbox, "")
	if err != nil || len(cr.view) != 1 {
		t.Fatalf("view %+v err %v", cr.view, err)
	}
	castRec, err := f.s.GetCommitmentRecord(context.Background(), f.seed.OwnerID, f.seed.ComputerID, cr.view[0].RecordID)
	if err != nil {
		t.Fatal(err)
	}
	rec := *castRec
	rec.ParentID = notTexture.RecordID
	view := inboxRecordView(rec)
	f.rt.engineeringRequestView(context.Background(), f.seed.OwnerID, f.seed.ComputerID, rec, &view)
	if view.Request != nil || !strings.Contains(view.RequestError, "not a Texture") || strings.Contains(view.String(), "secret research") {
		t.Fatalf("a non-Texture parent was shown: %s", view.String())
	}
	if isTextureCastTo(&notTexture, f.seed.ParentAgentID) || !isTextureCastTo(&good, f.seed.ParentAgentID) || isTextureCastTo(&good, "management:someone-else") || isTextureCastTo(nil, "x") {
		t.Fatal("isTextureCastTo accepts the wrong records")
	}
}

func TestEngineeringRequestIsBoundedAndStillVerifies(t *testing.T) {
	f := newRequestFixture(t)
	parent := f.textureCast(t, f.seed.TextureInbox, "big", strings.Repeat("o", 20000), strings.Repeat("é", 20000)) // objective 20000 bytes, body 40000 bytes
	inbox := f.open(t, &parent, "capability-big", boundDigest(parent))
	v := f.view(t, f.rt, inbox)
	if len(v) != 1 || v[0].Request == nil {
		t.Fatalf("view %+v", v)
	}
	r := v[0].Request
	if !r.Truncated || r.Digest != parentRequestDigest(parent.SenderInbox, parent.Directive.Objective, parent.Directive.Body) {
		t.Fatalf("truncated=%v digest=%q (want the full text's digest)", r.Truncated, r.Digest)
	}
	if len(r.Body) > engineeringRequestBodyMax+512 || len(r.Objective) > engineeringRequestObjectiveMax+512 {
		t.Fatalf("not bounded: objective %d body %d", len(r.Objective), len(r.Body))
	}
	if !utf8.ValidString(r.Body) || !utf8.ValidString(r.Objective) || !strings.Contains(r.Body, "truncated") || !strings.Contains(r.Body, parent.RecordID) {
		t.Fatal("cut text is not valid UTF-8 or not marked with the parent record id")
	}
}

// The 2026-10-24 case (docs/problems/engineering-cast-objective-cut-at-4kib-
// 2026-10-24.md): Texture put a 6,338 B request in the Cast's objective and
// engineering saw it cut at 4 KiB, with a marker naming a record it has no
// verb to read. A request this size must reach the Records() view whole, and
// the Body desks print must carry all of it.
func TestEngineeringRequestOf6338BytesReachesTheViewWhole(t *testing.T) {
	f := newRequestFixture(t)
	objective := strings.Repeat("scope line for the app and the file rules\n", 6338/42+1)[:6338]
	if len(objective) != 6338 {
		t.Fatalf("fixture objective is %d bytes", len(objective))
	}
	parent := f.textureCast(t, f.seed.TextureInbox, "whole", objective, `{"app":"podcasts"}`)
	inbox := f.open(t, &parent, "capability-whole", boundDigest(parent))
	v := f.view(t, f.rt, inbox)
	if len(v) != 1 || v[0].Request == nil {
		t.Fatalf("view %+v", v)
	}
	r := v[0].Request
	if r.Truncated || r.Objective != objective || strings.Contains(r.Objective, "truncated") {
		t.Fatalf("the 6338-byte objective was cut: truncated=%v kept %d bytes", r.Truncated, len(r.Objective))
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(v[0].Body), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	req, _ := body["request"].(map[string]any)
	if req == nil || req["objective"] != objective || req["truncated"] != false {
		t.Fatalf("the Body desks print does not carry the whole objective: %.200s", v[0].Body)
	}
}

// Text past the bound is still cut, and the marker must not promise a read
// engineering cannot make: no verb reads a record by ref, so it names the
// field and asks for a report, not a guess (no desk can read the cut bytes).
func TestEngineeringRequestCutMarkerNamesAWorkingRoute(t *testing.T) {
	f := newRequestFixture(t)
	parent := f.textureCast(t, f.seed.TextureInbox, "marker", strings.Repeat("o", 20000), "b")
	inbox := f.open(t, &parent, "capability-marker", boundDigest(parent))
	v := f.view(t, f.rt, inbox)
	if len(v) != 1 || v[0].Request == nil || !v[0].Request.Truncated {
		t.Fatalf("view %+v", v)
	}
	o := v[0].Request.Objective
	if strings.Contains(o, "the full text is on the tape") {
		t.Fatalf("the marker still points at a record engineering cannot read: %s", o[len(o)-300:])
	}
	if !strings.Contains(o, "choir.Report") || !strings.Contains(o, "objective") || strings.Contains(o, "choir.Ask") || !strings.Contains(o, "20000") || !strings.Contains(o, parent.RecordID) {
		t.Fatalf("the marker does not say the rest is unavailable and how to ask for it: %s", o[len(o)-300:])
	}
}

func TestEngineeringLaterCastsCarryNoRequest(t *testing.T) {
	f := newRequestFixture(t)
	parent := f.textureCast(t, f.seed.TextureInbox, "later", "first ask", "first body")
	inbox := f.open(t, &parent, "capability-later", boundDigest(parent))
	later := store.EngineeringCastRecord("management-cell:Cast:later", f.seed.ParentAgentID, f.seed.AssignmentIDs[0], parent.RecordID, "another thing")
	commit := types.CommitInboxRecordsRequest{OwnerID: f.seed.OwnerID, ComputerID: f.seed.ComputerID, CommandID: "later-cast", Records: []types.CommitmentRecord{later}}
	commit.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(commit)
	if _, err := f.s.CommitInboxRecords(context.Background(), commit); err != nil {
		t.Fatal(err)
	}
	v := f.view(t, f.rt, inbox)
	if len(v) != 2 {
		t.Fatalf("view %+v", v)
	}
	if v[0].Request == nil || v[1].Request != nil || v[1].RequestError != "" {
		t.Fatalf("only the Cast that opened the assignment carries the request: %+v", v)
	}
}

// Desks print r.Body, not r.Request (drills-0565-request-unseen-and-receipt-
// typos-2026-10-20): the opening Cast's Body carries the request inside its
// spec JSON, or the reason it is missing, so printing Body shows it.
func TestEngineeringCastBodyCarriesTheRequest(t *testing.T) {
	f := newRequestFixture(t)
	parent := f.textureCast(t, f.seed.TextureInbox, "body", "put the probe line in docs/probes/y.md", `{"path":"docs/probes/y.md","line":"hi"}`)
	inbox := f.open(t, &parent, "capability-body", boundDigest(parent))
	v := f.view(t, f.rt, inbox)
	if len(v) != 1 {
		t.Fatalf("%d records", len(v))
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(v[0].Body), &body); err != nil {
		t.Fatalf("body is not JSON: %v: %s", err, v[0].Body)
	}
	req, ok := body["request"].(map[string]any)
	if !ok || req["objective"] != parent.Directive.Objective || req["body"] != parent.Directive.Body || req["from"] != parent.SenderInbox {
		t.Fatalf("body does not carry the request verbatim: %s", v[0].Body)
	}

	g := newRequestFixture(t)
	parent = g.textureCast(t, g.seed.TextureInbox, "body-drift", "drift ask", "drift secret body")
	inbox = g.open(t, &parent, "capability-body-drift", func(string, string, string) string {
		return objectgraph.SHA256([]byte("a different parent"))
	})
	v = g.view(t, g.rt, inbox)
	if len(v) != 1 || !strings.Contains(v[0].Body, "request_error") || strings.Contains(v[0].Body, "drift secret body") {
		t.Fatalf("drifted body shows text or no reason: %s", v[0].Body)
	}
}

func TestRequestIntoBodyKeepsTheSpec(t *testing.T) {
	v := yaegikernel.InboxRecord{Body: `{"parent":"cell:x","network":"forbidden"}`, Request: &yaegikernel.InboxRequest{RecordID: "cell:x", From: "texture:d", Objective: "o", Body: "b"}}
	requestIntoBody(&v)
	var got map[string]any
	if err := json.Unmarshal([]byte(v.Body), &got); err != nil || got["parent"] != "cell:x" || got["network"] != "forbidden" || got["request"] == nil {
		t.Fatalf("spec keys lost or no request: %s (%v)", v.Body, err)
	}
	plain := yaegikernel.InboxRecord{Body: "not json", RequestError: "why"}
	requestIntoBody(&plain)
	if err := json.Unmarshal([]byte(plain.Body), &got); err != nil || got["spec"] != "not json" || got["request_error"] != "why" {
		t.Fatalf("non-JSON body: %s (%v)", plain.Body, err)
	}
	none := yaegikernel.InboxRecord{Body: `{"parent":"p"}`}
	requestIntoBody(&none)
	if none.Body != `{"parent":"p"}` {
		t.Fatalf("a record with no request changed: %s", none.Body)
	}
}
