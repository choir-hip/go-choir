# MissionGradient: Alternate Computer UX Experiment Portfolio v0

**Status:** checkpoint_incomplete; Wave 0 package/adoption checkpoint proven,
Wave 1 Chiron/animation owner-pullable package/adoption checkpoint proven,
Wave 2 Liquid/Python owner-pullable package/adoption checkpoint proven,
run-acceptance bridge patch prepared, duplicate-package quality gaps remain
**Date:** 2026-05-20
**Operator:** Codex-operated MissionGradient supervisor using Choir-in-Choir
candidate/background computers where healthy
**State ledger:** [platform-os-app-state.md](platform-os-app-state.md)
**Liquid design input:** [choir-liquid-material-engine-design-v0.md](choir-liquid-material-engine-design-v0.md)
**Python code mode input:** `/Users/wiz/Downloads/choir_python_code_mode_experiment.md`
**Preflight gate:**
[mission-promotion-substrate-preflight-hard-cutover-v0.md](mission-promotion-substrate-preflight-hard-cutover-v0.md)

> Preflight is complete as of `52e0612`, with deployed substrate proof at
> `98b73c5`. Wave 0 was reproven on deployed staging at `d1f3bb5`.
> The experiment portfolio must use the current
> AppChangePackage -> adoption -> recipient build -> promote/rollback path.
> It must not use `export_patchset`, `/api/promotions`, or synthetic recipient
> digests as acceptable evidence.
>
> Direct owner login to alternate experiment accounts is out of scope. It is
> not an acceptance path for this portfolio. The review invariant is package
> mobility: an experiment computer publishes a product-visible
> AppChangePackage, the owner inspects the evidence, then pulls, adopts, or
> promotes the package into an owner-controlled computer for manual QA,
> iteration, promotion, or rejection. Do not add auth handoff machinery merely
> to make Playwright-created source accounts directly loginable. The owner may
> review all promising experiments by pulling their packages into one existing
> owner-controlled account/computer.
> Package mobility is the auth solution for this portfolio: manual QA should
> happen by pulling or adopting experiment packages into an account the owner
> already controls, such as `ymnath@choir-ip.com`, not by making every source
> experiment account manually loginable. A worker-local package summary in
> Trace is evidence, but it is not yet owner-reviewable until the same
> AppChangePackage is visible through product package/adoption APIs.

## One-Line Goal String

```text
/goal Run docs/mission-alternate-computer-ux-experiment-portfolio-v0.md as a Codex-operated MissionGradient mission: create an owner-reviewable alternate-computer experiment portfolio, not a platform-default UX merge. Start from the deployed 74230a3 package-mobility substrate and the Wave 1/Wave 2 checkpoints where Chiron Shelf, process/window animation, Choir Liquid Material Engine, and Python code mode A/B all produced product-visible AppChangePackages that were pulled, verified, and promoted in recipient computers. Continue with the quality/evidence pass required before calling the portfolio complete: deploy and reprove the source-run plus recipient-run run-acceptance bridge so owner-pullable package/adoption evidence is reflected instead of remaining staging-smoke blocked, eliminate or precisely explain duplicate package identities, and produce a concise owner review certificate for all four packages with Trace/VText/run-acceptance refs, screenshots or Playwright video refs, benchmarks where relevant, rollback refs, residual risks, and promotion recommendations. The owner review path is product-visible package publish -> owner pull/adoption/promotion into an existing owner-controlled account/computer, including one of the owner's current accounts; direct login to alternate experiment accounts is out of scope and is not an auth work item. Maintain a concise learning log about MissionGradient behavior during the run: where persistence helped, where it overreached, where evidence gates prevented false success, where concurrency changed outcomes, and what should be simplified later. Do not use the learning log as permission to stop early. Do not copy binaries between computers, fake reviewability with labels, use platform deploy as proof of user-computer divergence, use export_patchset or /api/promotions, add auth-handoff machinery just for experiment QA, capture private DOM into liquid materials, hide prompt/Shelf controls behind animation, add Python beside bash within the same candidate profile, or claim completion while run-acceptance or duplicate-package evidence contradicts clean success. If a substrate blocker prevents owner-pullable packages or real package/adoption evidence, root-cause it, patch through git/CI/deploy when authorized, then continue; otherwise report blocked_incomplete with exact evidence and the next executable probe.
```

## Mission Frame

Choir needs a research mode where ambitious UI/runtime ideas can be expressed in
real user computers before platform promotion. The target is not one merged
feature. The target is an experiment portfolio the owner can inspect the next
day by reviewing package/adoption evidence, pulling promising packages into an
owner-controlled account/computer when useful, comparing results, and choosing
what should move toward promotion. Direct login to distinct experiment accounts
is not part of the QA path and must not become a mission subproblem.

Manual owner review should therefore look like:

```text
experiment computer produces package evidence
-> owner inspects package/Trace/VText/run-acceptance refs
-> owner pulls, adopts, or promotes selected package into an owner-controlled account/computer
-> owner tests that adopted package in their own active/candidate context
-> owner decides iterate, abandon, or promote
```

This route is better aligned with the promotion architecture than trying to
transfer alternate-account login state. Alternate accounts may be created by
Playwright or worker flows, but their loginability is not the product proof.
The proof is that the patch can move as source/package evidence and be adopted
inside a computer the owner can already access.

