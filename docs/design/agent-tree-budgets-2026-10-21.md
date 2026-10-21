# Budgets passed down agent trees: design (ARCH-08, 2026-10-21)

Problem: [agent-tree-budgets-are-time-only-2026-10-21.md](../problems/agent-tree-budgets-are-time-only-2026-10-21.md).
Station: [FP10 ARCH-08](../definitions/choir-launch-fp10-arch-08-2026-10-21.md).
This is a design with one observe-only slice built. Everything past slice 1
waits on DEC-07 or on other lanes.

## Principle (proposed reading of DEC-07)

"Track spending, never cap" (owner) and "a carve-out of the parent's
budget" (M6.5) fit together if a **budget is a scheduling limit, not a
spending refusal**:

- **Tracked always.** Every tree reports what it spent (tokens, calls,
  egress bytes, children, wall time) to the desk that started it and in
  one log line per tree.
- **Structural bounds refuse.** Depth and fan-out per call protect the
  computer from runaway shape (the 2026-10-11 tree reached 780 children).
  They are about shape, not cost, so they are not spending caps.
- **Spending slices queue; they never refuse.** A tree past its token or
  egress slice is not killed. Its next model call waits behind the root
  desks' calls (a lower priority at the computer's spacer, and later at the
  gateway's fair-share queue, RES-11). Time is the only budget that ends a
  tree, and it already does.

**Owner question (DEC-07):** is this reading right? In particular, is a
per-call child-count bound (proposed: 16) a structural bound or a cap?

## Per resource

| Resource | Slice | Mechanism | State |
|---|---|---|---|
| Wall time | 0.75 of the parent's remaining time; desk call at most 10 min | context deadline | **done** (research); tasks have 10 min each |
| Depth | 1 | child module set and broker handle | **done**; see deviation below |
| Tokens | tracked per child and per tree; later a soft slice that lowers priority | `ResearchChildResult` usage, tree log line | **slice 1 (this station, observe only)** |
| Model calls | computer-wide 40/min for sub-RLMs | `subRLMCallSpacer` | done; moves to the gateway with RES-11 |
| Child count | 4 concurrent per call; proposed refusal above 16 per call | `runResearchChildren` | slice 3, waits on DEC-07 |
| Egress bytes | tracked per tree | count bytes the host returns to a child for `Read`/`SearchStart`, in the broker | slice 2, no VM files; needs a design pass on the broker's return path |
| Memory | computer-wide bound on live child workers across desks | worker pool size from VM size | slice 4, with RES/FP8 VM sizing; not this lane |

## Depth: a deliberate deviation

M6.5 settled depth 2-3. The owner ruled on 2026-10-11 (PD1, research
fan-out) that children do not fan out further: "rounds over recursion".
The 10-11 incident shows why: a child that can delegate does so before it
does any work. Proposal: record depth 1 as the doctrine for research
trees. Depth 2 would come back only with a token slice that queues it
(slices 1 and 3) and a deployed proof that the tree finishes. Management
tasks are already depth 1: a task cannot `Delegate`.

## Slice 1 (built, observe only)

- `ResearchChildResult` carries the child's `input_tokens`,
  `output_tokens` and `cached_input_tokens`, so the parent cell sees what
  each child spent.
- `runResearchChildren` logs one `research tree:` line per call: root run,
  children, children with errors, summed tokens, duration.
- No refusal, no cap, no prompt change: the desk surface text is unchanged,
  so prompt caches are unaffected.

Mutation class orange (the cell-visible result type gains fields; a log
line). Rollback: revert the commit. JSON readers that do not know the new
fields ignore them.

## Later slices and their owners

2. Egress bytes per tree (broker return path; FP10 or a follow-up lane).
3. Per-call child bound and token-slice priority at the spacer (after
   DEC-07).
4. Computer-wide live-worker bound (VM sizing; with FP8 / RES items, since
   it touches vmctl/vmmanager sizing).
5. Management-task trees report the same totals to management (same
   shape as slice 1, `management_tasks.go`).

## Deployed proof for slice 1 (FP0)

On a test computer on staging after the deploy, start a research desk
turn whose cell calls `choir.Research` with three objectives. Then:

1. The cell's stdout JSON shows `input_tokens` > 0 on each child that
   returned findings.
2. The guest log has one `research tree: root=<run>` line whose
   `input_tokens` equals the sum over that root's `research child:` lines.
3. The paper's schedules and polls read the same before and after the
   deploy.
