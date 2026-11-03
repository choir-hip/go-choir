# Where a self-development round's 50 minutes go (2026-11-03)

Mutation class of this doc: green (evidence, read-only). Station:
[choir-selfdev-apps-feature-complete-2026-11-03](../definitions/choir-selfdev-apps-feature-complete-2026-11-03.md).
Owner prompt: "Self dev shouldn't be taking 50 minutes for these apps either.
Gotta review those traces and see what the holdup is."

Method (CLAUDE.md "Review the traces before attributing behavior"): `choir trace
export <trajectory>` on each arm, then the engineering loop's `run_events`
(`loop.progress` carries `provider_call_ms`, token counts and model; `tool.result`
carries `started_at`, `duration_ms`, tool name and arguments; `loop.progress`
phases `capsule_job_park*` and `loop.retry` reason `capsule_job_ended` carry job
waits), plus `lifecycle_events`, `choir self-dev list` and `lifecycle
capsule-evidence`. Metadata only: tool names, command lines, durations, sizes,
token counts. No engineer prose was used as evidence. The arms were not touched.
Analysis scripts are in the session scratchpad, not in the repo.

Rounds (engineering loop = assignment; all times Z):

| round | trajectory | assignment | engineering loop | outcome |
|---|---|---|---|---|
| claude r1 | b48525f9 | assignment-498e2829 | 03:15:50-04:02:40 | blocked at Freeze (mode off) |
| claude r2 | b48525f9 | assignment-87ef8525 | 04:11:01-04:35:09 | op selfdev-0c48dc23 judge-rejected |
| claude r3 | b48525f9 | assignment-74d314c1 | 04:42:16-05:30:01 | op selfdev-40faf6ee applied |
| deepseek r1 | 66e8a80a | assignment-eee45597 | 03:15:11-04:05:39 | blocked at Freeze (mode off) |
| deepseek r2 | 950f45ae | assignment-f0934b0d | 04:10:11-04:40:05 | op selfdev-35d61e7e passed judge, FP0 rejected |
| pool r1 | 5dc6a6ff | assignment-273d9f89 | 04:38:11-05:25:47 | op selfdev-1d052791 applied |
| luna r1 | 00338a51 | assignment-73be7adf | 04:34:38-04:34:59 | engineer ended "blocked" after 8 reads |

Every engineering loop on every arm ran DeepSeek V4.1 Flash (`hf:deepseek-ai/DeepSeek-V4.1-Flash-1017`,
provider synthetic, reasoning low), including the pool arm. Management ran
gpt-6-luna or DeepSeek. Luna ran gpt-6-luna.

## 1. Bucket table (minutes of the engineering loop)

Buckets are disjoint intervals of the loop: model call (`provider_call_ms`),
inline tool (`duration_ms` of `capsule_go_eval`), and the remainder, which is the
loop parked or passivated waiting on a started job (`choir.Poll` park or
`capsule_job_ended` wake). The tool-end to next-model-start residual outside job
waits is under 3 s per round (see 3.4), so the remainder is job wait.

| round | wall | model | job wait | delegate `wait` | inline tools | other |
|---|---|---|---|---|---|---|
| claude r1 | 46.8 | 25.8 (55%) | 12.4 | 7.7 | 0.8 | 0.1 |
| claude r2 | 24.1 | 15.6 (65%) | 7.9 | 0.0 | 0.6 | 0.0 |
| claude r3 | 47.8 | 33.9 (71%) | 13.1 | 0.0 | 0.7 | 0.1 |
| deepseek r1 | 50.5 | 36.7 (73%) | 12.8 | 0.0 | 0.9 | 0.1 |
| deepseek r2 | 29.9 | 16.5 (55%) | 12.7 | 0.0 | 0.6 | 0.1 |
| pool r1 | 47.6 | 39.1 (82%) | 7.7 | 0.0 | 0.7 | 0.1 |
| luna r1 | 0.4 | 0.3 | 0 | 0 | 0.1 | 0 |

Around the loop (all from lifecycle events and operation records):

