# Source-only freeze and candidate shots (2026-11-06)

Status: r2 (r0 SEND BACK by Devin and a Sonnet seat; r1 Devin ACCEPT with N1-N5, folded in; §6 adjudicates). Mutation class: red (bundle contract, Freeze,
materializer apply path). Mission: modular architecture
(`docs/definitions/choir-modular-architecture-2026-11-04.md`).

## 1. Problem

A platform (Go) self-development candidate cannot freeze. Evidence is on d@e
(ledger, 2026-11-06):

- apply-proof-3 (`selfdev-126285b6`) was refused by Freeze with "capsule
  release contains no frozen runtime artifacts".
  - The refusal is in `internal/capsule/executor.go` `StageGrantedRelease`.
  - A Go change must stage a capsule-built release tree: `release/bin/autoputer`
    plus `frontend/`.
  - The capsule cannot build it. Its module fetches time out, it has one CPU
    and about 1 GiB, and the link takes about 14 minutes
    (`docs/archive/problems/selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md`).
- Nothing from that tree is installed.
  - Every decision is consensus-made, so the materializer always host-builds
    (`self_development_materializer.go`, `hostBuildRequired`).
  - `layeredSelfDevelopmentManifest` replaces the files with the host build's
    binary and frontend (`self_development_host_build.go`).
  - The P5 report of 2026-10-15 named dropping the capsule build as "the next
    cut on the guest side". It was never made.
- The screens rule of 2026-11-04 asks every candidate, backend ones included,
  for screens, and for a before/after pair when it modifies existing files
  (`internal/capsule/screens_required.go`).
  - The after shot of a Go change needs the changed server running, which
    the capsule cannot build.
  - The shot run also cannot bind loopback when the cast's network is
    `forbidden`. That is by design in `capsule/seccomp.go`.
- apply-proof-2 completed without calling Freeze.
  - `Complete(completed)` with a platform diff and no operation is not
    refused.
- No platform-only operation has applied on d@e since 2026-11-02.

The screenshot tool is `scripts/choir-shot.mjs`: Node, Playwright and the
Nix Chromium, run as a capsule job. It builds the frontend with Vite (about
3 GiB on the first run in a capsule) and starts an autoputer over the
capsule's copy of the computer's state.

## 2. Owner rulings (2026-11-06)

- Screens for a platform candidate are shot after the host build. The
  host-built candidate is booted and shot. The before shot comes from the
  running build. The shot review judges the pair before the swap.
- "We should have a clean way to make screenshots, a yaegi-callable go
  function." Video too if resources allow. Reconsider Obscura and Lightpanda
  against Chromium.

## 3. Design

### 3.1 Freeze ships source (F1)

`StageGrantedRelease`:

- When the capsule staged no `release/` files and `emitSourcePatch` produced
  a patch, return no `Files` and no `Binary`, and skip the Option A reuse
  and the `hasFrontend` check.
- With neither files nor a patch, the existing refusal stands.
- A capsule that still stages a release tree keeps today's checks. The tree
  stays a test artifact.

Order inside `StageGrantedRelease`: `emitSourcePatch` moves ahead of the
empty-files refusal, the frontend check and the Option A reuse, so the
source-only decision is made with the patch in hand. The secret scans of
staged files are unchanged.

Bundle (`internal/capsule/transaction/builder.go` `Validate`):

- `RuntimeFiles` is required only when `SourcePatchSHA256` is empty.
- A source-only bundle carries `ReleaseBinary{Origin: "host_build"}` with no
  sha. `ValidateReleaseBinary` (`release_binary.go`) admits that origin only
  with an empty `RuntimeFiles` and a source patch.
- `RuntimeArtifactRef` keeps the `runtime-artifact:sha256:` prefix. For a
  source-only bundle it digests the canonical
  `{source_patch_sha256, source_patch_base_commit}`. `GeneratedArtifactRefs`
  holds that one ref (`tools_capsule.go` freeze, ~line 314-368).
- The bundle gains `ShotPlan` (§3.2), bound by sha like the screens. The
  strict decoders need it:
  - `yaegikernel/inspect_bundle.go` `bundleDraftMirror` (it uses
    `DisallowUnknownFields`), and the plan's own subtree in the inspector;
  - `selfdevprotocol` `CleanRebaseBundleFields`, so a rebase carries the
    plan unchanged.
