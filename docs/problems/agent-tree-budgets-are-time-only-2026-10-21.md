# Agent trees get a time slice only; no tree's spending is ever totalled (2026-10-21)

Checklist item: ARCH-08 in
[security-density-architecture-checklist-2026-10-21.md](../security-density-architecture-checklist-2026-10-21.md).
Station: [FP10 ARCH-08](../definitions/choir-launch-fp10-arch-08-2026-10-21.md).
Problem first; no fix here.

## Settled target

The RLM consensus (D7b/M6.5,
[rlm-implementation-consensus-2026-08-29.md](../archive/reviews/rlm-implementation-consensus-2026-08-29.md) §7):
a nested activation gets "a carve-out of the parent's budget", children are
"metered exactly like bare calls", depth cap 2-3, and one kill reclaims the
tree. The checklist names the resources: tokens, wall time, egress bytes,
child count and memory.

## What the code does (e4b792b7f)

| Resource | Today | Where |
|---|---|---|
| Depth | 1. A research child has no `Research` verb and its broker handle lacks `ActionResearch`; pinned by `TestResearchTreeReturnsWithinItsTime` | `yaegikernel/choir.go` (child module set, handle actions); owner PD1 2026-10-11, "rounds over recursion" |
| Wall time | Carved: a desk's `Research` call returns within 10 min; each child gets 0.75 of the time left when it starts and is told it | `research_children.go` `researchCallWindow`, `researchChildShare` |
| Model-call rate | One computer-wide spacer, 40 calls a minute for all sub-RLMs (research children and management tasks) | `research_children.go` `subRLMCallSpacer` |
| Child count | Unbounded per call; 4 run at once per call, the rest queue | `researchChildConcurrency` |
| Tokens | Not totalled. Each child logs its own tokens on one line; the parent cell never sees them and no line sums a tree | `runResearchChild` log line |
| Egress bytes | Not counted per child or per tree | host-mediated `SearchStart`/`Read` |
| Memory | Indirect: 4 worker processes per call; nothing bounds calls across desks | `researchChildConcurrency` |

Management tasks are the other sub-RLM tree: K slots (default 4, at most
8), a 10-minute deadline per task, the same call spacer, and no spending
total.

## Defects

1. **No tree total.** "Track spending, never cap" (owner; DEC-07, RES-11)
   needs a tracked number per tree. Today a tree's tokens can be
   reconstructed only by summing guest log lines by `root=`, and the
   desk that spent them never learns what it spent.
2. **Stale comments contradict the code.** `yaegikernel/choir.go:359-360`
   says a child "may fan out further"; `research_children.go:137` says a
   child has "further choir.Research". The child preamble, the surface
   summary and the module set all say it cannot. An agent that reads the
   comment could widen depth by mistake. This is how the tree in
   [research-fanout-overruns-computer-model-budget-2026-10-11.md](../archive/problems/research-fanout-overruns-computer-model-budget-2026-10-11.md)
   spread to 780 children.
3. **The settled design (depth 2-3) and the owner's later ruling (depth 1,
   PD1) disagree**, and no record calls the difference a deliberate deviation.
4. **Egress bytes and memory have no per-tree slice.** Memory is bounded
   per call, not per computer. That belongs with VM sizing (RES lanes,
   FP8's vmctl/vmmanager/image work).

## Not a defect

Depth 1 is enforced in two independent places and tested. The time
carve-out works and is tested. Whether a budget may refuse work is an open
owner decision (DEC-07), not a defect.
