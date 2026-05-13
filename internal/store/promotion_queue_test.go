package store

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestPromotionCandidateQueueStoresOwnerScopedRecords(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	queued, err := s.UpsertPromotionCandidate(ctx, types.PromotionCandidateRecord{
		OwnerID:           "owner-1",
		Status:            types.PromotionCandidateQueued,
		SourceRunID:       "run-1",
		TraceID:           "trace-1",
		VMID:              "vm-1",
		BaseSHA:           "base",
		WorkerHeadSHA:     "worker-head",
		ManifestPath:      "/tmp/manifest.json",
		PatchsetPath:      "/tmp/patch.diff",
		DestinationBranch: "main",
		Summary:           "launcher/uploads/themes candidate",
		CandidateJSON:     json.RawMessage(`{"purpose":"dogfood"}`),
		ContractsJSON:     json.RawMessage(`[{"name":"unit proof"}]`),
	})
	if err != nil {
		t.Fatalf("upsert queued: %v", err)
	}
	if queued.CandidateID == "" {
		t.Fatalf("candidate id was not assigned")
	}

	loaded, err := s.GetPromotionCandidate(ctx, "owner-1", queued.CandidateID)
	if err != nil {
		t.Fatalf("get queued: %v", err)
	}
	if loaded.Status != types.PromotionCandidateQueued || loaded.SourceRunID != "run-1" {
		t.Fatalf("loaded candidate mismatch: %+v", loaded)
	}
	if string(loaded.CandidateJSON) != `{"purpose":"dogfood"}` {
		t.Fatalf("candidate json mismatch: %s", loaded.CandidateJSON)
	}
	if _, err := s.GetPromotionCandidate(ctx, "owner-2", queued.CandidateID); err != ErrNotFound {
		t.Fatalf("other owner get error = %v, want ErrNotFound", err)
	}

	loaded.Status = types.PromotionCandidateVerified
	loaded.IntegrationBranch = "integration/candidate"
	loaded.ReportJSON = json.RawMessage(`{"status":"verified"}`)
	updated, err := s.UpdatePromotionCandidate(ctx, loaded)
	if err != nil {
		t.Fatalf("update verified: %v", err)
	}
	if updated.UpdatedAt.Before(updated.CreatedAt) {
		t.Fatalf("updated timestamp moved backwards: %+v", updated)
	}

	candidates, err := s.ListPromotionCandidates(ctx, "owner-1", 10)
	if err != nil {
		t.Fatalf("list candidates: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Status != types.PromotionCandidateVerified {
		t.Fatalf("list candidates mismatch: %+v", candidates)
	}
}
