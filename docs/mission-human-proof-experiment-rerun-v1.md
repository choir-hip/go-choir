# MissionGradient: Human-Proof Experiment Rerun v1

**Status:** ready resume mission after `b11ed4f` staging verification
**Date:** 2026-05-23
**Supersedes:** [mission-human-proof-experiment-rerun-v0.md](mission-human-proof-experiment-rerun-v0.md)
**Depends on:** [mission-async-supervision-runtime-hardening-v0.md](mission-async-supervision-runtime-hardening-v0.md)
**State ledger:** [platform-os-app-state.md](platform-os-app-state.md)

## One-Line Goal String

```text
/goal Run docs/mission-human-proof-experiment-rerun-v1.md as a Codex-supervised MissionGradient mission: finish the async supervision/source-transfer hardening, then rerun the four experiments sequentially through Choir-in-Choir. First prove Chyron Shelf Observability end to end: Choir agents must build the candidate change, publish an honest AppChangePackage as the transferable source artifact, attach owner-readable VText narrative plus screenshot/video human proof, verify recipient build/adoption/rollback through product APIs, and expose readable Trace/run-acceptance evidence. A package may be evidence_pending, but it must not be reviewable until human proof exists. Codex may patch runtime/harness/prompts/evidence plumbing but must not hand-code Chyron, Motion, Liquid, or Python experiment features. Continue to Motion, Liquid, and Python only after Chyron proves the loop; otherwise investigate, patch the substrate through git/CI/deploy, and rerun Chyron. Stop only on full Chyron loop success plus sequential next-step readiness, or a named invariant-level blocker with VText, media, Trace, run-acceptance, rollback refs, residual risks, and the next executable probe.
```

## Mission Frame

The v0 mission proved an important negative result: the first portfolio was not
human-reviewable. It produced static or weak product claims, package/build
receipts, and confusing Apps & Changes surfaces, but the owner could not simply
watch the four experiments work and decide whether to pull them.

The async-supervision follow-up fixed much of the runtime shape:

- super can start worker delegations asynchronously instead of disappearing
  inside one blocking delegate call;
- VText can receive mirrored worker updates and serve as a live mission
  dashboard;
- `worker-medium` and `worker-playwright` are separate classes, so ordinary
  worker VMs stay lighter while browser evidence has a bounded heavier path;
- worker/vsuper/co-super prompting now has a clearer control contract.

The latest Chyron rerun found a narrower gap. The implementation worker created
a candidate commit locally, but the proof worker could not inspect that commit
because it was not pushed or exported. The correct fix is not to push worker
commits to GitHub. The correct fix is to make the source delta transferable as
an AppChangePackage before asking a separate proof worker to inspect it.
Commit `b11ed4f2f517b2f1a7a3d8a054b17490b76510ec` deployed that
source-transfer contract to staging on 2026-05-23.

This mission resumes from that learning.

## Core Invariant

```text
AppChangePackage is the transferable source artifact.
Human proof is the review gate.
Recipient build/adoption/rollback is the installation gate.
```

A package may be `evidence_pending`. That is useful and honest: it means the
source delta exists and can be inspected or applied by a later proof/adoption
path.

A package must not be labeled reviewable until it has:

- owner-readable VText narrative;
- screenshot, video, or measured behavior benchmark evidence;
- relevant Trace/run-acceptance refs;
- explicit residual risks and rollback refs.

Do not let "package exists" collapse into "works", "reviewable", "installable",
or "installed".

## Real Artifact

The artifact is a working self-development review loop:

```text
Codex supervises the platform harness
  -> Choir-in-Choir builds candidate feature
  -> candidate source delta is published as AppChangePackage
  -> human proof attaches to that same package identity
  -> recipient computer pulls, rebuilds, verifies, adopts, rolls back
  -> Apps & Changes presents the result in human terms
  -> VText narrates the run as a live dashboard and final report
```

The first payload is **Chyron Shelf Observability**. The spelling matters:
`chyron` is the broadcast lower-third/ticker term. This is the product idea:
live tool calls, interim model updates, and agent-to-agent/channel messages
stream in a tasteful translucent Shelf-adjacent surface while the prompt and
desktop remain usable.

The other experiments remain queued:

1. Chyron Shelf Observability.
2. Process/window/agent animation language.
3. Choir Liquid Material Engine.
4. Python code mode.

Run them sequentially until the loop is stable. Do not restart concurrency in
this mission.

## Roles

**Codex**

- May directly patch runtime, harness, prompts, product proof, Apps & Changes,
  VText, Trace, run acceptance, and deployment plumbing.
