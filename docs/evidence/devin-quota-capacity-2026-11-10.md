# Devin swe-2-flash quota: what the gateway log shows, 2026-11-10

Class: green (read-only evidence; no product code). Rollback: revert.
Method: `ssh node-b` journalctl of `go-choir-gateway`, filtered and aggregated on the host (counts only, no raw
logs, no secrets). Journal window 2026-11-08 23:05Z to 2026-11-10 ~11:50Z, but the first Devin call is at
2026-11-10 00:xxZ (d@e) and the paper's first is in hour 01, so **only 12 hours of Devin data exist (00Z-11Z today),
not 48**. Counts come from `inference succeeded ... provider=devin tokens=IN+OUT cached=C` lines; VM from the id
prefix (paper 1669020a, d@e 70070cf7).

## Summary in plain language

1. The gateway never learns how much Devin quota is left. It finds out only when Devin answers 429
   `resource_exhausted`, and then only a reset time (read from Retry-After, trailer metadata, retry-delay details,
   or an "in N minutes" phrase; no limit or remaining number is parsed anywhere in `internal/provider/devin.go`).
   So every thread runs flat out until the account is empty, and then every thread falls over at once.
2. Three trips today (07:49, 08:23, 11:26Z), each naming a reset exactly on the hour (08:00, 09:00, 12:00). The
   reset is a fixed clock hour, not a rolling window: at 08:00 Devin served 40 calls in the first minute.
3. In hour 11 the gateway had served 43.5M input tokens / 1,562 calls when it tripped. Hour 10 completed clean at
   42.7M input tokens / 2,001 calls. So the ceiling, if it is a token count on the log's `IN` field, is **between
   42.7M and about 43.5M per clock hour plus whatever the owner used locally in hour 11**. A two-point fit, the only
   tight one.
4. The 07:49 and 08:23 trips happened at 18.6M and 13.1M gateway input tokens, far below that ceiling. Either the
   owner's local CLI/review runs consumed the other ~25-30M of those hours (reviews ran 05:00-09:30Z), or the unit is
   not the log's `IN` (cost-weighted, for example). The gateway log cannot tell these apart.
5. The 11:26 trip was caused mostly by d@e: in hour 11 d@e used 26.1M of 43.7M input tokens (60%; 807 calls at ~32k
   average prompt) against the paper's 17.6M, below the paper's usual 20-27M. The paper alone does not exhaust the
   hour; paper + a 30-story proof run + three local reviews does.
6. Exhaustion cost is not dollars: today's overflow (2,250 calls, 55M input tokens, mostly cache reads, on Bedrock
   Haiku) is about $3 at list price. The cost is the model switch (different model, prompt cache restarted) and one
   13-minute outage at 09:00-09:13 after the account had already reset (below).

## Per-hour table (UTC, 2026-11-10)

IN is the log's input-token figure, C the cached figure, OUT output; M = millions. "Trip" is the first
`provider quota exhausted (trip N ...)` line and the reset it names (all three were Devin-named, not estimated).

| Hour | Paper calls | Paper IN | Paper C | Paper OUT | d@e calls | d@e IN | d@e OUT | Total calls | Total IN | Trip (minute, reset) |
|---|---|---|---|---|---|---|---|---|---|---|
| 00 | 0 | 0 | 0 | 0 | 9 | 0.19M | 0.00M | 9 | 0.19M | none |
| 01 | 979 | 19.21M | 3.24M | 0.50M | 160 | 3.52M | 0.10M | 1,139 | 22.73M | none |
| 02 | 985 | 19.87M | 4.28M | 0.50M | 76 | 1.86M | 0.06M | 1,061 | 21.73M | none |
| 03 | 1,442 | 27.71M | 8.21M | 0.74M | 99 | 2.53M | 0.09M | 1,541 | 30.24M | none |
| 04 | 1,264 | 22.89M | 8.92M | 0.63M | 68 | 1.67M | 0.08M | 1,332 | 24.56M | none |
| 05 | 1,080 | 24.87M | 9.45M | 0.51M | 81 | 1.55M | 0.09M | 1,161 | 26.42M | none |
| 06 | 1,294 | 22.20M | 9.24M | 0.57M | 188 | 4.26M | 0.22M | 1,482 | 26.46M | none |
| 07 | 535 | 14.69M | 2.24M | 0.26M | 146 | 3.92M | 0.16M | 681 | 18.61M | 07:49:21, reset 08:00:00 (681 calls, 18.6M IN, 3.6M C, 0.43M OUT at trip) |
| 08 | 621 | 12.15M | 3.65M | 0.27M | 54 | 1.06M | 0.05M | 675 | 13.20M | 08:23:41, reset 09:00:00 (670 calls, 13.1M IN at trip) |
| 09 | 768 | 13.20M | 4.23M | 0.34M | 10 | 0.11M | 0.00M | 778 | 13.31M | none; hold until 09:13 (below) |
| 10 | 1,637 | 35.45M | 9.64M | 0.84M | 354 | 7.22M | 0.34M | 2,001 | 42.70M | none (clean) |
| 11 | 749 | 17.59M | 1.47M | 0.39M | 807 | 26.06M | 0.46M | 1,569 | 43.73M | 11:26:26, reset 12:00:00 (1,562 calls, 43.5M IN, 5.2M C, 0.84M OUT at trip) |

