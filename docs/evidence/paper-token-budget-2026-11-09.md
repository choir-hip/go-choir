# Where the Autopaper's tokens go, 2026-11-09

Mutation class: green (evidence doc, no runtime change). Rollback: revert the
commit. Everything here was read-only: the gateway journal on Node B, the
paper's own read-only API through the authenticated GET helper, and the public
front-page endpoint. Nothing was posted to the paper and no secret was read.

Question (owner, 2026-11-09): can the paper (computer
`computer-1355b83bced1613eb048ee28d66688a8`, VM `vm-1669020adcece51c2f5ff9b6df399ef5`)
produce the same or more output with 50%, 10% or 2% of the tokens?

## Short answer

About half of the paper's input tokens in the measured window were spent by one
wedged loop that produced nothing: the management (conductor) desk. Its desk
cell cannot start, because the frame it must hand the cell is 5.5 to 8.6 MB and
the limit is 4 MiB, so every `choir.Records()` fails. The model then "ends its
turn", the runtime re-delivers a final-checkpoint packet of about 5.5k tokens,
the model acknowledges it in about 200 tokens, and this repeats until the
transcript hits the 140k compaction threshold, then again, until providers run
out of quota. Stopping that loop is a 50% cut with no quality risk. That is the
50% answer.

The other half is research (30% of the total), Texture wakes (12%) and
management-task intake (7%). Their cost is agent-loop shaped: a transcript that
is resent on every turn, a fixed 3k to 12k token system prompt resent on every
call, and no prompt-cache hits at all on the model now serving them
(crouton-alpha reports cached=0 on all 3898 calls). A 10x cut (10%) needs the
wedge fixed plus caching or transcript trimming plus fewer research turns. A 50x
cut (2%) is a redesign, not a tuning pass.

## 1. Measured breakdown

All times are UTC. The gateway window is 16:29:47 to 18:29:45 on 2026-11-09 (two
hours). The paper-API window for run data is 16:33 to 18:31, the 500 most recent
runs the list endpoint will return.

### 1.1 Total, by gateway (measured)

Command:

    ssh node-b "sudo journalctl -u go-choir-gateway --since '-2h' --no-pager" > gw.log
    grep 'inference succeeded for autoputer vm-1669' gw.log   # tokens=IN+OUT cached=C

| quantity (paper VM only) | value |
|---|---|
| successful calls | 3749 (3720 openrouter crouton-alpha, 29 synthetic) |
| input tokens | 98.9M (95.6M openrouter, 3.4M synthetic) |
| output tokens | 0.97M, i.e. 1.0% of input |
| per hour | about 1875 calls and 49.5M input tokens |
| cached share | 0% on openrouter (all 3720), 77% on the 29 synthetic calls |
| failed requests (no tokens spent) | 1233 provider failures of 4931 requests: 429 concurrency, provider quota exhausted, circuit open |

The second VM (`vm-70070cf7...`) added 189 calls and 8.7M tokens in the same
window and is not counted above.

Input tokens per call (openrouter, 3720 calls): p10 4.9k, p50 16.2k, mean 25.7k,
p90 59.4k, p99 137.8k, max 150.9k. The tail matters: 427 calls (11%) were 50k or
larger and carried 40.7M tokens (43% of all input). Those are the wedged
management loop (section 1.3).

Whole day so far (00:00 to 18:40 UTC, paper VM, same journal command with
`--since 2026-11-09 00:00:00`): 38,315 calls, 1,206.8M input tokens, 64.5M per
hour, 45.6% cached overall. Cache hit rate by provider: synthetic 87%,
opencode-go 85%, openrouter 2%. By hour the paper ran 70 to 86M input tokens an
hour with 50 to 80% cached until 13:00, and 22 to 51M an hour with 0 to 13%
cached since 13:00, after the routing change to crouton-alpha. Fresh (uncached)
input per hour went up, from roughly 16 to 40M before to 41 to 51M since.

### 1.2 By role, from run events (measured)

