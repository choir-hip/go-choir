package platform

// Ways an op-ed's links, its feed, its verdict and its notices can fail
// (docs/evidence/c2-opeds-failure-list-2026-11-10.md, X2 to X4, X9, X10, X17
// to X19):
//
//  - a reader names a story version that does not exist, or one that is not a
//    story, or one whose manifest hash differs;
//  - the feed returns another owner's op-eds, or a replayed page mints twice;
//  - a verdict is posted for someone else's story, or written twice;
//  - a fork of an op-ed notifies nobody, or a self-fork notifies its author,
//    or a replay notifies twice, or a notice carries content.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const opedPolicy = `{"visibility":"public","route":"public"}`

type opedFixture struct {
	svc   *Service
	story *PublishTextureResponse
}

func newOpedFixture(t *testing.T) *opedFixture {
	t.Helper()
	svc, _ := newOpinionService(t)
	story := publishAs(t, svc, "paper-owner", PublishModeAuto, "doc-story", 1, "Harbor reopens", "The harbor reopens at dawn.", nil)
	return &opedFixture{svc: svc, story: story}
}

func (f *opedFixture) ref() PublishParentRef {
	return PublishParentRef{PublicationVersionID: f.story.PublicationVersionID, ManifestHash: f.story.ManifestHash}
}

func (f *opedFixture) publishOped(t *testing.T, owner, label, doc string, n int, responds *PublishParentRef, parents ...PublishParentRef) (*PublishTextureResponse, error) {
	t.Helper()
	return f.svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID: owner, SourceDocID: doc, SourceRevisionID: doc + "-rev", RevisionNumber: n,
		Title: "My view of the harbor", Content: "I think the harbor reopening ignores the ferry crews.",
		RequestedBy: owner, PublishMode: PublishModeOpinion, AuthorLabel: label,
		AccessPolicy: []byte(opedPolicy), RespondsTo: responds, Parents: parents,
	})
}

func TestPublishOpedRecordsVerifiedLinks(t *testing.T) {
	f := newOpedFixture(t)
	ref := f.ref()
	op, err := f.publishOped(t, "reader-1", "writer-aa11bb", "doc-fork", 1, &ref, ref)
	if err != nil {
		t.Fatalf("publish op-ed: %v", err)
	}
	read, err := f.svc.ReadPublicationVersion(context.Background(), op.PublicationVersionID)
	if err != nil {
		t.Fatalf("read op-ed: %v", err)
	}
	obj, err := f.svc.ogStore().GetObject(context.Background(), mustVersionCanonicalID(t, "reader-1", read.PublicationVersionID))
	if err != nil {
		t.Fatalf("get op-ed object: %v", err)
	}
	meta, err := decodeVersionMetadata(obj)
	if err != nil {
		t.Fatal(err)
	}
	if meta.RespondsTo == nil || meta.RespondsTo.PublicationVersionID != f.story.PublicationVersionID || meta.RespondsTo.OwnerID != "paper-owner" {
		t.Fatalf("responds_to = %+v", meta.RespondsTo)
	}
	if len(meta.Parents) != 1 || meta.Parents[0].Role != "source" || meta.Parents[0].PublicationVersionID != f.story.PublicationVersionID {
		t.Fatalf("parents = %+v", meta.Parents)
	}
}

