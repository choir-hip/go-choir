# Reuse a shot verdict when the judged content is unchanged (2026-11-09)

Status: design; the tonight cut (section 12) is built. Class of this document: green. Class of the build: red
(slices S1 to S4 loosen or add to an apply gate; S0 and S5 are orange).
Problem record: [shot-review-repeats-on-every-rebase-2026-11-09](../problems/shot-review-repeats-on-every-rebase-2026-11-09.md).
Owner, 2026-11-09: "I don't think we should need to have to do the same check on
every release. ... Once we check it once, shouldn't it be good to go?"
Inherits: [desk-shots-and-review-2026-11-07](desk-shots-and-review-2026-11-07.md) (the gate and the verdict),
[source-only-freeze-and-candidate-shots-2026-11-06](source-only-freeze-and-candidate-shots-2026-11-06.md) (FM2, FM11),
`internal/selfdevprotocol/clean_rebase.go` (the receipt). No runtime switch anywhere below.

## 1. The foundational question

What does a shot verdict attest, and which inputs decide the pixels?

The verdict attests one thing: the served frontend, run over a copy of the computer's
state, renders the plan's views in a way a person would accept (opaque surfaces,
readable text, styled controls, the requested change visible). It does not attest Go
code, API semantics or data. Those are attested by CI, the host build, and the
updater's health probe (`internal/updater/runtime.go:124`, the guest's `/health`),
which rolls an apply back.

The pixels are a function of: (a) the frontend tree the release serves, byte for byte;
(b) the shot plan (paths, viewports, themes, steps); (c) the state copy; (d) what the
backend answers to the frontend's calls; (e) the shot driver and browser (Chromium,
fonts, `internal/shot`); (f) time-varying content (clocks, animations, loading).

A clean rebase changes none of the verdict's own subject. It re-applies the approved
patch, byte for byte (`CompareCleanRebaseBundles`; the shot plan is not in
`CleanRebaseBundleFields`, so it is the same plan). What it can change is (a), through
the base underneath the layer, and (d) and (e), through platform code. The question
a rebase review should ask is therefore not "does the change look right" (already
judged) but "did the base move the frontend the layer sits on". That is a question about
bytes, answerable without a model.

## 2. What the code does today (confirmed)

- A clean rebase is triggered at boot by `stack_base_moved` or `stack_rematerialized`,
  after a five minute timer (`clean_rebase.go:137`, `:269`).
- The gate takes the relevant paths from the bundle's own file effects
  (`platformShotRelevantPaths`, `selfdev_candidate_shot.go:56`). For a rebase these are
  the layer's patch, so the set is the same non-empty set every time. The exemptions
  (docs, tests, build output) never see what the base changed.
- The verdict is keyed `shot-verdict:<op>:<host build job>` and checked against
  operation, host build job, bundle digest and plan digest
  (`selfdev_candidate_shot.go:112-129`). The new operation and new job on every rebase
  make an old pass unreadable. This closes FM2 (a stale or foreign verdict unlocking a
  swap) by making every verdict single-use.
- A round costs two sandbox passes and about ten to fourteen model calls (tonight, the
  failed round: 14 calls, 176k input tokens, 6m23s, 3m51s with an accepted operation
  blocking engineering admission).
- A failed review leaves the computer on its old layered release and old base, behind
  the fleet, until the rebase notice is answered by a model round. Nothing else
  moves it.
- `updater.FrontendDigestOfManifest` already digests the `frontend/` files of a
  manifest from the hashes the manifest carries (`served_content.go:43`), and the host
  build's file list is in hand at the gate (`hostBuiltRelease.Files`,
  `self_development_host_build.go:106`). The running release's manifest is the updater's
  `current`.

## 3. The option space, cheapest to dearest

