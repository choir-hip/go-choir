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

## Required Product Behavior

After the hard cutover, a current-events prompt such as
`What's new in the world?` must show:

1. Prompt bar submission creates or opens a Texture artifact.
2. The Texture surface shows an owner-legible intake state. Either `V0` contains
   the prompt or the UI shows the prompt as a first-class intake/instruction
   surface; a blank body alone is not acceptable.
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
- no blank-only owner intake unless the prompt is visible in a separate Texture
  intake surface;
- first Texture run evidence showing the full Texture tool affordance was
  available, not exact-filtered to only `patch_texture`;
- downstream researcher and/or super run evidence created by Texture choice,
  not manually seeded by the test;
- worker evidence packet delivered back to the Texture context;
- Texture `V2` or later revision created from that evidence;
- pending mutation/run state cleared or honestly failed with an owner-visible
  blocker;
- no direct prompt-bar-to-super route before Texture;
- `nix develop -c scripts/go-test-runtime-shards` or a documented narrower
  equivalent while shaping, full CI, Node B deploy, staging health identity,
  and deployed acceptance proof.

## Suggested Goal String

```text
/goal Use Parallax on docs/mission-texture-product-loop-recovery-v0.md. Continue the no-compatibility Texture hard cutover plus product-loop recovery mission from the audited Claude handoff. Current status is open_handoff with V=9. Local commits 05162395 and 02215cf7 performed a broad live-code rename/delete pass, but the handoff is not clean: internal/runtime focused tests fail because texture_prompt_unit_test.go still asserts exact first-turn tools after runtime.go changed initialTextureToolChoice to "required"; internal/platform tests fail because blind rename made duplicate platform_texture table definitions and tests now treat canonical Texture labels/classes as retired; the docs subagent changed current docs but those edits are uncommitted; no CI/deploy/staging proof exists. Owner override remains binding: no compatibility shims, no old route aliases, no dual-write tables, no app-id normalization, no legacy tool aliases, no actor/profile bridges, no rollback protection, and no preserving fake/test/demo old-ontology state. First repair the cutover fallout: make the tree compile and tests pass without reintroducing retired-name compatibility; delete duplicate schema blocks; repair corrupted tests so they assert canonical Texture, not its deletion; finish the first-turn tool-choice test update or revise the runtime change if evidence shows "required" is wrong; handle detector literals without allowing retired-name live surface. Then finish behavior repair: owner-legible Texture intake, full first-turn Texture affordance, Texture-created researcher/super work when needed, worker evidence back into the same Texture context, and V2+ from that evidence. Acceptance requires residue proof using git grep, focused tests, scripts/go-test-runtime-shards, frontend build where touched, full CI/deploy identity, and deployed browser/product-path proof on https://choir.news. Fix forward.
```

## Parallax State

status: settled

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

variant (ranking function) V: current V=0; last ΔV: 5 -> 0 after landing the
repair, forcing staging deploy, and proving the deployed prompt-bar -> conductor
-> Texture -> researcher -> Texture loop on `https://choir.news`. The product
trajectory completed with Texture-first routing, owner-legible intake, worker
evidence back into the same Texture context, and V3 from that evidence. The
only remaining axes are explicitly residual: run-acceptance synthesis does not
yet elevate Texture-created researcher evidence to an accepted level, and an
optional super/worker proof leased a worker but did not observe terminal worker
delegation before trace completion.

1. completed: document the no-compatibility owner override and
   cutover-before-repair order;
2. mostly completed for non-doc live code/tests: delete/rename retired V-name
   files, symbols, profiles, task types, metadata keys, event names, Trace
   labels, tests, prompts, routes, app ids, and tools from live surfaces.
   Current high-read docs still need final residue classification;
3. locally supported for touched surfaces: delete old compatibility aliases,
   normalization paths, dual writes, legacy table preservation, and rollback
   bridges. The duplicate `platform_texture_*` DDL and corrupted platform tests
   were repaired as deletion fallout, not preserved as compatibility;
4. partially supported: `git grep -n -i vtext -- ':!docs/**'` is clean, including
   `cmd/doccheck`; full docs residue proof/classification remains open;
5. locally supported, not deployed: decide and encode owner-legible intake
   behavior for prompt-bar `V0`. V0 remains intentionally blank canonical
   document state, while `intake_prompt`/`data-texture-intake` expose the
   original owner prompt as first-class Texture intake;
