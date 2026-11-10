# Jev call sites, limits and latency (wave 0 item 2, lane A input)

Date: 2026-11-10. Mutation class: green (evidence only). Source: `origin/main`
at 9bbdfe68c, read-only gateway logs and receipt metadata on Node B (7 ssh
commands; no secrets, no env files, no question text or answers read).

Owner ruling that frames this (2026-11-09): "the Jev parallelism should apply
for every time we're calling Jev. We should be making many calls at once...
the more we call Jev, the better, because that's going to be a key part of our
learning loop."

## 0. What Jev is in the code

- Model: `~typesafe/jev-latest`, pinned by the gateway (`internal/gateway/jev.go:22`,
  `jevModel`). Served model in every receipt in the sample:
  `typesafe/jev-1.14-20261014` (all 30,000 sampled receipts, status 200).
- Provider path: OpenRouter Decisions alpha, `https://openrouter.ai/api/alpha/decisions`
  (`jev.go:23`), with the one `OPENROUTER_API_KEY` held by the gateway. TypeSafe
  is the model vendor behind OpenRouter; there is no direct TypeSafe endpoint.
- Wire: `{state, questions{name: {type: noul|choice|score, ...}}}`; the answer is
  `answers{name: {choice, probabilities}}`, `model`, `usage{input_tokens,
  output_tokens, cost}`, `id`, `provider`.
- Two gateway doors, one transport (`JevTransport.Decide`, `jev.go:87`):
  - guest door `POST /provider/v1/judgments` (`HandleJudgment`, `jev.go:303`):
    bearer auth, peer pin, per-computer per-scope rate bucket;
  - host door `POST /provider/v1/host/judgments` (`host_judgment.go:28`): the
    host-credential callers table; **no rate bucket at all**, only the body cap.
- The runtime seam is `CandidateJudge.Judge` (`agentcore/support_judge.go:27`),
  bound to `gatewayruntime.JudgmentClient.Decide` (`internal/gatewayruntime/jev.go:48`,
  `internal/autoputer/run.go:706,1119`). `callJudgeWithin`
  (`agentcore/intake_labels.go:397`) wraps every guest call with a timeout and a
  goroutine.

## 1. Every Jev call site

Today every call is one HTTP request carrying one `state` and a map of
questions; "questions per call" below is what the site builds. Hard question
cap from `internal/yaegikernel/system1.go:26-28`: `System1MaxQuestions` 12,
`System1MaxChoices` 6 (host-built questions bypass the 6 where the site builds
its own).