| round | request/revision to engineering bound | engineer end to settled or verdict |
|---|---|---|
| claude r1 | 03:14:46 to 03:15:49.9 (63 s: Texture turn 55 s of model, Cast, bind) | blocked, Texture settled 04:05:35 |
| claude r2 | Texture revision 04:08:36 to 04:11:01 (2.4 min) | Freeze op opened 04:32:48, judge verdict 04:38:03 (5.25 min) |
| claude r3 | revision 04:39:56; Texture turn 04:38:36-04:41:58 (200 s of model), bound 04:42:16 | op 05:29:08 to applied 05:34:32 (5.4 min), Texture settled 05:35:47 |
| deepseek r1 | capsule created 03:15:10.74, spawned and granted 03:15:11.69 | blocked, settled 04:07:03 |
| deepseek r2 | revision 04:09:12 to bound 04:10:11 (59 s) | op 04:34:57 to judge pass 04:40:32 (5.6 min), FP0 reject 04:41:58 |
| pool r1 | 04:37:47 to bound 04:38:11 (24 s) | op 05:25:24 to applied 05:28:08 (2.7 min), settled 05:28:28 |

End to end (request or owner revision to Texture settled): claude r3 55.9 min,
pool r1 50.7 min, deepseek r2 31.9 min, claude r1 50.8 min (blocked). The engineering
loop is 85-93% of that; admission and spawn are not a cost (about 1 s); verify,
judge and apply add 2.7-5.6 min.

## 2. Model latency per call

| round | calls | median s | mean s | max s | total min | tokens in first / last | out total | cached share |
|---|---|---|---|---|---|---|---|---|
| claude r1 | 133 | 7.4 | 11.6 | 52.8 | 25.8 | 8.5k / 201k | 70.7k | 96% |
| claude r2 | 95 | 6.1 | 9.9 | 71.7 | 15.6 | 9.8k / 162k | 70.8k | 94% |
| claude r3 | 104 | 10.3 | 19.6 | 110.6 | 33.9 | 9.7k / 201k | 87.8k | 94% |
| deepseek r1 | 137 | 9.8 | 16.1 | 87.6 | 36.7 | 7.8k / 263k | 145.0k | 95% |
| deepseek r2 | 87 | 7.4 | 11.4 | 65.6 | 16.5 | 9.1k / 178k | 78.6k | 92% |
| pool r1 | 109 | 8.6 | 21.5 | 144.3 | 39.1 | 7.8k / 255k | 153.9k | 95% |
| luna r1 | 8 | 1.9 | 2.1 | 3.4 | 0.3 | 7.3k / 52k | 1.1k | 78% |

What drives call latency (least squares of `provider_call_ms` on output tokens
and input tokens, 665 calls over the six DeepSeek rounds):

- Output tokens: about 0.0105 s per output token (about 95 tokens per second),
  plus about 2.5-3.8 s for a call with under 150 output tokens (median per round).
- Input tokens: no measurable effect. The fit coefficient is -0.08 to +0.02 s per
  1k input tokens, and median latency by input bucket does not rise from 0-50k to
  150-250k at similar output sizes (3.4-9.8 s). Cache share stays 92-96%, so
  uncached input is 5-8k tokens per call.
- Calls with at least 2000 output tokens (file writes) are 18-56% of model time:
  claude r1 8 calls 4.6 min, r2 10 calls 6.2 min, r3 11 calls 9.4 min,
  deepseek r1 22 calls 15.3 min (87.6k tokens), deepseek r2 9 calls 6.3 min,
  pool r1 20 calls 22.0 min (107k tokens).
- Provider slow windows, measured as latency above 3 s + 10.5 ms per output
  token for calls more than 20 s over: claude r1 8 calls 4.1 min, claude r3 19 calls
  12.1 min (36% of its model time), deepseek r1 7 calls 3.6 min, pool r1 9 calls
  6.6 min, claude r2 and deepseek r2 none. The slow windows coincide across arms:
  03:10-03:30 (claude and deepseek both) and 04:40-05:10 (claude r3 and pool
  both, 335 and 259 s of excess in the 04:50 window).

Turn mix (model time by what the turn did next): deepseek r1 write-file turns 45
turns 18.1 min, read/search 52 turns 13.0 min, start-job 18 turns 3.4 min,
poll-job 18 turns 1.3 min. Pool r1 write 27 turns 21.5 min, read/search 40 turns
9.4 min, start 18 turns 3.9 min, poll 19 turns 3.6 min. Claude r1 read/search
50 turns 10.4 min, delegate poll 31 turns 4.7 min, start 19 turns 3.7 min.

Model turns per round: 133, 95, 104 (claude), 137, 87 (deepseek), 109 (pool).

## 3. Tool execution and waits

### 3.1 Inline tools
Every `capsule_go_eval` call returns in about 0.25-0.6 s (read, write, find,
grep, `choir.Start`, `choir.Poll`). Totals per round are 0.6-0.9 min across 78-137
calls. File reads, writes and searches are not a cost; they are a turn cost (3.2).

