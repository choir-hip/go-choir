# P4: No Path Accepts a Candidate Without an Owner Act; Standing Consensus Design

Date: 2026-10-15. Source: P4 of
[selfdev-update-architecture-2026-10-15.md](../reports/selfdev-update-architecture-2026-10-15.md)
(§1.3, §3 P4, §4 Station A), with its signatures from S2 of
[principal-identity-and-signed-acts-2026-10-15.md](../reports/principal-identity-and-signed-acts-2026-10-15.md)
(§3.2, §6 S2). Built on S1
([effect-accepted-admitted-without-authority-2026-10-15.md](../../problems/effect-accepted-admitted-without-authority-2026-10-15.md),
branch `feat/s1-owner-act-receipts`, 890fd42d6 and 8a71922a7), whose
corpusd gate refuses `qualified-consensus:` "until S2", and on P5
([p5-host-build-layered-apply-2026-10-15.md](../reports/p5-host-build-layered-apply-2026-10-15.md),
branch `feat/p5-host-build-layered-apply`, head e5a199bb7), whose corpusd
host build service builds the release a computer applies.

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

Status: **reworked on P5** (2026-10-15, landing order S1 → P5 → P4 →
P6-P8). The first build (branch `feat/p4-standing-consensus`, 5ceead609)
added a second host service, `corpusd host-builder` on 127.0.0.1:8089 with
its own key, that only checked the declared patch applies; the release the
computer applied was still the capsule's. This rework, on branch
`feat/p4-standing-consensus-on-p5` (P5 with S1 merged in), drops that
service: the `capsule-verifier` seat is cast by corpusd itself, on the P5
host build of the operation, so the vote binds the bytes the computer
applies. One host builder, not two. Sections 2, 3 and 5 below and the
failure modes, tests, ceremony and residuals are the reworked design; the
"Build" section, added with the code, names every change against the first
build. This commit is docs only; the code commit cites it.

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

## Design (P4 on P5)

