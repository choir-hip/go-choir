# The sub-call spacer outlived the bucket it protected (2026-11-24)

Mutation class: red (provider calls); a deletion. Rollback: revert the commit.

## Problem

One shared spacer (`callSpacer`, `spacedProvider`,
`Runtime.subRLMCallSpacer`, `internal/agentcore/research_children.go`) starts
every sub-call model call at least 1.5 s after the last one, across the whole
computer: 40 a minute for research children and management tasks together,
compaction included. A burst of K=4 management tasks and 4 research children
at once is a queue, not a burst. Owner, 2026-11-24: "how did the 40 calls a
minute thing get there? thats wrong." The owner's rule (2026-11-10): "too many
model calls is a code problem to fix, not a limit to impose."

A second self-imposed control lives beside it: the computer setting
`management.serial_tasks` (K = 1 when on). It is a rollback switch, and the
rule is no switches (the switch audit of 2026-11-03 already ruled it delete).

## Evidence

- 3faa001a1 (2026-10-10) added the spacer so research children stay under the
  gateway's per-computer bucket of 60 calls a minute
  (`docs/archive/problems/research-fanout-overruns-computer-model-budget-2026-10-11.md`).
  Its own comment names the premise: "under the gateway's default
  per-computer budget of 60".
- 433ce4d6e (2026-10-14) extended it to management task model calls and
  added `management.serial_tasks` (FM14, FM31).
- 149828338 (2026-11-10) removed that bucket
  (`gateway-inference-bucket-self-imposed-2026-11-10.md`). The spacer, whose
  only stated reason was the bucket, was not touched, and its comment still
  cites "the gateway's default per-computer budget of 60".
- `docs/problems/agent-tree-budgets-are-time-only-2026-10-21.md` listed the
  spacer as a resource control ("moves to the gateway with RES-11"); no later
  record asked whether it should exist.
- `docs/design/first-design-path-2026-11-22.md` noted the spacer against the
  rule and wrote "whether it is applied was not checked". It is applied:
  `research_children.go` (children) and `runtime.go` `executeWithToolLoop`
  (every management task run).
- The guest's wait on a provider 429 `Retry-After`
  (`internal/gatewayruntime/provider.go`, up to 10 waits of at most a minute,
  never past the caller's context) and the gateway circuit breakers are
  back-off after a provider's real refusal. They stay.

## Belief

The spacer has no premise left. It slows the capture path (management tasks)
and research for no protection: the gateway no longer refuses our own
computers' calls, and a provider's own refusal is already waited out and
broken on. Deleting it, and the serial-tasks switch, is a correct fix. The
caller-side question (too many calls) is answered by the capture desk and
by the one-context rule, not by spacing.

## Remaining error

- What a burst of 16 calls does at the provider is read from the code and not
  yet observed. The claim in the ledger (C244) tests it after deploy: no rise
  in provider 429s over 6 hours.
- The gateway's other controls (circuit breaker, paid budget holds, Devin
  pacing, the Jev limiter) were read, not load-tested.
