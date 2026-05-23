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
- Playwright screenshots and video from the bounded `worker-playwright`
  evidence class, not from every ordinary user/candidate VM;
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
last checkpoint: deployed worker-browser evidence patch `6135a245cb4b1a15dd1a3d5b362320025ca0d321`, verified staging `/health` with proxy and upstream both serving that SHA, then reran Chiron through the visible staging prompt bar in fresh product accounts.
current artifact state: Apps & Changes no longer trusts the static four-experiment seed portfolio; it derives visible changes from product AppChangePackage records, labels machine receipts as insufficient, opens only existing VText narratives, blocks Verify until candidate preview reports healthy, and deployed review evidence rejects blocked/failing refs, unavailable media, build-only proof, recommendation prose, benchmark refs without numeric measurements, stale-source laundering, and `human_summary` prose laundered into causal narrative refs. The browser substrate split is now explicit: ordinary VMs stay lean and Obscura-backed; high-fidelity screenshots/video require a separate `worker-playwright` evidence VM class.
what shipped: `08f165c7a31debc57b8e520ce7d888d85c475d89`, `f15a7e0727bbbb1edd69048f752fcfc05162e1a7`, `6e176d65640a53e7810f68ad7c0b8054f6b63e66`, `467958a46253baee85fd175402c92bd3578cba0c`, `2995853a9ea80c23a6268035b5a87737e894d9eb`, `6135a245cb4b1a15dd1a3d5b362320025ca0d321`, and `06bfeed3c5d742e4eb85474f8e1c71e76e6c67f7` shipped through CI/deploy. GitHub Actions run `26315718981` passed; deploy job `77474487610` completed; GitHub Actions run `26320124229` passed and deployed `06bfeed`; staging `/health` reported proxy/upstream at `06bfeed3c5d742e4eb85474f8e1c71e76e6c67f7`.
what was proven: frontend build passed; Apps & Changes Playwright specs passed locally and against staging; focused runtime/proxy human-proof tests passed in the repo Nix dev shell; staging product API for a new user returned no static seed packages/adoptions/acceptances; build-only/unavailable-media package `6e92f95c-a5b7-47c1-ad1f-512f9346266e` returned `evidence_pending` on deployed `467958a`; deployed `06bfeed` product API proof showed a positive package reaching `human_reviewable`, while summary-only and build-only packages stayed `evidence_pending`. Local verification for the worker-browser patch passed `git diff --check`, focused runtime tests, Nix service environment eval, and `nix build .#packages.x86_64-linux.guest-image --dry-run`. After `6135a`, a fresh Chiron rerun through the visible prompt bar produced AppChangePackage `ab67194c-c19b-4d18-b05d-ea9ba5da3af2` from source account `e4a57519-bb63-48df-bbed-e15d99d0c023`, source computer `go-choir-candidate-worker-14b177e5`, package app id `human-proof-chiron-1779491207148`, source ledger commit `d9fc7a8d048235e2bc6f3f32879d2cb02f598f85`, and UI delta including `frontend/src/lib/ChironStream.svelte` plus `Desktop.svelte` mounting. Review evidence reached `human_reviewable` with screenshot refs, VText/narrative refs, and a frontend build check.
unproven or partial claims: the Chiron package was `published_private`, so cross-account owner pull failed with `404 app change package not found`; there is no recipient adoption, install, rollback, or owner-pull proof. The Chiron proof still lacks video and used injected product event-listener activity rather than a full authenticated live websocket run. The source Trace remained `running/live` after prompt completion. Motion, Liquid, and Python mode have not been rerun.
failed follow-up probes: a recipient pull attempt for private package `ab67194c-c19b-4d18-b05d-ea9ba5da3af2` correctly failed with 404. Subsequent prompts that explicitly required an owner-pullable `unlisted` package (`d55613d5-3087-400e-bf44-b445d44b8a27`, `c8d7273e-a1cb-49c0-92b7-eb3256403bc9`, `e83b5f43-eff9-4d50-b669-d8a269909922`) produced no AppChangePackage and stalled with idle Trace. The first apparent 401 during this series was a proof-runner bug caused by raw `fetch` without session renewal; renewal-aware runs avoided that 401 but still produced no package. Diagnostics were weakened because proof-observer storage state was not persisted after refresh-cookie rotation.
belief-state changes: VText dashboard causality exists and can receive multiple revisions, but updates can still collide with completed VText mutation windows; super/vsuper now demonstrably delegates to worker VM and co-super children; the evidence gates prevented build-only Chiron candidates from being reviewable; Obscura is the right default browser substrate for ordinary VMs, but Playwright/Chromium is still needed for high-fidelity owner proof and therefore belongs in a special evidence-worker class. Owner-pullable package production and end-to-end live behavior proof remain unstable.
browser substrate finding: Playwright verifier/evidence capture and Obscura product-browser substrate are different tracks. `ddc5069dd69e020fb0801941b4f80764559c5ec7` replaced Playwright-in-worker-VM with pinned Obscura and treats Chrome/Playwright as external verifier tooling. `f452977af7532e99daf7850fd0b635915c3082e6` added a narrow internal `hibernate-worker` vmctl endpoint so proof workers can be suspended without killing parent desktops. GitHub Actions run `26318375143` deployed `ddc5069`; run `26318837141` deployed `f452977`. Fresh staging proof on worker `worker-ed239185b8596177` / VM `vm-840aa642db68196a0e4359085cfb28a0` showed `/api/browser/capabilities` reporting Obscura backend extraction as ready, created browser session `b134f3a2-70e6-4199-af4d-fc70c77eeb8c`, navigated to `https://example.com`, returned expected text/link snapshots, closed the session, and hibernated both proof worker and parent. Obscura is therefore proven for VM-local public-page text/html/link extraction, but not for Choir authenticated UI sessions, arbitrary actions, screenshots, or video. The runtime contains optional CDP screenshot/fill/click code paths, but the deployed worker proof did not enable or prove them. Screenshots/video should still be captured by bounded external verifier tooling until that contract is separately proven. The tangent mission and capability matrix are captured in `docs/mission-obscura-worker-browser-substrate-v0.md`.
browsing/evidence architecture update: the next local platform patch adds `worker-playwright` as a dedicated VM image class with Playwright/Chromium installed, while ordinary `worker-small`, `worker-medium`, `worker-large`, user, and candidate VMs remain on the lighter Obscura-backed image. `request_worker_vm` exposes `worker-playwright` only for bounded browser evidence such as screenshots/video, and the vmctl image profile wiring rejects missing or incomplete Playwright guest image artifacts. Local proof passed focused vmctl/vmmanager/runtime tests, `git diff --check`, `nix eval .#packages.x86_64-linux.guest-image-playwright.name --raw`, and `nix build .#packages.x86_64-linux.guest-image-playwright --dry-run`. This patch is not platform fact until committed, pushed, deployed, and proven on staging.
deployed review-gate hardening: `06bfeed3c5d742e4eb85474f8e1c71e76e6c67f7` is deployed and proves the reviewability classifier no longer treats `human_summary` prose alone as a causal VText narrative, and no longer lets build receipts alone masquerade as human-reviewable feature evidence. Proof JSON: `test-results/human-proof-staging-06bfeed/deployed-human-proof-gate-product-proof.json`.
remaining error field: rerun Chiron as an `unlisted` owner-pullable package through visible product-path Choir-in-Choir, with proof-observer session renewal persisted, Trace details saved independently of browser cookies, causal VText dashboard updates, screenshots/video from bounded external verifier tooling, recipient pull/build/adoption proof, and rollback refs.
highest-impact remaining uncertainty: whether Choir-in-Choir can now produce a real owner-pullable Chiron change with human evidence and adoption proof without Codex hand-coding the feature. Any remaining Chrome/Playwright use should stay outside user/candidate VMs as verifier-only tooling, not a per-VM dependency.
next executable probe: land the dedicated `worker-playwright` evidence-worker patch through the platform loop, prove staging can request/boot that class without inflating ordinary VMs, then run the Chiron rerun sequentially through the visible product path, requiring an unlisted package, live VText narrative, `worker-playwright` screenshots/video, recipient pull/build/adoption proof, and rollback refs. Use Obscura for ordinary VM-local web extraction during the run. If Chiron fails, root-cause whether the failure is package visibility, super/vsuper prompting, VText update consumption, evidence capture, adoption plumbing, Playwright evidence-worker liveness, or a missing Obscura CDP/auth capability.
suggested resume goal string: use the one-line goal string in this file
evidence artifact refs: GitHub Actions runs `26311137788`, `26312601119`, `26313776911`, `26314331457`, failed run `26315465920`, deployed run `26315718981`, deployed Obscura run `26318375143`, and deployed worker-cleanup run `26318837141`; deploy jobs `77474487610`, `77482417092`, and `77483760579`; staging `/health` deployed commits `08f165c7a31debc57b8e520ce7d888d85c475d89`, `f15a7e0727bbbb1edd69048f752fcfc05162e1a7`, `6e176d65640a53e7810f68ad7c0b8054f6b63e66`, `467958a46253baee85fd175402c92bd3578cba0c`, `6135a245cb4b1a15dd1a3d5b362320025ca0d321`, `ddc5069dd69e020fb0801941b4f80764559c5ec7`, and `f452977af7532e99daf7850fd0b635915c3082e6`; `test-results/human-proof-staging-08f165c/`; `test-results/human-proof-staging-f15a7e/`; `test-results/human-proof-staging-6e176d6/`; `test-results/human-proof-staging-467958a/`; `test-results/human-proof-chiron-delegate-error-capture-08f165c/`; `test-results/human-proof-chiron-rerun-f15a7e/`; `test-results/human-proof-chiron-rerun-467958a/`; `test-results/human-proof-chiron-rerun-6135a/`; `test-results/human-proof-chiron-rerun-6135a-unlisted/`; `test-results/human-proof-chiron-rerun-6135a-unlisted-renewal/`; `test-results/human-proof-chiron-rerun-6135a-unlisted-diagnostics/`; `npm --prefix frontend run build`; `npm --prefix frontend run e2e -- web-surface-rationalization.spec.js`; focused runtime/proxy human-proof tests; focused worker prompt tests; Nix eval of VM browser environment; `nix build .#packages.x86_64-linux.guest-image --dry-run`; local hardening proof: `nix develop .# --command go test -count=1 ./internal/runtime -run 'TestAppChangePackageReviewEvidenceRequiresNarrativeAndMediaForHumanReview|TestPublishAppChangePackageToolPublishesWithoutGitHubPush' -v`, `nix develop .# --command go test -count=1 ./internal/runtime -run 'TestAppChangePackageReviewEvidence|TestAppChangePackage|TestRunAcceptance'`, and `npm --prefix frontend run build`; `/Users/wiz/obscura` local branch `choir/playwright-parity-audit-2026-05-10`; `/Users/wiz/obscura-upstream-merge` branch `codex/choir-obscura-upstream-merge-2026-05-22`; Obscura commit `348a651e287ad370546762e78fc2095a7d33dc93`; Node A `/tmp/go-choir-obscura-probe`; Obscura output `/nix/store/gc6qvq656dm1wvxid97cpcxk69pfs9g6-obscura-0.1.0-choir-348a651`; guest image output `/nix/store/8fimb6vls8xj6ld9y1nyj0lyn5620wam-go-choir-guest-image`; fresh Obscura proof user `obscura-proof-1779497627`; proof worker `worker-ed239185b8596177`; proof VM `vm-840aa642db68196a0e4359085cfb28a0`; proof browser session `b134f3a2-70e6-4199-af4d-fc70c77eeb8c`; upstream compare `h4ckf0r0day/obscura` ahead by 36 commits.
rollback refs: revert `08f165c7a31debc57b8e520ce7d888d85c475d89` for the first shipped gate changes; revert `f15a7e0727bbbb1edd69048f752fcfc05162e1a7` for the second classifier change; revert `6e176d65640a53e7810f68ad7c0b8054f6b63e66` for the runtime/tool-contract classifier change; revert `467958a46253baee85fd175402c92bd3578cba0c` for stale-source review sanitization; revert `2995853a9ea80c23a6268035b5a87737e894d9eb` and `6135a245cb4b1a15dd1a3d5b362320025ca0d321` if old Playwright-in-VM evidence capture needs to be restored; revert `ddc5069dd69e020fb0801941b4f80764559c5ec7` if Obscura VM substrate regresses worker browser extraction; revert `f452977af7532e99daf7850fd0b635915c3082e6` if the internal worker hibernation endpoint causes vmctl lifecycle regressions.
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
