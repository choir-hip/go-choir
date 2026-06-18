# Mission: Texture As A Long-Running Agent v0

## Summary

Texture is currently a one-canonical-write-per-run, wake-driven actor that goes
dormant between deliveries. Staging measurement proves this caps the product at a
single revision (V1) that lands ~49-60s after the prompt with nothing shown in
between. That shape cannot deliver the three things Texture is for: a fast first
paint, a document that deepens across many revisions ("deep research" rather than
one chatbot answer), and a live control plane for supervising long-running coding
agent hierarchies (a fresh revision roughly every interval over hours/days).

This mission inverts the core invariant. The Texture agent `texture:<docID>`
becomes a single long-running logical actor: it writes an immediate from-weights
V1, then deepens the document across many canonical revisions as researcher and
super evidence streams in. When evidence pauses it parks cheaply; when the
sandbox restarts it sleeps and resumes as the same logical actor. The wake /
reconcile / one-write scaffolding that existed only to simulate continuity across
ephemeral runs is removed in favor of one uniform long-running-actor foundation
(park-and-wait + a per-actor budget) reusable by other roles.

This supersedes the cadence-fix increment shipped as `68d09cc3`, which disabled
Texture warm injection and thereby reinforced the very one-revision cap this
mission removes. The deployed probe (recorded in
`docs/mission-texture-product-loop-recovery-v0.ledger.md`) falsified that
increment as necessary-but-not-sufficient.

## Problem

Three coupled facts, all confirmed by code review and staging measurement:

1. One write per run is a hard, DB-backed block, not just a prompt. Each Texture
   run creates one `AgentMutation` row (`pending`); the first
   `patch_texture`/`rewrite_texture` flips it to `completed`
   (`internal/runtime/tools_texture.go:566-573`,
   `internal/store/texture.go:1753-1776`). A second write in the same run is
   rejected. So N revisions require N runs.
2. Texture deliberately opts out of warm injection
   (`internal/runtime/super_controller.go:434-436`, shipped in `68d09cc3`), so a
   live run cannot incorporate a second findings packet.
3. The first findings packet arrives late (~49s on staging) and the researcher
   is not checkpointing early despite its overlay mandating it
   (`internal/runtime/runtimeprompts/overlays/researcher_runtime.yaml:8,17-23`).
   First paint is gated on the first packet, not on the wake debounce.

The "persistent super" pattern is not an immortal run either: it is durable
identity + wake-driven ephemeral runs + warm injection while resident +
completion-chained inbox drain. Texture already mirrors it. The honest target is
therefore one logical actor per article, backed by a real park-and-wait + budget
primitive, because the runtime today has no cheap idle-wait, a bare
`maxToolLoopIterations=200` ceiling, and no cumulative cost budget.

## Owner Direction

- Texture is a long-running agent, not a dormant-then-woken one. The Texture
  agent should do multiple revisions per run; one run per Texture agent is the
  target minimal model.
- The first revision (V1) is an immediate from-weights draft produced before any
  researcher retrieval, then grounded and deepened by retrieval in later
  revisions.
- This is one paramission. How much lands in the first pass is open; Codex
  one-shots as far as it can and leaves precise blockers, then the owner-side
  reviewer (Cursor agent) critically reviews and iterates.

## Scope / Domain Ramp

The variant to burn down, roughly in dependency order. Codex should land as much
as is safely provable in one pass and record precise blockers for the rest.

- T1 Revision-cadence core. Remove the per-run one-write `AgentMutation` gate so
  a resident Texture run can write multiple canonical revisions; re-enable warm
  injection for Texture (supersede the `nil` short-circuit at
  `super_controller.go:434-436`); track consumed worker updates per revision, not
  by one per-run seq watermark (`runtime.go:2944-3019`,
  `markTextureWorkerUpdatesDelivered`).
- T2 From-weights V1. On activation, emit an immediate model-prior initial
  revision before researcher retrieval; flag it model-prior/interim in revision
  metadata so it is never mistaken for grounded; amend the grounding doctrine and
  overlays (`textureprompts/overlays/revision_policy.yaml`, `run_system.yaml`,
  `tools_coagent.go` guidance) to authorize this explicitly.
- T3 Long-running-actor foundation (harness, uniform). A park-and-wait primitive
  so an actor blocks with no billed provider calls until a new packet/signal or a
  budget/idle deadline; a cumulative per-actor cost/time budget + kill-switch
  replacing the bare `maxToolLoopIterations=200` for parked actors. Keep it
  role-uniform per AGENTS.md harness-minimalism; do not branch the core loop for
  Texture alone.
- T4 One-run-per-agent lifecycle. `texture:<docID>` owns at most one resident
  run; it parks when idle instead of ending. Remove or collapse the
  wake/reconcile/completion-chain/residency scaffolding it replaces
  (`texture_controller.go`, `reconcileCompletedTextureRun`) where the parked
  resident run now covers that role.
- T5 Passivation-as-sleep. vmctl refresh sleeps the actor; rewarm resumes the
  same logical actor from the run-memory snapshot (`run_memory.go:74-123`),
  with correct stale-mutation handling for multi-write runs
  (`runtime.go:1264-1312`).
- T6 Cancellation gaps. Document deletion must cancel the actor's run
  (`texture.go:1048-1061` currently does not); `CancelAgent` ends the parked run.
- T7 Verifier / tests / docs / heresy. Relax the verifier 1:1 revision-run-write
  causality to N:1 (`texture_workflow_verifier.go:527-593`); update comprehensive
  runtime tests, frontend specs, the doctrine in
  `docs/texture-agentic-invariants-2026-06-13.md` (Rule 4 and the cold/warm
  delivery contract), and the `initialTextureToolChoice`/`WithInitialToolChoice`
  heresy detectors (`cmd/doccheck/main.go:1135`, `docs/heresy-detectors.md`).
- T8 Deploy + staging proof. Fix the cadence probe's hang and the Playwright
  import portability flagged by Codex, then push -> CI -> staging deploy identity
  -> re-run `scripts/texture_revision_cadence_probe.mjs` and record a
  RunAcceptanceRecord.

## Suggested Goal String

```text
/goal Use Parallax on docs/mission-texture-long-running-agent-v0.md; that paradoc and its ledger are the spec, read them first. Make Texture (texture:<docID>) one long-running logical actor: write an immediate from-weights V1 (flagged model-prior/interim), then deepen across many canonical revisions as findings packets arrive, replacing the one-write-per-run wake-driven-dormant model. Work the T1-T8 ramp in order; land what you can safely prove in one pass and leave precise file-cited blockers for the rest. Mutation class red (Texture canonical writes, run tool loop / park-and-wait / budget, passivation/rewarm, verifier, Trace, deploy). Honor the paradoc's invariants and forbiddens (no compatibility shims, no semantic role-choreography; harness-minimal uniform park-and-wait). Verify with focused internal/runtime tests + scripts/go-test-runtime-shards, then commit -> push origin main -> CI -> Node B deploy identity -> deployed proof on https://choir.news via scripts/texture_revision_cadence_probe.mjs (fast from-weights first paint under the ~49s baseline, multiple grounded V2+ revisions tracking findings); record a RunAcceptanceRecord at staging-smoke-level. Rollback = revert mission commits.
```

## Parallax State

status: working

mission conjecture: if Texture becomes a single long-running logical actor that
drafts an immediate from-weights V1 and deepens the document across many
canonical revisions as evidence streams in - backed by a uniform park-and-wait +
per-actor budget foundation and sleep/resume across restarts - then Choir gets a
fast first paint, deep multi-revision answers, and a live supervision control
plane for long-running agent hierarchies, while the wake/one-write scaffolding
that only simulated continuity is removed.

