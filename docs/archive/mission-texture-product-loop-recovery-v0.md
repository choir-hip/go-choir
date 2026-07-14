# Mission: Texture Hard Cutover And Product Loop Recovery v0

## Summary

Texture's core product loop is broken in staging, and the failed cutover mission
made the break harder to see by preserving the retired V-name through live code,
schemas, APIs, prompts, tests, and docs. That compatibility posture is now
rejected.

This mission is a hard ontology cutover first and a behavior repair second. The
implementation must delete or rename the retired V-name across the live
codebase before polishing the product loop. No compatibility shims, no dual
write paths, no legacy route aliases, no legacy table preservation, no rollback
protection, and no "temporary" old-name affordances are authorized. We do not
have real users yet. Fix forward.

Settlement still requires the product loop to work: prompt bar to conductor to
Texture intake, Texture-owned first revision, Texture-chosen researcher/super
work when needed, worker evidence attached back to the same Texture context,
and V2+ from that evidence.

The 2026-06-16 deployed settlement claim is revoked. Manual owner QA after that
claim showed the product still violates the intended Texture semantics: prompt
bar input is rendered as separate prompt chrome instead of `V0`, `V1` can be a
generic one-shot response with no later researcher/super evidence, and live
runtime/tests still carry parent/child control vocabulary. The mission is
reopened as a documentation-first handoff.

## Problem

The previous hard-cutover mission went off the rails because it treated
compatibility work as mission progress. It changed visible labels, routes,
storage names, and evidence payload names while leaving the old ontology as the
live substrate. That created two failures:

- the codebase still taught the wrong object, so agents kept repairing around
  the old abstraction instead of deleting it;
- acceptance proved shallow route/app opening while the actual artifact loop
  stayed broken.

Manual QA on 2026-06-16 falsified the real product path. A prompt such as
`What's new in the world` opened Texture and completed a run, but the visible
intake was blank, the first revision was a thin working note, and no later
revision arrived from researcher or super evidence.

The corrective lesson is not "fix the loop while preserving old names." The
lesson is that compatibility let the mission lie to itself. The repair must
remove the old ontology before it repairs behavior, so tests, prompts, APIs,
Trace, storage, and UI all describe the same object.

## Owner Override

The owner explicitly authorizes a no-compatibility cutover.

- Do not preserve old route aliases.
- Do not dual-write old and new table families.
- Do not normalize old app ids into new app ids for product continuity.
- Do not keep old tool names as aliases.
- Do not keep old actor/profile names as compatibility bridges.
- Do not write migration code whose purpose is to let old binaries continue
  reading new state.
- Do not retain rollback-oriented runtime paths.
- Do not protect fake/test/demo state created under the old ontology.
- Do not let rollback planning weaken the cutover. Source control history is
  the archive; runtime compatibility is not.

If the cutover breaks stale local or staging state, delete that state or rebuild
it under Texture. If an implementation agent believes a compatibility shim is
unavoidable, it must stop and report the blocker instead of landing the shim.

## Hard-Cutover Scope

The first implementation phase is deletion/rename. It must remove the retired
V-name from all non-archival live surfaces:

- Go package/file/function/type/constant names for the artifact subsystem;
- runtime agent profile, actor id, task type, metadata keys, event names, Trace
  labels, and run acceptance labels;
- Dolt database/table/index names and store APIs for Texture documents,
  revisions, aliases, mutations, controller checkpoints, and decision notes;
- frontend component/file/module names, app ids, route names, data attributes,
  CSS/test selectors, public API clients, and UI labels;
- prompt defaults, tool names, tool descriptions, tool argument schemas, and
  tests;
- browser-public and platform-internal routes;
- docs checker rules and current high-read docs;
- mission graph current nodes and source paths, except where an intentionally
  archived historical document is left as evidence.

Historical material should be deleted rather than converted when it no longer
helps current operation. The only allowed current references to the retired
V-name are in explicitly historical/background evidence or in the cutover
mission's own residue inventory while the occurrence is marked as a deletion
target. New live code must not add any.

## Current Root-Cause Evidence

Read-only review before this mission found a coherent causal chain for the
behavior failure:

