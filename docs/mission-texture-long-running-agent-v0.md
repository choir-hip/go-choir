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

Remaining audited value: T3-T8 remain open. The runtime still has no
park-and-wait primitive or cumulative per-actor budget; the tool loop is still
bounded by `maxToolLoopIterations=200` (`internal/runtime/toolloop.go:203-209`).
The separate cold wake/reconcile scaffolding remains
(`internal/runtime/texture_controller.go:24-90`), so this construct proves
multi-revision capability inside a run but does not yet make
`texture:<docID>` a single parked resident actor. Restart still passivates
Texture revision runs by marking pending mutations stale
(`internal/runtime/runtime.go:1261-1302`), document deletion still deletes the
document without cancelling the actor (`internal/runtime/texture.go:1048-1060`),
and the workflow verifier still checks revision causality without proving the
new one-run-to-many-revisions lifecycle end to end
(`internal/runtime/texture_workflow_verifier.go:527-593`).

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
  rejected and retried before storage; staging must prove the live provider
  recovers with a useful V1 without regressing substantive V2.
- C3 active: "one run per agent" is more minimal as a model but requires a real
  park-and-wait + budget; without them a long run either idles on billed calls or
  hits the 200-iteration ceiling. The park-and-wait must be role-uniform.
- C4 active: passivation means even "one run" is one logical actor across
  physical runs; sleep/resume from the run-memory snapshot is the continuity
  mechanism, not an immortal process.
- C5 open edge: cost/runaway and cancellation are the top risks of a long-lived
  actor; the budget kill-switch and the doc-delete->cancel gap must close before
  or with the lifecycle change.
- C6 open edge: collapsing many revisions into one run must not muddy
  trajectory/work-item attribution or the per-revision supervision narrative;
  verifier and Trace projection must stay legible at N:1.
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
  the old first-findings window. The current local guard moves that branch from
  "stored no-op V1" to "retry exact patch_texture or fail without appagent V1";
  staging must decide whether the live provider recovers quickly.

next move: finish local verification for the initial model-prior no-op guard,
commit and push it, monitor CI/deploy identity, then rerun the deployed cadence
probe. Staging must show a useful model-prior V1 well under the old ~49s window
and a substantive V2, or expose a precise retry-exhaustion branch without
storing a prompt-copy V1.

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
draft locally; live provider recovery is still unproven.

settlement requirement: not yet met. The mission settles only with deployed
staging proof of a from-weights first paint well under the ~49s baseline and
multiple grounded revisions (V2+) that track findings packets, one logical actor
per doc surviving a vmctl refresh as sleep/resume, an enforced per-actor budget,
doc-delete cancellation, updated verifier/tests/docs/heresy detectors, and a
RunAcceptanceRecord at staging-smoke-level (or higher).
