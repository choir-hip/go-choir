# After Texture settles its work, the owner cannot revise the document (2026-10-09)

Found by the Texture acceptance suite, third run on staging (build
7bc8f374; receipt `evidence/texture-acceptance-2026-10-09T18-59-43-320Z.json`),
fresh disposable account `1d20b772…` (`computer-7799ad0b…`, VM
`vm-549ec1d9…`), document `1ae32977…`. Mutation class of the eventual
fix: red (lifecycle work items, owner revision intake). Problem first; no
fix in this commit.

## What the owner sees

T1 to T4 pass (list 25–35 ms; first draft in 46 s; revise in 21 s;
revision reads under 45 ms; no "Revising…" after reload). The next
owner revise (T5) is refused: `409 {"error":"lifecycle has no open desk
target work item"}`. In the editor this is a revise that fails on a
document that is live and idle.

## Trace (guest trajectory read, guest console log)

| Time (UTC) | Event |
|---|---|
| 19:00:00 | `trajectory_started` (create) |
| 19:00:43 | `texture_turn_committed` → revision `a26e405b…` (first draft); `work_opened` for `research:f342a530…`; `control_queued`, `control_delivered` (Texture delegated to research) |
| 19:00:46 | `artifact_head_advanced` → `ae887810…` (owner revise T2b) |
| 19:01:07 | `texture_turn_committed` → `418011c6…`; **`work_settled`**: the Texture work item `68ff5b1e…` becomes `completed` |
| 19:01:09 – 19:01:26 | research tool loop, iterations 4–8 |
| 19:01:26 | `update_queued` (research reports back toward Texture) |
| ~19:01:28 | owner revise → 409 no open desk target work item |

State after: trajectory `live`; Texture work item `completed`; research
work item `open`; no agent has an active run.

## Cause (code reading)

`handleLifecycleOwnerRevision` (`internal/textureowner/texture_agent_revision.go`)
accepts an owner revise only when exactly one open Texture (or
Engineering) work item is assigned to the document. When the Texture desk
settles its work item at the end of a turn, nothing reopens it, so every
later owner revise is refused. Whether the owner can keep revising
therefore depends on whether the model chose to settle: in the first run
(document `e053b60e…`) the first turn did not settle and the revise was
accepted.

## Second effect (O1)

Research's report (`update_queued` at 19:01:26) targets a Texture whose
work is completed and which has no active run. Whether that update is
delivered, consumed or stranded is not visible yet; the obligations
surface and the new consumption log lines (8a3d31c8) will show it.

## Fix directions (to decide before code)

1. **Owner input reopens Texture work.** An owner revise on a live
   document with no open Texture work item opens one (same authority the
   create path uses), then proceeds. The owner's document is never closed
   to its owner while the trajectory is live.
2. **Texture never settles document work while the trajectory is live.**
   Narrower, but leaves existing settled documents unrevisable and puts a
   product rule in the desk's hands.

Direction 1 is the conservative product choice: it changes nothing for
documents with open work and repairs every already-settled document on
its next revise.