| # | Option | Cost per rebase | Verdict |
|---|---|---|---|
| A | No shot when the platform diff `stackBase..booted` touches no shot-relevant path | zero | Rejected. The diff-path rule reasons about names, not output. Global CSS, a shared component, a changed font, a dependency bump all move pixels with no layer path touched. Tonight's `source projection` line says "the booted base changed no path the patch touches", which is true of the launcher change that did move the UI. |
| B | Reuse a prior pass when the digest of what the shots depend on is equal | one hash comparison, no shot, no model | Recommended core. Digest is of the built served frontend, not source paths. |
| C | Take the shots, compare them to the accepted ones (exact hash, later perceptual), call the model only if they differ | two sandbox passes, no model on a match | Recommended fallback, fails safe. |
| D | Today's rule: shoot and judge every rebase | ten to fourteen calls, minutes, a failure chance | Kept only for the case where B and C do not match. |
| E | Never shoot a clean rebase; trust health check and rollback | zero | Rejected. The health probe reads `/health`, so a build that serves a broken page passes it. |

B then C then D is a ladder: each rung is entered only if the one above did not close the
gate. Nothing is skipped by default; a reuse is a positive, checkable claim.

## 4. Recommendation and where it departs from the first proposal

Adopt the ladder B, C, D with a non-model liveness check (section 6) on every rung.
Three departures from the proposed direction.

1. Bind to the whole served frontend tree, not "the built tree and the layer's own
   shot-relevant files". The layer's files are already pinned: the patch SHA-256 is
   byte-identical by the receipt's rules, and the plan digest is checked today. Hashing
   them again adds nothing. What changes under a rebase is the rest of the tree, so the
   digest must cover every file the release serves (`FrontendDigestOfManifest` over the
   layered manifest), the plan, the patch and the driver identity.
2. Equality of a frontend digest is not equality of pixels. Inputs (c) to (f) are
   outside it. The reuse claim is therefore narrower than "this looks the same": it is
   "the frontend the judge approved is the frontend now served, with the same plan". The
   remainder (a backend-only platform change that breaks what the page loads) needs a
   mechanism of its own: the liveness check in section 6. The deploy health check alone
   does not do this (item 3 of the proposal is incomplete).
3. The Loading finding comes first. A reuse ladder that falls through to a shot which
   returns two Loading screens fails the same way the round did. A shot that cannot
   reach a ready page is a failed job, not evidence. So S0 (readiness) ships before the
   gate loosens, and the judge must not be asked q2 of a rebase's BEFORE image, which
   already contains the layer (problem record, last section).

Why a digest is right here: the compared object is an artifact the host build already
produced and hashed, the comparison is exact, and an unequal digest only costs what is
paid today. Non-determinism in the build (a timestamp inside a bundled file) makes the
digests differ and so falls to rung C or D, never to an unsound pass.

## 5. The new binding

### What is hashed

Every pass (and every reuse) records an attestation on the tape:

```text
attested = {
  frontend_digest:   FrontendDigestOfManifest(layered manifest of the judged release)
  plan_sha256:       bundle.ShotPlan.Digest()
  source_patch_sha256
  driver_id:         the shot driver and browser build the shots ran under
  shot_job_id, host_build_job_id, operation_id, bundle_digest   (as today)
}
```

`driver_id` is a digest of the sandbox browser build and viewport table the plan runs
under; a platform deploy that changes the driver changes it, and that forces rung C or
D. It is the one platform input to the pixels the frontend digest cannot see.

### Where it is recorded

- New optional field `attested` on `computerevent.ShotVerdictPayload`
  (`shot_verdict.go:74`), filled by the existing `ShotVerdict` reduction on every pass
  from the host-built manifest and the job. Old events have none and cannot be reused
  (fail closed; the first rebase after the build shoots once, then records).
- A reuse is its own event kind, `shot_reuse`, keyed in the same idempotency space as the
  verdict (`shot-verdict:<op>:<host job>`) so the gate keeps one lookup, and it carries
  `attested` of the new release, the prior event's id, and the reason `reused`. Reader
  accepts either kind on the key. A separate kind keeps the single-writer purity of
  `shot_verdict` (only the `ShotVerdict` reduction writes it) and the gate-side writer
  visible to `check-v1-writer-purity`. Default; see open question 2.
- `CleanRebaseReceipt` gains `PriorShotEvent`, the event id of the prior operation's
  accepted verdict or reuse. `runCleanRebase` fills it from the prior operation's key,
  and corpusd's receipt check (`internal/platform/clean_rebase.go`) verifies the event
  exists on the computer's tape and is a pass with an `attested` block. The gate then
  fetches that event by id; it never scans commitment records.

