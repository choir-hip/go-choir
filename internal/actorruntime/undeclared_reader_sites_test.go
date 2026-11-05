package actorruntime

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actor"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// d@e 14b71687 (deployed d16b911e7) counted one choir.texture_revision
// undeclared_reader at handleOwnerRevision. The actor handler loads the
// revision to route the occurrence: platform bookkeeping, read as the kernel.
func TestOwnerRevisionRoutingDeclaresItsReader(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "owner-revision.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	adapter := New(provideriface.Config{
		ComputerID: "autoputer-test", StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts"),
		ProviderTimeout: time.Second,
	}, s, events.NewEventBus(), provider.NewStubProvider(0), nil, WithKernelMode())
	t.Cleanup(adapter.cleanupLog)

	const ownerID, docID, trajectoryID = "owner-rev", "doc-rev", "traj-rev"
	now := time.Now().UTC()
	agentID := "texture:" + docID
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: "autoputer-test", CommandID: "start-rev",
		TrajectoryID: trajectoryID, Kind: types.TrajectoryKindDocument,
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: "work-rev", Objective: "route", AssignedAgentID: agentID},
		InitialDocument: types.Document{DocID: docID, OwnerID: ownerID, ComputerID: "autoputer-test", TrajectoryID: trajectoryID, Title: "rev", CreatedAt: now, UpdatedAt: now},
		InitialRevision: types.Revision{RevisionID: "revision-base", DocID: docID, OwnerID: ownerID, ComputerID: "autoputer-test", TrajectoryID: trajectoryID, AuthorKind: types.AuthorUser, AuthorLabel: ownerID, Content: "x", CreatedAt: now},
		Agent:           types.AgentRecord{AgentID: agentID, OwnerID: ownerID, ComputerID: "autoputer-test", Profile: "texture", Role: "texture", ChannelID: docID, CreatedAt: now, UpdatedAt: now},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(context.Background(), start); err != nil {
		t.Fatal(err)
	}
	mailbox := scopedActorMailboxID(ownerID, "autoputer-test", agentID)
	u := actor.Update{
		UpdateID: "rev-1", ToAgentID: mailbox, Kind: "owner_revision", TrajectoryID: "traj",
		Content: `{"revision_id":"revision-base","request_id":"r","lifecycle_version":1,"reducer_seq":1}`,
	}
	objectgraph.ResetUndeclaredReads()
	objectgraph.ResetReaderGateCounts()
	stop := objectgraph.TrackUndeclaredReadsFromProduct()
	t.Cleanup(stop)
	handler := newActorHandler(adapter.Runtime, nil)
	if _, err := handler.handleOwnerRevision(context.Background(), u, nil); err != nil {
		t.Fatal(err)
	}
	for key, n := range objectgraph.UndeclaredReads() {
		if strings.HasPrefix(key, "choir.texture_") {
			t.Errorf("undeclared read: %s x%d", key, n)
		}
	}
	for key, n := range objectgraph.ReaderGateCounts() {
		if strings.HasPrefix(key, "choir.texture_") {
			t.Errorf("reader gate violation: %s x%d", key, n)
		}
	}
}