- `frozenReleaseChanges` (`tools_capsule.go` ~530) skips the release-tree
  diff for a source-only bundle, because the tree is not the candidate. A
  capsule that built a test binary can still re-Freeze.

Materializer:

- A bundle with no `RuntimeFiles` and `hostBuildRequired == false` fails
  with a named cause. It never builds an empty plain manifest.
- Only the journal-only crash settlement reaches that branch, and it calls
  no updater.
- The stale witness text in `selfdev_release_witness.go` about "no host
  build … frontend-only" is corrected.

Completion guard (`runtime.go`):

- An engineering `Complete(completed)` whose capsule diff touches
  `/workspace/platform` refuses when the assignment holds no live
  operation, including after a refused Freeze that left the capsule dirty:
  "call choir.Freeze first; Complete does not freeze". The guard is new
  code; nothing in `runtime.go` does this today.

Prompts (`rlm_engineering_runtime.yaml`, `run_context.go` recipes):

- A platform change never builds `bin/autoputer` or stages a release tree.
- Freeze takes the source. The host builds it.
- Never set `GOPROXY=off`.
- The test `prompts_test.go` that asserts the build instruction is
  rewritten to assert its absence.

Later (F2, after d@e proves F1): delete the release-tree path from Freeze
outright (Option A reuse, `ReleaseBinaryReusedRunning`, the capsule-built
provenance). F2 is pure deletion, kept out of F1 so F1's rollback is one
revert.

### 3.2 The shot driver (S1): `internal/shot`

This is a Go package. It speaks CDP to the image's own Chromium through
`chromedp`, which is pure Go, so there is no Node and no Playwright.

- **Browser:** `nixpkgs.chromium` in `--headless=new`, already in the guest
  browser layer (`nix/guest-tools.nix`), so the closure is unchanged.
  - `chrome-headless-shell` is lighter but is not in our nixpkgs pin.
  - Lightpanda is rejected because it has no rendering engine (it has no
    pixels, and its screenshots are placeholders).
- **Plan:** data, not code. `ShotPlan{Views []View}`, at most
  `capsule.MaxScreens`. Each view has:
  - `name` (the existing `NN-before-<view>` naming);
  - `path`;
  - `viewport` (`desktop` 1440×900, `phone` 390×844 at DPR 3);
  - `theme`;
  - `steps`: `click`, `fill`, `press`, `wait_for` (selector), `wait_ms`
    (bounded), `scroll`.
- **Run:** one page per view, then `Page.captureScreenshot` to PNG.
  - Each view and the whole plan have timeouts.
  - A step that fails names the view and the step. The run never returns a
    partial set as success.
- **Video:** optional, one per plan, at most 20 s.
  - `Page.startScreencast` (JPEG, `everyNthFrame` 2, width ≤ 1280) writes
    frames to disk.
  - After capture, `ffmpeg-headless`, already in the layer as `CHOIR_FFMPEG`,
    encodes them to WebM (VP8), so frames are never encoded during the
    capture.
  - The existing video checks apply: an 8 MiB cap, the WebM header, and the
    secret scan in `capsule/screens_video.go`.
- **Physics:**
  - Chromium peaks at about 0.55 GiB on Choir's frontend
    (`docs/reports/playwright-in-capsules-memory-2026-10-15.md`).
  - The prebuilt frontend removes Vite, the 3 GiB first-run cost, from the
    shot path entirely.
  - The run leases 2 GiB like today's shot jobs.
  - Twelve views take about one to two minutes on one CPU.
  - Screencast at about 10 fps for 20 s is about 200 JPEG frames, about
    10 MiB of scratch.

### 3.3 The candidate shot (S2): a shooting step under `accepted`

The shot stage runs while the operation is `accepted`, before the
`accepted -> materializing` transition and outside `selfdevMaterializeMu`.
Desks keep working while it runs, and owner settle acts (3 s lock wait) are
not starved. It is a precondition of the materializer, not a phase inside
it. This follows the existing pending pattern of the host build
(`errHostBuildPending`).

**Interlock.** A materializer pass on an accepted platform operation whose
bundle carries a `ShotPlan` runs this check on every non-journal-only pass
(`!journalOnly`). A journal-only settle calls no updater, so it needs none.
The check:

1. Requires a `candidate_shot` tape event bound to this operation and this
   host build job id, with `verdict: pass`.
2. With no such event, starts the shot worker if it is not running, and
   returns pending. No transition happens, and the materializer lock is
   released.
