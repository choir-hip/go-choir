# P4: No Path Accepts a Candidate Without an Owner Act; Standing Consensus Design

Date: 2026-10-15. Source: P4 of
[selfdev-update-architecture-2026-10-15.md](../reports/selfdev-update-architecture-2026-10-15.md)
(§1.3, §3 P4, §4 Station A), with its signatures from S2 of
[principal-identity-and-signed-acts-2026-10-15.md](../reports/principal-identity-and-signed-acts-2026-10-15.md)
(§3.2, §6 S2). Built on S1
([effect-accepted-admitted-without-authority-2026-10-15.md](effect-accepted-admitted-without-authority-2026-10-15.md),
branch `feat/s1-owner-act-receipts`, 890fd42d6 and 8a71922a7), whose
corpusd gate refuses `qualified-consensus:` "until S2".

Owner rulings this design follows:

- **Autonomous first, approvals later** (owner notes 2026-10-14 §13):
  self-development runs with no approval until the loop is proven.
- **Autonomy only on test-tier computers until desk-cell confinement
  lands** (owner, with this brief). The identity report orders S2 after
  confinement because, before it, a desk cell is guest root and the
  computer's attribution of a ballot to a seat is not honest. The owner's
  ruling takes autonomy first on computers whose state does not matter,
  and keeps that ordering for every other computer.
- **The proxy signs owner acts** (owner notes §19): the standing arm is an
  S1 owner-act receipt.

Status: **design and failure modes recorded; build follows on branch
`feat/p4-standing-consensus`** (not pushed, not deployed). This commit is
docs only; the code commit cites it.

## The problem

No code path accepts a self-development candidate without an owner
credential, and the one consensus path that exists cannot be autonomous or
trusted:

1. **Acceptance needs an owner act per candidate.** The guest's decision
   handler (`internal/agentcore/api_self_development.go:246-583`) accepts
   only under `accept_once` (one operation, armed by the owner) or
   `qualified_consensus`. `qualified_consensus`
   (`internal/platform/self_development_modes.go`, transition validation)
   binds one operation, its bundle, its heads and a consensus receipt digest
   that must already be known when the owner arms it, and the guest
   consumes it back to `propose_only`. Someone has to re-arm it for every
   candidate, and the receipt it binds must exist before the arm.
2. **Nothing outside tests builds a `ConsensusBundle`.** The policy
   (`internal/decisionpolicy/reversible-selfdev-v1.json`, `human_seat:
   "absent"`, two verification seats, author recused) and the reducer
   (`decisionpolicy.Reduce`) exist. The only callers that build a bundle are
   tests (`internal/agentcore/effects_rehearsal_test.go`) and the guest's own
   decision handler, which reduces a bundle the *caller* supplies.
3. **Ballots are hashes, not signatures.** `BallotAttestation.Sign`
   (`internal/decisionpolicy/digest.go:104-117`) sets `attestation =
   sha256("choir-ballot-attestation-v1:" || JCS(ballot))`. Anyone who can
   write the JSON can produce every ballot, so whoever holds the guest can
   "reach quorum" alone.
4. **The guest is the actuator.** The only place a consensus bundle is
   reduced is inside the guest, which is the subject of the decision.
5. **corpusd refuses consensus authority entirely** (S1 gate,
   `internal/platform/owner_acts.go`, `effectAcceptedRefusal`:
   "qualified-consensus authority is refused until S2"). That is correct
   today, given 3 and 4, and it means the consensus path is closed in
   enforce and unaccountable in observe.

## Design (P4 as built)

Mutation class **red**: run acceptance (canonical `effect_accepted`
admission), approval authority (a standing owner grant), a new host key and
signing surface (the host builder's ballot signer), Trace/evidence (signed
ballots and decision receipts).

### 1. The standing mode: `standing_consensus`

A new mode beside the five that exist. It is bound to a computer, a policy
digest and an expiry, and to no operation.