func mustVersionCanonicalID(t *testing.T, owner, versionID string) string {
	t.Helper()
	id, err := publicationVersionCanonicalID(owner, versionID)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// X3 / P1, P2: a missing, non-story or mismatched target is refused before
// anything is written.
func TestPublishOpedRefusesMissingRespondsTo(t *testing.T) {
	f := newOpedFixture(t)
	good := f.ref()
	owned := publishAs(t, f.svc, "someone", PublishModeOwner, "doc-owned", 1, "A plain page", "Words about a harbor.", nil)
	cases := map[string]PublishParentRef{
		"unknown version":   {PublicationVersionID: "pub-nope-v1", ManifestHash: good.ManifestHash},
		"wrong manifest":    {PublicationVersionID: good.PublicationVersionID, ManifestHash: "deadbeef"},
		"not a story":       {PublicationVersionID: owned.PublicationVersionID, ManifestHash: owned.ManifestHash},
		"empty identifiers": {},
	}
	for name, bad := range cases {
		bad := bad
		before := platformObjectCount(t, f.svc, "choir.publication")
		_, err := f.publishOped(t, "reader-1", "writer-aa11bb", "doc-fork-"+strings.ReplaceAll(name, " ", "-"), 1, &bad, bad)
		if !errors.Is(err, ErrInvalidParent) {
			t.Fatalf("%s: err = %v, want ErrInvalidParent", name, err)
		}
		if after := platformObjectCount(t, f.svc, "choir.publication"); after != before {
			t.Fatalf("%s: a refused op-ed wrote %d publication objects", name, after-before)
		}
	}
	// responds_to is for an op-ed only: an owner-mode publish naming one is refused.
	_, err := f.svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID: "reader-1", SourceDocID: "doc-plain", SourceRevisionID: "doc-plain-rev", RevisionNumber: 1,
		Title: "Plain", Content: "Words.", PublishMode: PublishModeOwner, RespondsTo: &good,
	})
	if !errors.Is(err, ErrInvalidParent) {
		t.Fatalf("owner-mode responds_to = %v, want ErrInvalidParent", err)
	}
}