Other VMs: 20 calls, 0.1M tokens in hours 10-11. Sixteen Devin success lines in hours 04-06 and 10 did not parse
(different VM-id shape) and are excluded; under 1% of calls. The paper made no Devin calls before hour 01.

Gaps: the paper made no Devin requests 07:40-07:48 (cause unknown; not a quota event). While a trip is open only
the circuit probes reach the gateway (1-4 a minute), because guests skip a held model.

## The fit

| Hour | Outcome | Calls | IN | IN+C | OUT |
|---|---|---|---|---|---|
| 10 | clean, full hour | 2,001 | 42.70M | 53.7M | 1.19M |
| 11 | tripped at minute 26 | 1,562 | 43.54M | 48.7M | 0.84M |
| 07 | tripped at minute 49 | 681 | 18.61M | 22.2M | 0.43M |
| 08 | tripped at minute 23 | 670 | 13.06M | 16.9M | 0.30M |
| 03, 05, 06 | clean | 1,161-1,541 | 26-30M | | |

- Calls do not predict it (2,001 clean vs 1,562 tripped). Output does not (1.19M clean vs 0.84M tripped).
- `IN` is consistent for hours 10 and 11 (clean at 42.70M, tripped at 43.54M). `IN+C` is not (clean at 53.7M,
  tripped at 48.7M). Hour 11 also had the owner's three local reviews (10:57, 11:04, 11:16Z), so the true account
  ceiling is above 43.5M gateway IN; the gateway-only share is bracketed 42.7M..~43.5M+.
- Hours 07 and 08 tripped at under half that. They overlap the owner's local review runs (05:00-09:30Z), which are
  absent from the gateway log. Simplest reading: shared account quota (as the owner saw, the local CLI reported its
  limits resetting in 20 minutes). Not provable from this log. Hours 05 and 06 also overlapped those runs and stayed
  clean at 26M, so local use was not uniform, or the ceiling is not simply IN.
- Warning on the log: for Devin the `cached=C` figure is not a subset of `IN`. In 2,498 of 13,442 Devin calls today
  (18.6%) C exceeds IN (for example `392+5558 cached=30464`). The `spend2.sh` assumption (IN = whole prompt, C a
  subset) comes from the Bedrock path and does not hold for Devin; the provider's usage message gives separate
  counts (`internal/provider/devin.go`, usage parse ~1305-1440). Whether Devin meters IN, IN+C or a weighted mix is
  unknown. IN alone is the best fit on two points.
- Window shape: fixed clock hour. Three named resets all on the hour; 40 successful calls in the first minute of
  08:00. A rolling window would not reopen the account at 08:00 after a 07:49 exhaustion.

## What the gateway knows at the moment of each call

Nothing about remaining quota. Per call it knows the model and the VM; the token counts only after the call.
It records a trip, the reset time when Devin names one, and a gateway-made cooldown when not
(`internal/provider/devin.go` 294-375, `internal/provider/quota.go`, `internal/gateway/circuit_breaker.go`):

- 429 `resource_exhausted` is quota unless a retry time of 10 seconds or less is given (`devinShortRetry`), then it
  is a concurrency retry.
- Reset time is read from Retry-After, trailer metadata, error-detail retry delay, or "in N minutes" text. Devin
  sent on-the-hour times today. The "(estimated)" label is the gateway's own 15-second probe wait
  (`quotaProbeWait`) shown to calls refused while the half-open probe is in flight; it is not a Devin figure. With
  no reset time at all the breaker uses a 5-minute doubling cooldown.
- No limit, remaining, percentage or credits field is parsed from any header, trailer or body.

## What happens on exhaustion today

