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
