# Mission: Texture Durable Thread v1

Clean successor to [mission-texture-long-running-agent-v0.md](mission-texture-long-running-agent-v0.md).
Do **not** use the v0 ledger as the source program. The v0 ledger is historical
failure evidence: it shows why run-centric park/rewarm, semantic role
choreography, prompt classifiers, and exact-first-tool scaffolding are the wrong
shape. Read it only when auditing a specific cautionary claim.

## Mission Claim

If each Texture document becomes one durable, owner-visible agent thread with
runtime-owned mailbox identity, honest work-state revisions, same-thread
sleep/resume, and always-deep evidence gathering, then Texture becomes Choir's
reliable artifact control plane instead of a sequence of reconstructed runs.

## Current Problem

Current code still behaves like a run controller with helpful patches:

- A quiesced/parked Texture is reconstituted by new runs from document head,
  recent channel messages, and run-memory summaries rather than resumed as the
  same literal thread.
- Work delivery is discovered through pending-update queries and cold/warm
  injection machinery instead of being appended to one durable thread mailbox.
- Texture revision runs can be forced into a first `patch_texture`, producing
  tiny edits that hide background work instead of making owner-visible work state.
- `update_coagent` asks models to invent `update_id`, even though the store uses
  it as an owner-scoped idempotency and delivery key.
- Classifiers and guards still encode semantic role choreography in runtime
  (`texturePromptNeeds*`, explicit prompt scanners, model-prior completion guard,
  no-op/prompt-copy guards, exact-first-tool retry machinery).

## Target Shape

- One durable Texture thread per document.
- `V0` is owner input; `V1+` are Texture responses or owner edits in the same
  artifact history.
- Each canonical revision is an assistant turn that writes Texture.
- Owner follow-ups, researcher findings, super results, blockers, and source
  evidence append into the same thread mailbox.
- Runtime mints or derives `update_id`; the model describes payload, target,
  evidence, blockers, and questions, not delivery identity.
- Texture updates promptly. If work is pending, delegated, or incomplete, the
  next revision is an honest work-state / acknowledgement revision, not a fake
  final answer or instruction cleanup.
- The thread deepens by default. Research is not a mode and not keyword-gated.
- The thread quiesces on idle/await/budget, with zero provider calls while
  asleep; it resumes the same thread when input arrives.
- The thread ends only on document delete/cancel.

## Non-Goals

- Do not patch the old run-centric spine until it looks acceptable.
- Do not recreate a fixed Texture -> researcher -> super workflow.
- Do not make Trace or Chyron the owner-visible source of work state.
- Do not use model-authored labels such as `checkpoint-1` as durable delivery
  identity.
- Do not preserve compatibility shims for non-users. Hard cutover is preferred.

## Parallax State

status: working

mission conjecture: if Texture document work is represented as one durable
thread with runtime-owned mailbox identity and owner-visible work-state
revisions, then owner supervision, deep research, source evidence, and
foreground edits become one coherent artifact trajectory rather than
run-reconstruction side effects.

deeper goal (G): make Texture the artifact control plane for Choir's
multi-agent work, where canonical versions, active work state, and source
evidence remain legible to the owner and durable across process refresh.

witness/spec (A/S):
- runtime-owned `update_id` for model-facing `update_coagent`;
- direct Texture owner requests write honest work-state revisions when work is
  delegated or pending;
- durable per-document thread identity and mailbox cursor;
- same-thread passivation/resume with literal thread memory or structured
  compaction, not doc-head reconstruction;
- event-driven update delivery into the thread;
- deletion of classifiers/guards/exact-first-tool scaffolding that encode
  semantic choreography;
- staging proof that direct Texture revise, researcher delivery, and multi-round
  grounded deepening happen through one durable thread.

invariants / qualities / domain ramp (I/Q/D):
- I: Texture owns canonical document versions. Researcher/super evidence is
  non-canonical until Texture incorporates it.