The owner QA path may be centralized. Multiple experiment packages can be
pulled into the same owner-controlled account/computer as separate candidate or
adoption attempts. The mission should preserve source experiment identities,
package identities, verifier evidence, and rollback refs, but it should not
spend effort on making every source account a place the owner can manually log
into.

### Auth And Review Decision

The auth solution for this portfolio is package mobility, not alternate-account
login. The owner does not need to log into the source experiment accounts. A
successful experiment publishes a product-visible AppChangePackage that can be
inspected, pulled, adopted, promoted, or rejected from an owner-controlled
computer such as `ymnath@choir-ip.com`.

Direct login to source experiment accounts is therefore a non-goal. It is not
evidence of success and it must not consume mission time. Auth work belongs in
this mission only when the package/adoption/promotion path itself cannot
function without it.

The four experiments are:

1. **Chiron Shelf observability:** the Shelf streams tool calls, interim agent
   status, and major run events as readable ambient motion without blocking the
   Desk, app buttons, or prompt bar.
2. **Process/window/agent animation language:** boot, wake, app launch, window
   raise/minimize/restore, candidate work, and live sync transitions become
   legible through tasteful state motion.
3. **Choir Liquid Material Engine:** a custom GPU-backed shell material based
   on the design draft, using WebGL-first synthetic material fields and avoiding
   live private DOM capture.
4. **Python code mode A/B:** a candidate super/vsuper/co-super profile family
   replaces bash with a minimal Python execution primitive and benchmarks token
   use, tool-loop iterations, time, quality, and traceability.

The mission is allowed to use platform substrate deploys only to repair or
bootstrap the experiment-control path. The experiment outcomes themselves
should live in alternate/candidate computers, not be smuggled into the platform
default as if the research had already been promoted.

The promotion substrate perspective after preflight is:

```text
candidate computer work
-> AppChangePackage with source deltas/contracts/provenance
-> recipient computer adoption
-> recipient-specific Go/Svelte build evidence
-> verifier result
-> owner/platform decision
-> promote/rollback
```

Every experiment should preserve this topology even when a low-resolution proof
is necessary. A screenshot or local patch is useful evidence only when it is
attached to the package/adoption chain or to a precise blocker explaining why
that chain could not be reached.

## Real Artifact

The artifact is:

```text
experiment computer/package portfolio
-> four candidate/user-computer lanes
-> product-visible state and evidence for each lane
-> AppChangePackage/adoption/build evidence where code changed
-> owner pull/adoption/promotion path into an owner-controlled account/computer
-> owner-reviewable screenshots/video/benchmarks
-> Trace/VText/run-acceptance/certificate records
-> promotion recommendations and rollback/package/adoption refs
```

The artifact is not:

- a single platform UX branch with four half-finished features;
- a local-only prototype;
- screenshots without an owner-pullable package or product evidence;
- a library bake-off detached from Choir's shell;
- fake computer/package labels with no way for the owner to inspect or adopt
  the result;
- a platform-default promotion without owner review.

## Invariants

- Experiments are isolated by user/candidate computer. One experiment must not
  silently contaminate another experiment's source, state, files, prompts, or
  artifacts.
- The owner must be able to inspect each successful experiment after the run by
  product evidence and, for code-changing lanes, by an AppChangePackage that
  can be pulled, adopted, or promoted into an owner-controlled account/computer.
  Direct account login is out of scope and must not become auth-surface scope
  unless it is needed for the package/adoption/promotion path itself.
- Alternate experiment accounts may be non-loginable to the owner. That is not
  a blocker when package refs, source deltas, verifier evidence, screenshots or
  videos, and owner pull/adoption/promotion records are durable and
  inspectable.
- Platform deploys are allowed for substrate repairs, but not as proof that a
  user-computer experiment succeeded.
- Patch movement uses the current hard-cut path: AppChangePackage -> adoption
  -> actual recipient build -> verifier evidence -> promote/rollback. The old
  `export_patchset`, `/api/promotions`, and synthetic recipient digest paths are
  invalid acceptance evidence.
- Mutable app/platform work should be delegated through Choir-in-Choir
  worker/candidate flows when healthy. If that substrate blocks progress,
  root-cause and repair directly through git/CI/deploy inside authority.
- No experiment may degrade logged-out read/explore, auth-on-mutation, desktop
  state persistence, mobile overlapping-window behavior, or active computer
  recovery.
- All proof uses product paths: visible staging desktop, Trace, VText,
  run-acceptance, screenshots/video, product APIs, and owner-inspectable
  experiment packages/computers.
- Do not expose host/global telemetry in browser UI. Performance evidence must
  be product-safe and scoped to the user's computer/experiment.
- Incomplete work must be reported as `checkpoint_incomplete` or
  `blocked_incomplete`, not as success.

## Value Criterion

Maximize:

```text
owner-reviewable learning per experiment
+ product-path observability
+ evidence durability
+ cross-device/mobile realism
+ isolation between experimental computers
+ promotion decision quality
```

while minimizing:

```text
platform-default risk
+ fake completion
+ hidden resource cost
+ private-state capture
+ account/login ambiguity
+ local-only demos
+ animation that reduces task clarity
```

The mission moves uphill when tomorrow's review can answer:

```text
What does this experiment feel like?
What did it cost?
What broke?
What evidence backs the claims?
Should it be abandoned, iterated, user-selectable, or promoted?
```

## Quality Gradient

Target quality: **solid experiment infrastructure** and **excellent evidence
honesty**.