3. With `verdict: fail`, closes the operation `failed` with "shot review:
   ...". There is no swap, and the staged layer is discarded.

A platform bundle that changes non-exempt paths and carries neither screens
nor a plan is refused at the same place. This is a permanent rule, not a
landing-order note, so no build can ever apply an unshot platform candidate.

**The shot worker.** One per operation, serialized by its own lock:

1. **Candidate binary.** It materializes the host build's closure into the
   updater's private store (`updater/closure.go` `MaterializeClosure`).
   That step is idempotent and content-addressed, and the apply needs it
   anyway. The candidate binary and frontend are then addressed by store
   path. The staged layer dir holds only `closure.nar`, receipts and the
   extracted `frontend/`.
2. **Running build.** The updater's current release supplies the binary
   and frontend paths.
3. **State copy.** One runtime-side snapshot (`SnapshotTo`, with
   `VACUUM INTO` and the sanitize scan, run once). It covers the same kinds
   an engineering capsule's copy holds: it withholds Texture documents and
   revisions, leases and mail (`lease/capsule.go`), exactly as today. So a
   shot never shows the owner's mail or document text. Owner files are hardlinked, as in `buildComputerStateLayer`.
   - The executor's state floor check (`StateFloorBytes`, the copy factor)
     is reused, so the copy cannot push the volume under its floor.
   - The before and after runs each get their own copy of that snapshot
     file, a plain file copy, so both start identical.
   - The executor's floor check is exported from `capsule` for this
     caller.
   - The sandbox is platform-owned, has no egress, and is destroyed after.
4. **Sandbox.** A new spawn shape: a net namespace with only `lo` up, no
   veth and no forwarder, and the `WorkloadInetSeccompFilter`, so AF_INET
   binds loopback and nothing else. Chromium runs with `--no-sandbox`,
   because user namespaces are refused by the capsule seccomp.
   - Both servers bind ephemeral loopback ports, never 8082.
   - The worker ports the server half of `choir-shot.mjs` (lines ~948-1138)
     to Go:
     - the autoputer env (`RUNTIME_STORE_PATH`, `AUTOPUTER_FILES_ROOT`, the
       port, `CHOIR_GUEST_TRUST_LOOPBACK=1`);
     - a static frontend server that proxies `/api` to the autoputer and
       injects the owner identity header;
     - the `/auth` stub;
     - the per-view window reset.
   - The model provider is the stub.
5. **Before, then after.** It runs the frozen plan against the running
   build, then against the candidate, writing `NN-before-<view>.png` and
   `NN-after-<view>.png`, plus the video if asked.
   - Same fonts (`FONTCONFIG_FILE`), flags and viewport, so a difference
     is the candidate's and not the harness's.
6. **Review.** The existing per-shot review judges the pairs. The worker
   calls the inner `reviewShotsNoted` with the shots it took, not
   `reviewCandidateShots`, which reads `bundle.Screens`:
   - six questions, a batch of four, fail-closed on model error, one retry
     on a malformed answer.
   - The numbered-shot check takes the owner's request text through the
     operation's assignment, the same reader Verify uses today.
7. **Record.**
   - The screens go to `Self-development/<op>/screens/`.
   - One `candidate_shot` event carries:
     - the host build job id;
     - the plan sha;
     - per-shot digests;
     - the per-shot review;
     - the verdict;
     - the state-copy receipt (`copy_ms`, `scan_ms`, bytes), so the cost
       is measured from the first run.
   - `CandidateScreens` and `frozenScreensNote` (`selfdev_screens.go`) read
     this event for plan bundles instead of saying "carries no screens".

**Watchdog.** A pass that finds no `candidate_shot` event and no live
worker for the operation restarts the worker, at most twice per operation.
A third silent death writes `candidate_shot` with `verdict: fail` and the
cause "shot worker died".

**Clean rebase.** A rebased candidate has a new host build job, so it is
shot again. That is accepted: the pair must show the build that applies.

**Crash and restart.** The operation is `accepted` while shooting:

- A crash boot closes it "interrupted by a restart" by the existing rule.
  No `candidate_shot` event was written, so nothing resumes.
- A planned restart (marker present) may re-run the worker once from
  scratch. The event is written only at the end, so a re-run is idempotent.
- `accepted` is in `selfdevBusyStates`, so a deploy refresh defers and does
  not cut a shot run.
- The base can move between the shots and the apply (a base restore or a
  package migration under `currentReleaseMu`). The apply's base-join check
  then refuses, and the operation fails closed. Only the stage's time is
  wasted.