| # | Site (file:line of the call) | Question | Qs per call | Issue | Timeout | On failure | Recorded |
|---|---|---|---|---|---|---|---|
| 1 | Intake labels, `agentcore/intake_labels.go:391` (via `labelOneCapture` :244, `labelCaptures` :160) | "Which story / new capture reports the same event" (`choice`, up to ~30 recent + overlap picks + siblings as choices) and "which story shares the topic" | 2 (event, topic) per capture; one capture per call | concurrent, `intakeLabelConcurrency = 4` (:44), 15 s poll budget | 5 s (:46) | capture left unlabelled, logged | capture-label row (`store/capture_label.go:93`): model, event_p, topic_p, tokens, cost. No `model.served`, no distribution, no latency, no question text |
| 2 | Event layer, `agentcore/event_layer.go:387` (`resolveEventChunk`) | assign each capture to an event among shortlisted candidates | up to 12 captures per call (`eventChunkSize` :46), one question per capture; state capped 24 KiB (:67), splits in half past it | serial chunks under `idx.mu` (:341) | 30 s (:39) | chunk's captures "unresolved", logged | event membership rows; usage parsed but no `recordJevCostFor` call here. No `model.served` |
| 3 | Support judge, `agentcore/support_judge.go:288` | "does the cited passage support the sentence" per citation pair | all pairs of a revision in one call (no chunk cap seen; pairs named `p1..pN`) | background, runtime-wide semaphore `supportJudgeConcurrency = 2` (:59) | 90 s (:61), queue wait 2 min | pair stays pending, no resume at boot | `recordJevCost` (:297) -> run-progress event with model, provider, tokens, cost_usd, questions, plus a `model.served` event (role `system1`) (:399-411); verdicts as inbox records `Judge: system:support` |
| 4 | System 1 verb (research cells), `agentcore/system1_verb.go:274` (`system1Call` :208) | whatever the desk asks, closed choice questions | up to 12 (`System1MaxQuestions`) | one call per verb invocation; serial within a cell | 60 s (:73) | request event already on tape, `recordSystem1Failure`; error to cell | **best**: request event (digest, question ids, labels) and answer event (full result probabilities, usage, cost, model, response id) on the tape, plus `recordJevCostFor` (:331) and `model.served`. No latency |
| 5 | Story analysis, `agentcore/story_analysis.go:338` (through `system1Call`) | novelty / entity type / role per candidate | chunks of 12, budget 2 calls (24 qs, :67) | serial chunks | 60 s | `a.Error` set, candidates carry no answers | as site 4 (cross-run, with subjects) |
| 6 | Prompt router, `agentcore/prompt_router.go:82` (`askRouter` :205) | which desk gets the owner's prompt | 1 | one per prompt, in a goroutine | 8 s (:39) | rule fallback to the default desk | as site 4 plus a frozen route record; this is the only site with its own outcome scoring (route record) |
| 7 | Disclosure gate, `agentcore/disclosure_gate.go:583` | may this desk send this to this sink | 1 | one per gated egress, inline | 60 s (System 1 default) | the error returns to the gate (fails per gate policy) | as site 4 |
| 8 | Sub-call judge, `agentcore/subcall_judge.go:566`, `:635` | settle each delegated child's report against its rule | up to 20 children per call (`subCallJudgeBatchMax` :114), each up to `MaxSubCallQuestions+1` = 7 qs | **strictly serial** per computer: gate of 1, 2 s spacing (`subCallJudgeSpacing` :104), retry on 429 with doubling backoff | 90 s (:94) | work closed `failed` with the reason | `recordJevCostFor` (:597, purpose `subcall_judge`) -> run-progress + `model.served`; settlements as commitment records |
| 9 | Desk-cast judge, `agentcore/desk_cast_judge.go:481` | score each cast, batched | one call per batch of casts | through the same serial gate as 8 | 90 s | round marked failed | `recordJevCostFor` (:519) |
| 10 | Inert judge, `agentcore/inert_judge.go:173` | "could any plausible outcome have contradicted this prediction" yes/no | one per sampled record in a call (sampled share, `types/inertness.go:349`) | async, same serial gate as 8 | 2 min (:39) | "records left unjudged", logged; never retried | `recordJevCostFor` (:195, purpose `inert_judge`) |
| 11 | Front importance scorer, `frontimportance/scorer.go:407` (host door) | importance 1-10, promo, section | 3 per story, **one story per call** | **serial** loop (:156), `MaxCalls` 80 (:110), 3 min budget, run by a systemd timer | 45 s (:111) | story keeps its old score, `Deferred++` | a scores.json file; gateway log line `host judgment ... served (questions=3 bytes=~720)`; only the choice is kept, not the distribution; no tape record |

Sites 4-7 are the tape-recorded ones. The guest rate bucket applies to sites
1-10 (shared bucket `judgments`, except 1 and 2 which use bucket
`judgments:intake_labels`, `jev.go:46-49`). Site 11 has no bucket.

No site batches across callers: nothing coalesces calls from different
cells/desks into one request, so state is re-sent for each question set.
Site 8 batches children; site 2 batches captures; site 3 batches pairs.

### Body cap (the "8 KiB" figure)

The deployed judgment cap is **1 MiB**, not 8 KiB:
- code: `jevTransportMaxBodySize = 1 << 20` (`internal/gateway/jev.go:24`),
  mapped to route `judgment` in `body_caps.go:109`;
- nix: no `GATEWAY_BODY_LIMITS` override exists (`nix/node-b.nix:551` comment
  says "Jev (1 MiB)");
- live: the gateway boot log on Node B at 2026-11-10 01:22:04 reads
  `body caps: mode=enforce ... judgment=1 MiB,model=160 MiB ... search=1 MiB`.

