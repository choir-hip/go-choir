# Checkpoint identity on a layered computer blocks every second change (2026-10-10)

Status: open. Gate 2 track B. Mutation class of a fix: red (checkpoint
frontend identity, release record).

## Why it is on the critical path now

- Platform app-layer pushes were removed on 2026-10-09
  (`.github/scripts/deploy-impact-classify`; version skew). Layering
  now happens only through self-development releases.
- So every self-development apply makes the computer layered. If a
  layered computer cannot mint a checkpoint, then the first promoted
  change works, and the second change (or a restore pinned after the
  first) is blocked. This is not specific to the owner computer.

## Evidence (findings)

- The owner computer's boots at 06:26:30Z and 07:05:57Z on `cf0969cf`
  both log `computer surface baseline bootstrap deferred:
  self-development checkpoint: served SPA is underivable`. That is 20
  occurrences in its console (states and timings read only).
- The message has no `(current release: …)` suffix. So it comes from the
  baseline-import fallback in `ensureServingBaseline`
  (`agentcore/rematerialize.go`): `updater.VerifyCurrentRelease` failed,
  a trusted baseline root exists, and then building or importing the
  baseline manifest failed with a bare error. Import is refused on
  purpose so it never replaces a layered `current` pointer
  (`layered-release-spa-underivable-2026-10-09.md`).
- A layered release does carry its built frontend. The updater copies
  `<layer>/frontend` into the staged release dir (d32a7b23, Oct 5), so a
  self-development UI change is served.

## Hypotheses (to measure, not assumed)

- H1: the frontend identity hashes only the `frontend/` entries listed
  in the release manifest (`FrontendIdentityFromReleaseFiles`). A
  layered offer may list only an inline `frontend/index.html`, or none,
  so the identity is either weak (it misses assets) or underivable.
- H2: the owner computer's `current` is an older layered release from
  before S2-e, whose layout fails `VerifyCurrentRelease`. Its disk is
  not read.
- The bare import-path errors hide which step refused. They need names,
  as the verify path already has.

## Next observation

After the first apply on a disposable computer (M11), mint a second
checkpoint and record its result and error. The probe gains that leg.
This measures H1 on a computer we may inspect, without the owner's.
