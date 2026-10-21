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
  captured_at: 2026-10-21T18:30:00Z
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
    rejected; and that can hold the Claude delegate's key so the guest never
    carries it. Three separate rollback units: SEC-05, SEC-08 (gateway half),
    RES-11.
  artifact: >-
    Commits on lane/b7-gateway (docs, then SEC-05, SEC-08 Option K gateway
    relay, RES-11 scheduler), each behind a mode switch that defaults to the
    old behavior or to observe, with tests that fail first; the exact
    deployed proofs for FP0; and the guest-side Option K slice written as a
    precise change set (it is not built here).
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
    - action: the deployed proofs listed under "Deployed proofs for FP0" below
      proves: the claims on the deployed gateway, not on a worktree
      evidence_class: deployed proof (FP0 runs them; this lane cannot)
  rollback: >-
    Each of the three behavior commits reverts alone with git revert, and
    each has a runtime switch that restores the old behavior without a
    deploy of old code: GATEWAY_BEARER_BINDING=off, the relay's empty
    allowlist (the default), GATEWAY_MODEL_SCHEDULER=off (the default).
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
    - The Jev route's strict pin (BindPeer) is unchanged in every mode.
    - Who may run the delegate, and where, does not change: one test computer, the existing expiry.
    - No secret in the repository, a log, a test, a response or a guest disk; tests use fake keys.
    - Search, Jev judgments and source-api keep their own buckets and the old limiter.
    - A crash restart resumes nothing; the registry and the pin file are the only durable state this adds, and neither resumes work.
  excluded:
    - Moving the owner's subscription token through any relay (forbidden by Anthropic's terms).
    - The guest half of Option K (capsule egress route, broker, vmmanager, nix): written as a change set, not built here.
    - Live in-place bearer rotation (needs a host-to-guest channel that does not exist): designed, not built.
    - Removing the in-guest 1.5 s spacer (a guest change that follows the scheduler's staging proof).
    - SEC-16 and RES-12 unless time remains.
  protected_surfaces: [gateway and provider calls, gateway identity store, auth between guests and the host, credential custody (the relay holds a provider key)]

now:
  status: working
  slice: docs commit; then SEC-05 observe/enforce bearer binding
  source_ref: 26c4e1ed0
  deploy_identity: unknown to this lane
  candidate:
    id: none
    state: none
    ref: lane/b7-gateway
    base: 26c4e1ed0
    digest: none
    scope: [docs/definitions/choir-launch-fp7-gateway-identity-2026-10-21.md, docs/problems/gateway-bearer-works-from-any-computer-2026-10-21.md, docs/problems/claude-delegate-key-sits-on-the-guest-disk-2026-10-21.md, docs/problems/gateway-model-calls-have-no-scheduler-2026-10-21.md]
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
      nix sets observe first; unset is enforce; an unknown value is enforce.
      Ship the scheduler and the relay dormant (off, empty allowlist).
    kind: operational
    status: settled
    evidence_ref: the Jev pin incident (docs/problems/l2a-merge-recall-on-the-paper-2026-10-14.md finding 2) and hostauth's observe-then-enforce rollout
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      The code paths are as read; vmctl's ensure attests the guest address on
      every boot path; the source address is trustworthy because of the S1a
      tap rules.
    main_uncertainty: >-
      Whether every boot path of a token-reusing restart (recover, snapshot
      resume) runs vmctl's ensure before the guest's first gateway call; the
      observe counts on staging answer it.
    next_observation: bearer_binding.would_refuse for the paper after a planned refresh in observe mode
  blocker_or_risk: >-
    Option K needs the owner's Console API key (a credential question, not
    a design branch); a stale pin after a boot would stop a computer's model
    calls in enforce mode, which is why enforce is a separate, watched flip.
  next_action: write the SEC-05 tests (they fail first), then the registry and handler changes

receipts: []
---

# FP7: gateway identity and fairness

Three changes to one process, kept in three commits so each lands and rolls
back alone. Problem records:
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
| `enforce` (unset and unknown values) | binds to the caller's address (first use), passes | passes | refused (401), logged, counted |
| `strict` | refused | passes | refused |

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

**Rotation (designed, not built).** Today the bearer is replaced at every
cold boot (`IssueCredential` revokes the old one), so its age is bounded by
the boot interval; a computer that never cold boots for 720 h would lose its
bearer (the TTL), and the always-on paper depends on deploy refreshes being
more frequent than that. A live rotation needs the host to hand a running
guest a new bearer, and no such channel exists (the credential disk is read
at boot). Design: (1) the gateway keeps the old hash valid for an overlap of
a few minutes (`PreviousTokenHash`, `PreviousExpiresAt`); (2) vmctl pushes
the new bearer to the guest runtime on a hostauth-guarded route that
rewrites `RUNTIME_GATEWAY_TOKEN_FILE`'s in-memory copy; (3) the pin and the
new hash move together; (4) a failed push leaves the old bearer valid until
its TTL. Until then the cheap guard is visibility: `/health` can report the
oldest active credential's age. Slice owner: vmctl lane, after enforce.

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
allowlist (the three above, nothing else); the computer's bearer (bound to its
address by SEC-05); the computer must be on the relay allowlist
(`GATEWAY_ANTHROPIC_RELAY_COMPUTERS`) and inside the window
(`GATEWAY_ANTHROPIC_RELAY_EXPIRES`); the body is read under a 32 MiB cap and
its `model` must be on the relay's model allowlist (default
`claude-sonnet-5-5`, the delegate's model); a per-computer in-flight cap (2);
then every credential-bearing request header is dropped, `x-api-key` is set
from the gateway-held key, and only `anthropic-version`, `anthropic-beta`,
`content-type` and `accept` pass. Responses stream with a flush per read;
hop-by-hop and `set-cookie` headers are dropped; a client disconnect cancels
the upstream call. Logs carry computer, model, status, bytes, and token
counts read from the usage events: never a body, never the key. The allowlist
is a second copy of vmctl's `CHOIR_CLAUDE_DELEGATE_COMPUTERS`; the nix change
at activation derives both from one let-binding so they cannot disagree.

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
that out). When a slot frees, the next call comes from the eligible computer
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
   then copy that file into T2's guest and call from there (refused with 401
   "authentication required"; the gateway log names `bound to another
   computer`). Revoke by stopping and removing both test computers.
SEC-08 gateway half: with no key and an empty allowlist the route is dark:
6. `POST /provider/anthropic/v1/messages` from a guest returns 503 or 403
   (nothing is relayed). It goes live only with the owner's Console key and
   the guest half.
RES-11, after flipping `GATEWAY_MODEL_SCHEDULER=on`:
7. From one test computer, fire 40 concurrent small model calls through the
   product path while a second test computer makes one call: the second
   call's latency stays near a single call's; all 41 complete; `/health`
   shows queued and wait counters moving and returning to zero.

Mutation class: red. Protected surfaces: gateway and provider calls, the
identity store, credential custody. Heresy delta (running):
discovered: the Jev pin's first-use authority, the subscription token
cannot be relayed, two clocks for one call decision; introduced: none;
repaired: none yet.
