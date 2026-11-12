# Jev usage review, 2026-11-12 (read-only)

Question: are we still making excess parallel Jev calls where one call could carry many questions?

Tags: **O** observed (command or file:line given), **I** inferred (reasoning stated), **U** unknown.
Code is read at the working tree (HEAD 4faab153b). `git diff HEAD origin/main` over agentcore, gateway,
frontimportance, gatewayruntime and autoputer is empty, so lines equal origin/main 198880c03.
Reference pages read first: docs/reference/SYSTEM1.md and system1/our-usage.md, in full.

## 0. Method and data

- O: `ssh node-b "journalctl -u go-choir-gateway --since '2026-11-11 00:00' --until '2026-11-12 23:59' -o short-iso | grep -E 'jev|Jev judgment|host judgment|Started'"`, piped to
  scratchpad/jevrev/all.log (42,454 lines, 32,098 `jev call` lines). Nothing written on Node B; no receipts,
  env, key or token read. Journal on Node B starts 2026-11-10 19:15Z; newest Jev line 2026-11-12 20:11:58Z.
- Windows (status=200 only unless said):
  - **A** 11-12 13:00-15:20Z (2.33 h), busy normal paper, before the 15:26Z stall.
  - **B** 11-12 19:50-20:12Z (0.37 h), catch-up after the stall (log ends 20:12Z).
  - **C** 11-11 00:00-16:00Z (16 h), the earlier day, up to the credit outage.
  - **D** 11-12 00:00-15:20Z (15.3 h), for rates.
- O: Day totals: 11-11 has 22,290 call lines, 13,876 ok and **8,414 `status=402 failed`** (16:05:21Z to 22:38:14Z,
  "Insufficient credits"). 11-12 to 20:12Z has 9,808, all ok. No Jev line 15:30Z-19:47Z on 11-12 (the paper stall).
- O: The call line now carries `site=` and `bytes=`. Host calls say `site=front_importance`; **every guest call says `site=unknown`**.
- Scripts: scratchpad/jevrev/parse.py, an1.py ... an10.py (python3 -I, local only).

## 1. Measured usage

### 1.1 Volume, questions per call, bytes (O)

| | A (13:00-15:20) | B (19:50-20:12) | C (11-11 00-16Z) |
|---|---|---|---|
| paper calls (per h) | 1,629 (699) | 541 (1,502) | 11,540 (722) |
| paper questions (per h) | 5,649 (2,425) | 2,131 (5,915) | 40,656 (2,545) |
| questions per call | 3.47 | 3.94 | 3.52 |
| host front-importance calls (per h) | 135 (58) | 7 | 1,052 (66) |
| other computers | 0 | 9 | 19 total |
| classes | bulk 1,611, interactive 18 | bulk 541 | bulk 11,475, interactive 65 |

Questions per call, paper (calls, share):

| q | A | C |
|---|---|---|
| 1 | 180 (11%) | 1,416 (12%) |
| 2 | 532 (33%) | 3,781 (33%) |
| 3 | 119 (7%) | 710 (6%) |
| 4-8 | 725 (44%) | 5,078 (44%) |
| 9-12 | 64 (4%) | 481 (4%) |
| 13+ | 9 | 74 |

Host front importance: always exactly 12 questions, bytes 8.1-8.3 KB (all three windows).
Paper bytes per call (A): p10 3,022; p50 23,212; p90 28,398; p99 29,835; max 30,046. Sum 32.1 MB in 2.33 h.
Calls under 1.5 KB: 50; 1.5-5 KB: about 190; 12 KB and over: about 1,230 of 1,629.

Compared with 11-10 (our-usage.md, O by that page): paper calls an hour 836-867 then, 699 (A) and 552 (D) now;
questions per call 3.5 then, 3.47 now. **Calls per hour fell 16-35%, questions per call did not move.**

### 1.2 Concurrency (O, with the method stated)