- **Armed only by an owner act.** The owner's PUT through the proxy carries
  an S1 `OwnerActReceipt` of a new act kind, `selfdev.autonomy.arm`. corpusd
  requires a verified receipt for this mode **even when
  `CHOIR_OWNER_ACT_MODE=observe`**: a new authority starts fail-closed. A
  guest capability can never write it (`guestConsumptionAllowed` admits only
  consumption of `accept_once` and `qualified_consensus`).
- **Test tier only.** The proxy looks up the computer's warmness class in
  vmctl (the same lookup that proves ownership) and refuses the arm unless
  the class is `test` (`vmctl.WarmnessClassTest`: a computer of an account
  the auth DB marks as a test account). It signs `computer_tier: "test"`
  into the act's subject. corpusd refuses a standing arm whose verified
  subject does not say `test`, and refuses any computer listed as a
  platform computer in `internal/computertiers` regardless. The arm's
  ModeReceipt records `computer_tier`.
- **One policy.** `policy_digest` must be `reversible-selfdev-v1`'s
  (`c34ddf07…`), the one frozen policy with `human_seat: "absent"` and a
  reversible effect class. No other digest arms.
- **A bounded window.** `expires_at` is canonical UTC, in the future, and at
  most 24 hours ahead (`StandingConsensusMaxWindow`). corpusd's lazy expiry
  (the mode read that already turns an expired `accept_once` into `off`)
  covers it, so the first read after the window closes writes `off` and
  records it.
- **Never consumed.** The guest does not consume it per candidate; it stays
  armed across candidates until the owner disarms it or it expires.

### 2. Signed ballots (S2 §6 item 1-3)

`ConsensusBallot` is a `computerevent.Receipt` (RFC 8785, the
`choir-receipt-v1\0` preimage prefix, UUIDv7 id), with kind fields
`{policy_digest, subject_digest, seat, independence_domain, vote,
evidence_refs, computer_id, operation_id, desk, model_id, audience:
corpusd, nonce, not_after}`. Domain separation is by (`receipt_kind`,
`signer_domain`), both signed, and one allowlist every verifier applies:

| seat | signer domain | issuer | boundary |
|---|---|---|---|
| `capsule-verifier` | `host-builder` | `choir-builder` | a host service outside every guest |
| `independent-reviewer` | `guest-core` | `choir-management` | the computer's guest-core realization key, `desk: management` |

So the guest can sign at most one of the two seats the policy's quorum
needs, and the guest alone cannot reach quorum.

