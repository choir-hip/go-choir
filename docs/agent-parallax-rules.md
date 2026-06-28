# Choir Agent Parallax Rules

This file carries long-running mission rules for agents working on Choir. It is
loaded when doing Parallax work, multi-hour missions, overnight runs, staging
self-development, or broad architectural work. It inherits
[Choir Doctrine](choir-doctrine.md) and the [operating contract](../AGENTS.md).

## Parallax

For multi-hour, overnight, staging, self-development, or broad architectural
work, use Parallax. A Parallax mission document is a **paradoc**: it states the
mission conjecture, deeper goal, witness/spec, invariants and qualities,
domain ramp, variant, budget, authority bounds, live conjectures/open edges,
next move, ledger, lineage, learning state, and settlement requirement.

Read `parallax-design-2026-06-11.md` and the available Parallax skill before
authoring or executing broad missions. When a legacy MissionGradient document
is still the best source form, compile it in place into a Parallax State
section instead of starting a disconnected control file. Preserve historical
MissionGradient reports as evidence; do not treat them as current operating
doctrine unless a newer paradoc promotes the claim.

Do not turn Parallax into a brittle checklist. Treat the bridge from artifact
completion to deeper-goal progress as suspect until evidence supports it.
Select moves by expected variant decrease per budget, force observer shifts
when probes stop changing decisions, and exit only as settled, open_handoff,
blocked, or superseded.

## Texture Narrative for Long-Running Missions

For long-running Choir-in-Choir missions, maintain an owner-readable Texture
narrative. Each substantive change in plan, evidence, blocker, or result should
produce a concise revision that explains the whole run state so far in plain
language: objective, past work, current work, what changed, evidence, learnings,
risks, and next step. Do not make Texture a Trace-like topology/status table, and
do not dump low-level events into Texture. Trace is the causal ledger for dense
tool calls, LLM content, and agent-to-agent messages; feature-specific live
surfaces such as Chyron may show granular activity streams; Texture is the human
supervision narrative.

## Independent Review Threads

For second-opinion review, independent prover, or handoff-tier verification,
prefer Codex thread tools over in-thread subagents when the user authorizes a
separate thread. Use `list_projects` and `create_thread` to start a fresh
project-scoped verifier thread with a narrow review prompt, and ask that thread
to return a verdict with evidence rather than implementation. Keep the verifier
thread read-only unless it discovers a problem that must be documented under
Problem Documentation First.

When thread inspection or wakeup tools are available, use them to reconnect the
review to the spawning thread: `read_thread`/`list_threads` for the verifier
result, `send_message_to_thread` or the app's wakeup/follow-up mechanism when
the spawned reviewer needs to notify or continue the spawning thread, and
`handoff_thread` only when ownership of a checkout/worktree should move. If the
thread tools are unavailable, record the fallback used and do not treat a
same-context reread as an independent prover.
