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
    FRONT_IMPORTANCE_DIRECT=1 (SEC-16, while the unit still loads the key).
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
  slice: built and tested locally; waiting for FP0 to freeze, review, land and prove
  source_ref: 26c4e1ed0
  deploy_identity: unknown to this lane
  candidate:
    id: fp7-gateway-identity
    state: ready
    ref: lane/b7-gateway
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
      bearers for), because the gateway does not know the ComputerID.
    kind: operational
    status: settled
    evidence_ref: the Jev pin incident (docs/problems/l2a-merge-recall-on-the-paper-2026-10-14.md finding 2) and hostauth's observe-then-enforce rollout
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      The code paths are as read and tested locally; vmctl re-ensures the
      guest address after every start path; a snapshot resume keeps its
      address, a cold boot issues a new credential, a refresh and a recovery
      keep the token and take a new address; the source address is
      trustworthy because of the S1a tap rules.
    main_uncertainty: >-
      How long the stale-pin window is after a refresh (guest healthy until
      vmctl's ensure returns) against the guest's ten Retry-After waits; the
      observe counts after a planned refresh answer it.
    next_observation: bearer_binding.would_refuse for the paper after a planned refresh in observe mode
  blocker_or_risk: >-
    Option K needs the owner's Console API key (a credential question, not
    a design branch) and the guest half; a stale pin after a boot would delay
    a computer's model calls in enforce mode, which is why enforce is a
    separate, watched flip; the scheduler's caps are guesses until staging
    shows whether they bind.
  next_action: FP0 freezes the candidate, sends the red commits to a panel, lands in the order SEC-05 (with its nix line), RES-11, SEC-08, SEC-16, and runs the deployed proofs below

receipts:
  - id: fp7-docs
    boundary: define
    identity: 24026a9e6
    proof_refs: [docs/problems/gateway-bearer-works-from-any-computer-2026-10-21.md, docs/problems/claude-delegate-key-sits-on-the-guest-disk-2026-10-21.md, docs/problems/gateway-model-calls-have-no-scheduler-2026-10-21.md]
    rollback_ref: git revert 24026a9e6
    disposition: problems documented before any fix
  - id: sec-05
    boundary: implement
    identity: 37fc682a3 (+ nix af71fec80)
    proof_refs: [internal/gateway/bearer_binding_test.go, "go test ./internal/gateway -race: green"]
    rollback_ref: GATEWAY_BEARER_BINDING=off, or git revert
    disposition: built; not deployed; not closed until enforced on staging
  - id: sec-08-gateway-half
    boundary: implement
    identity: 496db5009
    proof_refs: [internal/gateway/anthropic_relay_test.go, "seven relay mutations each fail the tests"]
    rollback_ref: leave ANTHROPIC_API_KEY unset, or git revert
    disposition: built, dark; the guest half and the owner's Console key are open
  - id: res-11
    boundary: implement
    identity: 21c073fb0 (+ nix 6e5b99cc9)
    proof_refs: [internal/gateway/model_scheduler_test.go, "six scheduler mutations each fail the tests"]
    rollback_ref: GATEWAY_MODEL_SCHEDULER=off (the default), or git revert
    disposition: built, off; to be flipped after the staging proof
  - id: sec-16
    boundary: implement
    identity: 704ef5fef (+ nix 738a821d0)
    proof_refs: [internal/gateway/host_judgment_test.go, internal/frontimportance/gateway_decider_test.go, "go build ./cmd/corpusd"]
    rollback_ref: FRONT_IMPORTANCE_DIRECT=1 while the unit still loads the key, or git revert
    disposition: built; the scorer defaults to the gateway; the nix change that removes the key from the unit lands after the staging proof
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
(`GATEWAY_ANTHROPIC_RELAY_IDENTITIES`) and inside the window
(`GATEWAY_ANTHROPIC_RELAY_EXPIRES`); the body is read under a 32 MiB cap and
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
effort; a failure logs and leaves the previous scores in place.

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
   nix change that removes the unit's `EnvironmentFile`: the next tick still
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

Mutation class: red. Protected surfaces: gateway and provider calls, the
identity store, credential custody. Heresy delta (running):
discovered: the Jev pin's first-use authority; refresh and recovery reuse
a token across a changed address (every route, not only Jev); the
subscription token cannot be relayed; the gateway's identity is the VM id,
not the ComputerID; two clocks for one call decision; a doubly-keyed JSON
`model` is a parser-differential risk; introduced: a retry hint on a binding
refusal, a dark guest-reachable relay route, a shared scheduler pool (each
behind a switch); repaired: in code, SEC-05, SEC-16, RES-11 and the gateway
half of SEC-08, none closed until proved on staging.
