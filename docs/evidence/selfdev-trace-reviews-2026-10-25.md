# Self-development trace reviews, 2026-10-25

Read-only reviews of self-development rounds (owner, 2026-10-25), green. Sources:
`choir trace export` (run events with tool arguments and outputs), `choir
lifecycle capsule-evidence`, the observability endpoint and the node-b gateway
journal, read against `origin/main`. **Finding** means a trace line shows it;
**hypothesis** means code reading or inference.

## Round 1: reader-3 repair (assignment-99bc7d03), 2026-10-25

Computer b@c, trajectory e3e37430, engineer on Synthetic DeepSeek V4.1 Flash
1017 (reasoning effort low), deployed build 4e72dec (00:38 UTC). The round ran
01:00:47 to 02:00:50 and was cancelled by the 60 minute activation budget
(`context deadline exceeded`, `DefaultActivationBudget` in provideriface). It
made 159 model calls (one `capsule_go_eval` each) and never called Freeze or
Complete: the cancel report has no commands, mutations or candidate.
The round's first engineer (gpt-6-luna) had read the spec, written a 26 KB
Reader package and frozen in 2 min 40 s over 30 calls.

### 1. What the engineer did

| Phase | Time (UTC) | Calls | Content |
|---|---|---|---|
| Orient | 01:00:47-01:01:50 | 1-21 | Read the cast, 3 precommits, listed the tree, read App.svelte, auth.js, PdfApp. Reported to management at 01:01:50: the root route stays on Loading because a capture without an auth service keeps `authState` checking/reconnecting. |
| Author | 01:02:09-01:07:15 | 22-56 | Wrote a new `/read` direct route (ReaderRoute.svelte, 11 KB), ReaderBootstrapGate.svelte, read.html, reader.js, a vite.config.js change, App.svelte edits, a Chromium DOM-dump helper, PDF and EPUB fixture builders (an EPUB zip with hand-computed CRCs in Go). 4 cell errors here from Go raw-string and `import "os"` limits. |
| Build and capture | 01:07:19-01:08:57 | 57-74 | Frontend build (attempt 1 failed in 1 s on a Svelte error, attempt 2 done in 9 s), release copy, `choir-capture` (5 s), ad hoc Chromium probes, then authored `tests/reader-route.test.mjs`. |
| Test harness loop | 01:09:01-01:56:27 | 75-159 | Ran its own Chromium-based test 9 times, edited the helper and test between runs (12 cells re-fixing doubled backslashes), plus a Go test (see 2). |

Call mix: 47 edit or write, 43 Poll, 32 read or exec, 29 job starts, 8 other;
6 cells errored (3.8%). Model provider time totals 13.6 min of the 60; tool
cells total 55 s; the run was parked on jobs for 45 min (75%): 40.7 min in
finished parks (39.3 of them in 8 parks over 60 s) plus 4.3 min in the park open
at cancel. Roughly 47 minutes
(01:09 on) went to a bespoke harness that proves the new `/read` route, not to
the Reader the owner asked for. **Finding:** no call ever mentions
`packages/reader`.

### 2. What fills the 4.5 minute gaps

The gaps are not model time. The engineer's cell starts a job and the next Poll
shows it running; the runtime then parks the tool loop on the job's end
(`engineeringJobPark`, `capsule_job_park` events) and calls no model until it
ends. Model calls on both sides take 2-3 s.

| Park (UTC) | Seconds | Job |
|---|---|---|
| 01:10:00-01:14:30 | 269 | reader-route test, run 3 |
| 01:15:06-01:19:35 | 269 | same test, run 4 |
| 01:19:54-01:24:23 | 269 | same test, run 5 (NO_PROXY set) |
| 01:24:37-01:29:36 | 299 | serve probe, ended state `timeout` (5 min wall clock) |
| 01:30:09-01:34:40 | 271 | same test under `timeout 200` |
| 01:37:36-01:44:49 | 433 | `go test ./internal/visualcapture ./internal/updater` |
| 01:46:05-01:50:34 | 269 | same test, run 7 |
| 01:51:33-01:56:10 | 277 | same test, async Chromium spawn |
| 01:56:27-02:00:47 | cut off | same test, open at cancel |