- One 429 trips the model's breaker until the named reset (bounded at 1 hour). Log line: `model devin/swe-2-flash
  provider quota exhausted (trip 1, half-opens at ..., reset expected ...)`.
- While open, a call that reaches the gateway gets a typed refusal (`provider quota exhausted (devin); resets at ...
  (circuit open)`); guests read it and route the pool to the next member. After the first seconds almost no Devin
  requests arrive (1-4 a minute). So there is no per-call try-Devin-first: all threads fall over together.
- Overflow goes to Bedrock Haiku. Measured today: 07:49-08:00 316 calls, 6.3M IN (4.9M cached), 0.23M OUT;
  08:23-09:13 1,315 calls, 31.2M IN (24.2M cached), 1.16M OUT; 11:26-11:46 620 calls, 18.0M IN (13.3M cached),
  0.96M OUT. Priced with the `spend2.sh` Bedrock table, uncached input treated as 5-minute cache write: about $0.34,
  $1.69, $1.21, about $3.2 in all (assumed split; CloudWatch not consulted).
- Unexplained and costly: after the 09:00 reset the model stayed held until 09:13. In 09:00-09:12 the gateway logged
  only "circuit open ... resets at HH:MM:15 (estimated)" refusals (2-4 a minute); a probe-like request at 09:11:40,
  another at 09:12:08 failed `stream error ... INTERNAL_ERROR`; calls flowed from 09:13. No upstream 429 is logged in
  that window. Hypothesis (not confirmed): the single half-open probe was slow or hung for ~12 minutes and held the
  probe slot while every other call was told "estimated 15s". The 08:00 reset recovered at once, so it is not
  systematic. 13 minutes of an hour-long window lost while the account was full.

## Who used the quota in exhausted hours

The gateway line carries VM and `system_chars` but no role, desk or run label, so a desk split cannot be made from
the log. By VM:

- Hour 07 trip: 535 paper + 146 d@e calls; hour 08 trip: 621 + 54. The rest of those hours' quota is not in the
  gateway log (hypothesis: owner local CLI/review runs).
- Hour 11 trip: d@e 26.06M IN (807 calls, ~32k per prompt; 965 d@e Devin requests between 10:50 and 11:30 as the
  ~30-story run ran), paper 17.59M (749 calls). d@e is 60% of the hour's input.
- Prompt sizes of all 13,442 Devin calls today (IN field): 8,011 under 20k; 4,141 at 20-50k; 1,267 at 50-100k; 23 at
  100-150k; none beyond. The 1,290 calls over 50k (9.6%) carry about 29% of all IN (83.6M of ~284M). That is the
  lever for option (c).

## Measured, estimated, unknown

Measured (log): calls and IN/C/OUT per hour and VM; three trips with minute and reset; on-the-hour resets;
immediate recovery at 08:00; the 09:00-09:13 hold; overflow volumes; C > IN in 18.6% of calls; gateway code
behaviour.
Estimated: ceiling 42.7M-43.5M gateway IN per clock hour (two points; local use in hour 11 not accounted); overflow
dollars (assumed cache-write split).
Unknown: what unit Devin meters (IN, IN+C, weighted, per request); whether the limit is per hour or a credits pool
that resets hourly; how much of hours 07-08 was local CLI use; the role split; why the 09:00 hold lasted 13 minutes;
why the paper made no Devin calls 07:40-07:48; whether Devin sells another seat (not looked up, not asserted).

## Design options, cheapest physics first

The physical bound: about 43M input tokens (roughly 2,000 paper-sized calls) per clock hour, shared with the
owner's local CLI, against 22-30M for the paper alone in a normal hour and 43M+ when d@e runs a proof. This is a
provider quota, not a limit imposed on agents; the aim is to spend it evenly and spend all of it.

**(0) Fix the 09:00-09:13 hold first.** Needs: why the half-open probe took ~12 minutes (probe duration, deadline,
one slot gating all). Smallest change: bound the half-open probe (short deadline, or admit several probes) so a hung
probe cannot extend an open circuit. Risk: low, a correctness fix in the breaker, not a switch. Verify on staging:
a fake provider that holds the probe in a unit-level drill, and read the next real reset (13:00Z) in the journal for
time-to-first-success (target: seconds).

**(a) Pace by hour budget, with the excess threads on the fallback from minute 0.** Needs: a ceiling in tokens per
clock hour, which the gateway does not know (best estimate 42.7M-43.5M, shared with local use), and a way to assign
threads. Smallest change: the gateway sums Devin IN tokens in the current clock hour (it sees every call and the
on-the-hour reset) and, when the projection passes the ceiling, answers new-thread starts (not in-flight threads)
with the same typed quota refusal guests already route on, naming the next hour as reset. The pool then assigns
those threads to Bedrock for the rest of the hour and keeps them there (thread-sticky, so the Bedrock cache is built
once, not at every switch). It is derived from a measured physical bound, not a flag; it needs a stated starting
ceiling (conservative: 38M) and should self-correct from each 429 (the IN at trip becomes the new estimate). Risks:
a wrong ceiling idles Devin early (cost: more Bedrock, no downtime) or still trips, as today; local owner use moves
the ceiling, so keep (b). Verify on staging: run d@e's 30-story proof during a normal paper hour; success is no 429
trip in the hour and Devin use above ~90% of the ceiling, read with the same journal greps used here.

**(b) Reserve headroom for the owner's interactive use.** Needs: the owner's local consumption, which the gateway
cannot see. Smallest change: hold back a fixed fraction of the (a) ceiling (for example 10%, about 4M IN), or read
the account's remaining quota directly if the call named in the last section exists, which removes the guess. Risk:
low; wastes up to the reserve when the owner is idle. Verify: count hours where a trip occurs while the gateway
total is under the ceiling (today: hours 07 and 08).

**(c) Cut the paper's tokens per call.** Needs: where the 50-100k tail comes from (Texture calls reach 216k per the
earlier prompt-size finding) and what Devin meters for cached input. Smallest change: none before the unit is known;
if cached input is free, the lever is cache hit rate, not size. Risk: medium (prompt changes alter agent behaviour).
Verify: IN per story before and after on d@e, then the paper. It pays only if the meter is IN.

**(d) A second Devin seat or account.** Needs: whether the provider sells one, and the gateway holding two sessions
and spreading by thread. Not researched here; no prices or terms asserted. The only option that raises the ceiling
rather than spreading it. Verify: two-session spend over two clock hours.

Recommendation: (0) now; then (a) with a conservative self-correcting ceiling plus the (b) reserve; (c) only after
the unit is known; (d) is a business decision.

## The single observation that would most reduce the uncertainty

The Devin CLI reports its own limits and reset time ("20 minutes"), so Devin has a quota-status call the CLI makes,
which returns the unit and the remaining amount. Capture that one call (endpoint name and response field names, not
tokens) from a local CLI run, for example with its debug logging. It would give the unit, the hourly limit, the
remaining amount, and whether the window is a clock hour, and would turn every "unknown" above into a number. If it
exists, pacing in (a) becomes a read of the provider's own meter.

## Finding: the 09:00-09:13 hold was a hung half-open probe (2026-11-10, journal read via `ssh node-b`)

Class: green. Method: `ssh node-b "sudo journalctl -u go-choir-gateway ..."`, grep of Devin lines for 08:50-09:15Z,
metadata only. Confirmed from the journal and the code; one detail is unexplained.

- Code. After a quota trip with a named reset, the model breaker half-opens at the reset and admits exactly one probe
  (`HalfOpenMaxProbes: 1`, `cmd/gateway/main.go`). The slot is held until that call returns
  (`internal/health/circuit_breaker.go` `Allow`/`doneLocked`). Every other call is refused as "provider quota
  exhausted ... resets at <now+15s> (estimated) (circuit open)" (`quotaRefusal`, `quotaProbeWait`). The probe had no
  deadline of its own: only the 10-minute `inferenceTimeout` (`handlers.go`) and the 10-minute provider HTTP timeout.
- Journal. From 09:00:13 to 09:12:08 every Devin request is refused with the 15 s "estimated" text, one every ~15-20 s.
  One request, a 72-message call at 09:05:49, is admitted (no refusal line; the next request at 09:05:50 is refused).
  At 09:12:09 that call (it is retried at once with 72 messages) fails `devin: read stream frame: stream error:
  stream ID 3; INTERNAL_ERROR; received from peer`, and the next line is `circuit open (model unhealthy)`, the
  breaker reopened on its 60 s backoff. The next Devin success is 09:13:38, then normal flow (`tokens=12225+133`,
  2 s call). So the probe that held the slot was in flight for 6 min 20 s, and the failed probe reopened the breaker.
- Unexplained: 09:00:13 is already refused as if a probe were in flight, but no Devin request was logged between
  08:50:42 and 09:00:13, so the first slot-holder from 09:00 to about 09:05:4x is not in the journal. Something took
  the slot at the reset and released it without a counted outcome (a caller that went away is released unrecorded,
  `ExecuteClassified`/`releaseProbe`), which is consistent with a second hung probe, but the journal does not show it.
  Total slot time from the reset to recovery: 12 min 09 s (09:00:00 to 09:12:09) plus the 87 s backoff.
- Verdict: hypothesis confirmed in kind (an unbounded single probe held the circuit open; the account was already
  reset, and the first probe that returned failed with a transport error), not proven for the first 5 minutes.
- Same hour, other data: `devin/swe-2-flash` tripped again at 12:56:42 (reset 13:00). Gateway restarts in the window:
  07:49:20 (the first trip's minute), 09:46:39, 10:09:41, 11:51:41, 12:30:09, 12:51:31. None is in 09:00-09:13.
- Fix: the probe after a quota hold gets a 90 s deadline (`quotaProbeDeadline`, `internal/gateway/circuit_breaker.go`);
  a probe that times out or fails reopens the breaker on its existing doubling backoff (30 s, 60 s, ...).
  Worst case from reset to recovery with one hung probe: about 90 s plus 60 s, against 13 minutes. 90 s is a multiple of
  Devin's normal answer (2-5 s in the same journal) and of the largest prompts (100k tokens).
