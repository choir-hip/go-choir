# MissionGradient: Trace Provenance UX Hillclimb v0

Status: ready for execution
Date: 2026-05-16
Operator: outer Codex supervising Choir through staging, git, CI, deploy, Playwright, Trace, VText, and worker-VM evidence

## One-Line Goal String

```text
/goal Run docs/mission-trace-provenance-ux-hillclimb-v0.md as a Codex-operated MissionGradient mission: use investigate and cognitive-transform discipline to turn the export-level worker substrate at e27dcea into promotion-ready UX-improvement capacity. Start from the deployed proof trajectory 45b29971-c388-464e-9824-1373d18dad06 and root-cause the remaining evidence/readability gaps: VText truncates implementation/verifier provenance, worker state is reported as unknown despite worker_run_completed, Trace and run acceptance do not yet make full child-agent/channel/manifest/patch/rollback/verification evidence easy to inspect on desktop and mobile, and continuation-level proof is absent. Patch the implicated Trace API/UI, VText worker-update synthesis, run-acceptance synthesis, prompt contracts, tests, or diagnostics so full worker/vsuper/co-super provenance and export artifacts are durable, linked, and readable; commit, push main, monitor CI/deploy, verify staging identity, and rerun product-path proof. Then use the visible staging prompt bar to run a narrow Choir-in-Choir UX workload through super -> worker VM -> vsuper -> implementation/verifier co-super channels, prioritizing Trace readability, prompt-bar/window ergonomics, logged-out read/explore usability, and auth-on-mutation clarity. Continue receding-horizon investigate -> fix -> deploy -> product-proof loops until VText, Trace, screenshots/DOM metrics, run acceptance, and candidate/export/promotion or precise blocker evidence show a real UX improvement or a hard blocker after named root-cause probes; forbid fake-island placeholders, fake transclusion panels, internal/test acceptance bypasses, local-only proof for platform claims, and summaries that launder missing evidence into success.
```

## Mission Shift

The previous mission recovered the substrate far enough to prove:

```text
visible staging prompt bar
-> VText
-> super
-> request_worker_vm
-> delegate_worker_vm(profile=vsuper)
-> worker VM
-> vsuper
-> implementation/verifier co-super topology over channels
-> export_patchset
-> promotion candidate queued
-> run acceptance at export-level
```

The next obstacle is no longer raw worker liveness. The next obstacle is that
the successful run is still too hard for a product user, verifier, or owner to
understand and promote confidently. The same path should now become a UX
improvement engine, not an infrastructure cul-de-sac.

This mission therefore has two coupled targets:

1. make the existing worker/vsuper/co-super/export evidence fully durable,
   inspectable, and readable in VText, Trace, and run acceptance;
2. use that improved evidence substrate to make or precisely block one small
   real UX improvement, with Trace readability and prompt-bar/window ergonomics
   as the preferred workload.

## Research Basis

Latest deployed baseline:

- commit: `e27dceaab8420309812b605a11591a63d61c2369`
- CI run: `25972150467`, success
- staging health: proxy and sandbox both reported deployed commit
  `e27dceaab8420309812b605a11591a63d61c2369`
- evidence directory:
  `.gstack/evidence/worker-vm-liveness-export-checkpoint-e27dcea-2026-05-16T20-39-04Z`

Successful product-path proof:

- marker: `WORKER_VM_LIVENESS_1778963953084`
- trajectory: `45b29971-c388-464e-9824-1373d18dad06`
- VText doc: `71b39a59-57d6-4d29-99c1-6e696ee6884b`
- super loop: `7637f7bd-1c3c-4d6c-83ad-5fbe9f1e7222`
- worker: `worker-f1fbdaee823d2d61`
- worker VM: `vm-eca69b736a14d80ea56775bfb55e5375`
- worker sandbox URL: `http://172.102.0.2:8085`
- delegate loop: `a0d3299f-3327-4cf2-bcff-cd45b66f83e7`
- delegate status: `worker_run_completed`
- implementation child run: `db1f9f92-0c0d-416e-83d2-fb0f1a087edb`
- verifier child run: `7d5cdc20-7484-4010-b692-66fbe01b915a`
- duplicate/cancel child run: `7d1bf56b-40ef-46ed-a485-64e819968a2f`
- worker event count: `59`
- worker channel messages: `6`
- exported patchset:
  `/mnt/persistent/files/worker_exports/WORKER_VM_LIVENESS_1778963953084/changes.patch`
