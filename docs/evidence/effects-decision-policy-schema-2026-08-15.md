# Effects decision-policy schema — define freeze (2026-08-15)

**Boundary:** define. Not implementation. Effects remain OFF.
**Review:** repaired freeze ACCEPT 2026-08-15 (Devin, Gemini 3.6, Grok 4.5, GPT-5.6 Sol). Prior pre-addendum review was 3 ACCEPT / 1 REPAIR; Sol's seven items are now in this body.
**Does not:** rematerialize, invent `choir computer create`, delete `external-owner:` / `accept_once` / `awaiting_approval`, enable actuators, or use `OwnerRecovery` for promotion.
**Consumes:** choir-tape-recovery-2026-08-13 restore substrate (paid). Promotion of an effect excursion uses that restore path; this schema does not re-prove it.
**Authority:** owner correction 2026-08-13 (effect-specific multiagent consensus is the autonomy boundary; a human is a policy-selected seat). Fail-closed current gates stay until this schema, a typed consensus receipt, and a reducer exist together and pass deployed acceptance.

This is the frozen define candidate. A later **define sub-slice** must freeze complete policy bytes (quorum, roster, independence, typed bounds) before any participant output. Implement must not invent those values.

---

## Problem this schema solves

Today `verifyFinalizedSelfDevelopmentDecision` accepts an effect only when `AuthorityRef` strips to a non-empty `external-owner:` actor, and `accept_once` is the only mode that can bind a one-shot owner decision to exact operation, bundle, heads, pending transition, and commitments. That is a correct fail-closed pre-consensus gate. It is not the product authority.

The replacement is not "delete the owner check." The replacement is a versioned policy selected before participant outputs, a frozen seat/independence manifest, a typed consensus receipt bound to the exact subject, and a reducer that is the only canonical decision authority. Model-panel output is evidence, never an event.

## Objects

### DecisionPolicy

A content-addressed, versioned document. Selected **before** any participant output for that subject is visible. Immutable after selection for that subject. Participants cannot shrink seats, change quorum, or rewrite dissent rules after seeing results.

No `policy_digest` exists until a later define sub-slice freezes complete policy bytes. The ids below are reserved names, not frozen documents.

| Field | Meaning / fail-closed rule |
|---|---|
| `policy_id` | Reserved name (`reversible-selfdev-v1`, `irreversible-email-v1`, `human-required-v1`) |
| `policy_digest` | SHA-256 of frozen policy bytes. Absent until the define sub-slice. Missing at selection → refuse |
| `effect_class` | `reversible_computer_local` or `irreversible_external` |
| `subject_binding` | What must be exact: bundle digest, heads, recipient, payload digest, actuator |
| `eligible_seats` | Manifest of seat ids, credentials/profile class, independence domain |
| `independence_domains` | Named domains that must not collapse. A seat belongs to exactly one domain for a given policy |
| `quorum` | Per-domain and global minima; failure to meet quorum fails closed. Integers frozen in the define sub-slice, not invented at implement |
| `weighting` | Equal unless the frozen policy declares otherwise before outputs |
| `dissent` | `policy_blocking` vs `recorded`. Unresolved policy-blocking dissent fails closed |
| `abstention` | Whether abstention counts against quorum |
| `timeout` | Canonical UTC expiry of the decision window |
| `recusal` | Required recusal when a seat authored the subject or holds a conflicting domain |
| `replacement` | Whether a recused/expired seat may be replaced from a **predeclared** bench; empty bench fails closed |
| `human_seat` | `required` / `optional` / `absent` for this policy |
| `admissible_evidence` | Verifier receipts, capsule execution receipts, product-path observations. `OwnerRecovery: true` is not an admissible class |
| `actuator` | Trusted actuator id; CoSuper proposes, it does not fire |
| `consequence_receipt` | Required kinds (provider delivery, restore checkpoint, compensation intent) |
| `recovery` | Reversible: tape-recovery restore to pre-promotion checkpoint. Irreversible: compensation or new forward action; restore does not unsend |
| `expiry` | Policy document expiry; expired policy cannot authorize |
| `owner_revocation` | Owner can revoke the policy without agent cooperation |
| `capabilities` | Exact capability ids the actuator may use. Missing or extra → refuse |
| `scope` | Computer-local vs external; platform/cycle OUT. Scope widening after selection → refuse |
| `budget` | Max spend / token / send count. Exceeded → refuse |
| `privacy` | Privacy class of subject and receipts. Downgrade after selection → refuse |
| `blast_radius` | Named surfaces the effect may touch. Touch outside → refuse |