- I: Work-state revisions are canonical artifact state, not Trace dumps.
- I: `update_id` is internal delivery/idempotency machinery. Runtime owns it.
- I: Prompt-bar `V0` is owner prompt; direct Texture `V0/current` may be a full
  owner-authored document.
- I: Runtime may enforce mechanical protocols but not semantic role sequences.
- I: Hard cutover; prefer deletion over shims.
- Q: keep the v1 mission readable enough for a fresh agent to start without the
  v0 ledger.
- D0: focused unit tests for update-id minting/idempotency and work-state
  revision policy.
- D1: integration tests for researcher delivery into Texture and source/evidence
  consumption.
- D2: same-thread passivation/resume and event-driven mailbox tests.
- D3: deployed product proof on `https://choir.news`.

variant (ranking function) V:
1. runtime-owned update-id scheme designed and implemented;
2. model-facing `update_coagent` no longer requires model-invented `update_id`;
3. retry/collision tests prove idempotency without human-label collision;
4. direct owner-triggered Texture work-state revision test passes;
5. Texture first-tool policy stops forcing trivial patches as hidden work-state;
6. durable thread/mailbox data model implemented;
7. update delivery appends into the thread instead of cold reconstruction;
8. same-thread passivation/resume implemented;
9. classifiers/guards/exact-first-tool scaffolding deleted;
10. always-deep research prompt/loop behavior proven beyond the current V2 cap;
11. focused runtime tests and runtime shards pass;
12. landed to main, CI/deploy identity verified, staging acceptance recorded.
Current V=2; last ΔV=-1 on 2026-06-21 from deleting deterministic
initial-super request parsing/recording after the prior prompt-bar first-paint
exact `patch_texture` forcing deletion. Ordinary prompt-bar Texture starts now
receive the full Texture tool surface with empty provider `tool_choice`, so
Texture can record an off-document decision before patching when that is the
honest first action. Execution-shaped prompt text such as "ask downstream super
execution to" no longer creates runtime-owned initial super work before Texture
acts; Texture still has the `request_super_execution` affordance when it chooses
to delegate. Staging proof for the initial-super deletion at
`1e0166474e17369828a1e8a7bfd655c34ae1454b` used Comet-visible marker
`CHOIR_INITIAL_SUPER_DELETION_PROOF_20260621_001` and structured marker
`CHOIR_INITIAL_SUPER_DELETION_ACCEPT_1782019773071`, trajectory
`a2e96589-dc7a-4be4-9e5d-556427f5afc2`, doc
`8789314a-fc2c-498d-9634-f9af72de6f56`, Texture loop
`ad8443b9-c5a2-4171-b59d-7cc06db91885`, V1
`652e8729-0172-4420-935d-f747876ba8ea`, first Texture tool event
`record_texture_decision`, no `super` agent, no `request_super_execution` tool
call, no `texture_initial_super_request_*` metadata, and blocked
staging-smoke RunAcceptanceRecord `runacc-db363e8ce03c8e65f0fe`. CI run
`27894521422`, deploy job `82543893990`, and staging health all reported the
same commit. Staging proof for prompt-bar first-paint deletion at
`b4adb70ff4a01ea6be92ce30a062a66a824f89a9` used trajectory
`20bf8cd1-ff91-4bae-87a0-96d348d7b3ae`, Texture loop
`c0dfb742-9797-42e7-931f-6951d63cddbc`, first provider-call moment
`96b5c6e7-a1b9-4c18-965c-bc8cf4ba224c`, V1
`cb641f7b-c821-43f3-82fe-2c5611479e19`, and blocked staging-smoke
RunAcceptanceRecord `runacc-73a11f1381124ee76315`. The prior staging proof at
`1c202e525f77a2a6169c0bf0ac49b986b75047b7` still supports same-thread
researcher delivery through V2 in loop `c3cb6b21-6220-4d6f-a226-641906ea56b9`
with RunAcceptanceRecord `runacc-26cfe15a6fbd4fd6be6f`. The remaining
durable-thread risks are model-prior/world-knowledge guard residues and proving
always-deep research beyond the current V2/V3 cadence on staging.

