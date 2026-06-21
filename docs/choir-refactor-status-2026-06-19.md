# Choir Refactor Status - 2026-06-19

## Executive Summary

Over the last three days, Choir moved from a partially renamed, run-centric
Texture surface toward the durable-actor and versioned-artifact architecture
described by the mission portfolio. The work was unusually dense: local history
shows 171 commits since 2026-06-16 00:00 through current `HEAD` `b26efec3`.

The current staging deployment is healthy at `b26efec3eb45b5e87d6f3b2320e503d110c4edc7`.
`https://choir.news/health` reports proxy and sandbox on that commit, `status=ok`,
`upstream=ok`, and `vmctl_routing=enabled`.

The main architectural state is:

- M3 lifecycle cutover is settled. The deployed vmctl refresh proof showed a
  researcher activation passivated on refresh, rewarm via
  `trajectory_work_item_sweep`, delivery back to Texture, no stranded worker
  updates, and a staging-smoke `RunAcceptanceRecord`.
- M4 continuation deletion is designed but not executed. It remains the next
  core spine deletion after M3.
- Texture hard cutover made major progress: live routes, app identity, labels,
  storage-facing language, prompts, and docs are mostly cut to Texture. Historical
  mission paths and explicitly allowed evidence still carry the old V-name.
- Texture as a versioned artifact advanced far beyond a design doc. D1-D7 have
  largely landed: typed provenance, revision hash chains, typed source evidence,
  source-aware citation validation, full-history publishing, reader lineage
  disclosure, and platform Ed25519 signatures for each published revision.
- Texture as one durable deep-research thread is not implemented yet. The
  previous overnight run landed useful mechanics, but owner review correctly
  rejected its run-centric spine and semantic role choreography. The current
  paradoc now targets one durable thread per Texture document, same-thread
  resume, event-driven delivery, always-deep research, and deletion of cold
  rewarm/classifier/guard scaffolding.
- Foreground model policy has been hard-cut to current available ChatGPT models:
  conductor and researcher use `gpt-5.4-mini` medium, Texture uses `gpt-5.5`
  low, and super uses `gpt-5.5` medium. The old `roles.vtext` policy section was
  removed from live generated policy files. Active runtimes were updated; 170
  hibernated runtimes were intentionally not bulk-mutated offline.

## Current Ground Truth

Evidence checked while writing this report:

- Repo `HEAD`: `b26efec3`
- Staging deployed commit: `b26efec3eb45b5e87d6f3b2320e503d110c4edc7`
- Staging health: proxy `ok`, upstream sandbox `ok`, vmctl routing `enabled`
- Recent successful CI/deploy for the model-policy cutover:
  CI `27832611177`, Docs `27832611154`, FlakeHub `27832611224`
- Pre-existing local worktree dirt before this report:
  `docs/mission-texture-versioned-artifact-v0.ledger.md`

This report is a documentation artifact only. It does not claim new product
acceptance beyond the receipts already recorded in the mission ledgers.

## Last Three Days

### 2026-06-16 - Texture hard cutover and product-loop recovery

The main work was the hard ontology cutover from the old V-name to Texture and
the recovery of the prompt-bar to Texture product loop.

Key changes:

- Renamed live API, frontend, prompt, trace, storage-facing, and publication
  surfaces toward Texture.
- Cut the canonical frontend app identity from the old app id to `texture`, with
  staging DOM proof that Texture opens under `data-app-id="texture"` while legacy
  URL intent still resolves correctly at the boundary.
- Reworked publication/platform route naming toward `/texture` control routes.
- Added checker pressure for retired-name residue instead of letting old naming
  silently re-enter active docs and source.
- Recovered the product loop after the first-draft regression, including the
  prompt-as-V0 shape and the coagent cutover path.
- Added prompt grounding details such as `prompt_unix_ts` and current snapshot
  framing so current-events prompts do not pretend stale memory is live evidence.

Important receipts:

- `05162395` completed the hard ontology cutover over live surfaces.
- Product-loop recovery commits included the problem record, route/product-loop
  fixes, live-search QA auth, requested-by-run migration, progress-banner
  repair, and prompt grounding.

Net effect: Texture became the product object in current surfaces, not a label
on top of the old object. The remaining old-name residue is now mostly
historical evidence, mission-file lineage, or explicitly named deletion target.

### 2026-06-17 - Runtime substrate, M3 settlement, and mission recuts

The second day focused on durable-actor substrate and proof discipline.

Key changes:

- Prompt defaults moved into YAML and were pulled into Nix/source filtering.
- Search/research capability improved: search outage surfacing, SerpAPI and GLM
  planning, identity-minimal framing, broader result sets, and researcher
  parallel-saturation work.
- Coagent delivery and trajectory handling advanced.
- Texture integrate wake and first-turn durable action landed.
- Overlay-pinned prompt evaluation endpoints/specs were added.
- M4 continuation deletion was compiled as a paradoc.
- M3 lifecycle cutover settled with deployed vmctl refresh proof.

M3 settlement is the most important architecture receipt from this day:

- Deployed proof ran against `choir.news` at commit
  `968ff7ffc35a0e2ee87a262dc0d8cdcef5cb87b4`.
- A researcher activation was running with work item
  `dd490475-a00d-4370-9589-67521d448733`.
- vmctl refresh preserved the same VM identity while moving from epoch 1 to
  epoch 2.
- The original researcher activation became `passivated` with
  `passivated_reason=runtime_restarted`.
- A replacement researcher activation started from
  `trajectory_work_item_sweep` with the same work item ids.
- Texture consumed the researcher update after refresh and ended with
  `worker_updates_pending=[]`.
- The trajectory completed and `POST /api/run-acceptances/synthesize` produced
  `runacc-d8d804ad592825e169fe` at `staging-smoke-level`.

Net effect: the portfolio spine moved. M3 is no longer a theoretical lifecycle
claim; it has a deployed falsifier showing passivation, rewarm, and no stranding
under vmctl refresh.

### 2026-06-18 - Long-running Texture attempt, rollback of wrong spine, and versioned artifact start

The overnight long-running Texture effort produced useful mechanics but also
exposed a wrong architectural route.

Supported mechanics landed:

- Multi-write-in-one-run Texture behavior.
- Faster interim first paint from model weights.
- A park primitive that waits before the next provider call, avoiding billed
  idle model calls.
- Per-actor tool-loop budget.
- Cadence proof: a user V0 followed by multiple appagent revisions, with first
  paint below the previous roughly 49s baseline.

The owner-side review found three serious issues:

- Foreground owner revises could be silently dropped while a Texture mutation was
  pending.
- One-write-era idempotency logic could complete a live mutation and make the
  still-running actor lose write authority.
- Tail commits added semantic role choreography to the core runtime, forcing a
  worker/delegation path through broad string classifiers. That violated the
  harness-minimalism rule and caused the overnight loop to chase a self-imposed
  acceptance checkpoint.

Fixes:

- `f002e07a` reverted the Texture-super completion-guard choreography family.
- `dfc78fcd` delivered owner revises to the resident Texture actor and kept the
  live mutation writable.
- Staging at `dfc78fcd` passed the cadence probe with V0 at +0.30s, V1 at
  +26.09s, and V2 at +62.71s.

After that review, the mission was re-pointed. The current target is no longer
"patch run-centric Texture until it looks long-running." It is:

- one durable agent thread per Texture document;
- same-thread resume, not cold rewarm from document head;
- event-driven delivery into the thread;
- always-deep research by default;
- quiescence instead of terminal completion;
- deletion of keyword classifiers, exact-first-tool retry machinery, cold
  rewarm, idle-death/resume-cap gates, and run-centric controller scaffolding.

That same day, the versioned-artifact mission was carved out and began landing:

- D1 typed system-attributed provenance.
- D2 tamper-evident revision hash chain.
- D3/D4 typed source evidence plus citation/quote validation gate.
- D5 full-history publish manifest.

Net effect: we avoided preserving a bad lifecycle abstraction, kept the useful
mechanics, and split the document-chain/publish problem from the durable-thread
problem.

### 2026-06-19 - Versioned publishing, signatures, reader lineage, and model policy

The third day finished major parts of the Texture artifact chain and repaired
foreground model policy.

Versioned artifact work:

- D7 product-path probe discovered that canonical Texture publishing returned
  404 through the proxy router. The root cause was a duplicate
  `/api/platform/texture/publications` case introduced by the hard rename: the
  retired-route 404 case collapsed onto the canonical route and shadowed the real
  handler.
- `736bdc5c` deleted the shadowing route and added a router-dispatch regression
  test. CI and Node B deploy passed; staging reported `deployed_commit=736bdc5c`.
- D7 deployed acceptance passed: a multi-revision Texture published with
  `version_count=3`, a `version_history_hash`, causal parent order, V0 content
  preserved, and chain head equal to the head revision hash.
- `e859ef27` added published-reader version-history disclosure: revision count,
  manifest hash, chain-head hash, chain-verified affordance, and lineage rows.
- `f59ea7ff` added platform Ed25519 signatures for every published revision.
  The deployed spec verified all three signatures with Node crypto and confirmed
  tampering failed verification.

Model policy work:

- A problem checkpoint documented that DeepSeek credit exhaustion and fallback
  behavior made the effective model policy unclear.
- `b26efec3` hard-cut foreground policy for all users:
  - conductor: `chatgpt/gpt-5.4-mini`, medium reasoning
  - Texture: `chatgpt/gpt-5.5`, low reasoning
  - researcher: `chatgpt/gpt-5.4-mini`, medium reasoning
  - super: `chatgpt/gpt-5.5`, medium reasoning
- The old `roles.vtext` role was removed rather than migrated.
- The Texture toolbar sources dropdown now exposes the models used for
  conductor, Texture, researchers, and super.
- Active runtimes were updated; hibernated runtime disk images were left alone
  because offline mutation would be riskier than waking or policy-path editing.

Net effect: published Texture artifacts now carry a verifiable history and
platform-signed revision metadata, and foreground agents now route through the
requested ChatGPT policy rather than relying on failing DeepSeek defaults.

## Mission Portfolio Progress

### M1 - Trajectory Model

Status: settled before this three-day window.

Trajectory and work-item records exist and form the substrate that later M2/M3
work relies on. The portfolio still treats M1 as the base of the durable-actor
spine.

### M2 - Messaging Cutover

Status: settled before this three-day window.

The old parent/child message surfaces were replaced by structured coagent
updates over the actor send path. This is the messaging foundation that made the
M3 refresh proof meaningful.

### M3 - Lifecycle Cutover

Status: settled on 2026-06-17.

This is the largest confirmed spine movement in the last three days. M3 now has
deployed evidence that vmctl refresh can passivate a live activation, rewarm the
work through trajectory/work-item sweep, deliver the update back to Texture, and
complete without stranded worker updates.

Residual compatibility surfaces remain named, not repaired by M3:

- `parent_loop_id` provenance residue;
- `run_memory_entries.loop_id` physical identity;
- generic run-acceptance export-level blocking for non-package trajectories.

These do not reopen M3. They are successor cleanup edges.

### M4 - Continuation Deletion

Status: planned/design complete, execution not started.

The M4 paradoc now exists and frames continuation deletion as a spine deletion,
not cleanup. Its target is to delete residual `RunContinuation` record/API/event
surfaces and re-point continuation-level acceptance at trajectory/work-item
settlement. This is the next core portfolio move after M3.

Until M4 executes, `continuation-level` remains transitional residue and should
not be expanded into new architecture.

### M5 - Wire On Settlement

Status: open handoff, still gated on M4.