- prompt-bar intake `V0` is intentionally blank in current runtime code. The
  prompt is stored as metadata/instruction, not visible Texture content.
- initial Texture runs are exact-forced to `patch_texture` unless a narrow
  decision-note path applies.
- exact tool choice filters the available tool definitions to only that one
  tool, so the first Texture turn cannot call `spawn_agent`,
  `record_texture_decision`, or `request_super_execution`.
- successful `patch_texture` is terminal for the run, so a first working
  revision can complete the run before any researcher/super request exists.
- current tests manually seed researcher/super behavior after Texture opens,
  so they can pass while the deployed product path fails.
- the previous proof accepted `submitted` and `texture_opened`, which is not
  enough to prove the artifact loop.

The observed staging log shape from the owner's QA window matched this chain:
the live gateway request carried `tools=1` and
`tool_choice=function:patch_texture`, then the run completed.

Read-only review after the claimed settlement found a sharper causal chain:

- prompt-bar-created Texture `V0` is deliberately blank when
  `input_source=prompt_bar`; the user prompt is stored as `seed_prompt` metadata
  plus `prompt_bar_instruction_revision`;
- the Texture API derives `intake_prompt` from that metadata and the frontend
  renders a separate prompt band, so the prompt is product chrome rather than a
  canonical Texture version;
- `buildAgentRevisionRequest` tells Texture to treat prompt-bar `V0` as
  intentionally blank canonical document state and to use the prompt as
  instruction/context, not as canonical prose;
- tests assert the blank `V0` and prompt-band behavior, so the test suite now
  protects the wrong spec;
- `patch_texture` and `rewrite_texture` are terminal tool successes for Texture
  runs. A first working revision can end the run before Texture makes a later
  coagent decision;
- the same-response write-plus-researcher test is too weak. It proves only that
  a model can call `patch_texture` and `spawn_agent` in one batch, not that a
  normal `patch_texture` result leaves Texture able to continue to researcher,
  super, `record_texture_decision`, or an honest blocker;
- Texture currently receives the broad evidence tool bundle
  (`save_evidence`, `read_evidence`, `list_evidence`,
  `get_run_memory_entry`, and `verify_model_capability`) even though researcher
  should own ordinary evidence gathering and model-capability diagnostics do not
  belong in Texture's authoring affordance;
- live runtime and tests still normalize parent/child control through
  `StartChildRun`, `ParentRunID`, `parent_loop_id`, `parent_id`,
  child-run list/count helpers, parent/child channel helpers, researcher
  parent-target fallback, Trace/verifier inference, and cancellation/status
  vocabulary.

## Required Product Behavior

After the hard cutover, a current-events prompt such as
`What's new in the world?` must show:

1. Prompt bar submission creates or opens a Texture artifact.
2. `V0` contains the exact owner prompt as canonical Texture content. The prompt
   is not hidden metadata, an intake band, or a separate prompt chrome surface.
   `seed_prompt` may remain as provenance only; it must not be the product
   display mechanism.
3. The first Texture agent turn can choose among its real affordances, including
   `patch_texture`, `spawn_agent`, `record_texture_decision`, and
   `request_super_execution`.
4. Texture writes an initial working revision when useful, without claiming
   unsupported current facts.
5. Texture opens researcher and/or super work when the request requires external
   evidence, execution, generated artifacts, verification, or candidate work.
6. Worker evidence attaches back to the same Texture/artifact context.
7. Texture wakes from that evidence and writes at least one later revision that
   incorporates, cites, or honestly rejects the worker evidence.
8. Chyron and Trace distinguish "Texture wrote a working revision" from "the
   whole artifact loop is complete."
9. Parent/child runtime control is absent. Coagents communicate through
   trajectory, channel, durable work item, requester/provenance metadata, and
   addressed updates. No live API, schema, helper, prompt, or acceptance path
   treats one run as a parent that owns or cancels child runs.

## Not This Mission

- Do not resume M3 or source/news work before this settles.
- Do not add compatibility, alias, normalization, legacy migration, or rollback
  bridges.
- Do not settle with local tests, route probes, API-only submission checks, or
  evidence that only proves Texture opened.