**Verify.** `commitVerifyIntent` (`rlm_reduce.go` ~1792) skips
`reviewCandidateShots` for a bundle that carries a `ShotPlan`. That review
runs at the gate above.

Verify instead checks the plan against the request, with no model call. A
plan whose view numbers miss a numbered shot in the owner's request fails
"missing: 05, 06". This is today's FM9, moved from screens to plan views.
The verdict records `shot_review: deferred_to_candidate_shot`.

**Freeze rule.** For a platform operation, `screens_required` and
`before_after_required` are satisfied by a valid `ShotPlan` (schema, at most
`capsule.MaxScreens` views). `refuseLateBeforeShots` does not apply,
because the platform takes the before shots from the running build. An
all-exempt change (docs, tests) needs no plan, as today.

The corpusd consensus seat votes on the source and the host build alone
(`internal/platform/standing_consensus.go` `castHostBuildBallot` reads no
screens). The decision therefore precedes the shots by design, and the shot
gate is the visual check before the swap.

### 3.4 The yaegi function (S3): `choir.Shot`

- `choir.Shot(plan)` inside a capsule validates the plan and shoots it
  against the running build over the capsule's state copy. These are the
  before shots, in the same sandbox shape as §3.3.
- The engineer previews the views its plan names and fixes selectors before
  Freeze. It returns the PNG paths under `/var/lib/artifact/screens/`.
- It replaces the `choir-shot.mjs` job in the prompts for platform work.
- `choir.Shot` needs loopback, so a cast with `network: forbidden` gets a
  named refusal that says so.

### 3.5 Packages (S4, later)

A package candidate is built by the guest (`buildCheckedModule`) before it
publishes. The same shot stage runs there:

- before is the running build;
- after is the running build with the drafted package installed into the
  shot sandbox.

This retires capsule shots and the capsule Vite build for packages too.
Until S4, packages keep today's capsule shots.

### 3.6 Obscura (S5, spike only)

Obscura 0.2.2 is in our nixpkgs pin. Since v0.2.0 (August 2026) it renders
pixels and supports `Page.captureScreenshot` and screencast.

- The spike runs `internal/shot` against it over CDP on three Choir views
  and records pixel parity with Chromium, RSS, and closure size.
- It is adopted only on parity.
- The 2026-10-21 cut (untrusted page JavaScript as guest root, about 508 MiB)
  does not recur here, because the shot sandbox renders only our own
  frontend, unprivileged, with no egress.

## 4. Failure modes, decided before the code

- **FM1:** a source-only bundle reaches a path that reads `RuntimeFiles` as
  a release input. The review found every reader nil-safe or display-only.
  - The materializer's named failure covers a non-consensus source-only
    bundle with `hostBuildRequired == false`.
  - Each reader gets a test: the journal-only settle, clean rebase, the
    witness, `inspect_bundle`, and `rebase_content`.
- **FM2:** a capsule that staged a release tree and has a patch: the tree
  is checked for secrets as today, never installed, and never diffed on
  re-Freeze.
- **FM3:** the shot sandbox reaches the network. Only `lo` is in its
  namespace, and a test proves a non-loopback dial fails.
- **FM4:** the shot copy writes back to the live state. The copy is a
  private file, and a test compares the live `state.db` digest before and
  after.
- **FM5:** the before and after plans differ. One frozen plan sha drives
  both, and the event records it.
- **FM6:** the shot worker hangs. Each view, the plan, the worker and the
  review have timeouts, and the worker holds no materializer lock. A
  timeout writes `candidate_shot` with `verdict: fail` and the cause.
- **FM7:** a crash while shooting. The operation is `accepted` with no
  event, so the crash boot closes it "interrupted by a restart".
- **FM8:** the review model is unavailable. The verdict is a fail with that
  cause, never a pass by default.
- **FM9:** the candidate binary panics on start, or does not serve. The
  verdict is a fail with its stderr tail.
- **FM10:** the `Complete` guard fires on a legitimate no-change round. It
  fires only when the capsule diff touches `/workspace/platform`.
- **FM11:** an unshot platform apply. The interlock in §3.3 refuses any
  platform bundle with non-exempt changes that has no screens and no
  passing `candidate_shot`.
- **FM12:** Verify wrongly fails a plan bundle on numbered shots. Verify
  checks plan views, not screens.