### 3.2 Jobs (`choir.Start`) are where the work runs
All builds, harnesses and screenshots run as capsule jobs; the loop parks
or passivates while a job runs. Job counts per round: claude 19 / 15 / 14,
deepseek 18 / 9, pool 18. Waits of at least 60 s: claude r1 5 waits 9.2 min
(max 135 s), claude r2 3 waits 3.9 min, claude r3 7 waits 12.0 min (max 149 s),
deepseek r1 5 waits 10.7 min (max 275 s), deepseek r2 4 waits 10.2 min (max 218 s),
pool r1 2 waits 5.3 min (max 161 s).

Deepseek r1 job list (start offset in minutes, loop wait in seconds): B+S
means build, serve, then the Playwright shot script; the jobs that wait are the
B+S runs at +30m 275 s, +38m 115 s, +41m 62 s, +44m 93 s, +48m 95 s.

### 3.3 Frontend build and npm
- Every build job begins `cd /workspace/platform/frontend && { d=${CHOIR_FRONTEND_DEPS...}; if cmp -s package.json $d/package.json && cmp -s package-lock.json ... then rm -rf node_modules && ln -s $d/node_modules node_modules; else npm ci --no-audit --no-fund; fi && npm run build`.
  No trace output shows an `npm ci` ("added N packages") in any of the six rounds:
  node_modules is a symlink to the prebuilt dependency tree, so npm is warm, not cold.
- `vite build` reports "built in" 7.0 s (claude r3, 05:15:11), 12.9 s (deepseek r1,
  03:56:25), 12.4 s (pool r1, 04:59:28). The full `npm run build` (source contract
  check, check-packages, vite build) plus copying dist took about 15-20 s
  (deepseek r1 first build job: still running at 14 s, done at +17 s).
- The same job then starts a static server and runs the Playwright shot script
  (`node /tmp/fn-shots.mjs` and similar). That part is the 60-275 s. The
  ratio build : shot run is about 1 : 5 to 1 : 15.
- Shot scripts contain many fixed `waitForTimeout` calls (script text only:
  14-61 s of summed constants per round across all script versions). Which part of
  a 90-275 s run is fixed waits versus page load versus seeding is not in the
  trace (HYPOTHESIS, 3.5).

### 3.4 Idle gaps between a tool result and the next model call
Tool-end to next-model-start outside job waits, summed over a round: under 3 s
(claude r1 has 747 s of uncovered time, of which 552 s is five job parks of
89-135 s and the rest is shorter job waits; the median gap is 0.0 s). Wake latency from a `capsule_job_ended` retry or park end
to the next model call start is 0.01-0.03 s (medians, 13 / 8 / 18 / 15 wake
events). No orchestration overhead and no retry cost is visible; `loop.retry`
events (claude r2: 13 `capsule_job_ended`, 2 `capsule_job_outcome_undelivered`;
deepseek r2: 8 and 1) are job-end wakes, not failures.

### 3.5 Repeated work
- Full shot run per iteration: deepseek r1 had 10 jobs containing the build, 9 of them
  full build+serve+shots runs; pool r1 7 versions of `/tmp/harness/shots.mjs` and 7 of
  App.svelte; claude r3 14 versions of `/tmp/fx/shots.mjs` and 8 of
  `/tmp/fx/verify.mjs`; claude r2 8 versions of `/tmp/fnh/vite.config.mjs` and a
  go build of `./cmd/autoputer` (85 s, 25-minute allowance) before Freeze
  (also deepseek r2, 145 s). Literal-path WriteFile counts only; whether a write
  is a whole-file rewrite is HYPOTHESIS.
- Every round built its own harness under /tmp (`/tmp/fnh`, `/tmp/fx`,
  `/tmp/harness`, `/tmp/fn-*`): fixture generators (go, python, node), a static or
  mock server, and a Playwright script. Deepseek r1's first write was at +12.0 min after 42
  calls; claude r1 +8.5 min after 27 calls; pool r1 +2.0 min after 19 calls; claude r2 / r3
  +3.5 min after 28 / 29 calls.

### 3.6 Spec hunting (stale source tree)
Round 1 on claude and deepseek: the spec was not in the tree. Claude r1 ran
ListDir of the repo and docs, then `grep -rl 'Files (reinvented)'` at +2.5 min,
then read the design bar at +3.2 min. Deepseek r1 did ListDir, a `find ... -name
'apps-feature-complete*'`, a git log probe and a grep for a preset name, and read the bar at +3.3
min. About 7 search calls, roughly 3 min of the loop (about 6%), no spec found.
Pool r1 (source.projection on) read the spec at +0.3 min; claude r3 at +1.2 min.
Claude r2 and deepseek r2 were carried-source revisions with no hunt.
Not a main cause.