### The gate, on a clean-rebase operation only

The operation is a clean rebase when its decision's authority is a verified
`platform-clean-rebase:` receipt, not when its id has the `clean-rebase-` prefix.

1. Look up the key. A verdict or reuse on it is read as today (all bindings).
2. Otherwise recompute `attested'` from this operation's host build (not from anything
   the prior event says about the new build).
3. Rung B: fetch `PriorShotEvent`. It must be a pass on this computer, same plan
   digest, same patch SHA, same `driver_id`, and `frontend_digest` equal to
   `attested'.frontend_digest`. Then run the liveness check (section 6). On success
   commit the `shot_reuse` event and return nil to proceed to the swap. No shots owed
   record, no management wake, no model call.
4. Otherwise rung C, then D: the existing shots-owed flow. S4 adds an exact hash
   comparison of the new AFTER shots against the prior accepted AFTER shots; a match
   commits the reuse; a mismatch goes to management with the prior verdict's findings
   in the record.

An ordinary (non-rebase) platform change never takes the reuse path.

### How FM2 stays closed

- The reuse is bound to this operation and this host build job through the key, and to
  this bundle and plan through the payload checks, exactly like a verdict.
- The claim is recomputed from the new build's manifest by the gate. A prior event is
  evidence that something equal was judged; it never supplies the new build's digest.
- The prior event must be on this computer's tape, a pass, and named by the receipt that
  corpusd already validates, so a foreign computer's or a prior failed verdict cannot be
  named.
- The chain is one hop: a reuse event can itself be a `PriorShotEvent`, but only through
  its own `attested`, which is the new build's recomputed digest. A chain cannot drift,
  because every link is an exact digest equality with the judged tree.
- A frontend digest of `""` (no `index.html`, unreadable) never matches.

## 6. The backend-only break

A platform change that is Go only cannot alter frontend bytes, yet can break what the
page loads (an API shape the frontend reads, a handler that errors for this state).
The proposal leaves this to the deploy health check. That check reads `/health`.

Add a liveness check that runs in code, with no model: in the gate, boot the candidate
in the existing sandbox over the state copy (rung B pays one pass, not two) and run a
fixed platform-owned view whose steps are `navigate` to the shell, `wait_for` the shell's
ready selector, and assert zero console errors and zero failed first-party requests. Any
failure closes the reuse and falls to rung D, where management sees what failed. The
ready selector is the same readiness S0 gives every plan, so it also catches the
Loading screens that the problem record found.

Cost: about one minute of sandbox, no model call, no wake. If this proves too costly on
a busy node, the fallback is the existing post-apply health probe plus rollback and a
post-apply `GET` of the shell as part of the probe (`updater/runtime.go`); I prefer the
pre-swap check because a failure there leaves the computer where it is, not rolled back.

## 7. Ways it could fail, and the test for each

1. Digest equal, page broken because of the backend. Test: a Go-only change that makes a
   handler the shell calls return 500; the liveness check fails and the gate falls to
   rung D. E2E on d@e.
2. Build non-determinism makes digests differ every time (no savings). Test: host-build
   the same tree twice and compare `FrontendDigestOfManifest`; a CI gate on the host
   build fixture. A mismatch is not unsafe, only wasteful, and the test names the file.
3. A foreign, stale or failed prior event is named by a forged receipt. Test: receipt
   with an event from another computer, a fail verdict, an event without `attested`, and
   a reuse whose own digest differs; corpusd refuses each; the gate refuses each.
4. The prior verdict was a pass for a different plan or patch. Test: plan edited between
   the two; digest differs; shoot.
5. Driver or browser changed in the deploy. Test: change `driver_id`; reuse is refused.
6. The frontend digest covers only `frontend/`, not fonts or assets outside it. Test:
   inventory what the sandbox serves; if any served path is outside the digest, widen the
   digest; a unit test pins the list.