Solid means:

- each experiment has a clear computer/package identity and owner review path;
- evidence is preserved in VText/Trace/run-acceptance, not only chat logs;
- screenshots/video are captured for visual lanes;
- benchmark numbers are captured for liquid and Python lanes;
- code-changing lanes publish AppChangePackages or record the precise blocker
  before claiming reviewability;
- failures include exact blocker, root-cause probes, rollback refs, and next
  executable probe;
- mission docs are updated at checkpoint or completion.

Substandard work:

- treating "prototype exists locally" as an experiment computer;
- merging visual experiments to platform default without owner review;
- proving liquid material only in desktop Chromium;
- making animations decorative without state semantics;
- making Chiron block input or duplicate Trace without improving observability;
- adding Python as another tool beside bash, which invalidates the A/B.

## Wave 0: Substrate And Review-Path Gate

Before starting visual/runtime experiments, prove the experiment portfolio can
produce owner-reviewable alternate-computer package/adoption evidence without
falling back to the old promotion substrate.

Wave 0 must establish:

- current staging identity and preflight semantics are visible;
- the current package/adoption path can preserve distinct source and target
  computer identity without relying on direct login to the experiment account;
- candidate work will occur in candidate computers, not by mutating active
  computers directly;
- at least one tiny no-op or label-level AppChangePackage/adoption/rebuild path
  is observed if the product path supports it;
- if the package/adoption path cannot be exercised before the experiments, the
  blocker is recorded before any lane claims owner-reviewable package evidence.

Wave 0 is allowed to patch substrate through git/CI/deploy only when the missing
piece is a platform primitive required to create reviewable experiment
computers or package/adoption evidence. It must not use a platform deploy to
simulate the result of a user-computer experiment. It should not add auth
handoff complexity just to make virtual Playwright passkey accounts manually
loginable.

## Experiment Lanes

### Lane A: Chiron Shelf Observability

Real artifact:

```text
Shelf/prompt band
-> ambient Chiron stream from product live/Trace events
-> prompt focus suppresses Chiron text
-> Desk/app buttons/prompt stay interactive
-> Chiron history is inspectable through Trace or a product-linked log
```

Starting code seams:

- `frontend/src/lib/BottomBar.svelte` owns Shelf, prompt, app buttons, Desk menu,
  and live status.
- `frontend/src/lib/live-events.js` dispatches `/api/ws` product events to the
  browser.
- `internal/runtime/tools.go` emits `tool.invoked` and `tool.result` events.
- `internal/runtime/toolloop.go` emits tool-loop progress.
- Trace trajectory SSE already exists through `frontend/src/lib/trace.js`.

Acceptance evidence:

- Playwright video showing a prompt that calls super, while Chiron streams
  status/tool events without blocking Shelf or prompt interaction;
- Chiron hides or quiets when prompt input is focused;
- Trace or product evidence links to the underlying events;
- mobile `390x844` and desktop screenshots.

Forbidden shortcuts:

- a fake ticker with random text;
- Chiron messages not backed by live/run/Trace evidence;
- stealing pointer events from Shelf controls;
- hiding Trace failures behind a pretty stream.

### Lane B: Process, Window, And Agent Animation Language

Real artifact:

```text
state transition vocabulary
-> boot/wake/live-channel/restore motion
-> app launch/raise/minimize/restore motion
-> candidate/worker/package/adoption/promotion motion
-> live sync activity motion
-> reduced-motion fallback
```

Starting code seams:

- `frontend/src/lib/Desktop.svelte` already records boot lines and prompt
  status.
- `frontend/src/lib/FloatingWindow.svelte` already uses transform-based
  Overview motion.
- Desktop Overview already animates real windows spatially.
- live events can drive status pulses.

Acceptance evidence:

- Playwright video of boot/wake, app launch, multi-window overview, prompt
  execution, and candidate/worker status where available;
- reduced-motion mode proof;
- no regression in window raise/minimize/restore or mobile overlapping desktop;
- screenshots plus DOM metrics.

Forbidden shortcuts:

- decorative shimmer detached from state;
- constant motion that distracts from VText, Trace, media, or prompt work;
- hiding slow operations behind ambiguous spinners;
- breaking reduced-motion preferences.

### Lane C: Choir Liquid Material Engine

Real artifact:

```text
custom WebGL-first shell material prototype
-> one renderer context
-> owned synthetic material field
-> registered shell surfaces
-> DOM controls/text above GPU material
-> mobile Safari and desktop proof
```

Use [choir-liquid-material-engine-design-v0.md](choir-liquid-material-engine-design-v0.md)
as the design contract.

Acceptance evidence:

- mobile Safari or Playwright WebKit screenshots/video where possible;
- desktop Chromium screenshots/video;
- FPS/frame-time/resource comparison against CSS-only material;
- WebGL context count;
- heavy session proof with many windows;
- reduced transparency/reduced motion fallback;
- explicit recommendation: abandon, iterate, user-selectable, or promote after
  another proof loop.

Forbidden shortcuts:

- relying on `liquid-dom`/HTML-in-Canvas as mobile Safari proof;
- private app DOM capture into GPU textures;
- one GPU context per window/app;
- full-window liquid over readers/media/VText/Trace;
- claiming "GPU performance" without measurements.

### Lane D: Python Code Mode A/B

Real artifact:

```text
candidate profile family
-> super/vsuper/co-super use python instead of bash
-> same roles, prompts, delegation semantics, and worker topology
-> benchmark against existing bash family
-> traceable tool execution and token/time metrics
```

Design input:

- `/Users/wiz/Downloads/choir_python_code_mode_experiment.md`

Core constraints:

- Python replaces bash in the candidate profile family. It is not added beside
  bash.
- Model-facing schema should stay minimal, ideally `{ "code": "string" }`.
- Runtime owns cwd, network, timeout, environment, output caps, and safety.
- The same foreground mutation guard used by bash must apply.
- Trace must preserve code hash/full code availability, capped output,
  duration, exit status, changed files, and git status.

Acceptance evidence:

- A/B benchmark task set with same model/provider/profile semantics except tool
  mode;
- metrics for provider calls, tool-loop iterations, tokens, wall-clock time,
  tool execution time, package/adoption success, verification success,
  debugability, and foreground-mutation violations;
- Trace/run-acceptance evidence for each benchmark run;
- recommendation: abandon, iterate, or expand to another proof loop.

Forbidden shortcuts:

- exposing both bash and Python to the same candidate and treating model choice
  as the experiment;
- using Python to bypass authority/mutation guards;
- claiming token savings without measured baseline;
- adding recursive/model-calling code mode in this mission.

## Concurrency Policy

Start with two lanes in parallel, not four.

Recommended wave order:

```text
Wave 0: experiment computer/package creation, review path, and package/adoption smoke
Wave 1: Lane A Chiron + Lane B animation language
Wave 2: Lane C liquid material + Lane D Python code mode
```

If Wave 1 shows healthy account isolation, Trace evidence, and worker/candidate
throughput, continue with Wave 2 concurrently.

If concurrency causes:

- worker/vsuper timeouts;
- ambiguous computer/package identity;
- missing Trace/run/package/adoption evidence;
- cross-experiment contamination;
- staging instability;
- unbounded cost/resource pressure;

then regress to sequential lanes and record the concurrency blocker.

Concurrency itself is part of the experiment, but not at the cost of evidence
integrity.

## Package And Review Path

The mission must produce one of these outcomes for each lane:

```text
owner_pullable_experiment
checkpoint_package
blocked_incomplete
```

`owner_pullable_experiment` means:

- the experiment was built in an isolated user/candidate computer;
- an AppChangePackage exists with source deltas/contracts/provenance;
- an owner-controlled recipient computer, including one of the owner's existing
  accounts/computers, can inspect, pull, import, build, verify, adopt, promote,
  or reject it through product APIs;
- the owner does not need direct login to the source experiment account in order
  to inspect, test, or promote the result;
- evidence docs name the source computer, package, adoption/build, verifier,
  and rollback refs without leaking secrets.

Direct source-account login is a non-goal, not a portfolio outcome. It must not
be used as a substitute for package mobility and must not add auth work to this
portfolio.

`checkpoint_package` means:

- a package/adoption candidate and evidence are preserved for review, but the
  full owner-pull/adoption/promotion path has not been proven for that lane;
- the blocker to full owner pull/adoption/promotion is precise and has a next
  probe.

`blocked_incomplete` means:

- the lane could not reach a reviewable artifact after root-cause probes and
  cognitive transforms;
- no fake screenshots or labels substitute for the missing artifact.

Do not store or print reusable credentials in the mission doc. If current auth
requires passkeys or operator-mediated setup, do not widen auth solely for this
mission. Prefer package publish -> owner pull/adoption/promotion into an
owner-controlled account/computer. Pulling several packages into one owner
account is acceptable and preferable to auth work when the package identities
and rollback refs remain distinct. Record the exact missing capability only when
that package path itself is blocked.

## Dense Feedback

Every lane should produce:

- product-path screenshots or Playwright video;
- Trace trajectory IDs where agent/work events exist;
- VText report in the experiment computer or platform review document;
- run-acceptance or equivalent evidence synthesis;
- changed source/package/adoption refs where code changed;
- rollback refs;
- final recommendation and residual risk.

Lane-specific dense feedback:

- Chiron: prompt interaction video, event provenance, prompt focus behavior.
- Animations: transition video, reduced-motion proof, multi-window/mobile proof.
- Liquid: Safari/WebKit proof, WebGL context count, frame timing, memory/restore
  weight evidence.
- Python mode: baseline/candidate benchmark table with token/time/tool-loop
  metrics.

## Learning Side-Channel

Maintain a concise run log specifically for MissionGradient learning. This log
is an observation artifact, not a stopping condition and not a substitute for
portfolio evidence.

Record entries when the run reveals something about the mission method:

```text
timestamp:
lane or substrate:
observed situation:
mission-gradient pressure:
decision taken:
evidence produced:
cost/risk:
learning:
possible future skill simplification:
```

Useful observations include:

- persistence that produced real evidence rather than premature stopping;
- persistence that created churn or overreach;
- evidence gates that prevented false success;
- concurrency that improved throughput or degraded isolation;
- places where the mission wording was too dense, stale, or ambiguous;
- moments where a blocker became executable after investigation;
- moments where an experiment should have remained a checkpoint rather than a
  claimed success.

Do not use the learning log as permission to stop early. The default remains:
continue, redirect, delegate, or repair inside current authority until the
portfolio reaches reviewable evidence or a precise blocker.

## Forbidden Shortcuts

- Platform deploy as proof of user-computer divergence.
- Local-only screenshots as final evidence.
- Fake account/computer/package labels that the owner cannot inspect, pull, or
  adopt.
