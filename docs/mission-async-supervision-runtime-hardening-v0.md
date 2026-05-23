# MissionGradient: Async Supervision Runtime Hardening v0

**Status:** active checkpoint: async supervision is deployed through `a01595f`;
the current fix closes the candidate-source transfer gap found by the first
Chiron rerun.
**Date:** 2026-05-23
**Supersedes local next probe in:** [mission-human-proof-experiment-rerun-v0.md](mission-human-proof-experiment-rerun-v0.md)
**Returns to:** [mission-human-proof-experiment-rerun-v0.md](mission-human-proof-experiment-rerun-v0.md)
**State ledger:** [platform-os-app-state.md](platform-os-app-state.md)

## Supersession Note

This mission remains useful as the broader async-supervision architecture
sketch, but its immediate runtime blocker has moved to
[mission-supervision-runtime-repair-experiment-rerun-v0.md](mission-supervision-runtime-repair-experiment-rerun-v0.md).
The 2026-05-23 staging proof at `846cfbb` reached the runtime gate for the
next sequential experiment probe: request/start/observe/finish used one worker
run, worker `submit_worker_update` was mirrored into the active VText channel,
VText produced an owner-readable dashboard revision after the worker-update
synthesis wake, and run acceptance recorded `staging-smoke-level` without
requiring AppChangePackage evidence. The first Chiron rerun then found a
narrower runtime gap: `finish_worker_delegation` could still return
`worker_run_active` without the active worker/child evidence that super and
VText need for redirection. The current continuation patch makes active finish
return actionable worker evidence and checkpoints only nontrivial active
evidence, avoiding duplicate startup/update noise.

## One-Line Goal String

```text
/goal Run docs/mission-async-supervision-runtime-hardening-v0.md as a Codex-operated MissionGradient mission: harden Choir's runtime for async multiagent supervision, then resume the human-proof four-experiment rerun through Choir-in-Choir. Replace blocking worker delegation with async start/observe/redirect/finish semantics; remove the hidden synchronous request_worker_vm -> delegate_worker_vm transition; preserve super as the sole worker-control authority while allowing VText to ask super clarifying questions; implement typed multi-recipient/copy-aware agent messages so skip-level super -> co-super directives are only allowed when the supervising vsuper is copied atomically; split causal lineage from authority semantics currently overloaded as parent/child; add VText UI cancel that propagates through researcher/super/vsuper/co-super work and leaves resumable VText state; refine verifier authority so it can run code and write scratch tests/evidence without authoring candidate source or promotion state. Land through git/CI/deploy, verify staging identity, prove with product-path VText/Trace/run-acceptance evidence that super can supervise concurrent async worker runs without blocked tool loops or conflicting control, then resume docs/mission-human-proof-experiment-rerun-v0.md starting with Chiron and continuing sequentially only after the supervision loop is stable. Do not hand-code experiment features, keep a synchronous delegate compatibility path, allow private skip-level commands, create duplicate controllers, use raw parent/child wording as authority, rely on local-only proof, or call the experiment mission complete without owner-readable VText dashboards, screenshots/video/benchmarks, package/adoption/rollback evidence, residual risks, and the next realism axis.
```

## Mission Frame

The previous human-proof experiment rerun failed in a useful way: the problem is
not just that prompts need to ask for better VText updates. The runtime control
shape is wrong for supervision.

The current `delegate_worker_vm` path blocks super while it starts and polls a
worker run. Worse, `request_worker_vm` can secretly auto-chain into
`delegate_worker_vm`, so super can lose the ability to observe, answer VText,
or redirect work before it gets another model turn. That makes concurrent
Choir-in-Choir experiments structurally unsafe and explains why the prior
two-lane portfolio run degraded.

This mission fixes the control substrate first. Then it resumes the four
experiments through Choir itself. Codex may edit runtime/harness/platform code
for the substrate, but Codex must not hand-code Chiron, Motion, Liquid, or
Python mode experiment features.

## Real Artifact

The artifact is an async, typed, supervised runtime:

```text
VText mission dashboard
  -> asks super clarifying questions when uncertain
  -> narrates significant updates
  -> never controls workers directly

super
  -> foreground control authority
  -> starts worker delegations asynchronously
  -> observes checkpoints
  -> answers VText
  -> redirects/cancels vsuper when needed
  -> never privately commands vsuper-owned co-supers

vsuper
  -> candidate-world orchestrator
  -> receives every skip-level directive involving its co-supers
  -> curates co-super activity into owner-level updates

co-super
  -> implementation or verifier helper
  -> serves one immediate supervising controller
  -> reports primarily to vsuper

Trace / run acceptance
  -> durable causal proof
  -> signal-focused evidence for LLM content, tool calls, and agent messages
```

After this substrate is proven, the mission resumes the human-proof experiment
rerun:

```text
Chiron Shelf observability
-> process/window/agent animation language
-> Choir Liquid Material Engine
-> Python code mode
```

The experiments remain sequential until the async supervision loop has proved
it can safely supervise more than one worker without blocking or contradictory
control.

## Hard Invariants

- No long-running worker delegation may block super's tool loop until terminal
  state. Super must regain control after bounded start/observe calls.
- No hidden `request_worker_vm -> delegate_worker_vm` transition may start a
  long synchronous worker run behind the model's back.
- Super is the only authority that may redirect, cancel, or reassign worker
  delegation from the foreground.
- VText may ask questions, narrate uncertainty, and request clarification from
  super, but VText does not issue worker-control commands.
- Skip-level `super -> co-super` directives are allowed only if the supervising
  `vsuper` receives the same directive in the same atomic message event.
- A co-super must not be placed in a two-master condition. If atomic
  multi-recipient delivery is unavailable, route through `super -> vsuper ->
  co-super`.
- Causal lineage must remain durable, but authority must not be inferred only
  from vague `parent` / `child` naming.
- VText cancellation must preserve document history and leave the work
  resumable from a later revision.
- Verifiers may run code and write scratch tests, scripts, logs, and evidence.
  They may not author candidate source, publish packages, promote/adopt, grant
  capabilities, or silently mutate active state.
- Canonical user computer state changes only through verified promotion or
  adoption; background/candidate worlds mutate.
- Staging product proof is required for platform behavior claims.

## Design Position

### Multi-Recipient Messages

Prefer atomic multi-recipient typed messages over forced relay for every
skip-level observation. It is cheaper and preserves the information once.

But distinguish delivery from authority:

| Message Class | Sender | Recipients | Control Effect |
| --- | --- | --- | --- |
| `phase_checkpoint` | vsuper | super, VText, Trace | none until super acts |
| `evidence_ready` | vsuper | super, VText, Trace, Apps & Changes | review/adoption may inspect |
| `blocker` | vsuper/co-super | immediate supervisor, super, VText, Trace as appropriate | super decides next control step |
| `clarification_request` | VText | super | super may answer or redirect |
| `directive` | super | vsuper; optionally co-super copied atomically | worker control |
| `cancel` | user/VText UI via runtime; super/vsuper controllers | active run graph | bounded cancellation |
| `narrative_revision` | VText | document readers | observe only |

The runtime should store one canonical message/update with a stable id and
per-recipient delivery records. Do not implement this as two unrelated messages
that can diverge.

### Parent / Child Terminology

Keep immutable causal lineage, but stop using `parent` / `child` as the only
way to explain authority.

Preferred concepts:

- `spawned_by_run_id`: causal origin.
- `supervisor_agent_id`: who supervises this agent.
- `control_owner_agent_id`: who may redirect/cancel this run.
- `coordination_channel_id`: where updates live.
- `workspace_scope_id`: which computer/candidate world may be mutated.
- `role_slot`: implementation, verifier, researcher, narrator, proof worker.

Code may retain existing columns for migration, but docs, prompts, new APIs,
and Trace labels should make the authority model explicit.

### Async Delegation

Replace the synchronous delegate shape with:

```text
request_worker_vm
  -> returns a worker handle only

start_worker_delegation
  -> starts worker run
  -> returns worker_run_id, worker_id, vm_id, channel refs immediately

observe_worker_delegation
  -> bounded read of run state, checkpoint messages, package refs, blockers,
     child/co-super states, health, and retry/backoff facts

redirect_worker_delegation
  -> super-only command to vsuper
  -> may be copied to co-super only with supervising vsuper included

cancel_worker_delegation
  -> super-only control; records cancellation chain and preserved evidence

finish_worker_delegation
  -> terminal collection: package/adoption/blocker/run-acceptance evidence
```

If legacy tool names remain, their semantics must change to this nonblocking
shape. Do not keep a synchronous compatibility path as an accepted product
route.

## Homotopy Axes

Increase realism while preserving topology:

1. **Blocking to async:** from synchronous delegate polling to durable worker
   handles and bounded observation.
2. **One-to-one casts to typed fanout:** from ad hoc `cast_agent` delivery to
   canonical multi-recipient messages with explicit authority.
3. **Parent/child to authority graph:** from lineage-as-management to explicit
   supervisor/control/workspace semantics.
4. **Raw run cancel to resumable graph cancel:** from one run id to VText
   revision work cancellation across researchers, super, vsuper, and co-supers.
5. **Trace noise to supervision signal:** keep full causal events, but surface
   LLM content, tool calls, and agent messages as the first-class review path.
6. **Sequential proof to bounded concurrency:** prove sequential Chiron first;
   only then test safe concurrent supervision.

## Required Runtime Work

- Remove or hard-change the hidden `request_worker_vm` chained delegate
  transition.
- Add async worker delegation tools/API with start/observe/redirect/cancel/finish
  semantics.
- Propagate VText requester/update-target metadata into worker-vsuper runs.
- Make curated `vsuper` owner updates eligible for VText wake/synthesis.
- Add canonical typed update/message storage or adapt channel delivery so a
  message can atomically address multiple recipients with per-recipient delivery.
- Enforce skip-level copy rule for `super -> co-super` directives.
- Clarify prompts:
  - co-super reports primarily to vsuper;
  - vsuper curates significant updates to super/VText/Trace;
  - VText asks super for clarification when uncertain;
  - super can answer VText without automatically redirecting workers;
  - only super sends worker control commands.
- Add VText UI cancel for active revision work.
- Add graph-aware cancel propagation and resumable cancellation records.
- Refine verifier role semantics and tool/prompt policy around scratch writes
  versus candidate authorship.
- Update Trace/run-acceptance synthesis to expose async delegation handles,
  copied directives, VText clarification turns, cancellation chains, and
  verifier scratch evidence.

## Verification

### Focused Tests

Required test families:

- `request_worker_vm` does not invoke a delegate transition or block on worker
  run completion.
- Async delegation start returns promptly when worker execution is intentionally
  slow.
- Observe returns checkpoint batches and terminal status without long blocking.
- Super can receive/respond to a VText clarification while a worker delegation
  is still running.
- `super -> co-super` directive without copied `vsuper` is rejected.
- Atomic copied directive creates one canonical message and delivery records for
  both co-super and vsuper.
- VSuper-originated owner checkpoint wakes VText and is incorporated into a
  revision.
- VText cancel propagates to active researcher/super/vsuper/co-super work and
  records resumable cancellation metadata.
- Cancel does not destroy already-produced package/evidence refs.
- Verifier role can write scratch tests/evidence and run commands but cannot
  publish/promote/adopt or author candidate source as the implementation writer.

### Staging Product Proof

After deploy:

1. Verify staging `/health` reports the pushed commit.
2. Submit a visible prompt-bar workload that starts one long-running worker
   delegation and causes VText to ask super one clarification.
3. Prove super answers the VText clarification while the worker remains active.
4. Prove worker checkpoints appear in VText and Trace without waiting for
   terminal delegate completion.
5. Prove a copied skip-level directive, or prove the fallback path routes only
   through vsuper.
