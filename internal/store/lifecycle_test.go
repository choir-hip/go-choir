package store

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// seedHistoricalLifecycleFixture creates pre-cutover state only for tests that
// need to read or import historical lifecycle projections. Production mutation
// APIs are deliberately not used to construct this fixture.
func seedHistoricalLifecycleFixture(t *testing.T, s *Store) types.StartLifecycleRequest {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	req := types.StartLifecycleRequest{
		OwnerID: "owner-lifecycle", ComputerID: "computer-lifecycle",
		CommandID: "historical-start", TrajectoryID: "trajectory-lifecycle-1", Kind: types.TrajectoryKindTask,
		SubjectRefs:     map[string]string{"artifact": "texture://artifact/1", "doc_id": "document-lifecycle-1"},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: "work-lifecycle-1", Objective: "produce artifact", AssignedAgentID: "texture:document-lifecycle-1"},
		InitialDocument: types.Document{DocID: "document-lifecycle-1", Title: "Lifecycle artifact"},
		InitialRevision: types.Revision{RevisionID: "revision-lifecycle-v0", AuthorKind: types.AuthorAppAgent, AuthorLabel: "Choir", Content: "Initial artifact"},
		Agent:           types.AgentRecord{AgentID: "texture:document-lifecycle-1", Profile: "texture", Role: "texture", ChannelID: "document-lifecycle-1"},
	}
	digest, err := ComputeStartLifecycleRequestDigest(req)
	if err != nil {
		t.Fatal(err)
	}
	req.StartRequestDigest = digest

	trajectory := types.TrajectoryRecord{
		TrajectoryID: req.TrajectoryID, OwnerID: req.OwnerID, ComputerID: req.ComputerID, Kind: req.Kind,
		Status: types.TrajectoryLive, SubjectRefs: req.SubjectRefs, SettlementRule: req.SettlementRule,
		LifecycleVersion: 1, ReducerSeq: 1, CreatedAt: now, UpdatedAt: now,
	}
	document := req.InitialDocument
	document.OwnerID, document.ComputerID, document.TrajectoryID = req.OwnerID, req.ComputerID, req.TrajectoryID
	document.CurrentRevisionID, document.CreatedAt, document.UpdatedAt = req.InitialRevision.RevisionID, now, now
	revision := req.InitialRevision
	revision.OwnerID, revision.ComputerID, revision.TrajectoryID, revision.DocID, revision.CreatedAt = req.OwnerID, req.ComputerID, req.TrajectoryID, document.DocID, now
	work := req.InitialWork
	work.OwnerID, work.ComputerID, work.TrajectoryID = req.OwnerID, req.ComputerID, req.TrajectoryID
	work.Status, work.LifecycleVersion, work.LastReducerSeq, work.CreatedAt, work.UpdatedAt = types.WorkItemOpen, 1, 1, now, now
	event := types.LifecycleEvent{
		EventID: req.CommandID + ":1", OwnerID: req.OwnerID, ComputerID: req.ComputerID, TrajectoryID: req.TrajectoryID,
		Kind: types.LifecycleTrajectoryStarted, ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: 1,
		CommandID: req.CommandID, CommandDigest: req.StartRequestDigest, CreatedAt: now,
	}

	objects := make([]objectgraph.Object, 0, 5)
	add := func(kind objectgraph.ObjectKind, key string, body any, metadata map[string]any) {
		obj, buildErr := lifecycleObject(kind, req.OwnerID, req.ComputerID, key, body, metadata, now, now)
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		objects = append(objects, obj)
	}
	add(ogKindTrajectory, trajectory.TrajectoryID, trajectory, lifecycleMetadata("trajectory_id", trajectory.TrajectoryID, req.ComputerID, req.TrajectoryID, 1))
	add(ogKindTexDoc, document.DocID, document, lifecycleMetadata("doc_id", document.DocID, req.ComputerID, req.TrajectoryID, 1))
	add(ogKindTexRev, revision.RevisionID, revision, lifecycleMetadata("revision_id", revision.RevisionID, req.ComputerID, req.TrajectoryID, 1))
	add(ogKindWorkItem, work.WorkItemID, work, lifecycleMetadata("work_item_id", work.WorkItemID, req.ComputerID, req.TrajectoryID, 1))
	add(ogKindLifecycleEvent, event.EventID, event, lifecycleMetadata("event_id", event.EventID, req.ComputerID, req.TrajectoryID, 1))
	if err := s.ogStore.PutBatch(ctx, objectgraph.Batch{Objects: objects}); err != nil {
		t.Fatal(err)
	}
	return req
}