**Finding (cause of the 269 s):** each failing run had 3 Chromium dumps that
returned `bytes=0` (`FAIL ... chromium exit 0` x3 in the run output), and the
helper's `spawnSync` carries `timeout: 90000`: 3 x 90 s. The test serves the
page from a server inside the same Node process, and `spawnSync` blocks that
process, so Chromium never gets its page. The engineer found this at 01:50:46 (standalone probe,
`LOCAL code 0 bytes 57`) and made the helper async at 01:51:14; the next run's
documents loaded. It had tried proxy settings, `NO_PROXY`, a Host header fix and
a loopback proxy first. The same
symptom was re-run 5 times (01:09:58, 01:15:02, 01:19:51, 01:30:06, 01:46:00),
about 22 minutes, when the 1-second probe was always available.
**Finding:** `timeout 200` did not bound the job (271 s; each run prints
`timeout: warning: timer_create: Operation not permitted`). After the async fix a
run still took 277 s: **hypothesis**, Chromium dumps of the "Reconnecting" page
cost about 20 s each.

The 433 s park is a cold Go module cache: the egress log shows 466 `ok` GETs to
`goproxy.choir.internal` (about 50 MB) at 01:37:34-38. The change was
frontend-only; the cast asked for "relevant legacy/package checks".

Avoidable: yes. A 10 s smoke probe before a full test, per-step timeouts of 15
s, and `choir-capture` (5 s per run) cover the need; the go test was optional.

### 3. Context growth

| Call | Time | Messages | Input tokens |
|---|---|---|---|
| 1 | 01:00:49 | 2 | 7,274 |
| 22 | 01:02:09 | 44 | 41,297 |
| 50 | 01:06:01 | 100 | 71,046 |
| 90 | 01:14:32 | 184 | 101,788 |
| 135 | 01:44:52 | 282 | 130,813 |
| 159 | 01:56:27 | 335 | 149,263 |

**Finding:** history is append-only (2 messages per call, about 0.9K tokens per
call, 33K of it in the first 21 read-heavy calls). Elision is off by default and
research-only; compaction has not run (no compaction event in 545 run events).
Totals: 14.34M input tokens, 97.3% cached (13.95M), 0.39M uncached, 55K output.
Five full cache misses (calls 25, 51, 93, 104, 151; 35-72K each) are 64% of the
uncached tokens. The cache survived every 4.5 minute park (calls 90, 97, 102,
110, 135, 148, 157 were 99% cached). A call at 140K is about 139K cached read
plus 1K new input; latency is 2-4 s for a short reply and 10-30 s when the
engineer writes a 1-3K token file.
Window risk: the model is catalogued at 524,288 tokens and compaction starts at
70% (about 367K). At 149K the round was at 28% of the window; at 0.9K per call
the threshold is about 245 calls away, and the 60 minute budget ends the round
first. No window risk here.

### 4. Did the repair round get the earlier work?

No. **Finding:** it started from the base. Evidence: the assignment's
`subject_digest` (3c7721e5) equals the first round's original subject, and the
run metadata has `source_candidate_id: ""`. In the capsule `packages: [image]`,
PdfApp.svelte and EpubApp.svelte exist, `check-packages` prints "1 package(s)
ok (image)" (round 1 printed 2), and no call or output contains "carried" or
`/selfdev`. There is no carried-patch offer in the trace or in the node-b
journal. **Finding (why):** the deployed build 4e72dec predates 1d0f0eba8
(21:15 -0400), which carries a rejected or failed finished candidate; the older
rule carries only a round that did not complete, and round 1 reported
`completed`. The cast asked the engineer to "diagnose why the candidate route stays on
Loading", which a tree without the candidate cannot answer. The next assignment
(8e9a, 02:01:54) also opened on the base.

### 5. Loops, repeats, refused egress

- The same Chromium test, failing the same way: 5 runs, 22 minutes (see 2).
- 43 Poll cells (27% of calls); about 17 are the Poll the resume text asks for
  after a park ends. Backslash thrash writing JS through Go strings: calls 76-87,
  111-113, 118-119.
- Egress refusals: 19, all within 01:50:46-49, five hosts (www.google.com,
  accounts.google.com, clients2.google.com, update.googleapis.com,
  redirector.gvt1.com), reason "not on the L2 ecosystem allowlist". **Finding:**
  one burst, the second after call 149 started a Chromium probe; it is
  Chromium's own background traffic, not an engineer retry, and the engineer did
  not react to it. **Hypothesis:** earlier launches did the same; the log ring
  holds 512 lines from 01:37:34, 466 of them the goproxy downloads.