The log has whole-second stamps, so "in flight" cannot be read directly. A serial stream has the property that
the sum of `upstream_ms` over any interval cannot exceed the interval. Tested on trailing windows:

- A: sum(upstream)/wall over 5 s windows: p50 0.03, p90 0.16, p99 0.66, **max 0.78**, none above 1.15. Duty 2.8%.
  Trailing 4-log-second window sum: max 3.89 s; 6 of 1,629 calls sit above 3.5 s (two streams briefly overlapping).
- B: max 0.78 per 5 s; one 1-second window sums to 3.4 s (catch-up, two streams).
- C: serial from about 01Z; hour 00Z has windows up to 8.0 s per 4 s (older guest code), hours 01Z-15Z mostly at 4.0-4.8,
  max 6.0 at 12Z. 11-12 from 03Z: max 3.5-4.3.
- Host front importance: 11-12 maximum 11 calls in one log second, but the **sum of upstream time in any second is at most
  1,015 ms and any 4-second trailing window 1.43 s**: a serial loop of ~100 ms calls, not a fan-out.
- Peak calls started in one log second: A 8, B 8, C 14 (the 11-10 figure was 39 per our-usage.md).
  Those are serial bursts at 6-9 calls a second. **Peak in flight: 1 for each stream; 2 at the rare overlap of two streams (I).**
- queue_ms: 0 in all windows (a few of 1,623 calls over both days carry up to 82 ms, none in A, B or C-ok).
- Upstream latency (A): p50 140, p90 178, p99 282, max 639 ms. No 429, no 5xx. 11-11 16:05Z-22:38Z all 402.

Conclusion (O + I): **the concurrent fan-out of 11-09/11-10 is gone**. Commits 17e67897f (judges serial), 8a7aac080 (intake: one
flusher, group commit), 2b780e257 (scorer one story a call, serial) landed 11-10 evening and the logs from 11-11 01Z show it.

### 1.3 What the calls are (shape inference; the log cannot name the guest site)

Shape classes for the paper in A (calls an hour, questions per call, MB an hour). All **I**:

| class (rule) | calls/h | q/call | MB/h | share of bytes | likely site |
|---|---|---|---|---|---|
| even q, 12 KB and over | 484 | 3.67 | 12.16 | 85% | intake labels (intake_labels.go:1114) |
| odd q, 12 KB and over | 53 | 4.15 | 0.91 | 6% | event layer or support judge (undecided, see 1.5) |
| 5-12 KB | 63 | 4.39 | 0.53 | 4% | support judge, sub-call, cast, story analysis |
| under 5 KB | 101 | 1.56 | 0.25 | 2% | router, classifier, disclosure, desk judge, inert, op-ed |

D (night to afternoon): intake-shaped 335 calls an hour; C 455 an hour. So **intake is about 64% of calls and 85% of bytes**.

### 1.4 The burst structure of intake (O shape, I meaning)

Consecutive intake-shaped calls within 2 s form a burst. Run-length encoding the question counts of a burst
(`NxQ` = N calls of Q questions), D 11-12 00:00-15:20, 996 bursts:

- `4x2` (four calls of 2 questions): 187 bursts; `3x2`: 92; `5x2`: 30; `1x2`: 139.
- `4x4`: 89; `3x4`: 22. `1x8`: 39. `18x4 2x12`: 39.
- In A: `4x2` 65 of 228 bursts, `4x4` 28.
- The run length depends on the hour: 5 at 00Z, 4 at 02-03Z, 3 at 06-09Z, 4 at 10-15Z (hourly mode of `Nx2` runs).
- In `4x2` bursts the median bytes per call position are 22,857; 23,028; 23,051; **16,617** (last smaller).

Meaning (I, from intake_labels.go:910-1010 and the constants at :77-95, :1185-1203): a capture is asked against the candidate
story window (8 h, ceiling 480 stories) cut into chunks of 15,000 bytes of stories; **one call per chunk per group of captures**,
questions event+topic = 2 per capture. So `4x2` is one capture against four chunks, `4x4` two captures against four chunks.
The chunk count follows the number of stories in the window, hence the hourly drift.