func TestLegacyLifecycleWritersRefuseWithoutMutation(t *testing.T) {
	tests := []struct {
		name string
		call func(context.Context, *Store) error
	}{
		{"start", func(ctx context.Context, s *Store) error {
			_, err := s.StartLifecycle(ctx, types.StartLifecycleRequest{})
			return err
		}},
		{"open work", func(ctx context.Context, s *Store) error {
			_, err := s.OpenLifecycleWork(ctx, types.OpenLifecycleWorkRequest{})
			return err
		}},
		{"amend work", func(ctx context.Context, s *Store) error {
			_, err := s.AmendLifecycleWork(ctx, types.AmendLifecycleWorkRequest{})
			return err
		}},
		{"record refs", func(ctx context.Context, s *Store) error {
			_, err := s.RecordLifecycleRefs(ctx, types.RecordLifecycleRefsRequest{})
			return err
		}},
		{"queue update", func(ctx context.Context, s *Store) error {
			_, err := s.QueueLifecycleUpdate(ctx, types.QueueLifecycleUpdateRequest{})
			return err
		}},
		{"commit artifact head", func(ctx context.Context, s *Store) error {
			_, err := s.CommitLifecycleArtifactHead(ctx, types.CommitLifecycleArtifactHeadRequest{})
			return err
		}},
		{"apply update", func(ctx context.Context, s *Store) error {
			_, err := s.ApplyLifecycleUpdate(ctx, types.ApplyLifecycleUpdateRequest{})
			return err
		}},
		{"apply update with source graph", func(ctx context.Context, s *Store) error {
			_, err := s.ApplyLifecycleUpdateWithSourceGraph(ctx, types.ApplyLifecycleUpdateRequest{}, TextureSourceGraphWriteSet{})
			return err
		}},
		{"settle work", func(ctx context.Context, s *Store) error {
			_, err := s.SettleLifecycleWork(ctx, types.SettleLifecycleWorkRequest{})
			return err
		}},
		{"refuse work", func(ctx context.Context, s *Store) error {
			_, err := s.RefuseLifecycleWork(ctx, types.RefuseLifecycleWorkRequest{})
			return err
		}},
		{"settle trajectory", func(ctx context.Context, s *Store) error {
			_, err := s.SettleLifecycleTrajectory(ctx, types.SettleLifecycleTrajectoryRequest{})
			return err
		}},
		{"archive artifact", func(ctx context.Context, s *Store) error {
			_, err := s.ArchiveLifecycleArtifact(ctx, types.ArchiveLifecycleArtifactRequest{})
			return err
		}},
		{"cancel trajectory", func(ctx context.Context, s *Store) error {
			_, err := s.CancelLifecycleTrajectory(ctx, types.CancelLifecycleRequest{})
			return err
		}},
		{"replace activation", func(ctx context.Context, s *Store) error {
			_, err := s.ReplaceLifecycleActivation(ctx, types.ReplaceLifecycleActivationRequest{})
			return err
		}},
		{"project terminal run", func(ctx context.Context, s *Store) error {
			_, err := s.ProjectTerminalLifecycleRun(ctx, types.ReplaceLifecycleActivationRequest{})
			return err
		}},
		{"update document title", func(ctx context.Context, s *Store) error {
			_, err := s.UpdateLifecycleDocumentTitleAuthority(ctx, "", "", "", "")
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := openTestStore(t)
			fixture := seedHistoricalLifecycleFixture(t, s)
			ctx := context.Background()
			before, err := s.GetLifecycleSnapshot(ctx, fixture.OwnerID, fixture.ComputerID, fixture.TrajectoryID)
			if err != nil {
				t.Fatal(err)
			}
			beforeObjects, err := s.ogStore.ReadObjectSnapshot(ctx, fixture.OwnerID, fixture.ComputerID)
			if err != nil {
				t.Fatal(err)
			}
			if err := test.call(ctx, s); !errors.Is(err, ErrLifecycleAuthorityRequired) {
				t.Fatalf("error = %v, want ErrLifecycleAuthorityRequired", err)
			}
			after, err := s.GetLifecycleSnapshot(ctx, fixture.OwnerID, fixture.ComputerID, fixture.TrajectoryID)
			if err != nil {
				t.Fatal(err)
			}
			afterObjects, err := s.ogStore.ReadObjectSnapshot(ctx, fixture.OwnerID, fixture.ComputerID)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(after, before) || !reflect.DeepEqual(afterObjects, beforeObjects) {
				t.Fatal("legacy lifecycle writer changed historical object, event, or watermark state")
			}
		})
	}
}