Source: `GET /api/runs?limit=500` for the role and state of each run, then
`GET /api/runs/{id}/events` for every run, summing the `loop.progress` events
(`input_tokens`, `output_tokens`, `iteration`). Gateway tokens equal event
tokens call by call: 2994 of 3749 gateway calls matched an event on the exact
(input, output) pair, 48.2M tokens.

| role | runs | calls | input tokens | share of 98.9M | calls per run (mean, median, p90, max) | input per call (p50, mean, p90) |
|---|---|---|---|---|---|---|
| management (conductor), not in the run list | 4 activations | 755 | 50.8M | 51% | 55 to 94 each | 11k to 148k, growing |
| research | 73 | 1336 | 29.9M | 30% | 18.3, 18.5, 26, 38 | 21.5k, 22.4k, 34.5k |
| texture | 307 | 908 | 12.6M | 13% | 2.9, 2, 6, 14 | 11.1k, 13.9k, 25.3k |
| management_task (intake) | 79 | 793 | 6.6M | 7% | 9.5, 9, 17, 24 | 7.5k, 8.4k, 14.4k |
| Jev and system1 judgments (TypeSafe) | n/a | about 250 per 73 min | about 0.47M per 73 min | about 1% | | 1.4k to 2.3k |

The event sum for research, texture and management_task is 49.1M, a little
above the 48.2M the gateway matched, because a few runs straddle the window
edge. The management row is the 755 gateway calls that match no event in any
listed run (50.8M tokens); the identification is in 1.3. Cross-check on a
different window: `GET /api/costs?detail=1&limit=10000` covers 17:29 to 18:42
(2451 calls) and attributes 25.8M of 53.2M crouton input tokens (48.6%) to the
management desk, in four runs, with 57 management-task runs making up the rest
of the "management" desk label.

Tokens by hour in the gateway window: 16:xx (31 min) 29.2M, 17:xx 50.9M, 18:xx
(30 min) 18.9M.

### 1.3 The management wedge (measured, with one inference marked)

The four big management runs are `a11122b6`, `96a1e79a`, `78bec9c9` and
`a655b1c8` (full ids in the costs entries). They are persistent runs created on
2026-11-05, woken from passivation, so they never appear in the recent-runs
list; that is why they were missing from the run-level sums until the costs
route showed them. Event dump of `a11122b6` (94 model calls, 7.64M input tokens
between 18:08:36 and 18:21:42):

- Fifteen of 94 calls are tool calls, all `desk_go_eval`. Every desk cell fails
  immediately: `desk eval write: frame: payload 5489824 bytes exceeds 4194304`.
  The same error, with sizes of 5.49M, 5.55M, 5.59M, 5.60M and later 8.65M
  bytes, appears in every cell of all four runs (16, 14, 25 and, for the newest
  run `a475f115` at 18:41, 6 cells). The frame is growing, about 3 MB between
  17:27 and 18:41.
- The model says so in plain words at every end of turn: the "desk cell
  worker" crash-loops, `choir.Records()` and `choir.Updates()` cannot be read,
  "the same 41-update backlog remains unread and unprocessed, no acts
  committed".
- 79 of 94 calls end with `stop_reason=end_turn` and no tool call. Each one is
  followed by another call. Input grows by a near-constant 5.65k to 5.85k
  tokens per call (for example 99102, 104844, 110521, 116156, 121899, 127539,
  133280, 138940) while output stays near 200 tokens and the message count
  rises by exactly 2 per call.
- The last user message hash is the same across calls: `112581a3...` on 74 of
  94 calls in `a11122b6`, and on 31 of 64, 46 of 68 and 34 of 52 calls in the
  other three runs. The response text on those calls is "Final checkpoint
  acknowledged, no tool call made."
- Compaction fires at the 140k threshold (tokens_before about 115k to 119k,
  summary output about 1.5k to 2k tokens) and the loop immediately climbs again
  from about 31k to 37k, four times in this run (18:13:38, 18:15:45, 18:18:08,
  18:20:10), roughly one cycle every two minutes and 2.4M tokens.