- **The `capsule-verifier` seat (outside the guest).** A new host service,
  the host builder's ballot signer (`internal/hostbuilder`, run as
  `corpusd host-builder`, its own systemd unit and its own Ed25519 key under
  `LoadCredential=`; corpusd holds only the public key). It receives the
  frozen bundle's bytes and its `source.patch`, and checks, itself: the
  bundle's digest is the subject's `bundle_digest`; the bundle names a
  source patch whose sha256 is the patch's; the patch applies cleanly at
  the bundle's base commit on the host's own copy of the platform history
  (fetched from the platform's git remote, never from the guest); then the
  declared check command, if one is configured, passes in that patched tree.
  It votes `accept` only when every check passes, `reject` otherwise, and
  signs the ballot with the evidence refs it checked.
- **The `independent-reviewer` seat (inside the guest).** Management's
  Verify verdict, which is what moves an operation to `awaiting_approval`.
  The guest runtime signs it through the guest-core signer
  (`internal/receiptsigner`), which gains exactly one new kind,
  `ConsensusBallot`, issuer `choir-management`. corpusd pins the computer's
  guest-core public key on first use (a new `consensus_guest_core_keys`
  table) and refuses a different key afterwards.

The hash `BallotAttestation` stays only as the reducer's internal join
format: the actuator builds one from each *verified* signed ballot (signer
provenance = `signer_domain:key_id`, eligibility proof = the signed
ballot's digest). A ballot nobody signed never reaches the reducer.

### 3. The decision poster (corpusd)

`POST /internal/computers/self-development/consensus?computer_id=`, guest
capability `event:append` for that computer only. The guest runtime calls
it after management's Verify passes, under a standing mode. corpusd:

1. reads the mode (lazy expiry applies): it must be `standing_consensus`,
   unexpired, `computer_tier: test`, policy `reversible-selfdev-v1`;
2. checks the subject: computer is the route's, heads and commitments
   equal corpusd's own current head for the computer, `bundle_digest` is the
   sha256 of the bundle bytes the request carries;
3. verifies the reviewer ballot (guest-core, pinned key, `desk:
   management`, this subject, this policy, accept or reject);
4. asks the host builder for the `capsule-verifier` ballot and verifies it
   against the pinned `host-builder` key;
5. builds the `ConsensusBundle` (seat manifest from the policy, the subject,
   a `PolicySelectionReceipt` bound to the standing arm's ModeReceipt digest
   and generation, the two attestations) and reduces it with
   `decisionpolicy.Reduce`;
6. on quorum, mints a **`ConsensusDecisionReceipt`** signed by
   platform-control: `{computer_id, operation_id, subject, subject_digest,
   policy_id, policy_digest, qualified_consensus_receipt (the reducer's),
   ballot_receipt_digests, standing_mode_receipt_digest,
   standing_generation, computer_tier, not_after}`, with `not_after` the
   earlier of the policy's 30-minute decision window and the standing
   expiry; records it in `consensus_decisions`; returns it;
7. on a signed `reject` (dissent), returns 409 with `dissent: true` and the
   ballot digests, and mints nothing.

**Why the poster does not append the event itself.** The guest's appender
is the single writer of its chain projection (standing question 5: one
authority per state). If corpusd appended `effect_accepted` to the
canonical head, the guest's projection would fall behind and every later
guest append would fail its CAS until a reconstruct. So the poster *decides
and signs*; the guest's chain writer records the decision as
`effect_accepted` with `AuthorityRef: qualified-consensus:<decision receipt
digest>` and the decision receipt as input artifact 1; and corpusd's
admission gate (§4) admits that append only for a decision the poster
minted. The decision is made outside the guest; the guest only writes it
down, and cannot write down one the poster did not make.

### 4. The admission gate admits consensus authority

In `CompareAndSwap`, for `AuthorityRef = qualified-consensus:<d>`, replacing
S1's blanket refusal:

- `d` is a row in `consensus_decisions` for this computer (corpusd minted
  it; a guest-reduced `QualifiedConsensusReceipt` digest is not);
- the row's operation, bundle, heads and commitments equal the event's
  `ParentEventID`, `ProposedEffectRef` and expected heads and commitments;
- some input artifact of the event has digest `d`, and input artifact 0 is
  the standing arm's ModeReceipt the decision names;
- the decision's `not_after` has not passed (with the S1 offer grace of two
  minutes for the guest's own append latency);
- the current mode is still `standing_consensus` at the same generation as
  the decision's, unexpired, with `computer_tier: test` (an owner disarm or
  an expiry between decision and append stops autonomy);
- the row has not admitted a different event; the admission marks it in
  the same transaction.

This branch of the gate is **always enforcing**, independent of
`CHOIR_OWNER_ACT_MODE`: there is no pre-S2 consensus traffic to observe,
and observe would admit exactly the forgery this station exists to stop.

### 5. The guest's standing-consensus driver

After management's Verify reduction moves an operation to
`awaiting_approval` (`internal/agentcore/rlm_reduce.go`, the Verify
intent), the runtime starts one bounded attempt in the background: read the
mode (standing, verified), read the head, build the subject, sign the
reviewer ballot, POST the poster with the bundle and its patch, and on a
decision run the ordinary decision path with the decision receipt (a new
`consensus_decision` field on the decision request and a
`standing_consensus` case in the handler). On `dissent: true` it appends
`effect_rejected` with the dissent as the reason (rejection is always
allowed under a standing mode). On any other failure it records the reason
on the operation and stops: **no boot-time or reconcile-time resumer** (the
CLAUDE.md restart rule). A crash mid-attempt leaves the operation at
`awaiting_approval`, where the owner's `accept_once` still works.

## Failure modes the build must handle

The guest cannot forge quorum:

1. a `capsule-verifier` ballot signed by guest-core (or any domain but
   `host-builder`) is refused;
2. a `host-builder` ballot signed by a key other than the pinned one, or
   unsigned, or with a broken signature, is refused;
3. a reviewer ballot signed by a guest-core key other than the pinned one
   is refused;
4. a ballot whose subject digest, policy digest, computer or operation is
   not the request's is refused;
5. a ballot past `not_after`, or for another audience, is refused;
6. two ballots for one seat, or a ballot for the author seat, does not
   count toward quorum (the reducer's recusal and duplicate rules);
7. the host builder's `reject` is dissent: no decision is minted;
8. an `effect_accepted` with `qualified-consensus:<d>` where `d` is a
   guest-reduced receipt, a random digest, or a decision for another
   operation, bundle, head or computer, is refused at append;
9. a decision used twice (two events) is refused the second time;
10. the request's bundle bytes do not hash to the subject's bundle digest,
    or the patch's sha256 is not the one the bundle names, or the bundle
    names no patch: the host builder refuses (no accept ballot).

An expired window stops autonomy:

11. a mode read after `expires_at` writes `off`; the poster refuses;
12. an append after the window closed (decision minted inside it) is
    refused by the gate, and so is one after the owner disarmed;
13. a decision past its own `not_after` is refused at append;
14. an arm with `expires_at` more than 24 hours ahead, non-canonical, or in
    the past is refused.

A non-test computer is refused:

15. the proxy refuses to arm when vmctl's class is not `test`;
16. corpusd refuses an arm whose owner act does not sign
    `computer_tier: test`, or that carries no verified owner act at all
    (observe included), or that names a platform computer;
17. the poster and the gate refuse a mode whose receipt does not record
    `computer_tier: test`.

Other authority:

18. a guest capability cannot write `standing_consensus` (any mode write
    by a guest other than a consumption is refused, as today);
19. the standing mode never binds operation fields or a consensus receipt
    digest, and only `reversible-selfdev-v1`;
20. a reject under a standing mode is allowed; an approve under a standing
    mode needs a decision receipt whose standing digest is the current
    ModeReceipt's.

Still allowed (no regression):

21. `accept_once` arm, consume and admission (S1);
22. platform update offers (S1);
23. the S1 owner-act observe/enforce behavior for every other mode.

## Tests (written before the code they test)

- `internal/decisionpolicy`: signed ballot mint and verify; failure modes
  1-5 at the verifier.
- `internal/hostbuilder`: the patch checker on a real temporary git repo
  (applies at base; refuses a patch that does not apply; refuses a bundle
  digest or patch digest mismatch; refuses a bundle with no patch), and the
  service signs `accept` and `reject` ballots that verify against its key.
- `internal/platform`: standing arm rules (14-19), expiry (11), the poster
  (quorum, 1-7, 10), the gate (8, 9, 12, 13, 17), on the embedded Dolt
  store, as S1's tests do.
- `internal/proxy`: a standing arm on a non-test computer is refused (15);
  on a test computer the receipt's subject carries the tier.
- `internal/agentcore`, end to end in one process: the guest runtime wired
  as production (store, appender, real updater engine, verifier, route
  ledger) against real corpusd components on embedded Dolt (mode CAS, event
  CAS with the gate, the poster, the host builder's service with its git
  checker on a real repository). The owner arms once; a candidate is frozen
  into the updater store with a real `source.patch`; management's Verify
  runs (`recordSelfDevelopmentVerdict`, real finalization); the driver asks
  the poster; corpusd admits the `effect_accepted`; the materializer
  applies with one planned restart. Assertions: the decision event's
  authority is `qualified-consensus:<d>` with `d` a row corpusd minted, no
  `external-owner:` anywhere, the operation reaches `applied`. A second
  candidate in the same window applies with no further owner act. Then the
  negative legs: a guest-forged `effect_accepted` is refused; after the
  window expires the poster refuses.
- `scripts/reeng_tracer.mjs --autonomous`: arm the standing mode once
  (the only owner act), expect `applied` with no `owner_approval` step and a
  `qualified-consensus:` decision actor.

## Ceremony

- **Conjecture delta.** Before: an `effect_accepted` is admitted only
  through an owner act (S1) or a platform offer; consensus is refused.
  After: it is also admitted through a decision corpusd minted from two
  signed ballots, one of which only a host service outside the guest can
  sign, under an owner's standing arm limited to a test computer and a
  window. Conjecture: because the `capsule-verifier` seat's key never
  enters a guest and corpusd mints the decision, a guest acting alone (all
  of its desks, its runtime and guest root) cannot cause an acceptance
  without the host builder's own check passing; and because the arm is an
  owner act with a tier and an expiry, no autonomy exists outside a test
  computer and a window the owner chose. Falsifier: an `effect_accepted`
  admitted with `qualified-consensus:` whose decision is absent from
  `consensus_decisions`, or whose ballots do not include a valid
  `host-builder` signature, or on a computer whose arm is not test-tier or
  is expired.
- **Protected surfaces touched.** Run acceptance (the gate), approval
  authority (a new mode, owner-only), a new host key and a new host
  service (host builder ballot signer), Trace/evidence (signed ballots,
  decision receipts, the guest-core key pin), the guest-core signer's kind
  allowlist. Not touched: Texture canonical writes, checkpoint/route
  projection, vmctl lifecycle, gateway/provider calls, auth/session
  issuance, the replay reducer, deployment routing.
- **Admissible evidence.** In-process tests on embedded Dolt (corpusd is
  where every decision of this design is made), the in-process end-to-end
  test above, and after the Landing Loop a staging tracer run
  (`--autonomous`) on a fresh test computer. Local proof is not claimed for
  the host builder's git fetch from GitHub, the systemd credential wiring,
  or the vmctl test-tier classification on staging.
- **Rollback.** The owner disarms (any other mode); the window expiring
  disarms by itself; `git revert` of the code commit. Tables
  (`consensus_decisions`, `consensus_guest_core_keys`) and the new
  ModeReceipt field are additive; the gate's consensus branch returns to
  refusing when reverted. Stopping the `go-choir-host-builder` unit stops
  every new decision (no host ballot, no quorum).
- **Heresy delta.**
  - discovered: ballots are hashes anyone can compute; the only consensus
    reduction runs inside the subject; `qualified_consensus` cannot be
    autonomous (per-operation arm and a pre-known receipt digest); the
    policy still names the retired `CoSuper` profiles.
  - introduced: a second authority branch in the gate (consensus) beside
    owner acts and offers; a host service that runs `git apply` on
    guest-authored patches (unprivileged unit, no secrets but its own key,
    its own state directory).
  - repaired: acceptance without a per-candidate owner act exists, and its
    authority is verified outside the guest; ballots are signatures; a
    `ConsensusBundle` is built by a non-test caller.

## Residuals

- **The host seat checks the source, not the applied bytes.** Until P5 (the
  host build service and layered release), the materializer applies the
  bundle's capsule-built runtime files, while the host builder checks the
  source patch the bundle declares. A guest that declares a benign patch
  and ships different runtime files gets an accept. Accepted on test
  computers only (owner ruling); P5 closes it by making the host build the
  release it votes on.
- **The host check is shallow in the first slice.** It proves the patch
  applies at the named base on the platform's own history and runs the
  configured check command, if any. No check command is configured on
  Node B in this slice (whether the Go tests fit the builder's sandbox and
  timeout is the S2 UNVERIFIED claim); the declared test run comes with P5.
- **Seat independence inside the guest is attribution only.** The reviewer
  ballot is signed by guest-core, reachable by guest root and, before
  confinement, by any desk cell. That is why autonomy is test-tier only.
- **The guest-core key is pinned on first use** by the guest's own
  capability-authenticated request; it is attribution, not a trust anchor
  (S4 computer identity keys are the repair).
- **No resume.** A crash between Verify and the decision leaves the
  operation at `awaiting_approval` with no autonomous retry; the owner's
  `accept_once` remains the fallback.
- **The policy text still names `CoSuper`.** Renaming changes
  `reversible-selfdev-v1`'s frozen digest; left for a `-v2` policy.
- **The host builder key is generated on the host and not backed up**, like
  the S1 key; losing it stops autonomy until a new key is generated and
  both units restart.