- Auth handoff or credential machinery added only to make virtual experiment
  accounts manually loginable.
- `export_patchset`, `/api/promotions`, or synthetic recipient digest evidence.
- Fake Chiron text disconnected from product events.
- Decorative animation that hides state or breaks controls.
- `liquid-dom` or Chrome-only HTML-in-Canvas accepted as mobile Safari proof.
- Private DOM/content capture into liquid material textures.
- One GPU context per app/window.
- Python added beside bash instead of replacing bash in a candidate profile
  family.
- Test/internal route shortcuts for acceptance records.
- Claiming `complete` when any lane lacks `owner_pullable_experiment` or
  `checkpoint_package` evidence.

## Stopping Condition

The mission is `complete` only when all four lanes reach one of these reviewable
outcomes and the portfolio report is durable:

```text
Lane A: owner_pullable_experiment or checkpoint_package with evidence
Lane B: owner_pullable_experiment or checkpoint_package with evidence
Lane C: owner_pullable_experiment or checkpoint_package with evidence
Lane D: owner_pullable_experiment or checkpoint_package with evidence
portfolio report: VText/docs/Trace/run-acceptance refs, recommendations, rollback refs
learning log: concise MissionGradient observations and future simplification notes
```

This is intentionally not "all four promoted." The mission is research through
real computers and movable source packages. Completion means the owner can
review, pull/adopt where appropriate, and decide.

Use `checkpoint_incomplete` if meaningful progress exists but one or more lanes
still lack reviewable evidence and further safe probes remain.

Use `blocked_incomplete` only after root-cause probes and 2-5 route-changing
cognitive transforms fail to expose a safe next move inside current authority.

## Run Checkpoint And Resumption State

Latest checkpoint:

```text
status: checkpoint_incomplete
last checkpoint: Wave 1 two-lane prompt-bar probe reran on deployed staging at
  74230a3 after the package visibility/pull-path fix, then Wave 2 ran the
  remaining Liquid and Python lanes through the same staging product path.
  All four experiment lanes now have product-visible AppChangePackages that
  were pulled, verified, and promoted in recipient computers. The portfolio is
  still `checkpoint_incomplete` because run-acceptance records for Wave 2 are
  staging-smoke-level/blocked and duplicate package publication recurred.
current artifact state: four-lane experiment portfolio exists as owner-pullable
  package/adoption evidence. Wave 0 proved AppChangePackage -> adoption ->
  actual recipient Go/Svelte build -> promote through staging product APIs.
  Wave 1 added Chiron and animation packages. Wave 2 added Liquid Material
  Engine and Python code mode packages. The review invariant is owner-pullable
  packages, not loginable Playwright-created accounts. Source experiment
  accounts may be disposable or non-loginable; the package must be inspectable,
  pullable, adoptable, or promotable into an owner-controlled computer.
review-path decision: direct owner login to alternate accounts is out of scope
  for this portfolio and is not an acceptance path.
  Experiments should publish product-visible AppChangePackages that the owner
  can inspect, pull, adopt, promote, or reject inside an owner-controlled
  account/computer such as ymnath@choir-ip.com. Do not add auth handoff code
  solely for manual QA of virtual Playwright accounts. The owner may review many
  experiments by pulling their packages into one current owner account as
  separate candidate/adoption attempts; keep package/source/rollback identity
  distinct rather than making per-experiment manual login the review mechanism.
what shipped: preflight substrate hard-cut landed before this mission; during
  Wave 0, a run-acceptance false-success edge was identified and patched so
  records with blocked invariant checks cannot still claim accepted state.
  No auth handoff code should be added for this mission. Docs were updated
  to make owner-pull/adoption/promotion the review invariant. Commit a0a4d6c makes
  `delegate_worker_vm` mark package-required vsuper results without package
  evidence as `worker_run_incomplete` with blocker
  `vsuper_completed_without_required_app_change_package`, and tells super not
  to summarize package/candidate delegate results without `app_change_packages`
  as completed owner-reviewable work. Commit f11e848 propagates auto-chained
  `delegate_worker_vm` package/blocker fields onto the top-level
  `request_worker_vm` result, maps worker Git candidate branches into
  `source_ledger_candidate_ref` while allowing canonical product
  `candidate_source_ref` generation, and hardens run-status tests that had
  timing-sensitive sleeps. Commit 74230a3 adds the product-safe worker package
  visibility path: worker-published AppChangePackages are fetched through
  internal worker runtime package endpoints, mirrored into the active runtime
  store, and made pullable across computers through
  `/api/app-change-packages/pull`. It also updates the mission doc to make
  package mobility, not alternate-account login, the owner QA path.
what was proven:
  - old export_patchset and /api/promotions paths are invalid acceptance paths
  - current acceptance path is AppChangePackage/adoption/recipient build
  - staging package/adoption proof at deployed commit d1f3bb5 produced:
    package-alt-portfolio-wave0-1779270395944
    adoption-alt-portfolio-wave0-1779270395944
    target recipient runtime digest
      sha256:0ea79e13b92a4562392e514e53e170f3505c23d2bfb88666b6ebe06d155dda51
    target recipient UI digest
      sha256:b5cc68456c76598faa7d267f546ded558531cbd114e0a94cde2f3c445aa81519
    trace traj-alt-portfolio-wave0-1779270395944
    run acceptance runacc-19c10e4b57c2f0828c5b
    VText evidence doc ab093136-d504-4745-8f42-d9d30a008bdc
  - Wave 1 concurrent prompt-bar probe at marker
    alt-portfolio-wave1-1779272533882 created two source-computer submissions:
    Chiron submission 5f9db432-87d6-477a-ba10-26ccc0825641
    Animation submission 180525db-2944-4b08-b453-8a321ff709cf
  - both Wave 1 submissions reached completed prompt state and opened VText
    docs, with Trace mobile summaries:
    Chiron: 3 agents, 128 moments, 11 messages
    Animation: 2 agents, 26 moments, 2 messages
  - Wave 1 used no forbidden browser requests and wrote durable VText evidence:
    doc e7e927ec-09fa-404a-8df8-b2e4973a6f89
    revision b47d8471-13d6-4331-a6e2-66a38b67baf3
  - diagnostic Wave 1 probe at deployed commit a0a4d6c with marker
    alt-portfolio-wave1-1779279580320 captured Trace moment details:
    Chiron produced implementation commits 48d2d11959f6fe0fe02a4dfd8ce710d034f5ab7f
    and 17f4b9ca7d25be99cf709f42308b408603dfe35b, verifier PASS evidence,
    and repeated `publish_app_change_package` failures:
    `candidate_source_ref must be a candidate ref`.
    Evidence file:
    test-results/alternate-portfolio-wave1-diagnostics-a0a4d6c-20260520T1216/alternate-portfolio-wave1-evidence.json
  - deployed Wave 1 probe at f11e848 with marker
    alt-portfolio-wave1-1779282590489 ran for 40.5 minutes and honestly failed
    the product-visibility assertion because no lane became non-blocked through
    `/api/app-change-packages`
  - the same f11e848 evidence contains worker-local/Trace-visible package
    summaries:
    Chiron package 690de1f0-5153-4213-8baf-5c4ac08d6302 for
      portfolio-chiron-shelf-alt-portfolio-wave1-1779282590489
    duplicate Chiron package 956e38fa-727a-4a4f-9f30-f49d61130b2b for the same
      lane, which violates the one-package identity expectation
    Animation package e0f9ed77-b54f-4811-bf9f-8e72b80bdd23 for
      portfolio-animation-language-alt-portfolio-wave1-1779282590489
  - staging identity for f11e848 was verified through `/health`, with CI run
    26164257461 and Node B deploy success before the Wave 1 probe
  - deployed Wave 1 owner-pullable proof at 74230a3 passed on staging in
    29.4 minutes, using marker alt-portfolio-wave1-1779288477047 and evidence:
    test-results/alternate-portfolio-wave1-deployed-74230a3-20260520T1447/alternate-portfolio-wave1-evidence.json
  - Chiron Shelf observability produced product-visible package
    f2fe930b-6477-4146-ac9c-b5161c193dea for app
    portfolio-chiron-shelf-alt-portfolio-wave1-1779288477047, status
    published_unlisted, manifest
    91138f28ff1433618f92de9234512cccdbf78d1cf4661928cdb2670eba1a7479.
    Recipient adoption adoption-owner-review-chiron-alt-portfolio-wave1-1779288477047
    verified and promoted with status 200.
  - Process/window/agent animation produced product-visible package
    4fb2d440-6db4-4c7e-8154-291fafaa0047 for app
    portfolio-animation-language-alt-portfolio-wave1-1779288477047, status
    published_unlisted, manifest
    776a4264bc29400d28d74f3a8cf0fb296d6ea51586cd6676fa606126954a27b7.
    Recipient adoption adoption-owner-review-animation-alt-portfolio-wave1-1779288477047
    verified and promoted with status 200. A second animation package
    8ea326a4-2343-42e7-8edd-64ca3d9890dc was also published for the same lane,
    so duplicate package identity remains a quality issue even though the owner
    pull/adoption path passed.
  - Wave 1 74230a3 used no forbidden browser requests, wrote durable VText
    evidence doc ef3c2192-0352-406e-aa4c-048fbbc4a633 and revision
    1b9f77f5-6af4-4709-a86c-5a8c7a283878, and recorded deployed_commit
    74230a388ce1d2ff947867ae8059c86dee0b3086 in the evidence packet.
  - deployed Wave 2 owner-pullable proof at 74230a3 passed twice on staging.
    The stronger run-acceptance-enabled proof passed in 29.6 minutes, using
    marker alt-portfolio-wave2-1779292734577 and evidence:
    test-results/alternate-portfolio-wave2-deployed-74230a3-runacc-20260520T1558/alternate-portfolio-wave2-evidence.json
  - Choir Liquid Material Engine produced product-visible package
    6f059706-5cec-46e9-834a-3f2ead7a3455 for app
    portfolio-liquid-material-alt-portfolio-wave2-1779292734577, status
    published_unlisted, manifest
    344402c0e45467b6fb17945c270d7e57743ed79c9c64951db21bf55b7ef3d0c5.
    Recipient adoption adoption-owner-review-liquid-alt-portfolio-wave2-1779292734577
    verified and promoted with status 200. The package contains a UI source
    delta adding a WebGL-first LiquidMaterialShell, benchmark harness data
    attributes such as data-frame-cost-ms and data-webgl-context-count, reduced
    motion/transparency fallback, and explicit no-private-DOM-capture safety.
  - Python code mode A/B produced product-visible package
    9aebc4af-13fd-4d22-8906-676041c47450 for app
    portfolio-python-code-mode-alt-portfolio-wave2-1779292734577, status
    published_unlisted, manifest
    16e2f5eec04b20e02e9c3a4aef14b73b5773295862039cf82f487dfdbd6665bb.
    Recipient adoption adoption-owner-review-python-alt-portfolio-wave2-1779292734577
    verified and promoted with status 200. A second Python package
    e157058d-6ce8-4929-8935-518c5cec27df was also published for the same lane,
    so duplicate package identity remains a quality issue. The package contains
    runtime source deltas for a Python execution profile family and benchmark
    scaffolding for token use, tool-loop iterations, wall-clock time, tool
    execution time, traceability, changed files, and foreground mutation safety.
  - Wave 2 74230a3 used no forbidden browser requests, wrote durable VText
    evidence doc 87726ca2-3949-4358-bd69-ba36b2e1b4da and revision
    3a917a5f-53d2-4518-a2f4-956fc604d205, and recorded deployed_commit
    74230a388ce1d2ff947867ae8059c86dee0b3086 in the evidence packet.
  - Wave 2 synthesized RunAcceptanceRecords:
    Liquid runacc-2ca4fd849d5eaba99727, staging-smoke-level, blocked by
    worker_mutation_bounded even though package/adoption evidence exists.
    Python runacc-ce09fa718d843d6aed27, staging-smoke-level, blocked, with
    submitted/vtext/super/worker/app_package_published checkpoints. This shows
    run-acceptance synthesis still does not understand owner-pullable package
    adoption across source and recipient accounts well enough to call the lane
    accepted.
  - local quality pass after the Wave 2 checkpoint prepared a run-acceptance
    bridge patch: direct product AppChangePackage/adoption events now satisfy
    product-path and bounded-mutation invariants when verifier and rollback
    evidence is trace-derived, and the portfolio harness now records both
    source-run acceptance and recipient-run acceptance after owner pull,
    verification, and promotion. Focused runtime acceptance/promotion tests
    passed inside the repo Nix dev shell:
    `nix develop .# --command go test -count=1 ./internal/runtime -run 'TestRunAcceptanceSynthesize|TestAppPromotion|TestPrivateAppChangePackage|TestAppAdoption'`.
    A broader `./internal/runtime` run was started but stopped after several
    minutes without output; keep verification scoped until a timed full-suite
    runner is available.
  - focused local regression tests pass inside the repo Nix dev shell:
    nix develop .# --command go test -count=1 ./internal/runtime -run
    'TestDelegateWorkerVM(FollowsCompletedVSuperChildrenBeforeReturning|MarksCompletedVSuperWithoutExportOrUpdateIncomplete|MarksPackageRequiredVSuperWithoutPackageIncomplete|ReturnsFailedRunEvidence|ReturnsTimeoutRunEvidence)'
  - exact CI shard 0 was reproduced locally inside `nix develop .#`; running
    the same Dolt-backed tests outside the dev shell still fails on the known
    ICU header path, so worker/candidate environments that run those tests must
    enter the repo dev shell or equivalent build environment
  - session renewal is required across long recipient builds because access
    cookies are intentionally short-lived and refresh cookies are scoped to
    /auth
  - virtual-authenticator passkeys are not manually transferable, but that is
    no longer a mission blocker when package pull/adoption/promotion evidence is durable
