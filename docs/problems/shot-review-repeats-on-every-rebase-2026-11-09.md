# The candidate shot review repeats on every platform rebase (2026-11-09)

Status: problem record. No fix in this commit. Class: green (docs).
Design that answers it: [shot-verdict-reuse-2026-11-09.md](../design/shot-verdict-reuse-2026-11-09.md).
Times are UTC. Source: d@e's `/api/runtime/observability` (computer-59c902c5d46caecc0a180a8557164e72,
read 2026-11-10 01:00 and 01:02) and one read of the gateway journal on node-b for
vm-70070cf7 (3 hours to 01:00).

## The owner's words (2026-11-09)

"I don't think we should need to have to do the same check on every release.
That's going to add up for every computer. Once we check it once, shouldn't it
be good to go?" He approved fixing it as a prerequisite to the next mission.

## What happens on every deploy (confirmed from code and the trace)

1. A computer that carries a self-developed platform change serves a layered
   release: its patch on a base commit. A deploy boots a newer base. Boot sees
   `stack_base_moved`, then `stack_rematerialized` (00:38:52 and 00:38:58 tonight;
   `internal/agentcore/rebase_notice.go`, `source_projection.go`).
2. About five minutes later the rebase timer calls `cleanRebaseBeforeNotice`
   (`internal/agentcore/clean_rebase.go:137`). If the same patch bytes apply to the
   booted base, `runCleanRebase` (`clean_rebase.go:269`) opens a new operation whose
   bundle is the approved bundle with only the base fields changed
   (`selfdevprotocol.CleanRebaseBundleFields`), has the host build it, and accepts it
   under a receipt without an owner act (00:43:51 start, 00:46:23 accepted).
3. The materializer then runs the same gate as for any new platform change:
   `candidateShotGate` (`self_development_materializer.go:725`,
   `selfdev_candidate_shot.go:75`). It computes `platformShotRelevantPaths(bundle)`
   from the bundle's own file effects, which for a clean rebase are the layer's own
   patch, byte for byte the same as last time. That set is non-empty whenever the
   layer touches any non-exempt path, whatever the platform diff was. The exemptions
   (`capsule.ScreenExemptPath`, `screens_required.go:93`: docs, tests, build output)
   never look at what the base changed.
4. The verdict is looked up under `shot-verdict:<operation>:<host build job>`
   (`computerevent.ShotVerdictKey`, `shot_verdict.go:45`), and the payload must match
   operation, host build job, bundle digest and plan digest
   (`selfdev_candidate_shot.go:125`). A clean rebase has a new operation id and a new
   host build job, so no earlier pass can ever match. That is deliberate (FM2, a stale
   or foreign verdict must not unlock a swap; header comment, `selfdev_candidate_shot.go:33`).