- Providers run out of quota at 18:21 and the run dies at 18:39:53 at "tool
  loop iteration 111: every model is out of quota after 3 retry rounds".

The code path is `internal/toolregistry/toolloop.go` around line 1657: when the
model ends its turn, `injectUserTurns(true)` is called; if it returns any
message, the loop appends the assistant text and the injected turn and
continues, with no counter. The injector
(`coagentUpdateTurnInjectorWithInitialPhase`,
`internal/agentcore/management_controller.go` line 2453) returns the owner
revision turns and any pending update packets not already seen in run memory,
and the preamble for that phase is "Choir coagent update packet (final
checkpoint before ending this activation)" (`coagent_update_packet.go` line
215).

Inference, not yet proved: the message bodies are not in the event stream (the
`messages` field is a count), so that the 5.5k-token message appended on every
call is the same final-checkpoint packet is inferred from the repeated last-user
hash, the constant +2 messages and +5.7k tokens per call, and the model's own
"final checkpoint" text. It would be confirmed by reading run memory entries of
`a11122b6` between two consecutive calls.

The same obligations endpoint shows the standing cost of this: the oldest
unprocessed management update is from 2026-11-03T10:36 with 174 to 182 attempts,
1000 actor updates are due (827 `cell_terminal_deadline`, 167 `inbox_record`),
and `management_tasks` reports 804 queued tasks with the oldest 162,372 seconds
(45 hours) old (`/api/runtime/obligations`, `/api/runtime/observability`). The
management inbox has probably been unreadable since about 2026-11-03; that date
is an inference from the oldest unprocessed update, not a measured start of the
frame overflow.

### 1.4 Turns per run and how input grows with the turn number (measured)

Mean input tokens per call by iteration number:

| iteration | 1 | 2 | 3 | 5 | 8 | 10 | 15 | 20 | 25 | 30 |
|---|---|---|---|---|---|---|---|---|---|---|
| research (73 runs) | 9.2k | 10.0k | 11.7k | 15.0k | 19.1k | 21.8k | 28.1k | 32.3k | 38.9k | 47.5k |
| management_task (79 runs) | 3.0k | 3.6k | 4.9k | 7.6k | 9.3k | 11.4k | 14.2k | 14.8k | | |
| texture (307 runs) | 13.2k | 13.2k | 11.1k | 17.7k | 23.2k | 23.3k | | | | |
| management (wedged) | 12k at turn 3, 5.7k more per turn to about 148k | | | | | | | | | |

Research and management_task grow about 1.2k to 1.4k tokens per iteration, the
size of one capped tool result (search and read outputs are capped near 8.6k
characters, `outlen` 7.4k to 9.5k in the dumped run). The transcript is
append-only inside a run: the system hash is stable (1 hash for research, 1 for
management_task, 2 for texture), so the prefix is cacheable.

Decomposition of each role's input (measured sums with a stated assumption: the
fixed prefix is estimated as system characters divided by 4; system_chars from
the gateway request lines are 11986 for management_task, 17637 for research,
37943 and 49908 for texture):

| role | fixed system prompt x calls | turn-1 context x calls (system + task + first frame) | transcript growth after turn 1 |
|---|---|---|---|
| research | 20% | 41% | 59% |
| management_task | 36% | 36% | 64% |
| texture | 78% | 83% | 17% |

Texture is almost all fixed prompt: 2.9 calls per run, about 10.7k tokens of
system prompt on each, 331 wakes for 144 distinct documents in two hours (2.3
wakes per document, all with intent `integrate_execution_findings`).

### 1.5 What the paper publishes (measured, with sampling caveats)

Source: the public front page, anonymous GET
`https://choir.news/api/platform/publications/front?limit=100` paged by cursor
(`next_cursor`).