deeper goal (G): Texture is Choir's human-supervision narrative and artifact
control plane. A continuously-updating long-running actor is the substrate for
supervising hours/days-long coding-agent hierarchies through a revision stream,
not a one-shot draft pane.

witness/spec (A/S): a deployed staging run where the prompt yields a from-weights
V1 well under the ~49s baseline, multiple grounded revisions (V2..Vn) that track
findings packets rather than collapsing into one write, one logical actor per
doc, survival of a vmctl refresh as sleep/resume without lost doc or duplicate
revisions, bounded per-actor cost, and cancellation on doc delete.

invariants / qualities / domain ramp (I/Q/D):

- I: canonical integrity - monotonic version numbers, no lost foreground
  updates, no duplicate or garbled revisions.
- I: grounding honesty - the from-weights V1 is explicitly flagged
  model-prior/interim in revision metadata; grounded revisions cite evidence;
  priors never masquerade as grounded.
- I: bounded cost - a cumulative per-actor cost/time budget plus kill-switch
  replaces the bare maxToolLoopIterations ceiling; parked time costs no LLM
  calls.
- I: restart safety - passivation is sleep; rewarm resumes the same logical
  actor from the run-memory snapshot; multi-write stale-mutation cleanup is
  correct.
- I: harness-minimalism - park-and-wait and the budget are uniform, role-agnostic
  primitives, not Texture-only core-loop branches.
- I: no compatibility shims, no semantic role-choreography decision trees in the
  cadence.
- Q: acceptance proves the product path (prompt-bar + public APIs), not manual
  worker invocation.
- Q: per-revision Trace/Texture boundaries remain legible within one run;
  trajectory/work-item attribution preserved.
- D: land cadence + from-weights V1 under focused local verification first, then
  the park-and-wait foundation, then deployed staging proof.

variant (ranking function) V: large and open. V descends as each ramp item lands
with evidence: multi-write-per-run proven; warm injection on for Texture;
per-revision consumption tracking; from-weights V1 flagged and shown fast;
park-and-wait blocks without billed calls; per-actor budget enforced with a
kill-switch; one-run-per-agent lifecycle with scaffolding removed; sleep/resume
across vmctl refresh; doc-delete cancels; verifier N:1; tests/docs/heresy updated;
deployed probe shows fast from-weights first paint and V2+ tracking findings.
Current audited value after the 2026-06-17 T1/T2 construct: T1 is locally
repaired for the resident-run write cadence. Texture now participates in warm
coagent update injection (`internal/runtime/super_controller.go:424-438`), a
Texture run may commit multiple canonical revisions while its mutation remains
pending (`internal/runtime/tools_texture.go:564-688` and
`internal/store/texture.go:1744-1790`), and injected worker packets are marked
delivered only when a Texture revision consumes them
(`internal/runtime/runtime.go:2959-3072`). T2 is partially repaired: a
no-worker appagent revision is marked as `model_prior_interim` /
`revision_grounding=model_prior` in revision metadata
(`internal/runtime/runtime.go:2868-2926`), and the Texture prompt overlays now
allow an explicitly uncertain fast scaffold while still forbidding grounded
world facts from model recall
(`internal/runtime/textureprompts/overlays/revision_policy.yaml:22-48`,
`internal/runtime/textureprompts/overlays/run_system.yaml:12-25`).

Staging evidence after deploy of `8dbdd458` falsified full mission settlement:
`scripts/texture_revision_cadence_probe.mjs` observed first appagent paint at
47.057s and only one appagent revision for prompt-bar submission
`d33edcc6-7f05-43af-b3a8-063679d68a5e` / doc
`893bcb64-d82e-4c99-856e-93d7d97e2f06`, even though Trace reported researcher
activity (`web_search=6`, `source_search=2`, `spawn_agent=2`,
`update_coagent=2`). The deployed health identity did match
`8dbdd4585417974bc2dd3f3d07b9c5ad58af542b`, so this is a product-path cadence
falsification of the partial T1/T2 construct, not a stale-deploy artifact.

Current local construct after that falsification: the first Texture provider
turn for initial `texture_agent_revision` runs and `update_coagent` integrate
wakes is now mechanically constrained to `function:patch_texture`
(`internal/runtime/runtime.go:2214-2229`). This was intended to repair the
specific observed failure mode where `"required"` allowed the model to choose
terminal delegation (`spawn_agent`) before any canonical appagent revision. Local
tests proved first-call write-only tool filtering, unconstrained follow-up, and
V2 wake writes for both researcher and super evidence.

Staging evidence after deploy of `7d462629` falsified that repair as currently
implemented: `scripts/texture_revision_cadence_probe.mjs` observed only V0 user
content and no appagent revision at all for prompt-bar submission
`42fb44c0-a0f0-43c2-a883-6c85a007eb8c` / doc
`f01de6d5-c638-414c-8232-db483469da2f`, with `appagent_revision_count=0`,
`research.web_search=0`, `research.source_search=0`, `research.spawn_agent=0`,
`research.update_coagent=0`, and trajectory `state=completed`. Staging health
identified both proxy and sandbox at
`7d462629ca4a5df9b3df3c7b7a707742a8e5b6eb`, so this is not stale deploy
evidence. The most likely new conjecture is that exact first-tool selection is
not honored or is rejected/relaxed into a no-write completed path by the live
provider/adapter, whereas local stub providers accepted it.

Repair after the `7d462629` falsification: the general tool loop
now treats an `end_turn` response during an exact initial tool choice as a retry
condition, not normal completion (`internal/runtime/toolloop.go:630-655`). This
means a provider that ignores or declines exact `function:patch_texture` cannot
silently complete the initial Texture run without a write. The runtime also
settles failed no-write Texture mutations before reconcile
(`internal/runtime/runtime.go:3237-3273`), so a failed integrate wake cannot
immediately requeue the same undelivered packet forever. Local runtime shards
passed.

Staging evidence after deploy of `58f261c8` falsified that repair as sufficient:
`scripts/texture_revision_cadence_probe.mjs` again observed only V0 user content
and no appagent revision for prompt-bar submission
`08c13c3b-8f80-4567-a4a5-7656dfee16b4` / doc
`3d0ccff6-a89a-4a86-af43-c4a6189e9f28`, with `appagent_revision_count=0`,
`first_paint_ms=null`, `research.web_search=0`, `research.source_search=0`,
`research.spawn_agent=0`, `research.update_coagent=0`, and trajectory
`state=completed`. Staging health identified both proxy and sandbox at
`58f261c801f077e37f04ee480905422cbf925b52`, so this is not stale deploy
evidence. The retry/no-write settlement repair did not reach the live failing
branch or did not prevent the run from completing before any Texture appagent
write.

Follow-up Trace diagnostic on the same deployed SHA identified the concrete live
branch. Prompt-bar submission `02be18d3-dfa9-4294-9327-4567e1a4b008` / doc
`76dee478-d2f5-4c73-86ff-781fd9dadfee` activated Texture with exact
`function:patch_texture` and a one-tool `patch_texture` definition. The live
model emitted two `patch_texture` calls. The first failed (`edit 0: find text
not present`) because it tried to replace a fenced prompt block that was only
present in the prompt framing, not the canonical V0 content. The duplicate
write guard then returned the second call as a non-error duplicate notice even
though no revision had been stored. The next unconstrained provider turn ended
with prose, and the run completed with `Texture run completed without storing a
Texture revision`.

Current local repair after that diagnostic: Texture duplicate-write suppression
is now dynamic in sequential tool execution and only suppresses later Texture
write tools after a prior same-turn write has actually stored a structured
revision (`internal/runtime/tools.go:272-288`). The exact-initial-tool branch now
retries when the required initial tool was called but did not succeed
(`internal/runtime/toolloop.go:562-584`), preserving exact `patch_texture` until
there is a stored first revision or bounded retry exhaustion. Local focused tests
and runtime shards passed.

