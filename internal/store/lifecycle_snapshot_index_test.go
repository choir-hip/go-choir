package store

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// docs/problems/research-more-slow-and-hidden-2026-10-21.md, E6:
// GetLifecycleSnapshot read the metadata of every lifecycle object, document
// and revision of the computer (416 ms per call on the paper). It now reads
// the trajectory's rows through the trajectory index and the bound document
// and revisions by key. Failure modes:
//  1. the snapshot differs from the scan's for any trajectory (a row lost or
//     added, another order, another document or head);
//  2. it still scans;
//  3. a missing trajectory no longer reads ErrNotFound.

func TestLifecycleSnapshotByIndexEqualsTheScan(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	var reqs []types.StartLifecycleRequest
	for n := 0; n < 4; n++ {
		req := lifecycleStartFixture()
		suffix := fmt.Sprintf("-idx-%d", n)
		docID := "document-lifecycle" + suffix
		req.CommandID, req.TrajectoryID = "command-start"+suffix, "trajectory-lifecycle"+suffix
		req.InitialWork.WorkItemID = "work-lifecycle" + suffix
		req.SubjectRefs = map[string]string{"artifact": "texture://artifact" + suffix, "doc_id": docID}
		req.InitialDocument.DocID = docID
		req.InitialRevision.RevisionID = "revision-lifecycle" + suffix
		req.InitialRevision.BodyDoc = lifecycleStructuredBodyDoc(docID, req.InitialRevision.RevisionID, "Initial artifact")
		req.Agent.AgentID, req.Agent.ChannelID = "texture:"+docID, docID
		req.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(req)
		if _, err := s.StartLifecycle(ctx, req); err != nil {
			t.Fatalf("start lifecycle %d: %v", n, err)
		}
		reqs = append(reqs, req)
	}
	graph := s.ogReadStore
	if graph == nil {
		graph = s.ogStore
	}
	for _, req := range reqs {
		scansBefore := s.EngineMutexStats()["(*DoltStore).ReadObjectSnapshotFiltered"].Calls
		got, err := s.GetLifecycleSnapshot(ctx, req.OwnerID, req.ComputerID, req.TrajectoryID)
		if err != nil {
			t.Fatalf("snapshot %s: %v", req.TrajectoryID, err)
		}
		if scans := s.EngineMutexStats()["(*DoltStore).ReadObjectSnapshotFiltered"].Calls - scansBefore; scans != 0 {
			t.Fatalf("snapshot %s scanned %d times", req.TrajectoryID, scans)
		}
		objects, err := s.scanLifecycleSnapshotObjects(ctx, graph, req.OwnerID, req.ComputerID, req.TrajectoryID, req.InitialDocument.DocID)
		if err != nil {
			t.Fatal(err)
		}
		want, err := assembleLifecycleSnapshot(objects, req.TrajectoryID)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("snapshot %s by index differs from the scan:\n got %+v\nwant %+v", req.TrajectoryID, got, want)
		}
		if len(got.Events) == 0 || got.Document.DocID != req.InitialDocument.DocID {
			t.Fatalf("snapshot %s has %d events, doc %q: the fixture proves nothing", req.TrajectoryID, len(got.Events), got.Document.DocID)
		}
	}
	if _, err := s.GetLifecycleSnapshot(ctx, reqs[0].OwnerID, reqs[0].ComputerID, "trajectory-missing"); err == nil {
		t.Fatal("missing trajectory read a snapshot")
	}
}