The same 1 MiB bounds the response read (`jev.go:131-136`, and
`gatewayruntime/jev.go:15`). A 3-question answer is about 720 bytes, so 64
questions is on the order of 15 KB; the cap does not bind. The real binding
limits in code are `System1MaxQuestions` 12, `System1MaxChoices` 6, rune caps
(`system1.go:26-30`), and the guest rate bucket below.

## 2. Limits and behaviour from logs (Node B, last 6 hours)

How Jev shows in the gateway log:
- guest door: **successes are not logged**; only failures
  (`gateway: Jev judgment for <id> failed: ...`, `jev.go:~360`). Every guest
  success leaves a receipt in
  `/var/lib/go-choir/gateway/jev-transport-receipts.jsonl` (212 MB, fsync per
  call, no latency field, no question text, includes the full distribution).
- host door: `gateway: host judgment from host:front-importance served
  (questions=3 bytes=N)`, and failures with `questions=N`.
- provider/model: receipt `response.model` = `typesafe/jev-1.14-20261014`;
  the request is pinned to `~typesafe/jev-latest` via OpenRouter Decisions.

Measured (journal 6 h; receipt metadata, last 30,000 receipts, 2026-11-06 to
2026-11-10 02:xx UTC):
- Failures: **0** `Jev judgment ... failed` lines in 6 h; no `held after
  repeated failures`; **no 429 from the Jev provider**. (The 429 lines in the
  same window are the free-model inference route, `free-models-per-min`, and
  the per-computer inference limiter, not Jev. Not a Jev signal.)
- Calls per hour, whole fleet (receipts, UTC 2026-11-09 19:00 to 2026-11-10
  01:00): 440, 514, 519, 564, 579, 387, 433. Of 30,000 sampled receipts, 27,147
  are the paper (vm-1669020a, about 90%), 2,628 the front-importance host
  caller (about 480 a 6-hour window, 5-15 stories per half-hour run), 184
  vm-70070cf7, 39 a candidate-fleet computer, 2 vm-774a03f8. The paper runs at
  about 8-9 calls a minute, 0.14 a second on average.
- Questions per call (receipt `answers` count, 30,000 sampled): 1: 4,124;
  2: 14,581 (intake labels); 3: 5,748; 4-11: about 4,200; 12: 782; 13-20+:
  about 500. So the vendor handles 12 and above routinely; our median is 2.
- Observed concurrency (paper, last 8,000 receipts, completions per
  wall-clock second): 1 in 4,568 seconds; 2 in 372; 3 in 153; 4 in 215; 5-11
  in 76. So 4+ calls overlap in about 5% of busy seconds, a peak of 11 finishing
  in one second. Concurrency has already been exercised at 11 with no
  failures.
- Latency is not logged. Best estimate from the serial front-importance
  scorer on the host door: 32 inter-completion gaps in the last 30 minutes,
  p10 80 ms, **p50 109 ms**, p90 209 ms, max 254 ms. Each gap is one serial
  3-question `choice` call, so it is an upper bound on call latency
  including the scorer's own work (this contradicts nothing in the relayed
  70-500 ms).
- Concurrency at the host door: bursts of up to 11 served per second at
  02:03, but the scorer is serial; overlap there is zero.

## 3. Measurement on d@e: not possible without product code

No CLI verb or product endpoint issues a Jev judgment on demand:
- `choir policy verify` (`cmd/choir/policy.go:228`) verifies the model policy,
  not a judge role; no `judg`/`system1`/`jev` verb exists in `cmd/choir`;
- the only paths are an agent run (prompt router: one question, but uncontrolled
  and an agent run per probe) or the gateway doors, which need a guest bearer
  (guest door) or a host-caller entry (host door: `hostJudgmentCallers`,
  unknown names are refused, so adding one is product code).

So no d@e latency-vs-questions or consistency measurement was made. No call was
issued. The numbers in section 2 are the measured ones; the matrix below is
what the lane A builder should run first.

### Proposed probe (not committed; for the builder)

Run inside the d@e guest (it has the bearer; use `gatewayruntime.JudgmentClient`,
which is exactly what the runtime uses), on the guest door, **scope `""`**.
Budget: 12 size-matrix calls plus 63 concurrency calls = 75 calls, far under
300. Pace it: the guest bucket is 60 requests a minute per scope; the N=16
trials burst 16 at once, so sleep 25 s between trials.