### 3.7 The Claude delegate (claude arm)
- r1: `choir-delegate start` at +8.5 min (03:24:21); done 03:40:36, 974 s
  (16.2 min), 94 assistant messages, 57 tool uses, exit 0. The engineer
  made 31 delegate tool calls. `wait --seconds 150` and `--seconds 24` calls
  returned after 20-25 s each (the call is capped near 25 s), so 31 polls: 7.7 min
  of blocked `wait` time plus 31 poll turns (4.7 min of model time) = 12.4 min,
  26% of the round, and 16.3 min window of which the engineer
  did 17 other tool calls (fixtures, server, shot script) and 47 model calls.
  After the delegate finished at +24.8 min the engineer still needed 22 min.
- r2: delegate 04:22:39-04:23:05, 26 s. r3: delegate started +4.2 min with
  max_turns 30; the engineer only probed it (5 s waits).
- So the delegate did not shorten the critical path in r1: the long pole after it was
  build, shots and fixes, with no Freeze possible.

## 4. Findings and hypotheses

1. FINDING: model generation is 55-82% of an engineering round (15.6-39.1 min).
   It is output-token bound (about 95 tokens/s), not context bound. 18-56% of
   model time is 8-22 file-write calls of at least 2000 output tokens. Input
   tokens grow 8k to 160-263k per call with no latency effect (cache 92-96%).
2. FINDING: slow provider windows added 3.6-12.1 min in four of six rounds and hit
   concurrent arms in the same windows (03:10-03:30; 04:40-05:10).
   HYPOTHESIS: Synthetic queueing under concurrent DeepSeek load from several arms.
3. FINDING: job waits are 7.7-13.1 min per round (16-42% of wall), dominated by
   2-7 full build+serve+Playwright runs of 60-275 s. Build itself is 15-20 s
   and node_modules is a warm symlink, never `npm ci`.
   HYPOTHESIS: most of a shot run is fixed waits and serial per-preset page loads.
4. FINDING: no admission, spawn or wake overhead: spawn about 1 s, tool-to-model
   gap 0.0 s median, wake 0.01-0.03 s.
5. FINDING: delegate polling cost the claude r1 engineer 12.4 of 46.8 min
   (31 poll turns plus blocked `wait` calls capped near 25 s).
6. FINDING: spec hunting cost about 3 min in r1 on two arms only.
7. HYPOTHESIS: whole-file rewrites (App.svelte 7x, shots.mjs 7x and 14x) inflate output
   tokens; needs the write payloads to confirm.

## 5. Top three causes and the smallest change for each

1. Output-token generation time (finding 1, 2): pool r1 39.1 min of 47.6, 107k
   output tokens in 20 write calls; deepseek r1 98k tokens in 45 write turns.
   Smallest change: give `capsule_go_eval` a patch primitive (replace a range or
   string in a file) and tell the engineer to edit, not re-emit, a file it has
   already written; add the output-token count of each write to the round record
   so rewrites are visible.
2. Full screenshot-harness runs after every edit (finding 3): 2-7 runs of
   60-275 s, 5.3-12.0 min per round (deepseek r2 10.2 of 29.9). Smallest change:
   ship one platform shot harness (build once, serve, `--only <shot>` or
   `--preset`), so an edit re-shoots one preset, not all twelve; and run the
   full set once before Freeze.
3. Waiting by polling (finding 5, 4): claude r1 31 delegate poll turns and 7.7
   min of blocked `wait`; every job poll costs a model turn (poll turns 1.3-3.6
   min per round). Smallest change: make `choir-delegate wait` park the loop the way
   `choir.Poll` parks (deadline plus wake on exit; wake latency is 0.01-0.03 s)
   instead of a 25 s blocking call; and have `choir.Start` park until the job
   ends by default.

Not causes: npm install (warm), the vite build (7-13 s), capsule admission and
spawn (about 1 s), orchestration gaps, tool I/O, context growth, spec hunting
(3 min, r1 only).

## 6. Reproduce

`choir trace export --api-key-file <key> --computer <id> --out <file> <trajectory>`
for b48525f9 (claude), 66e8a80a and 950f45ae (deepseek), 5dc6a6ff (pool),
00338a51 (luna); `choir self-dev list` for operation times. Event kinds used:
`loop.progress` (`provider_call_ms`, `input_tokens`, `output_tokens`,
`cached_input_tokens`, `phase`), `tool.result` (`started_at`, `duration_ms`),
`loop.retry` (`capsule_job_ended`), and `lifecycle_events`.
