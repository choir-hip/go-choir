# Clustering assessment: engineering's work product is not a durable object (2026-10-24)

Status: assessment, not a fix. Written by FP0 under the repo's root cause
clustering rule (CLAUDE.md, "Convergence Before Patching"): five records in one
subsystem within one day. Class of this record: green. Class of the substrate
fix it points at: red (self-development candidate state, the apply path,
the guest source tree).

## The five records

| # | Record | What was lost or broken |
|---|---|---|
| 1 | successive-selfdev-applies-do-not-compound-2026-10-24.md | Each capsule is seeded from the guest's original source, so an apply is not the base of the next run. A Files candidate would have reverted Slides. |
| 2 | long-engineering-run-dies-on-corpusd-cas-invalid-connection-2026-10-24.md (lane/fp0-apprewrite) | Two 30 to 40 minute runs died in a host stall (the platform Dolt, fixed by D36). A candidate persists only at Complete, so nothing survived. |
| 3 | staged-engineering-rounds-break-on-desk-reads-of-blocked-2026-10-24.md (lane/fp0-apprewrite) | `blocked` is a failure claim by design, so staged rounds read as failures; the carry is per trajectory and dies when the trajectory settles; a completed newest round ends the chain. Three good stages lost. |
| 4 | the v5 round (ledger 10:4x) | A good Freeze (212 KB patch) was lost because the run's Complete verb ("partial") was refused, the run was cancelled, and the frozen operation went to failed. |
| 5 | the 4 KiB cut (lane/fp0-apprewrite 11fc66878) | Engineering's view of the cast cuts the request at 4,096 bytes, with no way to read the rest, so a scope was silently missing. |

Also adjacent: texture-supervision-doc-stops-at-the-decision-boundary-2026-10-24.md
(a settled trajectory refuses the owner's next revision).

## Common cause

Records 1 to 4 share one substrate. Engineering's work product (the source
change) has no identity of its own. It exists only as a property of a run's
terminal fate (the candidate at Complete), of one trajectory (the carried
chain), and of the capsule's seed (the guest's original tree). Whatever ends
the run, the trajectory or the seed without the right verb loses the work:
a host blip, a desk's recovery round, a refused completion verb, a settled
trajectory, the next run's seed. Each symptom is a different exit from that
same missing object.

Record 5 is a separate, smaller defect (a context-surface limit), but it
fed record 4: the missing scope is what made the run end in "partial".

## Existing replacement, not wired

The update architecture R&D (docs/design/computer-update-architecture-rnd-2026-10-24.md)
already proposes the object: the computer's ordered patch stack, a
projection of the tape, which every apply extends and restore rewinds. Its
first slice U1 makes applies compound on one computer. Extending the same
object one step earlier, to unapplied work, would answer records 2 to 4: a
round's diff is recorded as a draft patch on the tape when it is produced
(at each cell's commit or at Freeze), carried by identity rather than by
trajectory, and a later round, a recovery, or a new trajectory starts from
it. The run's fate then says what happened to the work, not whether it
exists.

## What not to do next

Do not patch the symptoms one by one: a softer completion guard, a desk
prompt that reads blocked as progress, a longer cast limit, a per-trajectory
carry rescue. Each moves the loss to the next exit.

## Proposed order (for the owner's design day)

1. U1: applies compound (the guest source tree is the base plus the applied
   stack). Already the update design's first slice.
2. Draft patches as tape objects: a round's diff persists on its own,
   addressable, carried by id; Freeze references a draft, not the capsule's
   live tree.
3. A result for "good, continue" (or drop `blocked` as a carry path once
   drafts exist).
4. The 4 KiB cut: engineering reads the full request (a read verb, or the
   cast carries a reference to the record). Small, can go any time.

## Evidence

The conductor's log /Users/wiz/Dev/lanes/_runs/apprewrite/log.md (local);
the saved patches v4/stage3.patch and v5/patch.diff there; the five records
above; the ledger entries 05:2x to 10:4x.