Staging evidence after deploy of `29265cae` shows partial repair, not
settlement. The formal cadence probe for submission
`00523d55-5dee-4a1b-94e6-bb205ea1618d` / doc
`771fa753-3c8e-4484-a24e-1c333a95271e` still observed only V0 and ended with
trajectory `state=failed`, `appagent_revision_count=0`, and no research or
delegation. A follow-up Trace diagnostic on the same deployed SHA then observed
the intended retry branch and a fast V1: submission
`05ddee7c-8ccb-48f9-bc93-1bc313593d2a` / doc
`9cdf22a3-4ac5-4f4b-81cd-564a76b69c1a` failed two initial malformed
`patch_texture` calls (`append edit requires text`), emitted
`required_initial_tool_failed`, retried exact `function:patch_texture`, and
stored appagent V1 at about +16s. That trajectory still stopped after V1 with
`delegation_count=0`, no researcher activity, and no V2+ grounded revision.

Current local repair after that partial staging proof: the tool loop has a
uniform completion-guard hook that can reject an `end_turn` as incomplete and
append an ordinary user-turn reminder, without selecting a semantic next tool
(`internal/runtime/toolloop.go:118-230`, `internal/runtime/toolloop.go:755-784`).
Texture uses that hook only for initial factual/current prompts whose latest
canonical head is still flagged `model_prior_interim` / `revision_grounding:
model_prior`; the guard stands down once the same run opens an evidence path or
records an off-document decision (`internal/runtime/runtime.go:2233-2343`). This
repairs the local V1-only/no-delegation branch without making `edit_texture`
smuggle a forced researcher continuation. New tests prove the generic guard and
the prompt-bar Texture path where a guarded interim V1 opens researcher Probe
work while preserving V1 model-prior metadata
(`internal/runtime/toolloop_test.go:453-538`,
`internal/runtime/texture_test.go:2543-2631`).

Staging evidence after deploy of `58895d28` falsified that local repair as
sufficient. CI test/build jobs passed, but the Node B deploy job again reported
failure while `/health` confirmed both proxy and sandbox deployed at
`58895d28e56dec72e63852fd9eb35bc9ce441ab7`. The formal cadence probe submitted
`012c7431-3645-4c7b-82a7-8efafedc4c2a` / doc
`0e0fcfba-ead8-411f-b264-32d495ba51dd` and still observed only V0:
`appagent_revision_count=0`, `first_paint_ms=null`, no `web_search`,
`source_search`, `spawn_agent`, or `update_coagent`, and trajectory
`state=failed`. This proves the completion guard did not reach the formal probe
path because the run failed before a successful model-prior V1, not because the
guard allowed V1-only completion.

Follow-up product-path diagnostic on the same deployed SHA refined the branch:
prompt-bar submission `653300e5-8f29-4094-8e45-00601bd378b0` / doc
`16301311-92a1-4e57-b87d-88c4c0f99c45` did store a fast appagent V1, but the
revision metadata identified it as `artifact_kind=article_revision`,
`revision_role=canonical`, and `texture_version_stage=article_revision`; it did
not carry `model_prior_interim`, `revision_grounding=model_prior`, or
`grounding_status=model_prior_interim`. Trace showed no `completion_guard` retry,
no researcher delegation, no findings, and trajectory `state=completed`. This
means the guard did not fire on this live V1 branch because prompt-bar initial
Texture revisions can be treated as Wire article revisions, bypassing the
model-prior/interim metadata that the T2 invariant requires.

Current local repair after that metadata diagnostic: prompt-only initial Texture
agent revisions now override accidental Wire article classification before
revision metadata is stored (`internal/runtime/runtime.go:3005-3059`). A
prompt-bar initial V1 with user-prompt / initial-conductor-workflow origin,
no consumed worker update, no scheduled message, and no `update_coagent` source
is marked `model_prior_interim`, `revision_grounding=model_prior`,
`grounding_status=model_prior_interim`, and `texture_version_stage=interim`,
and any leaked `artifact_kind=article_revision` / `revision_role=canonical`
metadata is downgraded to non-publishable working/input metadata. Genuine sourced
Wire article revisions keep the canonical article metadata because the override
only applies to prompt-only initial Texture revision runs. A regression test now
builds the exact article-shaped user-prompt branch and requires interim
model-prior metadata while proving the Wire classifier would otherwise match
(`internal/runtime/runtime_test.go:1662-1734`). The completion-guard product-path
tests were also updated to satisfy addressed `update_coagent` semantics and the
exact-first-write contract used by the current tool loop
(`internal/runtime/texture_test.go:2120-2683`).

Staging evidence after deploy of `f9626242` shows partial repair, not
settlement. CI test/build jobs passed, including internal/runtime shards 0-3,
but the Node B deploy job again concluded failure while public `/health`
confirmed proxy and sandbox both deployed at
`f96262421748902f257fd20aadd61477f7727353` (`deployed_at`
`2026-06-18T03:53:52Z`). The deployed cadence probe submitted
`bddc8556-602a-4cb1-b2be-134371cbb274` / doc
`fff50f6c-93b5-46e8-9a2e-b74cf02a2869`. It observed V0 at +0.386s and a fast
appagent V1 at +28.966s, then no V2: `appagent_revision_count=1`,
`total_revision_count=2`, `first_paint_ms=28966`, and final trajectory
`state=failed`. Trace summary showed the evidence path did open
(`web_search=2`, `source_search=2`, `spawn_agent=2`, `update_coagent=2`,
`delegation_count=1`, `agent_count=3`, `moment_count=128`). This supports the
metadata/guard repair enough to move past no-V1/no-delegation as the only live
branch, but it falsifies the V2+ cadence: worker findings are not being consumed
into a follow-on Texture revision before the trajectory fails.

Follow-up focused diagnostic on the same deployed SHA refined the remaining
branch again. Fresh submission `8b935f7f-339b-4934-959e-6070ad71243c` / doc
`568d6131-0988-4c77-b886-cb541e70c698` produced V0, an appagent V1 at about
+24s, and an appagent V2 at about +73s, with trajectory `state=completed`.
V1 metadata was correctly honest (`model_prior_interim=true`,
`revision_grounding=model_prior`, `grounding_status=model_prior_interim`,
`texture_version_stage=interim`, `revision_role=input`). V2 metadata consumed
researcher update seq 1 (`worker_updates_consumed` role `researcher`,
`worker_updates_pending=[]`) and became canonical `article_revision`, but the
edit was a no-op: `texture_edit_delta_chars=0`, content length stayed 794, and
the rationale said no substantive content change was intended. This means the
remaining live failure is not only delivery or wake: Texture can receive and mark
findings consumed, but the wake revision may satisfy the forced write without
actually grounding or deepening the document.

Current local repair after that no-op V2 diagnostic: `patch_texture` /
`rewrite_texture` now reject a worker-update-consuming Texture write when the
materialized content is identical to the current revision, before the revision is
stored or worker updates are marked delivered (`internal/runtime/tools_texture.go`).
The guard keys off actual content equality, not character-count delta, so
same-length substantive edits remain valid. A regression test constructs an
addressed researcher update, schedules an `update_coagent` Texture wake, attempts
an unchanged patch with the live-style "no substantive content change intended"
rationale, and proves no revision, checkpoint, or completed mutation is created
(`internal/runtime/texture_test.go`).

