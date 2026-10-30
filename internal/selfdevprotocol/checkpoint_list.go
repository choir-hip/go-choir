package selfdevprotocol

// The read-only listing of a computer's checkpoints
// (docs/archive/design/apply-checkpoint-without-replay-2026-10-26.md, 4.4 and slice
// 3). Checkpoints are bound automatically; this is how an owner or the
// cadence sees them, with the host's verdict on each.

const (
	// CheckpointClassVerifier: bound by a self-development apply.
	CheckpointClassVerifier = "verifier"
	// CheckpointClassOwnerRecovery: bound on the cadence (or, before slice 3,
	// by the owner's bind).
	CheckpointClassOwnerRecovery = "owner_recovery"
	// CheckpointClassPlatformFollow: bound by a platform update.
	CheckpointClassPlatformFollow = "platform_follow"
)

// CheckpointSummary is one row of a computer's checkpoint listing.
type CheckpointSummary struct {
	CheckpointDigest string `json:"checkpoint_digest"`
	// AcceptedSequence is the event sequence of AcceptedHead; zero when the
	// platform cannot place the head.
	AcceptedSequence uint64 `json:"accepted_sequence"`
	AcceptedHead     string `json:"accepted_head"`
	ReleaseDigest    string `json:"release_digest"`
	EvidenceClass    string `json:"evidence_class"`
	CreatedAt        string `json:"created_at"`
	// Verdict is the host's verdict. A checkpoint published before the host
	// judged checkpoints has status "unjudged": no verdict row exists.
	Verdict CheckpointVerdict `json:"verdict"`
}

// CheckpointVerdictUnjudged marks a listed checkpoint with no verdict row.
const CheckpointVerdictUnjudged = "unjudged"

// CheckpointList is the listing answer.
type CheckpointList struct {
	Checkpoints []CheckpointSummary `json:"checkpoints"`
}