- Stories whose latest publication was updated in each hour: 27 to 43, mean
  about 32 an hour, flat all day. 589 distinct stories were updated today by
  18:40, with 1294 total versions (mean 2.2 versions per story, 181 stories
  still at version 1). Section mix of the latest 100: world 59, tech_ai 20,
  business 14, us_politics 7.
- Publication events (a revision that passes the two-outlet rule) are higher
  than the story count, because a story can publish several versions in an
  hour. The paper log (`recent_log` in `/api/runtime/observability`, 512 lines,
  about 4 minutes deep) shows "story publish: ... published vN" 4 times in one
  4-minute window and 5 times across the merged windows I collected (about 17
  minutes with gaps). I therefore bound published versions at about 32 to 60 an
  hour. That is a rough bound, not a count.
- Revisions that are held: across the merged log windows, 13 of 18 publish
  attempts were "not published" (7 cite 0 outlets, 5 cite 1 outlet, 1 disclosure
  refusal). The rule is "a story publishes only when its citations span at least
  2 distinct outlets". Sample is small and not random; treat as a reason to
  measure, not as a rate.
- Texture document revisions created: 271 in two hours (event kind
  `texture.document_revision.created`), about 135 an hour.
- Intake funnel in the two hours: 79 management-task runs leased 174 captures
  (from the run metadata `management_task_leased_count`); in their cells there
  were about 122 story-opening casts, 44 feeds to existing stories and 25
  declines (counted by string match on the cell source, so approximate); 77
  research runs (75 distinct stories); 307 texture runs.

Tokens per published unit, using the two-hour total of 98.9M input:

- per story whose latest version moved in the window (about 85 stories: 36 + 35
  + the 18:xx share): about 1.2M input tokens each;
- per published version (32 to 60 an hour): 0.8M to 1.5M;
- for the whole day: 1,206.8M / 589 stories = 2.05M per story touched.

About half of that, 0.6M to 1.0M per story, is the wedged loop.

## 2. Levers, ranked

Savings are fractions of today's 49.5M input tokens an hour (the current
crouton-alpha regime). They overlap, so they do not add; section 3 gives the
combined picture.

### L1. Stop the wedged management loop. Saves about 45 to 51%. Quality risk: none for the loop, small and good for downstream.

Evidence: 1.3. 50.8M of 98.9M tokens in the gateway window, and 48.6% in the
costs window, produced zero acts. Two defects, both with traces: the desk-cell
frame is over the 4 MiB limit (5.5 to 8.6 MB) so no cell can ever run; and the
final-checkpoint re-injection in the tool loop has no repeat bound, so a
non-progressing packet loops until compaction and then again until providers run
out of quota. The saving is a floor-to-ceiling range because when the management
desk works again it will spend tokens on the real backlog (41 updates in the
inbox, 804 queued tasks); a healthy activation should cost on the order of a
management_task run (about 80k tokens), so I expect that to be a few percent of
today's volume, but that expectation is a hypothesis. Order of fix: the
frame-size defect is the cause, the unbounded re-injection is the amplifier and
would make the next wedge of any kind equally expensive. A bound such as "end the
activation with the visible fate when the same packet hash has been delivered
twice with no committed act" is a loop invariant, not a switch.

Side effect to check, as a hypothesis: while the conductor is down, Texture
rounds it should close stay open, which may be why texture wakes the same
document 2.3 times and why `cell_terminal_deadline` has 827 due updates. If so
fixing the wedge also reduces texture and research volume. Not measured.

### L2. Prompt caching on the model that serves the paper. Saves 60 to 80% of fresh (uncached) input tokens; zero change in raw tokens processed. Quality risk: none.