Staging evidence after deploy of `157db34f` supports that no-op V2 guard but
falsifies the current V1 quality/timing claim. CI test/build jobs passed,
including internal/runtime shards 0-3, but the Node B deploy job again concluded
failure while public `/health` confirmed proxy and sandbox both deployed at
`157db34f3330e64ff55541a71afc5776ba4e1410` (`deployed_at`
`2026-06-18T04:14:54Z`). The deployed cadence probe submitted
`dfb565f2-affe-4cab-a422-f3049777eff5` / doc
`947e4baf-65d0-47c5-97ea-20d04b692840`. It observed V0 at +0.326s, an
appagent V1 at +49.350s with only 53 chars (the same prompt-sized content as
V0), and an appagent V2 at +88.448s with 1336 chars. The trajectory completed
with `appagent_revision_count=2`, `total_revision_count=3`, `web_search=8`,
`source_search=2`, `spawn_agent=2`, `update_coagent=2`, `delegation_count=1`,
and `agent_count=3`. This proves the evidence-consuming no-op V2 branch can
recover into a substantive V2 on staging, but it also proves the initial exact
write can still burn the fast first paint as a no-op prompt copy and miss the
under-49s useful-V1 target.

Current local repair after that no-op V1 staging result: the same content-equality
guard now also rejects prompt-only initial model-prior Texture writes before any
revision is stored (`internal/runtime/tools_texture.go`). This applies only after
metadata classification proves the write is `model_prior_interim` /
`revision_grounding=model_prior`, so sourced Wire article revisions and ordinary
non-initial edits are not reclassified. New tests prove the low-level no-op
prompt-copy rejection leaves the mutation pending, and the prompt-bar path
retries exact `patch_texture` after the failed no-op result and stores a useful
model-prior V1 on the same run (`internal/runtime/texture_test.go`).

Staging evidence after deploy of `84038c4a` falsified that local repair as
sufficient. CI test/build jobs passed, including internal/runtime shards 0-3,
but the Node B deploy job again concluded failure while public `/health`
confirmed proxy and sandbox both deployed at
`84038c4ae972c0aa3a32b18b6b227e763a9be777` (`deployed_at`
`2026-06-18T04:34:01Z`). The deployed cadence probe submitted
`e83758bf-5b54-44a7-8838-3c4e686a8b30` / doc
`6ed92ad9-c861-458c-b5c4-a09ca48dc529`. It observed only V0 user content:
`appagent_revision_count=0`, `first_paint_ms=null`, `total_revision_count=1`,
`web_search=0`, `source_search=0`, `spawn_agent=0`, `update_coagent=0`, and
trajectory `state=failed`. This proves the prompt-copy guard prevented the bad
stored V1, but the live provider did not recover into a useful V1 before the run
failed.

Follow-up product-path evidence on the same deployed SHA split that branch
instead of confirming deterministic failure. A focused diagnostic submission
`534bdb39-b582-43d8-9fc9-8a961b8a4fbd` / doc
`83a9d899-e062-4f49-9052-2f8f15a112f8` produced an honest V1
(`model_prior_interim=true`, `revision_grounding=model_prior`,
`texture_version_stage=interim`) with 602 chars, then consumed researcher
updates into V2 and V3, finishing completed. A repeat formal cadence probe
submitted `ca430c89-b259-453b-900c-2863c0e38567` / doc
`37277fa3-7843-41ba-9e69-b6b6e16c37fb` and observed V1 at +23.795s
(569 chars), V2 at +49.844s (859 chars), and V3 at +65.631s (1509 chars),
with trajectory `state=completed`, `appagent_revision_count=3`,
`web_search=4`, `spawn_agent=2`, and `update_coagent=4`. This supports the
prompt-copy guard and multi-revision cadence on staging, but the earlier
same-SHA no-V1 formal probe remains a stochastic live failure to preserve as
residual risk rather than erase.

Current deployed T3 partial construct: the generic tool loop now accepts a
role-uniform cumulative `ToolLoopBudget` with provider-call, input-token,
output-token, total-token, and elapsed-time limits, emits Trace-visible budget
configuration and exhaustion evidence, and returns a budget-exhausted error
instead of falling through to the bare 200-iteration ceiling
(`internal/runtime/toolloop.go`). Texture revision runs attach a conservative
actor-labeled budget (`texture:<docID>`) through the existing run setup
(`internal/runtime/runtime.go`), with metadata overrides for provider-call,
token, and elapsed-time limits. This repairs only the bounded-cost/kill-switch
slice of T3; it does not implement park-and-wait, no-billed idle blocking, or
cross-passivation cumulative accounting.

Staging proof after deploy of `f5884e08` supports this partial construct as
non-regressing for the currently supported cadence slice. CI test/build jobs
passed, including internal/runtime shards 0-3; the workflow concluded failure
only because the known Node B deploy job failed. Public `/health` confirmed both
proxy and sandbox deployed at
`f5884e08977f74ed463a55a19e9ece3cd24dc06f` (`deployed_at`
`2026-06-18T04:59:36Z`). The deployed cadence probe submitted
`820581e2-ef7f-430f-80a5-5e148a3552d7` / doc
`1e9bae65-6953-49c2-a95d-c75370e3e855`, observed appagent V1 at +23.508s
(880 chars) and appagent V2 at +73.013s (1786 chars), and finished completed
with `web_search=2`, `source_search=2`, `spawn_agent=2`, `update_coagent=2`,
`agent_count=3`, and `delegation_count=1`. This does not settle the full mission:
the probe produced one V2, not an hours/days parked actor stream or a
RunAcceptanceRecord-backed lifecycle proof.

Current local T3 construct extends the budget substrate with a role-uniform
park-and-wait primitive. `RunToolLoop` can now accept `WithParkWaiter`, emit
`park_wait_started` / `park_wait_finished` progress, block without provider
calls, and resume only after runtime-owned user turns are injected
(`internal/runtime/toolloop.go`). Runtime now keeps an owner+agent waiter map
and `update_coagent` notifies resident waiters before the existing warm/cold
wake path (`internal/runtime/runtime.go`, `internal/runtime/super_controller.go`).
The coagent waiter is metadata-gated by `actor_park_on_idle`, so this is an
opt-in foundation and not yet the default Texture lifecycle. Local focused tests
prove no provider calls occur while parked until an injected update arrives, and
prove runtime owner+agent signaling wakes a parked waiter
(`internal/runtime/toolloop_test.go`). This repairs the no-billed-idle primitive
slice of T3 locally; it does not yet settle cross-passivation cumulative budget
accounting or make every `texture:<docID>` a parked actor by default.

Staging after deploy of `d7b7ae49` proves the park-and-wait primitive was
deployed but does not provide product-path cadence acceptance. CI test/build
jobs passed, including runtime shards 0-3, and public `/health` confirmed proxy
and sandbox both deployed at
`d7b7ae4929a92623dcdd99e766ffce0c189c0a86` (`deployed_at`
`2026-06-18T05:25:50Z`); the workflow concluded failure only because the known
Node B deploy job failed. The formal deployed cadence probe submitted
`a69f85b6-25b4-428e-be8f-63a366480383` / doc
`c4257df0-bd3b-433a-aa1d-1ac3ed775f69` and observed only V0 user content:
`appagent_revision_count=0`, `first_paint_ms=null`, `total_revision_count=1`,
`web_search=0`, `source_search=0`, `spawn_agent=0`, `update_coagent=0`, and
trajectory `state=failed`. This repeats the previously named stochastic no-V1
live branch. It does not falsify the metadata-gated park primitive directly,
because the probe does not enable `actor_park_on_idle`, but it blocks acceptance
for the deployed product path.

