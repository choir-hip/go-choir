# Mission M3.1 - Lifecycle Cutover Regression Recovery - v0

Source: M3 lifecycle cutover review after aggregate diff
`95d28dc373a66a668779f24ab4870a774b67a6e8..4c610ca90ae4c69c2cfe414b3722572aa9c9a3df`.
Predecessor paradoc: `docs/mission-lifecycle-cutover-v0.md`. Discipline:
`skills/parallax/SKILL.md`. This paradoc is a recovery mission, not a product
shipping mission.

## Source Form

**Kind:** regression recovery / doctrine repair.

**Problem:** the late M3 run turned a product-path acceptance precondition into
runtime workflow enforcement. VText researcher delegation became a hard
`edit_vtext -> spawn_agent` continuation. The M3 paradoc then evolved to tell the
next worker to debug and harden that forced continuation. This contradicts the
architecture: Choir is a multi-agent system with actor choices and authority
envelopes, not a fixed workflow engine.

**Immediate falsifier:** a worker can remove the hard VText researcher
continuation and still prove durable actor restart/rewarm through a product-path
trajectory whose evidence is about open obligations, activation passivation,
assigned work rewarm, delivered updates, and no stranded messages.

**Doctrine gap:** VText behavior was under-documented. Existing docs described
roles and authority, but did not encode the invariant that VText chooses whether
to delegate to researcher, super, both, or neither. This left an acceptance probe
free to define the runtime semantics. M3.1 starts by promoting that invariant
into `docs/vtext-agentic-invariants-2026-06-13.md` and `AGENTS.md`.

## Parallax State

status: working

**mission conjecture:** if M3.1 removes forced VText workflow control, restores
agentic VText delegation semantics, repairs acceptance surfaces so they measure
durable-actor lifecycle evidence rather than exact internal tool order, and
documents the invariant, then M3 can resume as a lifecycle cutover instead of a
probe-driven workflow-hardening loop.

**deeper goal (G):** keep Choir's rearchitecture centered on durable actors,
obligations, trajectories, and promotion evidence. Prevent short-term acceptance
pressure from converting agents into brittle procedural workflows.

**witness/spec (A/S):**
- a documented VText invariant: VText is an appagent in a multi-agent system,
  not a workflow stepper. It may decide to write, ask researcher, ask super,
  ask both, wait, or report a blocker within its authority envelope;
- removal or containment of VText researcher `next_required_tool=spawn_agent`;
- a constrained required-next-tool protocol only for mechanical handshakes, not
  semantic appagent choices;
- browser-public trajectory/work-item evidence sufficient for lifecycle review;
- M3 paradoc/handoff corrected so the next move no longer hardens the regression;
- tests that protect the invariant and the acceptance boundary.

**invariants / qualities / domain ramp (I/Q/D):**
- I: no runtime code may force a semantic delegation from VText to researcher,
  super, or any future appagent solely because prompt text or document metadata
  mentions that role.
- I: exact required tool choice is allowed only for bounded mechanical
  protocols whose next action is part of the same tool contract, such as a VM
  lease followed by worker-start. It is not an appagent policy mechanism.
- I: acceptance must not require exact internal actor/tool sequence unless that
  sequence is the product behavior being specified.
- I: M3 settlement remains the restart/amnesia falsifier: cold actors rewarm
  from durable backlog/open assigned obligations with no stranded messages or
  zero-obligation stalls.
- Q: fixes must reduce accretion. Delete or narrow, do not add another role
  branch to the core loop.
- D: start with static review and focused tests, then runtime shards, then
  staging proof. Local tests cannot settle vmctl, deployed actor rewarm, or
  Choir-in-Choir behavior.

