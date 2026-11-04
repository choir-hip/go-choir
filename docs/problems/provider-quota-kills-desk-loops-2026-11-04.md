# A provider quota kills every desk loop on the model after 45 s (2026-11-04)

Status: superseded by the switch ruling below (2026-11-04, about 18:30Z); the park and its machinery are deleted. Mutation class of the fix: red (gateway provider error
handling and circuit breaker, the guest's provider retry path in the tool
loop). This doc is green.

Owner ruling, 2026-11-04: "eradicate the waiting except for true resource
contention. and even then, that should be semantically exposed to eng,
mgmt, and texture." A provider quota is true resource contention on an
external resource, so it must be exposed, and the work it holds up must not
be thrown away. Owner rule, 2026-10-23: a computer that pins a concrete
model does not fall back ("I'd rather them fail than fallback"), so for a
pinned model the only choices are to wait or to die.

## Evidence (Node B gateway journal, 2026-11-04)

```
02:35:23 provider: synthetic quota weekly_percent_remaining=0.0000 weekly_next_regen=2026-11-04T03:07:16.000Z subscription_requests=0/500
02:35:23 provider: synthetic rejected request status=429 reason="You've exceeded your subscription rate limits. Upgrade, or try again later. ..."
02:35:23 gateway: model synthetic/hf:deepseek-ai/DeepSeek-V4.1-Flash-1017 cooling down (quota exhausted, trip 1, until 2026-11-04T02:40:23Z)
02:35:23 gateway: provider call failed for autoputer vm-04a4…: synthetic: quota exhausted: status 429 Too Many Requests (sanitized)
02:35:24 gateway: provider call failed for autoputer vm-04a4…: provider synthetic model hf:deepseek-ai/DeepSeek-V4.1-Flash-1017: circuit open (model unhealthy)
02:35:26 … vm-ab6b…: circuit open (model unhealthy)
02:35:39, 02:35:54, 02:36:09 … (15 s apart, per computer) circuit open (model unhealthy)
```

Every desk loop on that model, on four test computers (engineering,
Texture and management), failed the same way within about a minute:
`loop.cancelled` or `texture.agent_revision.failed`. One engineer was
mid-edit; another's Claude delegate had just finished a 22-minute job whose
result the loop may never have received. Nothing told the desks or the
owner what was happening or when it would clear, although Synthetic said
when: `weekly_next_regen=03:07:16Z`, 32 minutes later.

## The path (code at origin/main 751eb988a)

1. `internal/provider` types the 429 as quota (`upstreamQuotaError`) but
   keeps no reset time: neither `Retry-After` nor Synthetic's
   `x-synthetic-quotas` `planWeeklyLimit.nextRegenAt`, which the same
   function already logs.
2. `internal/gateway/circuit_breaker.go` trips the model breaker for the
   quota cooldown (5 min doubling to 1 h), unrelated to the reset time. Each
   later call gets `circuit open (model unhealthy)`: the quota is labelled
   ill-health, and the reason and the reset time are lost.
3. `internal/gateway/handlers.go` answers every provider failure 502 with
   one `error` string. The guest cannot tell a quota from an outage.
4. Guest, `internal/toolregistry/toolloop.go`:
   `callToolLoopProviderWithPoolRetries` waits 5 s on the first error (it
   contains "429"), then gets "circuit open", which it does not retry.
   `RunToolLoop` treats the 502 as a transient outage and retries 3 times
   at 15 s (`transientAttempts < 3`), then returns the error. The run fails
   or is cancelled about 45 s after the quota hit, its work item closes,
   and its in-memory loop state (messages, job deliveries) goes with it.

## What should happen

