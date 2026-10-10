# ACTIVE — Confirmed Work View

**Status:** curated transition view. It is narrower than the legacy mission
corpus and does not make an unverified graph status into a live work claim.
The mission roadmap is [`world-wire-mission-stack-2026-09-22.md`](world-wire-mission-stack-2026-09-22.md);
the mission format is throughline (`skills/throughline/SKILL.md`).

## Roadmap (owner-ratified 2026-10-09)

Three gates, in order, each with an exit test — see "v6 plan" in the
[metamission](definitions/choir-supervised-app-development-metamission-2026-10-01.md):

1. **Stable computer** — SH (O21) → SL (O1) → O7/O12 checks → O8/O9 →
   Texture contract. Exit: lose-the-disk proof, SL fault matrix, Texture
   acceptance, and owner confidence in the autoputer. Stable uptime is
   counted, not waited for (owner 2026-10-09: no 72 h clock).
2. **Self-development with live Texture supervision** — S1 → S4 → S5 → S6.
   Exit: owner-requested change, capsule build, live preview, approval,
   release, rollback.
3. **Autopaper** (formerly World Wire; owner 2026-10-09: Gate 1 today,
   Gate 2 tomorrow, Gate 3 in October; design draft
   [`autopaper-sw-design-2026-10-09`](autopaper-sw-design-2026-10-09.md);
   name retirement is a TODO there, §9) — four core desks, minimal
   structure, living Textures woken by evidence, automatic publishing into
   the host-level store, starting from zero content.

Deferred: S3, SA density, S7–S11, and
[SP production infrastructure](definitions/choir-appdev-sp-production-infrastructure-2026-10-09.md)
(O22; until Gates 1–2 pass human QA). Corpus (Store B) teardown owner-approved
2026-10-09; processor/reconciler deletion committed.

## Gate 1 hardening landed 2026-10-09 (owner-supervised overnight session)

Receipts are in `docs/problems/*-2026-10-09.md`:

- **Texture:**
  - Cold list down to 0.10–0.19 s (was 2.7 s first call); boot replay
    from 9–11 min to 4.7 min, with further scan fixes in c0903096.
  - The "Revising…" zombie cycle is fixed. 28 never-executed trajectories
    were disposed (owner-approved).
  - Cancel now reads a summary view and resumes a stuck intent.
- **Owner rule (2026-10-09):** a crash restart never resumes work, for any
  desk; a planned update restart (durable marker, consumed once at boot)
  may. Deploys restart only idle computers. AGENTS.md "Restarts End Work
  (Crash) Or Resume It"; later hibernate/wake and a management-owned
  resume subsystem (residual `management-owns-stalled-work`).
- **Host:**
  - memory budget with no swap;
  - shared Go build, so deploy jobs take ~3 min (was ~13);
  - node-a is Node B's remote builder.
  - Disk survey and reclaim list:
    [`node-b-disk-free-space-2026-10-09`](problems/node-b-disk-free-space-2026-10-09.md).
- **Signup:** credential issuance replays by key, and vmctl retries
  transient failures (964a68ea).
- **Deployed and verified 12:25Z on ae61f153:** boot replay 31 s (was
  4.7 min), no Texture work at boot, 22 documents show "Interrupted by a
  restart", list 0.13 s.
- **CI:** main pushes plan against the live staging commit (6d6db691), so
  a docs push can no longer skip deploying queued code.
- **Open:**
  - quarantine images (2 × 32 G) kept pending the 484-path check; the
    pre-compact backup and the old corpus repo are deleted (226 G free);
  - CI item 2 (main runs serialize whole runs; no longer hides code);
  - a credential failure that outlasts the retries still marks a computer
    `failed`. Low: the next resolve starts it again with a fresh epoch on
    the same VMID and data root (`internal/vmctl/ownership.go:1611`). The
    only failures today were at 02:42Z, before 964a68ea; the 12:25Z
    refresh on ae61f153 issued a credential and booted normally. A
    staging signup E2E was not run (it creates a real account and VM).
  - Store B: reset to an empty repo, dumped (23 GB), and the 104 G old repo
    deleted (owner, 14:04Z). Autopaper starts from zero content.
- **SH lose-the-disk (Gate 1 exit): PASSED on staging, 2026-10-09 14:45Z.**
  A disposable computer lost its realization and volume; the next
  realization got its key from escrow, hydrated 2,460 files and served the
  private proof file 67 s later
  ([receipt](evidence/sh-lose-the-disk-2026-10-09T14-44-09-081Z.json)).
  Fixes: c72c38c4 (hydration), 799097e3 (durable fresh-volume marker),
  a47122de (every runtime deploy takes the reboot path; no version skew).
  Bounded residual: a disk lost before the first projection checkpoint
  waits up to ~1 min for `checkpointd`. Remaining Gate 1 exit items: SL
  fault matrix, Texture acceptance suite.
  Debugfs key copier deleted (a72e2d32, deployed); routed cold recovery
  is proven at Gate 2 (it needs a route slot). SH remainder (slices 4-5)
  moves to Gate 2.
- **SL define slice (15:45Z):** [obligation inventory](evidence/sl-obligation-inventory-2026-10-09.md).
  One periodic driver, ~30 boot/event reconcilers, budgets on four paths,
  actor wakes retry uncounted. No fate surface exists, so measuring needs
  it built. Next: panel review, then registry plus fate surface.
- **CI item 2:** main runs stay serialized (parallel runs can invert deploy
  order; see the CI latency problem doc).
- **SL slices 1–3 deployed (afternoon, e3f2d560):** crash vs planned
  restart marker (all desks), event-driven wake outbox with a 5-attempt
  budget and a visible `dispatch_exhausted` fate, and
  `GET /api/runtime/obligations` (what a computer owes, no SSH). Fault
  matrix legs pinned: poison-wake isolation (unit) and "nothing owed after
  a crash" (Texture suite T8, staging).
- **O12:** CI lock-scope ratchet (`cmd/lockscope`): no new lock held across
  network, exec, sleep or channel waits; 5 known holds baselined.