```go
// scripts/jev-probe/main.go (proposal)
// usage: jev-probe -gateway $GATEWAY_URL -token-file F
package main

import (
	"context"; "encoding/json"; "flag"; "fmt"; "os"; "strings"
	"sync"; "time"

	"github.com/yusefmosiah/go-choir/internal/gatewayruntime" // check module path
)

// Known-answer question: parity of a number. Answer is "even" or "odd".
func question(n int) json.RawMessage {
	q, _ := json.Marshal(map[string]any{
		"type":         "choice",
		"instructions": fmt.Sprintf("Is the integer in state.numbers.n%d even or odd?", n),
		"criteria": map[string]string{
			"even": fmt.Sprintf("state.numbers.n%d is divisible by 2.", n),
			"odd":  fmt.Sprintf("state.numbers.n%d is not divisible by 2.", n),
		},
		"choices": []string{"even", "odd"},
	})
	return q
}

func request(k, base int) gatewayruntime.JudgmentRequest {
	nums, qs := map[string]int{}, map[string]json.RawMessage{}
	for i := 0; i < k; i++ {
		n := base + i*7 + 3 // mix of parities
		nums[fmt.Sprintf("n%d", n)] = n
		qs[fmt.Sprintf("q%d", n)] = question(n)
	}
	state, _ := json.Marshal(map[string]any{"task": "Answer each parity question.", "numbers": nums})
	return gatewayruntime.JudgmentRequest{State: state, Questions: qs}
}

type outcome struct{ lat time.Duration; right, wrong, missing int; err error }

func run(ctx context.Context, c *gatewayruntime.JudgmentClient, k, base int) outcome {
	req := request(k, base)
	t := time.Now()
	resp, err := c.Decide(ctx, req)
	o := outcome{lat: time.Since(t), err: err}
	if err != nil { return o }
	var d struct{ Answers map[string]struct{ Choice string } `json:"answers"` }
	_ = json.Unmarshal(resp.Distribution, &d)
	for name := range req.Questions {
		n := 0; fmt.Sscanf(strings.TrimPrefix(name, "q"), "%d", &n)
		want := map[bool]string{true: "even", false: "odd"}[n%2 == 0]
		a, ok := d.Answers[name]
		switch { case !ok: o.missing++; case a.Choice == want: o.right++; default: o.wrong++ }
	}
	return o
}

func main() {
	gw := flag.String("gateway", "", ""); tf := flag.String("token-file", "", "")
	flag.Parse()
	tok, _ := os.ReadFile(*tf)
	c := gatewayruntime.NewJudgmentClient(*gw, strings.TrimSpace(string(tok)))
	ctx := context.Background()
	for _, k := range []int{1, 8, 32, 64} { // note: 32 and 64 exceed System1MaxQuestions, which is host-side only
		for trial := 0; trial < 3; trial++ {
			o := run(ctx, c, k, 1000*(trial+1))
			fmt.Printf("size k=%d trial=%d lat=%s right=%d wrong=%d missing=%d err=%v\n", k, trial, o.lat, o.right, o.wrong, o.missing, o.err)
			time.Sleep(25 * time.Second)
		}
	}
	for _, n := range []int{1, 4, 16} { // N concurrent single-question calls
		for trial := 0; trial < 3; trial++ {
			var wg sync.WaitGroup; res := make([]outcome, n)
			start := time.Now()
			for i := 0; i < n; i++ { wg.Add(1); go func(i int) { defer wg.Done(); res[i] = run(ctx, c, 1, 5000+i*13) }(i) }
			wg.Wait()
			fmt.Printf("conc N=%d trial=%d wall=%s lats=%v\n", n, trial, time.Since(start), lats(res))
			time.Sleep(25 * time.Second)
		}
	}
}
func lats(r []outcome) []time.Duration { out := make([]time.Duration, len(r)); for i := range r { out[i] = r[i].lat }; return out }
```

The probe uses 1-question calls for the concurrency cells and parity
questions with known answers for the size cells; compare `right/wrong/missing`
at k = 1, 8, 32, 64. Record the p50/p90 per cell in the lane A ledger. The
gateway-side limit (`validateJudgmentRequest`) does not cap questions, so 64
passes the gateway; the 12 cap is in the cell-side verb only. If a body
refusal or a 400 appears, record its exact text.

