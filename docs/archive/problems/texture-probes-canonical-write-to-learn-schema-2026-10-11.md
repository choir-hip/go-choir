# Texture probes the canonical write path to learn the edit schema

Found 2026-10-11 in S1c (PD1 deployed proof, staging, test computer
demo-arxiv-2026-10-10c, deploy 16d7c21a). Evidence class: run trace
(scratchpad s1c-2, run 43c2237c, model deepseek-v4.1-flash).

## What happened

Research reported round one at 02:08:29. Texture's next turn started at
02:08:22 and committed its revision at 02:12:41, four minutes and nineteen
seconds later, in 13 cells. Ten of them failed. About eight were deliberate
probes: `choir.ApplyTexture` calls with `"rationale": "schema probe"`, an op
named `bogus_op_probe`, a block id `p-nonexistent-probe`, and integers where
strings belong (`block_type: 5`, `after_block_id: 5`), each built to fail so
that the rejection would list the valid values.

`choir.Help("ApplyTexture")` invites that. It names `op`,
`base_revision_id`, `content` or `edits`, and says "Rejections name the valid
fields and values". It does not list the edit operations or their fields.
The run prompt (revision_policy.yaml) does describe them, but in prose, many
lines from the verb's help.

## Why it matters

- **Latency.** The owner waited four minutes for findings research had
  already delivered. The value criterion is time to first grounded revision.
- **Canonical safety.** `ApplyTexture` is the sole write path for the
  document. A probe that happened to be well formed would have committed
  "probe" text to the owner's document. A model needs a way to learn the
  shape that cannot publish.

## Second problem in the same turn: Updates() shows what the commit refuses

The turn's first cell read `choir.Updates()` and saw two research packets:
message seq 6 and seq 8. Seq 8 arrived after the activation was scheduled.
The cell dispositioned both, and the commit refused with "does not name a
pending target-bound producer report eligible to this activation: its
message seq 8 is after this activation's scheduled seq 6". The cell frame and
the commit read different sets of updates. That breaks single state
authority (standing question 4), and it cost a failed cell plus a reread.

## Belief and test

I believe that if the verb's help gives the complete edit shape, and the
desk can check an edit without committing it, then probe cells disappear
whatever the model. The test is to count, in the next research-only demo,
the failed `ApplyTexture` cells per Texture turn, and how many carry a probe
rationale.

I also believe the frame should show exactly the updates the commit will
accept. The test is a focused test: an update past the scheduled seq is
absent from the frame, or accepted by the commit, but never one without the
other.

## Amendment 02:30Z — the cell cannot see block ids (root cause)

A later Texture turn in the same demo (run d2a6cdcb, started 02:21:27,
still running at 02:28 after 18 cells with 10 errors) shows the deeper
cause. Its probes are not about the op names. It is guessing **block
ids**: `"14"`, `"No dated, verifiable chips"` (a sentence from the
document), `p-<doc>-<salt>-5`, each refused with "block_id ... not found".
One probe was `update_block_text` with text `"probe"`. With a correct id,
that cell would have overwritten a paragraph of the owner's document with
the word "probe".

`choir.ReadDoc()` returns the head's revision id and markdown content only;
it has no block ids. The structured outline with ids is written into the
run's prompt once, at run start (texture_agent_revision.go). A reactivated
Texture run (`actor_reactivate_existing_memory`) carries its earlier memory,
so its outline describes an older revision. After the run's own commit, the
outline in its context is stale by construction. The cell is the one
surface that is current at every cell (R3d), and it lacks the ids the edit
schema requires.

Revised belief: the help text fix (118f3fb6) removes the invitation, but
probing ends only when the cell's document snapshot carries the current
outline (block and node ids, type, a text prefix). Test: in the next demo,
a block-id "not found" rejection in any Texture turn counts as a failure.
