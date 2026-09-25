package store

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestDesktopStateIsolation(t *testing.T) {
	db := openTestStore(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	now := time.Now().UTC()

	state1 := types.DesktopState{
		OwnerID: "user-1",
		Windows: []types.WindowState{
			{WindowID: "win-a", AppID: "texture", Title: "User 1 Doc", Geometry: types.WindowGeometry{X: 10, Y: 10, Width: 400, Height: 300}, Mode: types.WindowNormal, ZIndex: 1, CreatedAt: now, UpdatedAt: now},
		},
		ActiveWindowID: "win-a",
		UpdatedAt:      now,
	}
	if err := db.SaveDesktopState(ctx, state1); err != nil {
		t.Fatalf("SaveDesktopState user-1: %v", err)
	}

	state2 := types.DesktopState{
		OwnerID: "user-2",
		Windows: []types.WindowState{
			{WindowID: "win-b", AppID: "terminal", Title: "User 2 Terminal", Geometry: types.WindowGeometry{X: 20, Y: 20, Width: 500, Height: 400}, Mode: types.WindowNormal, ZIndex: 1, CreatedAt: now, UpdatedAt: now},
		},
		ActiveWindowID: "win-b",
		UpdatedAt:      now,
	}
	if err := db.SaveDesktopState(ctx, state2); err != nil {
		t.Fatalf("SaveDesktopState user-2: %v", err)
	}

	got1, err := db.GetDesktopState(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetDesktopState user-1: %v", err)
	}
	if len(got1.Windows) != 1 || got1.Windows[0].AppID != "texture" {
		t.Errorf("user-1 desktop state was affected by user-2 save")
	}

	got2, err := db.GetDesktopState(ctx, "user-2")
	if err != nil {
		t.Fatalf("GetDesktopState user-2: %v", err)
	}
	if len(got2.Windows) != 1 || got2.Windows[0].AppID != "terminal" {
		t.Errorf("user-2 desktop state incorrect")
	}
}

func TestDesktopStateMultipleDesktopsPerUser(t *testing.T) {
	db := openTestStore(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	now := time.Now().UTC()

	primary := types.DesktopState{
		OwnerID:   "user-1",
		DesktopID: types.PrimaryDesktopID,
		Windows: []types.WindowState{
			{WindowID: "win-primary", AppID: "texture", Title: "Primary", Geometry: types.WindowGeometry{X: 10, Y: 10, Width: 400, Height: 300}, Mode: types.WindowNormal, ZIndex: 1, CreatedAt: now, UpdatedAt: now},
		},
		ActiveWindowID: "win-primary",
		UpdatedAt:      now,
	}
	branch := types.DesktopState{
		OwnerID:   "user-1",
		DesktopID: "branch-a",
		Windows: []types.WindowState{
			{WindowID: "win-branch", AppID: "terminal", Title: "Branch", Geometry: types.WindowGeometry{X: 20, Y: 20, Width: 500, Height: 400}, Mode: types.WindowNormal, ZIndex: 1, CreatedAt: now, UpdatedAt: now},
		},
		ActiveWindowID: "win-branch",
		UpdatedAt:      now,
	}

	if err := db.SaveDesktopStateForDesktop(ctx, primary); err != nil {
		t.Fatalf("SaveDesktopStateForDesktop primary: %v", err)
	}
	if err := db.SaveDesktopStateForDesktop(ctx, branch); err != nil {
		t.Fatalf("SaveDesktopStateForDesktop branch: %v", err)
	}

	gotPrimary, err := db.GetDesktopStateForDesktop(ctx, "user-1", types.PrimaryDesktopID)
	if err != nil {
		t.Fatalf("GetDesktopStateForDesktop primary: %v", err)
	}
	gotBranch, err := db.GetDesktopStateForDesktop(ctx, "user-1", "branch-a")
	if err != nil {
		t.Fatalf("GetDesktopStateForDesktop branch: %v", err)
	}

	if len(gotPrimary.Windows) != 1 || gotPrimary.Windows[0].WindowID != "win-primary" {
		t.Fatalf("primary desktop mismatch: %+v", gotPrimary.Windows)
	}
	if len(gotBranch.Windows) != 1 || gotBranch.Windows[0].WindowID != "win-branch" {
		t.Fatalf("branch desktop mismatch: %+v", gotBranch.Windows)
	}
	if gotPrimary.DesktopID != types.PrimaryDesktopID {
		t.Errorf("primary DesktopID = %q, want %q", gotPrimary.DesktopID, types.PrimaryDesktopID)
	}
	if gotBranch.DesktopID != "branch-a" {
		t.Errorf("branch DesktopID = %q, want %q", gotBranch.DesktopID, "branch-a")
	}
}