- **O7:** fleet boot measurement
  ([evidence](evidence/o7-boot-cost-vs-history-2026-10-09.md)): health is
  flat (7–11 s). The ~17 s that looked like history cost is a surface
  bootstrap retrying a deterministic refusal on layered computers; fixed
  in b2a76845 (two attempts), deploy pending.
- **Texture acceptance suite (evening, six runs;
  [suite](texture-acceptance-suite.md) has the run table):** T1–T5 pass
  (list ~30 ms, first draft 26–46 s, revise 16–26 s, cancel clears in
  1 s). Each later failure was a different layer, documented first and
  fixed in order: settled work refused the owner's revise (4f9331cf); a
  caller cancel cooled down the whole search plane, a provider key rode
  error summaries to computers, and the gateway ops routes checked no
  caller (97970cda, b445fd21; key rotation is the owner's call); research
  packets were rejected by an unseen schema (19b7ef48); then the
  Texture/research loop had no stopping rule (dc3e86b6, two research
  openers per owner request). Open: T5b (Cancel ends the document;
  B2 leaning, `texture-terminal-trajectory-revise`), research work never
  settles (`texture-research-assignment-finish`), T7/T8 not yet reached
  on a green T6. Cluster record:
  [`clustering-texture-obligation-closure`](problems/clustering-texture-obligation-closure-2026-10-09.md).
- **Gate 2 opened (drafted):**
  [`definitions/choir-appdev-gate2-supervised-self-development-2026-10-09.md`](definitions/choir-appdev-gate2-supervised-self-development-2026-10-09.md).
  Reality slice: seven M11 reruns, each one layer deeper. Rerun 7
  (2a16a6db) froze, verified, reached approval and began apply for the
  first time since S2; apply stalls in materializing because the
  replay-completeness checkpoint sees resumed desk work and then an
  og_objects difference
  ([problem](problems/selfdev-apply-checkpoint-starved-by-resumed-work-2026-10-10.md)).
  Rerun 8 (5704ace6): the bounded apply hold worked; the replay diff
  showed the engineering path minting co-super-* ids that from-genesis
  replay upcasts. Fixed in f9531177. Rerun 9 (77406673): engineering
  replays exactly; the upcast still rewrote recorded tool output, so
  the probe and restore now replay in the live store's deposit mode
  (6e7e3c5e). Rerun 10 (cf0969cf), after the Node B OOM and the
  vmctl host-capacity guards: the verifier correctly rejected an
  unappliable source.patch (phantom-line creation hunks, which reruns 8
  and 9 also shipped). The freeze now proves the patch with git apply
  (1ae758ef). A trace review of every log found desks guessing enums
  against the reducer, and rejections now name the accepted values
  ([problem](problems/trace-review-desk-protocol-friction-2026-10-10.md)).
  Rerun 11 reached the first completed apply of Gate 2 (09:21), then a
  mid-run deploy I pushed killed the working guest through a boot path
  the new resolve guard (cfc17e05) does not cover. The chokepoint is
  vmmanager.bootVM's orphan kill (one of at least five kill sites; docs/vmctl-360-review-2026-10-10.md)
  ([problem](problems/vmctl-restart-reboots-busy-computer-2026-10-10.md),
  correction and clustering assessment). Probe scripts are now tooling
  (a820beea). Track D landed: an ordinary
  engineering freeze opens its own promotion candidate when the mode is
  armed (19834f68). Every desk now knows its REPL surface: a generated
  prompt block, choir.Help(), and compile errors that name what exists
  (79edc337). Owner-requested vmctl 360 review converged 8/8 on a
  redesign ([review](vmctl-360-review-2026-10-10.md)); its Phase 0
  (receipts, disarm escalation, close destructive paths, bounded
  admission) precedes rerun 12; step 1, kill receipts, is accepted on
  staging. The owner's computer carried five self-development operations
  stuck in executing since August, which kept it busy forever; a crash
  boot now closes them and a computer admits one open operation
  ([problem](problems/selfdev-zombie-operations-pin-owner-computer-2026-10-10.md)). Fixed tonight on
  the way: capsule worker, freeze on new directories, delegated-cast
  reports to Texture, verifier bundle mirror, inference breaker, apply
  hold, engineering id spelling.

## Blocking Substrate Mission — Platform-Dolt Capacity Stabilization (superseded 2026-10-09 by the corpus teardown)

[`definitions/choir-platform-dolt-capacity-stabilization-2026-10-01.md`](definitions/choir-platform-dolt-capacity-stabilization-2026-10-01.md)
(`readiness: drafted`, `now.status: working`) — promoted to top
priority by the 2026-10-01 convergent panel (verdict B): the platform-dolt OOM
→ realization-flap → serial-drain-starvation substrate now blocks both the
parked-run repair's `deployed_acceptance` and owner task submission (the `run
start` 502s). Clustering assessment:
[`problems/platform-dolt-oom-realization-cluster-2026-10-01.md`](problems/platform-dolt-oom-realization-cluster-2026-10-01.md).
**Repair applied live 2026-10-01** (owner authorized "fix the oom issues"):
corpus-dolt cgroup cap `MemoryHigh=12G`/`MemoryMax=14G` — the corpus store
(:13307) was the growth driver; platform-dolt (:13306) is healthy at ~0.8G.
`MemoryCurrent` dropped 16.6→11.6GiB, `OOMKills=0`, `NRestarts=0`, guest
rebooted clean and is draining the dead-wake backlog. Durable drop-in
persistence + a stable-uptime `run start` + the `362febb2` discharge are the
remaining acceptance. It precedes the metamission's M0a/M-SUB stations, which
are substrate-gated. The parked-run repair (`14f5682b`, deployed `4a718af4`)
stays open with `deployed_acceptance` pending — discharged as a side effect
once the guest holds an uptime window.

## Current Mission — Supervised App Development Metamission

[`definitions/choir-supervised-app-development-metamission-2026-10-01.md`](definitions/choir-supervised-app-development-metamission-2026-10-01.md)
(`readiness: executable`, `now.status: working`) — **owner-approved
2026-10-01** after agentic-consensus review (5/5 convergent + 7/7
divergent; all required revisions applied in v3). Thirteen stations;
2026-10-04 three closed in one pass:

- **S0m** record-native messaging **complete** (boundary
  `s0m-to-next-transition-2026-10-04`; ask→report→resolve
  deployed-verified on trajectory `684ddcb1`; consume-marking `d61c9b1b`;
  reporter letter `reports/s0m-record-native-station-close-2026-10-04.md`).
- **S1a** host-boundary hotfix **complete** (boundary
  `s1a-to-s0b-transition-2026-10-04`; deployed refusal matrix PASSED —
  5/5 refusals incl. tap→tap FORWARD drop + forged-identity 403/405,
  5/5 legitimate flows green; reporter letter
  `reports/s1a-host-boundary-station-close-2026-10-04.md`).
- **S0** reality + boot timeline **complete** (boundary
  `s0b-boundary-close-2026-10-04` on the S0 station file; 10 probe
  receipts on disposable `computer-ac1808b4` + second disposable for the
  Go-effect re-run; two new problem docs: M9a route-projection
  owner-binding defect + selfdev ops-launch freeze-authority gap;
  builder-substrate decision narrowed to host-service vs
  privileged-builder-capsule).

- **S2** layering-runtime-from-release **CLOSED** (2026-10-05, terminal
  receipt `s2-station-terminal-2026-10-05`): all six acceptance criteria
  discharged at deployed-proof class on disposable `computer-6450a253`
  plus the owner computer — builder-produced no-reboot apply with
  built-bytes frontend join, CI-driven M9a push (run 37378327268,
  2/2 healthy, t2h=131s), six refusal fences, post-disposal+reboot
  base-absent refusal, GC-rooted private-store boundary, clean rollback.
  Consensus round 2: 6 approve / 1 send-back (sole send-back receipt
  completion only, discharged by fc/boot continuity pins). Contract
  slices landed: S2-e rollback atomicity, S2-d state-compat gate, S2-c
  provenance, S2-f builder join, S2-g CI wiring. Open residuals (problem
  docs, not blockers): post-swap guest restart loop, canonical-head
  bootstrap authority, platform-artifacts GC cadence (S0 — first sweep
  reclaimed 35.4GB on Node B). Next spine station per v5: S3 fast resume
  after SR/SA density work; parallel: S1 remainder + SO ops substrate.
- **SMG** management RLM cutover **CLOSED** (2026-10-06, spine receipt
  `smg-to-sa-transition-2026-10-06`): legs 1-3 passed on a fresh
  disposable (computer-0ca7656f, build 475902d7,
  `docs/evidence/smg-rlm-acceptance-disposable-2026-10-06.json`) via the
  deterministic `POST /api/texture/management-open` acceptance surface;
  deployed schema leg = management exactly `{desk_go_eval}`
  (computer-03335285). Named edge: re-run legs on the owner computer
  after SA slice 1 (incl. weak legs: cell-level unbound refusal +
  work settlement). New substrate defects handed to SA slice 1:
  delegated-report poisoned delivered-page listing + mint-no-start slot
  deadlock (fixed 01199fb1) + AppendEvent unbounded scan (fixed
  475902d7) + wake-outbox re-arm storm.
- **SA** agent density **working** (slice 0 deployed 9f6f369c; slice 0
  residuals landed 2026-10-07: delegated-report delivered-page fix
  `ca8c8c18` + apply-fence liveness-tolerant probe `19d7913e` +
  commit-bound push gate `68397ae7`; owner runs `19d7913e`. Slice 1
  storm convergence open; drain fixes landed `91e9c03b`/`29817fda`/
  `5eb63161`/`7a36713c`. Management-open wake race fixed `e9cd9fed`
  (problem doc `smg-management-open-invalid-transition-2026-10-07.md`);
  deployed-verified 2026-10-07 on fresh disposable `computer-e472d237`
  (legs 1-3 + bound report green, evidence
  `smg-rlm-acceptance-fix-verify-2026-10-07.json`).
  Owner-VM boot-loop incident (2026-10-07) → permanent recovery policy
  landed and deployed 2026-10-08 (`67729e07`): verified incremental
  checkpoints (`checkpointd`), typed vmctl admission, deposit upcast.
  Cadence/replay repaired-with-residual; admission awaits its staging
  refusal demo. Record:
  [`problems/sa-projection-base-watermark-never-refreshed-2026-10-07.md`](problems/sa-projection-base-watermark-never-refreshed-2026-10-07.md).
- **SL** obligation terminality **working** (opened 2026-10-08, owner
  direction "proceed with mission next"):
  [`definitions/choir-appdev-sl-obligation-terminality-2026-10-08.md`](definitions/choir-appdev-sl-obligation-terminality-2026-10-08.md)
  — enforce operational invariant O1 from the
  [operational invariants register](operational-invariants-register-2026-10-08.md)
  (draft, owner-approved set). Absorbs SA slice 1 (Management storm
  convergence). First slice: read-only obligation inventory.
- **SH** state homes **working** (opened 2026-10-08, owner direction
  "good policy and good ideas. Let's do it"):
  [`definitions/choir-appdev-sh-state-homes-2026-10-08.md`](definitions/choir-appdev-sh-state-homes-2026-10-08.md)
  — enforce O21, a realization holds no unique state
  ([inventory](state-homes-inventory-2026-10-08.md)). Escrowed privacy key
  delivered to a computer's new realization (owner-ratified). Slice 1: typed
  `privacy_key_unavailable` refusal and prompt guest fatal-startup reporting.
Ordered ahead of the World Wire metamission per owner direction
2026-10-01 ("a good prerequisite to it").

## Parallel Mission — Jev / Supervision Metamission

[`definitions/choir-jev-supervision-metamission-2026-09-29.md`](definitions/choir-jev-supervision-metamission-2026-09-29.md)
is still `working` but no longer the live entrypoint — the app-dev
metamission took spine position 2026-10-01 on owner approval. Its
station files and receipts remain valid; it resumes as parallel work or
is folded when its remaining stations intersect S1-S11.
## Parallel Mission — Texture Latency Cutover

[`definitions/choir-texture-latency-cutover-2026-10-01.md`](definitions/choir-texture-latency-cutover-2026-10-01.md)
(`readiness: executable`, `now.status: working`) — owner-directed
2026-10-01: bound Texture editor request latency end-to-end (proxy resolve +
guest store + lifecycle snapshot). Three ordered cuts: trajectory-scoped or
version-memoized snapshot read, TTL-cached vmctl route resolution, and —
only if the timing matrix still fails — decoupling the objectgraph read
mutex from writes. Evidence map:
[`problems/texture-latency-layered-evidence-2026-10-01.md`](problems/texture-latency-layered-evidence-2026-10-01.md).
Independent of the capacity mission's substrate work; runs parallel.

## v5 (2026-10-05) — density before distribution

Owner-approved outline folded into the app-dev metamission (see its "v5 plan").
New stations:
- **SR** desk-surface cleanup: finish the research RLM cutover (registry →
  `{desk_go_eval}`, prompt rewrite) and delete processor/reconciler. Absorbs
  Jev M0a.
- **SA** agent density: many concurrent desk activations in a 2–4 GiB guest.
  Memory, the embedded-Dolt `engineMu` bottleneck, and the
  operational/versioned store split. Gates S3.
- **SM** model policy rewrite + evals. Absorbs Jev M2; Jev M3 is its first
  consumer.
- **SC** desk capability surface: per-desk packages, Graph/Ledger/Similar/Source
  verbs, management scorer fan-out.

World Wire phase 1 (the newspaper) can start after S5.

## Successor Spine — World Wire Metamission (SUPERSEDED 2026-10-05; rewrite pending)

The 10-01 file below predates the rearchitecture
([`world-wire-rearchitecture-2026-10-05.md`](world-wire-rearchitecture-2026-10-05.md)):
processor/reconciler and host ingestion are deleted. It is kept for its
receipts (owner verticals ordering, June failure receipt).


[`definitions/choir-world-wire-metamission-2026-10-01.md`](definitions/choir-world-wire-metamission-2026-10-01.md)
(`status: intent`, `entrypoint: false`) — designed from the 2026-10-01
convergent panel (7/13) + the June attempt-12 receipt. RLM-native compounding
knowledge base on the object graph: processor/reconciler stay non-desk but the
unit of reasoning shifts per-item→per-set (bounded collection-manifest
obligation), killing the June admission-freeze failure mode. Stations:
**W0** KB-graph schema (green, parallel) → **W1** tiered dedup → **W2**
batched processor → **W3** corpus reconciler → **W4** edition assembly →
**W5** AI vertical → **W6** Taiwan/geopolitics/semis/internal-democracy →
**W7** region ladder → **W8** scale hardening. Live W-stations are gated on
the capacity mission's deployed acceptance; W0 design runs green in parallel.


## Archived Receipt — Strand-2 Freeze-Order Patch (R0)

[`definitions/choir-strand2-freeze-order-patch-2026-09-23.md`](definitions/choir-strand2-freeze-order-patch-2026-09-23.md)
**landed 2026-09-24** (deployed commit `64468db2`, CI green). The
freeze-before-validate wedge class is closed: a refused `choir.Freeze` on a
document trajectory leaves the executor Active (a `capsule_go_eval` probe
executes after the refusal) and `choir.Complete` lands the fate saga to
`revoked`. A separate capsule-teardown defect (cgroup not empty on
`ForceDestroy`) surfaced during acceptance and is fixed in the same deploy.
This is an archived receipt, not a working entrypoint.

The desk-RLM rectification spine was **restructured 2026-09-25** after R2
closed on substrate. The current mission graph is
[`desk-rlm-rectification-plan-2026-09-23.md` §11](desk-rlm-rectification-plan-2026-09-23.md)
and the restructured-spine section below. The 2026-09-23 draft successors
were deleted; new throughline `/goal` files are authored per mission.

## Archived Receipt — Commitment Ledger + Desk Carrier (R2)

[`definitions/choir-commitment-ledger-desk-carrier-draft-2026-09-23.md`](definitions/choir-commitment-ledger-desk-carrier-draft-2026-09-23.md)
**closed on substrate 2026-09-25** (`now.status=complete`; carrier landed +
deployed `537fce04`, `choir.Resolve` `5404d2c7` on main). Delivered the
OG `choir.commitment_record`, the engineering in-cell carrier, the
semantic-act verb surface, and delegated-cast admission. Deferred to the
restructure (plan §11): the deployed management-cell cast proof, texture's
ledger consumer, the `commit`+cursor atomic-commit repair, and the
fate-sweep ungating residue. This is an archived receipt, not a working
entrypoint.

## Archived Receipt — Texture Owner Input Cutover (M1)

[`archive/choir-texture-owner-input-cutover-2026-09-22.md`](archive/choir-texture-owner-input-cutover-2026-09-22.md)
**landed 2026-09-23** (deployed commit `3b780ed2`, CI green, deployed
acceptance spec `frontend/tests/texture-owner-revision-deployed.spec.js`
passing on https://choir.news with a turn-consumption receipt binding the
owner revision). Owner input to a lifecycle-bound Texture document is a
canonical document revision event; the `tell`/`correct`/`roster`/
`LifecycleOwnerInstruction` side channel is deleted. Consensus review sent
the first acceptance back (spec bound any head, not the owner's); the
tightened spec closed it. This is an archived receipt, not a working
entrypoint.

## Superseded Archive — Sub-RLM Document Channel / Carrier Landing (M2)

[`archive/choir-sub-rlm-document-channel-2026-09-22.md`](archive/choir-sub-rlm-document-channel-2026-09-22.md)
is **superseded** by the desk-RLM rectification R-sequence. Its remaining
carrier and document-channel scope is retained as historical context; it is
not a working entrypoint.

## Superseded Archive — RLM Engineering Carrier

[`archive/choir-rlm-engineering-carrier-2026-09-11.md`](archive/choir-rlm-engineering-carrier-2026-09-11.md)
is archived historical evidence. Its remaining in-cell carrier, R7/R9
retirement, and canonical-run scope is addressed only through the desk-RLM
rectification sequence.

## Restructured Spine — Ratified 2026-09-25 (post-R2)

The 2026-09-23 draft `/goal` files were **deleted 2026-09-25** (stale
anchors `main@b0adf6f7`/`4de7fdf9`, dead `super_controller.go` citations,
and R3's smuggled R4 scope). The rectification spine was restructured by a
divergent+convergent consensus panel and **ratified by the owner
2026-09-25**; the current mission graph is
**[`desk-rlm-rectification-plan-2026-09-23.md` §11](desk-rlm-rectification-plan-2026-09-23.md)**.
The owner ratified the spine as a whole; each mission still runs under its
own throughline `/goal` file. `/goal`-station rule (AGENTS.md): after each
terminal receipt, set the next mission's goal — the entrypoint below is the
live one.
- **Spine meta-goal** [`definitions/choir-rectification-spine-2026-09-25.md`](definitions/choir-rectification-spine-2026-09-25.md) — **settled 2026-09-29** on M11's landed receipt; archived. The live entrypoint is the Jev/supervision metamission above.
- **R2x landed** (`ebdaef45`+`53035642`: deadline wake armed at bind, both selection sweeps deleted, commit self-heals via `recoverPartialActCommit`); deployed-cancel proof deferred — lever `CHOIR_ASSIGNMENT_DEADLINE` committed, needs `choir.assignment_deadline` cmdline plumbing for a staging probe.
- **R3a landed** (`4cf82057`: occurrence/evidence resolves desk acts from `choir.commitment_record`, dual-read vs `worker_updates_*`); deployed live-record proof deferred to a doc desk report-cast. — goal file: [`definitions/choir-texture-ledger-consumer-2026-09-25.md`](definitions/choir-texture-ledger-consumer-2026-09-25.md).
- **R3b landed+deployed** (`b9f43583`: host desk-cell carrier — `autoputer desk-session`, `desk_go_eval` sealed per profile, canonical-ledger reduction, `InCellCarrier` fan under `actuator=rlm`, unknown-desk cast rejected, kill mid-cell respawns clean); deployed live-desk acceptance deferred to R3c (actuator). — goal file: [`definitions/choir-desk-cell-carrier-2026-09-25.md`](definitions/choir-desk-cell-carrier-2026-09-25.md).
- **R3c landed+deployed** (`b7ae7596`: management on the desk-cell carrier — `desk_go_eval` + typed lifecycle controls, cast reaches delegated admission on the canonical ledger). — goal file: [`definitions/choir-management-live-cast-2026-09-26.md`](definitions/choir-management-live-cast-2026-09-26.md).
- **R3d landed+deployed** (`119e0edd`: texture full-RLM — `desk_go_eval` only; cell-authored `ApplyTextureTurn` commits mint `AuthorAppAgent` revisions; typed texture tools + `worker_updates` consumer deleted). — goal file: [`definitions/choir-texture-live-authoring-2026-09-26.md`](definitions/choir-texture-live-authoring-2026-09-26.md).
- **R3r landed+deployed** (`3b56c34e`: research on the desk-cell carrier — `desk_go_eval` + typed research/evidence/memory surface under a per-activation egress budget and 8GiB worker cap; generic host tools retired; D2 discharged). — goal file: [`definitions/choir-research-live-cell-2026-09-26.md`](definitions/choir-research-live-cell-2026-09-26.md).
- **R4 landed+deployed** (`68a2e023`: commitment ledger read surface — derived accrual views, falsification-visible materiality projection feeding `textureAvailableSourceEntities`, score-free acting packs on the cell frame via `choir.Pack`, learning-claims gate on the selfdev verification payload). — goal file: [`definitions/choir-commitment-scores-packs-2026-09-26.md`](definitions/choir-commitment-scores-packs-2026-09-26.md).
- **R5a** vocab decoders + seed freeze — gates M11; keep `choir:co-super-assignment:v3`. **R5b** rename-migrate deferred indefinitely.
- **M7 landed+deployed** (`722b49bf`: derivable selfdev continuations — post-commit observer + coalesced drain advance an op through materialization with no API driver; mid-materialize crash recovers derivably; parked ops mint management-addressed boundary records). — goal file: [`definitions/choir-selfdev-derivable-continuations-2026-09-26.md`](definitions/choir-selfdev-derivable-continuations-2026-09-26.md).
- **M9a** platform push + restore — on M11's restore edge; chartered under the meta-goal: [`definitions/choir-platform-update-push-restore-2026-09-26.md`](definitions/choir-platform-update-push-restore-2026-09-26.md).
- **M11 settled — self-dev gate LANDED 2026-09-29** (staging `e886f576`):
  the reversible-selfdev-v1 episode ran end to end on staging with no
  manual repair — probe run 9 predicate `satisfied`, all six legs
  (desk-authored op driverless frozen→awaiting_approval→applied under
  qualified consensus, apply tail events on the tape, candidate-B
  rejected, falsified record visible in the live Texture doc, pinned-head
  restore via tape_reconstruct). Substrate repairs that made it stick:
  nine obligation-edge closures/merges landed under the clustering
  assessment; the probe found + fixed the reject pin-verification defect
  (`89cd7247`) — candidate-B reject had never committed since 7d635330.
  Goal file: [`definitions/choir-selfdev-gate-2026-09-27.md`](definitions/choir-selfdev-gate-2026-09-27.md);
  evidence: [`evidence/m11-probe-run9-satisfied-2026-09-29.json`](evidence/m11-probe-run9-satisfied-2026-09-29.json).
  The spine is complete; the next frontier is the Jev/supervision
  metamission (entrypoint above). M9b/M10 of the world-wire stack are
  unblocked and fold into that metamission's station sequence.

**K** [`definitions/choir-ontology-kernel-2026-09-24.md`](definitions/choir-ontology-kernel-2026-09-24.md) — landed substrate (main@66981cef); its `now.status=working` only for a pending owner deletion-sweep ruling.


## Completed Definition — Private Programmable Go Actor Kernel

[`archive/choir-private-go-actor-kernel-2026-08-12.md`](archive/choir-private-go-actor-kernel-2026-08-12.md)
completed 2026-08-27 (deployed commit `53f80af4`). It establishes private,
interpreted Go activations via a process-per-activation Yaegi sidecar inside
disposable guest-local capsules with unified Bash/Go broker routing, opaque
handles, durable continuity across forced activation death, immutable Texture
transclusion of host-selected salient receipts, and verified genesis surface derivability.
## Completed Definition — Durable Substrate Overhauls

[`archive/choir-durable-substrate-overhauls-2026-08-23.md`](archive/choir-durable-substrate-overhauls-2026-08-23.md)
completed 2026-08-26 (commits `f0e68b0a`..`e65e91c4`). All four tracks are
implemented and verified: (1) Track K key escrow with 2-of-N quorum gate and
WebAuthn PRF wrapping (deployed & proven on staging); (2) Track F encrypted 4MiB
chunk file-CAS, Merkle manifests, tape citations (`file_root_committed`), sync barrier,
atomic boot hydration, and ProjectionBase materializer; (3) Track M host fsync'd MTA
spool, async LMTP drain, and guest Maildir; (4) Assurance & Scale self-describing
recovery capsules, automated restore drill runner, and background blob integrity scrubber.
## Completed Definition — Substrate Cleanup and Cutover

[`archive/choir-substrate-cleanup-and-cutover-2026-08-25.md`](archive/choir-substrate-cleanup-and-cutover-2026-08-25.md)
completed 2026-08-26 (commit `a12532d2`, staging `c3314c59`). The guest MicroVM
closure now executes strictly ONE immutable Nix store binary
(wrapper `safvdbs8...` has zero `choir-updater/current` fallback references),
standard embedded Dolt GC is restored (no `RUNTIME_DOLT_GC_DISABLED`), and the
`choir.refresh_runtime=1` deletion dance is deleted (`VMConfig.RefreshRuntime`
removed). Boot-proven on staging: a hibernated test computer resumed onto the
new guest image on month-old persistent disk and reported `deployed_commit
c3314c59` ready. Self-development effects remain formally suspended. The Node B
deploy disk-preflight floor was calibrated 120 GiB -> 100 GiB with a
problem-documentation-first receipt
([evidence](evidence/node-b-deploy-disk-preflight-floor-2026-08-26.md));
host disk expansion remains an owner decision.

## Completed Recovery — Account & Mail (`yusefnathanson@me.com` / `000@choir.news`)

Account `yusefnathanson@me.com` (`5bd6de97-3b58-408c-bf89-c42c81b083de`) is restored:
the route-bound 0333528 computer is active at epoch 804 under the host and guest
maintenance holds, authenticated shell bootstrap returns HTTP 200, and the owner
mailbox exposes the Ryan message. Guest Maildir synchronization is not claimed.
Evidence: [`evidence/account-recovery-yusefnathanson-2026-08-27.md`](evidence/account-recovery-yusefnathanson-2026-08-27.md).

## Completed Definition — Substrate & Scheduling Readiness

[`archive/choir-substrate-and-scheduling-readiness-2026-09-02.md`](archive/choir-substrate-and-scheduling-readiness-2026-09-02.md)
completed 2026-09-03 (deployed commits `2bf93be7`..`bf6c51c0`). Target achieved:
live-trigger-only Super wakes, FIFO selection under computer-scoped arrival ordinals
across 4 sequential cycles without supersession, boot-does-not-schedule assertion
verified across epochs, settlement (tombstone) of the nine 08-19 cancel producer
reports via dedicated store reducer with claim-scan retired, exact-run resume structurally
isolated, terminal-event probe negative and positive verified, and normal-boot stability
without `RUNTIME_MAINTENANCE_HOLD` on staging `computer-03335285269bdba4f94377e56879f9e6`
(pre-A checkpoint `99949fe2`). Deployed evidence:
[`evidence/effects-red-substrate-scheduling-readiness-complete-evidence-2026-09-03.md`](evidence/effects-red-substrate-scheduling-readiness-complete-evidence-2026-09-03.md).
Effects remain OFF.

## Completed Definition — RLM Restore-Zero

[`archive/choir-rlm-restore-zero-2026-09-08.md`](archive/choir-rlm-restore-zero-2026-09-08.md)
completed 2026-09-09 on owner-scoped retained-store boot of `9341b5d1`.
PlanRecovery resumed `computer-03335285269bdba4f94377e56879f9e6` at
`local=148431 W=148431 H=148431 tail=0` (epoch 894, `10.200.12.2`).
No prefix page fetches. CI constructed-computer skip (G4) was left intact.
Proof: [`evidence/choir-rlm-restore-zero-retained-boot-2026-09-09.md`](evidence/choir-rlm-restore-zero-retained-boot-2026-09-09.md).
Post-completion panel (12/13 routes; 8 accept-with-conditions, 4 reject):
[`evidence/choir-rlm-restore-zero-post-completion-consensus-2026-09-09.md`](evidence/choir-rlm-restore-zero-post-completion-consensus-2026-09-09.md).
The earlier W=1/W=13 HTTP 200 claim remains demoted. Follow-on: keep W near H;
nontrivial-tail rematerialize/restore drill; proxy 502-during-resolve is not this mission.
## Completed Definition — RLM Settlement Gate

[`archive/choir-rlm-settlement-gate-2026-09-09.md`](archive/choir-rlm-settlement-gate-2026-09-09.md)
completed 2026-09-09 (deployed commit `6b758878`, CI run `34401118732`). All 8
acceptance items are satisfied: Yaegi compile gate with non-mutating heap preservation,
deletion of session-spawn fallback `fallbackGoEval` from active RLM route,
v1 terminal identity contract with proposition digest, derived ReportID, slot conflict
gating and supersede tuple, narrow assigned-CoSuper admission grammar with sequential
execution and toolloop decoupling, resumable fate saga with atomic final settlement
strictly following durable revocation acknowledgement, single reducer author for orphan
closure, and physical staging proof on `computer-03335285269bdba4f94377e56879f9e6`
with effects OFF. Deployed evidence:
[`evidence/choir-rlm-settlement-deployed-proof-2026-09-09.md`](evidence/choir-rlm-settlement-deployed-proof-2026-09-09.md).
The cutover's withheld settlement acceptance is closed; the cutover stays remainder
holder (residue R6). Mission-0 live drill debt stays mission-0-owned (residue R1).

## Completed Definition — RLM Versioned Rename

[`archive/choir-rlm-versioned-rename-2026-09-09.md`](archive/choir-rlm-versioned-rename-2026-09-09.md)
completed 2026-09-11 (deployed commit `e3396329`, CI run `34571343061`). All 15
acceptance items are satisfied: frozen V1 field inventory artifact published (12 classes),
version-selected V1 decode seam, writer cutover to V2 canonical desks (`management`, `engineering`, `research`),
frozen per-function mapping table, unknown live refusal, live alias retirement, engineering desk end-to-end proof,
staging decoder matrix verification, carrier coverage over object-graph objects and edges with crash-consistent
migration and write guard, historic reader compatibility, closed write roots, longest-first ID substitution,
unified verifier spelling, and physical staging proof on `computer-03335285269bdba4f94377e56879f9e6` with effects OFF.
Deployed evidence: [`evidence/choir-rlm-versioned-rename-deployed-proof-2026-09-11.md`](evidence/choir-rlm-versioned-rename-deployed-proof-2026-09-11.md).
The cutover stays remainder holder (residue R6); mission-0 live drill debt stays mission-0-owned (residue R1);
tool and operation retirement belongs to the Engineering-carrier successor mission.
## Blocked Definition — RLM Target Architecture Cutover (remainder holder)

[`archive/choir-rlm-target-architecture-cutover-2026-09-04.md`](archive/choir-rlm-target-architecture-cutover-2026-09-04.md)
is **blocked and non-executable** (dispositioned 2026-09-09 with owner topology
authority). Execution proof retained (epoch 888 exit-0 cell, proof file, signed
receipt, fence intact); run acceptance closed under completed mission 1
(2026-09-09 with deployed proof); the cutover retains no open settlement
remainder and stays remainder holder (residue R6). No action runs under
it. Do not mark complete.

## Superseded Definition — RLM Session Interpreter Cutover

[`archive/choir-rlm-session-interpreter-cutover-2026-09-02.md`](archive/choir-rlm-session-interpreter-cutover-2026-09-02.md)
is **superseded** by `choir-rlm-target-architecture-cutover-2026-09-04.md`. Its session persistence scope
is subsumed by the comprehensive RLM target architecture.
## Queued Definition — Supervised Self-Development on RLM

[`archive/choir-supervised-self-development-on-rlm-2026-09-02.md`](archive/choir-supervised-self-development-on-rlm-2026-09-02.md)
is **paused pending restore-zero and desk-rename deployed acceptance** (settlement-gate acceptance closed 2026-09-09).
Target: Candidate change A solitaire implementation authored via RLM session cells, 5-ref freeze,
qualified consensus under `reversible-selfdev-v1`, promotion, live play verification, falsification with B,
and restore to pre-A checkpoint `99949fe2`.

## Superseded Definition — Scheduling Contract and Candidate Proof

[`archive/choir-scheduling-and-candidate-proof-2026-08-21.md`](archive/choir-scheduling-and-candidate-proof-2026-08-21.md)
is **superseded** by the 3-Definition autonomous engineering sequence (Definition 1 for substrate/scheduling,
Definition 3 for candidate A solitaire proof).
## Sealed Operation — Stabilize and Hold 0333528

[`archive/choir-0333528-stabilize-and-hold-2026-08-24.md`](archive/choir-0333528-stabilize-and-hold-2026-08-24.md)
is **settled and sealed** as a historical operation. Its hold statement below
is superseded: the hold was lifted 2026-09-03 during outage recovery
(see `docs/evidence/effects-red-computer-unresolvable-after-refresh-2026-09-03.md`
and `docs/evidence/effects-repair-verification-2026-09-03.md`), and the computer
has since served active staging duty across realizations into epoch 876 with
Effects OFF. Do not treat this section as a standing hold directive.
(Original 2026-08-24 record: held host-side with guest fence, serving `:8085`,
canonical head past 133,319, as immutable evidence artifact.)

## Completed Pre-Flight & Historical Recovery

[`archive/choir-durable-substrate-preflight-2026-08-24.md`](archive/choir-durable-substrate-preflight-2026-08-24.md)
completed 2026-08-24. All four pre-flight areas are settled and verified:
(1) Dolt 2.0 embedded-driver upgrade verified; (2) `candidate-fleet-d03dacaa...` invalid-genesis
loop resolved; (3) active-guest Dolt GC policy verified with 5 GiB safe-guard; (4) live computer
`computer-03335285269bdba4f94377e56879f9e6` liveness restored.

[`archive/choir-durable-substrate-recovery-2026-08-23.md`](archive/choir-durable-substrate-recovery-2026-08-23.md)
is settled historical recovery evidence. 0333528 was recovered to canonical head 132,436
via the fixed boot/replay contract. Offline ProjectionBase rebuild was deferred to Track F.
## Completed Substrate — Tape-Based Recovery

[`archive/choir-tape-recovery-2026-08-13.md`](archive/choir-tape-recovery-2026-08-13.md)
completed 2026-08-15. Staging `4ac90583` paid all six required
receipts, including `serving_join` (unsigned host shell ≠ retained
SPA ≠ secondary SPA after vmctl resolve) and
`capability_renewal_pass` across restore without a subsequent start
(epoch 268, `store_closed: false`). Independent review of checkpoint
+ rematerialization + serving join returned ACCEPT 2026-08-15. It is
settled evidence and the restore substrate, not an executable
entrypoint. Do not rematerialize or invent `choir computer create`
to reopen it.

## Superseded Effects Definition — Historical Evidence

[`archive/choir-supervised-self-development-effects-2026-08-11.md`](archive/choir-supervised-self-development-effects-2026-08-11.md)
is superseded historical evidence. Its policy, email, and restore reasoning remain
citable historical evidence; it is not an executable entrypoint. The tape-recovery Definition
owns restore substrate receipts; it too is settled evidence, not an entrypoint. No
archived Definition is executable; the desk-RLM rectification drafts named
above are the current working set under owner review.

The scope-disjoint
[`choir-instruction-substrate-prune-2026-08-11.md`](archive/choir-instruction-substrate-prune-2026-08-11.md)
completed 2026-08-12: 106/106 beads dispositioned, doccheck signal repaired,
instruction packet pruned with invariant conservation. It is settled evidence,
not an entrypoint; do not re-open the retired beads store.

The active Definition had one owner-ratified pre-effects subordinate contract:
[`archive/choir-sandbox-autoputer-rename-2026-08-11.md`](archive/choir-sandbox-autoputer-rename-2026-08-11.md).
It owned the single clean naming cutover before effects work resumed: service
surfaces became `autoputer`, persistent computer identity surfaces became
`computer`, and no compatibility path was permitted. The cutover is complete:
the retained pre-drop replay diff is evidence for the active Definition, staging
state was recreated afterward, and the renamed product path passed deployed
acceptance. It is settled evidence, not a second entrypoint or product schedule.

The latest staging runtime/proxy deployment observed by the rename acceptance is
`3cd12d1452ad1d06b5df57cf9183313568f60cb5`; `/health` reported proxy status
OK and vmctl status OK on 2026-08-12. The earlier `914f7a5d976a` frontend/proxy
capture is historical host/source identity only. Retained guest proof and all
effects remain OFF. The residual epoch `8253` record is historical evidence,
not an open unknown on any live Definition.

Historical handoff, guest-prefix, mailbox, and terminal-boot receipts remain in
[the joined runtime review](evidence/continuous-texture-supervision-joined-runtime-review-2026-08-08.md)
and [its requirement audit](evidence/continuous-texture-supervision-requirement-audit-2026-08-08.md),
plus the disposed Mission 0 direct-key ceremony at
[`continuous-texture-supervision-direct-key-ceremony-2026-08-09.md`](evidence/continuous-texture-supervision-direct-key-ceremony-2026-08-09.md)
(do not execute it as a live gate; no headless retry, substitute identity,
recovery bypass, SSH, or weaker authorization is admissible).
They are historical evidence, not rollback or live schedule; effects remain OFF.
The active executable slice and `next_action` live solely in
[`archive/choir-sub-rlm-document-channel-2026-09-22.md`](archive/choir-sub-rlm-document-channel-2026-09-22.md).
The tape-recovery restore proof is paid (complete 2026-08-15).
Completed Definitions are historical evidence, not executable entrypoints;
receipts remain in `mission-graph.yaml` and Git history. Retained settled
receipts: `choir-tape-recovery-2026-08-13.md` (whole-computer restore
substrate); `choir-coherent-computer-convergence-2026-07-21.md` (durable-work
kernel); `choir-audited-autoputer-construction-2026-07-15.md` (audited
construction and D-ROUTE); `og-dolt-heresy-completion-2026-07-08.md` (settled
storage/D-ROUTE/H031); `choir-cli-self-development-2026-07-16.md` (incomplete
construction). Retired to Git history 2026-09-22: the autoputer-completion,
run-suite, wire-store, vocabulary, and autopaper definitions.
None is executable unless explicitly promoted in the current registry.

## Superseded — Scheduling Contract and Candidate Proof

[`archive/choir-scheduling-and-candidate-proof-2026-08-21.md`](archive/choir-scheduling-and-candidate-proof-2026-08-21.md)
is superseded by the 3-Definition autonomous engineering sequence (2026-09-02):
substrate/scheduling scope lives in Definition 1
([`choir-substrate-and-scheduling-readiness-2026-09-02.md`](archive/choir-substrate-and-scheduling-readiness-2026-09-02.md));
candidate proof scope lives in Definition 3
([`choir-supervised-self-development-on-rlm-2026-09-02.md`](archive/choir-supervised-self-development-on-rlm-2026-09-02.md)).
It is not an executable entrypoint.

## Draft Successor Definitions — Not Executable

Draft successors are blocked hypotheses, not schedules or implementation
authority. Their source Definitions and three registries retain constraints:
[`archive/choir-computerversion-performance-optimization-draft-2026-07-15.md`](archive/choir-computerversion-performance-optimization-draft-2026-07-15.md)
and [`archive/choir-in-choir-computer-control-draft-2026-07-18.md`](archive/choir-in-choir-computer-control-draft-2026-07-18.md).
Neither authorizes implementation, host access, raw vmctl, SSH, shared
credentials, candidate VMs, or promotion without separate owner ratification.

## Supporting Maintenance

Supporting maintenance Definitions retain their evidence and status:
`choir-seam-repair-2026-07-10.md` and
`documentation-authority-reduction-2026-07-09.md`. They are settled,
superseded, or historical as stated by their source Definitions, not
entrypoints.

RLM restore-zero completed 2026-09-09 and is historical evidence, not an
entrypoint. RLM versioned rename completed 2026-09-11 (deployed commit `e3396329`) and is
non-entrypoint evidence. The Sub-RLM Document Channel / Carrier Landing (M2)
(`archive/choir-sub-rlm-document-channel-2026-09-22.md`) holds the sole
working entrypoint; the M1 owner-input cutover is landed evidence.
## Unowned External Work

No Definition owns runtime dissolution, broader Wire work, external capsules,
or ComputerVersion optimization. The sequenced private-Go actor-kernel
Definition owns actor activation extraction only after its predecessor closes.
Other new work requires current evidence, owner ratification, and registry
promotion.

## Graph Rule

[`mission-graph.yaml`](mission-graph.yaml) is discovery metadata; Definitions
own state, not Git history.

## Settled Deploy Receipt

The former `running_runs: 1` blockage is settled historical evidence at
`9dff369044c2147140782958de3e91971caed6bc`; see
`docs/evidence/s1-deploy-unblock-dispatch-2026-07-12.md`. Do not rerun its
topology; document any recurrence as a new problem with promoted authority.
