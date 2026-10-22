package agentcore

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// TestRunAcceptanceTwoDeskTrajectoryWritesAfterCutover drives the staging
// failure: a Texture trajectory that opened research yields the authority
// path "texture > research", and the acceptance write on a store past the
// vocabulary cutover was refused by the write guard
// (docs/problems/run-acceptance-refused-by-vocab-fence-on-multi-desk-trajectory-2026-10-22.md).
func TestRunAcceptanceTwoDeskTrajectoryWritesAfterCutover(t *testing.T) {
	ctx := context.Background()
	st, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.MigrateVocabularyToV2(ctx); err != nil {
		t.Fatalf("cutover: %v", err)
	}
	runs := []types.RunRecord{
		{RunID: "run-texture", AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture},
		{RunID: "run-research", AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research},
	}
	profile := acceptanceAuthorityProfile(runs)
	if profile != "texture > research" {
		t.Fatalf("authority profile = %q, want the two-desk path", profile)
	}
	rec, err := st.UpsertRunAcceptance(ctx, types.RunAcceptanceRecord{
		OwnerID: "owner-1", TargetMissionID: "mission-1", TrajectoryID: "traj-1",
		AuthorityProfile: profile, State: types.RunAcceptanceBlocked,
	})
	if err != nil {
		t.Fatalf("two-desk acceptance refused after cutover: %v", err)
	}
	got, err := st.GetRunAcceptance(ctx, "owner-1", rec.AcceptanceID)
	if err != nil {
		t.Fatal(err)
	}
	if got.AuthorityProfile != profile {
		t.Fatalf("stored authority profile = %q, want %q", got.AuthorityProfile, profile)
	}
	if _, err := st.UpsertRunAcceptance(ctx, types.RunAcceptanceRecord{
		OwnerID: "owner-1", TargetMissionID: "mission-1", TrajectoryID: "traj-2",
		AuthorityProfile: "texture > researcher",
	}); err == nil {
		t.Fatal("acceptance with a retired role in its path was written after cutover")
	}
}