- The gateway types a quota apart from model failure: a 429 with quota or
  rate-limit text, or with a reset time. It carries the reset time
  (`Retry-After`, else the provider's own reset such as Synthetic's
  `nextRegenAt`, else the gateway's own cooldown, marked as an estimate)
  and the reason "provider quota exhausted (<provider>)" to the guest in a
  typed answer (HTTP 429, kind `provider_quota`). The quota's circuit
  half-opens at the reset time, and a call refused by it gets the same
  typed answer, not "model unhealthy".
- A desk loop that gets the typed quota answer with no pool member left
  parks: it keeps its work item and its loop state, and calls again when
  the reset time passes (or after a bounded backoff up to 30 minutes when
  no time is known). While it waits it publishes a `types.WaitState` of
  kind `provider_quota`, class `contention`, with the provider, model and
  expected time, to the run's trace, to management's pack and the
  obligations surface, and through the engineering progress relay to
  Texture: "waiting for the Synthetic quota to reset (about N min)".
- Restart rules hold: the park is in-process, so a crash during it closes
  the run as interrupted; a planned restart may resume it, and the resumed
  loop parks again if the quota has not reset.
- A delegate job that ends during the park keeps its result: the end record
  is committed to the engineering inbox and the job view stays in the
  executor; the same tool loop's delivery record hands it to the model on
  the first turn after the park.

## Residuals known now

- The park lives inside the run's activation budget (60 min default; a
  management task's deadline for tasks). A quota that resets after the
  budget still ends the run, now with the wait visible.
- A management task run has a 10-minute deadline, so a task parked on a
  longer quota still ends at its deadline (review r1 DeepSeek).
- A computer whose loops are parked is busy, so an update waits for it like
  any busy computer.
- Mid-stream provider errors are still swallowed by the guest's SSE parser
  (pre-existing, review r1 Devin F5); desk loops call unstreamed.
- A pool parks on its last member only, even when an earlier member resets
  first.

## Fix review (tier 2)

Round 1: Devin APPROVE (non-blocking: announce-timer race, stale cached
assignment, Retry-After 0); DeepSeek v4.1 flash BLOCK on B1: a success or
failure from a call in flight when the quota tripped dropped the quota hold
while the breaker stayed open, so refusals until the reset read "model
unhealthy" again. Fixed in the same change: the hold lives as long as the
quota's breaker is open; a failure-opened breaker is not relabelled quota
(B4); the Synthetic reset time follows the spent limit (subscription
renewsAt vs weekly nextRegenAt, B2); a park that ends with its run is told
as such, not as a reset (B3); observations and the end are serialized.

Round 2 (Devin, one seat): BLOCK on R2-B1, the success and failure arms
read the breaker before taking the provider's lock, so a quota hold written
in between could still be deleted. Fixed: every arm reads the breaker under
that lock (the breaker's own lock is a leaf). Not re-reviewed after this
three-line fix (the mission brief allows one re-review); the remaining r2
items are non-blocking (an in-flight failure extends the open window and
refusals past the reset read as a 15 s estimate; the announce timer path
has no direct test).


## Staging defect after 7efaaeae2 (live8, 2026-11-04)

The management nudge loops on claude (trajectory f7f736ee) and pool
(d98fb630) parked on the Synthetic quota until 05:20:42Z (delay 1800000 ms),
then ended with `loop.cancelled` "activation budget exceeded: progress
deadline reached" at 05:04:46 and 05:05:27: the activation's progress
deadline kept running during the park, so the park became a cancel and the
work was lost (the residual named above). At 05:10 main moved those arms'
policy files from Synthetic to OpenRouter for the same model; a parked loop
keeps the model it resolved before the park and would not have seen it.

Fix: a park suspends the activation's progress deadline (the time parked is
not charged; the deadline moves out by it), bounded by the activation's
hard wall (6 h from its start, RUNTIME_ACTIVATION_HARD_WALL), where the run
ends with a fate naming the quota and its expected reset. The durable
deadline continuation re-arms at the moved deadline for a live activation
and closes a run with none (a crash still closes it at boot). A park
re-reads the run's model from the computer's policy every 30 s and at each
wake; a role whose spec changed ends the park on the new model.

Policy re-read is strict (review r1 Devin residual; owner rule, a pinned
model fails rather than falls back): a park moves a run only when the policy
file was read and parsed and names another model for the role
(modelpolicy.ResolveStrict, which neither falls back nor writes the file); a
missing, unreadable or unparsable file, a bad overlay, or an unknown pool
keeps the run on its model and logs once per distinct error.

## Superseding ruling: switch, never pause (owner, 2026-11-04, about 18:30Z)

Owner, verbatim: "On quota issues, we shouldn't stop but switch models.
Always continue, never pause." This replaces the park above and the
"pinned model fails rather than falls back" rule for quota (a non-quota
failure of a pin or a NoFailover pool still fails; the Claude delegate is
not in any chain, it is an engineering-only tool layer).

### Behaviour

On a typed quota answer the tool loop marks the model exhausted in the
computer's quota ledger until its reset (two minutes when the answer names
no reset time) and switches, with no sleep, to the first model after it that
the ledger does not hold out:

1. the rest of the role's own pool or fallbacks, in order;
2. the members it left earlier in the run (a reset may have brought one back)
   and its first model;
3. the computer-wide continuation chain (`modelpolicy.Manager.ContinuationChain`):
   the catalogued models the computer's own policy names (defaults, then
   roles, pools expanded), then the default pool. It adds nothing a computer
   may not already call; uncatalogued ids are dropped; the Claude delegate is
   not in the catalog and so never appears.

A model the ledger holds exhausted is skipped, including as the run's first
model (reason `known_exhausted`), until its reset time passes. The ledger is
shared by the computer's runs, so one run's finding spares the others the
doomed call. Each switch is a `run.retry` event named `model_switch`
(`reason` provider_quota, known_exhausted or provider_quota_retry;
`from_provider`, `from_model`, `to_provider`, `to_model`,
`from_quota_resets_at`, `provider_error`). Nothing waits.

### Every model exhausted at once: bounded retry plus a loud incident

This is the only wait left. Choice: retry the whole chain (the ledger is
forgotten, so the gateway is asked again) after a short backoff of 15 s, 30 s,
1 min, 2 min, 4 min, 5 min, 5 min (cut short when the earliest known reset is
sooner), about 18 minutes in all. Then the run ends with an error that names
the quota, which closes it with a visible fate; it is not a silent pause and
the fate is not "interrupted". Why a bounded retry and not an unbounded one:
if every model on the computer is out of quota there is nothing to switch to,
and an unbounded loop is the park under another name that also hides the
dead state; why not fail at once: a chain that is exhausted for seconds
(concurrency 429s that were typed quota, a cooldown that ends) should not
cost the work. While it lasts the incident is a typed `provider_quota`
wait: on the obligations surface (`provider_waits`), in every desk's pack
(Texture's included), as the assignment's wait for an engineering run
(management's pack, Texture's relay after a minute), and as `run.retry`
events `provider_quota_incident` (per round) and
`provider_quota_incident_cleared`.

### Deleted

`toolregistry.WithProviderWait`, `waitQuotaPark`, `providerQuotaParkDelay`,
the park delays and the 30 s policy re-read (`WithModelReselect`,
`modelReselectForRun`, `modelpolicy.ResolveStrict`), the quota's use of the
activation deadline's suspend/resume, the announce timer and the park
sequencing in the tracker. The tracker stays, slimmed, for the incident
only. The known gap above (non-engineering parks not announced to Texture;
concurrent research children corrupting the parent's activation deadline) is
gone with the parks: a quota never touches the deadline.

### Restart rules

Unchanged: the ledger and the incident are process memory; a crash closes the
run at boot. No resumer, no switch.
