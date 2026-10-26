package selfdevprotocol

import "sort"

// The host's verdict on a published checkpoint
// (docs/design/apply-checkpoint-without-replay-2026-10-26.md, 4.1 "The proof"
// and residual R1). The guest binds a checkpoint from its live store without
// replaying; the host (checkpointd) rebuilds the projection at the
// checkpoint's head from the tape alone and compares the two witnesses.

const (
	// CheckpointVerdictPending: published, not yet judged by the host.
	CheckpointVerdictPending = "pending"
	// CheckpointVerdictVerified: the host's tape-only rebuild at the
	// checkpoint's head has the checkpoint's witness.
	CheckpointVerdictVerified = "verified"
	// CheckpointVerdictNotDerivable: the rebuild differs and the two were
	// built in the same deposit mode, so the difference is rows the tape
	// cannot reproduce. Restore refuses it.
	CheckpointVerdictNotDerivable = "not_derivable"
	// CheckpointVerdictUnverified: the host did not or could not judge. It is
	// not a refusal: restore's own reconstruction gate stays the enforcement.
	CheckpointVerdictUnverified = "unverified"
)

const (
	// VerdictReasonWitnessMismatch accompanies not_derivable.
	VerdictReasonWitnessMismatch = "witness_mismatch"
	// VerdictReasonBaseUpcast is residual R1: the host rebuild always upcasts
	// recorded deposits (the V1 spellings become V2), while the live store the
	// checkpoint was bound from kept its recorded bytes (retained), or did not
	// say. A differing witness then can be the deposit mode, not a row the
	// tape cannot rebuild: restore replays in the live store's deposit mode
	// (matchLiveDepositMode) and may well succeed. The host says "cannot
	// prove", never "not derivable".
	VerdictReasonBaseUpcast = "base_upcast"
	// VerdictReasonNoBase: no base at or below the checkpoint's head exists
	// to start the rebuild from, and genesis is beyond the replay bound.
	VerdictReasonNoBase = "no_base"
	// VerdictReasonJobBlocked: the rebuild job for the head was refused.
	VerdictReasonJobBlocked = "job_blocked"
	// VerdictReasonInvalidWitness: a witness did not validate.
	VerdictReasonInvalidWitness = "invalid_witness"
	// VerdictReasonBaseHeadMismatch: the base the host holds at the
	// checkpoint's sequence is at another head.
	VerdictReasonBaseHeadMismatch = "base_head_mismatch"
)

const (
	// DepositModeRetained: the live store has no upcast ledger; it kept the
	// bytes its writers recorded.
	DepositModeRetained = "retained"
	// DepositModeUpcast: the live store's deposits are upcast to the serving
	// vocabulary, as every host rebuild's are.
	DepositModeUpcast = "upcast"
)

// CheckpointVerdict is the host's judgment of one checkpoint.
type CheckpointVerdict struct {
	Status           string   `json:"status"`
	Reason           string   `json:"reason,omitempty"`
	DifferingTables  []string `json:"differing_tables,omitempty"`
	BaseRef          string   `json:"base_ref,omitempty"`
	AcceptedSequence uint64   `json:"accepted_sequence,omitempty"`
	Detail           string   `json:"detail,omitempty"`
	UpdatedAt        string   `json:"updated_at,omitempty"`
}

// maxVerdictTables bounds the table names a verdict carries; a store with
// more differing tables than this is not derivable by any reading.
const maxVerdictTables = 32

// JudgeCheckpointWitness compares the witness a checkpoint carries with the
// witness of the host's tape-only rebuild at the same head.
//
// Equal content is verified, whatever the deposit mode: the rebuild produced
// the checkpoint's bytes from the tape. A difference is not_derivable only
// when both were built in the same deposit mode (the checkpoint's live store
// was upcast, as every host rebuild is). Against a retained live store, or one
// that did not say, a difference may be the deposit mode alone (residual R1),
// so the host reports unverified / base_upcast and leaves the decision to
// restore's own reconstruction, which replays in the live store's mode. The
// Dolt HEAD is an audit receipt and never decides.
func JudgeCheckpointWitness(depositMode string, checkpoint, rebuilt VMLocalContentWitness) CheckpointVerdict {
	if err := checkpoint.Validate(); err != nil {
		return CheckpointVerdict{Status: CheckpointVerdictUnverified, Reason: VerdictReasonInvalidWitness, Detail: "checkpoint witness: " + err.Error()}
	}
	if err := rebuilt.Validate(); err != nil {
		return CheckpointVerdict{Status: CheckpointVerdictUnverified, Reason: VerdictReasonInvalidWitness, Detail: "rebuilt witness: " + err.Error()}
	}
	if err := WitnessContentMatches(rebuilt, checkpoint); err == nil {
		return CheckpointVerdict{Status: CheckpointVerdictVerified}
	} else {
		tables := differingTables(checkpoint, rebuilt)
		verdict := CheckpointVerdict{DifferingTables: tables, Detail: err.Error()}
		if depositMode == DepositModeUpcast {
			verdict.Status, verdict.Reason = CheckpointVerdictNotDerivable, VerdictReasonWitnessMismatch
		} else {
			verdict.Status, verdict.Reason = CheckpointVerdictUnverified, VerdictReasonBaseUpcast
		}
		return verdict
	}
}

// differingTables names the tables whose schema or content hash differs
// between two witnesses, or that only one of them has, sorted and bounded.
func differingTables(a, b VMLocalContentWitness) []string {
	seen := map[string]struct{}{}
	for table := range a.Tables {
		seen[table] = struct{}{}
	}
	for table := range b.Tables {
		seen[table] = struct{}{}
	}
	var out []string
	for table := range seen {
		if a.Tables[table] != b.Tables[table] || a.Schema[table] != b.Schema[table] {
			out = append(out, table)
		}
	}
	sort.Strings(out)
	if len(out) > maxVerdictTables {
		out = out[:maxVerdictTables]
	}
	return out
}

// SealWitness returns w with its derivability digest computed from its
// content, the digest a bind or a rebuild gives it.
func SealWitness(w VMLocalContentWitness) (VMLocalContentWitness, error) {
	digest, err := witnessDerivabilityDigest(w)
	if err != nil {
		return VMLocalContentWitness{}, err
	}
	w.DerivabilityDigest = digest
	return w, nil
}
