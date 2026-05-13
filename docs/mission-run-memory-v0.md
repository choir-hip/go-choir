# MissionGradient: Choir Run Memory v0

Status: completed in repo
Date: 2026-05-13

Completion notes:

- Proof transcript: `docs/run-memory-v0-dogfood-2026-05-13.md`
- Next frontier: `docs/run-memory-next-frontier-2026-05-13.md`
- Next mission doc: `docs/mission-candidate-world-promotion-v0.md`

## Real Artifact

Choir durable run memory v0: the runtime substrate that lets long-running agents preserve operational identity across context pressure, compaction, overflow recovery, restart boundaries, and later background-VM leaps.

This mission is not primarily about UI polish, app launcher, uploads, themes, podcasting, or browser backend work. Those remain important product targets, but this run should first make Choir capable of surviving and learning through long runs.

## Value Criterion

Maximize verified continuation quality across long agent runs while minimizing canonical-state corruption, context loss, retry loops, user monitoring burden, unverifiable promotion, and work that disappears into transient chat.

The run improves Choir only when it leaves durable structure: code, tests, events, store records, docs, traces, or a reviewed next-frontier research artifact.

## Invariants

- Canonical foreground state is not speculatively mutated.
- Background/candidate work remains distinguishable from canonical state.
- Tool-call/tool-result adjacency is never broken by compaction.
- Every compaction is durable, inspectable, and linked to the run/session branch it summarizes.
- Every run-memory branch has one owner and one reconstructible leaf.
- Overflow recovery is bounded; one compact-and-retry attempt per logical overflow is enough for v0.
- Failed compaction becomes an explicit blocked/failed state with evidence, not undefined behavior.
- Existing user or agent work is not reverted or overwritten.
- Vtext remains the semantic owner of document changes.

## Homotopy Parameters

Increase realism continuously while preserving the same object:

- deterministic store/context rebuild tests;
- in-memory tool-loop context rebuild through persisted entries;
- manual compaction;
- threshold compaction;
- context-overflow recovery with one retry;
- child-run or appagent path using run memory;
- one real dogfood transcript showing continuation from compacted memory.

Do not jump to full Choir-in-Choir until the same run-memory substrate works at smaller scale.

## Dense Feedback Channels

- Store tests for run/session entries, compaction entries, and branch reconstruction.
- Runtime tests for context rebuild from compaction summary plus kept entries.
- Cut-point tests proving tool calls and tool results are not separated.
- Provider-shaped overflow tests.
- Event assertions for compaction start, compaction end, retry, and blocked recovery failure.
- A final dogfood note with the command/path used, what compacted, what continued, and where the evidence lives.

## Forbidden Shortcuts

- Do not keep compaction only in memory.
- Do not treat provider overflow as an ordinary failed run without structured recovery.
- Do not summarize away information required to understand retained tool results.
- Do not use fake UI success as proof of runtime behavior.
- Do not mutate canonical vtext or desktop state from a candidate/background path.
- Do not make "Choir-in-Choir" depend on a hand-edited success artifact.

## Rollback Policy

Use normal git discipline for code changes. Keep edits scoped to run memory and directly required tests/docs. If a migration is needed, make it forward-compatible and covered by tests. Do not remove existing runtime records or desktop/vtext state as part of the proof.

If implementation stalls, leave a small failing or skipped test only when it names the missing behavior precisely. Otherwise revert only your own incomplete edits.

## Learning Side-Channel

The final phase of this task is review and research for what comes next. Do not end immediately after tests pass.

Produce a concise next-frontier artifact in `docs/` covering:

- candidate-world promotion;
- background VM branch and rollback geometry;
- git/worktree/branch-per-VM implications;
- verifier contracts instead of verifier-agent ontology;
- skills-native super/cosuper support;
- whether a narrow Choir-in-Choir demo is feasible next;
- which product target should drive the next run: podcasting, browser backend/Obscura, launcher/uploads/themes, or vtext/radio.

End that artifact with a new one-line `/goal` string.

## Stopping Condition

Stop when either:

- run memory v0 is implemented, tested, and demonstrated through one real continuation path, and the next-frontier review artifact exists; or
- the mission is blocked, with the failed invariant, rollback point, evidence, and next smallest probe written down.

Completion requires proof of behavior, not just code existence.

## Research Anchors

- Real Pi repository: https://github.com/earendil-works/pi
- Pi compaction core: https://github.com/earendil-works/pi/blob/main/packages/agent/src/harness/compaction/compaction.ts
- Pi coding-agent session auto-compaction: https://github.com/earendil-works/pi/blob/main/packages/coding-agent/src/core/agent-session.ts
- Pi session manager and compaction entries: https://github.com/earendil-works/pi/blob/main/packages/coding-agent/src/core/session-manager.ts
- Pi overflow detection: https://github.com/earendil-works/pi/blob/main/packages/ai/src/utils/overflow.ts
