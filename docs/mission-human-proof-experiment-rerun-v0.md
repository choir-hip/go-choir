# MissionGradient: Human-Proof Experiment Rerun v0

**Status:** checkpoint_incomplete
**Date:** 2026-05-22
**Prior failed portfolio:** [mission-alternate-computer-ux-experiment-portfolio-v0.md](mission-alternate-computer-ux-experiment-portfolio-v0.md), [mission-apps-and-changes-store-sweep-v0.md](mission-apps-and-changes-store-sweep-v0.md)
**State ledger:** [platform-os-app-state.md](platform-os-app-state.md)

## One-Line Goal String

```text
/goal Run docs/mission-human-proof-experiment-rerun-v0.md as a Codex-supervised MissionGradient mission: first use Codex directly to delete false-proof experiment junk and harden the Choir harness/runtime around human evidence, then switch to Codex-controlling Choir-developing-Choir for the experiment rerun. Treat the previous Apps & Changes portfolio as failed product evidence: keep historical receipts for diagnosis, but remove or replace static seed-change UI, generated claim reports, permissive reviewability labels, preview-health bypasses, and acceptance paths that let package/build receipts masquerade as working features. Make a Change reviewable only after a Choir-generated causal VText narrative plus real screenshots/video or benchmark evidence exists; make live preview require a healthy candidate route; make install require recipient candidate build plus behavior proof and rollback. Rerun Chiron Shelf observability, process/window/agent animation language, Choir Liquid Material Engine, and Python code mode one at a time through visible product-path Choir-in-Choir, with no concurrency until the sequential loop is stable. Codex must not hand-code the experiment features at all. Codex supervises by reading the updating VTexts, screenshots, videos, Trace, and run acceptance evidence; if Choir-in-Choir fails, Codex investigates why, improves the runtime/harness/multiagent orchestration/prompting, lands those platform fixes through git/CI/deploy, then reruns the experiment through Choir again. Finish with owner-readable VText dashboards/reports, media evidence, Trace/run-acceptance evidence, rollback refs, residual risks, and the next realism axis. If incomplete, report checkpoint_incomplete or blocked_incomplete, update this mission doc with a resumable checkpoint, and continue/redirect/delegate any safe executable next probe inside current authority before stopping.
```

## Mission Frame

The prior portfolio produced machine receipts and UI surfaces, but not useful
human proof. The four experiments were not reviewable in the sense the owner
cares about:

- Liquid was a poster/prototype route, not a redesigned Choir desktop.
- Chiron did not show live tool calls or agent-to-agent messages streaming
  while work was happening.
- Motion did not provide convincing process/window/agent animation behavior.
- Python mode produced an execution-primitive benchmark, not an agent tool-loop
  benchmark or usable profile review.

This mission does not try to rescue those outputs. It treats them as failed
iteration-1 evidence and improves the loop that produced them.

There are two distinct control loops:

1. **Harness repair loop:** Codex may edit platform code directly to remove
   false-proof paths, strengthen review gates, and make the Choir-in-Choir
   evidence loop observable. This includes runtime, harness, multiagent
   orchestration, prompting, verification, preview, VText, Trace, and
   acceptance machinery.
2. **Experiment rerun loop:** after the harness is ready, Codex should operate
   Choir through the product path. Choir's super/vsuper/cosuper agents do the
   experiment work in candidate/background computers. Codex supervises from the
   outside by reading the live VText dashboard, screenshots, videos, Trace, and
   run-acceptance records, then redirects through product prompts. If the
   experiment fails, Codex diagnoses whether the failure belongs to the
   harness/runtime/orchestration/prompting/evidence system, fixes that layer
   directly, and reruns the experiment through Choir.

Codex is the outer supervisor, not the author of the second-pass experiment
features. It must not hand-code Chiron, Motion, Liquid, or Python mode. The
experiments are also a meta-experiment: prove Choir can productively improve
itself. Insofar as Choir-in-Choir fails, the output is not a Codex-written
feature; it is a root-cause learning and a harness/runtime/orchestration/prompt
improvement that makes the next Choir-run attempt better.

