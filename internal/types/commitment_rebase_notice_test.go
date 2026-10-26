package types

import (
	"strings"
	"testing"
)

// U4 slice 1 (docs/design/u4-stack-rebase-build-2026-10-26.md, "Slices",
// slice 1 and failure mode D7): the rebase notice is a system Note, minted
// only by the runtime's live timer. No agent record can pass for one.

const (
	rebaseTestBase = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	rebaseTestOp   = "op-a"
)

func validRebaseNotice(t *testing.T) CommitmentRecord {
	t.Helper()
	rec, ok := NewStackRebaseNotice(StackRebaseNotice{
		TextureInbox: "texture:doc-master", ComputerID: "computer-1", OperationID: rebaseTestOp,
		BootedBase: rebaseTestBase, StackBase: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Attempt: 1, DocID: "doc-master",
	})
	if !ok {
		t.Fatal("a valid rebase notice did not build")
	}
	return rec
}

func TestRebaseNoticeIDsAreDerivedFromTheAttemptsPair(t *testing.T) {
	if got, want := StackRebaseNoticeID(rebaseTestOp, rebaseTestBase, 2), "rebase-notice:op-a:"+rebaseTestBase+":2"; got != want {
		t.Fatalf("notice id %q, want %q", got, want)
	}
	if got, want := StackRebaseSenderInbox(rebaseTestOp), "selfdev-rebase:op-a"; got != want {
		t.Fatalf("sender inbox %q, want %q", got, want)
	}
	if StackRebasePairKey(rebaseTestOp, rebaseTestBase) == "" || StackRebasePairKey(rebaseTestOp, rebaseTestBase) == StackRebasePairKey(rebaseTestOp, "cccc") {
		t.Fatal("the pair key must be non-empty and differ by base")
	}
	op, base, n, ok := ParseStackRebaseNoticeID(StackRebaseNoticeID(rebaseTestOp, rebaseTestBase, 3))
	if !ok || op != rebaseTestOp || base != rebaseTestBase || n != 3 {
		t.Fatalf("parse = %q %q %d %v", op, base, n, ok)
	}
	for _, bad := range []string{"", "rebase-notice:", "rebase-notice:op-a:" + rebaseTestBase, "rebase-notice:op-a:" + rebaseTestBase + ":x", "rebase-notice:op-a:" + rebaseTestBase + ":0", "owner-act:op-a:apply"} {
		if _, _, _, ok := ParseStackRebaseNoticeID(bad); ok {
			t.Fatalf("%q parsed as a notice id", bad)
		}
	}
}

func TestRebaseNoticeShapeIsASystemNoteToTheRoot(t *testing.T) {
	rec := validRebaseNotice(t)
	if !IsStackRebaseNotice(rec) {
		t.Fatalf("a freshly built notice is not a notice: %+v", rec)
	}
	if rec.EmitterKind != CommitmentEmitterSystem || rec.Final || rec.Round != 0 || rec.Inbox != "texture:doc-master" ||
		rec.SenderInbox != "selfdev-rebase:op-a" || rec.Directive == nil || rec.Directive.Subtype != CommitmentDirectiveNote {
		t.Fatalf("notice shape %+v", rec)
	}
	if rec.RecordID != StackRebaseNoticeID(rebaseTestOp, rebaseTestBase, 1) {
		t.Fatalf("record id %q", rec.RecordID)
	}
	for key, want := range map[string]string{"operation_id": rebaseTestOp, "booted_base": rebaseTestBase, "stack_base": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "attempt": "1", "doc_id": "doc-master"} {
		if rec.SubjectRefs[key] != want {
			t.Fatalf("subject ref %s = %q, want %q", key, rec.SubjectRefs[key], want)
		}
	}
	// The body states the exact Cast the root makes, with the notice id.
	if !strings.Contains(rec.Directive.Body, rec.RecordID) || !strings.Contains(rec.Directive.Body, `choir.Cast("management"`) ||
		!strings.Contains(rec.Directive.Body, `"rebase_notice"`) {
		t.Fatalf("the body must state the root's exact Cast with the notice id: %s", rec.Directive.Body)
	}
}

