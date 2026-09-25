package computerversion

import (
	"strings"
	"testing"
)

func TestDoltHeadSnapshotRejectsMissingCommitHashAndProductionState(t *testing.T) {
	base := DoltHeadSnapshot{
		RepoRoot:   t.TempDir(),
		Database:   "objectgraph",
		CommitHash: "dolt-commit-123",
	}
	tests := []struct {
		name    string
		mutate  func(DoltHeadSnapshot) DoltHeadSnapshot
		wantErr string
	}{
		{
			name: "missing commit hash",
			mutate: func(snapshot DoltHeadSnapshot) DoltHeadSnapshot {
				snapshot.CommitHash = "  "
				return snapshot
			},
			wantErr: "commit hash is required",
		},
		{
			name: "contains production",
			mutate: func(snapshot DoltHeadSnapshot) DoltHeadSnapshot {
				snapshot.ContainsProduction = true
				return snapshot
			},
			wantErr: "production state is not admissible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mutate(base).Validate()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}