- Do not fix behavior by hardcoding semantic classifiers, prompt regexes, or a
  deterministic role sequence in runtime.
- Do not route ordinary prompt-bar input directly to super. Texture is the
  artifact control plane; super is downstream of Texture when Texture requests
  execution.
- Do not make researcher/super mandatory for every prompt. The invariant is
  that Texture has the full affordance and chooses well, not that every prompt
  follows the same choreography.
- Do not preserve a prompt-band compromise. The owner prompt is `V0`.
- Do not retain parent/child compatibility language or control fields as a
  convenience. If historical provenance is needed, use `requested_by_*` or a
  clearly provenance-only successor, never control-facing parent/child terms.

## Acceptance

Settlement requires both a hard-cutover residue proof and a deployed
browser/product-path proof on `https://choir.news` using an authenticated owner
session and public product APIs only.

Minimum hard-cutover proof:

- no retired V-name occurrences in non-archival live code, prompts, frontend,
  tests, scripts, route declarations, schema names, tool names, app ids, Trace
  labels, or current high-read docs;
- any remaining occurrences are explicitly historical/background evidence or
  deletion-target inventory inside this mission family;
- no old-name route alias, tool alias, app-id normalization, table dual-write,
  actor/profile bridge, or rollback compatibility path remains;
- fake/test/demo state made under the old ontology is deleted or rebuilt under
  Texture rather than migrated for compatibility.

Minimum product-loop proof:

- fresh prompt-bar submission for a current/research-requiring prompt;
- conductor route evidence showing Texture before any super handoff;
- Texture document id, visible UI screenshot or DOM proof, and revision list;
- `V0` content exactly matches the owner prompt for prompt-bar-created Texture;
- no blank-only owner intake;
- first Texture run evidence showing the full Texture tool affordance was
  available, not exact-filtered to only `patch_texture`;
- downstream researcher and/or super run evidence created by Texture choice,
  not manually seeded by the test;
- worker evidence packet delivered back to the Texture context;
- Texture `V2` or later revision created from that evidence;
- pending mutation/run state cleared or honestly failed with an owner-visible
  blocker;
- no direct prompt-bar-to-super route before Texture;
- no prompt-band / `intake_prompt` product surface;
- no `prompt_bar_instruction_revision` branch in Texture revision prompting;
- `patch_texture` and `rewrite_texture` are not terminal Texture run tools;
- Texture tool inventory excludes model-capability diagnostics and any
  researcher-owned evidence tool not justified as a Texture affordance;
- no live parent/child control API/schema/helper/prompt/test residue remains
  outside explicitly archived historical docs;
- `nix develop -c scripts/go-test-runtime-shards` or a documented narrower
  equivalent while shaping, full CI, Node B deploy, staging health identity,
  and deployed acceptance proof.

## Suggested Goal String

