# Cut 0 measurements (artifact 7)

Written 2026-10-13 for `docs/archive/definitions/choir-reeng-cut0-eyes-2026-10-12.md`
artifact (7). Sources: two kept trace bundles (the owner's data, outside the
repo; only counts, timings and ids appear here), the tracer receipts and log
for `c0-accept-1`, `scripts/reeng_invariants.mjs`, and GitHub Actions job times.
Only two bundles exist, not three: `c0-tracer-dev-8` (build e6995e08,
trajectory 6ba1a234) and `c0-accept-1` (build b9f195b1, trajectory d68f69cf).
`c0-tracer-dev-2` kept a drill file but no bundle. Both runs used the old
tracer defaults (12 min settle, 10 min quiescence); the fast defaults landed
afterwards in 19419ee0.

## 1. Push to a healthy fresh test computer

CI is one workflow; the deploy job starts when the test gate closes. Times
are UTC, from job start and end stamps.

| commit | CI tests (push to gate) | deploy job | push to deployed | notes |
|---|---|---|---|---|
| 2606c7b5 | 5 m 32 s | 2 m 51 s | 8 m 26 s | slowest race shard 5 m 27 s; SBOM acceptance runs after, total 9 m 57 s |
| e6995e08 (dev-8 build) | 6 m 02 s | 2 m 10 s | 8 m 14 s | |
| de4ebea2 | 4 m 39 s | 3 m 59 s | 8 m 40 s | |
| b9f195b1 (accept-1 build) | 1 m 01 s (tests skipped) | 7 m 00 s | 8 m 03 s | prebuild deploy artifacts took 5 m 27 s |
| 287734c5 | not finished | not finished | not finished | run 38349164843 still on its test shards (4 m) when read; deploy not started |

Push to deployed is 8 to 9 minutes whether or not tests run: when tests are
skipped the deploy job absorbs the time (artifact prebuild).

| mint phase | c0-tracer-dev-8 | c0-accept-1 |
|---|---|---|
| register owner | 0.19 s | 0.13 s |
| prompt bar visible | 11.4 s | 303.9 s |
| computer answered | 11.4 s | 573.4 s |
| scoped key minted (mint total) | 12.4 s | 574.2 s |
| first prompt accepted after mint | 0.9 s | 1.0 s |

So push to a healthy computer that accepts a prompt is about 8.5 to 9.5
minutes with free capacity (deploy 8 to 9 min, mint 12 s, prompt 1 s). On
accept-1 the mint started 03:30:59Z and the computer answered 03:40:33Z, four
seconds after the node reported the deployed commit (03:40:29Z). That wait
overlapped the b9f195b1 deploy job (03:32:44Z to 03:39:44Z) and the memory
condition in `docs/archive/problems/new-owner-waits-while-qa-computers-hold-node-b-memory-2026-10-13.md`;
these bundles cannot tell the two apart. dev-8 started 25 minutes after its
deploy finished, so its 12.4 s is a mint-only figure, not a push-to-ready one.

## 2. Per-turn latency and tokens

A model turn is one `loop.progress` event with `phase=provider_call_started`
paired with the next `loop.progress` of the same loop and iteration that
carries `stop_reason`, `input_tokens` and `output_tokens`. Latency is the
difference of the two event stamps (includes about 30 to 70 ms of event
persistence). Tokens are per turn, not cumulative. No cached-token field is
recorded. Latency in seconds; tokens as median / total.

| bundle, desk (model, effort) | turns | latency median / p90 / max | input tokens | output tokens |
|---|---|---|---|---|
| dev-8 texture (deepseek-v4.1-flash, high) | 36 | 6.0 / 46.1 / 110.5 | 19,295 / 769,103 | 732 / 70,838 |
| dev-8 research (gpt-6-luna, low) | 82 | 2.5 / 8.0 / 27.1 | 44,980 / 4,267,211 | 162 / 20,100 |
| accept-1 texture (deepseek-v4.1-flash, high) | 53 | 7.8 / 43.8 / 102.6 | 28,593 / 1,643,163 | 1,135 / 109,721 |
| accept-1 research (deepseek-v4.1-flash, low) | 78 | 6.7 / 18.2 / 62.9 | 30,069 / 3,153,111 | 738 / 92,165 |