6. Prove VText cancel stops the active work graph and a later VText revision can
   resume from preserved evidence.
7. Synthesize run acceptance for the runtime hardening proof.

### Experiment Resume Proof

Then resume the four-experiment mission through the product path:

1. Start with Chiron only.
2. Choir-in-Choir must produce the candidate feature, owner-readable VText
   dashboard/report, screenshots/video, and package/adoption/rollback evidence.
3. If Chiron fails, Codex diagnoses whether the failure is in runtime,
   prompting, evidence, Apps & Changes, candidate builds, or the experiment
   itself; patch the substrate if needed and rerun Chiron through Choir.
4. Only after Chiron proves the loop should the mission continue sequentially to
   Motion, Liquid, and Python mode.

## Forbidden Shortcuts

- Do not hand-code experiment features with Codex.
- Do not keep synchronous `delegate_worker_vm` as a success path.
- Do not hide long-running work behind a blocking tool call.
- Do not allow private skip-level directives from super to a vsuper-owned
  co-super.
- Do not create two independent messages when one copied directive is required.
- Do not let VText drive worker control directly.
- Do not use raw `parent` / `child` vocabulary as proof that authority is clear.
- Do not cancel by deleting evidence or losing resumability.
- Do not call a verifier read-only if it cannot run code or write scratch
  evidence.
- Do not use local-only proof for vmctl/runtime/product-path behavior.
- Do not claim four-experiment success from package/build receipts without
  human proof.

## Run Checkpoint & Resumption State

### 2026-05-23 Capacity And Worker-Liveness Checkpoint

The async worker-delegation/runtime patch was landed and pushed in staged
pieces through `27d07a1c85fab2b1ab1d872bbc88645854df2d08`, but the deploy for
that SHA failed before switch because Node B exhausted root/state-dir build
space while compiling `platformd` and building the Playwright worker guest
image. Root cause was accumulated terminal worker VM state under
`/var/lib/go-choir/vm-state`, plus orphaned Firecracker processes for some
terminal workers that prevented the existing stale-state reclaim from deleting
them. Operational recovery backed up `ownerships.json`, removed a bounded set of
old terminal worker VM states with no live Firecracker process, and restored
Node B from about 2 GB free to about 95 GB free without deleting primary
computers.

The current code patch makes this recovery durable: stale terminal worker /
candidate state is now ranked by actual state-dir disk usage under pressure,
orphaned same-VM Firecracker processes can be cleaned before stale state
deletion, and Node B's stale-state delete bound is raised to 25 per sweep. This
patch still needs commit, push, CI/deploy, staging identity verification, and
product proof.

