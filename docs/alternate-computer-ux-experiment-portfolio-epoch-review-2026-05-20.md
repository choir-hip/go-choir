# Alternate Computer UX Experiment Portfolio Epoch Review

Date: 2026-05-20

Status: `checkpoint_incomplete`

This is a forensic review of the long alternate-computer UX experiment
portfolio run. It should be read beside:

- [mission-alternate-computer-ux-experiment-portfolio-v0.md](mission-alternate-computer-ux-experiment-portfolio-v0.md)
- [alternate-computer-ux-experiment-portfolio-certificate-2026-05-20.md](alternate-computer-ux-experiment-portfolio-certificate-2026-05-20.md)

The mission should not be considered complete. The run produced a meaningful
checkpoint: four experiment lanes became owner-pullable AppChangePackages with
recipient build/adoption evidence. It did not complete hands-on owner QA in an
owner-controlled computer, richer Liquid/Python benchmarks, or a fully usable
VText-first observability/reporting loop.

## Executive Summary

The run did not merely build four UX ideas. It pushed the promotion substrate
hard enough to expose and repair several real platform problems:

- old promotion/export paths were removed from the acceptance path;
- AppChangePackage -> recipient adoption -> real recipient build became the
  current proof path;
- worker-published packages became product-visible and pullable;
- source-run and recipient-run acceptance synthesis learned to recognize
  package/adoption evidence;
- persistent-super concurrent inbox handling was fixed;
- repeated worker delegation/package duplication was fixed;
- Node B disk/auth failure was root-caused to full disk pressure from VM state,
  Nix generations, and journals;
- stale terminal worker/candidate VM-state reclaim was deployed.

The best artifact is not a platform-default merge. It is a package portfolio the
owner can pull into an owner-controlled computer for review:

| Lane | Package | Source Acceptance | Recipient Acceptance | Status |
| --- | --- | --- | --- | --- |
| Chiron Shelf observability | `28433c19-5d02-416f-9368-de56390e1927` | `runacc-a352091712fdd96aa00d`, export-level accepted | `runacc-c3d70f753b81fd591442`, promotion-level accepted | owner-pullable |
| Process/window/agent animation | `98b98c73-eef0-4a88-a6f5-b7dfe695be09` | `runacc-5784f0028b01753ad0ca`, export-level accepted | `runacc-3b54c9ae8dac2337184a`, promotion-level accepted | owner-pullable |
| Choir Liquid Material Engine | `1dad3dfc-7f83-4b22-bfb5-7f1714159f66` | `runacc-0194bfce2cdecffea784`, export-level accepted | `runacc-d144087c5ffacad2e147`, promotion-level accepted | owner-pullable |
| Python code mode A/B | `f31edbc8-1b43-44f5-82a1-834dce4833ca` | `runacc-a7e993d7c4f56d4420d9`, export-level accepted | `runacc-45495b8caebc3e1b82c5`, promotion-level accepted | owner-pullable |

The core review-model correction is important: direct login to generated source
experiment accounts is not the QA path. The QA path is package mobility. The
owner should pull selected packages into an owner-controlled computer, inspect
and run them there, then iterate, abandon, or promote.

## Epochs

### Epoch 0: Preflight Hard Cutover

Approximate artifact range: `305b0e4`, `98b73c5`, `52e0612`

Purpose: remove false-success paths before the experiment portfolio.

What happened:

- The old `export_patchset` and `/api/promotions` acceptance path was removed
  from the current proof model.
- Real recipient builds became mandatory for adoption/promotion evidence.
- Dolt/ICU local build friction was clarified: normal repo tests that touch the
  Dolt-backed runtime need the repo dev shell or equivalent environment.
- Preflight proof established that AppChangePackage/adoption/run-acceptance was
  the only acceptable movement path for this mission.

Learning:

- This was necessary. Without the hard cutover, later package/adoption evidence
  could have been falsely satisfied by older synthetic or export-only paths.