Follow-up product-path diagnostics narrowed that branch without using internal
routes. Diagnostic `038fc8b3-a422-42a3-bf63-dbf5d6d122fe` / doc
`06274f82-702f-41db-9567-ce7c0e0ccbf1` reproduced the no-V1 failure with
Texture failing after `required initial tool "patch_texture" did not succeed
after 2 retries`; every `patch_texture` result in that run was an error.
Diagnostic `ff54e889-bb5e-42c9-b679-f89aa9e90c9e` / doc
`bc97e669-3517-434a-8921-00978e5e4fa7` captured the actual live tool errors on
a recovered branch: first `tool_error: edit 0: find text not present`, then
`tool_error: initial model-prior Texture revision must change prompt content
before first paint is stored`, then a successful stored appagent V1 and later
V2. Commit `4da4ffa3` made the generic required-initial-tool retry reminder
specific for failed initial `patch_texture`: for first-paint drafts it tells the
model not to replace prompt text or copy it unchanged, and to use an append edit
with substantive draft content. CI test/build jobs passed and public `/health`
confirmed proxy and sandbox both deployed at
`4da4ffa3fc9d6831e3d5643b6993aaba4ad67d9e` (`deployed_at`
`2026-06-18T05:48:23Z`). The formal deployed cadence probe then submitted
`ce488219-549d-439e-8f90-a6c20edf2318` / doc
`f756294c-3610-4a7d-bf3e-97bc40e55665` and observed V1 at +23.469s
(670 chars), V2 at +67.701s (1284 chars), and V3 at +86.062s (1831 chars),
with `appagent_revision_count=3`, `web_search=6`, `source_search=2`,
`spawn_agent=2`, `update_coagent=4`, and trajectory `state=completed`.

An acceptance-enabled rerun synthesized RunAcceptanceRecord
`runacc-7760011a3b329bc50fb5` for trajectory
`7df99090-4ed4-4571-a63e-cb03ed5b2f78` at `staging-smoke-level`, state
`blocked`. That same rerun still exposed residual first-paint stochasticity:
V1 arrived at +49.355s with only 65 chars before a substantive V2 at +83.212s.
So the current construct supports the cadence slice on staging but does not yet
settle the mission's stronger "useful immediate V1 every time" or long-running
resident actor lifecycle claims.

Current local T5 construct after the bounded default-park staging proof:
replacement Texture activations now carry an `actor_rewarm_source_loop_id` and
budget-spend baseline from the latest completed/passivated run for the same
owner+agent. The generic tool-loop budget accepts prior provider-call and token
spend, emits `tool_loop_budget_usage` after every provider response, and checks
new activations against cumulative spend rather than granting a fresh full
budget (`internal/runtime/toolloop.go:151-160`,
`internal/runtime/toolloop.go:580-585`,
`internal/runtime/toolloop.go:992-1078`). Texture revision run metadata reads
that spend from durable prior-run events and stores it on rewarm
(`internal/runtime/runtime.go:2246-2325`,
`internal/runtime/texture_agent_revision.go:327-350`). The restart regression
now uses a real durable `update_coagent` row, passivates an interrupted Texture
run with prior run-memory and budget events, then proves the replacement
activation stores a recovered appagent revision, marks the old mutation stale,
seeds an `actor_rewarm` run-memory snapshot, carries cumulative budget metadata,
and consumes the pending worker update without duplicate pending mutation
(`internal/runtime/texture_test.go:4623-4827`).

Remaining audited value after deployed `6f54e890`: T6 is locally repaired.
DELETE `/api/texture/documents/{id}` now cancels a pending Texture actor
trajectory/mutation before deleting document rows
(`internal/runtime/texture.go:1048-1087`), and the focused runtime test proves
document deletion cancels the document, mutation, run, work item, and
trajectory (`internal/runtime/texture_test.go:674-769`). T7 is locally repaired:
the workflow verifier accepts N:1 loop-to-revision causality and scopes
worker-update checks to updates addressed to the routed Texture document
(`internal/runtime/texture_workflow_verifier.go:144-208`,
`internal/runtime/texture_workflow_verifier.go:250-258`), with direct N:1 proof
(`internal/runtime/texture_workflow_verifier_test.go:362-418`), doctrine updates,
and doccheck coverage for `WithInitialToolChoice`. Deployed staging proof after
`6f54e890` is cadence non-regression, a blocked staging-smoke acceptance
record, and delete-specific product proof. The formal deployed probe
`971f62fc-b451-4683-95c2-91e38a7e0c72` / doc
`ded70294-5bd8-46ee-8cca-9767a0c11301` reached V1 at 13.281s and V2 at
60.538s. The same-session acceptance proof
`da8a98bd-a42b-499f-ac94-e70d4b0d17b1` / doc
`d3269cd5-cc93-49c4-9cb0-8c38f4fbe7f2` reached V1 at 8.131s and V2 at
47.374s, then synthesized `RunAcceptanceRecord`
`runacc-bc65036ba592ab3b18cd` at `staging-smoke-level`, state `blocked`.
The delete-cancellation probe submitted
`33433a32-2d00-400d-b471-81277b731282` / doc
`40d38988-6427-4ee7-aefe-e36826570981`; before delete, Trace showed
`texture:40d38988-6427-4ee7-aefe-e36826570981` running and the document exposed
`agent_revision_pending=true` with run
`00d01775-4e20-48d2-96c2-0a847cd36797`. DELETE returned 200 at +0.372s, the
document then returned 404, and Trace ended `state=cancelled`, `live=false`,
with the Texture agent state `cancelled`. Remaining T8: non-blocked lifecycle
acceptance, elapsed-time budget across sleeps, full removal/collapse of
wake/reconcile scaffolding, Trace projection legibility, and stronger
first-write determinism if needed. A follow-up runtime-supervision acceptance
probe on the same deployed SHA submitted
`65cce9c1-86ab-4cef-871a-16481b25be49` / doc
`1aa44fff-62c7-4913-a17d-3a04fd79c317` with an explicit downstream-super /
worker-delegation prompt. Trace showed conductor + Texture + super agents, but
zero `request_super_execution`, `request_worker_vm`, `start_worker_delegation`,
`observe_worker_delegation`, `finish_worker_delegation`, `delegate_worker_vm`, or
`update_coagent` tool results; the trajectory completed after two revisions and
the synthesized `RunAcceptanceRecord` `runacc-3efa017d5d01175a8bcf` stayed
`staging-smoke-level`, state `blocked`, with only `submitted` and
`texture_opened` checkpoints. This newly documents the T8 blocker for
non-blocked lifecycle acceptance.

Current deployed proof after `623a33de` refines C9 but does not settle T8. CI
test/build jobs passed, including runtime shards 0-3; the workflow concluded
failure only because the known Node B deploy job failed. Public `/health`
confirmed proxy and sandbox both deployed at
`623a33de8157ce66714ead581575d5a188cf80d2` (`deployed_at`
`2026-06-18T08:20:04Z`). The required cadence probe then hit the known no-V1
branch: submission `b52686cd-d54a-4856-a08c-18e24ea64f6b` / doc
`61fa7f9f-00f9-4f4b-994e-f8262f461d48` produced only V0, no appagent
revision, no research/delegation activity, and trajectory `state=failed`. A
runtime-supervision discriminator on the same SHA submitted
`375ff7b6-25f4-4054-84c8-31c2d2509cfd` / doc
`789b7401-b03d-41cd-81c4-ee4d396a2854`; it produced two appagent revisions and
the acceptance synthesizer now recorded `super_requested` from
`request_super_execution`, yielding `RunAcceptanceRecord`
`runacc-d8560d65cb9f8a118c8f` at `staging-smoke-level`, state `blocked`, with
checkpoints `submitted`, `texture_opened`, and `super_requested`. Trace still
showed `request_worker_vm=0`, `start_worker_delegation=0`, and
`delegation_count=0`, while `update_coagent=2`. The acceptance repair therefore
closed the "super request invisible" half of C9, but the completion guard's
`update_coagent` fallback is too broad for objectives that explicitly require
worker/delegation evidence.