Mutation class **red**: run acceptance (canonical `effect_accepted`
admission), approval authority (a standing owner grant), a new use of the
platform-control key (the host's ballot on its own build), the order and
choice of the P5 host build under a standing arm, Trace/evidence (signed
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
| `capsule-verifier` | `platform-control` | `choir-host-build` | corpusd's P5 host build service, outside every guest |
| `independent-reviewer` | `guest-core` | `choir-management` | the computer's guest-core realization key, `desk: management` |

So the guest can sign at most one of the two seats the policy's quorum
needs, and the guest alone cannot reach quorum.

- **The `capsule-verifier` seat (outside the guest): the host's vote on
  its own build.** corpusd casts this ballot itself, in process, from the
  P5 host build job of this operation, and signs it with its
  platform-control key (signer domain `platform-control`, issuer
  `choir-host-build`, kind `ConsensusBallot`). No guest holds that key (a
  guest holds only its public half, to verify modes, offers and host
  builds), and the guest-core signer cannot sign that domain. The job is
  named by P5's own identity, derived by corpusd, not taken from the
  guest: computer (the route's), operation (the request's), bundle digest
  (the subject's, checked against the bundle bytes), base commit and source
  patch sha256 (read from those bundle bytes), and the booted base image
  digest (the request's; the host build refused any base that was not its
  deployed image). The vote:
  - job `done` and its result verifies under platform-control
    (`hostbuild.VerifyJob`): `accept`, with evidence refs naming the job
    id, the result digest, the closure digest, the builder receipt digest
    and the code commit;
  - job `failed` (any P5 reason code: recipe refused, patch does not apply,
    build failed or timed out, base moved, not admitted, interrupted by a
    host restart): `reject`, with the reason code. P5 fails the operation
    for every one of these under an owner decision too;
  - job `queued` or `running`: no ballot yet; the poster answers
    "pending" and mints nothing;
  - no job, or a job for another operation, bundle, patch or base: no
    ballot; the poster refuses.

  Every standing-consensus candidate is host-built, frontend-only ones
  included (P5's G2 keeps frontend-only candidates off the host builder
  under owner decisions; under a standing arm the host must build what it
  votes on, so G2 does not apply). A candidate with no source patch cannot
  be host-built and so cannot be accepted autonomously; the owner's
  `accept_once` still can.
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
4. reads this operation's P5 host build job (identity derived as in §2)
   from its own host build service, casts the `capsule-verifier` ballot on
   it, signs it with platform-control, and verifies it like any other
   ballot (pending: answers 409 `pending: true`, mints nothing);
5. builds the `ConsensusBundle` (seat manifest from the policy, the subject,
   a `PolicySelectionReceipt` bound to the standing arm's ModeReceipt digest
   and generation, the two attestations) and reduces it with
   `decisionpolicy.Reduce`;
6. on quorum, mints a **`ConsensusDecisionReceipt`** signed by
   platform-control: `{computer_id, operation_id, subject, subject_digest,
   policy_id, policy_digest, qualified_consensus_receipt (the reducer's),
   ballot_receipt_digests, standing_mode_receipt_digest,
   standing_generation, computer_tier, host_build: {job_id,
   result_digest, closure_digest, builder_receipt_digest, code_commit},
   not_after}`, with `not_after` the
   earlier of the policy's 30-minute decision window and the standing
   expiry; records it in `consensus_decisions`; returns it;
7. on a signed `reject` (dissent: the host build failed), returns 409 with
   `dissent: true`, the reason and the ballot digests, and mints nothing.

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
intent), the runtime starts one bounded attempt in the background (90
minutes: P5's 20-minute queue deadline plus its 40-minute build timeout,
with margin):

1. read the mode (standing, verified under platform-control);
2. **submit the P5 host build now, before any decision** (P5 builds after
   an owner's accept; under a standing arm the host's vote is on the build,
   so the build comes first), through the same guest path the materializer
   uses (`selfDevelopmentHostBuild`), and poll it until it is terminal. A
   done build is verified and staged where the materializer will find it;
3. if the guest could not get a host-recorded outcome (no source patch,
   quota exhausted, host builder unconfigured, a done build the guest
   cannot verify or stage), stop with the reason on the operation; the
   operation stays at `awaiting_approval` for the owner;
4. otherwise (done, or failed with a host reason) read the head, build the
   subject, sign the reviewer ballot, POST the poster with the bundle bytes,
   the booted base digest and the job id; on a decision run the ordinary
   decision path with the decision receipt (a `consensus_decision` field on
   the decision request and a `standing_consensus` case in the handler); on
   `dissent: true` append `effect_rejected` with
   `AuthorityRef: consensus-dissent:<host ballot digest>`; on `pending`
   wait and ask again; on a moved head re-read and ask again.

**The materializer applies only the decided build.** An operation whose
decision is a consensus decision is applied only as the P5 host-built
layered release, Go-touching or not. The materializer derives the same job
request from the bundle and the booted base, so it names the same
deterministic job id the poster voted on, and P5's `VerifyJob` and blob
re-hash bind what it stages to that signed result. It never falls back to
the capsule-built runtime files for a consensus decision: no host build, no
apply (the operation fails, as P5's G3).

No **boot-time or reconcile-time resumer** (the CLAUDE.md restart rule). A
crash mid-attempt leaves the operation at `awaiting_approval`, where the
owner's `accept_once` still works (a host build already done is reused by
its job id).

## Failure modes the build must handle

The guest cannot forge quorum:

1. a `capsule-verifier` ballot signed by guest-core (or any domain but
   `platform-control`, or with an issuer other than `choir-host-build`) is
   refused, and the guest-core signer refuses to sign that seat at all;
2. a `capsule-verifier` ballot signed by a key other than corpusd's
   platform-control key, or unsigned, or with a broken signature, is
   refused;
3. a reviewer ballot signed by a guest-core key other than the pinned one
   is refused;
4. a ballot whose subject digest, policy digest, computer or operation is
   not the request's is refused;
5. a ballot past `not_after`, or for another audience, is refused;
6. two ballots for one seat, or a ballot for the author seat, does not
   count toward quorum (the reducer's recusal and duplicate rules);
7. the host's `reject` (a failed host build) is dissent: no decision is
   minted, and the guest records a rejection citing the host ballot;
8. an `effect_accepted` with `qualified-consensus:<d>` where `d` is a
   guest-reduced receipt, a random digest, or a decision for another
   operation, bundle, head or computer, is refused at append;
9. a decision used twice (two events) is refused the second time;
10. the request's bundle bytes do not hash to the subject's bundle digest,
    or the bundle names no source patch or base commit: the poster refuses
    (no host ballot).

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

The host's vote binds the applied bytes (rework on P5):

24. no host build job for the identity corpusd derives (computer, operation,
    bundle, the bundle's base commit and patch sha, booted base): no host
    ballot, the poster refuses; a job id the guest names that is not the
    derived one is refused;
25. a job for another computer is invisible (P5's H13), so a guest cannot
    point the poster at another computer's build;
26. a job built from a patch other than the bundle's, or for another
    operation or bundle, never matches the derived identity;
27. a queued or running job yields no ballot and no decision (`pending`);
28. a failed job yields a signed `reject` with its P5 reason code
    (dissent), never an accept;
29. a done job whose result does not verify under platform-control is
    refused (no ballot);
30. the decision names the job and its result digest; the guest refuses to
    record a decision whose host build job is not the one it built;
31. the materializer applies a consensus-decided operation only as the host
    build of that job (same deterministic job id, P5 `VerifyJob`, blob
    re-hash), frontend-only candidates included; with no host build it
    fails the operation and never applies the capsule-built files;
32. there is no second host service, no host-builder key and no port 8089:
    hostauth's internal ports are what P5 and S1 left;
33. host build quota (P5 H21) bounds autonomy too: a rejected or failed
    standing candidate still spent one of the computer's daily builds.

Still allowed (no regression):

21. `accept_once` arm, consume and admission (S1);
22. platform update offers (S1);
23. the S1 owner-act observe/enforce behavior for every other mode;
34. P5 under owner decisions: Go-touching candidates build after the
    accept, frontend-only candidates stay on the plain path (G2).

## Tests (written before the code they test)

- `internal/decisionpolicy`: signed ballot mint and verify; failure modes
  1-5 at the verifier, with the `capsule-verifier` seat admitting only
  `platform-control` / `choir-host-build`.
- `internal/platform`: standing arm rules (14-19), expiry (11), the poster
  (quorum on a done host build, 1-7, 10, 24-29), the gate (8, 9, 12, 13,
  17), on the embedded Dolt store with an in-memory job table standing in
  for P5's service.
- `internal/proxy`: a standing arm on a non-test computer is refused (15);
  on a test computer the receipt's subject carries the tier.
- `internal/receiptsigner`: guest-core signs only the reviewer seat (1).
- `internal/agentcore`, end to end in one process: the guest runtime wired
  as production (store, appender, real updater engine, verifier, route
  ledger) against real corpusd components on embedded Dolt: mode CAS with
  S1 owner acts in enforce, event CAS with the gate, the poster, and P5's
  real `hostbuild.Service` behind its real corpusd routes (job records,
  admission, platform-update blob store, platform-control signing) with a
  stand-in nix build (it checks the patch applies at the base with real
  git and emits a layered closure). The owner arms once; a frontend-only
  candidate is frozen with a real `source.patch`; management's Verify runs;
  the driver submits the host build, waits, asks the poster; corpusd admits
  the `effect_accepted`; the materializer applies the host-built layered
  release with one planned restart (31). Assertions: the decision cites a
  decision corpusd minted that names the host build job, no
  `external-owner:` anywhere, `current` is that job's layer. A second
  candidate in the same window applies with no further owner act. Negative
  legs: a guest-forged `effect_accepted` is refused; after the window
  expires the driver does nothing; a patch that does not apply at the base
  fails the host build and the guest records a dissent rejection (28).
- `scripts/reeng_tracer.mjs --autonomous`: arm the standing mode once
  (the only owner act), expect `applied` with no `owner_approval` step and a
  `qualified-consensus:` decision actor.

## Build

What landed (red build, on P5's host build; no second host service):

- corpusd casts the `capsule-verifier` ballot on the operation's P5 host
  build, signed by platform-control, and mints the decision when it holds
  two valid ballots under a live standing arm (`internal/platform`).
- The guest driver host-builds every standing candidate before asking for
  the decision, refuses a decision that names another build (failure mode
  30, `runStandingConsensus`), and the materializer applies a
  consensus-decided operation only as that build (failure mode 31).
- `choir` can write the `standing_consensus` mode (policy digest and
  expiry); the proxy signs the test-tier classification.

Deploy notes:

- **No new unit.** corpusd, the proxy and the guest image carry the change;
  there is no port 8089 and no host-builder key.
- **The decision poster needs P5's host build enabled:** set
  `CORPUSD_HOST_BUILD_SOURCE_DIR` on corpusd (P5's switch; the other
  `CORPUSD_HOST_BUILD_*` variables keep their defaults). Without it there is
  no host build, no host ballot and no quorum, so no decision is ever
  minted.
- **Nothing is autonomous until the owner arms a test computer.** The new
  mode is owner-only, test-tier only, bounded by an expiry of at most 24 h,
  and defaults to off. A deploy changes no computer's behavior; production
  and primary-tier computers refuse the arm.
- **Staging proof**, on a fresh test computer, once the Landing Loop has
  deployed the SHA:
  `node scripts/reeng_tracer.mjs --label p4-autonomous --scenario engineering --change static --autonomous --autonomy-min 120 --decide-min 75`
  (`--change go` / `go-served` exercise the Go-touching host build; the
  decision wait covers P5's queue deadline and build timeout). The drill's
  only owner act is the arm; it expects `applied` with a
  `qualified-consensus:` decision actor and no decision call of its own.
- **Rollback.** Disarm (any other mode) or let the window expire; unset
  `CORPUSD_HOST_BUILD_SOURCE_DIR` to stop every new decision; `git revert`
  the code commit (tables and the ModeReceipt field are additive).

Local proof run before the commit: focused `internal/proxy`, `owneract`,
`receiptsigner`, `decisionpolicy`, `platform`, `hostbuild`, `cmd/choir`
and the `internal/agentcore` standing-consensus end-to-end tests, all
passing. Local proof does not cover a real nix host build of a standing
candidate or the vmctl test-tier classification (staging).

## Ceremony

- **Conjecture delta.** Before: an `effect_accepted` is admitted only
  through an owner act (S1) or a platform offer; consensus is refused.
  After: it is also admitted through a decision corpusd minted from two
  signed ballots, one of which only corpusd can sign (its vote on the P5
  host build of the candidate), under an owner's standing arm limited to a
  test computer and a window. Conjecture: because the `capsule-verifier`
  seat's key never enters a guest and corpusd mints the decision, a guest
  acting alone (all of its desks, its runtime and guest root) cannot cause
  an acceptance without the host having built the candidate's source
  itself; because the decision names that build and the materializer
  applies only it, the bytes the computer runs are the bytes the host
  voted on; and because the arm is an owner act with a tier and an expiry,
  no autonomy exists outside a test computer and a window the owner chose.
  Falsifier: an `effect_accepted` admitted with `qualified-consensus:`
  whose decision is absent from `consensus_decisions`, or names no done
  host build, or whose ballots do not include a valid platform-control
  `capsule-verifier` signature, or on a computer whose arm is not
  test-tier or is expired; or a consensus-decided operation applied with
  a release that is not the decided build's layer.
- **Protected surfaces touched.** Run acceptance (the gate), approval
  authority (a new mode, owner-only), the platform-control key's uses (it
  now also signs the host's ballot), the P5 host build's ordering (built
  before the decision under a standing arm) and the materializer's apply
  choice for consensus decisions, Trace/evidence (signed ballots, decision
  receipts, the guest-core key pin), the guest-core signer's kind
  allowlist. Not touched: Texture canonical writes, checkpoint/route
  projection, vmctl lifecycle, gateway/provider calls, auth/session
  issuance, the replay reducer, deployment routing.
- **Admissible evidence.** In-process tests on embedded Dolt (corpusd is
  where every decision of this design is made), the in-process end-to-end
  test above, and after the Landing Loop a staging tracer run
  (`--autonomous`) on a fresh test computer. Local proof is not claimed for
  a real nix host build of a standing candidate (P5's own staging proof
  covers the build), or the vmctl test-tier classification on staging.
- **Rollback.** The owner disarms (any other mode); the window expiring
  disarms by itself; `git revert` of the code commit. Tables
  (`consensus_decisions`, `consensus_guest_core_keys`) and the new
  ModeReceipt field are additive; the gate's consensus branch returns to
  refusing when reverted. Unsetting `CORPUSD_HOST_BUILD_SOURCE_DIR` (P5's
  switch) stops every new decision (no host build, no host ballot, no
  quorum).
- **Heresy delta.**
  - discovered: ballots are hashes anyone can compute; the only consensus
    reduction runs inside the subject; `qualified_consensus` cannot be
    autonomous (per-operation arm and a pre-known receipt digest); the
    policy still names the retired `CoSuper` profiles.
  - introduced: a second authority branch in the gate (consensus) beside
    owner acts and offers; a second use of the platform-control key (the
    host ballot, kind-separated); host builds of candidates that are not
    yet accepted (under a standing arm only; bounded by P5's quota).
  - repaired: acceptance without a per-candidate owner act exists, and its
    authority is verified outside the guest; ballots are signatures; a
    `ConsensusBundle` is built by a non-test caller.

## Residuals

- **corpusd does not watch the apply.** The decision names the host build
  and the guest's materializer applies only that build, but corpusd's gate
  does not yet check that the `materialization_applied` event's release is
  the decided build's layer. A guest root can run anything on itself; what
  it cannot do is get a consensus acceptance on the chain for a candidate
  the host did not build. Closing it is a gate check on the applied
  release's closure digest (with P6's restore identity).
- **The host's vote is "it builds", not "it passes tests".** P5 builds
  `.#autoputer` from the base plus the patch (recipe and schema changes
  refused); the host does not run the candidate's tests. The reviewer seat
  is management's Verify verdict, which ran the capsule checks inside the
  guest.
- **Seat independence inside the guest is attribution only.** The reviewer
  ballot is signed by guest-core, reachable by guest root and, before
  confinement, by any desk cell. That is why autonomy is test-tier only.
- **The guest-core key is pinned on first use** by the guest's own
  capability-authenticated request; it is attribution, not a trust anchor
  (S4 computer identity keys are the repair).
- **No resume.** A crash between Verify and the decision leaves the
  operation at `awaiting_approval` with no autonomous retry; the owner's
  `accept_once` remains the fallback.
- **Builds before acceptance.** Under a standing arm every verified
  candidate costs a host build, accepted or not (P5's per-computer daily
  quota, default 8, is the bound).
- **The policy text still names `CoSuper`.** Renaming changes
  `reversible-selfdev-v1`'s frozen digest; left for a `-v2` policy.
