# Why the computer cannot take a restore fence yet

**Date:** 2026-08-16  
**For:** owner, not a completion stamp  
**Definition:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`  
**Computer:** `computer-03335285269bdba4f94377e56879f9e6`  
**Staging:** `https://choir.news` still running `5557840c`  
**This is not permission to start Super, send mail, rematerialize, or weaken a gate.**

---

## In one paragraph

Choir can already restore a computer from the tape. That work finished. The next mission wants to let the computer change its own source, then put that change back. To put it back, we need a snapshot taken *before* the change — a pre-A checkpoint. Today that snapshot is refused, and the refusal is correct. The desktop and a few object-graph rows exist only as local writes. Replay from the event tape does not recreate them. Recording a checkpoint now would promise a restore the tape cannot perform.

The owner asked for this report rather than a guessed next move.

---

## What we are trying to prove

The effects Definition is not “turn the computer on and hope.” It is a specific proof:

1. The computer, under a named policy, authors a small reversible change (solitaire: headless play API, no web UI).
2. A qualified panel — not a one-off owner click — authorizes that exact change.
3. The change promotes, runs, writes real rows, then gets falsified and superseded.
4. The whole excursion is restored away, using the tape-recovery restore already paid.
5. Later, a separate irreversible policy may send one exact email. That mail has not been sent and must not be sent in this slice.

Step 4 needs a restore target taken **before** solitaire exists. That is the pre-A checkpoint. Without it, “we restored the excursion” has nothing honest to restore *to*.

Tape-recovery already proved that restore works when the checkpoint is eligible. This Definition is supposed to **consume** that substrate, not rebuild it.

---

## Where the computer actually is

As of 2026-08-16T05:53Z, after an owner-scoped refresh that only re-exchanged guest credentials:

| Surface | State |
|---|---|
| Host deploy | `5557840c` |
| Realization epoch | **272** |
| Mode | **`propose_only` generation 1** (signed receipt `01a0091b-…`) |
| Genesis | 409, effects still proxy-disabled |
| Kernel capabilities | 200, signed |
| Super | not started |
| Named solitaire prompt | written down, **not posted** |
| Outbox | `Armed=false` |
| Mail | none sent |
| Event heads | live == replay, sequence **26** |
| Replay probe | diagnostically **not equivalent** |
| Checkpoint | **409** |

Mode `propose_only` means: if someone presents that signed mode receipt on start, Super launches. This report does not do that.

The first checkpoint attempt failed because guest event credentials live five minutes and the renewal grace is sixty seconds. Refresh 271→272 fixed credentials. The second attempt failed for a real eligibility reason, not a timeout.

---

## What “eligible” actually means

A checkpoint is not a disk image. It binds:

- the event head (the tape address)
- code and artifact identity
- a VM-local content witness (hashes of what Dolt currently holds)
- a frontend identity (which SPA bytes this computer serves)

Restore rebuilds the projection **from the tape**, then checks those hashes. If a row existed only because some API wrote SQL, replay will not put it back. The witness would then either:

- lie (record the row, restore cannot recreate it), or
- drop the row silently (restore “succeeds” and the user’s desktop/state is gone)

The gate refuses instead. That is `finish.not_done_when`: *a checkpoint created while behavior-bearing local rows are not event- or receipt-derivable*.

The classification lives in `internal/agentcore/replay_eligibility.go`. Most VM-local tables are `empty_until_supported`: they may exist, but they must be **empty** until a reducer writes them from events. Only the event-projection tables (`computer_event_index`, `computer_event_projection_heads`, `computer_effective_state`) may hold data today.

Live probe at epoch 272:

```text
eligible: false
reason:  behavior-bearing direct-write tables are non-empty without reducers
heads:   sequence 26, live == replay
```

Non-empty unsupported tables:

| Table | Writer | Why it is there |
|---|---|---|
| `desktop_workspaces` | `internal/store/desktop_live.go` | saved window layout |
| `desktop_sessions` | same | which session is driving the desktop |
| `desktop_app_instances` | same | open apps |
| `desktop_window_placements` | same | where those apps sit |
| `og_objects` | VM-local object graph | runs, texture-adjacent objects, other graph rows |

Replay of those five tables is the empty hash (`e3b0c442…`). Live hashes are not. The content-root hash differs as a consequence. Six differences, one cause: local writes with no reducer.

This is the same class named on 2026-08-12 (`docs/evidence/choir-supervised-self-development-replay-difference-classes-2026-08-12.md`): *discard; fail-closed while nonempty*. The 2026-08-13 bootstrap made the computer eligible once. Using the computer filled the tables again.

---

## How tape-recovery got a checkpoint anyway

On 2026-08-14 the same computer published checkpoint `70f9ce2b…` at epoch 261. That receipt is real. Two facts about it matter, and they are easy to smash together:

1. **It was eligible at the time it was taken.** The later restore proof *then* dirtied Texture and the SPA, so eligibility went false *after* the snapshot. Restore returned the computer to an eligible witness. The current desktop/object-graph rows accumulated after that.
2. **It is an OwnerRecovery checkpoint.** No verifier run. The guest attests the witness; restore later checks it by reconstructing from the tape. Route projection and decision policy both refuse OwnerRecovery as promotion evidence. That is pinned by test.

So: tape-recovery paid “we can restore this computer from a recorded head.” It did **not** pay “the next effects promotion may use that OwnerRecovery artifact as its pre-A fence.” The heads have moved (sequence 26 vs the 2026-08-14 restore target). Using `70f9ce2b` as the effects restore fence would rewind the computer through every later VM-local change, and it would still be OwnerRecovery.

`choir computer checkpoint` today still publishes through `publishOwnerRecoveryCheckpoint`. Even that path calls `checkpointRestoreBindings`, which refuses unless eligibility is true. There is no “skip the gate because this is owner recovery” switch. The 2026-08-14 snapshot worked because the tables were empty enough then, not because the class is privileged.

---

## What would be cheating

These are named so a later session cannot “just do them”:

| Move | Why it is cheating |
|---|---|
| `DELETE FROM desktop_*` / `og_objects` by hand | Empties evidence instead of making it event-derived. The Definition already forbade emptying desktop rows by hand. |
| Change `EmptyUntilSupported` to allow nonempty tables | Records a checkpoint the tape cannot rebuild. Directly hits `not_done_when`. |
| Start Super now and checkpoint later | Super writes `og_objects` (runs). There would be no pre-A fence, only a post-mess snapshot. |
| Restore to `70f9ce2b` and call that the effects fence | OwnerRecovery is inadmissible for promotion. It is also the wrong head. |
| Invent a second computer | Owner product path still cannot mint one. Definition forbids inventing `choir computer create`. |
| Treat orange rehearsal as live proof | In-process tests walked propose → consensus → promote → restore. Staging has not. |
| Send the acceptance email | Different policy, different slice, `Armed=false`. |

Workspace-replace is a product verb, not a hidden SQL delete. It quarantines the whole VM-local Dolt workspace and opens current DDL. It also wipes the event projection, so heads go null and `bootstrap-chain` is required again. Doctrine allows that cutover as a **pre-launch exception** that *does not license restore or effects by itself*. Using it now would discard current desktop and object-graph residue — the same loss boundary as 2026-08-12 — then require a new eligible checkpoint before Super. Whether that is still “pre-launch” is a judgment, not a free pass.

---

## Honest options (no ranking)

These are the options a later panel should argue about. None of them are executed here.

### 1. Write reducers

Make desktop and `og_objects` event-derived. Then nonempty is fine: replay recreates them.

- **Pays the ontology.** This is what `empty_until_supported` was waiting for.
- **Cost.** Not a small slice. Desktop sync and the object graph are live product surfaces.
- **Mission fit.** Effects asked not to independently green restore legs. Reducers are restore-completeness work. They might belong in a successor, or they might be the actual blocker.

### 2. Cut over, bootstrap, checkpoint, then Super

`choir computer replace-workspace` → restart → `bootstrap-chain` → checkpoint immediately, before the desktop writes again → then present the mode receipt.

- **Product path exists.** Used in tape-recovery to get off retired schema.
- **Loss.** Current desktop layout and `og_objects` go to quarantine. Event chain is rebuilt from genesis import, not preserved as today’s sequence 26.
- **Trap.** The exception text says the cutover does not license restore or effects. The checkpoint *after* bootstrap would be the license, if eligibility holds long enough.
- **Race.** Opening the computer in a browser will refill desktop tables. Super start will refill `og_objects`. The checkpoint has to land in that window.

### 3. Restore to the paid 2026-08-14 checkpoint, then continue

Consume `choir computer restore` against `70f9ce2b`, then try to checkpoint a new fence.

- **Consumes tape-recovery**, which this Definition is supposed to do.
- **Wrong evidence class for promotion.** OwnerRecovery still cannot authorize route projection or consensus.
- **Destructive of current Dolt.** Sequence 26, desktop, graph rows, and any Texture local state since 2026-08-14 go away. Guest binary from refresh `5557840c` would remain; VM-local state would not.
- **Likely still needs a new eligible checkpoint** at the restored head before Super, because the paid artifact is not a pre-A fence for *this* excursion.

### 4. Wait. Do not start Super on this computer until eligibility is real

Keep `propose_only`, keep genesis 409, keep mail unarmed. Land this report. Let a panel or the owner pick.

