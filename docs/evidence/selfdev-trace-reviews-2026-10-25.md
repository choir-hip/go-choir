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