```text
/goal Use Parallax on docs/mission-texture-product-loop-recovery-v0.md. Mission focus this pass is the Texture revision-cadence defect confirmed by the 2026-06-17 read-only measurement recorded in docs/mission-texture-product-loop-recovery-v0.ledger.md. Deployed staging 2b4c4a3c, prompt "What's going on with Anthropic and the US government?" (submission f0c321a3-e7ff-4bcb-adb9-e4637b87ccb1, doc 15d89744-901a-4684-a28c-11e7c4cd5451): V0 prompt is instant, but the first appagent revision (V1, ~2.4k chars) lands at +60s with nothing shown in between, and the loop stops at one revision even though 4 update_coagent findings packets and 2 researcher spawns arrived. Root cause is cadence in internal/runtime/texture_controller.go, not missing research and not the prompts (textureprompts already mandate deepening). First reconcile the contradiction that the bottom of this paradoc still says "settled" (commit 689267df claiming V2+/V3) while the top is open_handoff and live behavior on 2b4c4a3c is V1-only: either 2b4c4a3c regressed from 689267df or the prior V2+ proof was non-representative; document which before fixing. Owner-chosen fix is runtime-driven cadence, treating interim-state delivery and a revision-cadence floor as mechanical invariants (legitimately runtime, not semantic role choreography), while "what to research" stays model-driven: (1) change scheduleTextureWorkerWake from resetting-trailing debounce to leading + max-interval flush so the first revision lands right after the first findings packet (~target under 20s), not after all research; (2) on resident Texture-run completion, re-wake reconcileTextureAgentWake when pending worker updates remain so multiple findings packets become multiple revisions (one canonical write per run preserved); (3) keep the loop alive to deepen while researchers/super keep delivering, with a soft revision/budget cap, letting the model decide when marginal returns diminish via record_texture_decision. This same wake path already accepts super/cosuper messages, so it must also serve as the long-running-agent supervision cadence (a fresh revision roughly every interval). Do not encode semantic decision trees, prompt classifiers, or fixed role choreography in the cadence; do not add compatibility shims. Mutation class red: protected surfaces are Texture canonical writes, the wake/debounce cadence, worker-update consumption/pending bookkeeping, and Trace. Rollback is revert of the cadence change; old V=0 settlement is already revoked. Verify with focused internal/runtime tests for leading-flush + re-wake-on-pending, scripts/go-test-runtime-shards, then commit -> push origin main -> CI -> Node B deploy identity -> deployed proof on https://choir.news using scripts/texture_revision_cadence_probe.mjs showing first-paint well under the 60s baseline and multiple appagent revisions (V2+) that track the findings packets. Record receipts and a RunAcceptanceRecord at staging-smoke-level. Earlier reopen requirements remain background context below.
```

## Prior Goal String (earlier reopen, background context)

```text
/goal Use Parallax on docs/mission-texture-product-loop-recovery-v0.md. Reopen the Texture product-loop recovery mission from the 2026-06-16 manual QA falsification. Treat the prior V=0 settlement as revoked: the deployed product still renders the owner prompt as separate `PROMPT`/`intake_prompt` chrome, leaves prompt-bar-created `V0` blank, can stop at a generic `V1` without Texture-created researcher/super evidence, grants Texture an overbroad evidence/model-diagnostic tool bundle, and still carries parent/child run control surfaces. Owner override remains binding: hard cutover, no compatibility shims, no rollback protection, no old route/tool/app/schema aliases, no preserving fake/test/demo old-ontology state, and no parent/child control semantics. First document the problem, then delete/fix forward. Required repairs: prompt-bar user prompt is exact canonical Texture `V0`; delete `prompt_bar_instruction_revision`, `intake_prompt`, and prompt-band UI/tests; remove `patch_texture` and `rewrite_texture` from Texture terminal-tool successes while preserving one canonical write per run; split Texture's tool inventory so researcher-owned evidence and `verify_model_capability` are not exposed to Texture unless separately justified; replace parent/child live control APIs/schema/helpers/prompts/tests with trajectory/channel/work-item/requester-provenance coagent semantics; update tests to prove patch-then-delegate and same-turn write-plus-delegate paths, Texture-created researcher/super evidence returning to the same Texture, and V2+ from that evidence; verify with focused tests, `scripts/go-test-runtime-shards`, frontend build if touched, grep residue proof, CI/deploy identity, and deployed browser/product-path proof on https://choir.news. Fix forward and stop rather than adding compatibility.
```

## Parallax State

status: open_handoff

mission conjecture: if the live codebase first deletes the retired ontology
without compatibility shims, and only then repairs/proves the deployed
prompt-bar Texture loop, then Choir regains a coherent artifact control plane
instead of carrying a broken behavior inside a split vocabulary.

deeper goal (G): restore Texture as Choir's central versioned, transclusive
artifact control plane: a standing surface that directs results with autonomy
and facilitates compounding learnings, rather than a one-shot draft pane or a
renamed legacy subsystem.

witness/spec (A/S): a hard live-ontology cutover plus runtime/product repair.
The witness is a codebase whose current surfaces say Texture, not the retired
V-name, and a deployed artifact loop that starts from a real owner prompt,
keeps Texture first, allows Texture to choose its full tool affordance, receives
downstream worker evidence, and produces a later revision from that evidence.

invariants / qualities / domain ramp (I/Q/D):

- I: no compatibility shims. Old-name routes, app ids, tool aliases, table
  families, actor/profile bridges, and rollback-preservation paths are
  forbidden.