Current deployed proof after `f4eca79c` supports the ordinary cadence slice but
falsifies the narrower Super-guard repair as sufficient for lifecycle
acceptance. CI test/build jobs passed, including runtime shards 0-3; the known
Node B deploy job failed while public `/health` confirmed proxy and sandbox both
deployed at `f4eca79c26d933a2038a097f79117d8c360f9add` (`deployed_at`
`2026-06-18T08:39:21Z`). The formal cadence probe submitted
`8ef2b5af-d081-40eb-bc12-11433fb622d4` / doc
`6e4f47ba-eb75-4e25-aa0a-3a0865cc6bc7`, reached V1 at +33.981s, V2 at
75.851s, and V3 at +104.807s with `web_search=6`, `spawn_agent=2`,
`update_coagent=4`, and trajectory `state=completed`. A lifecycle discriminator
on the same SHA submitted `c1d89f74-f77d-47ad-ab29-2770930df51b` / doc
`ff2f6142-e274-4ed1-81bd-50b410d609a5`; it completed with only conductor +
Texture agents, one appagent revision, no `request_super_execution`, no Super
agent, no `request_worker_vm`, and no delegation evidence. The synthesized
`RunAcceptanceRecord` `runacc-5ff8fb28e8fa5efa0d05` remained
`staging-smoke-level`, state `blocked`, with only `submitted` and
`texture_opened` checkpoints. This shows the next live gap is upstream of the
Super guard: Texture can still treat an explicit downstream-Super/worker prompt
as satisfied after a Texture-only revision.

Current deployed proof after `f26e1f7c` repairs that upstream C9 branch but does
not settle lifecycle acceptance. CI run `27748513416` concluded failure only
because `Deploy to Staging (Node B)` failed; Docs Truth Check, Go vet/build, Go
runtime shards 0-3, non-runtime tests, integration smoke, and TLA+ all
succeeded. FlakeHub run `27748513112` succeeded. Public `/health` confirmed
proxy and sandbox both deployed at
`f26e1f7c3650b84e346afff9394db1dd409f0fe0` (`deployed_at`
`2026-06-18T09:03:45Z`). The formal cadence probe submitted
`983f18ea-e15b-43e4-83a7-eb4bfc2b2e4a` / doc
`3cc32113-7625-44d9-80d5-f46aa1183cae`, reached V1 at +26.135s and V2 at
+65.212s, and completed with `web_search=8`, `source_search=2`,
`spawn_agent=2`, `update_coagent=2`, `agent_count=3`, and
`delegation_count=1`. The lifecycle discriminator submitted
`a6daf4be-c820-43e1-94ed-4ff58820c19c` / doc
`970aebb5-f43f-4fb3-804c-8f83b2dad4cf`; it reached V1 at +10.598s, V2 at
+49.628s, and V3 at +75.908s, with conductor + Texture + Super agents and
Trace tool counts `request_super_execution=2`, `request_worker_vm=2`,
`start_worker_delegation=2`, `observe_worker_delegation=2`,
`finish_worker_delegation=2`, `update_coagent=4`, and
`record_texture_decision=2`. The synthesized `RunAcceptanceRecord`
`runacc-3dba253452a1c51b98f9` remained `staging-smoke-level`, state `blocked`:
checkpoints `submitted`, `texture_opened`, `super_requested`, and
`worker_leased` passed, while `worker_delegated` was blocked because the worker
child run was still `running` and no AppChangePackage/adoption evidence existed.
The next live gap is now downstream of Texture and Super entry: worker
delegation must either finish with bounded worker-update/package evidence or
surface a durable blocker that acceptance can classify without overclaiming
promotion.

budget: one broad red-surface paramission executed iteratively (Codex one-shot ->
critical review -> iterate). Broad change is authorized; there are no real users
yet and the owner values the correct long-running-agent architecture over
incremental compatibility.

authority / bounds: document creation here is green. Execution is red: Texture
canonical writes, the run tool loop / park-and-wait / budget, passivation and
rewarm, worker-update consumption bookkeeping, the verifier, Trace, and
deployment routing. Before touching orange/red surfaces the executor must name
the conjecture delta, protected surfaces, admissible evidence class, rollback
path, and heresy delta.

mutation class / protected surfaces: planned execution mutation class red.
Protected surfaces: Texture canonical writes, run lifecycle / tool loop, the new
park-and-wait and budget primitives, passivation/rewarm, worker-update
consumption/pending bookkeeping, the Texture workflow verifier, Trace/evidence,
and deployment routing.

evidence packet: focused local tests for multi-write-per-run,
warm-injection-on-for-Texture, park-and-wait blocking without billed calls,
budget kill-switch, sleep/resume, and N:1 verifier causality; go-test-runtime
shards; frontend build if touched; CI run; deploy run; staging health host +
sandbox upstream commit identity; deployed probe output (from-weights first-paint
time, revision count, findings-packet correlation); prompt-bar submission id,
conductor run id, doc id, Texture run id; RunAcceptanceRecord at
staging-smoke-level; residual risk note. Rollback ref is revert of the mission
commits.

heresy delta: discovered - the one-write-per-run + warm-injection-off shape (the
latter introduced by 68d09cc3) caps Texture at a single late revision and is the
root of the V1-only product defect; the runtime has no cheap park-and-wait, no
cumulative cost budget, and a doc-delete that does not cancel runs. To be
repaired by this mission. Do not count these newly named heresies as regressions;
do not count their naming as repair.

position / live conjectures / open edges:

- C1 repaired locally: one-write-per-run was the artificial cap. The
  AgentMutation row is now run-liveness/idempotency state rather than the
  per-write terminal gate; stale base revisions still reject duplicate writes,
  while fresh same-run writes can deepen the document.
- C2 partially supported on staging: first paint can now be fast in the live
  product path (`29265cae` diagnostic V1 at about +16s, `58895d28` diagnostic
  fast V1, `f9626242` formal probe V1 at +28.966s, focused diagnostic V1 at
  about +24s). The `f9626242` focused diagnostic proves V1 metadata is now
  model-prior/interim on staging and that V2 can consume a researcher update.
  Staging `157db34f` then proved a consumed-evidence wake can recover into a
  substantive V2 instead of burning the update with identical content. The live
  T2/T8 blocker has moved back to initial V1 quality: the same formal probe
  stored a 53-char prompt-sized V1 only at +49.350s before deepening at V2, so
  first paint was neither clearly under the old ~49s baseline nor useful as a
  from-weights draft. Locally, identical prompt-only model-prior writes are now
  rejected and retried before storage; staging `84038c4a` proved both branches:
  one formal probe failed with no appagent V1, while a focused diagnostic and a
  repeat formal probe recovered into a useful V1 and multiple V2+ revisions.
  Commit `4da4ffa3` then produced a formal staging probe with V1 at +23.469s and
  V2/V3 revisions that tracked researcher activity, but the acceptance-enabled
  rerun still produced a weak late V1 (+49.355s, 65 chars) before substantive
  V2. The active residual risk is stochastic first-write quality/timing, not the
  deterministic no-op storage branch.
- C3 partially repaired locally: "one run per agent" is more minimal as a model
  but requires a real park-and-wait + budget. The budget and role-uniform
  park-and-wait primitive now exist locally; the remaining error is enabling
  them as the Texture lifecycle, making budget accounting cumulative across
  sleep/rewarm, and proving the actor parks without billed calls in the deployed
  product path.
- C4 active: passivation means even "one run" is one logical actor across
  physical runs; sleep/resume from the run-memory snapshot is the continuity
  mechanism, not an immortal process.
- C5 partially repaired: cost/runaway and cancellation are the top risks of a
  long-lived actor. Provider-call/token budget carry-forward and doc-delete
  cancellation now have local construct proof; doc-delete cancellation also has
  deployed product-path proof on staging. Elapsed-time budget across sleeps
  remains open.
