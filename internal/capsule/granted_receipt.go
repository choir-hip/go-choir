package capsule

// grantedExecutionBindReason names the first field that prevents freeze/grant
// from certifying an execution receipt against the exact frozen assignment.
// Empty means the receipt binds. Only authenticity binds per receipt: the
// run, capability handle, capsule, and frozen source snapshot. The exit code
// is the recorded observation of that command, and an intermediate receipt's
// WorktreeDigest is its own post-evaluation subject, not the final frozen
// one — a multi-cell assignment's earlier cells necessarily recorded earlier
// tree states. The final subject is certified once, by the chronologically
// latest receipt, at the resolver.
func grantedExecutionBindReason(receipt ExecutionReceipt, agentRunID, handleDigest, capsuleID, sourceDigest string) string {
	switch {
	case receipt.AgentRunID != agentRunID:
		return "run"
	case receipt.CapabilityHandleDigest != handleDigest:
		return "handle"
	case receipt.CapsuleID != capsuleID:
		return "capsule"
	case receipt.SourceTreeDigest != sourceDigest:
		return "frozen source"
	default:
		return ""
	}
}