unproven or partial claims:
  - owner-pull/adoption/promotion into ymnath@choir-ip.com specifically; the
    74230a3 proofs used newly registered recipient product accounts, which are
    acceptable substrate evidence but not yet the owner's personal QA pull
  - exactly one migrating AppChangePackage identity per lane; animation
    produced a duplicate product-visible package during Wave 1 and Python
    produced a duplicate product-visible package during Wave 2
  - Choir-in-Choir two-lane concurrency reached owner-pull/adoption for all four
    lanes, but run-acceptance synthesis remains blocked/too strict for
    source-account package publication plus recipient-account adoption
  - mobile Safari liquid material feasibility; the package contains a
    WebGL-first prototype and benchmark hooks, not a real mobile Safari manual
    review
  - Python mode A/B contains candidate implementation and benchmark
    scaffolding, but not yet a completed measured A/B table across real runs
  - deployed proof that recipient-side run acceptance reaches promotion-level
    accepted after owner pull/adoption/promotion; the local bridge patch must
    still be committed, deployed, and reproven on staging
remaining error field:
  - duplicate package publication/export behavior remains and should be
    corrected if it recurs before the portfolio claims clean one-lane package
    identity
  - persistent super serializes concurrent lane work enough that animation can
    lag behind Chiron under bounded proof windows
  - owner pull/adoption/promotion UX and evidence path for tomorrow's manual QA remains
    unproven specifically for ymnath@choir-ip.com, though it is proven through
    a fresh recipient product account
  - run-acceptance evidence synthesis has a local bridge patch but no deployed
    staging proof yet
highest-impact remaining uncertainty:
  - Can the deployed bridge produce a clean owner-review certificate where the
    four package lanes are deduplicated, their source/recipient run-acceptance
    evidence is connected across computers, and the owner can decide
    iterate/abandon/promote without reading raw traces?