- Management: no management run appears in either bundle, so there are no
  management turns. The conductor run made no provider call (route
  `server_content_reference`, done in about 0.1 s).
- Texture ended every turn with `tool_use` (never `end_turn`); research
  ended 4 of 82 (dev-8) and 2 of 78 (accept-1) turns with `end_turn`.
- 2 (dev-8) and 1 (accept-1) provider calls have a start and no completion:
  runs passivated at the restart or still open at export.
- Input per turn grows with the loop (research 4.7k to 102k tokens): each turn
  resends the whole message list.
- RunRecord tokens agree with the event sums for research (dev-8
  4,267,211) but not for texture (RunRecord 531,586 and 884,440 against
  event sums 769,103 and 1,643,163), and the interrupted runs have null
  tokens in the RunRecord. Cause not investigated; read the events.
- Research wall time per run: 370, 330, 98, 105 s (dev-8); 575, 663 s
  completed plus one passivated (accept-1).

## 3. Desk cell behaviour (baseline for Cut 1 prompts)

From `desk.cell` events. Durations in milliseconds; research p90 is
dominated by cells that run eight concurrent searches.

| bundle, desk | cells | cells with error | duration median / p90 / max |
|---|---|---|---|
| dev-8 texture | 36 | 7 (19%) | 1 / 2 / 9 ms |
| dev-8 research | 90 | 14 (16%) | 1 ms / 38.8 s / 85.5 s |
| accept-1 texture | 53 | 11 (21%) | 1 / 2 / 51 ms |
| accept-1 research | 75 | 19 (25%) | 5 ms / 38.1 s / 72.3 s |

Error classes by `diag` and message pattern (a cell can match two):

| class | dev-8 texture | dev-8 research | accept-1 texture | accept-1 research |
|---|---|---|---|---|
| undefined name (across cells) | 4 | 0 | 4 | 7 |
| expected declaration | 0 | 5 | 1 | 6 |
| refused import (incl. import preflight) | 0 | 4 | 0 | 2 |
| type or argument mismatch | 0 | 1 | 2 | 1 |
| syntax error | 0 | 1 | 0 | 0 |
| compile panic (`diag=panic`, cell marked unsafe_to_reuse) | 1 | 1 | 2 | 1 |
| other compile error | 2 | 3 | 2 | 2 |

- About one cell in five fails; the two largest classes are names that an
  earlier cell defined (undefined name) and statements outside a function
  body (expected declaration). Both are session-state mistakes, not search
  failures.
- Research issued 107 and 109 `web_search` calls (median 1.8 s, p90 14 s, max
  20 to 24 s; 11 and 5 errors), and 41 and 36 `import_url_content` calls (8
  and 4 errors).
- Texture `tool.result` marks only 3 (dev-8) and 7 (accept-1) of its cells as
  `is_error`, and research marks 0, so most compile errors reach the model
  as ordinary output. The `desk.cell.error` field is the count to use.
- No cell waited for a busy desk (`waited_ms` is 0 in all 254 cells).

## 4. Run-acceptance baseline

What the tracer story reached on both computers: a cited roughly 300-word
story written by the Texture desk through 4 (dev-8) and 5 (accept-1) desk
revisions, plus 3 owner revisions (prompt, the load follow-up, and the
owner asking again after the restart). Research ran 3 to 4 times. Open work
items at export: 4 and 5, none with a stated reason; 5 to 6 runs passivated
and 1 run still `running` at export.

Restart fate (identical on both): the tracer restarted the computer
through the product API with a run in flight (dev-8: a texture run;
accept-1: a research run). The host classed it `crash_or_stop`, the run was
`passivated` (event `activation.passivated`, recovery
`passivated_on_restart`), and nothing resumed. After 60 s of quiet the tracer
had the owner ask again; Texture then revised again (107.6 s later on dev-8,
460.6 s on accept-1). Publication happened only by owner act (the tracer's
publish call, status 201, accept-1 only; dev-8 did not publish, so its
publications layer is empty).