Evidence: crouton-alpha reported `cached_in=0` on 3898 of 3898 calls
(`cache_reported=true`), while the same paper on synthetic and opencode-go
today cached 87% and 85%. The prefix is stable (1 to 2 system hashes per role,
append-only transcript), so an in-run hit rate near 90% is available on a
provider that caches. The provider layer sends the cache key only to adapters
that document one (`internal/provider/provider.go` line 46: chatgpt and
OpenCode; "Others ignore it"), so openrouter gets none. Unknown, needs one
experiment: whether crouton-alpha or OpenRouter supports prefix caching at all.
The experiment is two identical long requests through the gateway and a look at
`cached_in` on the second; I did not run it. If it does not cache, the lever is
routing to a model that does, which the routing change at 13:00 moved away from
(fresh input rose from about 16 to 40M an hour to 41 to 51M an hour). Whether
the owner's constraint is dollars, quota or concurrency decides how much a
cached token is worth; stealth crouton-alpha is listed at $0 in `/api/costs`,
so here the constraint is the 429 concurrency limit and the free-model quotas
(1233 failures in two hours), where cached tokens may or may not count.

### L3. Research: trim the transcript, bound the turns, stop repeating queries. Saves 12 to 15% of the total (about 40 to 50% of research). Quality risk: medium.

Evidence: research is 30% of tokens at 18 calls and 410k tokens per run (max 38
calls, 1.24M). Transcript growth is 59% of research input (1.4). Counterfactuals
computed from the 1336 recorded calls:

- keep only the last roughly 3k tokens of old tool output (observation
  masking) beyond the turn-1 context: research input falls to 53% of today
  (saves 47%); a 6k allowance leaves 64%; 10k leaves 76%;
- cap research at 15 iterations: keeps 62% (saves 38%); at 20 iterations keeps
  83%; at 10 keeps 36%.

Trace of waste: run `52fb37c2` (38 iterations, 1.24M tokens) researched a local
police shooting; the `SearchStart` queries "SABC News Mdantsane shootout police"
and "Mdantsane two wounded shootout" were issued again at iterations 5, 16, 18,
20, 21, 23 and 24 with minor edits, each returning about 8k characters; two
`choir.CellOut` calls re-read earlier output. The top 10 research runs hold 30%
of research tokens. Duplicate-query suppression within a run (return the earlier
result) costs no quality. A turn cap does risk missing corroboration on hard
stories, and it interacts with the two-outlet publish rule, so cap first on
runs that have already found two outlets. I have not measured how many of the 73
runs ended with the story publishable; that is the missing quality number.

### L4. Texture: fewer wakes and a smaller system prompt. Saves 4 to 8% of the total. Quality risk: low to medium.

Evidence: 1.4. Texture is 13% of tokens, 78% of that is the fixed 38k to 50k
character system prompt resent on each of 2.9 calls per run, and each document
was woken 2.3 times in two hours. Halving the system prompt saves about 39% of
texture (5% of total). Coalescing wakes of the same document inside a window
(one wake per document per round rather than per inbox record) saves a further
third to half of texture, up to 6% of total. Also in the sample, 13 of 18
revision-publish attempts were held for the outlet rule or disclosure; a
pre-write check that a revision has at least two distinct outlets in its
citations before a Texture run is spent on it could remove those runs, but the
sample is too small to size it.

### L5. Intake (management_task): make the join/open/decline decision cheap and take the stale backlog off. Saves 5 to 6% of the total, plus a one-time stale-backlog avoidance. Quality risk: low for stale items, medium for the decision.

