# Settlement Gate Item 3 — Code-Free Define Receipt (2026-09-09)

Mutation class: green (docs only). No source changed in this receipt.
Mission: `docs/definitions/choir-rlm-settlement-gate-2026-09-09.md`, acceptance item 3.
Prior: item-2 repair `a6b898f0`. Worktree: 4 untracked unrelated-WIP paths preserved.

## Problem (observed, not inferred)

Terminal identity is contaminated at two layers:

1. **Agentcore mint** (`internal/agentcore/cosuper_assignment_fate.go:568-592`):
   `ReportID = "report:" + sha("choir:co-super-report:v1" + owner + computer + run +
   assignment + attempt + toolCallID)` — the provider tool-call identifier is a
   ReportID input, so a retry under a fresh call ID mints a new report. The
   terminal fingerprint additionally covers `ReportID + Summary + EvidenceRefs +
   Commands[].{ExecutionRef, CommandDigest}` — Summary is model prose, so a
   reworded identical outcome conflicts-or-duplicates instead of replaying.
2. **Store CAS digest** (`internal/store/cosuper_assignments.go:153-172`):
   `normalizeCoSuperReportForDigest` zeroes scope/schema/timestamps but KEEPS
   `ReportID`, `Summary`, `EvidenceRefs`, `Commands`, `Mutations`, `Outputs`,
   `ExecutorReceiptRefs`, `CandidateArtifactRef` — the toolCallID contamination
   rides into the CAS digest through ReportID, and Summary prose is digest input.

Consequences today: provider-fresh same-semantics submission cannot replay (new
ReportID → new row intent `capsule-freeze-intent:` + new fingerprint); an
Outputs-only change IS detected (fingerprint covers commands, not outputs —
actually outputs are NOT in the agentcore fingerprint at all, so an outputs-only
change replays silently: the inverse defect). Both directions are wrong.

Also observed (behavior the repair must change, all in
`recordAssignedCoSuperReportOnce`): `Mutations = nil`,
`ObservedSubjectDigest = binding digest`, `CertifiesOriginalSubject = false` are
forcibly overwritten pre-digest (lines 584-586), and cancellation rewrites
`Verdict = pass → abstain` on the submitted packet (lines 581-583) — state-
dependent rewrites of the packet before identity. Under v1 these become validated
claims and reducer dispositions, never packet rewrites.

## Frozen v1 field-classification receipt

Version: `choir:terminal-proposition:v1`. Hash domain: every digest input is
prefixed `choir:terminal-proposition:v1\x00`. Unknown submitted fields and
unknown extensions fail closed (strict admission decode rejects). Normalization:
UTF-8, trimmed; ordered sequences keep submission order; sets are key-sorted;
digest hex lowercase. Every class below is exhaustive over the listed types;
anything unlisted fails closed.

### Slot (terminal uniqueness/CAS key — exactly these four)

`owner_id`, canonical `computer_id`, `assignment_id`, `attempt`. The ordinary
terminal path and the delegated-run/orphan path compete for the same slot; a
delegated-run slot exists only where no assignment obligation exists. No second
terminal row per slot; no parallel orphan slot beside an obligated slot.

### Scope (validated exact, not digest inputs)

Validated by exact match (`ValidateAgainst` shape), never digested: `trajectory_id`,
`run_id` (loop_id), `assigned_agent_id`, binding kind. Stable per attempt; the
slot key plus these validations bind the obligation.

### Canonical (proposition digest inputs — ordered sequences ordered, sets sorted)

- `result`, `verdict`
- `observed_subject_digest`
- `certifies_original_subject`
- `candidate_subject_digest`
- `commands[]` (ordered): `command_digest`, `execution_ref` (as declared claim),
  `exit_code`. Ordered-sequence input.
- `outputs[]` (ordered): `output_id`, `kind`, `digest`, `ref`.
- `mutations[]` (ordered): `kind`, `before_digest`, `after_digest`,
  `subject_bytes_changed`.
- `evidence_refs[]`: key-sorted set of normalized ref strings. Dereferenced nested
  receipts contribute only their own classified canonical fields — never
  provider/transport/retry/batch/model/timestamp/summary/prose envelopes.
- Submitted `execution_attestations[]` (request-level, cardinality-matched): only
  `command_digest`, `exit_code`, `stdout_digest`, `stderr_digest`,
  `source/final_subject_digest`, `granted`, `frozen` contribute.

### Reducer-derived (never digest inputs; derived from reserved slot+digest)