**variant (ranking function) V:** current local V=0 after the deployed
acceptance overclaim repair was implemented in the deploy-impact classifier;
deployed V remains un-settled until the repair is pushed, CI/deploy restarts
vmctl, staging identity matches the new commit, and the deployed acceptance
synthesis probe returns blocked. The local rollback batch removed or accepted
all eight known edges:
1 forced VText researcher continuation removed; 2 generic
required-next-tool semantic trust narrowed to a typed mechanical protocol; 3
prompt-bar researcher intent removed from runtime routing; 4 tests rewritten to
protect VText choice/non-forcing semantics; 5 M3 handoff corrected away from
hardening the deterministic continuation; 6 browser-public run status exposes
trajectory/work evidence; 7 prompt/VText-only smoke no longer accepts a run
acceptance record locally and the deploy classifier now restarts vmctl when
sandbox runtime package sources change; 8 actor memory cross-trajectory policy
is named as a successor edge rather than a blocker for this rollback.
Settlement still requires CI/deploy, staging identity, deployed lifecycle
evidence, and a deployed acceptance synthesis probe proving the overclaim is
gone.

**budget:** one recovery mission before further M3 implementation. Solvency:
first pass must buy the doctrine fix plus remove the forced continuation path.
If the worker cannot make code changes safely, exit `open_handoff` with exact
patch plan and tests, not another prompt-route patch.

**authority / bounds:** repo docs and runtime code on main or a Codex branch.
Behavior-changing fixes require Problem Documentation First, focused tests,
`nix develop -c scripts/go-test-runtime-shards`, push/CI/deploy for settlement,
and staging proof for any vmctl/product-path claim.

**position / live conjectures / open edges:**
- C1 supported: original M3 did not ask for hard VText researcher workflow. At
  `95d28dc3`, the next move was lifecycle control-read inventory and restart
  proof, not researcher routing.
- C2 active: the pivot accumulated after Batch W. A deployed proof precondition
  demanded conductor, VText, researcher, and super before refresh; when
  researcher was absent, the paradoc converted that into "repair product route,"
  then "make explicit researcher obligations executable," then "enforce runtime
  continuation."
- C3 supported for docs and local code/tests: VText has a crisp invariant in
  shared doctrine and the forced researcher continuation is removed locally.
- C4 supported locally: required-next-tool remains only for the worker VM
  mechanical lease/start protocol with typed `start_args`; semantic tool results
  cannot force exact appagent delegation.
- Edge/deployed_acceptance_overclaim: deployed acceptance synthesis for
  `mission-lifecycle-cutover-m3.1-v0` created
  `runacc-94d318d49e2ba66a99ce` at `staging-smoke-level/accepted` from only
  submitted + VText-opened checkpoints. This is the exact M3.1 forbidden shape,
  even though local tests covered the older lifecycle mission path.
- C5 active: the overclaim came from deploy topology rather than the
  run-acceptance code in the pushed commit. The deployed product-path response
  was served through an authenticated user computer whose sandbox package can
  be supplied by vmctl's `/internal/vmctl/runtime-package/sandbox` endpoint;
  the deploy impact for `27af4f2f6cf9caddc8fc3ae0ea96d5dbbdc1428a` refreshed
  sandbox/gateway but did not restart vmctl. The classifier now marks sandbox
  runtime package changes as requiring vmctl restart so newly booted user
  computers receive the updated runtime package.
- Edge/deployed_oracle: CI/deploy and staging identity succeeded for
  `27af4f2f6cf9caddc8fc3ae0ea96d5dbbdc1428a`, and the deployed adaptive
  lifecycle Playwright proof passed. Settlement remains blocked by the deployed
  run-acceptance overclaim above.
- Edge/successor: actor memory cross-trajectory scope still needs a dedicated
  policy/test pass, but is not required to settle this regression rollback.

**next move:** commit and push the deploy-impact repair, verify CI/deploy
restarts vmctl, verify staging identity, rerun deployed adaptive lifecycle
proof, and rerun deployed acceptance synthesis to prove prompt/VText-only smoke
is `staging-smoke-level/blocked` for M3.1.

**ledger file:** `docs/mission-lifecycle-cutover-m3.1-v0.ledger.md`.