```text
status: checkpoint_incomplete
last checkpoint:
  Runtime hardening implementation checkpoint on 2026-05-23 before staging
  product proof and four-experiment rerun.
current artifact state:
  Previous human-proof rerun remains checkpoint_incomplete. Runtime now has a
  first async worker-delegation surface and VText cancellation path locally,
  but this has not yet been pushed, deployed, or proven through staging product
  flows.
what shipped:
  Not shipped yet at this checkpoint.
what was proven:
  Local focused proof inside `nix develop`:
  - `request_worker_vm` returns a worker lease plus `start_args`; the hidden
    `request_worker_vm -> delegate_worker_vm` auto-chain is removed.
  - `start_worker_delegation` / `delegate_worker_vm` now start a worker run and
    return immediately with `worker_run_started`.
  - `observe_worker_delegation`, `finish_worker_delegation`,
    `redirect_worker_delegation`, and `cancel_worker_delegation` are registered
    for super.
  - `redirect_worker_delegation` writes a super-authored worker inbox message
    through `/internal/runtime/channel-casts` with correct source run/agent
    attribution.
  - Super private skip-level casts to a vsuper-owned co-super are rejected;
    copied `cast_agent_update` directives to co-super plus supervising vsuper
    pass.
  - Curated `vsuper` channel messages are eligible to wake VText synthesis.
  - VText exposes a cancel action for active agent revisions; the API cancels
    the run graph and marks the mutation resumable.
  - Verifier co-super cannot publish AppChangePackages; verifier prompt policy
    now allows commands/scratch evidence while forbidding source/promotion
    authorship.
  - Run-acceptance synthesis recognizes async worker-delegation tool evidence.
  - Frontend production build passes.
unproven or partial claims:
  - No staging deploy or product-path proof has been run for these changes.
  - Multi-recipient messages are copy-aware through a shared `copy_group_id`,
    but they are still represented as per-recipient channel messages rather
    than a new canonical message table with per-recipient delivery rows.
  - Causal lineage versus authority is clarified in prompts/tool rules, but the
    database/API model still uses existing parent/child fields.
  - VText can cancel active revision work, but the full clarification
    question/answer loop between VText and super has not been staging-proven.
  - The full `internal/runtime` package exceeds a five-minute local test cap on
    this Mac due the existing broad Dolt-backed suite; focused runtime/store
    tests and compile-only package tests pass. CI shards runtime tests and must
    be monitored after push.
  - The Chiron/Motion/Liquid/Python experiment rerun has not restarted.
belief-state changes:
  Blocking worker delegation was indeed a runtime supervision failure. Removing
  the hidden synchronous chain and giving super explicit start/observe/redirect/
  finish tools should let future runs supervise instead of disappearing inside
  one long tool call, but staging proof is still required.
remaining error field:
  Staging behavior of async worker runs; whether vsuper checkpoints actually
  produce owner-readable VText revisions under real provider load; exact
  storage/API shape for canonical multi-recipient messages; migration strategy
  for parent/child terminology; Trace filtering/readability for async
  supervision evidence.
highest-impact remaining uncertainty:
  Whether async delegation plus copied directives is enough for super to
  supervise worker work without blocking, contradictory control, or excessive
  communication overhead in a real staging Choir-in-Choir run.
next executable probe:
  Commit/push the runtime hardening patch, monitor CI shards and deploy,
  verify staging commit identity, then run a visible prompt-bar proof that
  starts a worker delegation, observes a checkpoint before terminal completion,
  redirects or cancels through super, synthesizes run acceptance, and only then
  resumes Chiron through Choir-in-Choir.
suggested resume goal string:
  Use the one-line goal string in this file.
evidence artifact refs:
  Local verification:
  - `nix develop .# --command go test -count=1 ./internal/runtime -run 'TestRedirectWorkerDelegationPostsSuperAuthoredWorkerInbox|TestSuperSkipLevelCastRequiresCopiedVSuper|TestVerifierCoSuperCannotPublishAppChangePackage|TestVTextCancelAgentRevisionCancelsRunGraphAndLeavesMutationResumable|TestRunAcceptanceSynthesize' -v`
  - `nix develop .# --command go test -count=1 ./internal/runtime ./internal/store -run 'TestInstallDefaultAgentToolsProfiles|TestSuperRequestWorkerVMReturnsTypedHandle|TestExecuteTools|TestDelegateWorkerVM|TestSuperDelegateWorkerVM|TestVTextWorkerMessage|TestHandleTestVTextWorkerUpdate|TestRequestSuperExecution|TestPersistentSuperProcessesConcurrentInboxDeliveriesInFollowupRun|TestSubmitWorkerUpdateUsesVTextRequesterOverExplicitAgent|TestPublishAppChangePackageToolPublishesWithoutGitHubPush|TestCoagentToolsSupportAddressedCastAcrossProfiles|TestRestartRecoveryClearsInterruptedVTextMutation' -v`
  - `nix develop .# --command go test -count=1 ./internal/runtime ./internal/store -run '^$'`
  - `nix develop .# --command go test -count=1 ./internal/runtime -run 'TestInstallDefaultAgentToolsProfiles|TestRedirectWorkerDelegationPostsSuperAuthoredWorkerInbox|TestSuperSkipLevelCastRequiresCopiedVSuper' -v`
  - `npm --prefix frontend run build`