7. A bad pass reused forever (a judge error is copied to every rebase). Test: a reuse
   names its origin; `choir` shows the chain; an owner act on the origin invalidates
   descendants (open question 4). Drill on d@e with a seeded wrong pass.
8. The first rebase after the build has no attested prior. Test: it shoots once and then
   records; the next rebase reuses. E2E on d@e across two deploys.
9. The liveness view itself reads Loading (the sandbox fault). Test: S0's readiness
   failure is a failed job with a cause, not a pass; fixture with a page that never
   reaches the ready selector.
10. A reuse event written twice or after a restart. Test: the key is the idempotency key;
    a replay is a replay; a boot closes a pending liveness job "ended by a restart" like
    a shot job (no resumer).
11. A direction regression: a rebase where the frontend did change must still be judged.
    Test: a base commit that edits global CSS; digest differs; rung D runs; shots-owed
    record appears. This is the launcher-change case.
12. The judge asked the wrong question on a rebase. Test: the shots-owed text for a
    rebase states that BEFORE contains the layer, drops q2's "requested change" reading,
    and treats a not-ready page as a failed job.

## 8. Build slices

Order: S0, S1, S2, S3, then S4 and S5. Under the data-cutover operating tempo, runtime
checks are the verifier.

- S0, orange. Readiness: every shot view waits for a platform ready selector before the
  capture (a default `wait_for` on the shell's ready marker injected by the driver, not
  authored by the plan); a page that does not reach it fails the job with a cause.
  Rollback: revert. d@e proof: run a shot job on d@e; the job either captures a ready
  page or fails by name; the previous Loading result no longer reaches a judge.
- S1, red (additive). `attested` on `ShotVerdictPayload`, computed from the manifest,
  `driver_id`, plan and patch; reader tolerates absence. Rollback: revert; the field is
  optional. d@e proof: one ordinary platform candidate on d@e; the verdict on the tape
  carries the digest and it equals a recompute from the staged release.
- S2, red. `CleanRebaseReceipt.PriorShotEvent` and corpusd's check (tier 2 by the
  contract: tape, receipt). Rollback: revert; old receipts omit the field and take D.
  d@e proof: forge each bad receipt of failure 3 on d@e and see the refusal; no drill on
  the paper or on an owner's computer.
- S3, red (loosens an apply gate). The gate's rebase branch, the `shot_reuse` kind, the
  reader for either kind, FM2 tests. Rollback: revert (the gate returns to D for every
  rebase); a one-act revert shows on the next rebase. d@e proof: a Go-only deploy to d@e
  produces a clean rebase with zero shots-owed records and zero image calls in the
  gateway journal for its VM; a deploy with a global CSS change produces a full round.
- S4, orange. Liveness check (section 6) and the exact AFTER-hash comparison of rung C.
  d@e proof: the broken-handler build of failure 1 is refused before the swap.
- S5, orange (text only). Rebase framing in the shots-owed procedure (failure 12).
  d@e proof: a forced rung D round on d@e with the new text; the judge's answers are
  recorded.

Landing: each slice carries `Ring-Expedite` while the owner's expedite ruling stands. S3
must not land before S0, S1 and S2 are deployed to d@e and seen working.

## 9. What it would have saved tonight (confirmed inputs, one assumption)

The failed round: the layer sat on d6d91f1a6 and was rebased onto 03300ad5f. The 17
changed files are Go, docs and one nix script; none is under `frontend/`. The last
frontend commit on main is 1c2dc2a7d (23:27Z), already in d6d91f1a6's base. Under the
rule, assuming the host build gives a stable frontend tree (failure 2), the round would
have cost one hash comparison and one liveness pass: no 14 calls, no 176k input tokens,
no 6m23s of blocked admission, and above all no failed verdict that left d@e on the
old base. The later deploys of the night (e00541108 Go and `cmd/choir`, 5021acb74, the
modelpolicy change) are also Go only. The first round after 1c2dc2a7d is the only one
that legitimately differs, so I expect four of the five tonight to have been free; that
count is unverified because the earlier rounds' logs were reset by restarts.

## 10. Scale