**version / lineage:** M3.1 split from M3 after review of 47 commits on
2026-06-13. It supersedes the current M3 next move but does not erase M3
history. M4 continuation deletion and M5 Wire settlement remain successors only
after M3/M3.1 lifecycle settlement.

**learning state:** VText invariant promoted outward to `AGENTS.md` and
`docs/vtext-agentic-invariants-2026-06-13.md`. The local rollback now matches
that invariant: VText may choose delegation affordances, runtime no longer
forces researcher continuation from semantic prompt text, and acceptance cannot
settle M3 from prompt/VText smoke alone. The specific M3 failure chronology
stays here and in the ledger.

**settlement:** not claimed. Local code/docs and tests repaired the original
rollback batch, CI/deploy reached staging, and adaptive lifecycle Playwright
passed, but deployed acceptance synthesis overclaimed M3.1 from prompt/VText
smoke. The deploy-impact repair is implemented locally; final settlement
requires redeploying it, proving vmctl restart happened, and proving deployed
acceptance synthesis blocks the prompt/VText-only shape.

## Review Findings

### P1 - Acceptance Can Overclaim M3 Readiness

`internal/runtime/run_acceptance.go` now accepts `submitted + vtext_opened` as
`staging-smoke-level/accepted` when no blocked checkpoint exists. The test
`TestRunAcceptanceSynthesizeAcceptsPromptVTextStagingSmoke` locks this in. This
is acceptable only as a deliberately tiny product smoke. It is not M3 proof. M3
needs restart/rewarm evidence, open-work evidence, delivered update evidence,
and no stranded obligations.

Risk: a future worker can present prompt/VText smoke as lifecycle progress,
because the record says `accepted`.

Immediate fix: keep the smoke tier if useful, but name it as smoke only and make
the M3 paradoc require a separate lifecycle acceptance predicate. Do not let
`staging-smoke-level/accepted` settle M3.

Long-term fix: repoint acceptance levels around trajectory/work-item settlement
in M4, and add mission-specific acceptance profiles so each mission names the
evidence class it needs.

### P1 - VText Researcher Delegation Became Runtime Workflow

`internal/runtime/tools_vtext.go` calls `requiredContinuationAfterVTextEdit`
after every VText edit. If prompt/revision metadata trips explicit researcher
intent and no researcher participation exists, the result carries
`next_required_tool=spawn_agent`. `internal/runtime/toolloop.go` then converts
that into exact `spawn_agent` tool choice and retries if the model tries to end
the turn.

Risk: VText no longer decides. Runtime decides that the correct next move is
researcher spawn, even when VText might reasonably ask super first, ask both in
another order, create a work item, write a blocker, or do nothing because the
request is phrased negatively.

Immediate fix: remove the researcher branch from
`requiredContinuationAfterVTextEdit`. Keep deterministic email handoff separate
only if it is truly a single app protocol and owner approval remains preserved.
Replace forced-spawn tests with negative tests: mentioning researcher must not
force `next_required_tool`; VText can call `spawn_agent` when it chooses.

Long-term fix: model VText obligations as explicit trajectory work items or
policy-visible affordances, not hidden runtime continuations. Let VText settle
obligations by choosing tools and recording evidence.

### P1 - The M3 Paradoc Now Points Workers At The Regression

The current M3 paradoc says the next move is to debug why
`requiredContinuationAfterVTextEdit` did not attach `next_required_tool`.
The inventory says to fix the deterministic continuation path so worker-woken
VText edits attach and enforce researcher spawn.

Risk: the next worker will faithfully harden the regression because the paradoc
is the source program.

Immediate fix: update M3 or supersede it with this M3.1 paradoc before any code
fix. State explicitly that deterministic VText researcher continuation is
falsified as an abstraction, not merely failed as an implementation.

Long-term fix: add a Parallax tripwire: after two failed attempts to make a
probe precondition true, the next move must be a shift that asks whether the
precondition is the wrong witness.

