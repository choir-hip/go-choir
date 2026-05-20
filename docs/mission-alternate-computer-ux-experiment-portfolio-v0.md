# MissionGradient: Alternate Computer UX Experiment Portfolio v0

**Status:** draft; preflight gate complete
**Date:** 2026-05-20
**Operator:** Codex-operated MissionGradient supervisor using Choir-in-Choir
candidate/background computers where healthy
**State ledger:** [platform-os-app-state.md](platform-os-app-state.md)
**Liquid design input:** [choir-liquid-material-engine-design-v0.md](choir-liquid-material-engine-design-v0.md)
**Python code mode input:** `/Users/wiz/Downloads/choir_python_code_mode_experiment.md`
**Preflight gate:**
[mission-promotion-substrate-preflight-hard-cutover-v0.md](mission-promotion-substrate-preflight-hard-cutover-v0.md)

> Preflight is complete as of `52e0612`, with deployed substrate proof at
> `98b73c5`. The experiment portfolio must use the current
> AppChangePackage -> adoption -> recipient build -> promote/rollback path.
> It must not use `export_patchset`, `/api/promotions`, or synthetic recipient
> digests as acceptable evidence.

## One-Line Goal String

```text
/goal Run docs/mission-alternate-computer-ux-experiment-portfolio-v0.md as a Codex-operated MissionGradient mission: create a loginable alternate-computer experiment portfolio, not a platform-default UX merge. Use Choir-in-Choir with two-lane concurrency where the substrate is healthy, regressing to sequential only when evidence/account isolation degrades. First prove the current AppChangePackage -> adoption -> recipient build substrate with an owner-reviewable Wave 0 account/computer path, then produce four reviewable experiment computers: Chiron Shelf observability, process/window/agent animation language, custom Choir Liquid Material Engine, and Python code mode A/B. Keep each experiment in a user/candidate computer with product-path Trace/VText/run-acceptance evidence, screenshots or Playwright video, benchmarks where relevant, rollback/package/adoption refs, and a promotion recommendation. Maintain a concise learning log about MissionGradient behavior during the run: where persistence helped, where it overreached, where evidence gates prevented false success, where concurrency changed outcomes, and what should be simplified later. Do not use the learning log as permission to stop early. Do not copy binaries between computers, fake loginable accounts, use platform deploy as proof of user-computer divergence, use export_patchset or /api/promotions, capture private DOM into liquid materials, hide prompt/Shelf controls behind animation, add Python beside bash instead of replacing it in the candidate profile family, or claim completion without durable evidence the owner can inspect tomorrow. If a substrate blocker prevents loginable alternate computers or real package/adoption evidence, root-cause it, patch through git/CI/deploy when authorized, then continue; otherwise report blocked_incomplete with exact evidence and the next executable probe.
```

## Mission Frame

Choir needs a research mode where ambitious UI/runtime ideas can be expressed in
real user computers before platform promotion. The target is not one merged
feature. The target is an experiment portfolio the owner can inspect the next
day by logging into distinct computers/accounts, comparing evidence, and
choosing what should move toward promotion.

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
experiment account/computer portfolio
-> four candidate/user-computer lanes
-> product-visible state and evidence for each lane
-> AppChangePackage/adoption/build evidence where code changed
-> owner-reviewable screenshots/video/benchmarks
-> Trace/VText/run-acceptance/certificate records
-> promotion recommendations and rollback/package/adoption refs
```

The artifact is not:

- a single platform UX branch with four half-finished features;
- a local-only prototype;
- screenshots without a loginable computer or product evidence;
- a library bake-off detached from Choir's shell;
- fake account labels with no way for the owner to inspect the result;
- a platform-default promotion without owner review.

## Invariants

- Experiments are isolated by user/candidate computer. One experiment must not
  silently contaminate another experiment's source, state, files, prompts, or
  artifacts.
- The owner must be able to inspect each successful experiment after the run.
  If current auth/passkey/account mechanics make this impossible, the run must
  precisely isolate that blocker rather than fake loginability.
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
  experiment computers.
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

- each experiment has a clear computer/account identity and review path;
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
produce owner-reviewable alternate-computer evidence without falling back to the
old promotion substrate.

Wave 0 must establish:

- current staging identity and preflight semantics are visible;
- each intended experiment has a distinct owner-reviewable account/computer or
  a precise blocker to loginability;
- candidate work will occur in candidate computers, not by mutating active
  computers directly;
- at least one tiny no-op or label-level AppChangePackage/adoption/rebuild path
  is observed if the product path supports it;
- if the package/adoption path cannot be exercised before the experiments, the
  blocker is recorded before any lane claims `loginable_experiment`.

Wave 0 is allowed to patch substrate through git/CI/deploy only when the missing
piece is a platform primitive required to create reviewable experiment
computers or package/adoption evidence. It must not use a platform deploy to
simulate the result of a user-computer experiment.

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
Wave 0: account/computer creation, review path, and package/adoption smoke
Wave 1: Lane A Chiron + Lane B animation language
Wave 2: Lane C liquid material + Lane D Python code mode
```

If Wave 1 shows healthy account isolation, Trace evidence, and worker/candidate
throughput, continue with Wave 2 concurrently.

If concurrency causes:

- worker/vsuper timeouts;
- ambiguous account identity;
- missing Trace/run/package/adoption evidence;
- cross-experiment contamination;
- staging instability;
- unbounded cost/resource pressure;

then regress to sequential lanes and record the concurrency blocker.

Concurrency itself is part of the experiment, but not at the cost of evidence
integrity.

## Account And Review Path

The mission must produce one of these outcomes for each lane:

```text
loginable_experiment
checkpoint_package
blocked_incomplete
```