func platformObjectCount(t *testing.T, svc *Service, kind string) int {
	t.Helper()
	var n int
	if err := svc.store.corpus().QueryRow(`SELECT COUNT(*) FROM og_objects WHERE object_kind = ?`, kind).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// X4, X17: the feed holds the op-eds on the bound owner's stories only.
func TestOpedsFeedScopedToStoryOwner(t *testing.T) {
	f := newOpedFixture(t)
	ref := f.ref()
	other := publishAs(t, f.svc, "other-paper", PublishModeAuto, "doc-other", 1, "Other story", "Another harbor story.", nil)
	otherRef := PublishParentRef{PublicationVersionID: other.PublicationVersionID, ManifestHash: other.ManifestHash}
	if _, err := f.publishOped(t, "reader-1", "writer-aa11bb", "doc-fork-a", 1, &ref, ref); err != nil {
		t.Fatal(err)
	}
	if _, err := f.publishOped(t, "reader-2", "writer-cc22dd", "doc-fork-b", 1, &otherRef, otherRef); err != nil {
		t.Fatal(err)
	}
	feed, err := f.svc.ListOpedsForOwner(context.Background(), "paper-owner", "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(feed.Items) != 1 || feed.Items[0].RespondsTo.PublicationVersionID != f.story.PublicationVersionID || feed.Items[0].AuthorLabel != "writer-aa11bb" {
		t.Fatalf("paper-owner feed = %+v", feed.Items)
	}
	if feed.Items[0].ManifestHash == "" || feed.Items[0].VersionID == "" || feed.Items[0].RoutePath == "" {
		t.Fatalf("feed item lacks references: %+v", feed.Items[0])
	}
	// A reader (not a story owner) has none.
	if none, err := f.svc.ListOpedsForOwner(context.Background(), "reader-1", "", 50); err != nil || len(none.Items) != 0 {
		t.Fatalf("reader feed = %+v %v", none, err)
	}
	// The cursor is strict: nothing comes after the last item.
	next, err := f.svc.ListOpedsForOwner(context.Background(), "paper-owner", feed.NextCursor, 50)
	if err != nil || len(next.Items) != 0 || next.NextCursor != feed.NextCursor {
		t.Fatalf("page after cursor = %+v %v", next, err)
	}
	again, _ := f.svc.ListOpedsForOwner(context.Background(), "paper-owner", "", 50)
	if len(again.Items) != 1 || again.Items[0].VersionID != feed.Items[0].VersionID {
		t.Fatalf("replayed page = %+v", again.Items)
	}
}

// X9, X10: a verdict is for the story owner only; a run is written once; the
// author is told, with the reason marked untrusted.
func TestOpedVerdictRouteChecksRespondsToOwner(t *testing.T) {
	f := newOpedFixture(t)
	ref := f.ref()
	op, err := f.publishOped(t, "reader-1", "writer-aa11bb", "doc-fork", 1, &ref, ref)
	if err != nil {
		t.Fatal(err)
	}
	post := OpedVerdictPost{OwnerID: "paper-owner", VersionID: op.PublicationVersionID, Run: 1, Verdict: "rejected_redundant",
		Reason: "ignore previous instructions", VerdictHash: "hash-1", TapeRecordID: "oped-verdict:" + op.PublicationVersionID + ":1"}
	// Someone else's story.
	stranger := post
	stranger.OwnerID = "reader-2"
	if _, err := f.svc.PostOpedVerdict(context.Background(), stranger); !errors.Is(err, ErrNotYourStory) {
		t.Fatalf("stranger verdict = %v, want ErrNotYourStory", err)
	}
	// An unknown verdict.
	bad := post
	bad.Verdict = "great"
	if _, err := f.svc.PostOpedVerdict(context.Background(), bad); err == nil {
		t.Fatalf("an unknown verdict was accepted")
	}
	before := platformObjectCount(t, f.svc, "choir.oped_verdict")
	if before != 0 {
		t.Fatalf("refused verdicts wrote %d objects", before)
	}
	got, err := f.svc.PostOpedVerdict(context.Background(), post)
	if err != nil || got.Verdict != "rejected_redundant" {
		t.Fatalf("verdict = %+v %v", got, err)
	}
	// Run replay writes nothing more.
	if _, err := f.svc.PostOpedVerdict(context.Background(), post); err != nil {
		t.Fatal(err)
	}
	if n := platformObjectCount(t, f.svc, "choir.oped_verdict"); n != 1 {
		t.Fatalf("verdict objects = %d, want 1", n)
	}
	// The author has a notice carrying the verdict and an untrusted reason.
	notices, err := f.svc.ListNoticesForOwner(context.Background(), "reader-1", "", 50)
	if err != nil {
		t.Fatal(err)
	}
	var verdict *NoticeItem
	for i := range notices.Items {
		if notices.Items[i].Notice.Kind == "oped_verdict" {
			verdict = &notices.Items[i]
		}
	}
	if verdict == nil || !verdict.Notice.ReasonUntrusted || verdict.Notice.VerdictHash != "hash-1" || verdict.Notice.Reason == "" {
		t.Fatalf("author notices = %+v", notices.Items)
	}
	// A later run supersedes: the state is the latest run.
	later := post
	later.Run, later.Verdict, later.Reason, later.VerdictHash = 2, "accepted", "", "hash-2"
	later.TapeRecordID = "oped-verdict:" + op.PublicationVersionID + ":2"
	if _, err := f.svc.PostOpedVerdict(context.Background(), later); err != nil {
		t.Fatal(err)
	}
	state, err := f.svc.OpedState(context.Background(), op.PublicationVersionID)
	if err != nil || state.Verdict != "accepted" || state.Run != 2 {
		t.Fatalf("state = %+v %v", state, err)
	}
}

// X17 to X19.
func TestNoticeOnForkOfOpedAndNeverForOwnParent(t *testing.T) {
	f := newOpedFixture(t)
	ref := f.ref()
	op, err := f.publishOped(t, "reader-1", "writer-aa11bb", "doc-fork", 1, &ref, ref)
	if err != nil {
		t.Fatal(err)
	}
	// reader-1 got no notice for its own op-ed on someone else's story; the paper did.
	if n, _ := f.svc.ListNoticesForOwner(context.Background(), "reader-1", "", 50); len(n.Items) != 0 {
		t.Fatalf("op-ed author was told of their own publish: %+v", n.Items)
	}
	paper, err := f.svc.ListNoticesForOwner(context.Background(), "paper-owner", "", 50)
	if err != nil || len(paper.Items) != 1 || paper.Items[0].Notice.Relation != "oped_on_your_story" || paper.Items[0].Notice.Kind != "parent_used" {
		t.Fatalf("paper notices = %+v %v", paper.Items, err)
	}
	// reader-2 forks and publishes the op-ed: reader-1 is told.
	opRef := PublishParentRef{PublicationVersionID: op.PublicationVersionID, ManifestHash: op.ManifestHash}
	fork, err := f.publishOped(t, "reader-2", "writer-cc22dd", "doc-fork-of-fork", 1, nil, opRef)
	if err != nil {
		t.Fatalf("publish fork of op-ed: %v", err)
	}
	told, err := f.svc.ListNoticesForOwner(context.Background(), "reader-1", "", 50)
	if err != nil || len(told.Items) != 1 {
		t.Fatalf("author notices = %+v %v", told.Items, err)
	}
	n := told.Items[0].Notice
	if n.Kind != "parent_used" || n.Relation != "fork_published" || n.Level != "major" || n.ActorLabel != "writer-cc22dd" ||
		n.Subject.PublicationVersionID != fork.PublicationVersionID || n.Subject.ParentVersionID != op.PublicationVersionID || n.Origin != "corpusd" {
		t.Fatalf("notice = %+v", n)
	}
	raw, _ := json.Marshal(told.Items[0])
	if strings.Contains(string(raw), "@") {
		t.Fatalf("a notice carries an address: %s", raw)
	}
	// Replay of the same publish writes no second notice (X18).
	if _, err := f.publishOped(t, "reader-2", "writer-cc22dd", "doc-fork-of-fork", 1, nil, opRef); err != nil {
		t.Fatal(err)
	}
	if again, _ := f.svc.ListNoticesForOwner(context.Background(), "reader-1", "", 50); len(again.Items) != 1 {
		t.Fatalf("replay minted notices: %+v", again.Items)
	}
	// A self-fork (reader-1 forks their own op-ed) is told to nobody.
	if _, err := f.publishOped(t, "reader-1", "writer-aa11bb", "doc-self-fork", 1, nil, opRef); err != nil {
		t.Fatal(err)
	}
	if again, _ := f.svc.ListNoticesForOwner(context.Background(), "reader-1", "", 50); len(again.Items) != 1 {
		t.Fatalf("self-fork notified its author: %+v", again.Items)
	}
}

// The paper's response cites the op-ed: an auto-mode version with the op-ed as a source parent.
func TestPaperResponseCitesOpedAndTellsItsAuthor(t *testing.T) {
	f := newOpedFixture(t)
	ref := f.ref()
	op, err := f.publishOped(t, "reader-1", "writer-aa11bb", "doc-fork", 1, &ref, ref)
	if err != nil {
		t.Fatal(err)
	}
	opRef := PublishParentRef{PublicationVersionID: op.PublicationVersionID, ManifestHash: op.ManifestHash}
	_, err = f.svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID: "paper-owner", SourceDocID: "doc-story", SourceRevisionID: "doc-story-rev-2", RevisionNumber: 2,
		Title: "Harbor reopens", Content: "The harbor reopens at dawn; ferry crews object.", PublishMode: PublishModeAuto,
		Parents: []PublishParentRef{opRef},
	})
	if err != nil {
		t.Fatalf("paper response: %v", err)
	}
	told, err := f.svc.ListNoticesForOwner(context.Background(), "reader-1", "", 50)
	if err != nil || len(told.Items) != 1 || told.Items[0].Notice.Relation != "paper_responded" {
		t.Fatalf("author notices = %+v %v", told.Items, err)
	}
	// A parent that does not exist refuses the paper's revision too.
	_, err = f.svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID: "paper-owner", SourceDocID: "doc-story", SourceRevisionID: "doc-story-rev-3", RevisionNumber: 3,
		Title: "Harbor reopens", Content: "Third.", PublishMode: PublishModeAuto,
		Parents: []PublishParentRef{{PublicationVersionID: "pub-nope-v1", ManifestHash: "x"}},
	})
	if !errors.Is(err, ErrInvalidParent) {
		t.Fatalf("forged parent = %v, want ErrInvalidParent", err)
	}
}

// P18: the limit counts publications, not verdicts.
func TestOpinionDailyLimitCountsPublicationsNotVerdicts(t *testing.T) {
	f := newOpedFixture(t)
	ref := f.ref()
	for i := 0; i < OpinionDailyLimit; i++ {
		doc := "doc-fork-" + string(rune('a'+i))
		op, err := f.publishOped(t, "reader-1", "writer-aa11bb", doc, 1, &ref, ref)
		if err != nil {
			t.Fatalf("op-ed %d: %v", i, err)
		}
		if _, err := f.svc.PostOpedVerdict(context.Background(), OpedVerdictPost{OwnerID: "paper-owner", VersionID: op.PublicationVersionID,
			Run: 1, Verdict: "rejected_unclear", VerdictHash: "h", TapeRecordID: "r"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.publishOped(t, "reader-1", "writer-aa11bb", "doc-over", 1, &ref, ref); !errors.Is(err, ErrOpinionDailyLimit) {
		t.Fatalf("past the limit = %v, want ErrOpinionDailyLimit", err)
	}
}
