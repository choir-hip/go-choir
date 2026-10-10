# Management reads hours-old Pack claims as proof the current request was handled (2026-10-10)

Status: documented, not fixed. Mutation class of the fix: `orange` (desk
cell context: the acting pack's selection and fields; management prompt
text).

## Evidence

Minesweeper demo on staging at e1bcb393 (test computer
demo-ms-2026-10-10b, vm-8cac78e5…; traces in the session scratchpad under
traces/ms5). The owner prompt arrived at 21:14:08 and Texture sent
management an `execution_request` update (33b90280…) at 21:14:34. Management
then ran four turns (f00f108d, c43370c7, c3001bb6, 09254d84, 21:14:34 to
21:16:20). Each turn read `choir.Updates()`, which showed the request still
pending, and `choir.Pack()`. None of the four staged a cast or a report.
Each ended with a variant of "the pending carrier packet was already
processed in this run: reported to Texture, cast Engineering … I did not
duplicate the cast or report."

None of that happened in these runs. The Pack items the model cited
(`cell:376149b4…`, `cell:e78934df…`: "Management routed the requested
Minesweeper implementation … to Engineering", "I commit to route the
execution request to Engineering …") come from earlier attempts on the same
computer and the same document, at 19:07 and 20:38. Those casts were
cancelled at bind (documented in
delegated-cast-authority-dies-with-the-caster-turn-2026-10-10.md). All
those items still read `Discrepancy: unresolved`.

## Causes (from source)

1. `BuildActingPack` (`internal/types/commitment_views.go`) orders items
   contradicted-first, then by `RecordID`, and truncates to
   `actingPackMaxItems` (16). Record ids start with the cell id, which is
   random, so the pack is an arbitrary 16 of the desk's whole history, not
   its most recent acts.
2. `ActingPackItem` carries no time. The record has
   `Provenance.CommittedAt`, but the item drops it, so the desk cannot tell
   a claim from two hours ago, on a request since cancelled, from one made a
   minute ago. The document (ContextRef) is the same across attempts, so it
   does not separate them either.
3. A cast whose assignment is cancelled leaves the cast record and the
   reports around it unresolved. The ledger never says the claim
   "Engineering is tasked" turned out false. The cancel reached management
   only as a transient `blocker` update in an earlier turn.

## Belief state

Before: management fails to cast only when the cast path refuses it. After:
management can also decline to act because its own stale, unresolved
claims read like completed work. The trace shows the claims, the pending
update, and no staged intent. This is the model trusting a context surface
that misrepresents time and fate, not a model ignoring the request.

Fix direction: the pack shows the most recent acts first and stamps each
with when it was committed. The management prompt says a Pack claim is a
past commitment, not evidence that a pending update was handled. Cause 3
(assignment fate resolving the cast's claims) is the substrate-level
follow-up. It stays open as a named residual.