- I: Texture owns canonical artifact meaning and versions. Conductor routes
  exogenous input into Texture; it does not author the artifact.
- I: super is never the direct ingress target for ordinary prompt-bar or source
  prompts. Super may run only downstream of Texture through explicit Texture
  affordance.
- I: runtime may protect mechanical invariants, but must not encode semantic
  decision trees, prompt regex classifiers, or fixed role choreography.
- I: Texture delegation remains agentic. Texture may write, ask researcher, ask
  super, ask both, ask neither, wait, or report a blocker within authority.
- I: the owner must be able to see what prompt/intake the Texture is working
  from. A blank-only `V0` is not acceptable product evidence.
- Q: acceptance must prove the product path, not manually invoke worker tools
  after Texture opens.
- Q: Chyron/Trace wording must not imply completed mission/workflow when only a
  first Texture revision completed.
- D: first cut live code/test/docs surfaces to Texture under focused local
  verification; then run staging browser proof against `https://choir.news`
  with a real prompt-bar submission and public product APIs.

variant (ranking function) V: current V=6 after the 2026-06-17 cadence
measurement reopened a dimension the prior reopen did not name. The old V=0 claim
proved some route/evidence behavior but accepted the wrong prompt-bar ontology:
blank canonical `V0` plus separate prompt chrome. The active variant now descends
only when the exact owner prompt is canonical `V0`, prompt-band/intake metadata
is deleted, write tools no longer terminate Texture before delegation/decision,
Texture's tool inventory is trimmed to its real authority, parent/child control
semantics are removed from live runtime surfaces, the deployed loop emits an
eager first revision (first paint well under the 60s baseline) and multiple
appagent revisions (V2+) that track findings packets rather than collapsing them
into one write, and deployed browser/product-path proof succeeds.

1. completed: document the no-compatibility owner override and
   cutover-before-repair order;
2. completed locally: prompt-bar `V0` semantics. Owner prompt is exact canonical
   `V0`; blank prompt-bar revisions, `prompt_bar_instruction_revision`,
   `intake_prompt`, and prompt-band UI/tests are deleted;
3. completed locally: Texture write-loop semantics. `patch_texture` and
   `rewrite_texture` store one canonical revision without terminating the Texture
   run before delegation/decision/handoff;
4. completed locally: Texture tool inventory. Evidence/model-diagnostic tools
   are split out; Texture keeps run-memory retrieval only;
5. completed locally: parent/child hard cutover. `StartCoagentRun`,
   `RequestedByRunID`, requester provenance metadata/API fields, and updated
   tests replace parent/child control helpers and vocabulary;
6. completed locally: non-doc retired V-name residue remains clean by `git grep`;
7. invalidated: prior local/deployed proof that accepted a separate prompt
   intake surface no longer satisfies this mission;
8. completed locally: product-path tests cover same-turn write-plus-delegate,
   patch-then-delegate/decision continuation, and Texture-created
   researcher/super evidence returning to the same Texture with V2+ revisions;
9. required: deployed browser/product-path acceptance on `https://choir.news`
   must show `V0` prompt content, no prompt band, no direct prompt-bar-to-super
   edge, worker evidence from Texture choice, and V2+ from that evidence.

budget: one urgent red-surface fix-forward mission before M3 resumes. Broad
rename/deletion is authorized because the owner explicitly values coherent
ontology over compatibility and there are no real users yet.

authority / bounds: current document creation is `green`. Execution will be
`red`, touching prompt-bar route materialization, Texture run/tool choice,
Texture write tools, worker delegation, Trace/diagnosis, Chyron wording,
revision UI/API, schemas/storage, frontend app identity, routes, tests, and
staging deployment. Problem Documentation First still applies to newly observed
behavioral failures, but it must not become an excuse to reintroduce old-name
compatibility.

mutation class / protected surfaces: planned execution mutation class `red`.
Protected surfaces: Texture canonical writes, first-turn tool affordance,
worker request authority, super boundary, Trace/evidence, prompt-bar API,
desktop Texture UI state, Chyron completion semantics, storage/schema identity,
app identity, run acceptance, and deployment routing.