The product truth is:

```text
human evidence first
-> optional live preview
-> candidate build in the recipient computer
-> behavior verification
-> install/promote
-> rollback/disable/uninstall where honest
```

Machine receipts still exist, but they are hidden safety machinery. The owner
should first see a clear VText narrative, screenshots, video, and benchmarks
when relevant. Package ids, refs, digests, and manifests are supporting details,
not the review surface.

## Real Artifact

The artifact is a corrected experiment and change-review harness:

```text
Codex platform preflight
  -> remove false-proof product paths
  -> harden evidence gates
  -> deploy and verify staging identity

causal mission VText dashboard
  -> created and updated by Choir agents during the product-path run
  -> super/vsuper/cosuper worker updates after substantive changes
  -> VText revisions that summarize the whole run so far
  -> per-experiment narrative reports grounded in actual evidence

human proof packet
  -> narrative VText
  -> screenshots and video
  -> benchmark where relevant
  -> plain-English risks and capability requests
  -> machine refs hidden under details

reviewable Change
  -> not reviewable until the human proof packet exists
  -> Try Live only when candidate route is healthy
  -> Install only after recipient candidate build and behavior proof
  -> Rollback/Disable/Uninstall represented honestly

sequential experiment rerun
  -> Codex submits/redirects product prompts
  -> Choir-in-Choir performs candidate work
  -> Codex supervises via VText/media/Trace evidence
  -> Codex fixes only harness/runtime/orchestration/prompting blockers
  -> same experiment is rerun through Choir after each platform fix
  -> Chiron
  -> Motion
  -> Liquid
  -> Python mode
```

The artifact is not:

- static seed data pretending to be a catalog;
- VText reports generated from frontend constants;
- package/build acceptance relabeled as feature acceptance;
- candidate iframe preview that can be `502` while install still passes;
- concurrent experiment chaos;
- another store redesign that never shows the experiments working.

## Cleanup And Deletion

Delete or hard-replace the false-proof paths. Preserve historical evidence
files as diagnostic artifacts, but do not keep product code that lets those
artifacts appear successful.

Required cleanup targets:

- remove static four-experiment seed records from ordinary product UI;
- remove generated VText reports that are based on seed metadata rather than
  causal run updates;
- remove or rename status labels that say `reviewable` before human proof
  exists;
- remove acceptance summaries that treat `export-level` or package/build
  records as sufficient UX experiment evidence;
- make candidate preview route health a hard state transition, not a cosmetic
  iframe;
- ensure `502`, boot pending, auth failure, or route failure blocks preview
  success and install readiness;
- move technical refs behind details and keep them subordinate to human proof;
- mark the prior portfolio docs as historical failed/product-incomplete
  evidence where necessary.

No broad cleanup should erase rollback, trace, package, run-acceptance, or
diagnostic evidence needed to understand what happened.

## Harness Improvements

### VText As Live Mission Dashboard

For long-running self-development and experiment work, VText is not a final
report. It is the live narrative dashboard.

Required behavior:

- the dashboard is produced by Choir's agents during the product-path run, not
  by Codex filling in a report after the fact;
- super, vsuper, and cosuper send `submit_worker_update` after every
  substantive change, blocker, verifier result, evidence capture, or decision;
- the target VText document receives and consumes those updates;
- each new VText revision summarizes the whole run from the beginning through
  the current moment: past work, current state, learnings, next steps, and
  known risks;
- run acceptance records include VText dashboard doc/revision ids and worker
  update consumption counts;
- a missing or stale VText dashboard prevents a long experiment mission from
  being called complete.

The VText should be readable by the owner and useful to Codex as the outer
supervisor. Avoid raw package ids, hashes, and internal jargon in the body
unless they are under a technical appendix.

### Human Proof Gates

Introduce a review state model that separates machine receipts from human
evidence.

Suggested states:

| State | Meaning |
| --- | --- |
| `draft` | Work exists but is not reviewable. |
| `evidence_pending` | Machine receipts may exist, but human proof is missing. |
| `human_reviewable` | Narrative VText plus screenshots/video/benchmarks exist. |
| `preview_ready` | Candidate route is healthy and can be opened safely. |
| `install_ready` | Recipient candidate build plus behavior verification passed. |
| `installed` | Active computer switched to the verified candidate. |
| `rolled_back` | Active computer returned to recorded rollback state. |
| `blocked` | Precise blocker with evidence and next probe. |

The exact implementation can differ, but these transitions must not collapse.

### Behavior Acceptance

Run acceptance must not mark a UX experiment successful from package/build
evidence alone.

Add experiment-specific behavior contracts:

- Chiron: video or live trace proves tool calls, interim model messages, and
  agent-to-agent/channel messages stream through the Shelf without blocking
  Desk controls or prompt input.
- Motion: video proves boot, wake, app launch, window raise/minimize/restore,
  and agent/process state transitions are legible and not merely decorative.
- Liquid: screenshots/video prove the real desktop UI keeps functionality while
  material treatment changes; benchmark proves resource cost on desktop and
  mobile browsers within a named budget.
- Python mode: benchmark proves an actual agent/tool-loop profile family, not
  just a local execution primitive; result compares time, token payload, tool
  loop iterations, trace readability, and failure modes against bash.

### Preview And Install

Preview and install must be separate.

Valid path:

```text
human_reviewable
-> Try Live
-> candidate route healthy
-> preview_ready
-> recipient candidate build
-> behavior proof
-> install_ready
-> owner install/promote
```

Invalid path:

```text
package exists
-> iframe shows 502
-> verify/install buttons still imply success
```

### Security And Privacy

Speculative source changes are untrusted code.

Required security properties:

- no private prompts, uploads, files, provider credentials, tokens, cookies, or
  account-specific state in source deltas or demo media;
- source-delta publication uses structured secret/content scanning, not only a
  few substring checks;
- unlisted packages are treated as published to anyone with the link;
- capability requests are shown in plain English before install;
- preview candidates are isolated from the active computer until promotion;
- no live private DOM capture for Liquid materials;
- no code path allows technical evidence to forge human proof.

## Sequential Experiment Loop

Do not run concurrent experiments in this mission. Sequential stability is the
real test. Concurrency can return after the evidence loop is trustworthy.

Before this loop starts, Codex should have already landed any required harness
fixes through the platform deploy path. During this loop, Codex must not
hand-edit experiment feature code. The only direct Codex implementation surface
is the platform substrate that enables Choir-in-Choir to do the work:
runtime/harness/orchestration/prompting/evidence/preview/acceptance. The normal
route is: Codex submits or redirects product prompts, Choir-in-Choir does the
work, and Codex evaluates the live VText/media/Trace evidence.

For each experiment:

1. Codex starts or redirects the product-path Choir-in-Choir run for the
   experiment.
2. Choir opens or updates the mission VText dashboard and records the
   experiment intent.
3. Choir develops in a candidate/background computer through super/vsuper/cosuper.
4. Choir updates VText after every substantive change or blocker.
5. Choir captures screenshots and video before attempting install.
6. Choir runs the experiment-specific behavior contract.
7. Only then package/adopt/preview/install/rollback as appropriate.
8. Choir writes a per-experiment VText report with plain-English recommendation:
   promote, iterate, abandon, or blocked.
9. Codex reviews the VText/media/Trace packet and either redirects the product
   run, records a blocker, or moves to the next experiment.
10. If the packet reveals a Choir-in-Choir failure, Codex root-causes and fixes
   the harness/runtime/orchestration/prompting/evidence layer, deploys that
   fix, then reruns the same experiment through Choir.
11. Update the mission doc checkpoint before moving to the next experiment.

### Experiment Order

1. **Chiron Shelf observability**
   - Most strategically aligned with making long runs observable.
   - Must show live tool and agent message flow, not static text.

