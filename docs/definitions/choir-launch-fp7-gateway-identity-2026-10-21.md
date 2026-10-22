---
definition_version: 4

# Station FP7 of the launch first pass
# (docs/definitions/choir-launch-first-pass-metamission-2026-10-21.md), batch
# B7. Owner 2026-10-21 (owner notes section 36 and 37): the Claude delegate
# stays on its one test computer; before any wider use the key is held by the
# gateway; every security and density item that can land before launch does.
readiness: drafted

review:
  reviewer: none yet; FP0 sends each red commit's frozen diff to a panel before it lands
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-21T17:43:14Z # the base commit's time; the lane began about 18:00Z
  source:
    canonical_ref: origin/main 26c4e1ed0 (the first pass leaves the iPhone app out)
    deploy_identity: >-
      unknown to this lane (FP0 reads staging); the metamission records
      f2c4a3ec3 serving and c893a956a deploying
  worktrees:
    - path: scratchpad/lanes/b7-gateway
      status: clean
      class: goal_candidate
      owner: FP7 lane worker
      touch: goal_owned
      recovery: branch lane/b7-gateway; git revert of its commits
    - path: /Users/wiz/go-choir and every other worktree
      status: unknown
      class: other_agent_wip
      owner: other lanes and the orchestrator
      touch: forbidden
      recovery: leave in place

