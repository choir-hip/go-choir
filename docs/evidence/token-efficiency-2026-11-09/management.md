# Token efficiency audit, slice "management" (the management desk), 2026-11-09

Read-only. Sources: d@e and paper `GET /api/costs?detail=1`, `/api/runs`, `/api/runs/{id}/events`, paper
`/api/runtime/obligations`, Node B gateway journal (metadata lines only), and code on origin/main
(`a5103d5a0`). Raw pulls are in `scratchpad/eff/m/` (de_costs.json, paper_costs.json, ev_*.json, gw30.txt, p_runs.json, p_obl.json).
Every number is tagged (M) measured, (I) inferred, (H) hypothesis.

Scope note: "management desk" here is the persistent management agent (role `management`). The intake
`management_task` runs (labelled desk "management" in /api/costs) are covered in 1.4 because the brief lists them, but
the intake lever is only sketched.

## 0. Bottom line

1. Once the wedge is gone, the management desk is a small spender on the paper (a healthy activation measured on d@e
   is ~90k input tokens, 6 model calls; the paper's expected wake rate is a few per hour (H)). The paper's management
   problem is not a cost-per-activation problem. It is a missing-circuit-breaker problem: one wake was retried 175 to 183
   times over six days and each retry was a full LLM activation.
2. 66 to 83% of every healthy management activation is the 12k-token system prompt resent on each call (M). 0% cached
   on crouton-alpha (M). Output is 0.8% of input (M).
3. Most of what the desk does per wake is not judgment: it reads its inbox (always call 1), looks up verb signatures with
   `choir.Help` (12% of tokens), fumbles Go field names (compile errors), and spends a whole extra call just to say "done"
   (15% of tokens). All of that is removable with no quality risk.
4. The routine dispositions (clean task reports, residual captures, no-visible-change rebase shots, platform-refused
   Verify) can be host rules, with the LLM kept for exceptions. That is the 10x to 50x shape, in section 3.

## 1. What this slice spends

### 1.1 Prompt per call (M)

- Management system prompt is `system_chars=49908` (about 12.3k tokens) on the gateway: 785 requests in the 3h window,
  messages=2 on first calls, matching the 12.0k to 12.7k `input_tokens` of every first call in d@e
  (`loop.progress` iteration 1: 12009, 11 to 12k across 33 runs). Earlier builds: 49317 and 49612 chars.
  Command: `ssh node-b "sudo journalctl -u go-choir-gateway --since '-3h' --no-pager | grep 'inference request from autoputer vm-1669'"`, then group by `system_chars`.
- What it is made of (M for files, H for the remainder): `rlm_management_runtime.yaml` 26,772 chars (engineering casts
  with predictions, screens judgement with six questions, shot flow, install-review flow, verify, living stories,
  intake labels, event records, delegation), `management.yaml` 6,397, `core.yaml` 1,261, run_context, temporal_context
  and peer_coordination 1,922. That is about 36k chars; the other ~13k chars is the generated verb list and skill
  context (`tool_profiles.go` ~200-260: core + role + skillContext + overlay + hidden-verb filtering). The prompt is
  identical for every computer: the paper never casts engineering or judges shots, yet pays for ~70% of the text
  that is about them (H on the 70%; the sections are identified by file reading).
- One tool: `desk_go_eval` (`tools=1`). Tool schema is negligible.
- The wake turn is a pointer, not a payload: ~870 chars ("Your inbox has new records: read them with choir.Records()...").
  The records and updates ride the cell frame, not the chat (`buildCoagentUpdateUserMessages`).

### 1.2 A healthy activation (d@e, M)

d@e window 08:36 to 19:07 UTC, 32 management runs with >5k-token calls (`/api/costs?detail=1&limit=3000`, filter
`AgentID` prefix `management:`):

| era | runs | calls | input tokens | per run | calls/run | share of tokens that is the first-call prefix | output/input |
|---|---|---|---|---|---|---|---|
| late, after 14:00 (healthy) | 19 | 118 | 1.70M | 89k | 6.2 | 83% | 0.76% |
| early (Verify refusal loop, 09:52-10:29, plus others) | 13 | 197 | 3.99M | 307k | 15.2 | 59% | 0.85% |
| all | 32 | 315 | 5.69M | 178k | 9.8 | 66% | 0.82% |

What the 331 calls of the 34 sampled runs did (events dump, calls paired with the following `tool.result`; categories by
the cell source; M):

| what the call did | calls | share of calls | share of input tokens |
|---|---|---|---|
| ended the turn, no tool call (a call just to say done) | 53 | 16% | 15% |
| read cells: `Pack`, `Context`, `Updates`, `CellOut` | 47 + 13 errored | 18% | 18% |
| act attempts (`Verify`, `ReportPacket`, `Cast`, `ShotVerdict`, ...), succeeded | 44 | 13% | 14% |
| act attempts that were refused or errored | 29 | 9% | 11% |
| `choir.Help(...)` signature lookups | 38 + 1 | 12% | 12% |
| other cells (ProductAPI probes, loops) | 34 + 11 errored | 14% | 15% |
| first `choir.Records()` of the run | 32 + 2 | 10% | 7% |
| screens and shot waits | 23 + 4 | 8% | 7% |

Only the 44 succeeded acts (13% of calls) are "the work". The rest is discovery, retry, and closing.

Example, run f3f517e2 (a clean-rebase shot verdict, the cheapest realistic job, 9 calls, 116,512 tokens): call 1
`Records()`; call 2 `ShotStart` on an operation that was already applied (refused: "only an accepted platform operation
owes shots"); call 3 `ShotStart` on the right one; calls 4 and 5 `ShotWait(..., 60)` (60 s and 44 s of wall time, each a full
13k-token model call); call 6 `Help("ShotVerdict")`; call 7 the verdict; then the run was passivated on a restart and call
8 re-read `Records()` (13.3k) before call 9 ended. The before and after screens were byte-identical in the findings the
model wrote ("both identical loading screens"). 4 of 9 calls bought no information.

Per-activation floor: a call is ~12.0k (prefix) + 0.1 to 1k of new content. A 6-call activation therefore pays the prefix
6 times; the novel content is about 5% of what is sent.

### 1.3 Who wakes management on d@e (M, crude)

Of the 49 d@e management runs since 06:00 UTC, by result text: 21 are verify/shot related and hold 4.38M of the 5.69M
tokens (77%); 14 end with a "waiting for engineering / relayed progress / round held open" result and no decision (322k
tokens, 6%; keyword heuristic, treat as +/-30%). d@e is self-development, not the paper: this is the cost of supervising
engineering, and engineering itself is 83% of d@e's input (34.9M of 42.2M). That is another slice.

The Verify refusal loop (M): six consecutive activations (09:52 to 10:29: runs 26f8bd63, f0f67c6d, 1088787a, 592cdc03, 0d9e181f,
707a8bfe, plus 726f7bbd) each re-tried `choir.Verify(... "pass")` and were refused with the same platform string,
"legacy screens bundle; shots are taken after the host build". 134 calls, 2.9M tokens, 51% of d@e's whole management spend
on 2026-11-09, to learn the same refusal seven times. f0f67c6d alone: 43 calls, 1.0M tokens, 3 cells with Go compile or
reflection errors (`p.Acts`, `p.Text`, `reflect: call of reflect.Value.Field on zero Value`), 2 `ProductAPI` argument
type errors (`cannot convert untyped string to []uint8`).

### 1.4 The paper (M unless tagged)

- Management desk in `/api/costs` (1.5 h window 17:46 to 19:17 UTC, 2417 calls): 230 calls and 18.46M input tokens in 4 runs
  (a11122b6 94 calls 7.64M, a475f115 62 calls 5.14M, 96a1e79a 62 calls 5.16M, 50fd0c3c 12 calls 0.52M); 2 of every 5 input tokens
  of the paper were management and none produced an act. Mean 80k input per wedge call.
- How long the wedge ran (M for counts, I for tokens): gateway requests carrying the management system prompt
  (system_chars in {49317, 49612, 49908}) over the last 30 h number 9,086, every hour from 2026-11-08 13:00 UTC to 19:18
  UTC on 11-09, 85 to 715 per hour, mean 293 per hour. At the measured 80k mean input per wedge call that is ~730M input
  tokens in 30 h, ~24M per hour, in line with the 25M per hour in the token-budget doc. (The 30 h is the journal's
  retention window, not the wedge's start.)
- The obligation table says the wedge is older (M): `/api/runtime/obligations` on the paper: 1000 actor updates due, oldest
  `2026-11-03T10:36:40Z`, `by_kind`: 827 `cell_terminal_deadline`, 167 `inbox_record`, 2 `initial_dispatch`, 2
  `fresh_mint_management_resume_deadline`, 2 `activation_budget_deadline`. The samples show `attempts: 175` to `183` on the same
  wakes. Each attempt starts a management run; 8 management runs in the 500-run list were all `cancelled interrupted_by_restart`
  at 19:21 UTC. If attempts were all ~80k x 60 calls the 6.4 days cost on the order of 1.5 to 4 billion input tokens (I, very
  rough; only the last 30 h are measured).
- Why the packet is big (M/I): the final-checkpoint packet has one `update_refs` entry (~140 tokens: update_id, from_agent_id,
  from_role, channel_id, seq, kind) and one id stub per pending update, limit 100 (`pendingCoagentUpdatesForRun(..., 100)`).
  With 41 unread updates that is ~5.7k tokens per injection, matching the measured +5.65k to 5.85k per call. So the wedge
  per-call growth is ~140 tokens x number of unread updates (I; consistent with the doc).
- Management has 0 prompt-cache hits on crouton-alpha (M, `cache_hit_rate: 0`), and 3% on d@e (173,568 cached of 5.69M).
- Intake `management_task` (M): 48 runs, 465 calls, 3.39M input tokens in 1.5 h (2.26M per hour; ~32 tasks per hour), 70.5k
  tokens and 9.7 calls per task. System prompt 11,986 chars (3k tokens), so ~41% of each task is prompt resend. Runs started by the host
  (`withIntakeTask` in `arrival_task.go`, so the LLM never routes the arrival). At 174 leased captures in 2 h the task spends
  ~39k tokens per capture to do what Jev already labelled.
- Management LLM activations per published story on the paper: 0 on the intended path. A captures arrival is
  committed and its intake task minted in one host batch (`withIntakeTask`); a clean task Report "waits for your next turn"
  and wakes no one; only residuals, `official_single_outlet` event records, and operator notices wake management. Number of
  such wakes per hour cannot be measured while the desk is wedged (all 8 recent management runs are the wedge's). So the
  honest answer today is "unmeasured; expected small", with per-story management spend currently ~0 on the clean path and
  ~1.2M tokens per story of wedge (50.8M per 2 h / ~85 stories moved, token-budget doc).

### 1.5 Compaction, passivation, wake cadence

- Compaction (M): fires at the 140k-token threshold; seen only inside the wedge (tokens_before 115k to 119k, summary 1.5k
  to 2k output). A healthy activation never exceeds 27k input (max across 32 runs), so compaction is not a cost of a healthy
  desk. Cost of one compaction in the wedge = one 115k-token call, ~2.4M tokens per two-minute cycle (doc).
- Passivation (M, one example): run f3f517e2 shows `activation.passivated {"recovery": "passivated_on_restart"}` and a second leg
  that re-sends the 18-message transcript (13.35k) and re-runs `choir.Records()`: ~27k extra tokens for a restart. Cheap when
  rare; it matters only because deploys restart the paper often (the 19:21 restart cancelled 8 management, 6 + 2 task, 9 research, 12
  texture runs in one sweep).
- Wake cadence (M): management gets one `inbox_record` wake per record batch. On d@e that is ~49 runs in 12.5 h (4 per hour),
  each a fresh run (messages=2 at start) with a continuing Go session (cells keep variables). The paper's 8 runs in 2 h are
  retries of the same stuck wake.
- The desk cell frame is rebuilt and re-marshalled for every cell (`EvalFrame`: `json.Marshal(frame)`, frame = Inbox, Doc,
  Pack, Updates, Records, `desk_session_worker.go:458`). Records and Updates are unbounded in bytes (Updates limited to 100
  items but each is a full packet with `projectTerminalOutcomeContent`). A 5.5 MB to 8.6 MB frame is therefore re-built
  and re-sent to the worker for each cell. This is CPU and memory, not model tokens, but it is the mechanism of the "frame
  > 4 MiB" wedge, and it implies the frame should carry handles and counts, not bodies (H on the content mix; the cause of
  the growth is the unread backlog, which the paper had because management never ran).

## 2. Waste found

W1. Retry without a verdict. One wake retried 175 to 183 times (M, obligations samples), each a full LLM activation, with no
rule that says "this wake has failed to commit any act N times, close it with a visible fate". CLAUDE.md already forbids
resuming after a crash; this is the sibling failure: retrying after a non-crash failure. Trace: `/api/runtime/obligations`
`actors.samples[].attempts`.

W2. Re-injection of a packet the model already saw. Known (token-budget doc), here is a likely mechanism (H): the "seen"
check `lifecycleInjectionIDsFromRunMemory` accepts a runtime-injected packet only if its envelope's `trajectory_id` equals
`lifecycleControlTrajectoryForRun(rec)` and the target agent matches. For a non-lifecycle management run
(`request_source: inbox_record`, M from p_runs.json metadata) the pending list comes from
`ListCoagentMailboxBacklog`, and the packet's `trajectory_id` is copied from the first update
(`buildCoagentUpdateUserMessages`: `packet.TrajectoryID = update.TrajectoryID`), while the run's trajectory is its own run id
(5136d197 equals its `trajectory_id`, M). If those differ, every packet is "unseen" and the final-checkpoint injector
returns it again after every end_turn forever. Probe: read run-memory entries of a wedge run; compare the packet's
`trajectory_id` with the run's. This is root cause beyond "needs a bound", and a bound alone leaves a once-per-activation
re-injection.

W3. The same ~12k-token system prompt resent on every call, uncached (M): 66% of d@e management tokens, 83% in healthy runs.

W4. A call whose only job is to end (M): 53 of 331 calls, 15% of input tokens. In the cell carrier a terminal act
(`ReportPacket` with a final disposition, `Verify`, `ShotVerdict`, `Cast`) already commits; the model then needs another
call to say it is done. `WithTerminalToolResult("desk_go_eval", ...)` exists for Texture (`runtime.go:4253`); management has no
equivalent. (Management must keep going when it has more records; a terminal act could end the run only when the inbox has
nothing unread.)

W5. First call is always `choir.Records()` (M: 32 of 34 runs; all others are `Records()` loops). The wake turn already knows
the inbox is non-empty; preloading the records (cut) into the first user turn removes the call.

W6. `choir.Help(...)` lookups and API guesswork (M): 38 Help calls (12% of tokens) mostly for `Verify`, `VerifyFindings`,
`ReportPacket`, `ShotVerdict`, `Screens`; 29 refused act attempts (11%); compile errors from guessing struct fields
(`p.Acts`, `p.Text`, `p.Items`), `ProductAPI` arg types. The 27k-char overlay describes the verbs in prose but not the Go
signatures; the model asks the REPL. A 1-line signature for the ~8 verbs used would cost ~1k chars (250 tokens per call) against
~15k tokens per lookup.

W7. Waiting by polling in model calls (M): `ShotWait(job, 60)` is a host call that blocks up to 60 s; each time it returns
"running" the model is called again (13k tokens) to issue the same wait. In f3f517e2 two of nine calls. Wait should be a
platform wake on job done (the pack already has a "Shots owed" wait view).

W8. Deterministic platform refusals that wake the LLM again (M): the Verify "legacy screens bundle" refusal loop (7
activations, 2.9M tokens, see 1.3). The platform knows the Verify cannot succeed; the wake is a `verify_owed` notice
re-minted every time (`handleVerifyOwed`). A refusal that cannot change is a state, not a prompt.

W9. Activations whose outcome is "no decision, waiting" (M heuristic, 14 of 49 d@e runs, ~6% of tokens). Each engineering
progress report wakes a 12k-token model to "relay status" or wait.

W10. A prompt that serves every computer's maximum duty (M/H): engineering casts with predictions, six-question screen
judging, install review, shot flow are in the paper's management prompt. The paper does intake, events and operator notices.

W11. Unbounded frames and per-cell re-marshalling (see 1.5).

W12. Passivation resume re-reads Records (M, 1 example, 27k tokens): the resumed leg could be handed what the first leg
already read.

## 3. Ranked levers

Savings are of this slice's spend. "Slice" in the paper = the management desk; healthy-state share of the paper is small, so
the paper-total column is split into "while wedged" (the 51% already being fixed) and "healthy steady state" (H).

| # | lever | slice saving | paper total | quality risk | size | evidence |
|---|---|---|---|---|---|---|
| L1 | Wake retry budget + park: a management wake that has produced no committed act in 2 activations is closed with the visible fate "no_progress: <reason>" and the backlog shown to the owner/operator; never retried automatically. Plus per-activation caps (calls and input tokens) that end the run with a fate when the same last-user hash repeats | caps worst case: a wedge costs <=2 activations (~0.2M) instead of ~25M/h | removes the recurrence of the 45-51%; wedge-class bound | none (a stuck wake needs an operator; today it gets 175 blind retries) | small to medium (wake dispatcher + toolloop counter) | W1, 1.4 obligations |
| L2 | Fix the seen-check for mailbox-backlog packets (compare ids, not trajectory) so a final checkpoint never re-injects a delivered packet | ~15% of a long activation; kills the amplifier | with L1, makes the wedge impossible | none | small | W2 (H until the probe) |
| L3 | Preload the wake: put `Records()` (cut to N items), the update stubs and, for owed-verdict records, the evidence packet in the first user turn; end the activation when a final act commits and no records remain unread | removes first-record call (7%), most read/Pack/CellOut cells (~15%), the closing call (15%): 35 to 40% of tokens, 3 to 4 calls of 6 | healthy-state paper management <1%, so ~0.3% | low (same information, one fewer round trip); watch that long records are cut with a handle | medium | 1.2 table, W4, W5 |
| L4 | Verb signatures in the prompt (the ~8 used ones); Help only for the rare verb | 12% (Help) + a third of the 11% refused attempts + compile errors: 15 to 18% | ~0.2% | none | small | W6 |
| L5 | Prompt by capability: compose management's prompt from the packages the computer has (paper: intake, stories, events, decline, delegate, notices; d@e adds engineering, shots, install review). 49.9k chars to ~15k for the paper | 12.3k to ~4k tokens per call, 55 to 65% of a healthy paper activation's tokens | ~0.4% healthy; matters mostly if activations grow | medium: a desk loses affordances it was never using; needs the modular package registry to say which overlay belongs to which package | medium (package desks plan) | 1.1 |
| L6 | Prefix caching (provider choice or cache key) | prompt is 66 to 83% of tokens, so up to ~60 to 75% of fresh input at 90% hit | same fraction of management's share; systemic, owned by another slice | none | provider-side; unknown whether crouton-alpha supports it | 1.2, cache_hit_rate 0 (M) |
| L7 | Host rules for routine dispositions instead of LLM turns (section 4): clean task reports auto-ack; residual captures re-leased once then declined; no-visible-change rebase shots (identical before/after hash) auto-pass; platform-refused Verify becomes a state | each removes whole activations: on d@e ~20% of healthy tokens (rebase shots) + 51% (refusal loop); on the paper the residual path is 100% of its healthy management work | healthy paper management goes to ~0 | low for acks and identical-hash shots; medium for residual decline; needs replay | medium | W7, W8, 1.3 |
| L8 | Replace polling waits with a wake on job completion (`ShotWait` returns when done, the platform wakes the desk) | 2 calls of 9 in shot runs; ~20% of a shot activation | ~0 | none | small to medium | W7 |
| L9 | One structured call instead of a REPL for the jobs that are one decision (shot verdict, verify, install verdict, cast with predictions): preloaded evidence, a JSON schema output, platform commits | 6 calls to 1; 6x on those activations | ~0 | medium (decision quality should be checked against the REPL's) | medium | 1.2; the REPL exists for exploratory work these jobs do not need |
| L10 | Pointer frames: Records/Updates bodies behind handles, counts in the frame (permanent bound on frame size, no re-marshalling of MB per cell) | not model tokens; prevents the next frame-limit wedge | prevents recurrence | low | medium | 1.5 |
| L11 | Intake: replace the 9.7-call task with a per-capture Jev disposition (see 5) | 2.26M/h to ~0.2M/h (90% of intake), measured base | ~4% of the paper's 49.5M/h (intake is 7%, 6.6M per 2 h) | medium; replay 79 runs first | medium | 1.4 |

Ordering by value of effort: L1+L2 (wedge class, small), L3+L4 (cheapest real cut to a healthy activation), L7 (removes the
decisions the model should never see), L5 (needs the package registry), L6 (systemic).

### A 10x and 50x management loop

10x (about 9k tokens per activation, 1 to 2 calls): L3+L4+L5. The wake carries its records, signatures are in the prompt, the
prompt is capability-sized (~4k tokens), the terminal act ends the run. Healthy d@e activation 89k -> ~10k. No new
mechanism, only wiring and prompt work.

50x and beyond (most activations never reach a model): management becomes a deterministic reducer over the commitment
ledger, with the LLM as the exception handler.
- Clean task Report: the host marks it read and credits the ledger. Zero tokens. (Today it waits in the inbox and is the main
  growth path of the frame, H.)
- Residual captures: the host re-leases once as a new intake task (it already has `Delegate` semantics) and then
  declines the second residual with the task's own reason. Zero tokens.
- `official_single_outlet` records (event layer): host casts a research "second source" request from a template, or declines
  after the aged horizon. Zero tokens, or one Jev score.
- Shots owed: identical before/after content hash = pass with the findings the host can compute; otherwise one vision call
  with a schema (q1..q6 per pair). 1 call.
- Verify owed: a candidate that the platform will refuse is closed by the platform with the refusal as its reason.
- Engineering progress reports: relayed to Texture by the host, not summarised by a model (W9).
- What stays LLM: casting engineering with predictions, judging a candidate that has visible change, install review, anything
  the host reducer marks `unclassified`.
Result: activations per hour for the paper become the exception count; the per-activation cost is the L9 single-call cost
(~15k).

## 4. Candidate host rules, with the evidence each needs before it ships

| rule | replaces | evidence to check first |
|---|---|---|
| Auto-ack clean task Reports | the model never decides on them (prompt says "a clean Report waits for your next turn") | count of task Reports with no residuals vs all (paper: all 90 recent tasks have no "residual" text in `result`; M from p_runs.json) |
| Re-lease residuals once, then decline | model decides feed/decline/open on leftovers | residual rate per hour once the desk runs (unmeasured) |
| Identical-hash shot pair = pass | LLM judging a "loading screen" pair | 3 of 19 healthy d@e runs (ae6481a7, f3f517e2 both clean-rebases; 228ef446) |
| Verify refused by platform = fate, no re-wake | 7 identical refusal activations | `handleVerifyOwed` and the "legacy screens bundle" string |
| Wake closes after 2 no-act activations | 175+ retries | obligations attempts field |

## 5. Jev / classifier opportunities in this slice

Jev = TypeSafe Decisions via OpenRouter, $0.042 per M input, output free, parallel batches.

1. Capture disposition (feed to story / open story / decline): management_task today, ~32 tasks and ~87 leased captures per
   hour (174 per 2 h, token-budget doc), 2.26M tokens per hour (M). It is already a Jev-shaped label: `r.Labels[id]` has
   Says, Story as texture:<doc>, SameEventAs, TopicStory, and the prompt says to follow the label "unless you read
   otherwise". Shape: one batch per arrival, up to 8 parallel calls (one per capture), each 1.4k to 2.3k input tokens
   (measured Jev call size in the token doc) with a story shortlist from `intake_labels.go` (`intakeOverlapPicks`). 87 x 2k = 0.17M
   per hour, ~12x less than 2.26M. The model keeps only the "open a new story" case (headline plus hypotheses), which
   needs authoring. Probe: replay the 79 runs' dispositions against the labels (token doc L5 says the same).
2. Residual handling and `official_single_outlet` (ask research for a second source vs decline): a binary label on
   (outlet reputation, age, number of origins). Count unknown (the desk has not run). One Jev call each.
3. Notice triage ("closure", "cost-anomaly closure notice", `owner_act`, `unattended_request`): "does this require an act?".
   Today every such record wakes a 12k-prompt LLM (the 14 no-decision d@e wakes, 322k tokens). One Jev call per record
   at ~1k tokens, parallel; keep the LLM only for "needs act".
4. `p_success` and `weight` stated fields on every Cast, Ask, Escalate: the model invents a number each time. These are
   scored later. A base-rate lookup by request kind would match the information content; keep model-stated values only where
   they deviate. (H; verify against the calibration digest before touching it.)
5. Shot verdict q1..q6: these are visual yes/no questions: a vision classifier, not Jev as described, but the same shape (one
   call per pair, parallel). The host already has the pair hashes; identical hash is a pass.
6. Verify pass/fail: needs reading the engineer's report and screens; do not move to Jev without a quality probe.

## 6. Open questions and probes

1. Does the seen-check mismatch (W2) explain the loop? Probe: read run-memory entries of paper run 50fd0c3c or
   a475f115 between two consecutive calls: compare the last user message's embedded `trajectory_id`/`target_run_id` against the
   run's. Or a unit run of `lifecycleInjectionIDsFromRunMemory` with a backlog packet whose update has a different trajectory.
2. What are the 41 unread updates / 167 inbox_record wakes (task Reports? captures? notices?). Probe: after the fix lets the
   desk read them, `choir.Records()` counts by kind; or the read-only inbox listing route if one exists (I did not find a
   route; `/api/management/tasks` returns "api route not found").
3. Healthy management wake rate on the paper (per hour) and its token cost. Probe: the first 6 hours after the wedge fix,
   `/api/costs?detail=1` filtered to `management:` runs; count runs per hour and `kind` of the first `Records()` result.
4. Is the wedge fixed already? The paper had no gateway requests of any kind after 19:18 UTC and its 8 management runs
   were cancelled at 19:21 (restart), so a deploy probably landed; re-read `/api/costs` in an hour to see whether
   management returns at ~100k tokens per activation.
5. Is the remaining ~13k chars of the management system prompt the verb list and skill context (H)? Probe: render
   `buildSystemPrompt` for profile management in a test and print section sizes.
6. Does crouton-alpha support prefix caching at all? (Not tested; the token doc's two-request experiment.)
7. How much of the prompt can be dropped for the paper without breaking behavior? Needs a replay of captured wakes against a
   capability-composed prompt.
8. Pre-wedge baseline: none is available in the 30 h journal window; the earliest management activity that is not the wedge
   is on d@e only.