### P2 - Required Next Tool Is A Global Magic Key

`internal/runtime/toolloop.go` treats any successful tool JSON containing
`next_required_tool` or `next_tool` as a forced next tool. Research tooling
already has tests ensuring projections do not emit `next_required_tool`, which
shows the hazard is known.

Risk: any tool can accidentally or intentionally steer the model into an exact
tool call. A semantic appagent output and a mechanical worker lease use the same
control channel.

Immediate fix: allow required-next-tool only for an explicit allowlist of
mechanical producer/consumer pairs, or require a typed envelope that only those
tools can emit. VText, search, fetch, and document-edit tools should not be
allowed to force semantic delegation through this generic path.

Long-term fix: represent tool protocols as typed state machines. Tool results
may suggest next affordances, but only protocol states can require exact tools.

### P2 - Public Run Status Omits Trajectory Evidence

`runStatusWithTrajectory` exists and internal status uses it, but browser-public
status handlers still manually return old run-shaped responses. This keeps the
product-path observer attached to loop state instead of trajectory obligations.

Risk: M3's new architecture is not visible enough through public/product
surfaces. Reviewers and workers keep falling back to trace spelunking and
role-sequence probes.

Immediate fix: use the trajectory-aware status response for browser-public run
status where auth allows it.

Long-term fix: add a product-level trajectory status endpoint that reports open
work items, assigned agents, passivation/rewarm evidence, and settlement state
without exposing raw internal control routes.

### P2 - Researcher Intent Detection Is Too Broad

`promptBarExplicitResearcherIntent` marks any prompt containing the substring
`researcher` as explicit researcher intent. That signal changes initial routing
and feeds the forced continuation.

Risk: "do not ask researcher" or "should we ask a researcher?" can become a hard
researcher route. Even after removing the hard continuation, this remains a bad
semantic detector.

Immediate fix: replace the substring check with a stricter parser or remove the
runtime flag and leave delegation to VText.

Long-term fix: treat role mentions as owner intent facts in the document/trace,
not as routing commands, unless the product has an explicit command grammar.

### P3 - Actor Memory Rewarm Crosses Trajectories Implicitly

Run memory seeding loads the latest prior inactive activation by
`(owner_id, agent_id)` only. For persistent actors like `super:{owner}`, that
may intentionally carry actor memory across tasks, but the policy is not
documented or tested as such.

Risk: unrelated prior context can leak into a new mission and bias an actor.

Immediate fix: document the policy and add tests for what crosses trajectories
and what must not.

Long-term fix: actor memory should have scopes: owner-level durable memory,
trajectory-local memory, and task-local activation memory, each surfaced in
trace/acceptance.

## How This Happened

The failure was not in the original M3 paradoc. The original state at
`95d28dc3` had the right shape: inventory lifecycle control reads, separate
control semantics from provenance, preserve actor terminology, prove
kill/restart rewarm, and avoid permanent dual lifecycle models.

The pivot accumulated midway:

1. Batch U/V evidence showed a real lifecycle issue: a VText-spawned researcher
   could be passivated or non-delivering across refresh. The local fix created
   assigned work items and requester-route metadata. This was still M3-shaped.
2. Batch W reran the deployed proof and failed before refresh because the trace
   lacked a researcher. The proof required conductor, VText, researcher, and
   super before invoking vmctl refresh. The paradoc called this a product-path
   orchestration problem and said to make the proof reliably create both
   researcher and super work.
3. Batches X and Z tried route/prompt metadata fixes. Those were already
   drifting from lifecycle semantics toward satisfying the probe's pre-refresh
   role list, but they were still prompt/policy flavored.
4. Batch AA introduced `next_required_tool=spawn_agent` from `edit_vtext`.
   The paradoc then escalated from guidance to "enforce explicit researcher
   obligations in the runtime tool loop."
