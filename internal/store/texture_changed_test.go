package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// What the root desk reads: the documents that changed after a time, found
// through the document's object or its trajectory's, without stories and
// without archived documents. Ways it can fail, written before it: a story
// takes a slot of the bound; a ledger-only change (a settled work item
// leaves the document's head alone) is missed; the order is not oldest
// change first; a document found twice is listed twice.
func changedFixture(name string, kind types.TrajectoryKind) types.StartLifecycleRequest {
	req := lifecycleStartFixture()
	req.CommandID = "command-" + name
	req.TrajectoryID = "trajectory-" + name
	req.Kind = kind
	req.SubjectRefs = map[string]string{"artifact": "texture://artifact/" + name, "doc_id": "document-" + name}
	req.InitialWork = types.WorkItemRecord{WorkItemID: "work-" + name, Objective: "produce artifact"}
	req.InitialDocument = types.Document{DocID: "document-" + name, Title: "Doc " + name}
	req.InitialRevision = types.Revision{
		RevisionID: "revision-" + name + "-v0", AuthorKind: types.AuthorAppAgent, AuthorLabel: "Choir", Content: "Initial " + name,
		BodyDoc: lifecycleStructuredBodyDoc("document-"+name, "revision-"+name+"-v0", "Initial "+name),
	}
	req.Agent = types.AgentRecord{AgentID: "texture:document-" + name, Profile: "texture", Role: "texture", ChannelID: "document-" + name}
	req.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(req)
	return req
}

func TestChangedTextureDocumentsSkipStoriesAndFindLedgerOnlyChanges(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	const owner, computer = "owner-lifecycle", "computer-lifecycle"
	for _, name := range []string{"a", "b"} {
		if _, err := s.StartLifecycle(ctx, changedFixture(name, types.TrajectoryKindTask)); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond)
	}
	for i := 0; i < 3; i++ {
		if _, err := s.StartLifecycle(ctx, changedFixture(fmt.Sprintf("story-%d", i), types.TrajectoryKindStory)); err != nil {
			t.Fatal(err)
		}
	}
	got, more, err := s.ListTextureDocumentsChangedSince(ctx, owner, computer, time.Time{})
	if err != nil || more {
		t.Fatalf("list: more=%v err=%v", more, err)
	}
	if len(got) != 2 || got[0].Doc.DocID != "document-a" || got[1].Doc.DocID != "document-b" {
		var names []string
		for _, c := range got {
			names = append(names, c.Doc.DocID)
		}
		t.Fatalf("changed = %v, want document-a then document-b (oldest first, no stories)", names)
	}
	if got[0].Trajectory.TrajectoryID != "trajectory-a" || got[0].ChangedAt.IsZero() {
		t.Fatalf("entry = %+v, want its trajectory and change time", got[0])
	}

	// A ledger-only change: settling the work item changes the trajectory and
	// the work item, and not the document's head.
	cut := time.Now().UTC().Add(time.Millisecond)
	time.Sleep(3 * time.Millisecond)
	settleInitialLifecycleWork(t, s, changedFixture("a", types.TrajectoryKindTask), "settle-a")
	after, _, err := s.ListTextureDocumentsChangedSince(ctx, owner, computer, cut)
	if err != nil || len(after) != 1 || after[0].Doc.DocID != "document-a" {
		t.Fatalf("after a ledger-only change on a: %+v, %v; want exactly document-a", after, err)
	}
	if none, _, err := s.ListTextureDocumentsChangedSince(ctx, owner, computer, time.Now().UTC().Add(time.Hour)); err != nil || len(none) != 0 {
		t.Fatalf("since the future: %d, %v; want none", len(none), err)
	}
}

// A document object that cannot be decoded is named in the list with its
// error and change time, not skipped: the caller holds its watermark behind
// it.
func TestChangedTextureDocumentsNameAnUndecodableDocument(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	const owner, computer = "owner-lifecycle", "computer-lifecycle"
	writer, ok := objectgraph.CustodianOf(ogKindTexDoc)
	if !ok {
		t.Fatal("no custodian for texture documents")
	}
	if _, err := s.og.CreateObject(objectgraph.WithWriter(ctx, writer), objectgraph.CreateObjectRequest{
		Kind: ogKindTexDoc, OwnerID: owner, ComputerID: computer, IdentityKey: "undecodable", Body: []byte(`"not a document"`), Metadata: map[string]any{"doc_id": "undecodable"}, Now: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("write the bad object: %v", err)
	}
	got, _, err := s.ListTextureDocumentsChangedSince(ctx, owner, computer, time.Time{})
	if err != nil || len(got) != 1 || got[0].Err == nil || got[0].ChangedAt.IsZero() || got[0].ObjectID == "" {
		t.Fatalf("changed = %+v, %v; want the undecodable document named with its error and time", got, err)
	}
}