- Provider capacity (a free pool's real limit) may show as more 429s and
  breaker opens. That is the provider's refusal, handled as before; if it
  shows, the fix is fewer calls from the caller, not a spacer.

## What a provider's refusal does on the sub-call path (read from code, 2026-11-24)

Owner, 2026-11-24: "some concurrency limits may be required otherwise we get
rate limited by providers. and may cause system resource pressure. but thats
something we should learn." Before deleting the spacer, the question was
whether a refusal today lowers how many calls start next, or only makes each
call wait alone.

- **In the guest** (`gatewayruntime/provider.go` `call`): per call only. A 429
  with Retry-After is waited out by that call (up to 10 waits of a minute,
  within its context); a 429 without it is retried 3 times at 0.5 s and 1 s;
  a provider quota 429 (typed) returns at once and the tool loop switches
  model. Nothing is shared between calls in the guest.
- **In the gateway** (`gateway/circuit_breaker.go`): shared, per model, across
  every computer. A provider failure (429, 5xx, timeout) counts against its
  model; at the threshold the model's breaker opens and every later call is
  refused at once with "circuit open", which the guest fails over on (research
  children pass fallback models; task runs use the normal tool loop). An
  exhausted quota opens it at once until the quota's Retry-After; the breaker
  half-opens and one probe decides, so it widens again on success. Synthetic
  has a gate holding excess calls for a slot (`SYNTHETIC_MAX_CONCURRENT`);
  Devin has hourly pacing; the paid budgets hold by day.

So a shared, evidence-driven back-off per provider route already exists and is
what a burst meets. A second gate in the guest was not written (owner:
"if such a shared back-off already exists, use it"). A burst after the
deletion is not a wall of refused calls retrying alone: after the model's
threshold of refusals the rest are refused at the gateway without reaching the
provider and move to the next pool member. Pinned by
`TestBurstOfSubCallsIsHeldOnlyByTheProvidersRefusals`: no refusal, no
gating; refusals open the model for all callers; a successful probe opens it
to the whole burst.

One gap closed here: an ordinary provider failure (a plain 429, a 5xx) that
opened a model's breaker was not logged, only quota, concurrency-streak and
hard opens were. It is now: `gateway: model P/M narrowed by a provider failure
(<the refusal>; trip N, half-opens at T)`, one line per open. Every narrowing
of the bound is therefore visible with its cause.

Residual error: the breaker's failure threshold and open window are constants
somebody picked, not learned (see the table column). A wall of 429s before the
threshold is possible for a burst larger than the threshold; the claim C244
counts provider refusals on the sub-call path after deploy.

## What the 4-at-a-time research children bound protects

Its comment (`research_children.go`) says each child is a host worker process
and the bound "protects guest memory"; the task slots' comment says about 17
MiB a worker. Neither number was measured against a computer's size in any
record found: no run logs a child's peak resident memory. The bound is a
constant we picked, left as it is. What would let it be learned: log each
child's and each task worker's peak resident memory when it ends (against the
computer's memory and its free memory at start), and a count of out-of-memory
kills and boot interruptions while workers run; then size the cap from the
computer's memory and the measured peak, not a constant. No memory governor
is added in this change (owner).

## Every other limit on our own model calls still in the code

Column "Value from": evidence (derived from an observed refusal or a
measurement) or constant (we picked it). "Evidence that would set it" says
what to measure.

Verdict classes: forbidden limit (the rule forbids it); deadline or back-off
(kept); bound on strangers or hostile cells (kept); memory or size bound
(kept for now).

| Limit | Where | Verdict | What replaces it / what removes it | Value from | Evidence that would set it |
|---|---|---|---|---|---|
| Sub-call spacer, 1.5 s apart, 40 a minute | `research_children.go` `callSpacer` | forbidden limit | deleted in this change | constant (1.5 s picked to fit a bucket of 60) | none; deleted |
| `management.serial_tasks` (K = 1) | `store/computer_setting.go`, `management_tasks.go` | forbidden (a switch) | deleted in this change; a stored value is ignored | a switch | none; deleted |
| Research children 4 at a time per `choir.Research` call | `researchChildConcurrency` | memory bound: each child is a host worker process (about 17 MiB); it queues, never refuses | stays here. Removed by the delegation protocol (`first-design-path-2026-11-22.md`: a desk creates RLMs and messages both ways), which sizes workers by the computer's memory, not a constant | constant; the comment says memory, never measured | peak resident memory per child against the computer's size and free memory, plus out-of-memory kills (see above) |
| Management tasks K = 4, at most 8 | `DefaultManagementTaskSlots`, `MaxManagementTaskSlots`, `managementTaskSlots` | memory bound (guest memory); the comment also cites the gateway budget, which is gone (comment corrected here) | stays. The capture desk replaces the management task path, so the K slots go with it | constant (4; cap 8 for memory); the old "gateway budget" reason is gone | the same measurement for a task worker (about 17 MiB claimed, not measured) |
| 12 captures per task | `yaegikernel.MaxTaskCaptures`, `arrival_task.go`, `management_tasks.go` Delegate | prompt-size bound on one context; it splits a batch into more calls | stays. The capture desk replaces the management task path and its 12-capture batches (capture owns the graph custodian: `first-design-path-2026-11-22.md`) | constant | prompt size against the model's context for a capture batch; moot once the capture desk replaces the path |
| 120 wakes an hour per app desk | `deskmanifest.MaxWakesPerHour`, `desk_quota.go` | bound on what an installed package (a stranger's manifest) can cause; the comment says "a rate bound on a computer's own resources, not a spend ceiling" | not touched here, recorded. Revisit with the package-desk work; a built-in desk is never counted | constant | the wake rate a package desk needs, and the cost a hostile manifest could cause (a bound on strangers needs a number, set by what a stranger can spend) |
| 20 Texture root digests an hour, 120 s debounce | `provideriface.DefaultTextureRootDigestPerHour`, `textureowner/texture_root.go` | forbidden limit on our own model calls (PD3 S1), and the root desk sits behind `texture.root_desk`, off by default (a switch) | not touched here, recorded. Removed with the Texture root desk's own landing (the switch audit and the first-design path both name it); no unnamed residual: the digest runs a root run per hour, so removing the cap is that desk's change | constant (PD3 S1) | none to keep: model-call count is the caller's defect; the root desk's digest should be one call over one context |
| `System1MaxQuestions` = 12 per call, `system1RouteMaxQuestions` | `yaegikernel/system1.go`, `api_system1.go` | size bound on one Jev context and a bound on what a hostile cell can send; the code already splits longer lists | stays; vendor and prompt-size bound, not a count of calls | constant, set by the Jev prompt and answer size | Jev's measured context and output limits (`docs/evidence/jev-measurement-2026-11-10.md`) |
| `maxRateLimitWaits` = 10, one minute each | `gatewayruntime/provider.go` | back-off after a provider's real 429 with a deadline | kept exactly | evidence: waits the provider's own Retry-After, capped by a minute and ten waits we picked | the longest Retry-After the providers send (log it) |
| Circuit breakers, `upstream_hold`, Devin pacing, paid-budget holds | `internal/gateway` | back-off after real refusals, owner spend caps, quota shaping | kept exactly | mixed: quota holds and cooldowns follow the provider's Retry-After; failure thresholds and base windows are constants | the provider refusal logs: `narrowed by a provider failure` lines (added here) against later 429 counts |
| Jev limiter (15 a second, 16 in flight per computer) | `gateway/jev_limiter.go` | vendor-capacity pacing of Jev calls, measured (`docs/evidence/jev-measurement-2026-11-10.md`); not a sub-call or inference limit | not touched; the "as many questions as possible in one call" rule applies to callers | evidence: measured (`jev-measurement-2026-11-10.md`) | re-measure when the vendor changes |
| Anthropic relay per-computer counter | `gateway/anthropic_relay.go` | owner grant | kept | owner grant | the owner |
| Per-call and per-task deadlines (`managementTaskDeadline`, `managementTaskCallTimeout`, `researchCallWindow`) | `agentcore` | deadlines | kept | constant deadlines | observed run durations |