- Two choir-capture runs gave identical artifacts for `/`, `/read` and the PDF
  route (f678... at 01:08): the capture server answers an extensionless route
  with index.html, so `/read` was never captured (`/read.html` was needed).
  Nothing warned the engineer.

### 6. Proposals, ranked by time or cost saved

| # | Proposal | Evidence | Saves |
|---|---|---|---|
| 1 | Deploy 1d0f0eba8 to the test computer and also carry the work of a round cancelled by the activation budget. Today the cancel report has no mutations, so 60 minutes of edits are lost and the next round restarts from base. Finding for the gap; hypothesis that carrying fixes the repair loop. | section 4; `reports[0].mutations: []` | a whole round (60 min, 14M tokens) |
| 2 | Make `choir-capture` answer `/auth/session` 401 (a definitive signed-out) instead of 503, and warn when two routes produce one artifact. Finding for the mechanism: capture answers 503 for `/auth/` (visualcapture/capture.go:344), auth.js treats 5xx as `TransientAuthError`, App.svelte stays on reconnecting/Loading, contradicting the overlay ("renders its signed-out state"). Hypothesis that round 1 failed verification because of it; one 5 s capture of the unmodified base `/` would confirm. | 01:01:50 report; section 5 | the false verification failure that opened this round |
| 3 | Hang guard for test jobs: ship a working `timeout` in the capsule (seccomp blocks `timer_create`), and tell the engineer in the overlay to smoke-probe in under 10 s and to treat a job with no new output for 60 s as hung. Finding. | 5 runs x 269 s; `timeout 200` ran 271 s | about 22-35 min |
| 4 | Show the engineer its remaining activation budget (park events already carry the job deadline) and say "freeze what you have" when under 10 minutes remain. Hypothesis. | cancelled at iteration 159 while parked, no Freeze | the salvage of one round |
| 5 | Do not run Go tests for a frontend-only change unless the cast names them, or preseed the module cache in the capsule image. Finding for the cost, hypothesis for the need. | 433 s park, 466 goproxy GETs | about 7 min per round |
| 6 | Put the job tail (last 1.5 KB) in the park-ended resume message so the follow-up Poll cell is not needed. Hypothesis on size. | 17 follow-up Polls | about 17 calls, 2M cached tokens (14%) |

Limits: the round ended at 02:00:50 while this review ran; all of it is covered.
`recent_log` could not show the first 40 minutes; `choir trace export` and the
gateway journal can, and are the better sources. Not checked: cache-miss cause.

## Round 2: reader-3 resume (assignment-b6628b7a), 2026-10-25

Computer b@c, trajectory e3e37430, deployed build 091d10411 (built 02:39 UTC). Sources: `choir trace export`
(re-exported 03:12 so it holds the whole of 13d84), `lifecycle capsule-evidence --attempt 1`, `self-dev show/list/visual`,
read against 091d10411. Finding = a trace line shows it; hypothesis = code reading or inference.

### 1. Engineer b6628 (02:59:59-03:05:50, 86 calls, all hf:deepseek-ai/DeepSeek-V4.1-Flash-1017 via synthetic, effort low)
- Orient and carry 02:59:59-03:01 (calls 1-30): read the cast, 5 precommits, found `/selfdev/carried/{carried.json,source.patch}`
  (calls 3-5). First `git apply` at the wrong cwd failed; the second at /workspace/platform applied 10 files cleanly. Finding.
- Author and build 03:01-03:04: 6 file writes, 7 job starts, 8 Poll cells; 6 job parks (03:01:58-03:04:06), each followed by a call
  that acts on the job tail with no Poll cell (e.g. call 67 at 03:04:00 "Build green"). Finding: the job tail in the park
  resume works. Model text at call 71 "Budget is tight (56 min ...)" shows it read the budget line (the line text is not in the
  trace: run events keep message counts, not text; hypothesis from code that it came from withActivationBudget).
- Freeze: 03:04:20 refused (it pasted a job receipt in the toolchain slot: "not a receipt this computer issued"), 03:04:27 froze
  (call 73, bundle 43bdc755, 131 changes). 7 s lost.