- export manifest:
  `/mnt/persistent/files/worker_exports/WORKER_VM_LIVENESS_1778963953084/manifest.json`
- base SHA: `e27dceaab8420309812b605a11591a63d61c2369`
- worker head SHA: `dc687852c764cad04817c233f1719956e8291eb4`
- promotion candidate: `dc2932a7-8e60-46d5-8acf-459e27159ebc`
- run acceptance: `runacc-d3885e4882ac1a8e0376`, state `accepted`,
  level `export-level`

What improved:

- worker VM runtime stayed healthy through delegation;
- `delegate_worker_vm` collected child run IDs and child event counts;
- export patchset and promotion candidate were returned through structured
  parent evidence;
- the old false VText text saying there were no export patchsets did not recur;
- final worker health reported runtime `ready`, provider `gateway`, and
  `running_runs: 0`.

Remaining gaps observed in the same proof:

- VText still reported `worker state: unknown` even though the delegate output
  state was `completed`.
- VText had to say the implementation/verifier provenance was truncated before
  the full verifier id and rationale.
- Run acceptance reached `export-level`, not `promotion-level` or
  `continuation-level`.
- VText did not directly inspect or embed manifest JSON and patchset content;
  it relied on super-reported extraction.
- The verifier acknowledged waiting for implementation evidence, but the final
  user-facing proof does not yet expose a clear independent verifier rationale.
- The Trace surface contains the details, but the human/product surface still
  makes the owner work too hard to inspect agent topology, child IDs, channel
  messages, artifacts, rollback refs, and verifier conclusions, especially on
  mobile.

## Real Artifact

The artifact is a deployed, promotion-readable self-development loop:

```text
logged-out/readable staging desktop
-> auth only when mutation or model/search/worker action requires it
-> visible natural-language prompt bar
-> VText mission/report
-> Trace-visible conductor -> vtext -> super
-> super leases worker VM
-> delegate_worker_vm(profile=vsuper, timeout budget preserved)
-> worker VM vsuper coordinates implementation and verifier co-super agents
-> channel messages and child run IDs are durable and inspectable
-> implementation produces reviewable candidate/export evidence
-> verifier produces independent pass/fail rationale
-> VText integrates a complete, non-placeholder report
-> Trace makes the causal chain readable on desktop and mobile
-> run acceptance synthesizes export/promotion/continuation level honestly
-> owner can promote, discard, or continue with rollback refs
```

The UX improvement workload is not separate from the substrate. It is the
pressure test for whether Choir can improve itself through the intended product
path.

## Invariants

- Staging `https://draft.choir-ip.com` is the acceptance environment for
  platform behavior, worker VM, auth, gateway/model calls, Trace, VText,
  promotion, rollback, and run acceptance claims.
