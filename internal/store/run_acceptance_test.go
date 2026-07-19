package store

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestRunAcceptanceStoresOwnerScopedSynthesizedRecord(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	rec, err := s.UpsertRunAcceptance(ctx, types.RunAcceptanceRecord{
		AcceptanceID:          "acceptance-1",
		TargetMissionID:       "mission-run-acceptance-v0",
		SourcePromptObjective: "Build Choir in Choir",
		OwnerID:               "owner-1",
		DesktopID:             "primary",
		TrajectoryID:          "trajectory-1",
		RunID:                 "run-1",
		AuthorityProfile:      "conductor > texture > super > co-super",
		BaseSHA:               "base-sha",
		DeploymentCommit:      "deploy-sha",
		HealthCommit:          "deploy-sha",
		AcceptanceLevel:       types.RunAcceptanceExportLevel,
		VMMode:                "capsule",
		State:                 types.RunAcceptanceAccepted,
		Checkpoints: []types.RunAcceptanceCheckpoint{{
			Kind:  "submitted",
			State: "passed",
		}},
		InvariantChecks: []types.RunAcceptanceInvariantCheck{{
			Name:  "checkpoint_causal_order",
			State: "passed",
		}},
		VerifierContracts: []types.RunAcceptanceVerifierContract{{
			Name:  "trace-derived-state-machine",
			State: "passed",
		}},
		EvidenceRefs: []types.RunAcceptanceEvidenceRef{{
			RefID:   "event:event-1",
			Kind:    "tool.result",
			Summary: "worker export",
		}},
		RollbackRefs: []types.RunAcceptanceRollbackRef{{
			Kind: "git_base",
			Ref:  "base-sha",
		}},
		FailureResidualRisks: []string{"promotion-level not yet proven"},
	})
	if err != nil {
		t.Fatalf("upsert run acceptance: %v", err)
	}
	if rec.UpdatedAt.IsZero() || rec.CreatedAt.IsZero() {
		t.Fatalf("timestamps not assigned: %+v", rec)
	}

	loaded, err := s.GetRunAcceptance(ctx, "owner-1", "acceptance-1")
	if err != nil {
		t.Fatalf("get run acceptance: %v", err)
	}
	if loaded.AcceptanceLevel != types.RunAcceptanceExportLevel || loaded.State != types.RunAcceptanceAccepted {
		t.Fatalf("loaded acceptance mismatch: %+v", loaded)
	}
	if len(loaded.Checkpoints) != 1 || loaded.Checkpoints[0].Kind != "submitted" {
		t.Fatalf("checkpoint mismatch: %+v", loaded.Checkpoints)
	}
	if _, err := s.GetRunAcceptance(ctx, "owner-2", "acceptance-1"); err != ErrNotFound {
		t.Fatalf("other owner get error = %v, want ErrNotFound", err)
	}

	loaded.State = types.RunAcceptanceBlocked
	loaded.AcceptanceLevel = types.RunAcceptanceStagingSmokeLevel
	loaded.Checkpoints = append(loaded.Checkpoints, types.RunAcceptanceCheckpoint{Kind: "worker_leased", State: "blocked"})
	updated, err := s.UpsertRunAcceptance(ctx, loaded)
	if err != nil {
		t.Fatalf("update run acceptance: %v", err)
	}
	if updated.UpdatedAt.Before(updated.CreatedAt) {
		t.Fatalf("updated timestamp moved backwards: %+v", updated)
	}

	byTrajectory, err := s.ListRunAcceptancesByTrajectory(ctx, "owner-1", "trajectory-1", 10)
	if err != nil {
		t.Fatalf("list by trajectory: %v", err)
	}
	if len(byTrajectory) != 1 || byTrajectory[0].State != types.RunAcceptanceBlocked {
		t.Fatalf("list by trajectory mismatch: %+v", byTrajectory)
	}

	all, err := s.ListRunAcceptances(ctx, "owner-1", 10)
	if err != nil {
		t.Fatalf("list run acceptances: %v", err)
	}
	if len(all) != 1 || all[0].AcceptanceID != "acceptance-1" {
		t.Fatalf("list mismatch: %+v", all)
	}
}