6. locally supported, not deployed: remove the first-turn exact-tool-choice
   imprisonment that limits Texture to only `patch_texture`. `required` is now
   covered by focused tests that also assert the first Texture turn sees
   `patch_texture`, `record_texture_decision`, `spawn_agent`, and
   `request_super_execution`;
7. locally supported, not deployed: prove a first Texture model response can
   execute both a write and a worker request when the model chooses both.
   `TestInitialTextureRunCanWriteAndSpawnResearcherInSameFirstTurn` covers a
   prompt-bar-created Texture first turn that calls `patch_texture` and
   `spawn_agent`;
8. locally supported, not deployed: product-path tests now prove
   Texture-created researcher evidence and persistent-super evidence attach
   back to the same Texture context, wake Texture, and produce V2+ appagent
   revisions from that evidence. The repair also fixed revision metadata so a
   scheduled worker wake records the consumed evidence window even when the
   durable checkpoint has already caught up;
9. completed: manual-seeding witnesses have product-path replacements for
   researcher and super evidence, `scripts/go-test-runtime-shards` passed
   locally, behavior commit `689267dff0cd561395dfb99a4285256716e35740` passed
   CI/deploy, and staging health reported the deployed SHA;
10. completed: deployed browser/product-path acceptance proved no old live
   non-doc ontology, prompt-bar Texture intake, no direct prompt-bar-to-super
   route before Texture, and V2+/V3 from downstream researcher evidence.

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
is required or desired. Final deployed evidence is recorded in
`docs/mission-texture-product-loop-recovery-v0.ledger.md` under
`2026-06-16 - Deployed Texture Product Loop Recovered (V 5 -> 0)`.

heresy delta: discovered: compatibility posture let a broken artifact loop hide
behind rename progress. Repaired locally: first-turn exact tool imprisonment,
blank-only prompt-bar intake, duplicate Texture schema/test corruption, corrupted
model-policy/Universal Wire cutover assertions, and worker evidence metadata
that could lose the consumed packet proof after checkpoint catch-up. Introduced:
none accepted.
Candidate repair must avoid introducing semantic workflow gates, direct
prompt-bar-to-super routing, or compatibility shims.

position / live conjectures / open edges:

- C1 active: the retired ontology itself is now a causal contributor. Removing
  it first should reduce agent confusion and force tests/proofs to name the
  actual object.
- C2 active: blank prompt-bar `V0` is not a storage bug but a product/spec
  mismatch introduced by the current prompt-bar intake design. It must become
  owner-legible either as canonical intake content or as a first-class Texture
  intake surface.
- C3 active: the decisive first-turn behavior failure is exact tool-choice
  filtering. A live Texture turn with only `patch_texture` cannot delegate,
  regardless of prompt doctrine.
- C4 active: terminal write semantics may be correct only if the model can
  make all needed calls in the same response. Prove multi-tool write plus
  worker request before adding new tools or workflow gates.
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
- C11 supported and deployed: owner-legible prompt-bar intake does not require
  stuffing the raw prompt into canonical document prose. The API exposes
  `intake_prompt`, the frontend renders it separately from the editor body, and
  staging UI proof rendered `[data-texture-intake]`.
- C12 supported and deployed locally-by-test/staging-by-effect: broad
  `required` first-turn tool choice allows the
  model to emit both a write and researcher spawn in the same first Texture
  response, and both effects persist on the prompt-bar trajectory.
- C13 supported and deployed: Texture-created researcher and persistent-super runs
  can return `update_coagent` evidence to `texture:<docID>` on the same
  prompt-bar trajectory; Texture wakes and writes V2+ revisions from those
  packets without manual seeding. Staging proof reached V3 from researcher
  evidence.
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

next move: promote settlement evidence outward only where useful, then start a
separate mission for the residual acceptance-model/live-worker axis. Do not
reopen compatibility shims or old route aliases as a workaround.

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

settlement: settled for this mission. The live non-doc codebase has no
old-ontology runtime surfaces, pushed behavior commit
`689267dff0cd561395dfb99a4285256716e35740` passed CI and deployed to Node B,
staging health reports that commit, and deployed browser/product-path
acceptance proved prompt-bar Texture intake, Texture-first routing,
Texture-created downstream researcher evidence, and V2+/V3 revisions from that
evidence. Do not claim export-level, promotion-level, or continuation-level
acceptance from the synthesized records; those remain separate residual axes.