- C6 partially repaired locally: collapsing many revisions into one run must not
  muddy trajectory/work-item attribution or the per-revision supervision
  narrative. The workflow verifier now accepts N:1 loop-to-revision causality;
  Trace projection legibility remains a residual T8 edge.
- C7 active from 2026-06-17 audit: the current code has a contradictory
  transitional shape. `texture_controller.go` schedules multiple separate
  integrate runs to simulate cadence, while `super_controller.go` and
  `tools_texture.go` still block the resident-run multi-revision model. The next
  construct must replace that contradiction by letting the same Texture
  activation warm-inject addressed packets and commit more than one canonical
  revision, with revision metadata showing which packet set each write consumed.
- C8 falsified/refined by 2026-06-17/18 staging: prompt/tool choice can defeat a
  merely "required" first action by choosing a terminal tool before any canonical
  revision, but exact first `patch_texture` as implemented can also collapse the
  live provider path into no appagent revision and no delegation. Retrying
  provider `end_turn` during exact initial tool choice and preventing no-write
  failure reconcile loops did not change staging behavior at `58f261c8`. A
  follow-up Trace diagnostic narrowed the branch to failed `patch_texture`
  results being treated as a satisfied initial write because a duplicate
  same-turn Texture write returned a non-error notice. The `29265cae` repair
  reaches that branch and can produce V1, but stochastic invalid edit arguments
  can still exhaust/failed-run the first write. The current local completion
  guard targets the successful-V1/no-Probe sub-branch without forced semantic
  next-tool enforcement. Staging `58895d28` showed the formal probe can still die
  in the no-V1 branch; a fresh diagnostic also showed a successful V1 branch
  where `article_revision` metadata suppresses the model-prior/interim flags. The
  latest metadata repair fixes that classification without weakening real Wire
  article revision semantics or forcing a semantic researcher continuation.
  Staging `f9626242` moved the live failure forward: V1 and evidence work happen;
  one formal probe failed before V2, while a focused diagnostic reached V2 but
  stored a no-op revision that consumed researcher evidence without using it. The
  `157db34f` no-op guard prevents that exact burn-update branch on staging well
  enough for the formal probe to reach a substantive V2. The next discriminator
  is the initial no-worker exact-write branch: a prompt-sized no-op V1 should be
  rejected or retried without delaying the first useful from-weights draft into
  the old first-findings window. The `4da4ffa3` retry-guidance repair improved
  the formal probe into fast useful V1 plus V2/V3, but a follow-up acceptance run
  still hit a weak late V1 branch. The next discriminator is no longer whether
  multi-revision cadence can happen on staging; it is whether the first-paint
  obligation should become a stronger mechanical draft scaffold or whether T4's
  parked lifecycle should be enabled first and measure recovery over many
  packets.
- C9 discovered by 2026-06-18 runtime-supervision acceptance probe: an
  execution-shaped prompt that should open downstream super/worker evidence can
  complete as Texture-only for acceptance purposes. The trajectory has a super
  agent, but no durable `request_super_execution` tool result, no
  `request_worker_vm`, and no delegation/worker-update evidence. The next
  discriminator is whether deterministic initial Texture super requests need
  explicit Trace/acceptance evidence, whether super needs a bounded completion
  guard for Texture-requested execution objectives, or both. Commit `623a33de`
  repaired the Trace/acceptance visibility half: deployed acceptance now records
  `super_requested` from `request_super_execution`. It did not repair the worker
  evidence half: Super can still complete a worker-shaped Texture request after
  `update_coagent` without `request_worker_vm` or delegation evidence. Commit
  `f4eca79c` then tightened that Super branch locally, but deployed proof showed
  a more upstream live branch: Texture can complete the explicit
  downstream-Super/worker prompt before any `request_super_execution` occurs.
  Commit `f26e1f7c` repaired that upstream branch: deployed proof now shows
  Texture requesting Super, Super leasing a worker VM, and delegation tools
  appearing in Trace. The remaining C9 blocker is downstream worker settlement:
  the worker delegation can remain active/running with no AppChangePackage or
  acceptance-visible completed worker-update evidence, so the synthesized
  staging-smoke record remains blocked at `worker_delegated`. Commit
  `8c2bfa71` tightened the local Super completion guard so `worker_run_active`
  is progress, not terminal evidence, and the ordinary cadence probe passed on
  staging. The lifecycle discriminator exposed a different live C9 branch:
  Texture requested Super, but Super completed after `update_coagent` responses
  without any `request_worker_vm`, delegation, package, or explicit
  `record_texture_decision` evidence, so acceptance stayed blocked at
  `super_requested`. Commit `af141a05` then required explicit terminal blocker
  evidence for worker-shaped Texture-requested Super runs. Staging showed that
  stricter blocker predicate changed the live branch but did not settle C9:
  Super still made no worker request. The likely remaining defect is
  classification, not blocker semantics. Live persistent Super runs appear to
  carry a generic processing prompt/metadata while the actual Texture-requested
  objective sits in pending `update_coagent` delivery content, so the completion
  guard may not recognize the run as worker-shaped.

Current local T4 construct after the `4da4ffa3` proof: `LoadConfig` now defaults
Texture revision actors into bounded park-on-idle with
`RUNTIME_TEXTURE_ACTOR_PARK_IDLE` / `DefaultTextureActorParkIdle`, and
`submitTextureAgentRevisionRun` stamps `actor_park_on_idle` plus
`actor_park_idle_seconds` only when the runtime config enables it. This turns
the existing metadata-gated waiter from a test-only primitive into the deployed
default lifecycle for `texture:<docID>` revision runs, while keeping
hand-constructed test configs zero unless they opt in. A new comprehensive
runtime test proves one resident Texture revision run writes V1, remains running
while parked, consumes a later `update_coagent` packet without a cold wake run,
writes V2 in the same run, marks the worker update delivered, and keeps first
turn `patch_texture` exact while parked follow-up turns are unconstrained.

This is a bounded T4 construct, not full settlement. It has now been extended
with a T5 rewarm construct: a restarted Texture actor passivates the old
activation, marks the old mutation stale, starts a replacement activation for
the same logical `texture:<docID>` actor, seeds an `actor_rewarm` memory snapshot,
and carries provider-call/token budget spend into the new activation. T6/T7
added local cancellation and verifier proof: document deletion cancels pending
Texture actor trajectory/mutation before deleting rows, and the verifier accepts
N:1 loop-to-revision causality for Texture write tools. The remaining lifecycle
edges are elapsed-time budget across sleep, full wake/reconcile collapse,
delete-specific deployed product proof if required, Trace projection, and a
non-blocked lifecycle acceptance record.

Deployed proof after `68c6e5b0` shows the bounded default-park construct did not
regress the product cadence slice. Staging health identified proxy and sandbox at
`68c6e5b0b5dd4315719ee27cc11a861e8eaa70cb`, and the product prompt-bar cadence
probe reached V1 at 18.409s and V2 at 68.035s. A same-session acceptance rerun
reached V1 at 18.356s and V2 at 60.096s, then synthesized
`RunAcceptanceRecord` `runacc-60e41bc0a8f6cf708f3e` at
`staging-smoke-level`, state `blocked`. This records acceptance evidence for the
bounded T4 slice while explicitly preserving the T5-T8 blockers.

