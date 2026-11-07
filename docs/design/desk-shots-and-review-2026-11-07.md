# Desk shots and review (2026-11-07)

Status: r2 (r0 seats and the Devin r1 re-review adjudicated). Mutation class: red (self-development apply path, desk verbs,
tool loop failover). Mission: modular architecture
(`docs/definitions/choir-modular-architecture-2026-11-04.md`). Supersedes the
candidate-shot worker of `source-only-freeze-and-candidate-shots-2026-11-06.md`
§3.3 and its review; keeps §3.1 (source-only freeze) and §3.2 (the shot
driver).

## 1. Problem

The owner, 2026-11-07: "If we've left the desk pattern behind, we need to
use the desk pattern. There shouldn't be some platform level screenshot
service. That's not right."

What landed on 2026-11-06 and 07 (L1 and its fixes) is a platform service:

- A1. The materializer's `candidateShotGate` starts a runtime goroutine
  (`selfdev_candidate_shot_worker.go`) that stages the running and the
  host-built builds, runs `internal/shotsandbox`, and judges the pair by
  calling a vision model directly (`modelpolicy.VerifyImages`), with its own
  failover loop. Its `candidate_shot` event is signed by the platform, not a
  desk. Its verdict decides the swap.
- A2. Inside management's `choir.Verify`, the reducer calls the same direct
  vision review on a legacy screens bundle and can flip the desk's `pass` to
  `fail` (`rlm_reduce.go:1840-1895`).

Three fixes in a row on A1 (sandbox reach, socket length, review quota) were
symptoms of this. The audit of 2026-11-07 (§8) found the shared cause: desks
have no shot verb and cannot see images, so every caller took the platform
route.

## 2. The desk pattern, applied

- A desk decides to shoot, sees the images with its own model, and records
  its verdict as a semantic act on the tape. The platform keeps only what
  custody needs: the host build, the sandbox substrate (as a verb's
  implementation, like `choir.Research` uses the research machinery), and
  the swap.
- The desk is management: it owns the independent judgment of a candidate
  (doctrine; it already runs `choir.Verify`).
- One model path: management's own run, with the tool loop's failover. No
  `VerifyImages` on the apply path.

## 3. Design

### 3.1 Order

Unchanged up to accepted: freeze (source only), management's Verify (text
evidence, the plan's coverage of the request's numbered shots), standing
consensus with the host build, accepted. The host build stays keyed by the
final bundle digest (the audit: an earlier build would need a new build
identity; out of scope).

At accepted, before the swap, the gate needs management's shot verdict
instead of the worker's event.

### 3.2 The gate asks management (replaces the worker)

`candidateShotGate` (materializer, accepted, not journal-only):

1. A shot verdict event exists for the operation (§3.3, read by its
   idempotency key `shot-verdict:<op>:<job>` with `EventByIdempotency`,
   exactly as the gate reads the worker's event today) and is bound to this
   host-build job, the shot job it judged, the bundle digest and the plan
   digest: pass continues to the
   swap; fail closes the operation failed with management's reason
   (`shot_review_failed`). Only the `ShotVerdict` reduction writes this
   event kind; the gate never scans commitment records.
2. Before asking, the gate checks that management can see images: it
   resolves management's effective model selection and chain (the order
   the tool loop would build) and asks `modelcatalog.SupportsImage` of each
   member. With no image-capable member it closes the operation failed at
   once, naming the cause and the model policy to change. It never waits
   on a judge that cannot look.
3. Otherwise it commits, once, a record addressed to management: "shots
   owed" for the operation, carrying the operation id, the host-build job
   id and the plan digest (deterministic record id `shots-owed:<op>:<job>`,
   so a repeat is a replay), and returns pending. The 60 s retry wake
   stays; it re-runs the gate, not management.
4. Deadline: 45 minutes after the shots-owed record, or when the notice
   cap of §3.4 is spent, the gate closes the operation failed "shots not
   judged in time". An accepted operation counts as busy
   (`selfdevBusyStates`) and blocks engineering admission, so it may not
   wait unbounded.

### 3.3 Verbs (management only)

Added to the yaegi `choir` surface, management's module set, the broker
switch and allowlist, `deskmanifest` rows and `ValidatePayload` entries;
`security_epoch` rises with the manifest.