OwnerRecovery checkpoints are **not** admissible evidence for promotion. Route projection already refuses them (`internal/selfdevprotocol/control.go`). This schema repeats that refuse at the policy layer.

### EffectSubject

The exact thing being authorized. Frozen with the policy selection.

Reversible self-development subject:

- `computer_id`
- `operation_id`
- `bundle_digest` (CapsuleEffectBundle: SourceTreeRef, capsule-exec BuildRecipeRef, DependencyToolchainRefs, TestReceipts, RuntimeArtifactRef)
- `desired_event_head`, `effective_event_head`
- `pending_transition_ref`
- `desired_state_commitment`, `effective_state_commitment`

Irreversible email subject (later slice, same Definition):

- all of the reversible identity joins that apply to the proposing trajectory, plus
- `recipient` exact
- `payload_digest` exact
- `actuator = trusted_outbox`
- `acceptance_inbox` owner-controlled

A reversible-effect policy **must refuse** an irreversible subject. That refuse is an acceptance case, not an error.

### SeatManifest (frozen)

Selected with the policy, before outputs.

- `seat_id`
- `independence_domain`
- `kind`: `agent_profile` | `owner_human` | `independent_verifier`
- `eligibility_proof` (credential/profile/registry binding)
- `recused` (must be false at freeze; later recusal is a recorded event, not a silent drop)

The authoring CoSuper is in domain `authoring` and is recused from `verification` on its own bundle.

### PolicySelectionReceipt (ordering proof)

"Selected before outputs" requires a canonical artifact, not a participant timestamp.

| Field | Meaning |
|---|---|
| `receipt_kind` | `PolicySelectionReceipt` |
| `policy_digest` | Frozen policy bytes |
| `seat_manifest_digest` | Frozen SeatManifest |
| `subject_digest` | Frozen EffectSubject |
| `selected_at_head` | Canonical event head at selection |
| `selected_sequence` | Must precede every admissible ballot for this subject |
| `selection_digest` | SHA-256 of canonical bytes **excluding** `selection_digest` |

Ballots and the QualifiedConsensusReceipt must join `selection_digest`. A ballot whose `policy_selection_digest` is missing or later than the ballot is invalid.

### BallotAttestation (typed)

Each ballot is a typed object, not a seat label plus a vote.

| Field | Meaning |
|---|---|
| `ballot_id` | Unique in the decision window |
| `seat_id` | Joins frozen SeatManifest |
| `eligibility_proof_digest` | Digest of the eligibility artifact used at freeze |
| `independence_domain` | Exactly one; reducer refuses if the same signer appears in two domains |
| `policy_digest` | Must equal the selected policy |
| `seat_manifest_digest` | Must equal the frozen manifest |
| `subject_digest` | Must equal the EffectSubject |
| `policy_selection_digest` | Must equal the PolicySelectionReceipt |
| `vote` | accept / reject / abstain |
| `window_id` | Decision window / nonce; replay outside window → refuse |
| `signer_provenance` | Credential or verifier identity; seat labels alone are insufficient |
| `attestation` | Signature or equally explicit trusted attestation contract |
| `ballot_digest` | SHA-256 of canonical ballot bytes **excluding** `ballot_digest` |

### QualifiedConsensusReceipt (typed artifact)

