# The delegate builds in one round: dependencies from the image, no new egress

Date: 2026-10-21. Status: revised after the security panel; written before
the code. Owner decision: owner notes section 34 ("yes, ill go with your
recommendatation for #1"), option 1 of failure 26 in
[engineering-completion-path-cluster-2026-10-20.md](../problems/engineering-completion-path-cluster-2026-10-20.md).
Mutation class: red (capsule build environment, guest image closure).

## 1. Problem

A Claude delegate round runs in a `claude`-mode capsule whose egress
admits the model API and the host Go module proxy only
([claude-delegate-slice1-2026-10-14.md](claude-delegate-slice1-2026-10-14.md)
section 3). A frontend rewrite needs its npm dependencies, so every
delegate rewrite needs a second, `ecosystem_proxy` round, and that
handoff takes three model-mediated desk turns. Eight SlidesApp drills
stopped in it (failures 19 to 26); the delegate's own work never failed.

## 2. What the panel found in the first design

The first design (commit 37fa347e8) opened registry egress inside the
delegate's capsule after a one-way latch: kill the delegate, gate the
changed paths, widen the proxy, build. gpt-6.1-sol and gemini-3.8 both
returned "build with changes"; devin returned no verdict. Their findings,
all adopted by not building that design:

- The gate missed a path. Jobs run with HOME on a scratch mount outside
  the overlay's upper layer, so a file the delegate left there could
  make the trusted build run delegate code while the registry was open
  (gpt-6.1-sol reproduced the mechanism locally). P0/P1.
- The broker drops the capability a nested PID namespace needs, so the
  delegate kill could not be built as written; a cgroup kill would be
  needed. P0.
- A policy swap does not close tunnels already open, and nothing revoked
  the grant when a build failed. P1.
- Symlinks and overlay whiteouts were outside the patch diff the gate
  reused. P1/P2.
- A weakness in the egress proxy itself, present today for every
  `ecosystem_proxy` capsule, recorded in
  [egress-proxy-tunnel-identity-2026-10-21.md](../problems/egress-proxy-tunnel-identity-2026-10-21.md)
  with the detail kept out of this public repo until it is remediated.

gpt-6.1-sol's direction: separate dependency acquisition from execution,
and execute offline. Taken one step further, the acquisition does not
need to happen in the capsule at all.

## 3. Design

The frontend's dependencies are a pure function of
`frontend/package.json` and `frontend/package-lock.json`. The Nix build
already fetches them by hash for the frontend package (`flake.nix`,
`frontendPkg`, `npmDepsHash`). So:

1. **A dependency closure in the guest image.** `flake.nix` gains
   `frontendDeps`: the same `buildNpmPackage` inputs, no build step, its
   output is `node_modules` plus the two package files it was built from.
   The guest's autoputer service gets `CHOIR_FRONTEND_DEPS` naming that
   store path, which puts it in the guest closure.
2. **Carried to capsule workloads.** `CHOIR_FRONTEND_DEPS` joins the
   build-toolchain carry (executor to broker to cells, exec and jobs), as
   the cgo variables already do. The capsule mounts `/nix/store`
   read-only, so the path resolves inside every capsule and nothing in a
   capsule can change it.
3. **The build job uses it when the package files match.** The engineering
   overlay's frontend build job links `frontend/node_modules` to
   `$CHOIR_FRONTEND_DEPS/node_modules` when `frontend/package.json` and
   `frontend/package-lock.json` are byte-equal to the copies in the store
   path, then runs `npm run build` and the release copy; the existing
   cleanup (`rm -rf node_modules dist`) removes the link either way.
   When they differ (a rewrite that changes dependencies), it runs
   `npm ci` as today, which in `claude` mode fails for want of the
   registry, and the two-round path remains.
4. **Prompts.** Engineering's overlay says a `claude`-mode round builds,
   freezes and completes itself when the dependencies are provided; the
   management overlay says a delegate frontend rewrite needs one round.

## 4. What this grants

No egress changes. The `claude`-mode capsule keeps its policy (model API
and the Go proxy) for its whole life. Code that runs during the build
(npm, Vite, the configs, the source-contract script, anything the
delegate left under `/tmp`) runs with exactly the network the delegate
already had, so a delegate payload that the build executes gains no
authority it did not hold while it ran. The dependencies are
hash-pinned by the Nix fetch and read-only. The one new thing in the
capsule is a read-only directory of packages, the same packages today's
round 2 downloads.

Cost: about 133 MB more in the guest closure.

## 5. Failure modes (the tests, written before the code)

1. `CHOIR_FRONTEND_DEPS` does not reach a capsule job, or reaches it when
   it is not an absolute store path.
2. The build job links the dependencies when `package.json` or the
   lockfile differs from the store copy.
3. The build job leaves `frontend/node_modules` (link or directory) or
   `dist` behind, so the freeze carries them.
4. The build job uses `npm ci` when the files match (it must not need the
   network).
5. The `claude`-mode egress policy changes.
6. The Nix derivation's package files differ from the repo's (the
   closure would never match).
7. A build that fails still leaves the tree clean and exits non-zero.

## 6. Rollback

Git revert of the mission commits. The two-round path is unchanged and
still works with the change reverted.
