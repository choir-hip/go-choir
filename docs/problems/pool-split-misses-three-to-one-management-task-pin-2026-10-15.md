# Pool split misses 3:1: the management task pin sends a fifth of the paper's calls to DeepSeek (2026-10-15)

Status: documented; fix follows in a separate commit.
Mutation class of the fix: orange (provider/model routing). Rollback: git
revert of the fix commit.

## Owner intent

Owner, 2026-10-15 (owner-notes §23): "try to configure 3/4 gpt, with
DeepSeek only for 1/4 of runs". 7b4585cfc made the default pool
gpt-6-luna weight 3 : deepseek-v4.1-flash weight 1.

## Evidence

The paper (vm-1669…) restarted onto 7b4585cfc at 17:10:50. Gateway
journal on Node B, paper requests only, grouped by provider, model,
system-prompt size and reasoning effort.

Last 15 min, at about 17:35 UTC:

| provider/model | system_chars | reasoning | calls |
|---|---|---|---|
| chatgpt/gpt-6-luna | 11922 | high | 108 |
| chatgpt/gpt-6-luna | 19638 | low | 93 |
| opencode-go/deepseek-v4.1-flash | **8238** | high | **92** |
| opencode-go/deepseek-v4.1-flash | 19638 | low | 90 |
| opencode-go/deepseek-v4.1-flash | 11922 | high | 61 |
| opencode-go/deepseek-v4.1-flash | 25449 | high | 42 |
| chatgpt/gpt-6-luna | 25449 | high | 40 |

Over 30 min, the 8238-char class was 240 of 240 on DeepSeek. No other
class is single-provider. No cat-jam-free calls after the restart. No
ChatGPT call failed in the window, so failover did not cause the split.

DeepSeek share: 285 of 526 calls (54%) overall; 193 of 434 (44%) when the
8238 class is left out.

## Two causes

1. **A hard pin outside the pool.**
   - `modelpolicy.managementTaskSelection` resolves the `management_task`
     role to opencode-go/deepseek-v4.1-flash, whatever the pool says. See
     `internal/modelpolicy/model_policy.go`, the `ManagementTask*`
     constants.
   - This is management delegation S2a, design R2-9 and O5
     (docs/reports/management-delegation-design-2026-10-14.md §9). R2-9
     requires a non-preview, bounded-latency model, because a task has a
     10-minute deadline. It named deepseek-v4.1-flash as the *proposed*
     default.
   - The 8238-char, reasoning-high class matches this pin. It is the only
     class with no ChatGPT calls, and the pin sets reasoning high.
   - This attribution is inferred from the gateway shape, not read from
     the run ledger. It is falsifiable: if the pin moves, that class must
     move with it.
2. **Per-trajectory hashing is lumpy.**
   - `modelpolicy.Assign` hashes the thread key onto a member. The key is
     the Texture agent for a Texture run, otherwise the trajectory.
   - The owner said 1/4 of *runs*, and the hash gives that in expectation.
     The share of *calls* follows whichever heavy trajectories land on
     DeepSeek.
   - A 15-minute window has few trajectories, so the 44% sits within the
     variance. A longer window is needed before calling it a defect.

## Side condition

OpenCode Go is still flaky for the pinned role: 10 parked, circuit-open,
429 or 402 lines per 10 min after the restart. The pin has no model
failover (R2-9 table), so a parked key fails the task call outright.

## Decision (no blocking ask; conservative option stated)

- Keep R2-9's properties: one pinned model, non-preview, bounded latency,
  and the 3-minute per-call timeout.
- Change the pinned default from opencode-go/deepseek-v4.1-flash to
  chatgpt/gpt-6-luna. gpt-6-luna is the fastest and cheapest GPT-6 tier,
  and it already serves three quarters of the pool with no failures in
  the window.
- The owner override (O5: an explicit provider and model in the computer's
  model policy) is unchanged.
- Leave the per-trajectory hash alone. Re-measure the pool-only share over
  hours, not minutes.

## Acceptance for the fix

- After the deploy, the 8238-char class appears on chatgpt/gpt-6-luna, not
  opencode-go.
- Management tasks keep closing inside the 10-minute deadline. No task
  closes `deadline_exceeded` that would not have before.
- The paper stays healthy on all four checks.

## After the fix (77633ecdc, paper refreshed 17:44:37)

Source: the gateway journal, 17:46–18:06, and `/api/runs` on the paper
(run metadata `llm_assignment`, `llm_provider`, `llm_thread`).

- **Pin moved.** The 8238-char class ran 44 calls on chatgpt/gpt-6-luna
  and 0 on DeepSeek. Runs since 17:45: 10 management task runs, all
  `explicit` chatgpt/gpt-6-luna. Inferred cause 1 is confirmed.
- **The hash is exact.** I recomputed FNV-1a(`llm_thread`) mod 4 for
  every pooled run since 17:11; slot 0 is DeepSeek. All 90 runs used the
  predicted provider.
- **The run and thread split is variance, not a defect.**
  - Threads: 16 of 46 on DeepSeek (35%). At p = 0.25 that is z ≈ 1.5.
  - Runs: 37 of 87 hashed runs since 17:45 on DeepSeek (43%). Texture
    threads are long-lived (one per document), so a few DeepSeek-hashed
    documents carry many runs.
- **Calls are not runs.**
  - Pooled calls in the 20-minute window: 464 DeepSeek and 259 ChatGPT
    (64% DeepSeek), against 43% of runs.
  - So a DeepSeek run made about 2.4× the calls of a ChatGPT run in this
    window. This comes from call counts, not a per-run join, so it's a
    hypothesis until a per-run call count confirms it.
  - The owner's wording ("1/4 of runs") is met in expectation. A 1/4
    share of *compute* is not, and per-thread hashing cannot give it.
- **Deadline kept.** All 10 management task runs after the fix
  completed, median 45 s, max 73 s, against the 10-minute deadline. The
  runs API page went back only to 17:43, so there are no pre-fix task runs
  to compare.
- OpenCode failures: 0 in the window. ChatGPT cache ratio: 75.4% (the
  restart lowers it; earlier windows were 78%).

**Open, for the owner:** is the target 1/4 of runs (met in expectation)
or 1/4 of compute? If compute, the levers are a lower DeepSeek weight
(1:5 or 1:7), or a check of why DeepSeek runs make more calls. No change
until a multi-hour window confirms the call ratio.

## Longer window (about 20:10 UTC)

- **Gateway calls, 17:46–20:10 (2 h 24 min):** chatgpt 2,121 and
  opencode-go 1,573, so DeepSeek takes 43% of all paper calls. Before
  the fix it took 54%. The window spans planned restarts at 18:20 and
  19:16.
- **`/api/runs` (the route returns its newest 100 runs, here from
  19:31):**
  - Hashed runs: 28 of 90 on DeepSeek (31%).
  - Threads: 14 of 52 (27%).
  - Explicit management task runs: 10, all on chatgpt.
- **Reading:** threads and runs sit near the 25% target. Calls stay
  higher (43%) because a DeepSeek run makes more calls than a ChatGPT
  run. The owner's question (runs or compute) stands. No code change.
