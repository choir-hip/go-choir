# Doc Authority Witness and Heresy-Precision Drift

**Date:** 2026-08-01
**Status:** observed; repair committed in this mission
**Classification:** substrate — documentation-governance and detector precision
**Mutation class of the exposing change:** green/yellow documentation-truth repair
**Mutation class of the planned repair:** green docs + yellow `cmd/doccheck`

## Problem

Three distinct problems share one root cause: the documentation-governance
substrate (manifest witnesses + heresy detector precision) drifted from the
codebase it validates.

1. **Stale manifest witnesses.** `internal/runtime` was dissolved in commit
   `c791a0ae` (2026-07-14); the actor runtime is the only execution substrate
   and the code lives in `internal/agentcore`, `internal/textureowner`,
   `internal/coagentowner`, and `internal/actorruntime`. Three manifest entries
   still cite `internal/runtime` as a witness, so doccheck emits three R2
   "witness pattern matches nothing" warnings:
   - `docs/runtime-invariants.md` (witness `internal/runtime`)
   - `docs/texture-agentic-invariants-2026-06-13.md` (witness `internal/runtime`)
   - `docs/choir-prompting-invariants.md` (witness `internal/runtime`)

2. **Detector precision bug.** The H1 heresy scanner
   (`scanHeresyTerms`, `cmd/doccheck/main.go`) uses `strings.Contains` for
   term matching, so identifier-like terms match inside unrelated words. The
   `lease` family falsely matches `releases` (e.g.
   `docs/computer-ontology.md` "releases"), and `continuation-level` is
   flagged even when its `residue`/`transitional` qualifier is a different
   line. These are detector artifacts, not document defects.

3. **Design intent with no current home.** Two archived design docs
   (`design-conductor-supervision-protocol-2026-06-23.md`,
   `design-observer-hierarchy-2026-06-23.md`) carry "Revision 2026-07-31"
   banners asserting the supervision design is "still the intended shape."
   The archive is explicitly not current authority and is not in the default
   reading packet, so the intended-but-unbuilt supervision protocol has no
   current Target claim anywhere.

## Evidence

- `./scripts/doccheck --report doccheck-report.md` — 106 warnings total
  (H1=58, R2=3 among them).
- `git show c791a0ae --stat` — runtime package dissolved; files moved to
  `internal/agentcore`, `internal/textureowner`.
- `ls internal/agentcore/runtime.go internal/agentcore/tool_profiles.go
  internal/agentcore/qdrant_dedup.go` — surviving files.
- `internal/runtime` matches nothing in `witnessMatches`.
- H1 false positives reproduced on `lease`/`releases` and
  `continuation-level` split-line.

## Root Cause Belief

The actor-runtime cutover (H030 repair, 2026-07) changed the code layout and
deleted `internal/runtime`, but the manifest witnesses and the heresy detector
were not updated at the same time. The detector uses substring matching where
word-boundary matching was intended for identifier-like terms. The archived
supervision design was re-annotated as "still intended" without giving the
intent a current home, leaving Target claims stranded in the archive.

## Existing Replacement Opportunity

The replacement already exists: the live actor runtime packages
(`internal/agentcore`, `internal/textureowner`) are the correct witnesses, and
the supervision design intent has a natural home as a Target section in
`docs/current-architecture.md` plus a new current
`docs/supervision-protocol.md`. No new code is needed; the repair is rewiring
existing authority, not adding substrate.

## Bounded Repair Contract

The green/yellow repair touches:

1. `docs/doc-authority-manifest.yaml` — replace the three `internal/runtime`
   witnesses with the live packages; add the new supervision doc.
2. `docs/current-architecture.md` — note the runtime dissolution; add the
   Target supervision-protocol section.
3. `docs/conjecture-assertion-ledger-2026-06.md` — annotate A1/A5 receipts
   whose `internal/runtime` paths moved or were deleted.
4. `docs/heresy-detectors.md` — update the baseline scan note
   (`internal/runtime/prompt_defaults`).
5. `docs/source-external-data-publication.md` — update the conformance gap
   (`internal/runtime/qdrant_dedup.go`).
6. `docs/archive/design-conductor-supervision-protocol-2026-06-23.md` and
   `docs/archive/design-observer-hierarchy-2026-06-23.md` — banners become
   historical pointers to the new current doc.
7. `cmd/doccheck/main.go` + `cmd/doccheck/main_test.go` — word-boundary H1
   matching (yellow).
8. `docs/supervision-protocol.md` — new current Target design doc.

Settled definitions, evidence receipts, and `docs/runtime-dissolution-inventory.yaml`
are historical evidence and are not rewritten by this mission.

## Belief State

- Supported: the actor runtime is the only execution substrate; its live
  packages are correct witnesses.
- Supported: `lease` and `continuation-level` H1 hits in current docs are
  substring false positives.
- Rejected: "the archive is an acceptable home for still-intended Target
  claims" as the sole carrier of supervision design intent.
- Pending: doccheck R2 warnings drop to zero and the H1 count falls after the
  detector precision fix.

## Remaining Error Field

H1 warnings in settled `docs/definitions/*` and historical evidence remain
counted by design (historical vocabulary in evidence is legitimate); this
mission does not attempt to zero the corpus. The detector precision fix
removes only the substring false positives, not the legitimate historical
vocabulary signal.

## Rollback

Revert the doc edits and the yellow `cmd/doccheck` change independently. The
word-boundary change is isolated in `scanHeresyTerms`; reverting it restores
the substring behavior and the prior warning count. No runtime behavior,
route, or protected surface changes with this repair.

## Heresy Delta

- `discovered`: manifest witness drift, H1 substring false positives, and
  stranded Target design intent.
- `introduced`: none by this evidence record.
- `repaired`: witnesses re-pointed, detector precision fixed, supervision
  intent given a current home.