| rule | dev-8 | accept-1 |
|---|---|---|
| I1 Texture sole writer | pass (7 revisions) | pass (8 revisions) |
| I2 no open obligation without reason | fail (1 of 11 runs open, 4 work items) | fail (1 of 10 runs open, 4 work items) |
| I3 no dispatch after markerless boot | unknown (4 post-boot dispatches with unrecorded cause) | fail (1 dispatch after markerless boot) |
| I4 citations carry source and quote | fail (4 of 4) | fail (5 of 5) |
| I5 request ids / one worker per desk | fail (14 overlaps of workers or cells on one desk session; 184 egress results each carry their own request id) | unknown (184 egress ok; 128 cells one worker one cell at a time; load never appeared) |
| I6 | not yet defined | not yet defined |

dev-8 ran on e6995e08, before 668bf32e (research children name their own
desk session in trace events); its 14 "overlaps" may be children attributed
to the parent desk session (a hypothesis, not checked). accept-1 is the
post-fix reading. I2 and I3 are the known restart defect (Cut 1), I4 is
Cut 2. Level reached today: the story completes and publishes only when the
owner drives it, and a restart ends the work without a visible fate.

## 5. Wall time of one tracer attempt

| phase | dev-8 | accept-1 | who waits |
|---|---|---|---|
| mint | 12.4 s | 574.2 s | platform (vmctl boot) |
| submit to first revision | 47.5 s | 31.8 s | desk (first Texture turn) |
| submit to load settled (includes the row above) | 519.8 s | 794.2 s | desk (research runs of 330 to 663 s), plus the tracer's settle bound |
| restart call to ready | 9.1 s | 9.2 s | platform |
| ready to next revision | 168.4 s | 521.0 s | 60 s harness quiet window, then desk |
| quiescence wait | 600 s | 600 s | harness (never quiescent; 1 run open both times) |
| export and checker | 2.8 s | 2.5 s | harness |
| total, tracer start to bundle kept | 21 m 59 s | 41 m 48 s | |

The owner's pace ruling (metamission `boundaries.pace`, 2026-10-13) sets a
loop of push, deploy (about 9 minutes), one tracer run of 10 minutes or less,
read the trace, fix. Neither measured attempt fits: both spent 10 minutes
in a quiescence wait that never ended (now bounded at 2 minutes), and up to
12 minutes settling the load (now 3 minutes). Of what remains, mint (12 s) and
restart (9 s) are platform, the 60 s quiet window and the 2-minute
quiescence bound are harness waiting that the tracer can shorten, and the
rest is desk latency: first Texture revision (30 to 50 s), research runs
(330 to 660 s wall, which no tracer bound shortens), and Texture's
post-restart revision (108 to 460 s). On the dev-8 timings, new defaults
would give about 12 s + 180 s + 9 s + 170 s + 120 s + 3 s, roughly 8 minutes
(an estimate, not a run); on the accept-1 timings the post-restart revision
alone would take it past 10 minutes.

### First fast-default run (c0-fast-1, build 287734c5)

One attempt with the fast defaults, tracer start 05:07:42Z to bundle kept
05:19:55Z: **12 m 13 s**. Mint 12.4 s (Node B memory free after the finished
test computers were stopped), first prompt accepted 0.9 s after mint, first
revision 12.8 s after submit, load settled at the 3-minute bound (212.5 s),
restart to ready 8.8 s, ready to next revision 372.6 s, quiescence bound
2 minutes (two runs still open), export 2.0 s. The ready-to-revision figure
is inflated: a deploy of b1be90b7 started 27 s before the drill and staging
answered 502 for about six minutes, so the owner's ask-again got a 502. A
deploy can follow a docs push when an earlier scripts push's CI run was
cancelled and folded into it; the tracer should start only once no CI run
on main is queued or deploying. Checker: I1 pass, I2 fail, I3 fail (ten
post-boot dispatches; the first two are Cut 1 breakage B1,
docs/archive/problems/reeng-cut1-breakage-2026-10-13.md), I4 fail, I5 unknown (no
parallel egress in one cell this time).

Export refusal across test computers: with the key of a different (stopped)
test computer and the live computer's id in `X-Choir-Computer`, every route
answered 502 "failed to resolve user autoputer" and returned no data; the key
resolves only its own computer, so the header cannot redirect it. The CLI
export wrote a bundle with every layer `unserved` and exited 0 (it reports,
it does not fail).