evidence packet: residue proof, focused local tests for renamed/deleted
surfaces, staging screenshot/DOM proof, prompt-bar submission id, conductor run
id, Texture doc id, Texture run id, tool definition/tool choice evidence,
worker run id(s), worker evidence payload, revision list proving V2+ from that
evidence, Trace/diagnosis proof of no super-before-Texture, CI run, deploy run,
staging health commit, and residual risk note. No rollback compatibility proof
is required or desired. The previously claimed deployed evidence is recorded in
`docs/mission-texture-product-loop-recovery-v0.ledger.md` under
`2026-06-16 - Deployed Texture Product Loop Recovered (V 5 -> 0)`, but that
settlement is now revoked because it accepted prompt-band/blank-`V0` behavior.

heresy delta: discovered: compatibility posture let a broken artifact loop hide
behind rename progress; prompt-band/blank-`V0` behavior was accepted as repair
but is now rejected; Texture has an overbroad tool bundle; live parent/child
control semantics remain. Repaired locally in prior work: some first-turn exact
tool imprisonment and duplicate Texture schema/test corruption. Introduced or
preserved: prompt-band/blank-`V0` tests, terminal Texture write semantics, broad
Texture evidence/model-diagnostic tools, and parent/child control residue.
Candidate repair must avoid introducing semantic workflow gates, direct
prompt-bar-to-super routing, or compatibility shims.

position / live conjectures / open edges:

- C1 active: the retired ontology itself is now a causal contributor. Removing
  it first should reduce agent confusion and force tests/proofs to name the
  actual object.
- C2 active: blank prompt-bar `V0` is a product/spec violation. The owner prompt
  is canonical Texture `V0`. A separate first-class intake surface was tried and
  rejected by manual QA because it splits the artifact between document state
  and chrome.
- C3 active: the decisive first-turn behavior failure is exact tool-choice
  filtering. A live Texture turn with only `patch_texture` cannot delegate,
  regardless of prompt doctrine.
- C4 active: terminal write semantics are suspect. `patch_texture` and
  `rewrite_texture` should not end a Texture run merely because a write
  succeeded. The run must be able to continue to coagent delegation, super
  request, decision note, email handoff, or intentional end after the write.
- C5 active: current acceptance tests are false witnesses because they manually
  start researcher/super work. Replace them with product-path proof.
- C6 active: Chyron "completed a run" may be technically true but product
  misleading. It should not be the user's only cue for an unfinished artifact
  loop.
- C7 supported: Claude Code session
  `~/.claude/projects/-Users-wiz-go-choir/23c60ed4-6440-4d91-9165-4ebba0d56995.jsonl`
  executed a broad cutover and then hit a rate limit before finishing the
  behavior/test repair. Do not trust the ledger's prior "landed" claim without
  rerunning tests.
- C8 supported locally: blind rename corrupted platform tests and schema. The
  duplicate `platform_texture_*` table definitions were deleted and tests now
  assert canonical Texture labels/classes.
- C9 supported locally: detector behavior is preserved while detector source and
  tests no longer contain grep-visible retired-name literals. `cmd/doccheck`
  still passes.
- C10 settled for live surfaces: non-doc residue proof is clean. Historical
  docs that mention retired names remain classified as historical/background
  evidence, not live runtime surface.
- C11 revoked: owner-legible prompt-bar intake via `intake_prompt` and
  `[data-texture-intake]` is not acceptable. Delete the prompt-band design and
  its tests. `seed_prompt` may remain only as provenance, not product display.
- C12 partially supported: broad
  `required` first-turn tool choice allows the
  model to emit both a write and researcher spawn in the same first Texture
  response, and both effects persist on the prompt-bar trajectory. This is not
  enough; patch-then-delegate continuation must also work.
- C13 partially supported: Texture-created researcher and persistent-super runs
  can return `update_coagent` evidence to `texture:<docID>` on the same
  prompt-bar trajectory; Texture wakes and writes V2+ revisions from those
  packets without manual seeding in prior proof. This must be reproved after
  deleting prompt-band and terminal-write semantics.
