# Memo: Problem Documentation Must Precede Fixing

**Date:** 2026-05-23
**Author:** Review of commit `27d07a1` (Serialize heavy runtime tool calls)
**Status:** finding — requires codex session log investigation before closure

## Finding

Commit `27d07a1` introduced serial execution of heavy runtime tool calls after a
staging rerun revealed that parallel tool execution could CPU-saturate a worker
VM (Firecracker at ~95%, unresponsive to health checks). The problem was
discovered during the `3600b20` staging rerun, but the problem observation and
the code fix landed in the **same atomic commit**.

The mission doc update in `27d07a1` records the evidence:

> *"The worker VM became CPU-saturated (Firecracker at about 95% CPU), stopped
> answering `/health` even with a 20-second probe, and vmctl marked it unhealthy
> every 15 seconds while serial logs still showed gateway/tool-loop activity."*

and the belief-state capture:

> *"parallel side-effect tool execution can overload or wedge a candidate worker
> before it reaches terminal evidence."*

But neither the problem description nor the belief-state change existed in any
prior checkpoint. The fix was authored and committed in the same context that
first documented the problem.

## Why This Matters

Most problems have many potential solutions. Serializing all heavy tool calls
is one valid response to CPU saturation, but there are others: per-tool CPU
budgets, cgroup-based resource limits, priority queues, worker-medium VM class
as the default, or concurrent-but-capped execution. Each has different
implications for throughput, latency, debuggability, and the agent's model of
how tools work.

Because the fix and the documentation were atomic, there was no opportunity for:

- A separate review of the problem before committing to a solution direction.
- A consideration of alternative approaches with different tradeoffs.
- A refactoring window after the immediate mission pressure passed.
- A checkpoint that other agents or humans could examine to form their own
  opinion about what the right fix should be.

The fix was authored in the context of completing a MissionGradient run — "get
the experiment rerun past this blocker and to the next gate" — not with a
holistic view of the codebase's tool execution model, the runtime's performance
characteristics across VM classes, or the long-term architecture of tool
dispatch.

## Invariant

The following invariant must hold for all future platform behavior-changing
work:

> **Documenting a problem is the first priority. Fixing it is second.**

Concretely:

1. When staging evidence (or any reliable evidence) reveals a new problem, the
   **first commit** that follows must be a checkpoint or mission doc update
   that names the problem, records the evidence, and updates the belief state
   and remaining error field — without any code fix.

2. The code fix commit (or sequence) comes second, referencing the prior
   documentation. This allows the problem to be reviewed, alternative solutions
   to be considered, and the fix to be evaluated against the documented
   symptom independently of its implementation.

3. Exceptions require explicit justification in the commit message. The
   justification must name why the problem could not be documented before
   being fixed (e.g., the fix was trivially isomorphic to the diagnosis, or the
   window for deployment was closing and documentation was deferred to a
   follow-up checkpoint).

## Required Investigation

The codex session log for commit `27d07a1` should be reviewed to determine:

- Whether the problem was discussed in the session before the commit was
  authored (i.e., documentation existed ephemerally but was never committed).
- Whether alternative solutions were considered and rejected, and if so, why.
- Whether the mission pressure at the time ("get past this blocker")
  explicitly traded off documentation thoroughness for velocity.
- Whether the agent was aware of the invariant and chose to violate it, or
  whether the invariant was not yet established.

This investigation should produce a brief finding that either confirms the
one-commit pattern was a conscious trade (to be avoided in the future) or a
procedural gap (to be closed by tooling or prompt defaults).