func TestRebaseNoticeCannotBeForged(t *testing.T) {
	good := validRebaseNotice(t)
	mutate := func(f func(*CommitmentRecord)) CommitmentRecord {
		rec := good
		rec.SubjectRefs = map[string]string{}
		for k, v := range good.SubjectRefs {
			rec.SubjectRefs[k] = v
		}
		d := *good.Directive
		rec.Directive = &d
		f(&rec)
		return rec
	}
	cases := map[string]CommitmentRecord{
		"agent emitter":           mutate(func(r *CommitmentRecord) { r.EmitterKind = CommitmentEmitterAgent }),
		"wrong sender":            mutate(func(r *CommitmentRecord) { r.SenderInbox = "management:owner" }),
		"owner-act sender":        mutate(func(r *CommitmentRecord) { r.SenderInbox = OwnerActSenderInbox(rebaseTestOp) }),
		"sender names another op": mutate(func(r *CommitmentRecord) { r.SenderInbox = StackRebaseSenderInbox("op-b") }),
		"attempt zero": mutate(func(r *CommitmentRecord) {
			r.RecordID = StackRebaseNoticeID(rebaseTestOp, rebaseTestBase, 0)
			r.SubjectRefs["attempt"] = "0"
		}),
		"attempt over the cap": mutate(func(r *CommitmentRecord) {
			r.RecordID = StackRebaseNoticeID(rebaseTestOp, rebaseTestBase, 4)
			r.SubjectRefs["attempt"] = "4"
		}),
		"id not derived":          mutate(func(r *CommitmentRecord) { r.RecordID = "rebase-notice:forged" }),
		"id names another base":   mutate(func(r *CommitmentRecord) { r.RecordID = StackRebaseNoticeID(rebaseTestOp, "cccc", 1) }),
		"subject op differs":      mutate(func(r *CommitmentRecord) { r.SubjectRefs["operation_id"] = "op-b" }),
		"subject base differs":    mutate(func(r *CommitmentRecord) { r.SubjectRefs["booted_base"] = "cccc" }),
		"subject attempt differs": mutate(func(r *CommitmentRecord) { r.SubjectRefs["attempt"] = "2" }),
		"not a note":              mutate(func(r *CommitmentRecord) { r.Directive.Subtype = CommitmentDirectiveCast }),
		"no directive":            mutate(func(r *CommitmentRecord) { r.Directive = nil }),
		"final":                   mutate(func(r *CommitmentRecord) { r.Final = true }),
		"to a non-texture inbox":  mutate(func(r *CommitmentRecord) { r.Inbox = "management:owner" }),
	}
	for name, rec := range cases {
		if IsStackRebaseNotice(rec) {
			t.Errorf("%s passes for a rebase notice", name)
		}
	}
	// An owner-act record and a progress relay are not rebase notices.
	if act, ok := NewSelfDevelopmentOwnerAct(SelfDevelopmentOwnerAct{TextureInbox: "texture:d", OperationID: "op-a", Act: OwnerActApply, Body: "x"}); !ok || IsStackRebaseNotice(act) {
		t.Fatalf("owner act passes for a notice: %v", ok)
	}
}

func TestRebaseNoticeRefusesWhatDoesNotBind(t *testing.T) {
	ok := StackRebaseNotice{TextureInbox: "texture:d", ComputerID: "c", OperationID: "op", BootedBase: rebaseTestBase, StackBase: "aaaa", Attempt: 1, DocID: "d"}
	if _, built := NewStackRebaseNotice(ok); !built {
		t.Fatal("baseline notice must build")
	}
	for name, f := range map[string]func(*StackRebaseNotice){
		"not a texture inbox":  func(n *StackRebaseNotice) { n.TextureInbox = "management:o" },
		"bare texture inbox":   func(n *StackRebaseNotice) { n.TextureInbox = "texture:" },
		"no operation":         func(n *StackRebaseNotice) { n.OperationID = "" },
		"no booted base":       func(n *StackRebaseNotice) { n.BootedBase = "" },
		"no stack base":        func(n *StackRebaseNotice) { n.StackBase = "" },
		"attempt zero":         func(n *StackRebaseNotice) { n.Attempt = 0 },
		"attempt over the cap": func(n *StackRebaseNotice) { n.Attempt = RebaseNoticeMaxAttempts + 1 },
	} {
		n := ok
		f(&n)
		if _, built := NewStackRebaseNotice(n); built {
			t.Errorf("%s still builds a notice", name)
		}
	}
}

func TestCastRebaseNoticeReadsTheSpecField(t *testing.T) {
	if got := CastRebaseNotice(`{"parent":"r1","rebase_notice":"rebase-notice:op:b:1"}`); got != "rebase-notice:op:b:1" {
		t.Fatalf("got %q", got)
	}
	for _, body := range []string{"", "null", "not json", `{"parent":"r1"}`, `{"rebase_notice":7}`, `[1]`} {
		if got := CastRebaseNotice(body); got != "" {
			t.Fatalf("CastRebaseNotice(%q) = %q, want empty", body, got)
		}
	}
}

// U4 slice 8 (docs/design/u4-stack-rebase-build-2026-10-26.md): the owner's
// retry act re-opens a spent pair under a fourth and later attempt carrying an
// owner-act stamp. Ways it can fail: an agent-forgeable or automatic attempt
// above the cap passes; an owner-stamped notice poses as an automatic one;
// the claim key for a later attempt collides with the pair's first claim; a
// gap in the attempt numbers leaves an older attempt unsuperseded.