- **Does not fake a fence.**
- **Does not advance route map 9 red promote+restore.**
- **Honest about the stall.** Using the computer fights eligibility. That is the design, not a bug.

### 5. Prove restore of the excursion only in-process, and say live pre-A is blocked

Orange rehearsal already did propose → consensus → promote → restore without a live send.

- **Under-claims the Definition.** Finish wants a staging trajectory: promote, play, falsify, restore to pre-A.
- **Might be a scoped repair of the Definition** if the owner decides live pre-A on this accumulated computer is the wrong substrate. That is an owner correction, not an agent shortcut.

---

## What is already paid (so we do not redo it)

- Tape-recovery complete: checkpoint witness, scope refusal, destructive rematerialization, serving join, owner-reachable restore, capability renewal.
- Effects wiring: decision policies, reconnection, freeze/propose tools, reducer, trusted outbox (unarmed), supervision identities, orange rehearsal.
- Guest: mode authority, owner-scoped refresh, kernel route, verifier socket. Live 409/200 smokes.
- Named modes: start = `propose_only` (live); decision = `qualified_consensus` bound to `reversible-selfdev-v1` digest `c34ddf07…`.
- Named prompt: solitaire, policy-before-outputs, no mail, no rematerialize, no OwnerRecovery promotion.

What is **not** paid: a restore-eligible pre-A checkpoint, Super start, freeze, qualified consensus CAS, promote, live restore of the excursion, live mail, route map 10, `goal.complete`.

---

## Recommendation from this author (weak, not a decision)

Do not start Super. Do not weaken the gate. Do not SQL-empty the desktop.

The interesting disagreement is between **(1) reducers**, **(2) a deliberate workspace cutover plus a fast checkpoint**, and **(4) wait**. (3) looks like a category error. (5) looks like shrinking the Definition.

I am not certain which of 1, 2, or 4 is right. That is why this file exists.

---

## Constraints that stay in force

- Effects remain OFF until a named red promote+restore actually happens.
- Genesis stays 409.
- `Armed=false`. No live mail. No live provider.
- Do not delete `external-owner:`, `accept_once`, or `awaiting_approval`.
- Do not use OwnerRecovery for promotion.
- Do not rematerialize as a new product path.
- Do not invent `choir computer create`.
- Do not `goal.complete`.

---

## Sources

- Live probe: epoch 272 replay-completeness, `eligible=false`, five unsupported tables, six hash differences, heads sequence 26.
- Evidence: `docs/evidence/effects-red-pre-a-checkpoint-ineligible-2026-08-16.md`
- Gate: `internal/agentcore/replay_eligibility.go`, `internal/agentcore/checkpoint_restore_bindings.go`
- Publish path: `internal/agentcore/rematerialize.go` `BindCheckpointRestoreSet` / `publishOwnerRecoveryCheckpoint`
- Paid snapshot: `docs/evidence/tape-recovery-checkpoint-witness-published-2026-08-14.json` (`70f9ce2b…`, `owner_recovery: true`)
- Classification: `docs/evidence/choir-supervised-self-development-replay-difference-classes-2026-08-12.md`
- Definition: `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

---

## Panel result (2026-08-16, convergent)

Five independent agents read this report. None edited the tree. Super was not started.

| Agent | Lens | Verdict | Confidence |
|---|---|---|---|
| GPT 5.6 Sol | cutover-window | **OPTION_2** (replace-workspace → bootstrap → checkpoint) | medium |
| GPT 5.6 Terra | consume-paid-restore | **OPTION_2** | medium |
| Cursor Grok 4.5 | orange-is-enough | **OPTION_2** | medium |
| Gemini 3.6 | stall-is-honest | **OPTION_1** (write reducers) | high |
| Devin | reducers-are-the-ontology | **OPTION_4** (wait) | high |

**Majority:** 3/5 OPTION_2. **Dissent:** reducers (Gemini), wait (Devin). Nobody picked restore-to-`70f9ce2b` or shrinking live proof to orange.

Shared forbidden list, unanimous: no Super, no SQL-empty, no gate weaken, no OwnerRecovery promotion, no rematerialize, no invented computer, no live mail, no `goal.complete`. Cutover itself does not license effects; only a later eligible checkpoint could.

Shared objection to the majority: the 2026-08-12/13 workspace-replace was a *pre-launch* exception on a null-head computer. Sequence 26 is not that computer. A second cutover discards current desktop/graph residue and rebuilds the chain. If that exception was one-shot, OPTION_2 lacks authority and the fallback is wait.

This author does **not** execute OPTION_2 from the majority alone. Medium confidence plus a live authority objection is an owner call.

Transcripts: `.agentic-consensus/effects-pre-a-checkpoint-20260816/` (gitignored). Receipt: `docs/evidence/effects-pre-a-checkpoint-eligibility-review-2026-08-16.md`.