Produced by **consensus reduction** (stage 1, non-event). Consumed by **canonical event reduction** (stage 2) as the only authority that can move `effect_accepted` / `effect_rejected`.

| Field | Meaning |
|---|---|
| `receipt_kind` | `QualifiedConsensusReceipt` |
| `policy_id`, `policy_digest` | Exact policy frozen before outputs |
| `subject_digest` | Digest of the EffectSubject |
| `eligible_seats_digest` | Digest of the frozen SeatManifest |
| `selection_digest` | Must equal the PolicySelectionReceipt |
| `ballots` | One BallotAttestation per participating seat |
| `quorum_evaluation` | Computed by consensus reduction, not by a participant |
| `dissent_disposition` | All policy-blocking dissent resolved or the receipt is invalid |
| `human_seat_state` | `present` / `absent` / `not_required`; invalid if policy says `required` and state is `absent` |
| `window` | Started_at / expires_at canonical UTC |
| `reducer_version` | Pinned consensus-reduction version |
| `receipt_digest` | SHA-256 of canonical receipt bytes **excluding** `receipt_digest` |

A model panel may produce ballots as **evidence inputs to stage 1**. Stage 1 writes the QualifiedConsensusReceipt. Panel output without that receipt is not a decision. Panel output never enters stage 2.

### ConsequenceReceipt

Required after actuator success, and on compensation.

- Reversible promotion: checkpoint identity (tape-recovery checkpoint, not OwnerRecovery for route join) plus later restore receipt if reverted.
- Irreversible send: provider delivery id, payload digest, recipient, timestamp; compensation intent if correction is later required. Plus the dispatch-contract fields below.

Partial actuator success never greens.

## Two reducer stages (not circular)

1. **Consensus reduction** (non-event): consumes BallotAttestations + PolicySelectionReceipt + frozen policy/manifest/subject; **produces** `QualifiedConsensusReceipt`. This stage is not `effect_accepted`. It does not write canonical computer events.
2. **Canonical event reduction**: consumes a verified `QualifiedConsensusReceipt` as an input artifact on `effect_accepted` / `effect_rejected`. This is the only stage that moves the operation out of the pre-consensus gate.

## Digest encoding

`receipt_digest`, `ballot_digest`, and `selection_digest` are SHA-256 of the canonical encoding of all other fields of that object, **excluding** the digest field itself. Canonical byte form is frozen in the policy-bytes define sub-slice before outputs. Do not hash a structure that already contains its own digest.

## Cutover (atomic, fail-closed)

Keep until deployed acceptance of the replacement:

- `external-owner:` prefix check in `verifyFinalizedSelfDevelopmentDecision`
- `accept_once` exact bindings
- `awaiting_approval` vocabulary on the operation state machine
- Effects OFF as the computer's default mode

Add, then switch, then remove:

1. **Define** (this freeze): schema objects, two stages, refuse matrix.
2. **Define sub-slice:** freeze complete `reversible-selfdev-v1` policy bytes (quorum integers, seat roster, independence domains, capabilities, scope, budget, privacy, blast radius) before any participant output.
3. **Implement together:** `DecisionPolicy` store, PolicySelectionReceipt, BallotAttestation verifier, consensus reduction, QualifiedConsensusReceipt verifier, canonical event reducer case that accepts `effect_accepted` only with a valid ConsensusReceipt join, and a new mode that binds the same exact operation/bundle/heads/commitments `accept_once` binds today, plus `policy_digest` and `consensus_receipt_digest`.
4. **Rehearse on staging** with effects still not generally ON.
5. **Delete** `external-owner:` as the *only* accepted authority only after the new verifier is the one production path. Historical receipts keep old field names.

Forbidden intermediate: production can accept an effect with neither owner gate nor consensus receipt.

Mode `off` remains the default. This schema does not arm effects.

## Reserved policy ids (not frozen-complete)

### `reversible-selfdev-v1`