- **FM13:** fonts or flags differ between before and after. One launcher
  with fixed flags and `FONTCONFIG_FILE` serves both runs.
- **FM14:** the shot copy shows withheld or empty tables. The copy
  withholds exactly what an engineering capsule's copy withholds (mail,
  Texture, leases), so a view of those surfaces renders empty in both the
  before and after shots. The pair still compares like with like.

## 5. Slices, proof and rollback

- **S1 (orange, lands first):** `internal/shot`.
  - The chromedp driver, the plan schema and validation, PNG and WebM
    output.
  - The new dependencies are `chromedp`, `cdproto` and `gobwas/ws`, pinned,
    with the nix `vendorHash` updated and nix eval of both systems.
  - An end-to-end test shoots a served frontend and writes PNGs and a WebM
    as artifacts.
  - Nothing calls it yet, so it is safe alone.
- **L1 (red, tier 2): F1 and S2 as one landing.** Builders may work in
  parallel, but one push carries both.
  - Source-only freeze, the bundle contract and the strict decoders.
  - The `Complete` guard.
  - The prompts.
  - The shot worker and its sandbox.
  - The `candidate_shot` event.
  - The materializer interlock.
  - The Verify change.
- **S3 (orange):** `choir.Shot`.
- **S4 and S5:** later.

**Proof on d@e.** apply-proof-4 is a one-line platform change with a
one-view plan.
- It freezes as source only.
- corpusd decides it.
- The shot worker writes `candidate_shot` with a passing review and
  before/after screens under `Self-development/<op>/screens/`.
- It applies with no owner act.
- Then revert proof steps 4-6 of the apply-and-revert design.

A negative proof checks that a plan whose candidate binary fails to serve
closes `failed` with no swap.

**Rollback.** Revert L1 to restore today's refusal, under which platform
rounds do not freeze. S1 is inert alone.


## 6. Review adjudication (r0)

r0 was reviewed by Devin and a Sonnet seat; both returned SEND BACK.

**Blocking findings, all adopted.**

- **F1 alone opens an unshot apply** (both seats). Adopted:
  - L1 lands F1 and S2 together.
  - The materializer interlock (§3.3, FM11) makes the rule permanent.
- **Verify needs a named change** (both seats). Adopted: the skip for plan
  bundles, and the numbered-shot check on plan views (FM12).
- **The stage's placement against the transition and the lock** (Sonnet).
  Adopted: a shooting step under `accepted`, outside the lock, with a
  persisted event.
- **FM7's contradiction** (Sonnet). Resolved by the placement: the crash
  boot closes an accepted operation.
- **The `RuntimeArtifactRef` prefix** (Sonnet). Adopted: keep
  `runtime-artifact:`.
- **The `frozenReleaseChanges` re-Freeze refusal** (Sonnet). Adopted: skip
  the diff for source-only bundles.

**Non-blocking findings, adopted.**

- The `emitSourcePatch` reorder.
- The strict `inspect_bundle` mirror and `CleanRebaseBundleFields`.
- The binary comes from the closure: materialize it, then address it by
  store path.
- The state-copy producer: runtime-side, every kind, with the floor check.
- The lo-only spawn shape.
- Ephemeral ports.
- Ports of the `choir-shot.mjs` server half.
- Fonts and flags.
- Pinned dependencies.
- The `Complete` guard after a refused Freeze.
- `CandidateScreens` reads the event.
- Measured copy receipts.
- The base moving between the shots and the apply fails closed.

**Noted.**

- The consensus seat votes without screens. That is by design under the
  owner ruling.
- A non-consensus, frontend-only, source-only bundle fails by name (FM1).

## 7. Review adjudication (r1)

Devin r1: ACCEPT. All six claims were confirmed against code:
- refresh defers while `accepted`;
- a crash closes the operation and a marker allows one re-run;
- the pending pattern runs outside the lock;
- the closure can be materialized before the swap;
- `SnapshotTo` works, with the floor check exported;
- the hole is closed on every path, including clean rebase and the journal
  settle.

Non-blocking items, all adopted in r2:
- **N1:** the worker calls `reviewShotsNoted`.
- **N2:** the copy keeps the capsule's withheld set instead of widening to
  every kind, so screens never show mail or Texture text.
- **N3:** the interlock is scoped `!journalOnly`.
- **N4:** a pass-side watchdog restarts the worker at most twice.
- **N5:** a clean rebase re-pays the shot stage; this is noted as intended.