- `choir.ShotStart(operationID)` starts the shots of the operation's plan:
  the running build (before) and the staged host build (after), over one
  state copy, in the existing sandbox. It returns a shot job id. The work
  is the existing staging and sandbox code (and its work-root sweep), moved
  behind the verb, not a new service. It is refused unless the operation is
  accepted and owes shots. One live job per (operation, host-build job): a
  second call returns the live or finished job. Every boot, planned or
  crash, closes an open shot job "ended by a restart" (the sandbox and
  browser die with the process; the host-build precedent,
  `hostbuild/service.go:187`), and a later `ShotStart` starts a new job
  with a new id.
- `choir.ShotWait(jobID, maxSeconds)` waits up to `maxSeconds` (at most
  240, inside the 300 s cell deadline) and returns `{state, screens}`:
  `state` is running, done, failed (with the cause) or restarted (the job
  was closed by a boot; call `ShotStart` for a new job, never a verdict);
  `screens` lists
  name, sha256, width and height per shot, in NN-before/NN-after order.
  When the state is done, the verb also queues the job's images for this
  run's model (§3.5): the run sees exactly the job's screens and no other
  ref. The queueing is idempotent per (run, job): if the run's memory since
  its last compaction already holds that job's image turn, nothing is
  queued again. If queueing would take the run's images past the request
  cap (an earlier job's turn is still in memory), `ShotWait` refuses by
  name and management finishes in a fresh run. There is no free-form View verb, so no desk can send an arbitrary
  stored image to a model through this path.
- The deterministic checks the worker and the A2 branch carried stay
  code, not prompt text: `ShotWait` fails the job on a blank shot or a
  shot that is not the recorded size, and reports the recorded size (the
  verdict's size question is the recorded size, never the model's
  estimate).
- `choir.ShotVerdict(operationID, verdict, findings, reason)` records
  management's verdict on the pair: `pass` or `fail`, the reason (required
  on fail), and one finding per screen pair (its sha256 and the answers to
  the overlay's six questions). It is a staged intent reduced at cell end,
  in the style of `commitVerifyIntent`, into the dedicated shot verdict
  event of §3.2, bound to (operation, host-build job, shot job, bundle
  digest, plan digest). It is refused unless the operation is accepted and owes shots,
  unless the findings cover every screen of the job, and unless the run's
  memory since its last compaction holds the injected turn carrying every
  screen of that shot job (the persisted injection message entries, not an
  in-memory map, so a passivated or resumed run is judged the same way; a
  continuation run with a new run id must call `ShotWait` again).
  Images are injected after the cell that called `ShotWait`; the verdict
  must come in a later cell, and the overlay says so.
- The job result (state, screens, image refs) is durable, keyed by
  (operation, host-build job), so a planned restart does not lose it.

Every verb call is on the tape like the other verbs (the cell's intents).

### 3.4 Management acts on the record

A record addressed to management is not enough by itself: the repo already
learned this with Verify (`management-ends-without-verify-2026-11-04`). The
shots-owed record gets the same four parts as verify-owed, as a sibling in
`verify_owed.go`:

- a completion guard: a management run with an accepted operation owing
  shots and no verdict may not end quietly;
- an end-of-run backstop continuation;
- a capped notice, each with a fresh wake id (a content-identical record
  dedups into a consumed wake, so re-committing the shots-owed record never
  re-wakes management; the notice does); the cap feeds the deadline of
  §3.2;
- a typed wait `WaitKindShotsOwed` in management's pack.

None of these arms at boot (it is not a resumer).

### 3.5 Images reach the desk's own model

- The queued images use the tool loop's existing user-turn injection
  (`InjectUserTurnsFunc`); the message shape is the one `runVerify` builds
  (a text label, then `{"type":"image","source":{"kind":"artifact_ref"}}`
  per shot), resolved by the gateway provider per request. No provider
  adapter changes.
- DS1: the tool loop's failover skips members that may not take an image
  when the messages carry one, in both the quota order and the failover
  branch, and moves a text-only current member to the first image-capable
  member before the call, through the failover path's own marking (the
  marked member, the failover cache key and conversation id), so the run
  does not drift back; a run with no image-capable member fails with a
  named error. Pinned and no-failover selections are not rerouted. A media
  refusal is a non-failover 400, so the move must happen before the call.
- The guest gateway client (`gatewayruntime.Provider.resolveImageRefs`)
  inlines each `artifact_ref` into the wire request only; DS2 checks that
  management's provider has the image resolver wired.
- Cost. A request carries at most 20 images (`MaxImagesPerRequest`), and
  images persist in run memory, so every later call in the run carries
  them. A plan judged by management therefore has at most 10 views (20
  shots); a larger plan is refused at Verify with that reason.
  `estimateRawMessageTokens` counts about 1.5k tokens per image block, so
  compaction sees the real weight. Compaction keeps text only; a run that
  compacted after its injection must call `ShotWait` again (which
  re-queues the images) before `ShotVerdict`, and the viewed-set check of
  §3.3 enforces it.

### 3.6 Management's prompt

The management overlay gains a "Shots owed" section: on a shots-owed
record, ShotStart, ShotWait until done, look at the pairs in the next turn,
judge each on the six questions already in the overlay (the request's
change is not a regression; reflow from a changed string is not; any
unrequested restyle is), then ShotVerdict with one finding per pair. A
failed shot job is a `fail` verdict naming the cause; a restarted job is
not a failure: ShotStart again. For a package bundle with screens, Screens
then Verify with one finding per screen. The overlay's
ProductAPI vision route note stays until that route is removed (residual).

### 3.7 Screens bundles (A2)

The reducer's direct vision override is deleted, and its job moves to
management. Package operations still carry the engineer's screens in the
bundle (`requireScreensOrPlan`; the gate skips package operations), so
they need a judge:

- `choir.Screens(operationID)` (management only) queues the frozen
  bundle's screens for this run's model, with the same injection,
  idempotence and cap as `ShotWait`. It is refused unless the operation is
  frozen or verified and its bundle carries screens.
- On a bundle that carries screens, management's `Verify` pass must carry
  one finding per screen (sha256 and the six answers) and is refused
  unless the run's memory since its last compaction holds every screen
  (the same check as `ShotVerdict`). Verify is the verdict.
- A platform bundle with screens and no plan is refused at Verify (platform
  shots are taken after the host build); a package bundle is judged as
  above.

### 3.8 Restart

Open shot jobs close at every boot (§3.3). For the operation, the existing
rule covers it, with no new closure code: a crash boot marks
accepted operations interrupted and settles them failed "interrupted by a
restart" (`self_development_materializer.go:221-251`); a planned boot
leaves them accepted, the §3.4 notice re-asks, and management starts a new
shot job (a finished job's durable result survives and can still be
judged). The open shot job closes with its operation. New: settling the
operation also resolves its shots-owed record, so a fresh management run
does not chase a dead operation.

### 3.9 Deletions

- The worker goroutine, its watchdog and its review caller in
  `selfdev_candidate_shot_worker.go`, and the platform-signed
  `candidate_shot` event's writer; the gate's event reader reads the
  verdict event instead. Staging, the reach check, the sandbox launch and
  the work-root sweep move under `ShotStart` with their tests.
- `reviewCandidateShots` / `reviewShotsNoted` / `shotReviewCaller` and the
  A2 branch in `rlm_reduce.go`.
- The release witness joins the shot verdict event, so release evidence
  names who judged the pair.
- `modelpolicy.VerifyImages` loses its apply-path callers; the `runVerify`
  failover of 2026-11-07 stays for the ProductAPI route until that route
  is removed (residual).

### 3.10 Order and consensus

Standing consensus votes on the host build before any shot exists. A shot
fail then closes an operation the consensus accepted; the shots are the
stricter gate, and the fail reason reaches the engineer's report. A shot
fail does not open a revision round (it is not a Verify rejection);
unchanged from today, a residual. Moving the host build and shots before
the vote, so the consensus is the single close, changes the cutover shape
and is a residual, not this slice.

## 4. Failure modes, decided before the code

- FM1 Management never answers: the guard, backstop and capped notice of
  §3.4; then the 45 min deadline closes the operation failed. No swap
  without a verdict.
- FM2 A verdict for another job, digest or plan: refused at the verb; the
  gate reads only its own idempotency key and binding.
- FM3 A pass without seeing the shots: refused (the run memory since the
  last compaction lacks the injected screens, or the findings miss a pair).
- FM4 Management's member cannot take images: DS1 moves it to a capable
  member; none in the chain: the gate closes the operation failed before
  writing the record (§3.2 step 2).
- FM5 ShotStart twice: the same job. After a planned restart: the durable
  result, or a fresh run of a job with no result.
- FM6 A shot job fails (sandbox, browser, state copy, blank or mis-sized
  shot): ShotWait returns failed with the cause; management's verdict is
  fail.
- FM7 Restart mid-job, planned or crash: the job closes at boot (§3.3,
  §3.8); a planned boot re-asks and management starts a new job.
- FM8 A desk other than management calls a verb: refused by the module set
  and the broker.
- FM9 ShotStart or ShotVerdict for an operation that owes no shots:
  refused.
- FM10 Too many images: a plan over 10 views is refused at Verify; the
  injection never exceeds the request cap.
- FM11 Image weight: each request re-sends the images (the guest gateway
  client inlines them per request) and a model move breaks the provider cache. Bounded by
  FM10 and by the run ending at its verdict; compaction counts them
  (§3.5).
- FM12 Compaction after injection drops the images: the verdict is refused
  until `ShotWait` re-queues them.
- FM13 A planned boot closes a job mid-shot: `ShotWait` returns
  restarted, not failed; management starts a new job.
- FM14 `ShotWait` or `Screens` called twice: queued once per (run, job);
  over the request cap: refused by name, never a bricked run.
- FM15 A package bundle with screens: management Views them with
  `Screens` and its Verify carries per-screen findings; no direct
  platform review remains.

## 5. Slices

- DS1 (toolregistry): image-aware failover (§3.4 second bullet). Lands
  alone; benefits every desk.
- DS2 (agentcore, yaegikernel, runtimeprompts, deskmanifest): the verbs, the
  image queueing, the gate change, the capability check, the shots-owed
  record and its guard, backstop, notice and wait, the deadline, the
  durable job result, the verdict event, the image token estimate, the
  plan view cap, the overlay, the deletions. Red, tier 2.

Proof on d@e: a platform one-line change with a one-view plan reaches
applied with management's ShotVerdict pass on the tape, management's run
showing the injected images and the model that saw them; then revert (the
revert proof); a forged verdict and a verdict without the injected screens
are refused.

Rollback: revert DS2 (the worker returns with it); DS1 is independent.

## 6. Out of scope (named residuals, from the audit)

The audit's other platform sites are not on the apply path and predate this
design: the host judge service (sub-call, desk-cast, inert, support
judges), intake labels, the event layer, story analysis, the prompt router,
the disclosure ask, front-page importance, install review seats, and the
MA7.10 anchor sampler. Each needs the same treatment (a desk act, recorded),
mostly through a desk-callable judge verb; that is the next design.

## 7. Review

r0, Sonnet seat: send back, six blocking. All adopted in r1: B1 the gate
deadline (§3.2.4); B2 the verify-owed sibling (§3.4); B3 the capability
check before the record (§3.2.2); B4 the image cap, token estimate and both
failover paths (§3.5, DS1); B5 the viewed set from run memory with
per-screen findings (§3.3); B6 the dedicated verdict event and durable job
result (§3.2.1, §3.3). Non-blocking: N1 the restart rule is the existing
one (§3.8); N2 deterministic checks stay code (§3.3); N3 ordering stated
(§3.10); N4 surfaces listed (§3.3); N5 FM11 reworded; N6 no adapter change.
Simpler design adopted: View merged into ShotWait; the legacy screens
bundle is refused, not judged.

r0, Devin seat: send back, three blocking, converging with the Sonnet seat
on two (no deadline, one-shot ask) and adding a third: an open shot job
must close at every boot, planned too, or ShotWait polls a zombie. Adopted
(§3.3, §3.8). Non-blocking adopted: the verdict binds the shot job id; the
viewed set comes from persisted injection entries and a continuation run
re-queues; the pre-call move reuses the failover marking; the release
witness joins the verdict; FM11 names the guest gateway client. Considered
and kept: a synchronous ShotStart (no job) would delete the job lifecycle
but risks the 300 s cell deadline.

r1, Devin re-review: send back, three blocking, all adopted in r2. B1
package bundles still carry screens and the A2 branch was their only judge:
`choir.Screens` and per-screen findings on Verify (§3.7). B2 a
restart-closed job read as a failure: the restarted state (§3.3, FM13). B3
re-queueing could exceed the request cap: idempotent queueing per (run,
job) and a named refusal (§3.3, FM14). DS1 review, finding 2: an image turn
does not walk the computer's continuation chain (a quota hit does); this
is deliberate for now, and the gate's capability check (§3.2) reads the
same order the loop uses.

## 8. Audit (2026-11-07, summary)

Sixteen platform sites outside the desk pattern: A1 to A3 on the apply path
(A3, standing consensus, is a mechanical quorum authorized by the owner's
mode row), B1 to B11 host-judge and System1 consumers, C1 install-review
seats, C2 the anchor sampler. Only `choir.System1` (research only, text
only) is a desk-callable judge. No desk had a shot or view verb.