- The Dolt/ICU situation is not a product feature, but it remains a developer
  environment footgun. Worker/candidate environments should enter the dev shell
  for local Dolt-backed tests.

### Epoch 1: Wave 0 Substrate Proof

Approximate artifact range: `d6790ba`, `d1f3bb5`

Purpose: prove the package/adoption/recipient-build path before running the four
experiments.

Evidence:

- `test-results/alternate-portfolio-wave0-deployed/alternate-portfolio-wave0-evidence.json`
- package `package-alt-portfolio-wave0-1779270395944`
- adoption `adoption-alt-portfolio-wave0-1779270395944`
- run acceptance `runacc-19c10e4b57c2f0828c5b`
- VText doc `ab093136-d504-4745-8f42-d9d30a008bdc`

What happened:

- A package was adopted into a recipient computer and rebuilt with recipient
  runtime/UI digests.
- A run-acceptance false-success edge was found and patched so blocked
  invariant checks cannot still produce accepted records.

Learning:

- Wave 0 did its job: it kept the portfolio from starting on top of a fake
  acceptance surface.
- The run already showed that acceptance synthesis needs to be treated as a
  verifier object, not as a success-label formatter.

### Epoch 2: First Wave 1 Attempts

Approximate artifact range: `a0a4d6c`, `f11e848`, `74230a3`

Purpose: run Chiron Shelf observability and process/window/agent animation as
two concurrent candidate-computer lanes.

Evidence:

- `test-results/alternate-portfolio-wave1-diagnostics-a0a4d6c-20260520T1216/alternate-portfolio-wave1-evidence.json`
- `test-results/alternate-portfolio-wave1-deployed-f11e848-20260520T1310/alternate-portfolio-wave1-evidence.json`
- `test-results/alternate-portfolio-wave1-deployed-74230a3-20260520T1447/alternate-portfolio-wave1-evidence.json`

What happened:

- The initial product path did not make worker-published packages visible enough
  to recipient/owner review.
- Worker-local Trace evidence showed real package attempts, but product package
  APIs could not yet reliably pull them.
- Commit `74230a3` added the product-safe worker package visibility path:
  worker-published AppChangePackages are fetched through internal worker runtime
  package endpoints, mirrored into the active runtime store, and made pullable
  through `/api/app-change-packages/pull`.

Learning:

- Trace-local package summaries are not enough. A package is only reviewable
  when the product package/adoption APIs can inspect and pull it.
- Two-lane concurrency was useful because it exposed attribution and package
  mobility gaps quickly.

### Epoch 3: Wave 2, Source/Recipient Acceptance, And Duplicate Packages

Approximate artifact range: `65956c4`, `09a95ad`

Purpose: run Liquid Material Engine and Python code mode A/B while improving
run acceptance.

Evidence:

- `test-results/alternate-portfolio-wave2-deployed-65956c4-runacc-20260520T170222/alternate-portfolio-wave2-evidence.json`
- `test-results/alternate-portfolio-wave2-deployed-09a95ad-runacc-20260520T180144/alternate-portfolio-wave2-evidence.json`

What happened:

- Commit `65956c4` bridged recipient adoption evidence into promotion-level
  run acceptance.
- Commit `09a95ad` allowed source trajectories that delegate worker work and
  publish AppChangePackages to synthesize export-level accepted records.
- The harness learned to treat multiple package identities for one lane as a
  duplicate-package blocker instead of a clean success.
- One Wave 2 run succeeded for Liquid but cleanly blocked Python when no
  package appeared.

Learning:

- Duplicate packages are not harmless. They confuse identity, review, rollback,
  and the idea of "one migrating AppChangePackage."
- Source-run acceptance and recipient-run acceptance are different facts and
  both matter.

### Epoch 4: Persistent Super Inbox Fix

Commit: `6db0632`

Purpose: fix concurrent prompt delivery so the second lane is not swallowed by a
long-running first lane.

Evidence:

- `test-results/alternate-portfolio-wave2-deployed-6db0632-runacc-20260520T191506/alternate-portfolio-wave2-evidence.json`
- GitHub Actions run `26183958385`
- staging `/health` reported `6db0632541a6a408fedf3d48ad129f2211d368fa`

What happened:

- Root cause: generic inbox injection could mark pending deliveries delivered
  before the active model run actually handled them.
- Fix: persistent-super inbox runs own only the delivery batch in their initial
  prompt, skip live inbox injection during the run, and start a follow-up super
  run if deliveries remain pending.
- After this, Wave 2 produced one Liquid package and one Python package with
  accepted source and recipient records.

Learning:

- Concurrency is safe only when delivery accounting is explicit.
- The "two-lane" portfolio is an effective stress test for the real super/vsuper
  scheduler.

### Epoch 5: Worker Delegation Dedupe

Commit: `575ff30`

Purpose: stop one super run from repeatedly delegating to the same worker/profile
and producing multiple packages for one lane.

Evidence:

- `test-results/alternate-portfolio-wave1-deployed-6db0632-runacc-20260520T194502/alternate-portfolio-wave1-evidence.json`
- `test-results/alternate-portfolio-wave1-deployed-575ff30-runacc-20260520T202754/alternate-portfolio-wave1-evidence.json`
- GitHub Actions run `26187590374`
- staging `/health` reported `575ff3014a85524da4233e60ce44345804d46807`

What happened:

- A `6db0632` Wave 1 audit showed Chiron produced three package identities for
  one lane.
- Root cause: repeated same-run `delegate_worker_vm` calls could start or
  collect separate package work for the same worker/profile.
- Fix: `delegate_worker_vm` became idempotent for matching same-run
  worker/profile terminal results and skips same-turn duplicate delegate
  payloads.
- Fresh Wave 1 on `575ff30` produced exactly one selected Chiron package and one
  selected animation package.

Learning:

- The mission did not just produce experiment packages; it hardened
  Choir-in-Choir itself.
- The remaining repeated `spawn_agent`, `cast_agent`, `bash`, and `wait_agent`
  duplicate tool-call errors in traces suggest more idempotence/loop-shaping
  work remains.

### Epoch 6: Node B Disk/Auth Incident

Artifact range: failed Wave 2 attempts at `575ff30`, then operational recovery

Evidence:

- `test-results/alternate-portfolio-wave2-deployed-575ff30-runacc-20260520T204915/error-context.md`
- `test-results/alternate-portfolio-wave2-deployed-575ff30-runacc-20260520T204915/test-failed-1.png`
- `test-results/alternate-portfolio-wave2-deployed-575ff30-runacc-20260520T211157/error-context.md`
- `test-results/alternate-portfolio-wave2-deployed-575ff30-runacc-20260520T211157/test-failed-register-502.png`
- `test-results/alternate-portfolio-wave2-deployed-575ff30-runacc-20260520T212756/alternate-portfolio-wave2-evidence.json`

What happened:

- First failure: `/api/app-change-packages?limit=100` returned `401` after a
  long run. The harness needed product-session renewal during long package waits.
- Second failure: `/auth/register/begin` returned `502`.
- Root cause: Node B root filesystem reached 100%; auth service crashed with
  SQLite WAL disk I/O errors/SIGBUS.
- Recovery: journal vacuum reclaimed enough space to restart auth; Nix system
  generation cleanup plus `nix store gc` reclaimed roughly 201 GiB.
- Fresh Wave 2 after recovery succeeded with one Liquid and one Python package.

Learning:

- Long product-path proof is an infrastructure load test.
- VM state and Nix generations must be managed as first-class operational
  concerns, not ad hoc cleanup.
- Session renewal must be a standard Playwright/product-path proof primitive.

### Epoch 7: Stale VM-State Reclaim

Commit: `664dc1b`

Purpose: prevent stale terminal worker/candidate VM state from filling Node B.

Evidence:

- GitHub Actions run `26193426970`
- staging `/health` reported proxy and sandbox commit
  `664dc1b7949e852705daebd2c3f94416e61733ab`