finish:
  deliver: >-
    A gateway that knows which computer a bearer belongs to and refuses it
    from any other; whose model calls are scheduled fairly instead of
    rejected; that can hold the Claude delegate's key so the guest never
    carries it; and that is the one holder of the Jev key (the front-importance
    timer calls it instead). Independent rollback units: SEC-05 (with its nix
    line), RES-11 (with its nix line), SEC-16 (code, then a nix change);
    SEC-08's gateway half depends on SEC-05.
  artifact: >-
    Commits on lane/b7-gateway (docs, SEC-05, SEC-08 Option K gateway relay,
    RES-11 scheduler, SEC-16 host Jev route), each behind a switch that
    defaults to the old behavior or to observe, with tests that fail first;
    the exact deployed proofs for FP0; the guest-side Option K slice written
    as a precise change set (not built here); RES-12 as a design.
  acceptance:
    - action: >-
        go test ./internal/gateway -run 'BearerBinding|PeerPin|ConstantTime' -count=1
        (SEC-05), and the whole package through the heavy wrapper
      proves: >-
        a bearer pinned to computer A is refused from B's address on every
        route that authenticates a computer; A's own address passes; the host
        moves the pin; modes off/observe/enforce/strict behave as written;
        the pin survives a gateway restart and a rollback to the old key name
      evidence_class: local test
    - action: >-
        go test ./internal/gateway -run 'AnthropicRelay' -count=1 (SEC-08)
      proves: >-
        the relay refuses an unlisted computer, an expired window, a wrong
        path, method or model, an oversize body; never forwards a caller
        credential; sends only the gateway-held key upstream; streams
        events as they arrive; never logs the key; with no key configured it
        refuses and falls back to nothing
      evidence_class: local test (fake key, fake upstream)
    - action: >-
        go test ./internal/gateway -run 'ModelScheduler' -count=1 (RES-11)
      proves: >-
        no slot leaks on cancel, timeout, panic or error; a light computer is
        served ahead of a heavy one's backlog; per-computer and global caps
        hold; queue depth and wait are bounded and answered with a 429 and a
        Retry-After
      evidence_class: local test
    - action: >-
        go test ./internal/gateway ./internal/frontimportance -run
        'HostJudgment|GatewayDecider|DeciderIsTheGateway' -count=1 (SEC-16)
      proves: >-
        the host Jev route refuses a guest, an uncredentialed host process
        and an unlisted caller; labels receipts from its own table; the
        scorer's default decider is the gateway client and direct OpenRouter
        is an explicit opt-in
      evidence_class: local test (fake key, fake upstream)
    - action: the deployed proofs listed under "Deployed proofs for FP0" below
      proves: the claims on the deployed gateway, not on a worktree
      evidence_class: deployed proof (FP0 runs them; this lane cannot)
  rollback: >-
    Each of the three behavior commits reverts alone with git revert, and
    each has a runtime switch that restores the old behavior without a
    deploy of old code: GATEWAY_BEARER_BINDING=off, the relay's empty
    allowlist (the default), GATEWAY_MODEL_SCHEDULER=off (the default),
    FRONT_IMPORTANCE_DIRECT=1 (SEC-16; the unit keeps loading the key in
    batch 1, because the nix removal, 8c8bd7e54, is reverted out of it by
    9d53138fc and FP0 cherry-picks it only after deployed proof 7 passes; once
    that lands, the rollback is a nix revert of it plus FRONT_IMPORTANCE_DIRECT=1).
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize what a stolen or copied gateway credential, and a heavy
    computer, can do to another computer's budget and availability, and
    minimize the provider keys that sit on a guest, while preserving every
    computer's own model calls and the paper's availability.
  goodharting_would_be: >-
    Binding that passes its tests but is never enforced on staging; a
    scheduler that is fair in a unit test and stalls the paper; a "relay"
    that forwards the caller's credential or any path; moving the delegate's
    key to the gateway by moving the owner's subscription token through it
    (which Anthropic's terms forbid); counting a dormant route as SEC-08
    closed.

homotopy:
  realism_axis: >-
    Unit tests with fake keys and fake upstreams, then the mode in observe on
    staging reading the paper's and the test computers' real counts, then
    enforce on staging, then a day of the paper's real load; for the relay,
    then the guest half and the owner's Console key.

boundaries:
  mutation_class: red (gateway credentials and provider calls); the docs commit is green
  authority_sources:
    - owner notes section 36 and 37 (2026-10-21); checklist SEC-05, SEC-08, SEC-16, RES-11, RES-12, DEC-03, DEC-07
    - docs/choir-doctrine.md and CLAUDE.md (Mutation Classes, Convergence, Problem Documentation First)
    - docs/design/claude-delegate-slice1-2026-10-14.md and the Anthropic terms quoted in docs/reports/claude-agent-sdk-in-choir-2026-10-14.md section 2.2
  must_preserve:
    - Nothing here makes a call succeed that was refused before. A change that cannot decide refuses.
    - The Jev route refuses a mismatched pin in every mode (its 403). It pins an unpinned bearer on first use in every mode but strict, which refuses it (SEC-05 findings 13 and 14).
    - The Anthropic relay serves only a bearer already pinned to the caller's address, in every binding mode, and never writes identity state (batch 1 review, SEC-08 findings 11 and 12).
    - Who may run the delegate, and where, does not change: one test computer, the existing expiry.
    - No secret in the repository, a log, a test, a response or a guest disk; tests use fake keys.
    - Search, Jev judgments and source-api keep their own buckets and the old limiter.
    - The Claude delegate runs only for engineering-desk self-development work, on the one test computer (owner, 2026-10-21 ruling); Option K changes how its key is held, never who runs it, where or for what.
    - A crash restart resumes nothing; the registry and the pin file are the only durable state this adds, and neither resumes work.
  excluded:
    - Moving the owner's subscription token through any relay (forbidden by Anthropic's terms).
    - The guest half of Option K (capsule egress route, broker, vmmanager, nix): written as a change set, not built here.
    - Live in-place bearer rotation (needs a host-to-guest channel that does not exist): designed, not built.
    - Removing the in-guest 1.5 s spacer (a guest change that follows the scheduler's staging proof).
    - RES-12 (batch endpoints) beyond a design: it needs durable parked calls (SP1) and the provider batch APIs.
  protected_surfaces: [gateway and provider calls, gateway identity store, auth between guests and the host, credential custody (the relay holds a provider key; the front-importance unit stops holding one)]

now:
  status: working
  slice: >-
    batch 1 repaired after bunch 4b (SEC-05 finding 14); waiting for FP0 to
    re-freeze, re-review, land and prove. B7 body caps (residual 4, the
    scheduler's precondition) built on lane/fp7-gateway-body-caps from main
    a35b3586f, waiting for FP0 to review and land (section 6)
  source_ref: 26c4e1ed0
  deploy_identity: unknown to this lane
  candidate:
    id: fp7-gateway-identity
    state: ready
    ref: lane/b7-gateway-b1fix
    base: 26c4e1ed0
    digest: none
    scope: [internal/gateway, internal/frontimportance, cmd/gateway, nix/node-b.nix (gateway and front-importance lines only), docs]
  conjecture:
    id: C-FP7-bind
    claim: >-
      If every gateway route requires the bearer's pinned address, a copied
      bearer is useless from any other computer, and no computer's own calls
      fail, because vmctl already attests each boot's address.
    test: >-
      Observe mode on staging for a day: bearer_binding.would_refuse stays 0
      for the paper and the test computers across deploys and reboots, and
      pinned reaches the number of running computers; then enforce.
    edge: missing_oracle
    delta_o: >-
      The gateway's /health reports binding mode, pinned and unpinned counts,
      would-refuse and refused counts, so staging can be read without SSH.
    scope_if_supported: gateway-bearer routes served to Firecracker guests on Node B
    status: active
    evidence_refs: []
  decision:
    what: >-
      Ship the binding in modes (off, observe, enforce, strict) and staged:
      nix sets observe first; unset is enforce; an unknown value is enforce;
      a binding refusal is 429 with Retry-After so a deploy refresh heals.
      Ship the scheduler and the relay dormant (off, empty allowlist). The
      relay allowlist is keyed by gateway identity (the VM id vmctl issues
      bearers for), because the gateway does not know the ComputerID. Batch 1
      review: the relay's pin check is its own and ignores the rollout mode;
      the Jev route keeps its first-use pin in every mode but strict, which refuses an unpinned bearer; the SEC-16 nix removal
      is a separate, later landing.
    kind: operational
    status: settled
    evidence_ref: the Jev pin incident (docs/problems/l2a-merge-recall-on-the-paper-2026-10-14.md finding 2), hostauth's observe-then-enforce rollout, and the batch 1 panel (d5a62eace)
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      The code paths are as read and tested locally; vmctl re-ensures the
      guest address after every start path except a brand-new interactive
      VM's first resolve (residual below); a snapshot resume keeps its
      address, a cold boot issues a new credential, a refresh and a recovery
      keep the token and take a new address; the source address is
      trustworthy because of the S1a tap rules.
    main_uncertainty: >-
      How long the stale-pin window is after a refresh (guest healthy until
      vmctl's ensure returns) against the guest's Retry-After waits, which
      only non-streaming inference honors (residual below); the observe
      counts after a planned refresh answer it.
    next_observation: bearer_binding.would_refuse for the paper after a planned refresh in observe mode
  blocker_or_risk: >-
    Option K needs the owner's Console API key (a credential question, not
    a design branch) and the guest half. Residuals from the panel, none
    blocking observe: (1) only non-streaming Provider.call waits out a 429
    Retry-After; streaming inference, guest search, source-api and Jev do
    not, so before enforce the stale-pin window must be measured or those
    clients taught to wait; (2) a brand-new interactive VM's first
    resolveDesktopContext issues its credential before boot and never
    ensures it, so it stays unpinned until the next resolve (refused in
    strict; strict must wait for a vmctl fix); (3) withdrawn (bunch 4b: Jev
    pins on first use in observe again); (4) scheduler on: decoded
    bodies (no MaxBytesReader) sit in memory while queued, up to 256 per
    computer, and Acquire does not check an already-cancelled context; both
    are preconditions of turning the scheduler on (repaired in code on
    lane/fp7-gateway-body-caps, section 6: bodies capped per route, a model
    call takes its slot before reading its body, Acquire refuses a
    cancelled caller; closed only after proofs 10 to 12, with the caps in
    enforce, before the scheduler flip); (5) scorer.go sets the
    direct-mode receipt path only for FRONT_IMPORTANCE_DIRECT=1 while
    newDecider also accepts true and on (the reverted-in unit sets the path
    itself, so it is harmless while the unit is as it is).
  next_action: >-
    FP0 freezes lane/b7-gateway-b1fix, re-reviews the repair commits (now including 6f9381bca),
    lands the batch (8c8bd7e54 and its revert 9d53138fc cancel; land them
    together or drop both), runs the deployed proofs below, and only after
    proof 7 passes cherry-picks 8c8bd7e54 as its own landing. B7: FP0
    reviews lane/fp7-gateway-body-caps (ee0183488 code, 56a6df469 nix;
    land them together), deploys with gatewayBodyCaps observe, runs proofs
    10 and 11, after a day runs 12, flips to enforce as its own deploy,
    runs 12 again; only then the scheduler flip and proof 9

receipts:
  - id: fp7-docs
    boundary: define
    identity: 24026a9e6
    proof_refs: [docs/problems/gateway-bearer-works-from-any-computer-2026-10-21.md, docs/problems/claude-delegate-key-sits-on-the-guest-disk-2026-10-21.md, docs/problems/gateway-model-calls-have-no-scheduler-2026-10-21.md]
    rollback_ref: git revert 24026a9e6
    disposition: problems documented before any fix
  - id: fp7-b1-review-docs
    boundary: define
    identity: d5a62eace
    proof_refs: [the batch 1 panel reports (GPT-6.1-sol send back, Gemini 4 Argon accept with edits)]
    rollback_ref: git revert d5a62eace
    disposition: the panel's findings recorded in the problem records before any fix
  - id: sec-05
    boundary: implement
    identity: 8434448b6 (+ nix 8f9d8443b, + renewal, see sec-05-renewal, + Jev fixes 9d612fffc and 6f9381bca)
    proof_refs: [internal/gateway/bearer_binding_test.go, "go test ./internal/gateway -race: green"]
    rollback_ref: GATEWAY_BEARER_BINDING=off, or git revert
    disposition: built; observe deployed; the enforce flip is lane/fp0-fp7-enforce (see "SEC-05 enforce flip"); not closed until enforced on staging and step 5 runs
  - id: sec-05-renewal
    boundary: implement
    identity: c84fe6fe8
    proof_refs: [internal/gateway/credential_renewal_test.go, "three mutations fail the tests"]
    rollback_ref: git revert (the expiry then returns to one TTL from issue)
    disposition: built; closes the 720 h time bomb for refresh-only computers once deployed; live rotation stays designed
  - id: sec-08-gateway-half
    boundary: implement
    identity: 6ff5b6254 (+ relay pin fix 94edd2f54)
    proof_refs: [internal/gateway/anthropic_relay_test.go, "seven relay mutations each fail the tests"]
    rollback_ref: leave ANTHROPIC_API_KEY unset, or git revert
    disposition: built, dark; the guest half and the owner's Console key are open
  - id: res-11
    boundary: implement
    identity: 2f4039cad (+ nix dd9b1a73f, + deadline fix f2d3a77be)
    proof_refs: [internal/gateway/model_scheduler_test.go, "six scheduler mutations each fail the tests"]
    rollback_ref: GATEWAY_MODEL_SCHEDULER=off (the default), or git revert
    disposition: built, off; to be flipped after the staging proof
  - id: sec-16
    boundary: implement
    identity: 8fd6fcc31 (nix 8c8bd7e54 reverted out of batch 1 by 9d53138fc; FP0 cherry-picks it after proof 7)
    proof_refs: [internal/gateway/host_judgment_test.go, internal/frontimportance/gateway_decider_test.go, "go build ./cmd/corpusd"]
    rollback_ref: FRONT_IMPORTANCE_DIRECT=1 while the unit still loads the key, or git revert
    disposition: built; the scorer defaults to the gateway; the nix change that removes the key from the unit lands after the staging proof
  - id: b7-body-caps
    boundary: implement
    identity: ee0183488 (+ nix 56a6df469; problem 861d3fe69, tests 9d748f316)
    proof_refs: [docs/problems/gateway-request-bodies-unbounded-2026-10-22.md, internal/gateway/body_caps_test.go, internal/gateway/model_scheduler_test.go TestModelSchedulerRefusesAnAlreadyCancelledCaller, "go test -race ./internal/gateway/...: green", "every new test failed on main a35b3586f by behaviour; two mutations (no Content-Length check, no would_refuse count) each fail them"]
    rollback_ref: gatewayBodyCaps observe (model routes), GATEWAY_BODY_LIMITS to raise a limit, or git revert of both commits
    disposition: built on lane/fp7-gateway-body-caps; observe for the model routes on the first deploy, every other route enforces; not closed until proofs 10 to 12 pass in enforce
  - id: b7-body-caps-review-round
    boundary: implement
    identity: a303a223e (+ nix 95240b65c; findings d5abe8f22, tests a843f5e14)
    proof_refs: [docs/problems/gateway-request-bodies-unbounded-2026-10-22.md "Repair of the review findings (bunch 88)", "internal/gateway/body_caps_test.go TestBodyCapsBoundTheBytesInFlightWithTheSchedulerOff, TestBodyCapsQueuedCallReadsItsBodyAfterTheServerReadTimeout, TestBodyCapsTrickledBodyIsCutAtTheBodyReadDeadline, TestBodyCapsEnvOverridesAndHealth: each failed on 02010a2e0", "go test -race ./internal/gateway -run 'TestBodyCaps|TestModelScheduler': green"]
    rollback_ref: git revert of a303a223e and 95240b65c; GATEWAY_BODY_LIMITS in_flight= or GATEWAY_BODY_READ_TIMEOUT to loosen without a code change
    disposition: F1, F2, F4 fixed, F3 decided (160 MiB), F5 wording fixed; F6 and part of F7 declined with reasons in the problem record
  - id: b7-body-caps-review-round-2
    boundary: implement
    identity: 9f9e1e658 (+ nix comments e07fdf76d; findings 91509d7df, tests 007c9ee0d)
    proof_refs: [docs/problems/gateway-request-bodies-unbounded-2026-10-22.md "Repair of the review findings (bunch 109)", "internal/gateway/body_caps_test.go TestBodyCapsOneCallerCannotHoldTheWholeInFlight, TestBodyCapsAPanicInTheDecodeGivesItsBytesBack, TestBodyCapsReadWindowStartsWhenTheBodyFits and internal/provider/media_test.go TestNormalizeImageDecodesAtMostABoundedNumberAtOnce: each failed on 2e9ebc5eb", "go test -race ./internal/gateway -run 'TestBodyCaps|TestModelScheduler': green", "go test ./internal/gateway/... ./internal/provider/...: green"]
    rollback_ref: git revert of 9f9e1e658; GATEWAY_BODY_LIMITS per_caller= (up to in_flight) loosens the share without a code change
    disposition: F-A fixed (per_caller = in_flight/4, first body always fits), DeepSeek F1 fixed (4 decodes at once; MaxImagePixels kept; nix arithmetic corrected), F-B, F-C and DeepSeek F3 fixed, F5 wording fixed; DeepSeek F2 and F4 declined with reasons in the problem record
  - id: b7-body-caps-review-round-3
    boundary: implement
    identity: a3d04df9d (+ nix comment 2e9329f22; findings 281cfa3e1, tests 0317092cb)
    proof_refs: [docs/problems/gateway-request-bodies-unbounded-2026-10-22.md "Repair of the review findings (bunch 116)", "internal/provider/media_test.go TestPrepareImageDecodesItsBase64InsideADecodeSlot, TestImageDecodeSlotWaitEndsWithTheCallsContext, TestImageDecodesOneCallerHoldsAtMostItsShareOfTheSlots, TestSummarizeMediaHashesWithoutDecodingTheWholeImage and internal/gateway/body_caps_test.go TestBodyCapsModelCallNamesItsComputerAsTheImageDecodeCaller, TestBodyCapsOneCallerCannotHoldTheWholeInFlight (refused.per_caller): neither package builds on 982a70f3a (new API); with only the API shimmed onto 982a70f3a each fails behaviorally", "go test -race ./internal/gateway -run 'TestBodyCaps|TestModelScheduler': green", "go test ./internal/gateway/... ./internal/provider/...: green"]
    rollback_ref: git revert of a3d04df9d (the round 2 code stays)
    disposition: F1 fixed (base64 decoded inside the slot; SummarizeMedia streams), F2 fixed (ctx-aware wait; 2 of 4 slots per computer), F3 fixed (refused.per_caller); F4 declined, F5 and F6 recorded; F7 (downscaled copies held for the call) and DeepSeek round 3 findings recorded as named residuals in the problem record
  - id: b7-body-caps-f7-output-budget
    boundary: implement
    identity: cd6c1cd6e (+ nix comment 15e00ccad; decision and sizing e06f5364e, tests ae30081c0)
    proof_refs: [docs/problems/gateway-request-bodies-unbounded-2026-10-22.md "Repair of F7 (FP0 round 5)", "internal/provider/media_test.go TestImageOutputIsHeldUntilTheCallReleasesIt, TestImageOutputOneCallerHoldsAtMostItsShare, TestImageOutputIsGivenBackOnErrorAndPanic and internal/gateway/body_caps_test.go TestBodyCapsUnitMemoryArithmeticFitsUnderGOMEMLIMIT (5.80 GiB of 6), TestBodyCapsHealthReportsImageMedia: neither package builds on 142c17bd1 (new API); with only the API shimmed onto 142c17bd1 each fails behaviorally (the arithmetic at 6.38 GiB with four slots)", "go test -race ./internal/gateway -run 'TestBodyCaps|TestModelScheduler': green", "go test ./internal/gateway/... ./internal/provider/...: green"]
    rollback_ref: git revert of cd6c1cd6e (the bunch 116 code stays; F7 returns as a residual)
    disposition: F7 fixed by FP0's decision (an output byte budget that calls wait on, no re-encode); decode slots 4 to 3 and 1 per computer to make room, with the full arithmetic in the nix comment, section 6 and the problem record
  - id: res-12
    boundary: define
    identity: this file, section 5
    proof_refs: []
    rollback_ref: not_applicable
    disposition: designed only; needs durable parked calls (SP1) and a provider batch adapter
---

# FP7: gateway identity and fairness

Four changes to one process, kept in separate commits so each lands and rolls
back alone (SEC-08's gateway half uses SEC-05's bound bearer, so it follows
it; RES-11 and SEC-16 are independent of both). Problem records:
[SEC-05](../problems/gateway-bearer-works-from-any-computer-2026-10-21.md),
[SEC-08](../problems/claude-delegate-key-sits-on-the-guest-disk-2026-10-21.md),
[RES-11](../problems/gateway-model-calls-have-no-scheduler-2026-10-21.md).
Standing questions answered: single authority for a computer's pin is the
gateway's identity store, moved only by vmctl's ensure (5); the pin survives
a restart in the store and the old key name is kept for rollback (8); every
state is readable from `/health` without SSH (9).

## 1. SEC-05: bind the bearer to its computer

**Rule.** An identity has one pin, the guest address the host attests. Every
gateway route that authenticates a computer (inference, OpenAI-compatible
chat and models, search, source-api, Jev judgments) compares the call's
source address with the pin and refuses a mismatch.

| mode | unpinned bearer | pinned, same address | pinned, other address |
|---|---|---|---|
| `off` | passes, nothing written | passes | passes |
| `observe` | passes, counted, nothing written | passes | passes, logged "would refuse", counted |
| `enforce` (unset and unknown values) | binds to the caller's address (first use), passes | passes | refused (429 + Retry-After 2), logged, counted |
| `strict` | refused (429 + Retry-After 2) | passes | refused (429 + Retry-After 2) |

The Jev route follows the same columns with its own 403 for every refusal,
and writes a first-use pin in `off`, `observe` and `enforce`, as it did
before SEC-05; only `strict` refuses an unpinned bearer (bunch 4b review,
SEC-05 finding 14, which corrects finding 13's observe reading). The Anthropic relay ignores the mode: it serves only
a pinned bearer from its pin's address and writes nothing (batch 1 review).

A binding refusal is still a refusal (the call never reaches a provider,
the body names no address), but it is a 429 with a short `Retry-After`, not
a 401. The reason is in the boot paths: a snapshot resume reuses the
snapshot's host port, so its guest address does not change; a cold boot
issues a new credential, which is unpinned and binds on first use; but a
deploy refresh and a recovery keep the token and take a new address from the
next host port, and vmctl re-ensures the credential only after the boot
returns, once the guest is healthy, while a planned restart resumes work at
once. The guest waits out a 429 `Retry-After` up to ten times without
counting it against its attempts, so the refusal heals when the host moves the
pin. Other authentication failures stay the generic 401; a pin that cannot
be written is 503. The Jev route keeps its own 403.

The host moves the pin through the existing ensure route (`peer_ip`); the
guest cannot. A pin the first use set is overwritten by the host's next
ensure with the true address, so an attacker who wins the first-use race
loses it back. The pin's JSON key stays `JevPeerIP` so a stored file and a
rollback keep working.

**Rollout.** Nix sets `observe` (a `gatewayBearerBindingMode` let-binding,
like `internalAuthMode`). FP0 reads `/health`: flip to `enforce` when
`would_refuse` has stayed 0 for the paper and the test computers
across a planned refresh and a deploy, and `unpinned` has drained. `strict`
later, once no running computer is unpinned. Rollback: `observe`, then `off`.

**Rotation.** A new secret reaches a guest only at a cold boot
(`IssueCredential` revokes the old one); a planned refresh and a recovery keep
the token (`resolveGatewayToken` reads `persist/gateway-token`) and nothing
extended its expiry, so a computer that is only ever refreshed would lose its
gateway one TTL (720 h) after its last cold boot, with the ensure route
refusing to renew it. Built, as the small safe part: **host-attested
renewal**. An ensure of an active credential with less than half its life
left extends it by one TTL (one write, with the pin move if any; fail closed
if the write fails); an expired or revoked credential is still refused;
using a bearer never renews it; `/health` `credentials` reports active,
expiring within 7 days, hours to the soonest expiry and renewals, with no
name or secret. Designed, not built: live rotation of the secret. It needs the
host to hand a running guest a new bearer, and no such channel exists (the
credential disk is read at boot). (1) The gateway keeps the old hash valid for
an overlap of a few minutes (`PreviousTokenHash`, `PreviousExpiresAt`); (2)
vmctl pushes the new bearer to the guest runtime on a hostauth-guarded route
that rewrites its in-memory copy of `RUNTIME_GATEWAY_TOKEN_FILE`; (3) the pin
and the new hash move together; (4) a failed push leaves the old bearer valid
until its TTL. With that, the TTL can fall from 720 h to days. Slice owner:
vmctl lane, after enforce.

## 2. SEC-08: Option K, the gateway holds the delegate's key

**What the delegate's process needs.** `claude -p` (Claude Code 2.1.197)
speaks the Anthropic Messages API: `POST /v1/messages` (`stream: true`, server-sent
events), `POST /v1/messages/count_tokens`, `GET /v1/models`; headers
`anthropic-version`, `anthropic-beta`, `content-type`, `accept`, and a key
(`x-api-key` from `ANTHROPIC_API_KEY`, or `Authorization: Bearer` from
`ANTHROPIC_AUTH_TOKEN`); a base URL (`ANTHROPIC_BASE_URL`). It resends the
whole conversation on each request.

**Which credential.** Only a Claude Console API key. The owner's
subscription token cannot be held or relayed by a service (the terms quoted in
the report, section 2.2); the delegate stays on Option T until the owner
creates a Console key and places it in `gateway-provider.env` on Node B as
`ANTHROPIC_API_KEY`. No key, no relay: the route answers 503.

**How the gateway authenticates the delegate (gateway half, built).**
Route `POST|GET /provider/anthropic/v1/...`. In order: method and path
allowlist (the three above, nothing else); the computer's bearer, validated
without touching its pin; the computer must be on the relay allowlist
(`GATEWAY_ANTHROPIC_RELAY_IDENTITIES`) and inside the window
(`GATEWAY_ANTHROPIC_RELAY_EXPIRES`), and a key must exist; the bearer's pin
must already name the caller's address, whatever `GATEWAY_BEARER_BINDING`
says (unpinned or mismatched: 429 + Retry-After, nothing relayed, nothing
written); the body is read under a 32 MiB cap and
its `model` must be on the relay's model allowlist (default
`claude-sonnet-5-5`, the delegate's model); a per-computer in-flight cap (2);
then every credential-bearing request header is dropped, `x-api-key` is set
from the gateway-held key, and only `anthropic-version`, `anthropic-beta`,
`content-type` and `accept` pass. Responses stream with a flush per read;
hop-by-hop and `set-cookie` headers are dropped; a client disconnect cancels
the upstream call. Logs carry computer, model, status, bytes, and token
counts read from the usage events: never a body, never the key. An
ambiguous `model` (repeated, another case, not a string) is refused so the
allowlist checks the model the upstream will run; redirects are never
followed; error bodies are redacted of the key.

**Identity finding.** The gateway's identity is the VM id vmctl issues the
bearer for (`issueGatewayTokenAt(gwURL, own.VMID)`), not the ComputerID in
`CHOIR_CLAUDE_DELEGATE_COMPUTERS`. So the relay allowlist is keyed by gateway
identity, a second list for the same computer. At activation the two must name
the same computer (a mismatch refuses, never widens). The better shape, for the
activation slice: vmctl's ensure names the stable ComputerID, the gateway
stores it on the identity, and the relay allowlist uses vmctl's own list, so
there is one authority. That touches `internal/vmctl/ownership.go`
(`ensureExistingGatewayCredential`) and is left to the host lane.

**Guest half (not built; the precise change set).**
1. Capsule egress (`internal/capsule/egress.go`, `executor.go`): a route
   `anthropic.choir.internal` that accepts POST with a body and streams the
   reply, strips any `x-api-key` and `Authorization` the capsule sends, and
   adds `Authorization: Bearer <the computer's gateway bearer>` read by the
   guest core (never the capsule) from `/run/choir-bootstrap/gateway-token`,
   forwarding to `/provider/anthropic/v1/...` on the gateway. Today routes are
   plain-HTTP GET/HEAD with no body and no injection.
2. Claude mode environment (`internal/claudedelegate/spec.go`,
   `cmd/capsule-broker/delegate.go`, `internal/capsule/claude_delegate.go`):
   `ANTHROPIC_BASE_URL=http://anthropic.choir.internal`, a non-empty
   placeholder `ANTHROPIC_API_KEY`, no `CLAUDE_CODE_OAUTH_TOKEN`; the F4 to
   F7 tests (token never in env, logs, files) become "no secret exists".
3. `internal/vmmanager/claude_delegate.go`: in gateway mode write only the
   grant (`mode: gateway`), never `claude-oauth-token`.
4. `nix/node-b.nix`: drop the token stage and `CHOIR_CLAUDE_DELEGATE_TOKEN_FILE`
   for the gateway mode; add the gateway's relay env from the shared let.
5. `scripts/claude_delegate_probe.py`: assert the credential disk holds no
   token and the gateway journal shows the relay calls.
Proof when built: the credential disk lists no `claude-oauth-token`; the
egress ledger has no `api.anthropic.com` CONNECT; the gateway journal shows
`anthropic relay` lines for the computer; another computer's bearer is
refused.

## 3. RES-11: a fair model-call scheduler

**Rule.** With `GATEWAY_MODEL_SCHEDULER=on`, inference and OpenAI-compatible
chat calls take a slot from one gateway scheduler instead of the fixed
60-per-minute window. Global in-flight cap (default 24), per-computer cap
(default 6), per-computer FIFO queue of at most 256, a wait of at most 120 s
after which the caller gets 429 with `Retry-After` (the guest already waits
that out). The wait counts against the call's ten-minute deadline, so wait
plus call stay inside the 10m30s write and client timeouts, and a configured
wait is capped at five minutes. When a slot frees, the next call comes from the eligible computer
with the fewest in flight, ties to the one served longest ago: a computer
with one call is not queued behind another's backlog. A slot is held for the
whole call, including a stream, and is released on every exit path. Search,
Jev and source-api keep the old limiter. `off` (the default) is exactly the
old behavior.

**Fairness, not a spending cap.** There is no per-minute count. The bounds
are concurrency, queue depth and wait, which keep a runaway loop from
reaching the provider at full speed while a legitimate burst queues. The
in-guest 1.5 s spacer (`internal/agentcore/research_children.go:30-37`) stays
until the scheduler has run on staging; removing it is a guest change that
follows.

## 4. SEC-16: the front-importance timer stops holding the Jev key

The `go-choir-front-importance` timer loaded `gateway-provider.env` and
called OpenRouter with the gateway's transport code, outside the gateway's
meter. Now the gateway has a host route, `POST /provider/v1/host/judgments`
(`internal/gateway/host_judgment.go`): host-internal credential only
(loopback, the marker and hostauth; a computer's bearer is refused), an
allowlisted caller name (`X-Choir-Host-Caller: front-importance`, mapped to
the label `host:front-importance` by the gateway's own table), no scope or
model accepted, the exact upstream body returned, one metering log line and
one transport receipt per call. The scorer's default decider is the gateway
client (`internal/frontimportance/gateway_decider.go`); the old direct path
is `FRONT_IMPORTANCE_DIRECT=1`, kept for one release as the rollback. A
separate nix commit then removes `EnvironmentFile` (and the receipt-path
variable) from the unit, so the key is held only by the gateway; land it
after the gateway route is proved on staging. Unchanged: the scorer is best
effort; a failure logs and leaves the previous scores in place. Batch 1
carries the code only: the nix removal (8c8bd7e54) is reverted out of it
(9d53138fc) and FP0 cherry-picks it after deployed proof 7.

## 5. RES-12: batch endpoints for calls that can wait (design only)

Intake labels, support judgments and front-importance scores do not need
an answer in seconds. A batch endpoint would take a call with
`deadline: wait`, return a ticket at once, and let the gateway hold the call
in the same fair queue behind live calls, send it through a provider's batch
API where one exists (about half the price) or through the ordinary path when
the queue is idle, and keep the answer until the computer fetches it. It needs
three things this pass does not have: durable parked calls (a gateway restart
or a computer's hibernate must not lose a ticket; the CPU-efficiency report's
parked-call design, SP1), a per-provider batch adapter, and a guest client
that polls instead of waiting. Restart rule: a ticket whose computer crashed is
closed "interrupted by a restart", never re-dispatched, but its usage is still
recorded. Build after the scheduler has run on staging; the scheduler's
queue and `/health` counters are its measurement.

## 6. B7: request body caps (the scheduler's precondition)

Problem record:
[gateway-request-bodies-unbounded](../problems/gateway-request-bodies-unbounded-2026-10-22.md).
The inference, OpenAI chat and search routes decoded a guest's body with
no size limit; the only bound was the 120 s read timeout.

**Rule.** Every decoded body goes through one helper
(`internal/gateway/body_caps.go`, `decodeBody`). A declared
`Content-Length` over the cap is refused before a byte is read. A streamed
body is cut one byte past the cap (`http.MaxBytesReader`, which also closes
the connection). Both answer 413 with `request body too large: the <route>
limit is <size>`.

| route class | routes | limit | evidence for the limit |
|---|---|---|---|
| model | inference, OpenAI chat | 160 MiB (ceiling 576 MiB in observe) | 20 images at the gateway's own 5 MiB downscale bound are 133.3 MiB as base64, plus a 16 MiB 1M-token context: about 150 MiB (raised from 64 MiB in the bunch 88 round, F3; `DefaultModelBodyLimit`). The ceiling is the media contract's maximum, 20 images of 20 MiB as base64 plus a context |
| search | search | 1 MiB | a query, a count, a depth |
| source_api | source-api | 64 KiB (as before) | its own `sourceAPIMaxRequest` |
| judgment | Jev, host judgments | 1 MiB (as before) | `jevTransportMaxBodySize` |
| operator | credentials issue and ensure, search health reset, breakers reset | 64 KiB | one token, one address or one name; host-only routes |

Host fetch (2 MiB + 64 KiB, 413) and the Anthropic relay (32 MiB, 413)
keep their own caps. `GATEWAY_BODY_LIMITS` (e.g. `model=96MiB`) overrides a
limit, read once when the handler is built; an unreadable value is logged
and the default kept.

**Bounds in every mode (bunch 88 and 109 rounds).** Observe applies to the
model limit only; these bind in every body caps mode and whatever the
scheduler's. `limits.in_flight` (1 GiB) bounds the bytes of all bodies held
at once; a model body stays counted until its call ends. One caller (the
computer behind the bearer, or the host caller) holds at most
`limits.per_caller` of it (in_flight/4, 256 MiB), except that its first
body always fits; a caller over its share waits behind its own bodies only.
A body that does not fit waits, then is answered 429 with `Retry-After`,
unread. The wait lasts at most `GATEWAY_BODY_READ_TIMEOUT` (60 s) from
`decodeBody`, which on the model routes runs after the slot is granted, so
a long queue wait no longer spends the server's 120 s read timeout; the
read then has its own 60 s from the moment the body fits, and a body cut
there is answered 503 with `Retry-After`. Both are transient to the guest.
A body passed over to its deadline because its own caller held its share
is counted `refused.per_caller`, not `refused.in_flight` (bunch 116).
Images are decoded at most 3 at once process-wide
(`provider.MaxConcurrentImageDecodes`), at most 1 of them for one computer
(`MaxImageDecodesPerCaller`), each from its base64 inside its slot. The
downscaled images all calls hold, as base64, are bounded by
`provider.MaxImageOutputBytes` (192 MiB, 96 MiB per computer, a
computer's first call always fits): a call reserves its worst case
(6.67 MiB per base64 image) once before its first decode, keeps what it
made, and gives it back when its upstream call returns (FP0 round 5, F7).
A call whose context ends while it waits for a slot or for output bytes
leaves the queue, answered without an upstream call and never a 400.
`/health` `image_media` reports the slots and the output budget, their
use, waits and refusals. The gateway unit has `MemoryMax=8G` and
`GOMEMLIMIT=6GiB` as a backstop. Its worst case, every term at its bound:
bodies 3 x 1 GiB (the decoder's buffer, or the decoded request and its
upstream body) = 3.00 GiB, decodes 3 x about 620 MB = 1.73 GiB, output
3 x 192 MiB (the string, its upstream copy, the encoder's growth) =
0.56 GiB, runtime about 0.5 GiB: about 5.80 GiB, under `GOMEMLIMIT` and
1.65 GiB under `MemoryMax` (the nix comment and the problem record carry
it; `TestBodyCapsUnitMemoryArithmeticFitsUnderGOMEMLIMIT` computes it from
the constants).

**Observe first, for the model routes only.** The model limit is below the
media contract's ceiling, so it could refuse a real call: a long
conversation that keeps many uploaded photos. A 413 is not transient to the
guest, so that turn would fail. `GATEWAY_BODY_CAPS=observe` (the nix value
for the first deploy) serves a model body between 160 MiB and 576 MiB and
logs `body cap would_refuse route=model caller=<vm> bytes=<n> limit=<l>`;
over 576 MiB it is refused. Unset or unknown enforces. Every other route
enforces in every mode: no legitimate body comes near its cap. `/health`
`body_caps` reports the mode, the limits, and per route class `refused`,
`would_refuse` and `max_seen` (the largest body read by this gateway
process). Enforce when `would_refuse.model` stays 0 and `max_seen.model`
stays under the limit for a day of observe.

**The scheduler.** Inference and OpenAI chat now take their RES-11 slot
before they read the body. A queued call holds an unread socket, not a
decoded body. With the scheduler on, body memory is bounded by the global
cap (24) times the body cap, and in every mode by `in_flight`. Off, the slot is a no-op and the order does not
matter. `ModelScheduler.Acquire` now returns the context's error at once
for a caller already gone, never queued and never granted. A slot that
arrives together with the cancellation is given back.

**Order.** Land the caps, then deploy in observe. Run proofs 10 and 11, then
12 after a day. Flip to enforce as its own deploy and run 12 again. Only
then flip the scheduler on and run proof 9. While the scheduler is on, the
caps should already be in enforce: in observe, 24 slots times the 576 MiB
ceiling is no bound.

**Residuals.** (a) Closed in code by the bunch 88 round: `in_flight` is
the aggregate bound with the scheduler off. (b) Bounded by the bunch 109
and 116 rounds and FP0 round 5: images are decoded at most 3 at once, at
most 1 for one computer, about 620 MB each at 50 Mpx; the downscaled
output is at most 192 MiB, 96 MiB for one computer; the shares are per
computer, not per owner. The worst case is computed, not measured. (c) The refusal log has one
line per refusal and no throttle. (d) No real body sizes have been measured
yet. The observe deploy's `max_seen.model` is the first measurement.
(e) Proof 11 reads no large under-ceiling model body (review F6). (f) The
gateway's normal RSS is unmeasured; `MemoryMax=8G` is generous until
proof 10 records `MemoryPeak`.

## Batch 1 review (2026-10-21, candidate b80a9bcac)

GPT-6.1-sol sent the candidate back; Gemini 4 Argon accepted with edits.
Their blocking findings, recorded before any fix:
- SEC-08 findings 11 and 12
  ([record](../problems/claude-delegate-key-sits-on-the-guest-disk-2026-10-21.md)):
  the relay followed the general binding mode (a copied bearer was served in
  observe), and a disabled or unlisted relay call wrote the shared pin.
- SEC-05 finding 13
  ([record](../problems/gateway-bearer-works-from-any-computer-2026-10-21.md)):
  the Jev route wrote the shared pin on first use in every mode, bypassing
  strict and writing in observe.
- SEC-16 landing order: the candidate carried the nix commit that removes
  `EnvironmentFile` from `go-choir-front-importance` (8c8bd7e54) in the same
  batch as the host route, so the first deploy would remove the
  `FRONT_IMPORTANCE_DIRECT=1` rollback before deployed proof 7 has shown the
  route works, contradicting section 4 and the rollback text above. Repair:
  the batch reverts that commit; FP0 re-applies it (cherry-pick 8c8bd7e54)
  as a separate landing after proof 7 passes.
- RES-11 finding 11
  ([record](../problems/gateway-model-calls-have-no-scheduler-2026-10-21.md)),
  non-blocking: the queue wait sits outside the call's ten-minute deadline,
  so wait plus call can pass the 10m30s write timeout.

## Bunch 4b review (2026-10-21, candidate 6dafee596)

GPT-6.1-sol sent the delta back with one blocking finding; Gemini 4 Argon
accepted. Accepted: SEC-05 finding 14
([record](../problems/gateway-bearer-works-from-any-computer-2026-10-21.md)),
observe mode had dropped Jev's first-use pin, so a copied bearer was served
before the host's ensure. Repair: Jev pins on first use in every mode but
strict; strict still refuses an unpinned bearer. Argon's non-blocking note
(`BindPeer` has no production caller) stays as a residual: its registry
tests still cover the pin semantics, so it can be deleted later.

## SEC-05 enforce flip (FP0, 2026-10-22)

Revised after review (bunch 54: flash accept; DeepSeek and a Sonnet reviewer accept with edits; their texts are in FP0's run folder `_runs/bunch54`).

**The counters reset with the gateway.** `would_refuse` and `refused` are in-memory counters, and the gateway restarts at every deploy. So each reading covers only the current gateway process. The table gives each process's start time and the pin moves inside it. Each move is a journal line `Jev bearer of autoputer <vm> moved to its current guest address <addr>`, written when vmctl's ensure moves a stale pin to the guest's new address. A move means the stale-pin window (guest healthy, pin not yet moved) occurred for that computer inside the counted window.

| gateway process (start, UTC) | pin moves inside it (journal) | reading | would_refuse | refused |
|---|---|---|---|---|
| 07:10:45 | (not collected) | 07:25 | 0 | 0 |
| 11:26:13 (deploy N) | 4: the fleet 11:26:35, vm-48bc0981 11:26:47, the paper 11:27:21, 71afbf14 11:28:37 | 12:00 | 0 | 0 |
| 12:05:41 (deploy O) | 7: vm-48bc0981 12:06:00, the fleet 12:06:16, the paper 12:07:08, the owner's computer 12:09:06 (cold boot), 71afbf14 12:13:13, 12:14:38, 12:15:41 (cold boots during the wake-orphan proof) | 12:31 | 0 | 0 |
| 12:32:09 (deploy P) | 6: 71afbf14 12:32:27, the owner's computer 12:32:36, the paper 12:33:10, the fleet 12:34:27, vm-48bc0981 12:34:38, the FP2 test computer 12:34:53 (wake) | 12:50 | 0 | 0 |

Earlier processes on 2026-10-22 (10:39:54, 10:56:47) show the same pattern: four moves each, all after the gateway's start.

**What this shows and what it does not.**
- Every computer running at 12:50 had its pin moved by the current process: the paper, the owner's computer, the fleet, vm-48bc0981, 71afbf14, and the FP2 test computer. So the six running computers are among the 51 pinned, with pins at their current addresses. The bare count did not show that.
- In 17 pin moves across three counted processes, no call met a stale pin.
- This bounds the window in practice. It does not prove the window is empty. Zero hits in 17 moves still allows a per-move hit rate of up to about 16% (95%).
- The argument for enforce is therefore not that the window is empty. It is that a hit fails closed and heals: a 429 with `Retry-After: 2`, and vmctl's ensure moves the pin within the refresh. The cost of a hit is the residual below.

**What enforce changes for an unpinned bearer.** Observe passes an unpinned bearer and writes nothing. Enforce passes it and writes a first-use pin to the caller's address (`bearer_binding.go:194`, `CheckPeer` with bind true; `first_use_bound` counts it). If that write fails, enforce answers 503 and writes no pin, where observe would have passed the call. A first-use pin moves to the true address at the host's next ensure for that computer. Until then, the legitimate guest is refused if a different runtime peer (loopback included) used the bearer first. A pinned bearer also becomes usable on the Anthropic relay, which accepts a bearer pinned to the caller's address. The relay is dark today (no key, empty allowlist), so this changes nothing yet. The 221 unpinned bearers are credentials of computers that do not run, so none of this touches running computers. Draining them remains strict's criterion.

**Accepted residuals.**
- **A hit costs non-model calls more.** Only `Provider.call` (non-streaming inference) waits out a 429, for up to about 20 s. Streaming inference, guest search, source-api and host-fetch fail at once. An exhausted wait is booked as a model failover for that run.
- **The paper is the most exposed computer.** It takes a planned refresh while busy and resumes at once. A hit there costs a failed source poll or a failover, not data.
- **A failed ensure has no retry timer.** vmctl retries it only on its next trigger: a boot, a refresh, a reattach, or a status lookup. A failed ensure can therefore leave a stale pin until one of those.

**The watch after the flip** replaces "refused stays 0", which would misread a healed window as a failure. For one hour:
- read `/health` `bearer_binding` (`mode` enforce, `refused`, `first_use_bound`);
- read the gateway journal's `bearer binding enforce: refused <vm> from <addr>` lines beside the `moved to its current guest address` lines;
- read the paper's failed source polls and model-failover retries.

A refusal is the design while its computer's pin is stale. The rollback trigger is a computer still refused after the journal shows its pin moved to its current address, or the paper's polls failing for more than one cycle. Proof step 4 now reads it that way (`scripts/proofs/fp7-gateway.sh step_4`).

**Step 5 is the security proof, and it is not yet run.** Step 5 shows that a token copied between two test computers is refused. Step 4 proves only that nothing breaks. Unit tests cover the refusal on every route (`TestBearerBindingRefusesAnotherComputersAddressOnEveryRoute`, `TestBearerBindingOwnCallsPassAndOthersFailEndToEnd`). On staging it needs a shell in two disposable test guests. FP0 will try it after the flip through a host-side shell into two test guests. If FP0 has no such route, it stays an owner-visible residual: SEC-05 stays unticked in the checklist until step 5 runs.

**Rollback.** Set `observe` and redeploy. There is no runtime switch: the mode is read at gateway start. Pins written under enforce persist, and in observe they are only counted. Re-enforcing after a rollback can refuse a legitimate computer whose first-use pin was written by another caller, until that computer's next ensure moves it. A refresh of that computer clears it.

## Deployed proofs for FP0 (exact steps; none needs a secret)

SEC-05, after deploy with `observe`:
1. Read the gateway's `/health` (port 8084 on Node B, through the ops path
   FP0 already uses; it is public and holds no secret): expect
   `bearer_binding.mode=observe`, `pinned`, `unpinned`, `would_refuse`.
2. Start a test computer, make one model call through the product (a Texture
   prompt). `would_refuse` must not rise.
3. Hibernate and wake it; repeat; then a planned refresh of the paper's
   computer; repeat. `would_refuse` must stay 0.
SEC-05, after the flip to `enforce`:
4. The same test computer's model call still passes; `refused` stays 0.
5. Cross-computer refusal, on two disposable test computers T1 and T2 only:
   from T1's guest run one gateway call with T1's own token file (passes),
   then copy that file into T2's guest and call from there (refused with 429
   and `Retry-After: 2`; the gateway log has `bearer binding enforce: refused`
   naming the computer and both addresses; the provider is never called).
   Revoke by stopping and removing both test computers.
SEC-08 gateway half: with no key and an empty allowlist the route is dark:
6. `POST /provider/anthropic/v1/messages` from a test computer's guest with
   its own bearer returns 403 (not on the relay allowlist) or 503 (no key);
   nothing is relayed. `/health` shows `anthropic_relay` as `off`. It goes
   live only with the owner's Console key, the allowlist and the guest half.
SEC-16, after the code deploy (the unit still holds the key):
7. Wait one timer tick (10 minutes). The front-importance journal says
   `Jev reached by gateway`; the gateway journal has `host judgment from
   host:front-importance served` lines; `scores.json` is rewritten. Then the
   nix change that removes the unit's `EnvironmentFile` (FP0 cherry-picks
   8c8bd7e54 as its own landing, only after the first half passed): the next tick still
   scores, and `systemctl show go-choir-front-importance -p Environment`
   shows no `OPENROUTER_API_KEY` (the unit has none).
8. From a guest address (a test computer's shell), `POST` to the host route
   with the computer's own bearer: expect 403. From the host without the
   internal credential: expect 403.
RES-11, after flipping `GATEWAY_MODEL_SCHEDULER=on`:
9. From one test computer, fire 40 concurrent small model calls through the
   product path while a second test computer makes one call: the second
   call's latency stays near a single call's; all 41 complete; `/health`
   shows queued and wait counters moving and returning to zero.
B7 body caps (`scripts/proofs/fp7-gateway.sh --step 10|11|12`; run before
the scheduler flip):
10. After the deploy with `gatewayBodyCaps = "observe"`: `/health`
    `body_caps.mode=observe`, limits model 167772160, search 1048576,
    `in_flight` and `per_caller` present, `refused.in_flight` and
    `refused.per_caller` read apart, `image_media` present with
    `decode_slots` 3 and `output_limit` 201326592; the
    journal has `gateway: body caps: mode=observe ...`; the gateway's
    `MemoryCurrent` and `MemoryPeak` are saved as the baseline.
11. From a disposable test computer's guest shell, as root:
    `GW=$(sed -n 's/^RUNTIME_GATEWAY_URL=//p' /run/go-choir-autoputer.env)`,
    `TOK=$(cat /run/choir-bootstrap/gateway-token)`. Then:
    (a) a streamed 100 MiB search body, which must answer 413, or curl
    reports the connection closed mid-upload:
    `{ printf '{"query":"'; head -c 104857600 /dev/zero | tr '\0' a; printf '"}'; } | curl -sS -w ' %{http_code}\n' -X POST -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' -T - "$GW/provider/v1/search"`;
    (b) a model body declaring 700 MiB, over the observe ceiling, which
    must answer 413 before a byte is read:
    `head -c 734003200 /dev/zero | curl -sS -w ' %{http_code}\n' -X POST -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' -H 'Content-Length: 734003200' -T - "$GW/provider/v1/inference"`;
    (c) one ordinary Texture prompt on the same computer, which still
    answers. Then rerun step 11. It checks that `refused.search` and
    `refused.model` rose and that the journal has `body cap refused
    route=search` and `route=model` naming the computer. It also checks
    that memory stayed flat: `MemoryPeak` and `MemoryCurrent` each rose
    less than 256 MiB over the baseline.
12. After a day of observe: `would_refuse.model` is 0 (or raise the limit
    past `max_seen.model`); flip `gatewayBodyCaps` to `enforce` as its own
    deploy; rerun: mode enforce; repeat 11 (b) with 200 MiB streamed
    instead of declared: 413 `the model limit is 160 MiB`, and the memory
    peak rises at most about twice the limit.

Mutation class: red. Protected surfaces: gateway and provider calls, the
identity store, credential custody. Heresy delta (running):
discovered: the Jev pin's first-use authority; refresh and recovery reuse
a token across a changed address (every route, not only Jev); the
subscription token cannot be relayed; the gateway's identity is the VM id,
not the ComputerID; two clocks for one call decision; a doubly-keyed JSON
`model` is a parser-differential risk; introduced: a retry hint on a binding
refusal, a dark guest-reachable relay route, a shared scheduler pool (each
behind a switch); repaired: in code, SEC-05, SEC-16, RES-11 and the gateway
half of SEC-08, none closed until proved on staging. B7 (section 6):
discovered: unbounded guest body decodes on inference, OpenAI chat and
search; a queued model call held its decoded body; a cancelled caller could
be granted a slot; introduced: none; repaired: in code, not closed until
proofs 10 to 12.