Any computer with an applied platform self-development operation carries a layered
release (`layered: true`, a closure digest and layering entrypoint in the manifest;
`layeredSelfDevelopmentManifest`, `self_development_host_build.go:355`). The clean
rebase runs on every such computer, whatever its tier (`clean_rebase.go` header). I did
not enumerate real computers. The cost is roughly (layered computers) x (platform
deploys) x ten to fourteen image-capable calls today; with the ladder it is (layered
computers) x (deploys that change frontend bytes or the driver).

## 11. Open questions, with defaults

1. Perceptual threshold for rung C. Default: exact hash only. Live state and clocks make
   byte equality rare, so it will mostly miss; that fails safe. Revisit with data after S4.
2. Is a gate-written `shot_reuse` event acceptable to the writer-purity gate, or must a
   desk reduction write it? Default: a separate kind written by the materializer's
   appender; if the gate refuses, route it through a platform-signed record the
   `ShotVerdict` reduction accepts from the gate only.
3. Should a failed rebase review keep the computer on the old base silently? Default:
   unchanged here (the notice follows); a separate problem, since it strands the computer.
4. Should an owner act on the origin verdict (a seeded wrong pass) invalidate reuse?
   Default: yes, a reuse names its origin and a later fail on the origin makes the next
   rebase take rung D.
5. Should an ordinary (non-rebase) platform change ever reuse? Default: no. A new patch is
   new judged content.
6. `driver_id` definition. Default: the digest of the sandbox browser binary identity and
   the viewport table; confirm what `internal/shotsandbox` already records.

## 12. Tonight-cut: review record and failure list

Status: build spec. Class: red (loosens an apply gate), additive, one-act rollback by
revert. Devin reviewed sections 1 to 11 (VERDICT: BUILD WITH CHANGES); its "Findings by
class" and "Recommended tonight-cut" override sections 4 to 8 where they differ. This
section is what the build follows. No runtime switch.

### 12.1 Review record