- focused tests: `go test ./internal/vmctl ./internal/vmmanager ./cmd/vmctl`

What happened:

- Low `/var/lib/go-choir/vm-state` free space became a pressure signal.
- Bounded deletion of stale terminal worker and unpublished candidate VM-state
  directories was added.
- Active, primary, published, premium, recent, and critical work are protected.

Learning:

- This is necessary but not yet fully proven under another large run.
- Nix generation/journal cleanup remains a separate operational axis.

### Epoch 8: Review Model Correction And Owner Pull Probe

Commits: `d737a71`, `9f58666`

Purpose: correct the mission away from direct login to generated source accounts
and toward owner-controlled package pull/adoption.

Evidence:

- `docs/alternate-computer-ux-experiment-portfolio-certificate-2026-05-20.md`
- `test-results/owner-pull-probe-20260520223845/owner-pull-chiron-evidence.json`
- `test-results/owner-pull-probe-20260520224725/owner-pull-chiron-renewal-evidence.json`

What happened:

- The docs were updated to make package mobility the review path.
- A Chiron owner-pull probe into a fresh authenticated staging recipient account
  verified and promoted the package.
- The first probe hit a `401` at promote after the long recipient build because
  the ad hoc script did not renew the session.
- The second probe renewed during promote and advanced the target source lineage,
  but its run-acceptance follow-up is still recorded as `blocked_incomplete`
  because fetching the synthesized acceptance detail returned `404`.

Learning:

- The architecture correction is right: the owner does not need to log into
  every source experiment account.
- The next owner-review proof should use the actual owner account/computer or a
  deliberately owner-controlled review computer, not another generated
  throwaway recipient if the goal is hands-on QA.
- The run-acceptance detail/fetch path needs a focused check before the next
  long run relies on it as durable owner-facing evidence.

## Screenshots And Trace Clips

The final selected waves produced screenshots and Playwright traces. I did not
find original `.webm` or `.mp4` recordings for this particular mission under
`test-results` or `frontend/test-results`, even though the Playwright spec
requested video. The pass traces contain JPEG frame snapshots; the MP4 clips
below were reconstructed from those trace snapshots after the run for review
convenience and should not be described as original screen recordings.

Final proof screenshots:

- Wave 1 source desktop:
  `test-results/alternate-portfolio-wave1-deployed-575ff30-runacc-20260520T202754/alternate-portfolio-wave1-source-desktop.png`
- Wave 2 source desktop:
  `test-results/alternate-portfolio-wave2-deployed-575ff30-runacc-20260520T212756/alternate-portfolio-wave2-source-desktop.png`

Final proof traces:

- Wave 1 trace:
  `test-results/alternate-portfolio-wave1-deployed-575ff30-runacc-20260520T202754/trace.zip`
- Wave 2 trace:
  `test-results/alternate-portfolio-wave2-deployed-575ff30-runacc-20260520T212756/trace.zip`

Derived review clips:

- Wave 1:
  `test-results/alternate-portfolio-epoch-review-20260520/wave1-trace-review.mp4`
- Wave 2:
  `test-results/alternate-portfolio-epoch-review-20260520/wave2-trace-review.mp4`

Failure screenshots:

- Session/auth expiry failure:
  `test-results/alternate-portfolio-wave2-deployed-575ff30-runacc-20260520T204915/test-failed-1.png`
- Auth/register 502 failure:
  `test-results/alternate-portfolio-wave2-deployed-575ff30-runacc-20260520T211157/test-failed-register-502.png`

## VText Observability Gap

VText was used, but not well enough.

What exists:

- Wave 0 VText doc: `ab093136-d504-4745-8f42-d9d30a008bdc`
- Wave 1 VText doc: `1d74a744-23be-4c07-8357-54beea5010ab`
- Wave 2 VText doc: `12bf4059-5036-47fd-9209-053729d80055`

Problem:

- These VTexts are short receipt-style summaries.
- Candidate computers did not publish rich per-lane VTexts that explain the
  design intent, changed files, verification, screenshots/video, benchmark
  results, risks, and promotion recommendation in a human-reviewable narrative.
- Trace remained the main evidence object, but Trace is still too hard to read
  as the primary owner-facing observability surface.

Recommended invariant for future portfolio runs:

```text
Every experiment lane must publish a lane VText before it can be called
owner-reviewable.

That VText must include:
- objective and marker;
- source computer/ref and AppChangePackage id;
- changed files and design summary;
- verifier result and commands;
- source acceptance and recipient acceptance ids;
- screenshots/video/benchmark links;
- rollback refs;
- promotion recommendation;
- explicit residual risks.
```

Trace should remain the event/evidence ledger. VText should become the
owner-readable interpretation layer.

## MissionGradient Learnings

Persistence helped:

- The mission kept pushing through real blockers instead of stopping at the
  first broken package, bad acceptance record, auth timeout, or full disk.
- Evidence gates prevented false completion several times.
- The mission found platform bugs that a narrow feature task would not have
  discovered.

Persistence overreached:

- The mission doc became extremely dense and hard for a human to review.
- Restarts made the artifact field noisy: many historical test-result dirs and
  package ids now exist.
- The run generated product evidence but did not produce enough owner-facing
  narrative evidence in VText.

Concurrency helped:

- Two-lane concurrency exposed inbox delivery bugs and duplicate worker
  delegation/package identity bugs.
- It produced a realistic stress test for super/vsuper/cosuper coordination.

Concurrency hurt:

- It made causality and package attribution harder to inspect.
- Long lanes created session-renewal and infrastructure pressure failures.
- More than two concurrent lanes would currently be premature without better
  observability and cleanup.

Evidence gates worked:

- Missing product-visible package evidence blocked success.
- Duplicate package identities blocked success.
- Source-run acceptance and recipient-run acceptance were kept separate.
- The docs now call the mission `checkpoint_incomplete`, not complete.

What should simplify later:

- The mission should produce an epoch report automatically after long runs.
- VText should be first-class evidence, not a late receipt.
- Run acceptance should have one obvious detail/list/read path.
- Package trace acceptance fields that remain `docs-level`/blocked should be
  cleaned up or removed from the main success surface.
- Long proof harnesses should have built-in session renewal, storage cleanup
  observation, and artifact inventory.

## Current State

Proven:

- Four experiment packages are owner-pullable in the product-path sense.
- Each selected package has source export-level acceptance.
- Each selected package has recipient promotion-level acceptance.
- The selected package identities are clean in the final `575ff30` Wave 1 and
  Wave 2 proof sets.
- Worker-published packages can cross into recipient computers through product
  package/adoption APIs.
- Stale VM-state cleanup is deployed.

Not proven:

- The owner has manually reviewed these packages inside `ymnath@choir-ip.com`.
- Liquid has measured mobile Safari/WebKit performance and resource evidence.
- Python mode has a matched bash-vs-Python A/B table from real comparable runs.
- VText is a sufficient observability surface for candidate computers.
- Stale VM-state reclaim prevents the next large run from hitting disk pressure.
- Run-acceptance detail retrieval is clean for the latest owner-pull probe.

## Recommended Next Action

Do not restart the whole 13-hour mission.

Run a shorter review mission:

```text
Use the package refs in
docs/alternate-computer-ux-experiment-portfolio-certificate-2026-05-20.md.
Pull one selected package, preferably Chiron, into an owner-controlled
review computer. Verify adoption, source lineage, recipient build, rollback,
and owner-readable VText evidence. Fix the run-acceptance detail/fetch gap if
it reproduces. Then decide which experiment deserves a second iteration.
```

After that, run targeted follow-ups:

- Liquid benchmark mission: WebKit/mobile + desktop performance/resource cost.
- Python benchmark mission: matched bash-vs-Python task set.
- VText evidence mission: make candidate computers publish rich lane reports.
- VM-state observation mission: run a bounded portfolio stress test while
  observing stale state reclaim.