2. **Process/window/agent animation language**
   - Builds on Chiron by making state changes legible.
   - Must improve understanding, not add ornamental motion.

3. **Choir Liquid Material Engine**
   - Visual system experiment.
   - Must apply to real desktop function and stay within resource/privacy
     budgets.

4. **Python code mode A/B**
   - Runtime/profile experiment.
   - Must benchmark real agent/tool-loop behavior against bash.

## Invariants

- Human proof is the first-class review artifact.
- Machine receipts support review; they do not constitute review.
- Codex may directly repair the harness/runtime/orchestration/prompting
  substrate, but must not hand-code experiment feature work.
- Chiron, Motion, Liquid, and Python mode changes must be produced through
  Choir-in-Choir candidate/background computers.
- VText is causal and live from Choir agents, not a static report generator or
  Codex-authored after-action summary.
- Preview requires route health.
- Install requires recipient-specific candidate build, behavior proof, and
  rollback refs.
- Active computers are not mutated directly.
- Package ids, hashes, refs, and digests are hidden from ordinary UI but
  inspectable under details.
- No experiment is concurrent until the sequential loop is stable.
- No static catalog claims, fake thumbnails, fake videos, fake VText reports,
  local-only proof, or package/build-only success.
- Preserve staging deploy discipline for platform behavior changes.

## Value Criterion

Maximize:

```text
owner-understandable evidence that real experimental changes work
+ harness ability to prevent false success
+ speed of feedback through live VText and media proof
+ safe package/adoption/install/rollback topology
- static claims
- hidden technical jargon in review surfaces
- preview/install confusion
- concurrent-loop confusion
- private data leakage or capability ambiguity
```

## Dense Feedback

Required evidence:

- VText mission dashboard with multiple causal revisions;
- per-experiment VText reports;
- Playwright screenshots and video;
- benchmark JSON and plain-English summaries where relevant;
- Trace trajectories for super/vsuper/cosuper worker updates;
- run acceptance records that distinguish machine receipts from human proof;
- candidate preview health checks;
- recipient build logs and artifact digests when install is attempted;
- rollback refs;
- staging identity after any platform deploy.

## Forbidden Shortcuts

- Do not keep static seed records as production catalog truth.
- Do not generate VText reports from frontend constants and call them evidence.
- Do not claim success from `export-level`, package publication, or recipient
  build alone.
- Do not let a `502` preview proceed to install readiness.
- Do not run the four experiments concurrently.
- Do not let Codex hand-code Chiron, Motion, Liquid, or Python mode.
- Do not hide failed demos behind technical refs.
- Do not use platform deploy as proof of user-computer experiment success.
- Do not copy binaries between computers.
- Do not publish private content or credentials in source deltas.
- Do not call a checkpoint complete.

## Rollback Policy