M5 remains the product-path falsifier after the actor spine, not a substitute for
the spine. The last three days improved Texture and publication evidence, but M5
proper should still wait for continuation deletion.

### Texture Hard Cutover

Status: open handoff, but most live-product work is complete.

The cutover is materially advanced: Texture is the canonical product language in
live routes, app identity, frontend, prompt defaults, source contracts, and
publication paths. The current open edge is not "rename more strings blindly"; it
is to finish deletion receipts and keep historical evidence separated from live
ontology.

### Texture Versioned Artifact

Status: staging-smoke settlement for the main artifact-chain claim; paradoc body
needs a small factual refresh because D6 signatures landed even though the
original scope said signatures were out of scope.

Landed:

- D1 typed provenance.
- D2 revision hash chain.
- D3 typed researcher evidence replacing regex prose scraping.
- D4 deterministic citation/quote validation gate.
- D5 full-history publish manifest.
- D7 product-path publishing proof and reader lineage disclosure.
- D6 platform Ed25519 signatures for every published revision.

Open:

- Reader UX options B/C: revision browser, diff, and per-revision sources.
- Production signing-key provisioning via `PLATFORM_SIGNING_KEY_PATH`.
- Promotion-level evidence remains out of scope until AppChangePackage adoption
  and owner review are exercised.

### Texture Durable Deep-Research Thread

Status: planned after re-pointing; not yet implemented.

This is now the highest-leverage Texture runtime mission after versioned-artifact
settlement. The key shift is from "a run that parks and gets woken" to "one
durable thread per document." Settlement will require:

- same `loop_id` resume after passivation/refresh;
- durable inbox cursor and event-driven delivery;
- no cold rewarm from document head;
- no idle-death or resume cap as lifecycle gates;
- quiescence records instead of terminal completion;
- active deep research beyond V2;
- deletion of prompt keyword classifiers and exact-tool retry scaffolding.

## What Is Better Now

- Staging is on a coherent current model policy instead of a broken DeepSeek-led
  fallback.
- Texture is visibly and structurally the artifact control plane.
- Publication no longer flattens a document to the head revision; it can publish
  a verifiable history.
- Published revisions now carry platform signatures over hash-chained revision
  records.
- Citation/quote validation has moved from scraped prose toward typed evidence
  and deterministic validation.
- M3 has real deployed lifecycle proof, not just local tests.
- The bad instinct to encode semantic role choreography in the runtime was
  caught and reverted.
- The long-running Texture target is sharper: one durable deep-research thread,
  not patches over a run-centric controller.

## Open Risks And Next Moves

1. Execute M4 continuation deletion.
   This is the next architecture-spine move. Delete the residual continuation
   API/record/event/control surfaces and re-point acceptance language to
   trajectory/work-item settlement.

2. Implement the durable Texture thread model.
   Start with R1/R2: same-thread lifecycle and event-driven delivery. Do not add
   another classifier or guard tree. The mission's deletion pressure is correct.

3. Refresh the versioned-artifact paradoc.
   The ledger is ahead of the mission body because D6 signatures landed. Update
   the body to reflect that signatures are now implemented and identify key
   rotation/provisioning as the remaining edge.

4. Keep product-path proof honest.
   The D7 route 404 showed why handler-direct tests are insufficient. For
   protected paths, keep router/API/browser acceptance in the proof set.

5. Avoid offline bulk mutation of hibernated computers.
   Active policy was updated. Hibernated computers should pick up policy through
   the product/runtime path or a deliberately designed wake/update operation, not
   ad hoc disk-image editing.

## Bottom Line

The last three days repaired several visible product failures, but the larger
progress is architectural: M3 settled, Texture became a real versioned and
signed artifact chain, and the wrong run-centric long-running-agent path was
identified before it ossified. The major refactor is not done. It is now at a
clear fork: delete continuation residue with M4, then make Texture a true
durable thread rather than another lifecycle workaround.