## 4. Per-organization rate limits

What the code and config assume:
- Gateway guest door: **60 requests per 1-minute window per computer per scope**
  (`ratelimit.go:12,16`, default; `GATEWAY_RATE_LIMIT_*` is not set in nix).
  Scopes: `judgments` and `judgments:intake_labels` (separate buckets). 429 with
  `Retry-After` (`jev.go:345`).
- Host door: no bucket (`host_judgment.go`); the scorer self-limits with
  `MaxCalls` 80 per run.
- Sub-call gate: `subCallJudgeSpacing` 2 s (comment `subcall_judge.go:100-104`
  assumes "~60 a minute the support judge and the labeller share").
- Gateway-level hold: `upstreamHold` (`jev.go:67`, `upstream_hold.go`) refuses
  all Jev calls for every computer for a period after repeated upstream failures.
  That shared fate is not a rate limit but it is global.
- Upstream: **no code or config encodes an OpenRouter or TypeSafe org limit**.
  No `x-ratelimit-*` or `Retry-After` header from the provider is logged or
  read (`jev.go:166` reads no header other than `X-Request-ID`). The relayed
  figures in the handoff (about 20 requests a second, 250k tokens a second per
  organization; 64k tokens per request, 32k for state plus one question) are
  **unverified**.

Unknown without the owner: the true OpenRouter org limits and the TypeSafe
org limits as the consoles state them; whether they are per key or per
organization; whether OpenRouter applies its own cap on the alpha decisions
route below the vendor's. The 6-hour log shows no Jev 429, with a fleet peak
of 11 completions in a second, so the limit is above that.

## 5. Recommendation for lane A

### Where the number comes from

Latency is about 0.11 s median, 0.21 s p90 for a 3-question call (upper
bound). By Little's law, a sustained 15 requests a second needs only
15 x 0.2 = **3 in flight, organization-wide**. So the binding control for the
relayed 20 rps is a **rate**, not a concurrency cap; concurrency caps only
need to be big enough to hide latency spikes. Every per-site pool below is far
above what the org-wide rate allows, so a shared token bucket must sit behind
them.

### Concurrency per call site (in flight, per computer)

| Site | Today | Set to | Why |
|---|---|---|---|
| 11 front-importance scorer | 1 (serial), 80 calls | 12 | one story per call, 109 ms each: 80 calls serially take about 9 s, a pool of 12 about 1 s; this is the re-sort time in handoff section 6. Raise `MaxCalls` with the pool |
| 1 intake labels | 4 | 12 per poll | one capture per call; cost scales with captures |
| 2 event layer | serial chunks | keep commits ordered (an event minted in one chunk is a candidate in the next, `idx.mu`); fan out the question calls ahead of the commits, pool of 4 |
| 3 support judge | 2 | 8 | publication waits on it; pairs are independent |
| 8, 9, 10 sub-call, cast, inert | 1 call per 2 s per computer | remove the 2 s spacing and the gate of 1; pool of 4 | the 2 s spacing exists only because the gateway bucket is 60 a minute; replace both with the shared limiter |
| 4, 5 System 1 verb, story analysis | serial | pool of 4 for story-analysis chunks; split verb calls so long as questions are independent | |
| 6, 7 router, disclosure | 1 | 1 per event | these are latency-critical, one question |

Questions per call: lift 12 to 64 at the verb, but keep 12-20 where a call
mixes unrelated questions until the probe above shows answers stay consistent
at 32 and 64; prefer many parallel 8-16-question calls over one 64 call,
because a failed or timed-out big call loses all of its answers and the
vendor's per-question latency effect has not been measured by us.

### Shared limiter: at the gateway, in `JevTransport.Decide`

All doors converge in `JevTransport.Decide` (`jev.go:87`), inside the single
Node B gateway process, which already holds the one provider key and the
upstream hold. Put one process-wide token bucket (rate about 15 a second,
burst about 30, from the 20 a second relayed limit; adjust after the console is
read) plus a semaphore of about 32 in flight **there**:
- covers the host door (currently unmetered) and the guest door;
- scopes: interactive (router, disclosure, system1 verb) in its own reserved
  share so background bulk (scorer, labels, inert, support) cannot starve it;
  the existing two scopes (`jev.go:46`) become priority classes;
