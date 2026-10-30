// Package calibration is calibration level 2: any agent writes calibrated
// claims (precommitments) into a Choir computer through an API call, scorers
// append settlements, and the computer returns a calibration summary the
// agent pastes back into its context — in-context reinforcement learning
// from its own settled track record (owner notes 2026-10-14 §1, §3).
//
// Design, failure modes and ceremony:
// docs/archive/reports/choir-mcp-precommitments-2026-10-14.md. Level 1 is the
// Markdown ledger docs/calibration/claude-ledger.md.
//
// Records reuse choir.commitment_record and its existing record kinds: a
// claim is a precommit, a settlement is a resolve linked to the claim by
// ParentID, and both carry a types.CommitmentCalibration body. Nothing new
// is registered in the object graph. Claims and settlements are append-only:
// there is no edit route, a replayed write with the same body is a no-op,
// and a different body under the same identity is refused.
//
// The routes are served by the guest runtime under /api/precommit/ and
// reached through the proxy with the precommit:read and precommit:write API
// key scopes. The arithmetic (Brier, strict Brier, calibration bins, the
// overconfidence index, the self-score gap) is pure and lives in math.go.
package calibration