- effect_class: `reversible_computer_local`
- human_seat: `absent` for this acceptance
- subject: CapsuleEffectBundle solitaire promotion
- recovery: tape-recovery restore to a checkpoint taken **before** A promoted, through owner CLI `choir computer restore` (already paid). Checkpoint used for route join must not be OwnerRecovery.
- reversible policy refuses irreversible-email subjects
- quorum, roster, typed bounds: **not yet frozen**; define sub-slice owns those bytes

### `irreversible-email-v1`

- effect_class: `irreversible_external`
- human_seat: `absent` for this acceptance
- subject: one exact email to an owner-controlled acceptance inbox
- stronger quorum / independence than reversible-selfdev-v1 (exact numbers in the email define sub-slice, declared before outputs)
- consequence: provider delivery + payload digest + dispatch contract
- recovery: compensation / new forward action; restore does not unsend
- same Definition, later slice

### `human-required-v1`

- human_seat: `required`
- used to prove the absent-seat refuse: same irreversible subject, seat missing → fail closed

## Irreversible dispatch contract

Restore cannot unsend. Required in this schema before any irreversible actuator is wired. Not required to implement reversible-selfdev first.

| Field | Object | Meaning |
|---|---|---|
| `revocation_check_point` | policy | Revocation is checked immediately before dispatch, not only at proposal |
| `dispatch_idempotency_key` | dispatch intent | Exact-subject dispatch; retries do not double-send |
| `dispatch_intent_receipt` | dispatch intent | Recorded **before** the provider call |
| `provider_outcome` | ConsequenceReceipt | accepted / rejected / unknown |
| `uncertain_outcome_reconciliation` | ConsequenceReceipt | Required when provider outcome is unknown; partial success never greens |
| `crash_window` | ConsequenceReceipt | If process dies after provider acceptance and before consequence persistence, reconciliation must find or compensate the send |

## Refuse matrix (must be true before any green of autonomous effects)

| Case | Result |
|---|---|
| No policy / expired / revoked | refuse |
| Policy selected after outputs exist | refuse |
| Missing PolicySelectionReceipt or ballot not joined to it | refuse |
| Missing required seat | refuse |
| Below quorum | refuse |
| Unresolved policy-blocking dissent | refuse |
| Seat silently dropped after outputs | refuse |
| Independence fabricated (one signer, two domains) | refuse |
| Reversible policy + irreversible subject | refuse |
| `human-required-v1` + human absent | refuse |
| ConsensusReceipt subject ≠ proposed bundle/heads | refuse |
| OwnerRecovery checkpoint as promotion/route evidence | refuse |
| CoSuper packet.kind opens privileged Super execution | refuse (executability is sender authorization) |
| Effect executed while computer mode is `off` | refuse |
| Ballot without attestation / window / eligibility proof | refuse |
| Capabilities/scope/budget/privacy/blast_radius violated | refuse |

## Epoch `8253` / `ak_45ce1796`

**Not current identity. Not an invoke blocker. Not a tape-recovery reopen.**

- Epoch `8253` is a CTS-era failed retained-lifecycle observation. Paid tape-recovery restore proof is retained computer `computer-033352…` **epoch 268**.
- `ak_45ce1796` is a historical API key; later receipts record HTTP 401.

Red mutation that binds retained-computer identity must use the current epoch from product status (last paid: 268), not `8253`.

## What this freeze does not decide yet

- Exact quorum integers, seat roster, independence domain map, and typed bound values for `reversible-selfdev-v1` (next define sub-slice; freeze before outputs).
- Canonical serialization algorithm for digests (same sub-slice).
- Production call sites for freeze/propose tools (route map 5) and CoSuper `update_coagent` (route map 4).
- Irreversible email implement/rehearse (same Definition, later slice; dispatch-contract field ownership is assigned in the table above).

## Implement still forbidden until

- This repaired freeze is accepted (this file).
- The define sub-slice freezes complete policy bytes.
- Schema + receipt + reducer can land together.
- `external-owner:` / `accept_once` / `awaiting_approval` remain until that deployed acceptance.