5. Batches AB/AC expanded that path to non-user bases and durable revision
   metadata. The current next move asks to debug why that deterministic
   continuation did not attach in staging.

So the bad pivot was not present at mission start. It accumulated through a
fixed-position loop: the probe did not reach the restart oracle unless a
researcher appeared first, so the worker optimized the system to satisfy that
precondition instead of shifting the probe or questioning whether exact
researcher participation was a valid M3 witness.

The deeper cause is a missing invariant. The codebase documents authority
boundaries, but not the specific VText rule: VText is an agentic document owner
inside a multi-agent system. It can delegate, but runtime must not turn a role
mention into a forced workflow edge. Without that invariant, the acceptance
probe became the de facto spec.

## Immediate Recovery Plan

1. Documentation checkpoint: commit this M3.1 paradoc or equivalent before code
   changes, satisfying Problem Documentation First.
2. Remove the VText researcher branch from
   `requiredContinuationAfterVTextEdit`.
3. Replace tests that expect `next_required_tool=spawn_agent` with tests that
   assert no forced continuation and prove VText still has `spawn_agent` and
   `request_super_execution` affordances.
4. Narrow `extractRequiredNextTool` to an allowlist or typed protocol envelope.
   Preserve worker VM lease/start behavior only if covered by tests.
5. Narrow or delete `promptBarExplicitResearcherIntent`.
6. Update public run status to include trajectory obligations.
7. Revise `docs/mission-lifecycle-cutover-v0.md` or point workers to this M3.1
   doc so no one continues the deterministic continuation route.
8. Run focused VText/tool-loop/API tests, then
   `nix develop -c scripts/go-test-runtime-shards`.
9. Only after local proof, run staging acceptance aimed at lifecycle evidence,
   not role sequence.

## Long-Term Architecture / Doctrine

- Add a VText architecture section: VText owns canonical document versions and
  may coordinate other agents, but it is not a workflow executor. Delegation is
  an agent decision under an authority envelope.
- Add a harness-minimalism rule specific to appagent semantics: role-specific
  runtime branches require documented invariant, rejected alternatives, and
  explicit human approval.
- Define required tool protocols as typed state machines. Avoid magic JSON keys
  that any tool can emit.
- Make acceptance profiles mission-specific. A mission can require "researcher
  participation" only if the mission is actually about researcher participation.
  M3 is about durable actors.
- Add a Parallax "proxy capture" tripwire: when three successive fixes target
  an acceptance precondition rather than the mission conjecture, the next move
  must be an observer shift.
- Split actor memory into explicit scopes before relying on it for persistent
  super behavior.

## Suggested Goal String

```text
/goal Use Parallax on docs/mission-lifecycle-cutover-m3.1-v0.md. Treat it as the active M3.1 lifecycle-cutover regression recovery source program and the required preflight before resuming M3 proper. Current status is working: local rollback V=0 after removing forced semantic VText researcher continuation, narrowing required-next-tool to mechanical protocol output, deleting prompt-bar researcher routing intent, exposing trajectory/work evidence, preventing prompt/VText-only run acceptance, updating M3 handoff, and adding tests that protect VText choice. Settlement is not claimed until landing evidence exists. Preserve Choir Doctrine as apex, VText as an agentic canonical-document owner rather than a workflow stepper, harness minimalism, trajectory/work-item evidence over run-tree smoke, and Problem Documentation First. Mutation class is orange/red: runtime VText tools, generic required-next-tool control, prompt-bar researcher intent, public run status, run acceptance, and lifecycle proof surfaces are protected. First next move: commit/push the rollback batch, monitor CI/deploy, verify staging identity, and run deployed lifecycle evidence before settlement. Ledger: docs/mission-lifecycle-cutover-m3.1-v0.ledger.md. Settlement requires no forced semantic VText delegation, no generic semantic next_required_tool control, tests protecting the invariant, M3 handoff corrected, no M3 settlement claim from prompt/VText smoke alone, and deployed lifecycle proof or a named successor edge.
```
