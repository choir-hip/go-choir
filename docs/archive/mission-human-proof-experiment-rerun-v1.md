# MissionGradient: Human-Proof Experiment Rerun v1

**Status:** checkpoint_incomplete — substrate deployed at `a8b02af`, model/gateway smoke found ChatGPT auth refresh failure, Chyron human proof still unproven
**Date:** 2026-05-23
**Supersedes:** [mission-human-proof-experiment-rerun-v0.md](mission-human-proof-experiment-rerun-v0.md)
**Depends on:** async supervision runtime hardening lessons now folded into
runtime invariants and the current campaign compiler mission.
**State ledger:** [platform-os-app-state.md](../platform-os-app-state.md)

## One-Line Goal String

```text
/goal Run docs/mission-human-proof-experiment-rerun-v1.md as a Codex-supervised MissionGradient mission: restore a clean deployed baseline, then rerun Chyron Shelf Observability through Choir-in-Choir until it produces human-proof feature evidence or a precise substrate blocker. Start from staging 415b87e, where runtime model/context substrate is deployed, and local commit adbc04f, which must be pushed, CI/deploy-monitored, and staging-identity verified before product proof. Run a narrow staging auth/gateway/model-policy/context smoke, then use visible product path and VText narrative supervision to have Choir agents build Chyron in a candidate computer, publish an honest AppChangePackage as transferable source, attach owner-readable VText narrative plus screenshot/video or benchmark proof, verify recipient build/adoption/rollback through product APIs, and expose readable Trace/run-acceptance evidence. A package may be evidence_pending, but it must not be reviewable until human proof exists. Codex may patch runtime/harness/prompts/evidence plumbing but must not hand-code Chyron, Motion, Liquid, or Python experiment features. Continue to Motion, Liquid, and Python only after Chyron proves the loop; otherwise investigate, patch the substrate through git/CI/deploy, and rerun Chyron. Stop only on full Chyron loop success plus sequential next-step readiness, or a named invariant-level blocker with VText, media, Trace, run-acceptance, rollback refs, residual risks, and the next executable probe.
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
  narrative;
- `worker-medium` and `worker-playwright` are separate classes, so ordinary
  worker VMs stay lighter while browser evidence has a bounded heavier path;
- worker/vsuper/co-super prompting now has a clearer control contract.

An earlier Chyron rerun found a narrower gap. The implementation worker created
a candidate commit locally, but the proof worker could not inspect that commit
because it was not pushed or exported. The correct fix is not to push worker
commits to GitHub. The correct fix is to make the source delta transferable as
an AppChangePackage before asking a separate proof worker to inspect it.
Commit `b11ed4f2f517b2f1a7a3d8a054b17490b76510ec` deployed that
source-transfer contract to staging on 2026-05-23.

Subsequent checkpoints proved package preloading into proof workers and
recipient adoption/rollback. Commit `b8086961ce2d8ec35b5c06a53037002d11806840`
then shipped the missing verified adoption UI preview route. The first deployed
proof run against that route did not reach the Chyron package/adoption stage:
it failed during Playwright re-authentication with
`login/begin failed: 500 {"error":"failed to save challenge"}`. This mission
therefore resumes from a narrower substrate checkpoint, not from Chyron feature
success.

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
  -> VText narrates the run as a live human-readable story and final report
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
- Supervises by reading VText narratives, screenshots/video, Trace, and
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

- Is the live narrative supervision surface.
- Produces revisions that summarize the whole run so far in prose: objective,
  past work, what changed, current state, evidence, learnings, blockers, next
  steps, and owner-relevant risks.
- Must not become a Trace-like topology dashboard, worker table, raw event log,
  or hash/id ledger. Technical ids belong in a short appendix only when they
  help review or rollback.
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

- a video shows Choir operating normally while Chyron text streams over or near
  the Shelf during real work;
- the Chyron stream is more granular than VText and carries concise live
  activity items such as tool calls, interim model messages, worker/run status,
  and agent-to-agent/channel messages;
- the stream does not block Desk menu, app buttons, prompt input, or window
  controls;
- prompt input focus hides, pauses, or de-emphasizes the stream;
- screenshot/video evidence is captured through `worker-playwright` or another
  explicitly verified product-path browser proof;
- VText explains the objective, what changed, what evidence exists, what is
  still missing, and what happens next in plain narrative prose.

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
- improve VText narrative readability;
- make Trace highlight the important events;
- verify Apps & Changes presents the package in human terms;
- record residual risks and rollback refs.

Do not polish visual styling before the behavior/evidence loop works. Do not
move to the next experiment while the current one has only machine receipts.

## Verification

Required local checks for platform patches:

```text
nix develop -c scripts/go-test-runtime-shards
nix develop -c scripts/go-test-local
nix develop -c go test ./internal/vmctl ./internal/vmmanager -count=1
npm --prefix frontend run build
```

Use narrower focused tests first when iterating. The runtime package is broad
and embedded-Dolt-heavy; use the sharded script for real local runtime coverage
instead of an unbounded serial `go test ./internal/runtime` run.

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
-> VText narrative revision
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
status: checkpoint_incomplete
last checkpoint:
  Runtime model/context substrate shipped at 415b87e. Follow-up checkpoint
  a8b02af40b6d3808f123d38775d4f1efd2924e78 is now pushed to main, CI run
  26345624615 passed, and staging reports proxy and sandbox deployed_commit
  a8b02af40b6d3808f123d38775d4f1efd2924e78.
  Follow-up commit adbc04f871f772dbda6feba33ab2c3abaa639ddf improves
  the runtime test loop by removing default artificial stub-provider delay,
  deleting a no-op sleep-only test, replacing one streaming sleep with terminal
  polling, and adding shared local/CI runtime shard scripts. That commit is
  included in the pushed a8b02af checkpoint and is deployed.

  Previous Chyron source-transfer/adoption proof remains useful but
  incomplete:
  Chyron source-transfer/adoption proof reached on staging after iterative
  runtime fixes. The proof worker saw the product-visible AppChangePackage
  after the worker-delegation preload fix, and recipient build/adoption/rollback
  succeeded for a Chyron package. Commit b808696 then shipped a verified
  adoption UI preview route so Apps & Changes / evidence tooling can open the
  recipient-built UI artifact at /api/adoptions/{adoption_id}/preview. The
  first deployed proof run after that patch failed before publishing a new
  package because the Playwright auth helper hit login/begin 500
  "failed to save challenge".
current artifact state:
  Runtime has async worker supervision, distinct worker classes, package
  publication mirroring, recipient AppChangePackage pull/adoption/build/verify,
  rollback proof, and a new worker-delegation preload path that imports
  referenced visible AppChangePackages into the proof worker runtime before the
  worker run starts. Apps & Changes now has a product-visible verified adoption
  preview route backed by the recipient build workspace; ordinary preview is
  still gated by a verified/adopted/rolled_back adoption state and owner scope.
  Runtime model/context substrate has also landed: auth busy handling,
  Fireworks model catalog additions, editable per-computer model policy,
  provider image blocks, per-run model routing, and run-memory raw-entry
  retrieval. Local test execution guidance now points agents to dev-shell
  sharded runtime tests.
what shipped:
  a28a7a added this mission. 8ad20d8 normalized ledger role refs for adoption
  builds. b222079 made duplicate worker starts idempotent. aee2b81 honored
  explicit package app_id/visibility constraints. 81d29d2 preloads referenced
  AppChangePackages into proof workers and tightens the super proof-worker
  prompt. b808696 serves verified adoption UI previews from recipient build
  workspaces. CI run 26342292117 passed for b808696, and Node B staging health
  reported proxy and sandbox deployed_commit b808696 at 2026-05-23T20:07:36Z.
  415b87e hardened runtime model/context substrate; GitHub Actions run
  26344950167 passed and staging health reported proxy/sandbox deployed_commit
  415b87e at 2026-05-23T22:17:41Z. a8b02af checkpointed the mission docs and
  included adbc04f test-loop cleanup; GitHub Actions run 26345624615 passed and
  Node B deploy completed in 8m51s. Staging health reported proxy/sandbox
  deployed_commit a8b02af at 2026-05-23T22:50:47Z.
what was proven:
  On staging 81d29d2, Chyron package 466c8786-4bc1-4b8c-8be9-7aeb45226707 was
  published as unlisted with app_id
  human-proof-chyron-chyron-seq-1779565124843. It was pulled into recipient
  computer owner-review-chyron-chyron-seq-1779565124843, applied to a
  recipient candidate, built with recipient-specific Go/Svelte artifact hashes,
  verified, promoted, and rolled back. The proof worker's observe output showed
  one product-visible preloaded AppChangePackage, proving the package-context
  preload path itself works.
  Locally and in CI, b808696 added tests that a verified adoption preview
  rewrites built UI asset paths and serves assets from the recipient build
  workspace without exposing arbitrary paths.
unproven or partial claims:
  Chyron still lacks actual screenshot/video/benchmark human proof of the
  feature behavior. The VText narrative remains too technical and does not yet
  function as an owner-readable live narrative. The b808696 deployed proof did
  not establish that the preview route can show the Chyron behavior because it
  stopped at auth challenge persistence before package/adoption preview. Motion,
  Liquid, and Python have not been rerun.
belief-state changes:
  Human proof must come after transferable source packaging in multi-worker
  evidence paths. A package can be evidence_pending; reviewability cannot.
  Passing only a package id is no longer enough even after preload; the proof
  worker also needs a durable package-derived candidate/adoption preview route
  or an equivalent materialized workspace/app URL that can be opened by browser
  evidence tooling. That preview route now exists, but the next proof must
  first show the auth/session path is healthy enough for long product-path
  Playwright runs.
remaining error field:
  A narrow staging auth shell smoke passed, but a trace-level product-path
  model/context smoke failed before useful LLM work with
  "gateway client: chatgpt: auth: refresh chatgpt auth via http: status 401
  Unauthorized" and provider error code "refresh_token_reused". The run did
  not surface llm_provider/llm_model/model_policy evidence. Root-cause whether
  conductor/VText are still pinned to ChatGPT, whether model policy fallback is
  not being applied or not being recorded, and whether ChatGPT refresh is not
  single-flight/persisted correctly before rerunning Chyron. Separately, verify
  whether the next Chyron run can produce human-readable VText/media proof
  instead of only package/build receipts.
highest-impact remaining uncertainty:
  Can Choir itself produce the first useful self-development change without
  Codex hand-coding it once the model/gateway substrate is healthy?
next executable probe:
  Investigate the a8b02af ChatGPT refresh_token_reused failure and missing
  product-visible model-policy evidence. Patch the smallest implicated
  provider/model-policy/auth/runtime evidence boundary through git/CI/deploy,
  verify staging identity, then rerun the trace-level auth/gateway/model-policy
  smoke. If healthy, rerun the Chyron proof and require the proof harness to
  open /api/adoptions/{adoption_id}/preview after verification and capture
  screenshot/video of real behavior.
suggested resume goal string:
  Use the one-line goal string in this file.
evidence artifact refs:
  the folded async supervision runtime hardening lessons;
  docs/mission-runtime-model-context-substrate-v0.md;
  /Users/wiz/go-choir/test-results/chyron-sequential-b808696-20260523T201837Z;
  /Users/wiz/go-choir/test-results/frontend-tests-chiron-sequ-68a22-evidence-or-precise-blocker
rollback refs:
  Revert aee2b81 for package constraint regressions. Revert 81d29d2 if worker
  delegation starts failing before worker run submission or if package
  preloading exposes packages outside normal visibility rules. Revert b808696
  if adoption preview leaks paths or serves previews for non-owner,
  non-verified adoption records.
  Revert 415b87e if model/context substrate regresses auth/gateway/runtime
  behavior. Revert adbc04f/a8b02af if the shared runtime shard script path
  regresses CI or local developer test behavior.
deferred adjacent work:
  Runtime model catalog/config should become durable runtime configuration:
  adding model ids to an already-configured provider should not require a
  platform deploy or ChatGPT auth replacement. Fireworks is already credentialed
  on Node B, while ChatGPT auth should remain on the existing account because
  the previous rate limit reset. This provider/model work is important but is
  deliberately deferred until this mission checkpoint is recorded.
  Node B deploy also exposed disk pressure: during the a8b02af deploy, `/` on
  node-b had about 57G free on a 476G filesystem while rebuilding guest and
  guest-playwright images. Add a dedicated cleanup/retention task for old guest
  images, stale VM disks, Nix generations/store paths, worker evidence bundles,
  and candidate artifacts. The policy must preserve rollback refs and active
  computer state while reclaiming largest stale candidate/worker image data
  first. Do not treat ad hoc `rm` of old images as the solution; define a
  retention controller/report that inventories largest consumers, proves which
  artifacts are disposable, preserves rollback/product evidence, and gives the
  operator an explicit emergency reclaim path. A follow-up check during the
  c0ca8fe deploy found the sharper starting point: `/var/lib/go-choir` was about
  265G, `/var/lib/go-choir/vm-state` about 261G, and there were 3,231 `vm-*`
  directories with only 15 running Firecracker processes. Current base guest
  images were only about 1.5G and 2.3G, so the largest safe target is stale
  per-VM state after active/rollback/evidence classification.
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