next executable probe:
  - Commit/push/deploy the run-acceptance bridge patch, verify staging
    identity, rerun the product-path portfolio proof with recipient-side
    acceptance synthesis, and confirm that owner-pullable package/adoption
    lanes no longer remain staging-smoke blocked. Then investigate duplicate
    package publication in animation/Python lanes enough to either eliminate it
    on a fresh run or precisely classify it as duplicate delegate/tool-call
    behavior with a single primary package per lane. Finish with a concise owner
    review certificate covering all four packages, their Trace, VText,
    source/recipient run-acceptance, adoption, rollback, screenshot/video,
    benchmark, and promotion recommendation refs.
suggested resume goal string:
  - Use the One-Line Goal String in this document.
evidence artifact refs:
  - test-results/alternate-portfolio-wave0-deployed/alternate-portfolio-wave0-evidence.json
  - frontend/test-results/alternate-computer-portfol-682da--or-records-precise-blocker-chromium/alternate-portfolio-wave0-desktop.png
  - test-results/alternate-portfolio-wave1-deployed/alternate-portfolio-wave1-evidence.json
  - test-results/alternate-portfolio-wave1-diagnostics-a0a4d6c-20260520T1216/alternate-portfolio-wave1-evidence.json
  - test-results/alternate-portfolio-wave1-deployed-f11e848-20260520T1310/alternate-portfolio-wave1-evidence.json
  - test-results/alternate-portfolio-wave1-deployed-74230a3-20260520T1447/alternate-portfolio-wave1-evidence.json
  - test-results/alternate-portfolio-wave2-deployed-74230a3-20260520T1525/alternate-portfolio-wave2-evidence.json
  - test-results/alternate-portfolio-wave2-deployed-74230a3-runacc-20260520T1558/alternate-portfolio-wave2-evidence.json
  - frontend/test-results/alternate-computer-portfol-9bf24-s-package-adoption-evidence-chromium/alternate-portfolio-wave1-source-desktop.png
  - frontend/test-results/alternate-computer-portfol-9bf24-s-package-adoption-evidence-chromium/trace.zip
rollback refs:
  - previous_active_source_ref
    refs/computers/target-computer-alt-portfolio-wave0-1779270395944/active-foreground-tail-alt-portfolio-wave0-1779270395944
  - Wave 1 74230a3 Chiron rollback refs:
    adoption-owner-review-chiron-alt-portfolio-wave1-1779288477047
    previous_active_source_ref
    refs/computers/owner-review-chiron-alt-portfolio-wave1-1779288477047/active-foreground-tail-alt-portfolio-wave1-1779288477047
  - Wave 1 74230a3 animation rollback refs:
    adoption-owner-review-animation-alt-portfolio-wave1-1779288477047
    previous_active_source_ref
    refs/computers/owner-review-animation-alt-portfolio-wave1-1779288477047/active-foreground-tail-alt-portfolio-wave1-1779288477047
  - Wave 2 74230a3 Liquid rollback refs:
    adoption-owner-review-liquid-alt-portfolio-wave2-1779292734577
    previous_active_source_ref
    refs/computers/owner-review-liquid-alt-portfolio-wave2-1779292734577/active-foreground-tail-alt-portfolio-wave2-1779292734577
  - Wave 2 74230a3 Python rollback refs:
    adoption-owner-review-python-alt-portfolio-wave2-1779292734577
    previous_active_source_ref
    refs/computers/owner-review-python-alt-portfolio-wave2-1779292734577/active-foreground-tail-alt-portfolio-wave2-1779292734577
learning log:
  - Evidence gates prevented a fake direct-login claim: Playwright passkey
    accounts are real product accounts but their credentials are trapped in the
    virtual authenticator.
  - User clarified the better architecture: review experiments by package
    publish -> owner pull/adoption/promotion, not by adding temporary auth handoff code.
  - Evidence gates also found a run-acceptance false-success edge: promotion
    checkpoints alone were enough to claim accepted even when invariant checks
    were blocked.
  - Wave 1 concurrency was useful because it exposed the real next substrate
    gap: both lanes could be submitted and observed, but no lane crossed from
    VText/Super activity into AppChangePackage publication.
  - Root-cause sharpened from "VText/Super did not delegate" to "delegation can
    still launder package-required work into completed narrative updates when
    no AppChangePackage was published."
  - Diagnostic detail capture changed the search space: Chiron was not stuck at
    implementation, verification, or delegation. It was blocked at the boundary
    between worker Git refs and product source-lineage refs.
  - The f11e848 rerun changed the blocker again: package publication can happen
    inside worker/vsuper, but owner-reviewability fails if the package remains
    worker-local. The right auth solution is still package mobility; the next
    substrate fix should make the package product-visible, not make source
    experiment accounts loginable.
  - The 74230a3 rerun validated that package mobility is enough for auth/review:
    the owner does not need source-account login when a package can be made
    product-visible, pulled into another product account, verified, promoted,
    and reported with rollback refs.
  - Wave 2 showed the same product path can carry more ambitious experiments,
    including UI/WebGL and runtime/tooling candidates, but it also exposed a
    more subtle evidence gap: run acceptance can remain blocked even after
    product package/adoption proof succeeds, because acceptance synthesis is
    still centered on source trajectory evidence rather than the cross-computer
    adoption chain.
  - The better acceptance shape is not one overloaded record. A source-run
    acceptance record should prove prompt/VText/super/worker/package evidence;
    a recipient-run acceptance record should prove pull/adoption/build/verify/
    promote/rollback evidence. The owner review certificate can connect both
    without requiring direct login to source experiment accounts.
```