- 03:04:42 `choir.Report` to management (claim: Reader done, PdfApp/EpubApp NOT restored).
- 03:04:51 `choir.Complete("partial","none",...)` call 76: cell ok, report 33bc0bc3 recorded, output has NO `fate_terminal`.
- 03:04:56 end_turn with prose (call 77) -> completion-guard retry 1 (`loop.retry`, reason engineering_overlay_terminal_fate_required).
- Retry 1 (call 78): "a prior Complete staged here last turn did not reduce"; re-issued the identical Complete(partial) ->
  `tool_error: reduce: persist tray-1: lifecycle command digest conflict` (03:05:05). It then ran choir.Pack, and re-issued Complete(partial)
  with new wording (03:05:18, report 52b44459, cell ok, no fate_terminal), end_turn (call 81) -> retry 2.
- Retry 2 (calls 82-85): read an unread Note from management, replied (03:05:35), Complete(partial) a third time (03:05:45, report e31652cb,
  no fate_terminal), end_turn 03:05:49 -> `loop.cancelled`: "completion guard ... was not satisfied after 2 retries".
- Complete was never refused for being partial. The guard text it saw (code, runtime.go:3815 at 091d10411, not in trace) is:
  "The turn ended without a committed terminal fate. Finish the assignment ... call choir.Freeze ..., then choir.Complete once with
  execution_refs - or commit the corresponding terminal failure fate ... Narrative text alone does not end the run." plus
  "Activation budget: N min left; Freeze before it ends or the round's work is lost." It does not say that result="partial"
  is non-terminal. The overlay (rlm_engineering_runtime.yaml:39) does: "result=partial records progress (a non-final Report) and keeps
  the assignment open."

### 2. Why the operation failed (selfdev-79f71616, frozen 03:04:20, failed 03:05:50)
- Finding (code): `commitCompleteIntent` returns `result != partial` as the terminal flag (rlm_reduce.go:1734 ff); only `fate_terminal=true` satisfies
  `engineeringOverlayTerminalFateCommitted`. The engineer chose partial every time, so no terminal fate ever committed.
- Finding (trace): the run error text equals the operation's `error`; capsule fate revoke_requested 03:05:49.70, revoked 03:05:49.91,
  assignment disposition `cancelled`, cancel-report ed3a5b25 (no commands, no candidate). `failBoundSelfdevOperation` and
  `closeEndedAssignmentSelfdevOperation` close a frozen op when the assignment ends non-completed ("its candidate cannot be verified").
- Management's Verify at 03:05:07 was refused: "assignment ... is bound; only a completed assignment's candidate is verified". So verification
  never got a chance. The coupling run-ends-without-terminal-fate -> op failed is intended (probe-wedge guard, runtime.go comment);
  that it discarded a frozen, evidenced candidate over a one-word result choice is a gap (hypothesis that this was unintended).

### 3. Second assignment 13d84 (opened 03:05:22.8, bound 03:05:52.5, completed 03:10:04)
- Correction to the brief: 03:05:52 is its bind (`engineering-bind:...:1`), not its end. It ran 4 min 12 s, froze selfdev-a3eab154 at 03:09:46,
  completed `completed` 03:10:04 (report with 1 mutation), op awaiting_approval 03:10:22. It is open for the owner now.