| # | Devin finding | Adjudication |
|---|---|---|
| 1 | The digest of build output never matches across a rebase: the host build bakes the synthetic rev and base date into the bundle (`builder/util.go:124`, `flake.nix:279`, `vite.config.js:23`). | Adopted. The comparison is of SOURCE ids at the commits: git tree ids of `frontend/` and `nix/`, and blob ids of `flake.nix` and `flake.lock`, read from the guest's release repository (`rt.releaseRepo()`, the repository `sourceproj.Reapply` already needs both commits in). Build output is never hashed. The flake's frontend derivation takes `./frontend` as its only source, so that tree id is the whole input. |
| 2 | `"" == ""` matches. | Adopted. Every id and digest must be non-empty and well formed; an empty or unreadable value is an explicit refusal before any comparison. |
| 3 | Reuse without a page-level check is weaker than today. | Adopted. The liveness pass is in the same commit as the gate branch; a reuse cannot be committed without it. |
| 4 | No console/request instrumentation exists. | Adopted, minimal. A counter in `NewFrontendHandler`'s proxy, reported in the stage `Result`; the existing marker `.app-root[data-auth-state="signed_in"]`. Console errors deferred. |
| 5 | `driver_id` omits fontconfig and ffmpeg. | Adopted with a limit. The browser, fontconfig and ffmpeg paths (nix store paths as the guest sees them) are recorded on every `shot_reuse` and compared with the prior reuse's. A verdict written before this build carries no environment on the tape, so the first reuse in a chain compares the driver's SOURCE instead (`nix/`, `internal/shot/`, `internal/shotsandbox/` tree ids): a deploy that changes the driver or the browser pin changes a tree id and takes the judged round. |
| 6 | Bind the prior event to the rebased operation. | Adopted. `prior.OperationID == receipt.RebaseOf`, `prior.BundleDigest == receipt.PriorBundleDigest`, same plan digest, a pass. |
| 7 | Branch before `managementCanSeeImages`. | Adopted. |
| 8 | Register the kind and the reducer binding; the writer-purity worry is a misread. | Adopted (`validEventKinds`, the reducer's `DecisionRef`/`ProposedEffectRef` binding). `TestSemanticKindsNeverEnterTheQueue` counts `types.EventKinds()` (telemetry kinds), not `computerevent` kinds, so its count does not change. |
| 9 | A frontend-only layer has no host build and fails the gate today. | Noted, unchanged. Reuse needs a host build (`built != nil`); with none the branch is not entered and today's refusal stands. |
| - | Defer `attested`, `PriorShotEvent` and the corpusd check, the AFTER-hash comparison, injected readiness (S0). | Deferred as Devin recommends; the gate reads the prior event off the same tape it already trusts. |
| - | Make a failed judged round advisory after a passed liveness. | Not built. An owner decision. |
| - | S5, the rebase framing text. | Built (a few lines): for a clean-rebase operation the shots-owed text says BEFORE already contains the change. |

### 12.2 The rule

The branch runs only for an operation whose finalized decision's authority is a
`platform-clean-rebase:<receipt digest>`
(`selfdevprotocol.CleanRebaseReceiptDigestFromAuthority` on the transition
`FinalizedDecisionForOperation` returns), never by operation id prefix. It runs after the
verdict-key miss and before `managementCanSeeImages`. All of these must hold, each
non-empty, or the gate does what it does today:

1. The receipt (read from the tape by its digest, digest re-verified) names this
   operation, bundle and host build job, and its patch SHA equals the bundle's. Its
   booted base equals the build's base commit and the runtime's booted base.
2. The prior event is the latest on `shot-verdict:<RebaseOf>:`; it is a pass bound to
   `RebaseOf`, `PriorBundleDigest` and this plan digest. Either kind may be the prior.
3. The origin is a judged pass: the prior is a `shot_verdict`, or a `shot_reuse` whose
   recorded origin key still reads back as a `shot_verdict` pass with the recorded digest.
4. Source ids are equal across the judged base, the receipt's stack base and the booted
   base (the fifteen `ShotInputPaths`, 12.7;
   `flake.nix`, `flake.lock` blobs).
5. The driver environment is non-empty (browser, fontconfig; ffmpeg when the plan has
   video) and equal to the prior reuse's recorded one when there is one.
6. The liveness pass is clean (12.4).

Then the gate commits `shot_reuse` under `shot-verdict:<op>:<job>` and proceeds to the swap.

### 12.3 Chaining decision

A reuse may chain (a rebase of a rebase is the normal case: after one reused rebase the
layer's operation is itself a clean rebase whose prior event is a `shot_reuse`). The chain
is bounded three ways. It names its origin: every `shot_reuse` records the origin event
digest and key, and the next hop re-reads the origin from the tape and requires a
`shot_verdict` pass with that digest. It is pinned to the judged content: the judged
source ids are copied unchanged from hop to hop, and every hop requires the booted base's
ids to equal the judged ones, not merely the previous hop's. And it is at most 32 hops
long (the platform's own `cleanRebaseMaxChain`). A frontend change anywhere in the chain
ends it: the next rebase takes the judged round, and its pass is a new origin.

### 12.4 The liveness pass

One candidate pass in the shot sandbox over a private state copy, no before pass, no
model: navigate `/`, `wait_for` `.app-root[data-auth-state="signed_in"]`, a 2 s settle.
The stage's frontend proxy counts the page's `/api` responses (requests carrying the
stage header `X-Choir-Stage-Reset`, the desktop reset, are not the page's and are not
counted). Pass requires: no stage error, at least one counted request (a page that never
reaches the backend proves nothing), and zero counted responses with status 400 or above.
A proxy failure answers 502 and counts. A 101 (WebSocket upgrade) and 3xx are not
failures: the rule is "no status of 400 or more", the honest reading of "non-2xx" for a
proxy that carries upgrades. The sandbox auth stub makes the owner signed in, so a 401 or
403 on a healthy boot is a defect (the owner header was lost). Whether a healthy signed-in
boot is entirely 2xx is NOT established by anything in the repo (no recorded status
histogram exists); the Result therefore reports the status histogram and the first
offenders, the gate logs them, and the d@e proof reads them before the rule is trusted.
A false failure costs only today's judged round. Any failure falls to the judged round,
once per (operation, job) per process.

The pass runs in a goroutine the gate starts (one per operation and job, single-flight
under a mutex); the gate returns pending while it runs, as it does for the host build.

### 12.5 Failure list, each with its test

| # | Way it fails | Defence | Test |
|---|---|---|---|
| F1 | Stale or foreign prior verdict (another op, bundle or plan; a fail). | Bound to `RebaseOf`, `PriorBundleDigest`, plan digest and verdict pass; the latest event on the prefix. | `TestShotReuseDecisionRefusals` rows prior_other_op, prior_other_bundle, prior_other_plan, prior_fail |
| F2 | A replayed or foreign `shot_reuse` unlocks another job. | The key names op and job; the reader re-checks op, job, bundle, plan, reason, prior and liveness fields; the reducer binds `DecisionRef` to the live pending decision. | `TestReadShotReuseEventBindings`, `TestReducerCandidateShotBindsThePendingDecisionAndMovesNoHead` (extended to shot_reuse), `TestShotReuseEventUnlocksTheSwapAndIsIdempotent` (the real chain), `TestShotReuseForeignJobDoesNotUnlockTheSwap` |
| F3 | Empty digests or ids match each other. | Every compared value is checked non-empty and well formed first. | `TestShotReuseDecisionRefusals` rows empty_*; `TestInputIDsMissingPathIsAnError` |
| F4 | The wrong base commits are compared (the receipt's pair vs. the booted base vs. the build's). | The receipt's booted base must equal the build's and the runtime's; ids are read at the receipt's stack base and booted base and must equal the judged ids the origin recorded. | `TestShotReuseDecisionRefusals` rows booted_mismatch, judged_mismatch |
| F5 | Crash between the liveness pass and the event. | Nothing is persisted before the event; the next gate pass re-runs the pass. The event is idempotent on its key. | `TestShotReuseEventUnlocksTheSwapAndIsIdempotent`, `TestShotReuseGateAnswersFromTheRunsState` |
| F6 | Liveness passes on a login page, a blank page or an error page. | The marker requires `signed_in`; a page that never mounts the app times out as a stage error; at least one counted `/api` request is required. | `TestLivenessVerdict` |
| F7 | The counter counts an expected non-2xx (a 101 upgrade) or the driver's own reset. | 101 and 3xx do not count; requests with `X-Choir-Stage-Reset` (the stage's own reset) are excluded; 4xx, 5xx and proxy 502 count. | `TestAPICounter` |
| F8 | Concurrent materializer passes start two liveness runs or commit two events. | A single-flight per (op, job); the event key is the idempotency key. | `TestShotReuseGateAnswersFromTheRunsState` (a running run answers pending), `TestShotReuseSingleFlight` |
| F9 | A rebase of a rebase reuses a reuse whose origin is gone or changed, or chains forever. | 12.3: origin re-read, judged ids pinned, depth bound. | `TestShotReuseDecisionRefusals` rows origin_*, chain_too_deep; `TestShotReuseChains` |
| F10 | A restart mid-liveness resumes it. | Nothing is persisted and a boot starts nothing; the accepted operation reaches the gate through the materializer as it does today and runs one fresh pass, which is a new check, not a resumed one. | `TestShotReuseSingleFlight` (a new runtime has no run) |
| F11 | A frontend-only layer (no host build). | `built == nil` leaves the branch unentered; today's refusal stands. | the branch is skipped; existing `TestCandidateShot...` |
| F12 | Non-rebase operations or computers with no prior pass change behaviour. | The branch returns "not applicable" for a non-clean-rebase authority and for no prior event; the rest of the gate is untouched. | `TestShotReuseNotApplicable` |
| F13 | The frontend source changed. | Tree id inequality refuses, and the judged round runs. | `TestShotReuseDecisionRefusals` row frontend_changed; `TestInputIDsRealGit` |
| F14 | The driver environment differs or is unset. | Browser and fontconfig must be non-empty; equal to the prior reuse's record when there is one; the driver source trees are compared. | `TestShotReuseDecisionRefusals` rows env_empty, env_changed, driver_source_changed |

### 12.6 As built

- Where the two bases and the ids come from. The receipt (read from the tape by its
  digest, re-verified) names `stack_base` and `booted_base`; the guest's release
  repository (`rt.releaseRepo()`) already holds both commits, because the clean rebase
  that produced the operation ran `sourceproj.Reapply` on that repository in the same
  boot. `sourceproj.ObjectIDs` runs `git rev-parse <commit>:<path>` there. If either
  commit or path is missing the read is an error and the gate keeps its judged path.
- The shot job's staging is repeated for one side in `livenessPass` (about 80 lines);
  it was not factored out of `shootCandidate` to keep this change additive.
- The release witness and `choir` screens listings read `shot_verdict` only, so a
  reused operation shows no verdict there (residual, not blocking the swap).
- A frontend-only layer, or a computer with no image-capable judge, behaves as before
  (the first has no host build; the second now gets the reuse when it qualifies).
- Test for F11 is the unchanged `built == nil` early return; there is no separate row.

### 12.7 Review round 2 (Sonnet: accept with a path-set finding; Devin: repair)

Devin's blocking finding falsified the "unchanged source" claim of 12.1 row 5 and 12.2
item 4: inputs the shot pass runs lie outside the six compared paths. The compared set
is now `ShotInputPaths` (`internal/computerevent/shot_reuse.go`), fifteen entries:
the original six plus `go.mod` and `go.sum` (the chromedp and cdproto pins linked into
the running binary), `internal/capsule` (sandbox ids, `BuildShotFilesLayer`, state-copy
floors, `ScreenRelevantPaths`, plan bounds), `internal/lease` and `internal/objectgraph`
(`CapsuleStoreKinds` and the kind registry: which state enters the copy),
`internal/pkgruntime` (the package bundler and `css-fetch-functions.json`),
`internal/appdesk/firstparty.json` and `required.json`, and `styles/`. Every path was
checked to exist (`TestShotInputPathsExist`). Backend handlers are not compared; liveness
covers them.

| # | Finding | Adjudication | Test |
|---|---|---|---|
| N1 | Path set too narrow (Devin 1, Sonnet). | Adopted: the fifteen paths; the record carries every id; an event recorded with the six-id set lacks the rest, is incomplete, fails `Validate` and so cannot be a hop. | `TestShotReuseEveryInputPathIsCompared`, `TestShotInputPathsExist` |
| N2 | `appendShotReuse` returned success for any event on the key (Devin 2). | Adopted: an occupied key is read through the verdict or reuse reader. | `TestAppendShotReuseDoesNotTrustAnOccupiedKey` |
| N3 | Runs map never shrinks (Devin 8); a panic in the goroutine leaves a run "running". | Adopted: a committed run is deleted (the event is then the record); a panic becomes a failed run. | `TestShotReuseGateAnswersFromTheRunsState`, `TestShotLivenessPanicBecomesAFailedRun` |
| N4 | A transient git or tape read error was cached as a refusal. | Adopted: only definitive refusals are cached (receipt not the one its digest names, no prior, decision refusal, liveness failure); unread facts take the judged path for that pass and are read again on the next. | `TestShotReuseUnreadFactsAreNotCached` (tape read; the git read shares the path) |
| N5 | A benign 4xx would disable reuse (Devin 3). | Deferred, by design: fail-safe direction; the d@e proof reads the logged histogram. | none |
| N6 | `shot_verdict`-only readers omit a reused operation (Devin 4). | Deferred: documented residual; the swap does not depend on it. | none |
| N7 | Hop-1 driver environment not comparable (Devin 5). | Deferred: the driver source trees now include `internal/capsule`, `go.mod` and `go.sum` too; exact paths are compared from hop 2. | none |
| N8 | Prose says `X-Choir-Driver` requests are excluded (Devin 6). | Fixed in the prose: only the stage's own reset, marked `X-Choir-Stage-Reset`, is excluded; the page's own calls, whatever header they carry, are counted (12.4 and F7 now say so). | `TestAPICounter` |
| N9 | `receipt.PriorDecisionEvent` is unchecked at the gate (Devin 7). | Not a defect: the prior pass is bound to `RebaseOf`, the prior bundle digest and the plan, and corpusd verifies the prior decision at admission (`internal/platform/clean_rebase.go`). | none |
