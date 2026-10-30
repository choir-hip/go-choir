# L3 goal-file review, 2026-10-15

The file review of
[choir-ledger-l3-hypotheses-settled-2026-10-13.md](../archive/definitions/choir-ledger-l3-hypotheses-settled-2026-10-13.md).
It took four agentic-consensus convergent rounds, from `drafted` to `reviewed`.
Raw transcripts are local session diagnostics under `.agentic-consensus/`
(gitignored). This file is the durable, adjudicated record.

The panel excluded `claude` (the owner's rate limit). The local OMP OpenCode Go
account returned `402 Insufficient account funds`, so the deepseek, glm and
muse anchors could not run. That account is not the paper's credential.

| Round | Frozen at | Blob | Verdicts |
| --- | --- | --- | --- |
| 1 | f658017ce | 6175277d | gpt-6-luna send_back; gpt-6.1-sol, gemini-3.8 accept_with_repairs; deepseek, glm failed (402) |
| 2 | 8650c8f74 | e343ad51 | gpt-6.1-sol, gpt-6-luna send_back; codex, gemini-3.8 accept_with_repairs |
| 3 | 3d7630420 | d849a56a | gpt-6.1-sol, gpt-6-luna send_back; gemini-3.8 accept_with_repairs; codex accept |
| 4 (B2 only) | 7cb82e902 | bdf232c5 | gpt-6.1-sol accept; gpt-6-luna accept |

## Adjudicated findings and repairs

**Round 1.** All findings were adopted (8650c8f74):

- Hypotheses get two kinds, `assertion` and `forecast`, in the vocabulary
  of precommitment-records §5.5 and `types.CalibrationClaim`.
  - Only forecasts carry `resolves_by`, at most 7 days out.
  - Both kinds carry a criterion.
  - No probability band is required. One would reward artificial
    uncertainty.
  - Legacy hypotheses (no kind) stay outside the forecast cohort and are
    never backfilled.
- `omitempty` alone does not prove compatibility. The repair:
  - bind the change to the Cut 2 freeze receipt, with an appended
    compatible-extension receipt;
  - replay and head verification never apply admission-time checks;
  - the new fields join the identity of new commands only;
  - use a trustworthy commit clock;
  - a revert preserves records.
- `HypothesisState` and the fold carry the new fields.
- Live-paper risk: a malformed hypothesis refuses the Cast with a
  per-field cell error, never wedging the desk. Add a smoke check, a
  rollback path and the red ceremony.
- `InboxSeq` is numbered per inbox, so it is not comparable across inboxes.
  L3-2 is gated on the off-host copy; L3-1 is the spine's clock-only
  exception.
- The finish criteria add:
  - per-scorer pinned records;
  - scorer track records against an audit sample;
  - non-governing self-scores;
  - disagreement kept, never averaged;
  - per-source analytics.
- "Independent scoring exists" overstated the code. It is corrected under
  `start.corrections`.

**Round 2.** Four blockers converged and were adopted (3d7630420):

- Rollback reverts the admission check and the overlay together. After a
  rollback, the smoke check must see a story Cast admitted and a story
  published.
- The ordering witness is named.
- Every eligible settlement gets a non-governing self-score and at least
  two independent pinned scorers. Coverage counts scored, pending and
  failed.
- The red ceremony gains the `repaired` leg of the heresy delta.

Gemini's proposed timestamp witness (`CommittedAt` versus `fetched_at`) was
**not adopted**. Timestamps from different clocks cannot prove order, and
codex objected to mutable timestamps.

**Round 3.** B1, B3 and B4 were confirmed repaired. For B2, three of four
found my claim false: I had written that captures go through the paper's
projection chain. Production captures are written to Store B through
corpusd (`internal/agentcore/tool_profiles.go` CorpusdCaptureSink,
`internal/platform/captures.go`). The paper's chain holds only the
`CaptureArrival` record (`internal/agentcore/capture_arrival.go`).

The repair (7cb82e902): a capture counts as after the commitment only when
both of these hold.
- (a) The first `CaptureArrival` naming the capture projects after the
  commitment on the paper's chain.
- (b) A first-creation witness bound to the capture, on the Store B path,
  places its first write after the commitment.

Supporting rules:
- Timestamps can exclude evidence but never admit it.
- An order that cannot be proven fails closed: the settlement is not
  scored.
- A read-only precondition establishes witness (b) before the forecast
  cohort starts. If none exists, L3-1 records one at the corpusd write.

**Round 4.** Both reviewers who had sent it back accepted. Both note that
corpusd's current `created_at` (`time.Now` at the write) cannot serve as
witness (b). The write-time fallback must bind the actual first creation
and survive duplicate or racing writes. These are L3-1 acceptance
obligations, not reasons to reopen the file.

## Not certified

The review certifies the goal file only. It does not certify these:
- that the witness precondition has passed;
- that a guest read route for projection events exists (none does today);
- any runtime behavior.

## Witness (b) design (panel, 2026-10-15, at c7539911f)

Prompt: `.agentic-consensus/l3-witness-b-design-2026-10-15/prompt.md`. It
offered four candidates:
- D1: a Store B first-insert sequence, with a watermark recorded on the
  Cast;
- D2: watermarks recorded on later arrivals;
- D3: a host-attested anchor;
- D4: only the paper's own first inserts count.

**D3 as a per-computer causal anchor** (gpt-6.1-sol, gemini-3.8, codex):
- corpusd reads the writing computer's committed canonical head *before*
  the first insert, and stores a host-signed witness with the winning
  insert only.
- The ordering is: Cast committed → anchor head read → first insert →
  CaptureArrival.
- The Cast must be a strict ancestor of the anchor, and the writer must be
  the paper.

Points the three agreed on:
- Reading the head after the insert is unsound. The sequence insert →
  Cast → read → delayed arrival would admit old evidence.
- Bare D4 (`created=true`) is insufficient for the same reason.
- No platform-wide counter: it widens the red surface and needs
  linearization across stores.
- A capture write must never fail because the witness failed. The
  capture is simply unscorable, and is never filled in later.
- The witness is row data, so a Dolt squash does not touch it. It must be
  immutable against `PutObject` upserts and `DeleteObject`, with
  tombstones.
- The off-host copy carries the signatures, the keys and the chain
  artifacts.

**Dissent** (gpt-6-luna): D1′, a Store B first-insert sequence plus a
post-commit watermark anchor taken off the Cast path. It is sound only
with a serialized Store B cut, and it adds contention on shared writes.

**Adopted:** D3. It is the "record one at the corpusd write" fallback the
reviewed contract already allows, so the file review stays bound to
bdf232c5 for `finish`. The decision is recorded in the L3 now card.
