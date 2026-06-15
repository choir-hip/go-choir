# Mission M3.4 Ledger - Prompt-Bar VText First-Draft Regression

## 2026-06-15 - Problem Checkpoint

Mutation class: red. Protected surfaces include prompt-bar API, conductor
route materialization, VText document/revision/mutation state, provider
tool-loop handling, Trace/VText projection, vmctl restart/passivation recovery,
and staging deploy routing.

Initial owner evidence: prompt "What's new with Iran war" opened a VText titled
the same, showed `V0` and `Writing first draft...`, but stayed empty/pending for
minutes. Screenshot exposed run prefix `386f6c28-5...7be3ad`.

Node B evidence:

- Exact VText child run:
  `386f6c28-5594-4605-ba02-5c90387be3ad`.
- Parent prompt-bar/conductor run:
  `7855146d-59f0-419a-ab99-3ebb0e28481f`.
- Owner:
  `5bd6de97-3b58-408c-bf89-c42c81b083de`.
- Start time:
  `2026-06-15T12:15:16Z`.
- Gateway sequence: xiaomi `402`, deepseek `402`, then repeated ChatGPT
  successes with `tool_choice=function:edit_vtext`, `text_len=0`, and tool-use
  stops.
- VM interruption: `vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19` Firecracker exited
  with `signal: killed` at `2026-06-15T12:17:22Z`.
- Restart recovery: guest runtime passivated the same VText run as `was
  running` at `2026-06-15T12:18:00Z`.
- Later direct active guest `/health` was ready at `10.200.64.2:8085`, but
  authenticated data routes for prompt-bar status, Trace trajectory, and VText
  document listing timed out during probing.

Conjecture delta:

- The prompt seed may be present in metadata and initial VText prompt; visible
  empty V0 is expected for prompt-bar instruction revisions.
- The first real product failure is non-creation of a non-empty V1 and/or stale
  pending-state recovery after VM interruption.
- Repeated `edit_vtext` tool-use responses need tool-result inspection before
  choosing a repair.

Expected next move: extract the failed run's tool result errors, document id,
revision list, mutation row, and trace moments. If product routes remain timed
out, use a read-only, snapshot-safe store inspection route rather than mutating
live VM state.