rollback refs:
  Pending commit SHA. Revert the eventual runtime-hardening commit if
  worker-delegation, VText revision, or coagent channel delivery regress on
  staging.
```

### 2026-05-23 Chiron Source-Transfer Checkpoint

After the capacity fix, staging deployed
`a01595ffff50c97569c8d2a87163317e74c4bbf3` and `/health` reported both proxy
and upstream at that SHA. The visible product-path Chiron rerun was executed
with Playwright against `https://draft.choir-ip.com` and produced local evidence
under:

```text
/Users/wiz/go-choir/test-results/chiron-sequential-a01595f-20260523T152628Z
```

Mechanical Playwright status was `1 passed`, but product outcome was
`no_matching_package`, not experiment success. The run did prove useful runtime
facts:

- source VText document: `5fe4f267-2015-408f-97c7-36cc2139cdea`;
- VText head revision: `52bb6a61-55a5-42e2-867e-62a762ee5992`;
- trajectory: `8279b78c-aa14-497e-b647-9dea961bccd2`;
- source run acceptance: `runacc-6b74cf32519d4d94155c`,
  `staging-smoke-level`, accepted;
- implementation worker:
  `worker-18fc4a60a02157b1` / `vm-f09452c1fb0f11c64c0955200daba4e2`,
  `worker-small`;
- proof worker:
  `worker-b247d7194b989767` / `vm-24f022facb00684a282c3b102f649e49`,
  `worker-playwright`.

Root cause of the failed product proof:

```text
implementation co-super created commit d62e15a841a91cc291b93d2ba30a3c0beb4fde59
inside the implementation worker checkout only;
the separate worker-playwright proof worker could not fetch that commit from
the GitHub remote because it was never pushed or otherwise exported.
```

The implementation worker reported a real candidate diff touching:

- `frontend/src/lib/ChironStream.svelte`;
- `frontend/src/lib/Desktop.svelte`;
- `frontend/src/lib/BottomBar.svelte`.

But it did not call `publish_app_change_package` before the parent attempted
proof-worker handoff. That exposed a subtle contract bug: the prompts treated
human proof as if it had to exist before the candidate source delta could be
published. In a multi-worker proof path, that is backwards. A worker-local Git
commit is not a transferable source artifact. The AppChangePackage can be
`evidence_pending` and still carry the source delta; Apps & Changes and review
evidence must simply refuse to label it human-reviewable until the VText
narrative plus screenshot/video/benchmark refs exist.

Current patch:

- clarifies super/vsuper/co-super prompts and embedded prompt defaults;
- clarifies remote worker bootstrap and vsuper delegate contract;
- says proof workers must inspect a package id or package-derived
  candidate/adoption route, never only an unreachable worker-local commit;
- says implementation workers should publish an honest `evidence_pending`
  AppChangePackage after commit and focused verification when external browser
  proof is still missing;
- updates the Chiron product proof prompt to use `worker-medium` for
  implementation/build work and reserve `worker-playwright` for bounded
  screenshot/video evidence after package evidence exists.

Local verification for this checkpoint:

```text
nix develop -c go test ./internal/runtime -run 'TestWorkerRepoBootstrapPromptIncludesHumanEvidenceBrowserContract|TestWorkerVSuperDelegateContractPreventsCheckoutRaces|TestInstallDefaultAgentToolsProfiles|TestPublishAppChangePackageToolPublishesWithoutGitHubPush|TestAppChangePackageReviewEvidenceRequiresNarrativeAndMediaForHumanReview' -count=1
nix develop -c go test ./internal/vmctl ./internal/vmmanager -count=1
nix develop -c go test ./internal/runtime -count=1
```

All three passed locally before this checkpoint was committed.

Next executable probe:

```text
commit/push this source-transfer contract fix -> monitor CI/deploy ->
verify staging identity -> rerun the same Chiron product proof.
Expected improvement: at minimum, the implementation run publishes one
product-visible AppChangePackage, possibly evidence_pending. If proof/adoption
still fails, the blocker should move from "unreachable local commit" to a
specific package/adoption/proof-worker failure.
```