func ownerRetryNotice(t *testing.T, attempt int) CommitmentRecord {
	t.Helper()
	rec, ok := NewStackRebaseNotice(StackRebaseNotice{
		TextureInbox: "texture:doc-master", ComputerID: "computer-1", OperationID: rebaseTestOp, BootedBase: rebaseTestBase,
		StackBase: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Attempt: attempt, DocID: "doc-master", OwnerRetry: true,
	})
	if !ok {
		t.Fatalf("owner retry notice %d did not build", attempt)
	}
	return rec
}

func TestOwnerRetryNoticeIsStampedAndOnlyAboveTheCap(t *testing.T) {
	rec := ownerRetryNotice(t, 4)
	if !IsStackRebaseNotice(rec) || rec.SubjectRefs["owner_retry"] != "true" || rec.Provenance.AgentID != RebaseOwnerRetryAgentID {
		t.Fatalf("owner retry notice %+v", rec)
	}
	if !strings.HasPrefix(rec.Directive.Body, "The owner asked for another try.") || !strings.Contains(rec.Directive.Body, rec.RecordID) {
		t.Fatalf("the body must name the id and say the owner asked: %s", rec.Directive.Body)
	}
	// The automatic notice does not claim the owner asked.
	if strings.Contains(validRebaseNotice(t).Directive.Body, "The owner asked") {
		t.Fatal("an automatic notice says the owner asked")
	}
	if !IsStackRebaseNotice(ownerRetryNotice(t, 9)) {
		t.Fatal("later owner retries are notices")
	}
	// An owner-stamped notice cannot be built at an automatic attempt number,
	// and an automatic notice cannot be built above the cap.
	for _, n := range []int{0, 1, 2, 3} {
		if _, ok := NewStackRebaseNotice(StackRebaseNotice{TextureInbox: "texture:d", OperationID: "op", BootedBase: rebaseTestBase, StackBase: "aaaa", Attempt: n, OwnerRetry: true}); ok {
			t.Errorf("an owner retry built at attempt %d", n)
		}
	}
	if _, ok := NewStackRebaseNotice(StackRebaseNotice{TextureInbox: "texture:d", OperationID: "op", BootedBase: rebaseTestBase, StackBase: "aaaa", Attempt: 4}); ok {
		t.Error("an automatic notice built above the cap")
	}
	// Stripping the stamp, or stamping an automatic attempt, is not a notice.
	unstamped := ownerRetryNotice(t, 4)
	delete(unstamped.SubjectRefs, "owner_retry")
	if IsStackRebaseNotice(unstamped) {
		t.Error("an unstamped attempt 4 passes")
	}
	wrongAgent := ownerRetryNotice(t, 4)
	wrongAgent.Provenance.AgentID = RebaseNoticeAgentID
	if IsStackRebaseNotice(wrongAgent) {
		t.Error("an owner retry with the automatic provenance passes")
	}
	autoStamped := validRebaseNotice(t)
	autoStamped.SubjectRefs["owner_retry"] = "true"
	if IsStackRebaseNotice(autoStamped) {
		t.Error("an automatic attempt carrying the owner stamp passes")
	}
}

func TestRebaseClaimKeyIsPerGeneration(t *testing.T) {
	pair := StackRebasePairKey(rebaseTestOp, rebaseTestBase)
	for _, n := range []int{1, 2, 3} {
		if StackRebaseClaimKey(pair, n) != pair {
			t.Fatalf("attempt %d must share the pair's claim", n)
		}
	}
	if StackRebaseClaimKey(pair, 4) == pair || StackRebaseClaimKey(pair, 4) == StackRebaseClaimKey(pair, 5) {
		t.Fatal("each owner generation has its own claim key")
	}
}

func TestStackRebaseNewerAttemptIDsCoverGaps(t *testing.T) {
	ids := func(n int) []string { return StackRebaseNewerAttemptIDs(rebaseTestOp, rebaseTestBase, n) }
	id := func(n int) string { return StackRebaseNoticeID(rebaseTestOp, rebaseTestBase, n) }
	if got := ids(1); len(got) != 3 || got[0] != id(2) || got[1] != id(3) || got[2] != id(4) {
		t.Fatalf("newer than 1: %v", got)
	}
	if got := ids(2); len(got) != 2 || got[0] != id(3) || got[1] != id(4) {
		t.Fatalf("newer than 2: %v (an owner retry may follow attempt 2 with a gap)", got)
	}
	if got := ids(3); len(got) != 1 || got[0] != id(4) {
		t.Fatalf("newer than 3: %v", got)
	}
	if got := ids(4); len(got) != 1 || got[0] != id(5) {
		t.Fatalf("newer than 4: %v", got)
	}
}