5. So every rebase owes a new shot: management is woken with a "shots owed" record,
   calls `ShotStart` (two sandbox passes, before = running build, after = candidate,
   over a copy of the computer's state), `ShotWait`, looks at the images on an
   image-capable model, and records `ShotVerdict` (`desk_shots.go`). The accepted
   operation blocks engineering admission until the verdict or the 45 minute deadline
   (`desk_shots.go:60`).

## Tonight's round on d@e (confirmed unless marked)

The ring buffer in `recent_log` starts at the 00:38:51 boot, so only the round after
the 03300ad5f deploy is readable in full.

| Time | Event |
|---|---|
| 00:38:50 | deploy of 03300ad5f reaches d@e (`build.deployed_at`) |
| 00:43:51 | clean rebase starts |
| 00:46:23 | host build 1163a44c done, rebase accepted, shots owed |
| 00:48:19 | shot job done in 1m51s |
| 00:50:14 | verdict fail: "both shots show the identical Loading... state over the same hearts-and-stars background; q2 and q3 are no" |
| 00:53:54 | rebase notice minted for the model route (engineering) |

- Wall time of the round: 6m23s from start to verdict; 3m51s with the operation
  accepted and blocking engineering admission.
- Gateway calls for vm-70070cf7 between 00:46:00 and 00:50:40: 14, all succeeded.
  4 on provider `devin` (24,183 input tokens, 976 output) before the shot job ended,
  10 on `openrouter` (151,885 input, 8,117 output, 104,064 cached) after it. Total
  176,068 input, 9,093 output. Hypothesis: the devin calls are management's
  `ShotStart` and `ShotWait` cells (text only), and the openrouter calls are the image
  turns on an image-capable model after failover. The gateway line does not name the
  role, and my read cut off the model name, so which model took the images is
  unconfirmed. Dollars are not computable from this read (OpenRouter calls may be
  free-tier; Haiku rows were earlier).
- What the platform diff was: `d6d91f1a6..03300ad5f`, the base the layer left and the
  base it was rebased onto, is 17 files, all Go, docs and one nix script
  (`git diff --stat`). No file under `frontend/`. The last commit on main that touches
  `frontend/` is 1c2dc2a7d (2026-11-09 23:27Z); nothing from it to the current main
  (0ebab72a7) does. The layer was rebased onto a base after 1c2dc2a7d (d@e's
  `effective_release.base_commit` is d6d91f1a6).
- Earlier rounds, hypothesis only: the gateway journal shows round-shaped clusters
  (9 to 10 calls, 107k to 163k input tokens) at 00:06 to 00:10 and 00:34 to 00:38 on
  openrouter, and Bedrock clusters at 22:31, 22:55 to 23:09 and 23:42 to 00:10. The
  journal does not tag the work, and `recent_log` has been reset by each boot, so I
  cannot say which are rebase rounds. What is confirmed is that the layer being
  rebased, `clean-rebase-e0c17f9c`, is itself a clean-rebase operation, so at least
  one earlier round passed and was applied. The owner counts five rounds tonight;
  the trace confirms one failed round in full. Passes, timeouts and per-round token
  counts for the other four are not verified.

## What a failed rebase leaves the computer running (confirmed)

The gate failure ends in `recordMaterializationFailed` with "the computer keeps its
current release" (`self_development_materializer.go:727`). Current release is the
layered release on the old base: after the failure, `effective_release` still shows
`layered: true`, `base_commit: d6d91f1a6`, marker `selfdev-f7b2a20a`, while the host
build is 03300ad5f. The computer is behind the fleet on every platform change since
d6d91f1a6, including the Devin provider and its routing, until a second path lands. That
path is the rebase notice (00:53:54), which goes to the model route and costs another
engineering round. This is the shape already recorded in
[rematerialized-stack-never-served-2026-11-04](rematerialized-stack-never-served-2026-11-04.md):
a self-developed computer stops getting platform updates. A shot verdict that fails on
a rebase therefore costs more than the failed round: it costs the update.

## The separate finding: the "Loading..." verdict

The failed verdict says both shots, before (the running build, which is the serving
production build) and after (the candidate), show the app in its Loading state. A
page that is stuck on Loading in the running build too is not evidence about the
candidate. Two readings fit and the trace does not separate them:

- H1: the shots were taken before the app finished loading in the sandbox (the view's
  steps do not wait for a ready marker, or the sandbox's session or state copy gave
  the shell nothing to load).
- H2: both builds really did not render in the sandbox (a reach, auth or state-copy
  fault in the sandbox, not the candidate).

Evidence for "not the candidate": the "before" shot has the same state, so the fault
is shared by both passes. Evidence the capture is not blank: `shootCandidate` refuses
blank PNGs (`selfdev_shot_job.go:~390`), and these passed, so the pixels were a real
page, the Loading screen. The plan's own steps decide when a shot is taken
(`internal/shot/shot.go`, `runStep`); a plan without a `wait_for` on a ready selector
shoots whatever is on screen. I did not read the images or the plan (both on the
computer's tape and files root) so neither reading is confirmed.

The rule also asks the wrong question of a rebase. The `shotsOwedProcedure` text treats
BEFORE as the base build, "cannot show the requested change", and judges AFTER by q2
("shows what its name describes"; "a change with nothing visible leaves AFTER matching
BEFORE: that is q2 yes"). For a rebase, BEFORE already contains the change. A judge
who reads the Loading screens literally answers q2 and q3 no, which is what happened.
Hypothesis: the same shots would have drawn a pass from a judge who does not read
"Loading..." as a placeholder defect (q3). The verdict turns on judge variance as much
as on the candidate.

## Belief state

- Confirmed: every clean rebase reruns the whole shot interlock; the binding makes any
  earlier verdict useless; the failed round cost 14 model calls and 6m23s and left the
  computer on the old base; the platform diff of the failed round touched no frontend.
- Confirmed: there is no skip for a rebase
  whose patch is unchanged.
- Hypothesis: "Loading..." was a sandbox readiness or reach fault shared by both builds.
- Hypothesis: four more rounds tonight, mostly passes, at 10 calls and 100k to 160k
  input tokens each.
- Not verified: whether the host build of an unchanged tree gives a byte-identical
  frontend tree (the digest rule depends on it; test in the design).

## Remaining error field

1. A computer that carries a layer pays image-model calls, a 4 to 45 minute blocked
   operation and a failure chance for each platform deploy, whatever the deploy touched.
2. A failed review strands the computer on the old base.
3. The Loading finding may mean shot passes are unreliable as a gate in general.


## Investigation 2026-11-09 evening

Read-only: d@e at about 01:30Z (build 03300ad5f, `effective_release`
unchanged from the section above: `layered`, base d6d91f1a6, `diverged:
true`). Status: the design is being built by another builder; nothing here
re-opens it.

Confirmed again: the failed round's log lines stand as written (verdict 00:50:14,
notice 00:53:54), and d@e is still on the old base 40 minutes later. The
notice's pair was claimed by an engineering assignment at 01:08:59, and an
engineering run is live in a capsule on d@e at 01:31Z (iteration 104). So the
recovery route is working; its cost is the point of the record.

The one thing to add for the build: what stranding remains after verdict reuse
is written into
[rematerialized-stack-never-served-2026-11-04.md](rematerialized-stack-never-served-2026-11-04.md)
(Investigation 2026-11-09 evening): rung D failures, the first rebase after
the build (no attested prior), driver changes, and a failed *job* treated as a
failed *verdict*. For the builder: S0 should make "not ready" a failed job
with a named cause, and the rebase branch of the gate should not turn a failed
job into `recordMaterializationFailed` on the same path as a judged fail.
The "Loading..." question (H1 sandbox readiness versus H2 both builds really
unrendered) is still open; neither the images nor the plan were read.