Evidence: 79 intake runs, 6.6M tokens, 36k tokens per capture (84k per run, 9.5
calls). The events layer already makes a Jev-class decision per capture at a
tiny price: "event layer: resolved 12 captures in 4 calls: 0 joined, 1 tentative,
11 new events ... $0.00094". The intake task then spends 5 to 12 full-model
cells to look up stories (`choir.Stories` in 193 cells), open (122), feed (44)
or decline (25) the same captures. If open and feed follow the events layer's
verdict by default and the full model is kept only for tentative or declined
items, the loop shrinks to a few calls. That the two decisions agree is a
hypothesis; the check is to replay the 79 runs' dispositions against the event
layer's verdicts. Separately, `management_tasks` reports 804 queued tasks, the
oldest 45 hours old, which at about 84k tokens each is 67M tokens (1.4 hours of
today's paper spending) of work on captures that are no longer news; a queue
age limit costs no quality and prevents the backlog from being paid for.

### L6. Jev for more classification. Saves at most about 7% of tokens, but cheaply in dollars. Quality risk: medium.

Evidence: the judgments that are plainly classification already go to Jev:
`system1`, `support_judgement`, `desk_cast_judge`, `inert_judge` (392 calls,
about 0.75M tokens in the sampled run events, about 1% of the total), the
events layer, intake labels and front importance. What is left is not mostly
classification: research is a search-and-read loop, Texture is generation, and
the conductor is a tool user. The only remaining classification-shaped full-model
work I can point to is the intake disposition (L5, 7%), and, inside research, the
"is this corroborated, should I stop" check, which is not separable in the
traces. At $0.042 per million input tokens even moving all of L5's 3.3M tokens
an hour is about $0.14 an hour, so Jev changes the dollar bill more than the
token count.

### L7. Output tokens and reasoning. Saves under 1%, not a lever.

Output is 1.0% of input tokens (0.97M of 98.9M). Hidden reasoning tokens may not
appear in `tokens_out`; I did not see a field for them in the events, so that
part is unmeasured. Reasoning effort is high on management, research and
management_task and low on texture per the gateway request lines.

### L8. Failed and retried requests. Saves nothing in tokens.

Of 4931 gateway requests, 1233 failed with no tokens charged (251 concurrency
429s on crouton, then 204 flint quota, 176 nemotron quota, 162 synthetic quota,
113 opencode-go circuit-open, 54 chatgpt quota, 53 crouton circuit-open). The
loop walks a fallback chain on each failure. It costs latency and gateway
chatter. The paper runs 12 slots with 4 intake slots; when every model is out
of quota runs sit in retry rounds (an 18-minute bound), which is how the wedged
management run burned wall-clock but not tokens during its last 18 minutes.

## 3. The two or three biggest levers, plainly

1. Fix the management desk-cell frame size and bound the final-checkpoint
   re-injection. This alone is the 50% answer: about half of all tokens in the
   window were spent by a loop that commits nothing.
2. Make the prefix cacheable on whatever serves the paper. The prefix is stable
   and 85 to 90% of every call is repeat, so a provider that caches turns most
   of the remaining volume into cheap or free tokens. Raw tokens processed do
   not change; fresh tokens fall by 60 to 80%.
3. Shrink the research loop (observation masking, duplicate-query suppression,
   an iteration cap that respects the two-outlet rule). Research is 60% of what
   remains after lever 1; masking plus a cap takes about 45% off it.

What each target needs, from the 49.5M an hour baseline (about 1.2M tokens per
story touched):

- 50% (about 25M an hour): lever 1 alone, no quality trade. This is also what a
  healthy management desk should cost plus its real backlog work.
- 10% (about 5M an hour, about 150k tokens per story): lever 1, plus research
  at roughly 40% of today (masking plus cap plus no repeated queries), plus
  Texture wakes halved with a halved prompt, plus intake made mostly
  deterministic. That reaches about 12M an hour raw tokens, so the last factor
  of two to three has to come from caching. So 10x is available as 10x fewer
  fresh tokens, and about 4x fewer raw tokens. Quality risk is concentrated in
  the research cap.
- 2% (about 1M an hour, about 30k tokens per story): about 2 to 3 model calls
  per published version. The paper's work shape today is about 60 calls of 25k
  tokens per story. 2% needs a pipeline rather than agent loops: Jev for
  join/open/decline, one bounded retrieval step feeding one Texture write call,
  and heavy caching. I do not see evidence in these traces that the current
  quality survives that; it is a design question for the owner, not a tuning
  step.

Whether the paper produces "the same or more output" with fewer tokens is the
open quality question. Output is stable at about 32 stories updated an hour all
day regardless of whether the paper spent 86M or 22M tokens an hour (hourly
table, 1.1), and half of the tokens today produce nothing, which says output is
not proportional to tokens. It does not say how much output a research cap
would cost; that number needs the per-run "ended publishable" measurement named
in L3.

## 4. Measured versus inferred

Measured: totals, calls, cache share, per-hour series (gateway journal); role
split, calls per run, input by iteration, and the management loop's per-call
growth, error text, compaction events and last-user-hash repetition (run events
and costs entries); stories and versions per hour (public front endpoint);
queue depth, attempts and ages (observability, obligations).

Inferred or hypothesised, each marked above: that the repeated 5.5k-token
message is the same final-checkpoint packet; the date the management inbox first
became unreadable; that crouton-alpha cannot cache; that the wedge keeps Texture
rounds open and inflates texture wakes; that the events layer and the intake
task agree on dispositions; the published-versions-per-hour bound (32 to 60);
the held-revision rate (small sample); and every saving fraction other than L1,
L3's counterfactuals and L4's prompt-halving, which are computed from recorded
token counts but assume the removed context is not needed.

## 5. Commands

    # gateway (UTC): totals, per hour, per provider, cache share
    ssh node-b "sudo journalctl -u go-choir-gateway --since '2026-11-09 00:00:00' --no-pager | grep 'inference succeeded for autoputer'"
    # request lines carry messages=N tools=1 system_chars=N reasoning=...; failure lines say 'provider call failed'

    # paper VM id: vmctl journal line 'computer=computer-1355b83b... vm=vm-1669020adcece51c2f5ff9b6df399ef5'
    ssh node-b "sudo journalctl -u go-choir-vmctl --since '-6h' --no-pager | grep 1355b83"

    # paper API (authenticated GET helper, read only)
    paper.sh /api/runs?limit=500                       # roles, states, input_tokens per completed run (metadata)
    paper.sh /api/runs/{run_id}/events[?after=SEQ]     # loop.progress: iteration, input_tokens, output_tokens, stop_reason,
                                                       # last_user_sha256, messages (a count), loop.compaction.*, desk.cell errors
    paper.sh '/api/costs?detail=1&limit=10000'         # per-call entries with RunID, AgentID, Desk, tokens (about 73 min)
    paper.sh /api/runtime/observability                # recent_log (about 4 min), management_tasks queue, quota ledger
    paper.sh /api/runtime/obligations                  # due actor updates, oldest unprocessed, attempts, provider waits
    paper.sh '/api/texture/documents?limit=1000'       # capped at 500; created/updated/revision_count

    # public, anonymous
    curl 'https://choir.news/api/platform/publications/front?limit=100&cursor=...'

Method notes. Run ids for the persistent management runs come from the costs
entries with `Desk == "management"` and an `AgentID` that does not start with
`management-task`; their events were paged with `?after=next_after`. Matching
gateway calls to run events used the exact (input, output) token pair, which
left 755 unmatched gateway calls (50.8M tokens) and they sit in the same time
windows and size ramp as the management run's events (for example 18:15:03
99102+207, 18:15:05 104844+9, 18:15:14 110521+100 in both). The counterfactual
caps and masks are in the analysis scripts kept in the session scratchpad
(`an3.py`, `an4.py`), which read the per-run event dumps; they are not committed.

Code pointers: `internal/toolregistry/toolloop.go` (final-checkpoint inject at
line 1657, compaction hooks near line 158), `internal/agentcore/management_controller.go`
(injector at 2453), `internal/agentcore/coagent_update_packet.go`,
`internal/agentcore/run_memory.go` (compaction at 467 and 820),
`internal/provider/provider.go` (cache key comment at line 46),
`internal/gateway/jev.go` (Jev transport and rate scopes), the desk-cell frame
limit message `desk eval write: frame: payload N bytes exceeds 4194304` (write site `internal/yaegikernel/desk_session_worker.go` line 478; the limit is a frame constant in `internal/yaegikernel`, `MaxFramePayload` is the broker-side one).
