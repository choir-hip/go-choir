# A computer running a layered app-layer release serves no SPA

Date: 2026-10-09. Reported by the owner (screenshot from choir.news, logged
in, about 02:22Z): the whole page is the text `self-development checkpoint:
served SPA is underivable`. Mutation class of the fix: red (served
computer surface; checkpoint frontend identity).

## Evidence

- Owner computer `candidate-fleet-e15cb89f…` applied the `16adefa1`
  app-layer release at 02:17:56. This is the first app-layer apply that did
  not roll back on this computer; the replay-page fix `16adefa1` removed the
  rollback. At 02:18:06 the console logs `actorruntime: computer surface
  baseline bootstrap deferred: self-development checkpoint: served SPA is
  underivable`.
- Every computer that ran a layered release logs the same line:
  - `vm-48bc0981…`: 20 times, first 2026-10-06 17:05;
  - `vm-c9015adc…`: 4 times, from 01:32 today (`c72c38c4` push);
  - `vm-2d8b21b5…`: 2 times.
- The release package has the SPA: Node B
  `/nix/store/9mjfysz4…-autoputer-0.1.0/frontend/index.html` exists
  (`flake.nix:221` copies the frontend into the package).

## Cause (code reading)

- `internal/autoputer/computer_surface.go` serves
  `CHOIR_UPDATER_ROOT/current/frontend`, else falls back to
  `CHOIR_BASELINE_RELEASE_ROOT/frontend`. The fallback is accepted only
  under `/nix/store/`.
- For a layered release, `current` is the updater release directory
  (manifest, `closure.nar`, entrypoint) with no `frontend/`. The boot
  wrapper sets `CHOIR_BASELINE_RELEASE_ROOT` to the release root inside
  the updater's private store, `/mnt/persistent/choir-updater/store/…`
  (`nix/autoputer-vm.nix:202`). The comment there says this is so "the
  computer surface serves the new release".
- The `/nix/store/` prefix check refuses that root, so there is no SPA.
  `trustedBaselineReleaseRoot` (`internal/agentcore/rematerialize.go`) has
  the same rule for checkpoint frontend identity.

S2 made app-layer pushes the deploy path for tracking computers, but no
acceptance step fetched the SPA of a layered computer. The deploy gate
checks `/health` and `build.commit` only.

## Fix direction

One trust rule for release roots, shared by the surface and checkpoint
code: a root is trusted if it is under `/nix/store/`, or under the
updater's private store (`$CHOIR_UPDATER_ROOT/store/`). The updater fills
that store only by verified nar replay of a signed offer (S2). Add an SPA
fetch to the app-layer deploy gate.

## Fix and residuals

Fix: the surface's read-only fallback trusts a baseline root under the
updater's private store as well as `/nix/store`
(`internal/autoputer/computer_surface.go`, test
`TestComputerSurfaceServesLayeredReleaseFromUpdaterStore`).

`trustedBaselineReleaseRoot` in agentcore was **not** widened on purpose.
Its caller `ensureServingBaseline` imports the root as a new `current`.
That would replace the app-layer release pointer the boot wrapper reads
its layering entrypoint through. Residuals:

1. **Checkpoints on layered computers.** Checkpoint frontend identity on a
   layered computer probably still reports "underivable": `current` fails
   `VerifyCurrentRelease` and the import path is refused. Not measured.
   Belongs to S6, which owns the release record.
2. **No SPA check in the app-layer deploy gate.** The gate checks
   `/health` and `build.commit` only, so this outage passed a green
   deploy. Add an SPA fetch.

## Status (2026-10-09 02:35Z)

Deployed `39c0d991` (CI run 37874573568): `app-layer push: 3/3 applied,
time-to-healthy=106s`. The owner computer layered the release at 02:33:38.
A direct fetch from Node B to the owner guest (`10.200.13.2:8085`) returns:
`/` 200 (Choir SPA), `/desktop/texture` 200, `/assets/index-DWfAnd8o.js`
200 (270,756 B), `/health` `ready`. **Fixed-verified** for the served SPA.
Owner browser confirmation pending. Residuals 1–2 open.

## Residual 1 measured: every boot of a layered computer spends ~17 s retrying (2026-10-09 18:19Z)

Found while checking O7 (boot cost against history,
`docs/evidence/o7-boot-cost-vs-history-2026-10-09.md`). Owner computer
`candidate-fleet-e15cb89f…`, boot at 17:52:22, console log (timings only):

- `starting server on 0.0.0.0:8085` at 17:52:22;
- `computer surface baseline bootstrap deferred: self-development
  checkpoint: served SPA is underivable` at 17:52:39;
- the first runtime boot phase begins right after, at 17:52:39.

The 17 s is the boot loop in `internal/actorruntime/adapter.go`
(`EnsureComputerSurface`, 10 attempts, 250 ms apart). Each attempt runs
`updater.VerifyCurrentRelease` on the layered release (about 1.5 s) and
then fails the same way: `current` is not a full release, and
`trustedBaselineReleaseRoot` refuses the updater-store baseline (kept
narrow on purpose, see above). The verdict depends only on disk state, so
attempts 2 to 10 cannot succeed. The other computer measured (3.1 GB,
10:58Z boot) shows the same ~16 s gap between lifecycle reconcile and
runtime start.

Console logs on Node B show the deferred line on 5 computers (owner
computer 18 times). The served SPA is unaffected (the read-only fallback
fix above); the cost is boot time (runtime start, and so Texture and the
desks, ~17 s later on every boot) and residual 1 itself: checkpoint
frontend identity on a layered computer is underivable, which Gate 2's
self-development checkpoints depend on.

Correction to the O7 reading: the boot term that looked proportional to
stored data is this retry loop on layered computers, not a history scan.

Fix direction (this doc first, fix second): retry only while the verdict
changes. The same error twice in a row is a state verdict; stop and log
it once. Residual 1 proper (a trusted checkpoint identity for layered
releases) stays with S6 / Gate 2.