`report_id` (`report:` + proposition digest), `command_id`
(`co-super-report:` + assignment + report), fate/outbox/receipt identifiers,
freeze/revoke overlay fields (actual ack observations), `mutation_id`s,
mutation `evidence_ref`s, `candidate_id`, `candidate_artifact_ref`,
`executor_receipt_refs` as stored, dispositions (`late`, cancellation,
late-fate), `disposition_reason`, all timestamps. A mismatch between a submitted
canonical claim and an authenticated post-reservation fact finalizes a rejection
against the original slot/digest — never a digest rewrite.

### Excluded (never identity, never digest)

Provider tool-call identifiers, transport/retry/batch/model metadata, `summary`
and all prose, timestamps, `late` flag, `schema` tags, envelope/derivation
strings. Summary stays stored and human-readable; it authenticates nothing.

## Replay / conflict / rejection (frozen)

- Digest computed BEFORE any report/command/fate/outbox/receipt identifier is
  derived; those identifiers are domain-separated derivations of (reserved slot,
  proposition digest), never digest inputs. Intent refs derive from the
  proposition digest (`capsule-freeze-intent:` + digest), not the old fingerprint.
- Same slot + same digest → replay the original pending/final receipt; no second
  report, fate, outbox, wake, or physical effect.
- Same slot + different digest → conflict before any report, fate, outbox, wake,
  or effect.
- (a) Pre-reserve admission reject (schema/auth/nonterminal-misuse, strict-decode
  unknown fields) occupies no slot: the same attempt may submit a well-formed packet.
- (b) Reserved same-digest replays the original pending/rejected/final receipt;
  never a second accept.
- (c) Reserved different-digest conflicts; correction proceeds only as a new
  attempt carrying the frozen tuple (`supersedes_assignment_id`,
  `supersedes_attempt`, `prior_receipt_ref`, `supersede_kind` in
  {correction, retry_after_block, owner_reopen}, `reason_enum`, `delta_digest`
  over structured fields only — no summary or prose).
- Partial reports (`result == partial`) use the explicitly separate nonterminal
  sequence and never occupy the terminal slot. Cancellation-wins and late-fate
  are reducer dispositions on the receipt, never packet rewrites.

## Charter behavior cases (must hold post-repair)

Each canonical-field mutation (including Outputs-only) conflicts; excluded
metadata (fresh toolCallID, reworded summary, retry/batch/model envelopes) never
does; provider-fresh same semantics replays the original receipt with no second
report/fate/outbox/wake/effect.

## Authorized repair boundary (item 3 only; red, next commit[s])

1. Add the v1 canonicalizer (pure function + frozen version constant) covering
   the table above with normalization and fail-closed unknown-field admission.
2. Agentcore: mint ReportID from (slot, proposition digest); fingerprint replaced
   by proposition digest; remove pre-digest packet rewrites (mutations nil-out,
   observed overwrite, certifies false, verdict rewrite become claims/validation
   + reducer dispositions).
3. Store: extend `normalizeCoSuperReportForDigest` to the v1 table (drop ReportID
   and Summary from digest input; add outputs/mutations-canonical coverage);
   slot-keyed uniqueness (same slot + same digest replays; different conflicts).
4. Correction-as-new-attempt tuple + rejection classes (a/b/c); partial sequence
   kept nonterminal.
5. Tests (local_test): provider-fresh replay (new toolCallID + reworded summary →
   same receipt, zero new effects); each canonical mutation incl. Outputs-only →
   conflict; same/different digest CAS storms; rejection-class cases; unknown
   field fails closed.

## Explicitly not in this boundary

Admission grammar (item 4), fate saga (item 5 — consumes the v1 digest/slot, owns
the crash window), orphan close (item 6), staging proof (item 7). No physical
effect, outbox, or wake semantics change: replay/conflict gate placement only.

## Evidence floor / rollback

Floor: local_test cases listed above. Rollback: revert repair commit(s); this
Define retained. If any listed canonical field cannot be normalized without
message inspection beyond the table, stop and record the refusal here.

## Standing-questions answers

1. Owner-chartered item 3; settles nothing. 2. Conforms to charter entrypoints
   (`cosuper_assignment_fate.go`, `cosuper_assignments.go`). 3. No deletion in
   this boundary (fingerprint/ReportID derivation replaced in place with citers
   updated same commit). 4. Consumers: RLM terminal path + store CAS + replay
   readers; replay bytes stay decodable (additive versioning, V1 rows valid
   history). 5. Single authority: reducer-owned slot row; report rows are
   derivations. 6. Artifact: local_test replay/conflict matrix. 7/8. No new
   substrate, nothing durable added beyond rows the path already writes. 9. No SSH.