`loginable_experiment` means:

- the owner can log into the experiment account/computer tomorrow;
- the experiment UI/runtime state is present there;
- evidence docs name the account/computer identity without leaking secrets;
- rollback/package/adoption refs are recorded.

`checkpoint_package` means:

- loginability was not achieved, but an AppChangePackage/adoption candidate and
  evidence are preserved for review;
- the blocker to loginability is precise and has a next probe.

`blocked_incomplete` means:

- the lane could not reach a reviewable artifact after root-cause probes and
  cognitive transforms;
- no fake screenshots or labels substitute for the missing artifact.

Do not store or print reusable credentials in the mission doc. If current auth
requires passkeys or operator-mediated setup, record the approved review path
or the exact missing capability.

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
- Fake account/computer labels that the owner cannot inspect.
- `export_patchset`, `/api/promotions`, or synthetic recipient digest evidence.
- Fake Chiron text disconnected from product events.
- Decorative animation that hides state or breaks controls.
- `liquid-dom` or Chrome-only HTML-in-Canvas accepted as mobile Safari proof.
- Private DOM/content capture into liquid material textures.
- One GPU context per app/window.
- Python added beside bash instead of replacing bash in a candidate profile
  family.
- Test/internal route shortcuts for acceptance records.
- Claiming `complete` when any lane lacks `loginable_experiment` or
  `checkpoint_package` evidence.

## Stopping Condition

The mission is `complete` only when all four lanes reach one of these reviewable
outcomes and the portfolio report is durable:

```text
Lane A: loginable_experiment or checkpoint_package with evidence
Lane B: loginable_experiment or checkpoint_package with evidence
Lane C: loginable_experiment or checkpoint_package with evidence
Lane D: loginable_experiment or checkpoint_package with evidence
portfolio report: VText/docs/Trace/run-acceptance refs, recommendations, rollback refs
learning log: concise MissionGradient observations and future simplification notes
```

This is intentionally not "all four promoted." The mission is research through
real computers. Completion means the owner can review and decide.

Use `checkpoint_incomplete` if meaningful progress exists but one or more lanes
still lack reviewable evidence and further safe probes remain.

Use `blocked_incomplete` only after root-cause probes and 2-5 route-changing
cognitive transforms fail to expose a safe next move inside current authority.

## Run Checkpoint And Resumption State

Latest checkpoint:

```text
status: checkpoint_incomplete
last checkpoint: Wave 0 package/adoption proof reached product-path recipient
  build and promotion evidence, but not owner-loginable experiment accounts
current artifact state: four-lane experiment portfolio defined; Wave 0 proved
  AppChangePackage -> adoption -> actual recipient Go/Svelte build -> promote
  through staging product APIs
what shipped: preflight substrate hard-cut landed before this mission; during
  Wave 0, a run-acceptance false-success edge was identified and patched so
  records with blocked invariant checks cannot still claim accepted state
what was proven:
  - old export_patchset and /api/promotions paths are invalid acceptance paths
  - current acceptance path is AppChangePackage/adoption/recipient build
  - staging package/adoption proof at deployed commit 98b73c5 produced:
    package-alt-portfolio-wave0-1779268905976
    adoption-alt-portfolio-wave0-1779268905976
    target recipient runtime digest
      sha256:60b5479f7b1adf6bb9ecb61b0b9b3707dc917b66141fc4cb532c313be294f0ee
    target recipient UI digest
      sha256:b5cc68456c76598faa7d267f546ded558531cbd114e0a94cde2f3c445aa81519
    trace traj-alt-portfolio-wave0-1779268905976
    run acceptance runacc-48e62a7bac080d7da68a
  - session renewal is required across long recipient builds because access
    cookies are intentionally short-lived and refresh cookies are scoped to
    /auth
unproven or partial claims:
  - account/loginable experiment creation path
  - Choir-in-Choir two-lane concurrency under current substrate
  - mobile Safari liquid material feasibility
  - Python mode A/B implementation and benchmark
  - run acceptance after the invariant-state patch must be redeployed and
    reproven on staging; the previous acceptance record exposed the false edge
    by returning accepted while product_path_observed and
    worker_mutation_bounded were blocked
remaining error field:
  - experiment account/passkey review path
  - worker/candidate throughput under concurrent lanes
  - evidence synthesis across alternate computers
  - owner-reviewable account enrollment or package-pull flow for tomorrow's
    manual QA accounts
highest-impact remaining uncertainty:
  - Can Choir create owner-reviewable alternate experiment computers without
    falling back to platform-default deploys?
next executable probe:
  - Land and deploy the run-acceptance invariant-state patch, rerun Wave 0, and
    require status checkpoint_package until a true owner-loginable alternate
    account/computer review path exists. Then launch Wave 1 lanes in two-lane
    concurrency only if account isolation evidence remains clear.
suggested resume goal string:
  - Use the One-Line Goal String in this document.
evidence artifact refs:
  - test-results/alternate-portfolio-wave0/alternate-portfolio-wave0-evidence.json
  - frontend/test-results/alternate-computer-portfol-682da--or-records-precise-blocker-chromium/alternate-portfolio-wave0-desktop.png
rollback refs:
  - previous_active_source_ref
    refs/computers/target-computer-alt-portfolio-wave0-1779268905976/active-foreground-tail-alt-portfolio-wave0-1779268905976
learning log:
  - Evidence gates prevented a fake loginable_experiment claim: Playwright
    passkey accounts are real product accounts but their credentials are trapped
    in the virtual authenticator.
  - Evidence gates also found a run-acceptance false-success edge: promotion
    checkpoints alone were enough to claim accepted even when invariant checks
    were blocked.
```
