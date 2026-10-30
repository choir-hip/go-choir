# Texture: cancel ends the document; research never converges (2026-10-09)

Found by the Texture acceptance suite, fourth run on staging (build
4f9331cf; receipt `evidence/texture-acceptance-2026-10-09T19-19-29-623Z.json`),
fresh disposable account on VM `vm-6071b4cd…`. Problem first; no fix in
this commit. Mutation class of any fix: red (lifecycle trajectories,
Texture work items and occurrences).

Run 4 passed T1–T5: list 27–30 ms; first draft 36 s; revise 16 s;
revision reads fast; no "Revising…" after reload; the revise that run 3
refused was accepted (4f9331cf); cancel cleared pending in 1 s.

## 1. Cancel ends the document (T5b)

After the cancel, the owner's next revise on the same document returns
`409 {"error":"durable lifecycle state is unavailable or terminal"}`.

The editor's Cancel (`frontend/src/lib/texture.js` `cancelAgentRevision`)
posts `/api/trajectories/{id}/cancel`, and `CancelLifecycleTrajectory`
(`internal/store/lifecycle.go`) sets the document's trajectory to
`cancelled`, a terminal state. The owner revise requires a live trajectory.
So "stop this turn" in the editor means "this document can never be
revised again". The owner has to create a new document.

## 2. Research never converges (T6)

On a fresh document the owner asked for research with cited sources. No
research revision landed in 6 minutes. Trace (guest console log, guest
trajectory read; times UTC):

| Time | Event |
|---|---|
| 19:21:50 | first draft applied (72c7707a…) |
| 19:21:51 | owner research revise (head ee496288…) |
| 19:21:54 – 19:22:35 | research tool loop; `update_queued` |
| 19:23:08, 19:24:18 | Texture turns committed; at 19:24:18 two more research work items opened |
| 19:24:19 | Texture `decide`, `decision_kind: wait_for_evidence` |
| 19:24:20 – 19:26:14 | each research report triggers a Texture activation; Texture decides `wait_for_evidence` again (19:25:16, 19:26:00), opens a fourth research item (19:25:16); after each, the dispatcher defers the occurrence and its re-fire logs `consumed without a turn: terminal (... head already consumed by a Texture turn)` |
| 19:26:14 – 19:28:21+ | one occurrence re-defers every ~35 s (`deferrals=9` at 19:28:21; poisoned at 64) |

State at 19:28: trajectory `live`; Texture work open; **four research
work items open**, all research agents with no active run; every research
report `update_applied`.

So research ran and reported, Texture saw each report (an activation ran
for it), and Texture kept choosing to wait. The research work items stay
open after their reports are applied: nothing settles them. A Texture
that waits while research work is open waits forever, and its waiting
re-spawns research.

This is the same research-work-left-open state as
`texture-settled-work-refuses-owner-revise-2026-10-09` ("Second effect").

## 3. The consumption path, confirmed

The new log lines (8a3d31c8) confirm H5 of
`texture-create-occurrence-deferred-never-refires-2026-10-09` on staging:
an activation that ends without disposing its trigger defers; its re-fire
finds the head consumed by the activation's own `decide` turn and
consumes the occurrence as terminal without a turn. That is visible now;
whether it is wrong depends on the turn contract (below).

## Correction (19:33Z): T6's cause is the search plane, not Texture

The committed turn reasons (trajectory events) say why Texture waited:
every research desk returned a blocker with no claims and no sources
because live web search was unavailable (`search_outage`, zero results).
The gateway confirms a plane-wide cooldown from 19:24:26
(`search-plane-cooldown-on-caller-cancel-2026-10-09.md`). Texture's
`wait_for_evidence` was the right call. The open research work items
(§2 state) are still an O24 gap, but they did not cause T6; the earlier
reading in §2 ("a Texture that waits while research work is open waits
forever") was an inference the trace did not support.

## T5b: what a fix touches (19:41Z, code reading; no decision yet)

`CancelLifecycleTrajectory` (`store/lifecycle.go`) is one transaction:
trajectory to `cancelled` (terminal), every open work item to `cancelled`,
every pending worker update to `cancelled`. The owner revise
(`textureowner/texture_agent_revision.go`) refuses any trajectory that is
not live, and its reopen (4f9331cf) only reopens Texture work the desk
**completed**; cancelled work still refuses, by the August rule.

Two shapes, both red (Texture canonical lifecycle):

- **B, turn-level cancel.** A new store command cancels open work and
  pending updates and fences runs, but leaves the trajectory live; the
  revise reopen accepts cancelled Texture work. Risk: whether settlement
  reconciliation then settles a live trajectory whose work is all
  terminal (the residual named in
  `texture-settled-work-refuses-owner-revise-2026-10-09`).
- **B2, a new episode per terminal trajectory.** Cancel keeps its present
  meaning. An owner revise on a document whose trajectory is terminal
  (cancelled **or** settled) starts a new trajectory bound to the same
  document. This also closes the settled-trajectory residual. Cost:
  `StartLifecycle` requires the document not to exist and the desk agent
  id is `texture:<doc>`, so it needs a new store transaction that rebinds
  an existing document and its agent to a new trajectory.

B2 removes two failure classes with one rule (a live document is always
revisable, O23) and leaves the cancel transaction alone; B changes a
transaction with pinned semantics. Leaning B2. Named residual
`texture-terminal-trajectory-revise`; T5b stays a soft check until it
lands. Not started tonight in favour of the search-plane fix, the suite
rerun, and the Gate 2 reality probe.
