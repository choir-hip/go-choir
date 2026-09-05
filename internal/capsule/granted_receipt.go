package capsule

// grantedExecutionBindReason names the first field that prevents freeze/grant
// from certifying an execution receipt against the exact frozen assignment.
// Empty means the receipt binds.
func grantedExecutionBindReason(receipt ExecutionReceipt, agentRunID, handleDigest, capsuleID, worktreeDigest, sourceDigest string) string {
	switch {
	case receipt.AgentRunID != agentRunID:
		return "run"
	case receipt.CapabilityHandleDigest != handleDigest:
		return "handle"
	case receipt.CapsuleID != capsuleID:
		return "capsule"
	case receipt.ExitCode != 0:
		return "exit"
	case receipt.WorktreeDigest != worktreeDigest:
		return "final subject"
	case receipt.SourceTreeDigest != sourceDigest:
		return "frozen source"
	default:
		return ""
	}
}