- Must not hand-code Chyron, Motion, Liquid, or Python experiment features.
- Supervises by reading VText dashboards, screenshots/video, Trace, and
  run-acceptance evidence.
- If Choir-in-Choir fails, investigates why, patches the substrate if
  authorized, deploys, and reruns the same experiment through Choir.

**super**

- Foreground orchestration authority.
- Starts and supervises worker VMs asynchronously.
- Leases `worker-medium` for implementation/build work.
- Leases `worker-playwright` only after package evidence exists and browser
  proof is needed.
- Redirects or cancels workers; VText does not control workers directly.

**vsuper**

- Candidate-world orchestrator.
- Coordinates implementation and verifier co-supers.
- Curates substantive owner-readable checkpoints to VText/super/Trace.
- Publishes an AppChangePackage after commit plus focused verification, even if
  external human proof is still pending.

**co-super implementation**

- Owns candidate source edits while active.
- Commits source changes.
- Publishes an AppChangePackage or submits a precise blocker.

**co-super verifier**

- Runs independent checks and may write scratch tests/logs/evidence.
- Does not author candidate source, publish packages, promote/adopt, or grant
  capabilities.

**VText**

- Is the live narrative dashboard.
- Produces revisions that summarize the whole run so far: past work, current
  state, learnings, blockers, next steps, and owner-relevant risks.
- May ask super clarifying questions.
- Does not issue worker-control commands.

## Homotopy Axes

Increase realism while preserving the same object:

1. **Source transfer:** from local worker commit to product-visible
   AppChangePackage.
2. **Human evidence:** from build receipt to VText narrative plus real
   screenshot/video/benchmark refs.
3. **Proof route:** from proof worker chasing a commit SHA to proof worker
   inspecting a package id or package-derived candidate/adoption route.
4. **Adoption:** from package existence to recipient rebuild, verification,
   adoption, and rollback.
5. **Experiment breadth:** from Chyron only to Motion, Liquid, and Python after
   the first loop proves stable.

## Acceptance Contracts

### Runtime / Source-Transfer Gate

Before judging Chyron as a feature:

- staging health must report the current deployed commit;
- visible prompt-bar run must start super/vsuper async delegation;
- implementation worker must publish a product-visible AppChangePackage or
  return a precise source-transfer blocker;
- proof worker must not be asked to inspect only a worker-local commit SHA;
- VText must receive substantive worker updates.

Minimum checkpoint improvement after the current source-transfer patch:

```text
one product-visible AppChangePackage exists for Chyron,
possibly evidence_pending,
with source delta hashes and rollback/base refs.
```

If this does not happen, continue runtime/prompt/harness investigation. Do not
call that a Chyron experiment result.

### Chyron Human-Proof Gate

Chyron is not reviewable until evidence shows:

- live prompt/tool/run status text appears while work is happening;
- agent-to-agent/channel messages are represented at human-readable resolution;
- the stream does not block Desk menu, app buttons, prompt input, or window
  controls;
- prompt input focus hides, pauses, or de-emphasizes the stream;
- screenshot/video evidence is captured through `worker-playwright` or another
  explicitly verified product-path browser proof;
- VText explains the behavior and limitations plainly.

Do not accept:

- fake random ticker text;
- a poster/mockup;
- generated claims without live product evidence;
- build-only proof;
- package-only proof.

### Recipient Adoption Gate

Recipient proof requires:

- package can be pulled without exposing raw package ids in ordinary UI;
- recipient candidate build produces recipient-specific Go/Svelte artifact
  hashes;
- verifier checks source refs, source deltas, build, behavior proof, and
  rollback profile;
- adoption/promote changes active recipient computer state only after
  verification;
- rollback is executed or at least product-visible and verified, depending on
  current product capability.

## Experiment Queue

### 1. Chyron Shelf Observability

Goal: make live agent/tool activity glanceable while preserving the desktop and
prompt as the primary interaction surface.

Required proof: VText narrative, screenshots/video, Trace/run-acceptance refs,
AppChangePackage, recipient build/adoption/rollback or precise blocker.

### 2. Process / Window / Agent Animation Language

Goal: make boot, wake, app launch, window focus, agent progress, and process
state changes legible without decorative noise.

Required proof: Playwright video of real transitions plus resource/UX notes.

### 3. Choir Liquid Material Engine

Goal: explore a real Choir desktop material language, not a poster route.

Required proof: screenshots/video of the actual desktop retaining functionality,
resource benchmark on desktop and mobile-class browser, privacy constraints for
blurred/translucent surfaces.

### 4. Python Code Mode

Goal: evaluate Python as a replacement or alternative profile for arbitrary
headless computer use.