budget: one broad red-surface mission, but execute in reviewable slices. R0a
(`update_id` + work-state revisions) may land first if it reduces risk, but it
does not settle the mission by itself.

authority / bounds: repo changes only until landing. No Node B tracked-file or
environment shortcuts. Behavior settlement requires commit, push to `origin/main`,
CI, deploy identity, and deployed product acceptance.

mutation class / protected surfaces: red. Protected surfaces are Texture
canonical writes, revision metadata, `update_coagent` schema and persistence,
worker-update delivery/marking, run lifecycle, passivation/rewarm, prompt
defaults, Trace/evidence semantics, and product acceptance.

evidence packet:
- focused tests for update-id minting, retry idempotency, and collision
  resistance;
- focused tests for owner-visible work-state revisions;
- integration tests for researcher delivery and Texture consumption;
- tests for durable thread mailbox, passivation/resume, and delete/cancel;
- runtime shard suite or justified scoped substitute before commit;
- staging proof with doc id, trajectory/thread id, revision count, source count,
  deployed commit, CI/deploy status, rollback ref, residual risks.

heresy delta: repairs H024a (trivial first patch as hidden work-state) and H024b
(model-invented coagent update ids); should also repair run-centric cold rewarm
and classifier/guard residues named in the v0 cautionary mission.

position / live conjectures / open edges:
- C1 supported for the update-identity half of R0a: runtime-owned
  `update_id` can land before the full thread rewrite without adding run-centric
  routing. It remains a ramp repair, not mission settlement.
- C2 active: the durable mailbox cursor can replace pending-update rediscovery
  without losing crash recovery.
- C3 active: same-thread resume needs a run-lifecycle model change, not only
  prompt or controller edits.
- C4 active: always-deep research should be prompt/obligation driven, not
  keyword-classifier driven.
- C5 supported at focused-test scope: direct user-authored revise requests no
  longer force exact `patch_texture`, and an end-to-end direct revise test proves
  a canonical Texture work-state revision is written before researcher
  delegation. This repairs the local R0a H024a shape; staging still must prove
  the product path with a real provider before mission settlement.
- C6 supported at runtime scope: `coagent_mailboxes` records a durable
  contiguous processed cursor per addressed actor, and actor backlog reads no
  longer filter on `delivered_at`; delivered markers are audit compatibility,
  not the actor-facing cursor authority.
- C7 supported at focused-test scope for established Texture documents:
  resident, sleeping, restart-reactivated, and already-threaded documents no
  longer mint replacement wake runs for ordinary addressed `update_coagent`
  backlog. New `update_coagent` packets are appended as durable run-memory user
  turns, including first-turn `activation_mailbox_turn` delivery only for
  explicit first activation when no Texture revision-thread history exists.
- C8 supported at focused-test scope: normal Texture idle quiescence now
  passivates the run as a sleeping actor instead of completing it, keeps the run
  memory intact, and resumes the same `loop_id` when new mailbox input arrives.
- C9 supported at focused-test and staging-smoke scope for semantic delegation with parking:
  `spawn_agent`, `request_super_execution`, and email handoff no longer act as
  terminal shortcuts for parked Texture revision actors. After a work-state
  revision and delegation, the actor parks/sleeps and later researcher evidence
  can produce V2 in the same `loop_id`. Staging trajectory
  `a893f0ca-a8b6-41de-a73b-1e8b05c7c80d` showed `spawn_agent`, park wait,
  researcher `update_coagent`, V2 `patch_texture`, and final passivation all on
  Texture loop `c3cb6b21-6220-4d6f-a226-641906ea56b9`.
