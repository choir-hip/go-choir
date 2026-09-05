package capsule

import "testing"

func TestGrantedExecutionBindReasonNamesExactMismatch(t *testing.T) {
	ok := ExecutionReceipt{
		AgentRunID:             "run",
		CapabilityHandleDigest: "handle",
		CapsuleID:              "capsule",
		ExitCode:               0,
		WorktreeDigest:         "worktree",
		SourceTreeDigest:       "source",
	}
	if reason := grantedExecutionBindReason(ok, "run", "handle", "capsule", "worktree", "source"); reason != "" {
		t.Fatalf("matching receipt reason = %q", reason)
	}

	cases := []struct {
		name   string
		mutate func(*ExecutionReceipt)
		want   string
	}{
		{name: "run", mutate: func(r *ExecutionReceipt) { r.AgentRunID = "other" }, want: "run"},
		{name: "handle", mutate: func(r *ExecutionReceipt) { r.CapabilityHandleDigest = "other" }, want: "handle"},
		{name: "capsule", mutate: func(r *ExecutionReceipt) { r.CapsuleID = "other" }, want: "capsule"},
		{name: "exit", mutate: func(r *ExecutionReceipt) { r.ExitCode = 1 }, want: "exit"},
		{name: "final subject", mutate: func(r *ExecutionReceipt) { r.WorktreeDigest = "pre-eval" }, want: "final subject"},
		{name: "frozen source", mutate: func(r *ExecutionReceipt) { r.SourceTreeDigest = "other" }, want: "frozen source"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			receipt := ok
			tc.mutate(&receipt)
			got := grantedExecutionBindReason(receipt, "run", "handle", "capsule", "worktree", "source")
			if got != tc.want {
				t.Fatalf("reason = %q, want %q", got, tc.want)
			}
		})
	}
}