The 30,000-byte guard (`intakeRequestMaxBytes`, :74) is what limits captures per call, not the 6-capture cap:
of 586 four-question calls in A, 530 are 25 KB or larger; mean bytes by q: 2 -> 21.8 KB, 4 -> 27.3 KB, 6 -> 24.8-27.9 KB,
8 -> 22-25 KB (later chunks are smaller). Each extra capture adds about 5.5 KB (I: its two questions each list every
story handle of the chunk as criteria keys, plus two instruction texts), so a full chunk holds about 2-3 captures.
Average captures per call is q/2 = 1.8 against a cap of 6.

### 1.5 The owner's examples (O + I)

15:26:30Z `bytes=10596 q=6` and `bytes=10667 q=6` in the same second, then 15:26:34-35 four calls
of 15,234 / 15,561 / 15,884 / 15,716 bytes, q=3 each. These are the last minutes before the stall.

- Serial (O): upstream times 118, 111, 134, 117, 116, 136 ms; 0.73 s over about 6 s.
- The q=6 pair: 3 captures, ~10.6 KB, small because chunks later in the window are partial (I). Two of them at the same moment
  is one group of captures against two partial chunks, or two groups against one chunk (U).
- The four q=3 calls of ~15 KB, 0.5-1 s apart: odd q, 12-20 KB, 7-19 s apart in groups of 4-7. Intake gives odd q only when a capture has no topic
  question, so this family is probably another site (event layer chunks, or the support judge's pairs of one revision) (I).
  Three or four calls with the same q and bytes within 6% is a repeated pattern (A has 71 calls like it). The site cannot be
  decided from the gateway log (U). Passing the site (section 3) settles it.

### 1.6 Near-duplicate context, quantified (O metric, I reading)

Rule: same computer, bytes within 6%, within 2 s, over 5 KB. Cluster of two or more calls:

| | A | B | C |
|---|---|---|---|
| paper calls over 5 KB | 1,395 | 498 | 9,420 |
| in near-duplicate clusters | 917 (66%) | 326 (65%) | 6,438 (68%) |
| clusters | 177 | 73 | 914 |
| calls beyond the first of each cluster | 740 | 253 | 5,524 |
| MB beyond the first of each cluster | 19.7 of 32.1 | 6.8 of 11.7 | 145 of 215 |
| same-second pairs, bytes within 2% | 911 | 382 | 8,282 |

Reading (I): **most of these are not the same text sent twice.** They are the same captures against different slices of the story
list (one slice per call), and the cluster metric overstates literal duplication. What is literally repeated per call is the task
paragraph, the captures block, the corrections block if present (up to 12 examples, buildIntakeCall, intake_labels.go ~:749-757) and the question
instruction texts. A rough bound is 6-8 KB of a ~23 KB call (I, from the 15,000-byte story budget; not measured).
The per-capture waste is the multiplication of calls: 3-5 calls (about 100 KB sent) per group.

### 1.7 Cost (estimate; the log has no tokens or dollars)

- Price O (SYSTEM1.md section 7): $0.042 per million input tokens, output billed 0.
- Tokens per byte U. Two bounds: 0.22 (vendor-fit in SYSTEM1.md 3.3: 0.224 per state char; I) and 0.36 (the ratio implied by
  11-10's 6.8k tokens per call against today's 18.8 KB per call; I, weak).
- A (all computers, 14.25 MB/h): $0.13-0.22 an hour = **$3.2-5.2 a day at that rate**. D (10.35 MB/h): $2.3-3.8 a day.
  C (16 h): 14.0 MB/h, $3.1-5.1. B (catch-up): 32 MB/h, $7-12 a day rate.
- 11-10 measured $7.47 a day (our-usage.md 3.1, receipts). Today's volume in bytes is lower, so spend is lower, but this is an estimate.
  The receipts file would give exact tokens (numeric tail, as our-usage.md section 7), I did not read it.
- Intake is 85% of bytes, so about $2.7-4.4 a day-equivalent at A rates. The 402 calls (8,414) were not billed (I).

## 2. Call sites mapped to code

Doc says 14. The grep for the single guest chokepoint `judgeCall` (intake_labels.go:1219) and for the host door gives the same 14, plus the
phone route (api_system1.go:99, new since the doc's table, one question set per request, interactive). Per site, current lines.

| # | Site, call at | Context | Questions per call | Batched | Concurrent? | Trigger | Log rate (A) |
|---|---|---|---|---|---|---|---|
| 1 | intake labels, call intake_labels.go:1114, plan :910, serial loop :1004 | story chunk (15,000 B of stories) + captures + corrections | 2 per capture, up to 6 captures, 30,000 B guard | across captures of a flush, not across chunks | **No**: one flusher per runtime (:300-470), calls one after another | each poll with new captures; group commit, no timer | about 484/h (I) |
| 2 | event layer, event_layer.go:410 | candidate events + up to 12 captures, 24 KiB | 1 per capture | yes, chunk of 12 | No (prefetch deleted in 17e67897f) | 60 s flush (eventFlushInterval :38) | about 50/h (I) |
| 3 | support judge, support_judge.go:438 | one revision's pairs, 30,000 B split (:398) | 1 per pair | yes, per revision | No: slots = 1 (:64) | each Texture revision commit | some of 5-12 KB / odd q (I) |
| 4 | System 1 verb, system1_verb.go:286 | cell's refs, 16 per call | up to 12 | yes | No (serial in a cell) | desk calls | small (I) |
| 5 | story analysis, story_analysis.go:356 | one story | 12 per chunk, up to 24 -> 2 calls | chunks of 12 | No, serial loop | per analysed story | small (I) |
| 6 | prompt router, prompt_router.go:82 via askChoice :212 | owner text + destinations | 1 | no | No | prompt-bar submit | interactive 8/h |
| 7 | prompt classifier, prompt_class.go:58 | owner text + classes | 1 | no | No | prompt-bar input | interactive |
| 8 | disclosure gate, disclosure_gate.go:583 | egress request | 1 | no | No | per gated egress | small |
| 9 | sub-call judge, subcall_judge.go:566 (gate :608) | up to 20 children | up to 7 per child | yes | No: one gate | per fan-out | small |
| 10 | desk-cast judge, desk_cast_judge.go:489 | up to 20 casts | up to 7 each | yes | No: same gate | per round | small |
| 11 | inert judge, inert_judge.go:172 | up to 20 records | 1 each | yes | No: same gate | periodic | small |
| 12 | front importance, frontimportance/scorer.go:824 (askBatch), `maxStoriesPerCall = 1` :698 | one story | 12 (importance, promo, sections) | one story per call | **No**: serial, max 1,015 ms upstream in any second | 10-minute timer; 2-14 stories a run | 58/h (O) |
| 13 | op-ed gate, oped_gate.go:242 | an op-ed | 4 | yes | No | per op-ed | tiny |
| 14 | desk judge, deskjudge/choice.go:84 | evidence for one event | 1 | no | No | per event; reached from deskmcp/host.go | tiny |
| + | phone route, api_system1.go:99 | owner request | per request | caller | n/a | phone app | tiny |

Measurement hook (O, code): the gateway already reads header `X-Choir-Jev-Site` (jev_limiter.go:349, :368-371) and logs it
(`jevSiteFrom`). The guest client (gatewayruntime/jev.go:58-95) never sets it, so guests log `unknown`, though
`judgeCall` already has `meta.Site` (intake_labels.go:1219). Cheap fix: carry `meta.Site` in the context to `JudgmentClient.Decide`
and set the header (the header is sanitised and never sent upstream). About 15 lines over `callJudgeWithin`,
the `autoputer/run.go:1129-1135` adapter and the client. Also adding a flush id would show chunk fan-out exactly (header or log only).

## 3. The answer

**Parallel calls: no, not any more.** Peak in flight is 1 per stream (2 at rare overlap), peak 8 calls started in a second against 39 on 11-10.
queue_ms is 0. Every site is serial (section 2).

**Many calls over the same or overlapping context: yes, in one place, and it is most of the bill.** Intake labels (about 64% of calls, 85% of bytes):
- a capture group costs one call per story chunk, 3-5 chunks, serial, each 17-23 KB; the story text in each chunk is distinct, the rest is repeated;
- groups are small: 1.8 captures per call against a cap of 6, because the 30,000-byte guard binds at 2-3 captures per full chunk and because
  group commit asks the first capture alone;
- the group-commit design is correct for serial; the multiplier is chunks x groups.

| Item | Calls/h now (A) | With one call, many questions | What prevents merging | Size of change |
|---|---|---|---|---|
| Intake: captures of a burst into fewer groups | 484 | model, perfect packing at 6 per call inside the 2 s burst: 34% fewer calls (D), I; 40% at 10 s window, 50% at 30 s, 57% at 60 s | the 30,000 B guard (vendor 32,000-byte long-context line, SYSTEM1.md 3.3) binds at 2-3 captures when per-question criteria and instructions are 5.5 KB a capture; accuracy above 12 questions unmeasured (SYSTEM1.md rule 7) | small if only trimming per-question bytes; medium if a collect window |
| Intake: chunks per capture (3-5 calls) | 484 | 1 chunk: up to ~75% fewer intake calls and ~70% fewer bytes | 8 h window (commit 1e5fd14a3, ee2082f14: duplicates opened 40 min to 6.5 h later) needs 300-480 stories = 45-48 KB, over the 32 KB line | medium; needs a shadow comparison (accuracy) |
| Event layer vs intake | 50 | both ask a model whether a capture matches an existing thing; possibly one mechanism | U; two labelers over the same captures; not investigated | investigation |
| Front importance (1 story per call, 12 q) | 58 | 3 stories in 1 call: about 20/h | separate contexts; accuracy and anchoring unmeasured (commit 2b780e257 chose 1) | small, already built; 3% of bytes |
| Prompt router + classifier (same owner text, 2 calls) | about 8 | 1 call with 2 questions | different surfaces and different timing (prompt bar vs submit); latency | small, tiny gain |
| Story analysis (24 q -> 2 calls of 12) | small | 1 call of 24 | verb cap 12 (`yaegikernel.System1MaxQuestions`), accuracy above 12 unmeasured | small, tiny gain |

What was already fixed (git log since 11-01, all on main): c53cd09cf (one call per chunk for up to 4 captures; 8-q calls appear), 8a7aac080
(one serial flusher, group commit up to 6 captures), 17e67897f (event layer, support, sub-call, cast, inert, story analysis serial; pools deleted),
6030e6240/2b780e257 (scorer packed then returned to one story a call), 48b7b730c (log names site and bytes), 3c1cf9c9c (12 section questions in one call).
**Regressed:** nothing found in this window; no intake or event-layer commit since 11-10. **Never done:** reducing the chunk multiplication
(1e5fd14a3 grew the window from a count to 8 hours in chunks, which is where chunks per capture came from), passing the site from the guest,
and a check that the 30,000-byte guard (not the 6-capture cap) is the real limit.

## 4. Ranked fixes (no limits, no quotas; fix the caller)

Savings use the A rates unless stated; all calls-an-hour figures are from this log; byte and dollar figures use the 0.22-0.36 tokens per byte bounds.

1. **Pass the site (and a flush id) from guest to gateway log.** Saves 0 calls; makes every row above measurable instead of inferred.
   Class: orange (guest request path; additive header the gateway already reads). Check `scripts/check-guest-security-epoch` paths for
   the trailers. Rollback: revert. Risk: very low (sanitised, never sent upstream). Size: ~15 lines. Do first.
2. **Shrink the per-capture bytes of an intake question so more captures fit under the 30,000-byte guard.** Move the common "options are
   handles ..." text (event instruction ~900 characters, topic ~500) to `state.task` and keep one sentence per question; keep criteria keys.
   Expected: per capture 5.5 KB -> about 3.5-4 KB, full chunk 2-3 -> 3-4 captures per call, intake calls down ~10-20% (I; upper bound 34% by section 3).
   Class: orange (prompt text with behaviour, answers can move). Needs the shadow comparison (design doc section 9) because accuracy is unmeasured.
   Rollback: revert. Risk: label drift. Size: small.
3. **Cut chunks per capture from 3-5 to 1-2.** Shortlist like the event layer does (lexical overlap with the capture's title over the whole 8 h window,
   plus the newest N), so each call carries one chunk and many captures. Expected: intake calls ~484 -> ~120-160 an hour, bytes -60-70%
   ($1.6-3.1 a day-equivalent, I). Biggest lever; changes what Jev sees, so it needs the same shadow comparison and an owner view on the 8 h guarantee.
   Class: orange. Rollback: revert (restore chunking). Risk: missed same-event matches that sit in the dropped stories (this is what 1e5fd14a3 fixed).
4. **Fail fast and visibly on Jev 402 / real refusals.** 11-11 16:05-22:38Z: 8,414 failed calls (5,115 guest, 3,294 host, 5 other); the front scorer
   asked 100 stories a run for six hours; the guest kept its normal 600-900 calls an hour. Backing off after a provider refusal is allowed by
   CLAUDE.md. Not a limit; a circuit on a real refusal. Class: red (gateway/provider call path). Saves no money (402 not billed, I);
   saves log volume, guest time and shows the credit outage. Rollback: revert.
5. **Investigate whether event layer and intake labels are one job** (both: captures against existing stories/events). Read the guest
   `jev-calls` records for a day (outside this brief). Potential: ~50 calls an hour and the second labeler. Class: investigation.
6. **Front importance: pack stories after measuring** (maxStoriesPerCall already coded). 58 -> ~20 calls an hour; 3% of bytes. Low priority.
7. **Collect window for intake group commit (5-10 s).** Model says 40% fewer calls at 10 s (upper bound; with the byte guard maybe half that).
   Cost: arrival wake delayed by the window. **Contract conflict to settle first:** CLAUDE.md bans "spacing" on our own calls; a coalescing delay is batching
   not a rate limit, but the owner should say so. Also design doc section 9 rejected the timer. Rank last.

Reference page update owed with any of the above (CLAUDE.md, same-change rule): our-usage.md 2.4 and SYSTEM1.md 4.1/6.1 still say concurrent
(39 a second, scorer concurrent, 12 pools); that is stale. Rerun `scripts/proofs/jev-call-stats.sh` on a fresh pull and update it.

## 5. Side findings

- O: 402 outage 11-11 16:05-22:38Z (credits). Paper kept sending; the host scorer sent 100 failed calls a run.
- O: after the stall (19:50Z), catch-up ran at 1,502 calls and 32 MB an hour for 22 minutes, serial, no 429.
- O: interactive class is the owner's own prompt bar: 100 calls on 11-12, 95 on 11-11; q=1, ~1.5-2.2 KB, p50 about 90 ms.
- U: bytes per token (receipts not read). U: which site the odd-q 12-20 KB family is. U: arrivals per burst (guest flush lines not read).

## 6. Not done / limits

Guest records (`jev-calls/calls.jsonl`) and the guest flush log line (captures, calls, bytes_max) were not read: Node B was logs and metadata only.
They would turn every "I" in 1.3-1.5 into "O". Journal retention on Node B starts 11-10 19:15Z, so no other whole earlier day than 11-11 was available.