Deployed proof after `5f1f056a` shows the T5 rewarm/budget carry-forward
construct did not regress the prompt-bar cadence slice. Staging health identified
proxy and sandbox at `5f1f056a3a8ffce8d26e8f04dc1900e0628c5d78`. The formal
cadence probe submitted `b0397265-e664-4ef6-8a0c-bbf56ec5f108` / doc
`d7d6b6f6-2236-456d-8d54-10984d8a2247`, reached V1 at 23.635s and V2 at
86.381s, and completed with `web_search=6`, `source_search=2`, `spawn_agent=2`,
and `update_coagent=2`. A same-session acceptance rerun submitted
`b35e05de-6aae-4b34-874f-1df2b8a6642b` / doc
`b7a51541-5a44-4c8b-9e8a-76320ba1bdd7`, reached V1 at 13.220s and V2 at
39.238s, then synthesized `RunAcceptanceRecord`
`runacc-b5021f57de1fbd3a5c97` at `staging-smoke-level`, state `blocked`.

Deployed proof after `6f54e890` shows the T6/T7 cancellation/verifier/heresy
slice did not regress the prompt-bar cadence slice, and later product-path proof
shows doc-delete cancels a live pending Texture actor on staging. Staging health
identified proxy and sandbox at `6f54e8906205e38db14a2460c13d44666cef9532`. The formal
cadence probe submitted `971f62fc-b451-4683-95c2-91e38a7e0c72` / doc
`ded70294-5bd8-46ee-8cca-9767a0c11301`, reached V1 at 13.281s and V2 at
60.538s, and completed with `web_search=2`, `source_search=2`,
`spawn_agent=2`, and `update_coagent=2`. A same-session acceptance rerun
submitted `da8a98bd-a42b-499f-ac94-e70d4b0d17b1` / doc
`d3269cd5-cc93-49c4-9cb0-8c38f4fbe7f2`, reached V1 at 8.131s and V2 at
47.374s, then synthesized `RunAcceptanceRecord`
`runacc-bc65036ba592ab3b18cd` at `staging-smoke-level`, state `blocked`.
The delete-cancellation probe submitted
`33433a32-2d00-400d-b471-81277b731282` / doc
`40d38988-6427-4ee7-aefe-e36826570981`, observed
`agent_revision_pending=true` and Trace `state=running`, `live=true`,
`agent_count=2` before deletion, then DELETE returned 200, document GET returned
404, and Trace ended `state=cancelled`, `live=false`, with the Texture agent
state `cancelled`.

Current deployed proof after `af141a05` supports ordinary cadence but still
blocks lifecycle acceptance. CI test/build jobs passed, including runtime shards
0-3; the workflow concluded failure only because the known Node B deploy job
failed, while public `/health` confirmed proxy and sandbox both deployed at
`af141a055718f46a2ac4f272959aaaec95031676` (`deployed_at`
`2026-06-18T09:54:47Z`). The formal cadence probe submitted
`ece414f0-6e9b-4036-9708-6062afb0795f` / doc
`97310201-83da-4db5-b711-da5985f8bdf5`, reached V1 at 8.064s and V2 at
57.606s, and completed with `web_search=2`, `source_search=2`,
`spawn_agent=2`, and `update_coagent=2`. The lifecycle discriminator submitted
`988e2640-8bb8-4674-a043-839ec1193376` / doc
`775b1920-4e3c-47d5-9348-3942f6374be7`, reached V1 at 10.629s, V2 at
39.206s, and V3 at 52.348s, then synthesized `RunAcceptanceRecord`
`runacc-852978f224d07441271d` at `staging-smoke-level`, state `blocked`.
Trace showed `request_super_execution=2`, `update_coagent=4`, and
`record_texture_decision=2`, but `request_worker_vm=0`,
`start_worker_delegation=0`, `observe_worker_delegation=0`,
`finish_worker_delegation=0`, and `publish_app_change_package=0`.

next move: repair the C9 Super worker-shape classification branch exposed by
`af141a05`. The completion guard must inspect the actual Texture-requested
objective delivered to persistent Super, not only the generic Super run prompt
or top-level metadata. For a delivered objective that explicitly requires worker
VM / delegation evidence, Super must either request the worker and drive it to
terminal worker-update/package evidence, or return a durable
acceptance-visible blocker. Residuals after that remain elapsed-time budget
across sleep, full wake/reconcile collapse, Trace projection legibility, and
first-write stochasticity.

ledger file: docs/mission-texture-long-running-agent-v0.ledger.md

lineage: supersedes and folds in
`docs/mission-texture-product-loop-recovery-v0.md` (the V1-only cadence defect
and the `68d09cc3` falsification are its direct lineage). Sits on the portfolio
spine after the Texture product-loop work and before continuation deletion (M4);
this mission's lifecycle/passivation work should be reconciled with M4 and the
durable-actors rearchitecture.

learning state: prior increment `68d09cc3` deployed and falsified on staging
(V1-only at ~49s), proving warm-injection-off was necessary-but-not-sufficient
and reinforced the cap. The `8dbdd458` construct then re-enabled Texture warm
injection and same-run multi-write, but staging still produced V1-only at
47.057s because the initial "required" tool choice allowed terminal delegation
before a canonical write. The `7d462629` exact-first-write construct locally
proved the intended order but staging produced no appagent revision at all. The
redesign direction (long-running actor + from-weights V1) was owner-selected
after the first falsification; the current learning is that the repair must be
live-provider-compatible, not just exact-tool correct under stubs. The latest
local repair made provider `end_turn` non-terminal during exact first-tool
obligations and avoided reconcile spin on no-write failures, but staging at
`58f261c8` still produced V0-only with no research activity. Follow-up Trace
proved the live failure is inside the write batch: a failed first
`patch_texture` plus non-error duplicate notice let the initial write obligation
fall through. The `29265cae` repair makes success, not mere tool-call presence,
the condition for satisfying exact initial `patch_texture`; staging proved it can
produce fast V1, but also proved V1-only/no-delegation remains and the cadence
probe can still hit a failed no-V1 run. The current local repair is a bounded
completion guard rather than a required researcher continuation: it preserves
Texture agency by allowing Probe, Execute, follow-up, or an audit decision, but
rejects silent completion while the only canonical appagent revision is still
model-prior/interim for a factual/current request. Staging `58895d28` showed this
does not yet solve acceptance: the formal probe still hit a no-V1 failure, and a
fresh product-path diagnostic showed that the successful V1 branch can omit the
model-prior/interim flags by classifying the prompt-bar revision as
`article_revision`. The latest repair covers the honest-grounding half by making
prompt-only initial revisions model-prior/interim even when article-shaped, and
staging `f9626242` reached V1 plus research/delegation. A formal probe still
showed a failed no-V2 branch, but the focused diagnostic proved the wake path can
produce V2 and consume researcher evidence. The new live learning is sharper:
the V2 wake can be a no-op that marks evidence consumed without incorporating it,
so consumed-evidence revisions must be substantively grounded or explicitly
accountable. The `157db34f` repair rejects identical consumed-evidence writes
before marking updates delivered, and staging then recovered into a substantive
V2. The current live learning is that the initial no-worker exact write can
still store a prompt-sized no-op V1 after roughly the old first-findings delay,
so the first-paint repair must make unchanged initial writes retry or fail
without erasing the route that now reaches substantive V2. The current local
repair applies the same content-equality principle to prompt-only
model-prior/interim writes and proves exact-tool retry can recover into a useful
draft locally. Staging `84038c4a` disproved live recovery: the bad V1 was not
stored in one formal probe, but no useful V1 or evidence path appeared before
trajectory failure. A focused diagnostic and repeat formal probe on the same SHA
then recovered into useful V1 + multiple V2+ revisions. The learning is now that
the guard is structurally right but first-write recovery remains probabilistic on
the live provider path.

settlement requirement: not yet met. The mission settles only with deployed
staging proof of a from-weights first paint well under the ~49s baseline and
multiple grounded revisions (V2+) that track findings packets, one logical actor
per doc surviving a vmctl refresh as sleep/resume, an enforced per-actor budget,
doc-delete cancellation, updated verifier/tests/docs/heresy detectors, and a
RunAcceptanceRecord at staging-smoke-level (or higher).