- C14 supported locally: worker evidence revision metadata now summarizes the
  scheduled evidence window from the previous Texture head when the stored
  controller checkpoint has already reached the scheduled message. This repairs
  a proof gap without weakening worker-message eligibility or treating
  Texture-to-super assignments as Texture evidence.
- C15 supported and deployed: full runtime shards pass after repairing corrupted
  hard-cutover tests that rejected canonical Texture fields/role keys.
- C16 residual: `RunAcceptanceRecord` synthesis is not yet aligned with
  Texture-created researcher evidence. The researcher trajectory synthesized
  `runacc-e27492fe9a16fc636550` as `staging-smoke-level/blocked` despite
  completed V3 product proof. A separate super/worker probe synthesized
  `runacc-a2bd46027d5d836cb06e`, passed `submitted`, `texture_opened`,
  `super_requested`, and `worker_leased`, but remained blocked at
  `worker_delegated` with the worker still observed as running.

- C17 active (2026-06-17 measurement): the deployed loop on `2b4c4a3c` is
  V1-only with ~60s first paint. A read-only product-path probe
  (`scripts/texture_revision_cadence_probe.mjs`) showed V0 at +0.3s, the first
  appagent revision at +60.1s, and exactly one appagent revision despite 4
  `update_coagent` findings packets and 2 researcher spawns. The cadence
  collapses many evidence packets into one write; `reconcileTextureAgentWake`
  returns early while a loop is resident and the trajectory then completes with
  no re-wake. This is a cadence defect, not missing research or weak prompts.
- C18 active: revision cadence is the agent-supervision control plane. The same
  wake path already accepts super/cosuper messages, so an eager + max-interval
  cadence simultaneously fixes interim-results UX, deep-research depth, and
  owner supervision of long-running super/coding-agent trajectories. The fix is
  runtime-driven (mechanical interim-state delivery) while research choice stays
  model-driven.
- C19 contradiction to reconcile: the settlement block below claims this mission
  is settled on `689267df` with V2+/V3 proved, but live behavior on `2b4c4a3c`
  is V1-only. Either `2b4c4a3c` regressed from `689267df` or the prior V2+ proof
  was non-representative (manual-seeded or a prompt that happened to trigger
  multiple wakes). Resolve which before landing the cadence fix; the bottom
  settlement is superseded by the open_handoff status and the 2026-06-17
  evidence.

next move: implement the runtime-driven cadence fix in
`internal/runtime/texture_controller.go` (leading + max-interval flush; re-wake
on resident-run completion when pending updates remain; deepening re-wake with a
soft cap), reprove on staging with the cadence probe, and only then promote
settlement evidence outward. Do not reopen compatibility shims or old route
aliases as a workaround.

ledger file: docs/mission-texture-product-loop-recovery-v0.ledger.md

version / lineage: created 2026-06-16 as a corrective successor to the M3.4
first-draft regression mission and as a blocker in front of the failed Texture
hard-cutover mission. Updated after Claude Code session
`23c60ed4-6440-4d91-9165-4ebba0d56995` was reviewed and found incomplete.
Owner override remains promoted: no compatibility, no rollback protection, fix
forward.

learning state: the central learning is retained here until promoted outward:
Texture mission acceptance must prove both ontology coherence and the artifact
loop. Compatibility progress is not progress.

settlement: SUPERSEDED by status open_handoff and the 2026-06-17 cadence
evidence (see C17-C19 and the ledger measurement receipts). The deployed loop on
`2b4c4a3c` is V1-only with ~60s first paint, which contradicts the V2+/V3 claim
in the note below. Treat the following as the prior (now-revoked) settlement
record, retained as evidence, not current truth.

prior settlement (revoked): settled for this mission. The live non-doc codebase has no
old-ontology runtime surfaces, pushed behavior commit
`689267dff0cd561395dfb99a4285256716e35740` passed CI and deployed to Node B,
staging health reports that commit, and deployed browser/product-path
acceptance proved prompt-bar Texture intake, Texture-first routing,
Texture-created downstream researcher evidence, and V2+/V3 revisions from that
evidence. Do not claim export-level, promotion-level, or continuation-level
acceptance from the synthesized records; those remain separate residual axes.