- Platform behavior changes complete:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed acceptance proof
```

- Public/browser acceptance uses visible product surfaces and public
  authenticated product APIs only: `/api/prompt-bar`,
  `/api/prompt-bar/submissions/*`, `/api/vtext/*`, `/api/trace/*`,
  `/api/promotions/*`, `/api/continuations/*`, and
  `/api/run-acceptances/*`.
- Browser-public acceptance must not use `/api/agent/*`, `/api/prompts`,
  `/api/test/*`, `/internal/*`, raw event mutation endpoints, direct service
  ports, or manually seeded success records.
- Internal diagnostics may use vmctl, worker sandbox health, Node B logs,
  runtime run records, and direct worker endpoints. Diagnostics guide fixes but
  do not substitute for product-path acceptance.
- Logged-out users keep read/explore usability. Auth is required only for
  mutation, owned state, model/search/worker-backed actions, publishing,
  uploads, candidate computers, or promotion.
- Super may do bounded ephemeral diagnostics and low-risk scratch work. Durable
  Choir app/harness/repo/runtime/candidate/export/promotion mutation must go
  through worker VM/vsuper or outer Codex platform fixes through git/CI/deploy.
- Vsuper owns candidate-world coordination. It should coordinate one
  implementation co-super and one verifier co-super over channels unless a
  precise blocker is recorded.
- Worker/co-super/verifier evidence must be real runtime evidence. No fake
  channel transcripts, fake export refs, fake promotion records, fake
  transclusion panels, or decorative placeholders.
- Trace is an acceptance surface, not a hidden debug log. If it cannot explain
  the run, that is a product blocker.
- VText must not launder uncertainty into success. Missing evidence should be
  named precisely with the next safe probe.

## Value Criterion

Minimize:

```text
provenance loss + Trace readability friction + UX regression risk
+ hidden state + verifier Goodharting + fake-island pressure
```

while preserving the worker/candidate/promotion authority boundaries.

The run moves uphill when:

- the same product-path proof exposes full worker/vsuper/co-super topology in
  Trace and VText without truncation-dependent gaps;
- `delegate_worker_vm` and run acceptance expose state, child run IDs, agent
  IDs, channel IDs, verifier rationale, export manifest, patch summary,
  promotion candidate, and rollback refs as durable structured evidence;
- Trace on desktop and mobile lets a user inspect trajectory, agents, moments,
  messages, tool calls, exports, and promotion refs without overlap, tiny text,
  unusable scroll traps, or ambiguous labels;
- the first real UX candidate is small, reviewable, exported or promoted only
  with evidence, and improves a real user-facing surface;
- failed attempts leave actionable diagnostics and do not erase logged-out
  usability.

The run moves downhill when:

- a cleaner VText summary is produced without fixing the missing structured
  evidence path;
- acceptance level is inflated without verifier/owner/promotion evidence;
- Trace gets prettier but less causally informative;
- the worker path is bypassed to land UX changes directly as a false
  self-development proof;
- fake panels, fake citations, or fake transclusions are used to satisfy a
  visual expectation.

## Quality Gradient

Expected quality: `solid`.

Solid means:

- focused fixes with tests at the layer that failed;
- no parallel fake evidence model;
- UI changes preserve dense technical readability without marketing-style
  decoration;
- mobile and desktop screenshots or DOM metrics prove the reading surface;
- product-path acceptance is repeated after deploy;
- final report states exact evidence, rollback refs, residual risks, and the
  next realism axis.

Substandard work:

- local-only screenshots for deployed UX claims;
- styling-only Trace changes that do not improve inspection of the latest
  trajectory;
- adding text that describes evidence instead of linking or preserving it;
- accepting `export-level` as if it were `promotion-level`;
- weakening logged-out UX or auth-on-mutation boundaries.

## Homotopy Parameters

Increase realism along these axes without changing topology:

- Evidence granularity: summary strings -> structured refs -> linked detail
  cards -> verifier-ready promotion record.
- Acceptance level: export-level -> promotion-review-level -> promotion-level
  -> continuation-level.
- Trace viewport: desktop only -> desktop plus mobile -> narrow mobile with
  long payloads and active worker events.
- Workload realism: marker patch -> narrow Trace/prompt UX patch -> owner
  reviewed promotion -> continuation of the same mission.
- Agent topology: vsuper plus child IDs -> full channel transcript and
  verifier rationale -> bounded repair loop.
- Artifact inspection: manifest path -> manifest JSON excerpt -> patch summary
  -> diff/rollback affordance.
- Runtime stress: single run -> rerun with warm worker/dedupe -> concurrent
  worker or longer budget after evidence backpressure is stable.

## Starting Belief State

Believed current state:

- Worker VM liveness is recovered enough for one deployed export-level proof.
- The structured delegate output contains most of the needed provenance.
- VText and run acceptance preserve enough for export-level, but not enough for
  owner-friendly promotion or continuation.
- Trace can fetch details through `/api/trace/*`, but the UI and summarized
  surfaces still make provenance hard to inspect, especially for mobile and
  long worker payloads.
- UX/onboarding work can now be a legitimate test workload, but only if it goes
  through the worker/vsuper boundary or through outer Codex platform fixes with
  deploy proof.

Evidence:

- `runacc-d3885e4882ac1a8e0376` accepted at export-level.
- VText final head `de29db24-cba4-484f-9b90-1cd41f8f84ba` matched the export
  paths and SHA values.
- Worker health snapshots showed deployed commit `e27dcea...`, runtime
  `ready`, gateway active, and no running runs at the end.
- VText final still named provenance gaps rather than full verifier rationale.

Main uncertainties:

- Which layer should own full implementation/verifier provenance for promotion
  readability: `delegate_worker_vm`, worker-update synthesis, Trace artifacts,
  run acceptance, or all of them with different projections?
- Is `worker state: unknown` a mapping bug, an output schema mismatch, or a
  legitimate unknown state from the worker endpoint?
- Does Trace mobile fail because of layout, payload length, inspector
  placement, window geometry, or missing affordances?
- Can vsuper reliably wake the verifier after implementation export, or does
  the verifier currently only acknowledge waiting?
- What is the smallest real UX patch that should be attempted first through the
  self-development path without overfitting to one proof script?

Highest-impact uncertainty:

```text
Where is the nearest fix that turns existing successful worker evidence into a
promotion-readable user/verifier surface without bypassing the worker/candidate
boundary?
```

Next observation:

Replay or inspect the latest trajectory in Trace on desktop and mobile, select
the `delegate_worker_vm` moment, and compare what is visible in the UI against
the structured event payload, final VText report, worker-update record, and run
acceptance record.

## Investigation And Cognitive Reframing

Use the investigate skill loop before each nontrivial fix:

1. Investigate: inspect product Trace, VText revisions, worker-update records,
   run acceptance JSON, screenshots, DOM metrics, and relevant source.
2. Analyze: classify whether the gap is missing data, truncated data, bad UI
   layout, prompt contract failure, run acceptance omission, or verifier
   coordination failure.
3. Hypothesize: name the layer and the exact evidence expected after a fix.
4. Implement: patch the smallest implicated layer with tests and deployed proof.

Selected cognitive transforms:

1. User-facing verifier surface: Trace and VText are not just logs; they are
   the owner review UI. This changes the verifier from "JSON contains it" to
   "a user can inspect it and decide promote/discard."
2. Causal carrier split: preserve the same evidence across different
   projections. `delegate_worker_vm` should carry machine detail, VText should
   carry narrative plus refs, Trace should carry inspectable causal detail, and
   run acceptance should carry promotion-level criteria.
3. Anti-Goodhart: do not make acceptance look better by moving assertions into
   the proof harness. Acceptance must derive from durable run/trace/promotion
   evidence.
4. UX hill-climb: each substrate fix should reduce a real user friction: reading
   Trace, understanding worker evidence, deciding whether to promote, or using
   the prompt bar/window shell.
5. Receding-horizon autonomy: if a blocker names a safe next probe inside the
   current authority boundary, run it before ending.

Route-changing consequences:

- implementation: favor structured provenance and Trace/VText UI improvements
  over more prompt text;
- verifier/evidence: require screenshots or DOM metrics for readability, plus
  JSON assertions for provenance;
- scope: start with the smallest surface that makes the latest successful run
  inspectable, then attempt a real UX candidate;
- stopping: only stop on success, invariant/external boundary, or a hard blocker
  after root-cause probes and cognitive reframing.

## Receding-Horizon Control

Operate in short control intervals:

1. Choose the highest-information gap from the latest proof.
2. Predict the exact VText/Trace/run-acceptance/screenshot delta.
3. Patch one layer or add one diagnostic surface.
4. Run focused tests.
5. Commit, push, monitor CI, monitor staging deploy, verify health identity.
6. Rerun product-path Playwright proof.
7. Update belief state and either continue, narrow, or stop.

Suggested first intervals:

1. Provenance gap replay:
   - open latest trajectory in Trace on desktop and mobile;
   - inspect `delegate_worker_vm` detail;
   - identify which fields exist in payload but are not user-readable.
2. Structured provenance fix:
   - ensure worker-update synthesis carries full child run IDs, agent IDs,
     channel ID, export paths, base/head SHAs, verifier rationale if present,
     and state `completed`;
   - ensure run acceptance records the same fields where relevant.
3. Trace readability fix:
   - make the latest worker/export/provenance detail readable on desktop and
     mobile, including long payloads and artifact refs.
4. UX candidate run:
   - submit a narrow self-development prompt through staging;
   - require worker VM/vsuper/co-super implementation and verifier evidence;
   - accept export/promotion or precise blocker only.

## Dense Feedback Channels

- `git status`, `git diff --check`, focused tests, broad runtime/store tests
  where touched.
- GitHub Actions run and deploy job results.
- Staging `/health` commit identity for proxy and sandbox.
- Deployed Playwright screenshots/video from visible prompt-bar proof.
- Trace snapshot and moment detail for the same trajectory.
- VText document revisions and metadata for worker updates consumed/skipped.
- Run acceptance records and verifier contracts.
- Worker VM health and vmctl/log diagnostics when worker behavior is implicated.
- Browser request audit proving forbidden public routes were not used for
  acceptance.
- Mobile and desktop DOM metrics for Trace readability: no horizontal overflow,
  readable selected moment, reachable inspector, visible artifact refs, no text
  overlap with prompt bar or window chrome.

## Evidence Ledger Format

For every nontrivial claim, record:

```text
claim:
evidence source:
command or observation:
artifact path:
result:
uncertainty/caveat:
promotion relevance:
```

The final report must name:

- pushed commit SHA;
- CI run and deploy status;
- staging health/build identity;
- deployed acceptance command and result;
- prompt submission / trajectory / VText doc / revision ids;
- worker id, VM id, delegate loop id, child run ids, channel ids;
- export manifest path, patchset path, base SHA, worker head SHA;
- promotion candidate id or precise blocker;
- screenshots and/or DOM metrics for Trace/UX;
- run acceptance id and level;
- verifier contracts and evidence refs;
- rollback refs;
- residual risks and next realism axis.

## UX Workload Priority

After provenance readability is coherent enough to verify the run, attempt the
smallest real UX improvement that stays within the mission topology.

Preferred order:

1. Trace readability for latest worker/export trajectory:
   - desktop and mobile layout;
   - clear agent topology;
   - selected moment detail;
   - channel messages;
   - export/promotion artifacts;
   - rollback refs.
2. Prompt-bar/window ergonomics:
   - prompt bar remains usable with windows open;
   - mobile windows do not hide controls or important text;
   - foreground reading/explore remains available logged out.
3. Auth-on-mutation clarity:
   - login required only at mutation/model/search/worker boundaries;
   - intent is preserved after authentication.
4. Initial onboarding through VText-native artifact:
   - no marketing landing page;
   - no fake transclusion or placeholder proof.

## Inner Choir Prompt For UX Candidate

Submit through the visible staging prompt bar only after provenance/readability
fixes are deployed enough to inspect the result:

```text
Create a VText mission report and safely improve one small part of Choir's user experience without changing the active computer directly. This is Choir app/harness/repo/candidate work, so VText should ask super to lease a worker VM and delegate to vsuper. Vsuper should coordinate exactly one implementation co-super and one verifier co-super over channels. Prioritize a narrow Trace readability or prompt-bar/window ergonomics improvement that helps a user inspect the latest worker/export trajectory on desktop and mobile. The implementation co-super should make the smallest reviewable repo change, run focused tests or a screenshot/DOM proof, and export a patchset. The verifier co-super should independently inspect the evidence and report pass/fail with rationale. The final VText report must include trajectory id, VText revision ids, worker VM id, child run ids, channel ids, verifier conclusion, tests/screenshots/DOM metrics, export manifest path, patchset path, base SHA, worker head SHA, promotion or rollback refs, residual risks, and next objective. Do not use fake transclusion panels, decorative placeholders, internal/test acceptance routes, raw event mutation, local-only platform proof, or canonical mutation without verified promotion. If blocked, record the precise blocker and the next safe probe.
```

## Forbidden Shortcuts

- Do not use `/internal/*`, `/api/test/*`, `/api/agent/*`, `/api/prompts`, raw
  event mutation, direct service ports, or manually seeded records as public
  acceptance.
- Do not bypass worker/vsuper for the inner UX candidate and then claim
  Choir-in-Choir self-development.
- Do not replace missing structured provenance with prose that sounds complete.
- Do not add fake transclusion panels, fake provenance cards, fake promotion
  records, fake verifier summaries, or static placeholders.
- Do not claim mobile readability without mobile evidence.
- Do not claim promotion-level without verifier contract evidence plus owner
  review and promotion/rollback evidence.
- Do not claim continuation-level without run-memory compaction and continuation
  evidence.
- Do not weaken logged-out read/explore usability while improving mutation UX.

## Rollback Policy

- Platform changes roll back by git revert or fix-forward on `origin/main`,
  followed by CI, staging deploy, health identity, and deployed acceptance.
- Candidate/worker changes remain inside candidate/worker VM boundaries until
  exported, verified, owner-reviewed, and promoted.
- Unpromoted candidates are rollback-safe by discard/archive. Record candidate
  id, base SHA, worker head, patchset SHA, manifest path, and promotion state.
- If Trace/VText UI changes regress readability, revert the commit or add a
  corrective commit and rerun desktop/mobile proof.
- If provenance changes break acceptance synthesis, stop promotion claims and
  preserve the broken run's evidence before patching.

## Learning Side-Channel

Classify discoveries:

- Tactical learning: patch tests, source, or proof runner during the mission.
- Target-level learning: update this mission or define a narrower next mission
  if the UX workload should change.
- Invariant-level learning: stop and escalate before changing authority,
  product-path, or promotion semantics.

Record durable learning in:

- mission final report;
- focused regression tests;
- relevant docs if the contract changes;
- Trace/VText/run acceptance artifacts for the proof trajectory.

Do not leave strategic learning only in chat.

## Stopping Conditions

Stop successfully only when:

- latest behavior-changing commit is pushed to `origin/main`;
- CI and staging deploy have completed;
- staging health reports the expected commit;
- deployed product-path proof has run through the visible prompt bar;
- VText, Trace, and run acceptance preserve full enough provenance to inspect
  worker/vsuper/co-super/export evidence without fake placeholders;
- Trace readability is proven on desktop and mobile or precisely blocked with
  screenshots/DOM metrics and next safe probe;
- a real UX candidate is exported/promoted, or the blocker is precise after
  named root-cause probes;
- rollback refs and residual risks are named.

Stop on blocker only when:

- continuing would cross an external or invariant boundary;
- a destructive/risky mutation needs owner review;
- the next probe would repeat an already-falsified path without new evidence;
- or the mission has run the relevant diagnostics and can name a hard blocker
  with durable refs.

If the final report can name an executable safe next probe inside the mission's
authority boundary, run that probe instead of ending.