- Why opened: management loop 1b3316ab (gpt-6-luna, submitted 03:04:42 on the engineer's Report). 03:05:07 Verify refused; 03:05:17 Note to
  the engineer ("confirm this round is final; if so I will open a follow-up"); 03:05:22.6 `choir.Cast` "Restore PdfApp/EpubApp from base commit
  091d10411f71 in the carried source tree"; it did not wait for the answer (given 03:05:35). Cast parent = Texture's cast record from run 230c5a4e.
- The "another candidate is open" freeze refusal did NOT fire: 79f7 was already failed (03:05:50) before 13d84 froze (03:09:46). The 30 s
  from 03:05:22 to 03:05:52 is the wait for b6628's capsule to be revoked (hypothesis; bind came 2.6 s after the revoke), not work.
- Contradictory inputs (finding): Texture run 230c5a4e (03:04:42-03:05:01, prompt_length 7243 = the original spec) re-cast the original spec verbatim
  ("Delete the old readers: remove PdfApp.svelte and EpubApp.svelte ...") and wrote revision ee41afb6 "The owner wants the deleted PDF and EPUB
  readers replaced ...", reverting the owner's 02:59 revise (retain both, Reader default). 13d84 got that body plus management's "restore" objective;
  it restored both files from the patch's deletion hunks (the workspace is not a git repo, exit 128: nothing to restore "from base 091d10411f71"),
  then removed pdf/epub aliases and `opens` from the Reader manifest to pass check-packages (Reader no longer the default opener). Texture's 03:11:04
  revision e83d9255 then rejects that candidate for "restored the legacy apps ... while removing the Reader aliases".

### 4. Carried tree and screenshots
- Carried: yes. Offer `choir.carried_patch.v1`, 113,548 B, source_assignment 8e9a8ff9 (completed, op 2f90 rejected), base_ref 4e72dec, delivered as a patch file,
  not applied; the engineer applied it (calls 4-6). The round built on it (check-packages "2 package(s) ok (image, reader)", reader.html added). Finding.
  Side effect: the carried patch deleted PdfApp/EpubApp, the opposite of the owner's retain directive.
- Visual evidence (self-dev visual, 1280x800, all blank:false resolvable:true), 79f7: `/` 62,793 B (fb70df7d), `/reader.html` 226,166 B (6d5c3f7c),
  `/reader.html?app=reader` identical digest. I viewed them: `/` is the signed-out desktop with the Autopaper window ("Could not load stories") - not
  Loading, so the 091d10411 capture fix works. `/reader.html` is the desktop under a "Sign in. Pick up where you left off" modal ("After sign-in: Open
  Reader with your saved work"): not blank, not Loading, but the Reader surface is never shown. The engineer called it "much richer" (call 70).
  a3eab (13d84): only `/` (62,796 B, e2eca035), the same signed-out desktop; the first capture job timed out ~60 s (768 MiB) and was retried.

### 5. Model and cost
Both engineers: hf:deepseek-ai/DeepSeek-V4.1-Flash-1017, provider synthetic, effort low (b6628: 86 calls, 4.97M input tokens, 95% cached,
17K output, 266 s provider time of 351 s run). Management and Texture ran gpt-6-luna. Round 1's 60 min / no Freeze became 4 min 21 s to Freeze.

### 6. Proposals, ranked by Reader unblock
1. Make a committed Complete(partial) after a Freeze impossible to strand: the guard reminder names the cause ("your Complete was result=partial, which
   keeps the assignment open; end with completed/failed/blocked") and, when the guard exhausts with a frozen candidate, close the assignment `completed`
   (verdict none) instead of cancelling, so management's Verify runs. Finding for the cause; hypothesis for the fix.
2. Texture must treat the owner's latest revision as the spec, not the original request: it re-cast the original "delete" text over the owner's retain
   revision (230c5a4e). Until then every candidate is judged against a contradictory spec. Finding for the mechanism.
3. Management: do not Cast a follow-up while the Note answer is pending and the assignment is open (cast 03:05:22.6, answer 03:05:35).
4. Reader evidence: ship a signed-out Reader fixture route (a sample PDF opened without sign-in) and require its capture; `/reader.html` shows only a sign-in modal.
5. Give the engineer the base bytes (or a base-only mount) so "restore from base commit" is executable; the workspace is not a git repo.
6. Deliver the carried tree already applied (and say which candidate and its deletions); the engineer lost a cell on the wrong cwd.

## Clustering assessment after rounds 1 and 2 (2026-10-25 03:3x, FP0)

Nine problems in the self-development loop in one night, so per CLAUDE.md (Root Cause Clustering) the next fixes are classified before more patches land.

**Cluster A: a desk acts on a partial projection of the state it is told to obey.** Texture builds its run prompt from the newest revision's metadata only, so one Texture revision after an owner revise the owner's direction vanishes (finding, lane fp0-texture-spec). The completion-guard reminder does not show the committed Complete's result, so the engineer could not see why partial did not count (finding). Management cast a follow-up from a Texture cast record before reading the engineer's answer (finding). The engineer did not see its activation budget (fixed, 091d10411). The common substrate is that each desk's context surface is a hand-built projection of the commitment ledger, and each projection drops a different field. A substrate fix is one ledger-derived "current obligations" projection (the latest owner directions in order, the open assignment and its last committed fate, open questions and their answers, the budget) that every desk prompt renders, instead of each prompt picking metadata keys. Not built; the three lane fixes are the symptom-level repairs for tonight, and each is behind a switch or is prompt text.

**Cluster B: a round that ends without a terminal fate discards its work.** The budget cancel threw away an unfrozen tree (round 1); the guard exhaustion closed a frozen, evidenced candidate as failed (round 2). Same substrate: the fate path treats "not completed" as "nothing to keep". Lanes fp0-carry-cancelled (keep the unfrozen tree) and fp0-partial-trap (refuse a partial Complete after a Freeze) close both entries; the residual question (should guard exhaustion with a frozen candidate close completed) is held for a red decision.

**Not clustered:** the capture's /auth/session 503 (a harness bug, fixed); the alias clash between the Reader and the old apps (a spec gap, resolved in the brief); no git base in the capsule; the carried tree arriving unapplied.

**Next substrate step (proposed, not started):** design the ledger-derived current-obligations projection as the one input every desk prompt renders (docs/design, Opus, reviewed as an architectural design), after tonight's three symptom fixes land and their staging proof shows which fields matter.

## Round 3: reader-3 after the f0c7f836 rejection (3440, 9d5d), 2026-10-25 04:35 UTC

Computer b@c, trajectory e3e37430, trace re-exported 04:32 (`trace export`, `lifecycle capsule-evidence`, `self-dev show`), build per the export 091d10411 (built 02:39), not b1667677d. Finding = a trace line shows it; hypothesis = code reading.
Lanes that landed after b1667677d (225323cee carry_unfinished, 02758edf9 texture.owner_directions, 345b8575e partial trap) are not in 091d10411, and the first two are flag-off; every problem they target recurred.

### 1. Assignment 3440 (03:45:16-04:12:08, 489 model calls, 488 cells, 0 jobs, 0 Freeze)
- Orient and carry 03:45:18-03:45:37 (calls 1-9): read cast (objective "build on selfdev-f0c7f836 ... pdf-classic/epub-classic ... signed-out sample PDF", record 66c94298 cast), carried.json (source_assignment 2d61, f0c7f836's tree). `git apply` at /workspace/platform printed APPLIED (call 8, 03:45:34). Finding.
- Self-inflicted confusion 03:46-03:53 (calls 15-146): re-implemented patch application by hand on the already-patched tree (call 22: "applied 6 errs 128"), copied the patched tree to /tmp/pt and saw "patch does not apply" (call 26), applied and reverted the patch 6 times (calls 32, 92-94, 122-125, 139). Never recognised that call 8 had applied it. Finding.
- Real edits (calls 118-135, 03:51:54-03:53:05): registry.ts gains pdf-classic / epub-classic, app-packages.spec.js updated. This is the owner's resolution, but it never ran a build, capture or check.
- Echo loop 03:53:52-04:11:22 (calls 147-480, 334 cells, 17.5 min, 68% of all cells): `echo CMD-n word-n+1` with every output correct (0 odd outputs). No model text. Finding. Ended only when the model end_turned (call 481, 04:11:25).
- Freeze: never called (0 of 488 sources mention choir.Freeze). Guard retry 1 at 04:11:25 (`engineering_overlay_terminal_fate_required`); then call 481 `rm -rf frontend/src/packages/reader` (04:11:28, it deleted the Reader package), re-restored PdfApp/EpubApp (call 484), rewrote registry.ts, Report at 04:11:57, Complete at 04:12:05.8.
- The capsule "frozen" step is the quiesce of the terminal-fate commit, not a Freeze: freeze_requested 04:12:06.071 comes 0.3 s after the model call that carried Complete (04:12:05.755), then frozen .573, revoke .830/.330, report 08.406; no self-dev op was created. Finding. The report did mint candidate key-e55c83 (subject f267de37); 9d5d's carried.json (15,973 B) names it.

### 2. How 3440 ended
Management did not cancel; no guard exhaustion; 27 of 60 minutes used. `choir.Complete("blocked","none",...)`, fate_terminal true, run state failed, assignment disposition failed, report bddde8ae. Summary: "Blocked by capsule tooling, not by the task. Verified defects: (1) git apply --check exits 0 but the tree never reaches the patched state ... (2) choir.WriteFile folds a blank line ... (3) capsule_go_eval repeatedly returned other conversations' transcripts ...".
All three claims are unsupported (finding): (1) is its own call 8; (2) is `println` adding a space after "hello\n" (call 34, receipt adb1caee); 9d5d proved round trip equal at 04:13:00 (receipt 09fb19e6); (3) all 488 receipts are distinct and every output matches its cell. Hypothesis: a low-effort Flash model in a degenerate loop invented a tooling cause.

### 3. Spec delivery
- 3440 got the resolution: cast from Texture 66c94298 (03:29:14, owner revise f0b70a62) carries pdf-classic/epub-classic, retain PdfApp/EpubApp, signed-out preview, Complete(completed). It did not get FP0's rejection text, only the cast. Finding.
- Texture then undid it. 830c5f00 (03:29:52, prompt 7243 = original) rewrote the doc to e14fec6c, dropping the classic ids. 4c325b75 (04:11:57, woken by 3440's Report; one cell, no ReadDoc/Records) and b540525e (04:19:10, woken by 9d5d's Report) cast the original text verbatim, including "Delete the old readers" (revs db71f934, 24caced4: "before deletion"). Prompts of 830c5f00, 4c325b75, b540525e contain no "classic" and no "f0c7f836"; only 66c94298 did. Finding.
- The original request itself orders "Texture: cast this request to management verbatim in your first cell", so each wake re-casts it. Finding (text), hypothesis (cause).

### 4. 9d5d (04:12:33-04:26:04) and 9e18 (04:26:06, running)
- 9d5d objective: "Continue the Reader replacement ... correcting the previous round's verified capsule/tooling failures ... avoid choir.WriteFile if its newline corruption recurs ... Preserve the old reader files ... only if its clean-base plan permits"; body is the original spec with "Delete the old readers". Not consistent with the owner (keep as pdf-classic). The 3440 false claims reached management through Report -> Texture -> management. It deleted PdfApp/EpubApp (04:16:55), captured only `/`, no Reader preview.
- 9d5d died on the guard at 04:26:04: "completion guard ... not satisfied after 2 retries", op selfdev-5b4eb184 failed. Two Freeze refusals (04:25:19 "workspace changed while job ran"; 04:25:57 "visual evidence from a different frontend build"). 9e18 (cast 04:19:54 while 9d5d ran) carries the same stale body; no carried tree.

### 5. Models
Engineering (faf32, 3440, 9d5d, 9e18): hf:deepseek-ai/DeepSeek-V4.1-Flash-1017, synthetic, effort low. Texture and management: gpt-6-luna, chatgpt, effort high. 3440: 58.7M input tokens (99.3% cached), 22.8 min provider time.

### 6. New problems
- N1 Guard kills an engineer whose job finished before the guard looked. faf32 (03:25:44-03:45:14, 19.5 min, a working Reader with sample PDF, preview route and 2-page render proof) and 9d5d both died on the 3rd end_turn. Guard reminders arrive 0.14-0.22 s after the end_turn (faf: 03:37:36.37 +0.22 s, 03:44:58.76 +0.18 s; 9d5d: 04:23:42.51, 04:25:07.97), while true waits take 1.5-15 s (`capsule_job_ended`, attempt not incremented). Finding. Cause hypothesis: `engineeringJobWait` (capsule_jobs.go:210) returns false when the job already ended (2-3 s probes, captures), so the generic reminder is charged; `engineeringJobPark` handles "already ended", the guard does not.
- N2 faf32's work was lost: cancel report 0 mutations, 0 candidate (carry_unfinished not live); 3440 carried the worse f0c7f836 tree instead. Finding.
- N3 Serialised slot: 3440 opened 03:29:19, bound 03:45:16 (16 min) behind faf32; 9e18 queued behind 9d5d. Management casts onto a busy slot and never supersedes the stale assignment. Finding.
- N4 Each engineer Report wakes Texture to re-cast the full spec (4c325b75, b540525e) -> duplicate assignments. Finding.
- N5 Freeze order: capture must follow the last build, with nothing between (2 refusals in 9d5d). The overlay does not say so. Hypothesis.
- N6 A refused first Freeze still opens an op row (5b4eb184 created 04:25:18.9, before the 04:25:19.3 refusal); it was failed by the assignment end. Finding.