Required proof: benchmark against existing bash profile for time, token use,
tool-loop iterations, failure modes, and Trace readability.

## Forbidden Shortcuts

- Do not hand-code experiment features with Codex.
- Do not run the four experiments concurrently.
- Do not claim package/build receipts as human proof.
- Do not make a proof worker chase an unreachable worker-local commit.
- Do not label `evidence_pending` as reviewable.
- Do not use static seed changes, generated reports, fake media, or poster
  routes.
- Do not use `export_patchset` or `/api/promotions`.
- Do not mutate active computers directly during Try/preview.
- Do not hide failures behind debug controls or internal/test routes.
- Do not skip recipient build/adoption/rollback when claiming install proof.
- Do not call this mission complete after Chyron package creation alone.

## Quality Bar

Default quality target: **solid**.

After the first successful Chyron loop, perform a quality pass before moving to
Motion:

- simplify prompts or duplicated runtime paths exposed by the run;
- improve VText dashboard readability;
- make Trace highlight the important events;
- verify Apps & Changes presents the package in human terms;
- record residual risks and rollback refs.

Do not polish visual styling before the behavior/evidence loop works. Do not
move to the next experiment while the current one has only machine receipts.

## Verification

Required local checks for platform patches:

```text
nix develop -c go test ./internal/runtime -count=1
nix develop -c go test ./internal/vmctl ./internal/vmmanager -count=1
npm --prefix frontend run build
```

Use narrower focused tests first when iterating, but do not skip the relevant
full package tests before push when runtime contracts change.

Required landing loop for behavior-changing platform changes:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed product proof
```

Required product proof after source-transfer patch:

```text
visible prompt bar on staging
-> Chyron candidate run through super -> vsuper -> co-super
-> product-visible AppChangePackage or precise blocker
-> VText dashboard revision
-> run acceptance synthesis
```

Required full Chyron proof:

```text
AppChangePackage
+ human proof refs
+ recipient adoption/build/verify/promote or precise blocker
+ rollback refs
+ readable VText/Trace/run-acceptance evidence
```

## Run Checkpoint & Resumption State

```text
status: ready_resume
last checkpoint:
  Async supervision deployed through a01595f; source-transfer contract patch
  b11ed4f deployed to staging and /health reports proxy/upstream at that SHA.
current artifact state:
  Runtime has async worker supervision and distinct worker classes. First
  Chyron rerun produced a worker-local candidate commit but no package because
  the proof route expected a separate worker to inspect an unreachable commit.
what shipped:
  Up through b11ed4f2f517b2f1a7a3d8a054b17490b76510ec. GitHub Actions run
  26337846243 passed and staging /health reported proxy/upstream at that SHA.
what was proven:
  Async worker start/observe evidence, VText worker-update mirroring,
  worker-playwright browser smoke, and review-gate distinction between machine
  receipts and human proof.
unproven or partial claims:
  Chyron has not yet reached end-to-end package + human proof + recipient
  adoption/rollback. Motion, Liquid, and Python have not been rerun.
belief-state changes:
  Human proof must come after transferable source packaging in multi-worker
  evidence paths. The package can be evidence_pending; reviewability cannot.
remaining error field:
  Whether Choir-in-Choir now publishes a Chyron AppChangePackage on staging and
  whether the proof worker can attach media evidence through a package-derived
  route.
highest-impact remaining uncertainty:
  Can Choir itself produce the first useful self-development change without
  Codex hand-coding it?
next executable probe:
  Rerun Chyron through the visible staging prompt bar. If no package appears,
  investigate runtime/prompt/orchestration. If package appears but
  proof/adoption fails, investigate Apps & Changes, candidate routing, proof
  worker, or adoption.
suggested resume goal string:
  Use the one-line goal string in this file.
evidence artifact refs:
  docs/mission-async-supervision-runtime-hardening-v0.md;
  /Users/wiz/go-choir/test-results/chiron-sequential-a01595f-20260523T152628Z
rollback refs:
  Revert b11ed4f if source-transfer prompt changes regress package publication,
  worker delegation, or review-gate behavior after deploy.
```

## Stopping Condition

This mission is complete only when:

- Chyron completes the full loop: package, human proof, recipient
  build/adoption/rollback, VText/Trace/run-acceptance evidence;
- the loop is stable enough to proceed sequentially to Motion;
- any platform changes have deployed staging proof and rollback refs;
- residual risks and next realism axis are recorded.

If Chyron reaches package-only proof, report `checkpoint_incomplete`, not
complete. If Chyron cannot produce a package after root-cause probes and
runtime/prompt fixes, report `blocked_incomplete` with the exact blocker and
next executable probe.