- on upstream 429 honor `Retry-After` with one shared backoff (the transport
  today only holds on 5xx/429 for a fixed hold, `jev.go:126-143`, and does not
  read the header);
- keep a per-computer fairness cap so one computer (the paper) cannot take the
  whole org share, and **replace the 60 a minute per computer bucket for Jev**
  (1 a second is the real ceiling today, below the fleet's need).
The guest keeps only a small pool and a 429-aware retry; it must not own the
org limit because many computers share it. If a second gateway process exists
on another node, the limit must be split or moved to a shared store; unknown
whether node-a runs one.

Also fix two transport costs before raising concurrency (hypotheses from the
code, to confirm with the probe): the receipt writer does `Sync` under one
mutex per call (`jev.go:277`), which serialises all completions behind
fsyncs and grows an unrotated 212 MB file; and the Jev `http.Client` uses the
default transport (2 idle connections per host), so 12+ concurrent calls to
openrouter.ai open fresh TLS connections. Give it a transport with
`MaxIdleConnsPerHost` of 64.

### What each Jev call must record (calibration)

One durable row per call, on the tape for guest calls (a `jev.call` record next
to the existing `model.served`), including host-door callers:
- call id, site/purpose, computer id, run id, timestamp;
- request digest and the question set: ids, text or criteria digest, type,
  options and their order;
- the full per-question probability distribution (not just the choice), so a
  new threshold replays without a new call;
- model and served model, provider, response id, OpenRouter request id;
- input/output tokens, cost, and **latency** (queue wait, gateway time,
  upstream time) and attempt number;
- number of questions and the state size;
- the decision it fed: the record or action id (label row, event membership,
  route record, settlement, publication check) and the threshold in force;
- the **settling outcome**: a pointer to what later says whether it was right
  (editor override of a label via `RecentCaptureLabelOverrides`, an event split
  or merge, a route correction, the owner moving a story's section, a support
  verdict later corrected by a Texture write, a prediction's outcome).
  Sites 1, 2, 3, 6, 8 already have an outcome record to point to; 11 and 7 do
  not and need one.
Today only sites 3, 4-10 write cost to the tape, only 4-7 keep the answer and
probabilities, none keeps latency, and 1, 2 and 11 write nothing or a single
choice without a distribution.

### The three call sites where more Jev would help most tonight

1. **Front importance scorer (11)**: serial, one story per call, only the
   chosen option kept. Parallel pool of 12, store the distributions, and add
   questions per story (the handoff's section and re-sort inputs), since 109 ms
   a call makes re-scoring the whole front a few seconds. Also gives the cheapest
   calibration data.
2. **Intake labels and event layer (1, 2)**: 2 questions per capture and one
   30-way choice to find the same event; the hardest quality problem (the
   30-newest candidate list, handoff section 4). Many small yes/no "same
   event?" questions per capture-candidate pair, in parallel with a pool of 12,
   replaces one large choice; the paper already issues 14,581 two-question
   calls in the sample, so the call rate is already proven safe.
3. **Support judge (3)**: publication is gated on it, concurrency is 2, one
   call per revision. Splitting into per-pair calls in a pool of 8 shortens
   the publication path and lets one question per pair carry sub-questions
   (supported, overstated, wrong-source) for calibration.

Runner-up: the sub-call, cast and inert judges (8-10) are held to 1 call per
2 s by a rule that the shared limiter removes.

## Unknowns

- True OpenRouter and TypeSafe organization limits (requests, tokens,
  concurrency): owner/console.
- Latency as questions per call grow, and accuracy at 32/64: not measured (no
  d@e path without product code); the probe above is the first task of lane A.
- Upstream latency at the 1.14 model under load: only the serial host-door
  estimate exists.
- Whether Node A runs a second gateway sharing the key.
- Whether the guest-door 429 path has ever fired for Jev: no such line in 6 h,
  and the older logs were not searched.

## Build: failure list (lane A, steps 1-3)

Mutation classes: step 1 red (gateway), steps 2 and 3 orange. Rollback for
each: revert the commit. Written before the code; each line names its test.
Step 1 = the shared limiter in `JevTransport.Decide`; step 2 = one bounded
fan-out helper at every call site; step 3 = one recorder behind every call.

### Limiter and transport (step 1)

| Failure | Test |
|---|---|
| A scorer burst (host door, 80 calls at once) fills the rate and in-flight budget and the prompt router or disclosure gate waits behind it. The interactive class owns 8 of the 32 in-flight slots and 5 of the 30 burst tokens that bulk callers cannot take. | `TestJevLimiterInteractiveNotStarvedByBulk` |
| Upstream 429 with `Retry-After`: today it opens a 1-minute doubling hold for every computer. A rate answer is not an outage. It now sets one shared backoff until `Retry-After` (default 2 s, cap 60 s) that every waiter honours; 5xx and timeouts keep the hold. | `TestJevLimiterSharedBackoffHonoursRetryAfter`, `TestJevTransport429SetsBackoffNotHold` |
| A waiter outlives its caller (context cancel, client gone): it must leave the queue and release nothing it did not take. | `TestJevLimiterCancelledWaiterLeavesNoSlot` |
| In-flight slot leak on any return path of `Decide` (error, bad JSON, receipt failure). | `TestJevLimiterReleasesOnEveryPath` |
| One computer takes the whole organization share. Per-computer in-flight cap 16 (below the bulk ceiling of 24, so one computer cannot hold every bulk slot) and a per-computer rate of 10/s burst 30 replace the 60-a-minute bucket. The abuse bound of a compromised guest is therefore 10 calls a second, 600 a minute, ten times the old bucket; question count per call is capped only by the 1 MiB body. | `TestJevLimiterPerComputerCap`, `TestJevLimiterOneComputerCannotHoldTheBulkShare` |
| The scope is the guest's own claim, so a guest asserts `interactive` to draw the reserve. The interactive class is a per-computer budget keyed on the authenticated computer id: 2 in flight and 2/s; past it the call is bulk. A per-computer cap is enough because the claim cannot widen the budget, one computer can hold 2 of the 8 reserved slots, and draining the reserve takes four authenticated computers, each already bounded by its own rate. | `TestJevLimiterInteractiveClaimIsBoundedPerComputer` |
| A queued caller reaches the provider after it failed (the hold was checked only before the queue, so queued failures re-armed the hold and rolled it out to 15 minutes). The hold is re-checked after the queue. | `TestJevTransportRechecksTheHoldAfterTheQueue` |
| Limiter refusals invisible in the per-call log and hour summary; the host door answered 502 to a refusal. Refusals are logged and counted; the host door answers 429 and 503 with `Retry-After`. | `TestJevLimiterQueueWaitBounded` (limited error), review of `host_judgment.go` |
| Waiting forever behind a stuck upstream: the queue wait is bounded at 20 s, then the caller gets 429 with `Retry-After`. | `TestJevLimiterQueueWaitBounded` |
| Many computers together exceed the organization rate: the limiter is process-wide, in the one gateway that holds the key, so the sum is bounded by construction (15/s sustained). Unverified: whether a second gateway process shares the key. | `TestJevLimiterRateIsProcessWide` (many computer ids, one budget) |
| Receipt `Sync` under the one mutex serialises completions behind fsyncs. The write stays under the mutex, the `Sync` moves out of it; durability before the answer returns is kept. | `TestJevReceiptWriterConcurrentWrites` (race; every line complete and valid) |
| Two idle connections per host: 12+ concurrent calls open fresh TLS. Transport with `MaxIdleConnsPerHost` 64. | `TestJevTransportIdleConnsPerHost` |
| Latency log missing or the hourly summary racing: counters under a mutex; the summary is emitted by the first call after the hour, so there is no goroutine to leak. | `TestJevStatsSummary` |
| No-switch rule: no env flag turns the limiter off; rate, burst and caps are constants in code. | `scripts/check-switch-registry` plus a source assertion in `TestJevLimiterHasNoSwitch` |

### Fan-out (step 2)

| Failure | Test |
|---|---|
| Results arrive out of order where order matters (event layer commits: an event minted in one chunk is a candidate in the next). The helper returns results indexed by input, never by arrival; the event layer asks its questions concurrently and commits strictly in chunk order. | `TestFanOutKeepsInputOrder`, `TestEventLayerCommitsInChunkOrder` |
| Partial failure: one task errors. Every other answer is kept and returned with its own error; the caller treats an errored slot exactly as it treated a failed call before (unlabelled, unresolved, pair pending). The helper never cancels siblings on a sibling's error. | `TestFanOutPartialFailureKeepsOthers` |
| Goroutine leak on context cancel or restart: tasks not yet started never start; the helper returns only after started tasks return, and each call is already bounded by `callJudgeWithin`'s timeout. A crash closes work and nothing resumes. | `TestFanOutCancelStartsNoMore`, `TestFanOutNoGoroutineLeak` |
| A panic in a task kills the process: recovered into that slot's error. | `TestFanOutPanicIsAnError` |
| Duplicate judgments on retry: a fan-out never retries a slot; the existing per-site retry (sub-call 429 backoff) is the only retry and it re-asks one batch. Verdict writes stay idempotent by their existing keys. | `TestFanOutNeverRetries` |
| Pool size 0 or negative, more workers than tasks. | `TestFanOutBounds` |
| Concurrency exceeds the bound. | `TestFanOutRespectsLimit` (race) |
| The 2 s serial gate removed, a burst of settlements then 429s from the gateway. The gateway limiter now queues rather than refusing; the sub-call 429 backoff stays as the fallback. | `TestSubCallJudgeRunsConcurrently` |

### Recording (step 3)

| Failure | Test |
|---|---|
| The record write fails or is slow and holds the hot path. The recorder hands the record to a bounded channel with a non-blocking send (a full channel drops and counts the drop) and one writer goroutine appends; a call never waits on the disk. | `TestJevRecorderNeverBlocksCaller` |
| The writer goroutine leaks at restart. It stops on `Close` and when the runtime's context ends; a crash loses the unwritten tail, which is acceptable for observation. | `TestJevRecorderCloseDrains` |
| A failed call records nothing (front scorer today). Every call, success or failure, writes one record; failure carries the error text, clipped. | `TestJevCallRecordedOnFailure` |
| Large questions or states make a record huge. Digest plus a 512-rune excerpt; a record is capped at 8 KiB and sheds the excerpt, then trailing answers. | `TestJevRecordBounded` |
| File growth. Rotation at 32 MiB, 4 files kept. | `TestJevRecorderRotates` |
| Secrets or the owner's text in records. The state is never stored. Question text is stored (160-rune excerpt and options) only for sites whose questions the host builds; for the System 1 verb and the sub-call, cast and inert judges, whose questions an agent writes and may fill with private text, only the digest and name are kept. | `TestNoTextKeepsNeitherExcerptNorOptions` |
| No-switch rule: no env flag disables recording. | `scripts/check-switch-registry` |

### Where the record lives, and its volume

Bytes per judgment, from the fields kept (ids, question names and options,
a 512-rune excerpt, the top six probabilities per question, model, tokens,
cost, latency, decision id): about 1.5 KB for the median call (2 questions),
8 KiB hard cap.

Calls per hour for the paper: 500 to 5,000.

- 500 an hour: 0.75 MB an hour, 18 MB a day.
- 5,000 an hour: 7.5 MB an hour, 180 MB a day.

On the semantic tape, 180 MB a day is more than the tape's whole current
growth, and every byte is replicated and replayed. A batched telemetry kind
would still be tape. The gateway already keeps every successful distribution
(212 MB in four days) outside the tape.

Chosen: a guest-local append-only JSONL file `jev-calls/calls.jsonl` in the
data directory beside the store, rotated at 32 MiB with 4 files kept (at most
128 MiB; at 5,000 an hour about 17 hours, at 500 an hour about 7 days). This
is observation like the source-projection fact log: not authority, not
replayed, and a crash may lose the tail. The tape keeps what it keeps today
(the cost lines, `model.served`, the System 1 request and answer events, the
decision rows). The decision id in each record is the join key to those. Read
path: none exists for similar guest-local logs; a reader and a shipper off
the computer are follow-ups.