- C10 supported at focused-test and staging-smoke scope: prompt-bar first-paint
  exact `patch_texture` forcing is deleted. Ordinary first-paint Texture runs
  have empty provider `tool_choice` and the full Texture tool surface, while
  `update_coagent` integration remains grounded by narrow mechanical
  continuation behavior. Staging trajectory
  `20bf8cd1-ff91-4bae-87a0-96d348d7b3ae` showed first provider call
  `tool_choice=""`, eight available tools including `patch_texture`,
  `record_texture_decision`, `spawn_agent`, and `request_super_execution`,
  first tool batch `record_texture_decision`, then `patch_texture` to V1.
- C11 supported at focused-test and staging-smoke scope: deterministic
  initial-super request parsing/recording is deleted. Execution-shaped prompt
  text is now preserved as Texture document content and handled by the Texture
  provider turn first. Focused tests prove the old phrase no longer stamps
  initial-super metadata or auto-creates super work while manual
  `request_super_execution` from Texture context still works. Staging trajectory
  `a2e96589-dc7a-4be4-9e5d-556427f5afc2` showed `conductor + texture` only,
  no `super` agent, first Texture tool event `record_texture_decision`, then
  `patch_texture` to V1.
- C12 active: `textureModelPriorCompletionGuard` and
  `texturePromptNeedsWorldKnowledge` still enforce a deterministic
  keyword-scanned Probe/Execute path after a model-prior/interim first
  revision. That guard is less invasive than the deleted initial-super path,
  but it is still runtime-authored semantic choreography: runtime decides that
  phrases such as "latest", "news", "today", or "government" require a
  follow-up worker path and injects a retry instruction after Texture tries to
  end. The desired shape is prompt/tool agency: Texture may write an honest
  interim V1, call `spawn_agent` or `request_super_execution`, record a blocker,
  or end with an audit-worthy decision through its normal tools, without a
  hard-coded completion guard.

next move: delete the Texture-specific model-prior completion guard and its
world-knowledge keyword classifier while preserving model-prior/interim
metadata, prompt obligations, no-op protection, and agentic researcher/super
tool affordances. Then prove a current-events prompt can open a researcher path
by model/tool choice without a completion-guard retry.

ledger file: docs/mission-texture-durable-thread-v1.ledger.md

version / lineage: v1 supersedes `docs/mission-texture-long-running-agent-v0.md`
as the source program. v0 and its ledger remain cautionary evidence only.
Depends on `docs/mission-texture-versioned-artifact-v0.md` for provenance,
source validation, hash chain, and full-history publication substrate.

learning state: retain old failure history as a warning against matching the
wrong pattern. Promote only compact lessons: avoid run-centric reconstruction,
avoid role choreography, avoid model-invented delivery identity, and keep
owner-visible work state in Texture.

settlement: settled only when landed and deployed proof shows runtime-owned
coagent update identity, direct Texture work-state visibility, durable same-thread
resume, event-driven delivery, deepening beyond V2, deletion of old scaffolding,
and no stranded source/evidence updates.

## Suggested Goal String

```text
Use Parallax on docs/mission-texture-durable-thread-v1.md. Treat it as the source program; do not use docs/mission-texture-long-running-agent-v0.ledger.md except as cautionary evidence for specific claims. Current status planned, V=12. First move: implement R0a if it does not entrench the run-centric spine: model-facing update_coagent without model-invented update_id, runtime-owned idempotency, and honest owner-visible Texture work-state revisions. Then continue toward one durable Texture thread per document: same-thread mailbox, event-driven delivery, passivation/resume, always-deep research, and deletion of cold rewarm/classifier/guard/exact-first-tool scaffolding. Mutation class red; protected surfaces are Texture canonical writes, update_coagent persistence/delivery, run lifecycle, passivation, prompt defaults, Trace/evidence, and product acceptance. Settlement requires commit -> push origin main -> CI -> deploy identity -> staging proof on choir.news with direct Texture work-state, researcher delivery without model-authored update_id, durable same-thread resume, deepening beyond V2, and rollback refs.
```