Platform harness fixes must land through:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging identity -> run deployed acceptance proof
```

Every install/adoption attempt must record:

- previous active source ref;
- candidate source ref;
- runtime/UI artifact digests;
- route/default-base changes if any;
- rollback action and result;
- residual state that rollback does not undo.

If a harness fix regresses active user computers, rollback the platform commit
and preserve the failed evidence packet for diagnosis.

## Run Checkpoint & Resumption State

```text
status: checkpoint_incomplete
last checkpoint: deployed stale-source review sanitizer `467958a46253baee85fd175402c92bd3578cba0c`, then reran Chiron through the visible staging prompt bar. The rerun reached super -> worker VM -> vsuper -> implementation/verifier co-super, but produced an `evidence_pending` AppChangePackage because the worker VM could not capture browser-backed human proof.
current artifact state: Apps & Changes no longer trusts the static four-experiment seed portfolio; it derives visible changes from product AppChangePackage records, labels machine receipts as insufficient, opens only existing VText narratives, blocks Verify until candidate preview reports healthy, and deployed review evidence rejects blocked/failing refs, unavailable media, build-only proof, recommendation prose, benchmark refs without numeric measurements, and stale-source laundering. Pushed commit `2995853a9ea80c23a6268035b5a87737e894d9eb` adds Playwright browser evidence capture support to worker VMs and hardens worker/super prompts against fixture-only UI proof; CI then exposed one stale prompt-contract test expectation, now corrected locally for the next push.
what shipped: `08f165c7a31debc57b8e520ce7d888d85c475d89`, `f15a7e0727bbbb1edd69048f752fcfc05162e1a7`, `6e176d65640a53e7810f68ad7c0b8054f6b63e66`, and `467958a46253baee85fd175402c92bd3578cba0c` shipped through CI/deploy; staging `/health` reported proxy/upstream at `467958a46253baee85fd175402c92bd3578cba0c`. Worker-browser evidence-capture commit `2995853a9ea80c23a6268035b5a87737e894d9eb` is pushed but not deployed because CI run `26315465920` failed on a stale test assertion expecting the old worker prompt text; the follow-up must pass CI before deploy.
what was proven: frontend build passed; Apps & Changes Playwright specs passed locally and against staging; focused runtime/proxy human-proof tests passed in the repo Nix dev shell; staging product API for a new user returned no static seed packages/adoptions/acceptances; build-only/unavailable-media package `6e92f95c-a5b7-47c1-ad1f-512f9346266e` returned `evidence_pending` on deployed `467958a`; Chiron rerun trajectory `2c5be307-50da-4f4e-a8a4-302aece43563` produced VText doc `6da53e77-6076-47c0-9ecd-65ffb51d5a41` with final revision `75c8605c-3920-4922-a560-c9816ae74bd8`, worker VM `vm-bcc11e240e20aee93bc155dd4d7e482c`, worker `worker-1fa3f60305a48f6a`, and AppChangePackage `0ff4ba86-a53d-4651-9d1d-5153cebe6210`; review evidence correctly remained `evidence_pending` because successful screenshot/video/benchmark proof was missing. Local verification for the worker-browser patch passed `git diff --check`, focused runtime tests, Nix service environment eval, and `nix build .#packages.x86_64-linux.guest-image --dry-run`.
unproven or partial claims: Chiron has not yet produced real Shelf screenshots/video from a worker VM with Playwright installed; Motion, Liquid, and Python mode have not been rerun; recipient install/adoption and rollback proof are not reached; worker-browser evidence capture is not yet deployed to staging; warm existing worker/user computers may need replacement after deploy to pick up the new guest image and environment.
belief-state changes: VText dashboard causality exists and can receive multiple revisions, but updates can still collide with completed VText mutation windows; super/vsuper now demonstrably delegates to worker VM and co-super children; the evidence gates successfully prevented a build-only Chiron candidate from being reviewable; the main current blocker is not review-state laundering but worker/candidate browser evidence capture and the prompt contract that tells verifiers not to accept static markup fixtures.
remaining error field: push/deploy the prompt-contract test fix after `2995853a9ea80c23a6268035b5a87737e894d9eb`, verify staging identity, ensure new worker VMs expose Playwright browser binaries, then rerun Chiron and require real screenshots/video or a precise browser-capture blocker without allowing fixture-only tests to count as behavior proof.
highest-impact remaining uncertainty: adding Playwright browsers to the worker VM image has a large closure cost and may still expose missing runtime dependencies or resource pressure. If deployed worker browser capture is too heavy or flaky, the harness needs a first-class product evidence-capture path that is bounded, reproducible, and independent of ad hoc worker-local browser setup.
next executable probe: push/deploy the prompt-contract test fix for `2995853a9ea80c23a6268035b5a87737e894d9eb`, verify staging identity, run a focused Chiron product-path rerun against a fresh worker VM, and inspect whether the new package has real screenshot/video/benchmark refs and a real app/component/product-path verifier rather than fixture-only markup.
suggested resume goal string: use the one-line goal string in this file
evidence artifact refs: GitHub Actions runs `26311137788`, `26312601119`, `26313776911`, `26314331457`, and failed run `26315465920`; staging `/health` deployed commits `08f165c7a31debc57b8e520ce7d888d85c475d89`, `f15a7e0727bbbb1edd69048f752fcfc05162e1a7`, `6e176d65640a53e7810f68ad7c0b8054f6b63e66`, and `467958a46253baee85fd175402c92bd3578cba0c`; `test-results/human-proof-staging-08f165c/`; `test-results/human-proof-staging-f15a7e/`; `test-results/human-proof-staging-6e176d6/`; `test-results/human-proof-staging-467958a/`; `test-results/human-proof-chiron-delegate-error-capture-08f165c/`; `test-results/human-proof-chiron-rerun-f15a7e/`; `test-results/human-proof-chiron-rerun-467958a/`; `npm --prefix frontend run build`; `npm --prefix frontend run e2e -- web-surface-rationalization.spec.js`; `nix develop .# --command go test -count=1 ./internal/runtime -run 'TestAppChangePackageReviewEvidence|TestPublishAppChangePackageToolPublishesWithoutGitHubPush|TestPrivateAppChangePackageIsNotVisibleAcrossOwners|TestAppChangePackageMigratesAcrossCandidateComputers'`; `nix develop .# --command go test -count=1 ./internal/runtime -run 'TestAppChangePackageReviewEvidenceRequiresNarrativeAndMediaForHumanReview|TestPublishAppChangePackageToolPublishesWithoutGitHubPush'`; `nix develop .# --command go test -count=1 ./internal/runtime -run 'TestAppChangePackage|TestPrivateAppChangePackage|TestInternalAppChangePackage|TestPublishAppChangePackageToolPublishesWithoutGitHubPush|TestDelegateWorkerVMToolRunsWorkerRuntimeAndCollectsExport'`; `nix develop .# --command go test -count=1 ./internal/proxy -run 'TestAppChangePackageReviewEvidence'`; `nix develop .# --command go test -count=1 ./internal/runtime -run 'TestWorkerRepoBootstrapPromptIncludesHumanEvidenceBrowserContract|TestInstallDefaultAgentToolsProfiles'`; `nix develop .# --command go test -count=1 ./internal/runtime -run 'TestWorkerRepoBootstrapPromptIncludesHumanEvidenceBrowserContract|TestPrepareRemoteWorkerRepoBootstrapUsesConfiguredSourceOutsideGit'`; Nix eval of `PLAYWRIGHT_BROWSERS_PATH`, `PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD`, and `PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS`; `nix build .#packages.x86_64-linux.guest-image --dry-run`
rollback refs: revert `08f165c7a31debc57b8e520ce7d888d85c475d89` for the first shipped gate changes; revert `f15a7e0727bbbb1edd69048f752fcfc05162e1a7` for the second classifier change; revert `6e176d65640a53e7810f68ad7c0b8054f6b63e66` for the runtime/tool-contract classifier change; revert `467958a46253baee85fd175402c92bd3578cba0c` for stale-source review sanitization; revert `2995853a9ea80c23a6268035b5a87737e894d9eb` plus its prompt-contract follow-up if worker-browser evidence capture bloats or destabilizes worker VMs after deploy
```

## Stopping Condition

Complete only when:

- false-proof product paths are removed or replaced;
- VText mission dashboard causality is proven;
- candidate preview health gates are enforced;
- human-proof review states exist;
- the four experiments are rerun sequentially or precisely blocked;
- each experiment has a plain-English VText report, screenshot/video or
  benchmark evidence, Trace/run-acceptance refs, recommendation, residual risks,
  and rollback/package/adoption refs where relevant;
- staging identity and deployed product proof are recorded for any platform
  harness changes.

If the mission cannot reach all of that, report `checkpoint_incomplete` or
`blocked_incomplete`, update this section with the exact resumption state, and
continue/redirect/delegate the next safe executable probe if it is inside the
current authority boundary.
