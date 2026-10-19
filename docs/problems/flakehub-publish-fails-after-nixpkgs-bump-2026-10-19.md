# FlakeHub rolling publish fails after the nixpkgs bump: x86_64-darwin devShell (2026-10-19)

Mutation class: green for this document. The fix it names is orange (flake
output surface; no host closure or guest image change, see Fix).
Protected surfaces: none. The failing job only publishes the rolling flake to
FlakeHub; it does not gate `check`, deploy, or the Node B closure.

## Evidence

- CI run 39328756096 (push of merge `9d94241c6` to `main`), job
  "Publish Rolling Flake" (job 118321907533, `flakehub-publish` in
  `.github/workflows/ci.yml`), step `DeterminateSystems/flakehub-push@v7`.
  The same job succeeded on runs 39197635727 and 39188448647 (before the bump).
- The tool's own error: `failed to evaluate all system attrs of the flake`,
  running `nix flake show --all-systems --json --no-write-lock-file
  /nix/store/...-source` with status 1 (`src/flake_info.rs:140`).
- The decisive stderr, after `evaluating 'devShells.x86_64-darwin...'`
  started and the lock's new nixpkgs revision was unpacked
  (`github:NixOS/nixpkgs/fe13242`):

  ```
  error: ... while calling the 'throw' builtin
         at .../nixpkgs/fe13242/lib/trivial.nix:1003:37:  throwIf = cond: msg: ...
         error: Nixpkgs 26.11 has dropped support for x86_64-darwin.
  ```

- Local reproduction (darwin, Determinate Nix 3.16.0, clean checkout of
  `95c210918`, the merge's parent chain tip): `nix flake show --all-systems
  --json --no-write-lock-file .` fails with the same message, exit non-zero.
  It is a flake-eval failure, independent of CI, FlakeHub, or credentials.

## The 401 is noise

The log is full of `warning: unable to download
'https://cache.flakehub.com/nix-cache-info': HTTP error 401 ... "User is not
authorized for this resource."` and `error: unable to download
'https://edge.cache.flakehub.com/<hash>.narinfo': HTTP error 401`. Each is
followed by `copying path ... from 'https://cache.nixos.org'...`: Nix treats
the FlakeHub cache as an unusable substituter and falls back, and evaluation
continues (the `evaluating ...` lines keep coming, and `packages.x86_64-linux.*`
and `devShells.*` evaluate past them). Cause of the 401s themselves: not
investigated (the FlakeHub cache rejects this job's token). The green run
39197635727 prints no `nix flake show` stderr at all (flakehub-push only dumps
it on failure), so whether the 401s also occurred there is unknown; they are
harmless either way. The terminating error is the `throw` above.

## Cause

`flake.nix` defines

```
devSystems = [ "aarch64-darwin" "x86_64-darwin" "aarch64-linux" "x86_64-linux" ];
devShells = forDevSystems (devSystem: { default = mkDevShell devSystem; ... });
```

and `mkDevShell` calls `import nixpkgs { inherit system; }` for each. Normal
use only evaluates the caller's own system, so the `x86_64-darwin` entry was
never evaluated in our workflows. `flakehub-push` runs
`nix flake show --all-systems`, which forces every `devShells.<system>`.

The weekly bump (`44368ebdd`, merged by `9d94241c6`) moved `nixpkgs` from
`ec84054698e3` (nixos-unstable 2026-07-01) to `fe13242281a6` (2026-10-15).
The newer nixpkgs is 26.11, which `throw`s on import for `x86_64-darwin`
(release note: x86_64-darwin-26.11). The older snapshot did not. Hence green
before, red after, with no change to `ci.yml`.

Ruled out: the new `nixpkgs-qdrant` input and `choirQdrantModule` (they are
only reached through `nixosConfigurations.go-choir-b` on x86_64-linux, which
evaluated fine in the same log); IFD; `legacyPackages` across systems (our
flake does not re-export it); `prev.rustc.llvm.version` (that attribute is read
inside the module under `x86_64-linux` only). `packages.x86_64-linux.*`,
`nixosConfigurations.*`, `sbom`, and the other `devShells.*` all evaluated
before the throw in the CI log, and everything evaluates after the fix below.

## Options

1. Remove `x86_64-darwin` from `devSystems` (chosen). nixpkgs 26.11 does not
   support it and the base tracks unstable, so the shell cannot exist on this
   base regardless. Changes only the `devShells.x86_64-darwin` output, which
   was already unevaluable on the new lock. No Intel-mac developer shell
   remains; the 26.05 branch is the upstream's suggested path for such users
   and is out of scope. Comment in `flake.nix` says to re-add only if the base
   supports it again.
2. Keep the system but make it lazy-safe (for example `builtins.tryEval`, or
   gate on `nixpkgs.legacyPackages` having `meta.availableSystems`). More code
   to keep a shell that cannot build; rejected.
3. Make the CI step tolerant (`continue-on-error`) or skip `--all-systems`.
   Hides the real condition and publishes a stale or missing flake; rejected.
   (`flakehub-push` has no knob for the system list.)
4. Pin an `x86_64-darwin` shell to a 26.05 nixpkgs input. Adds a second base to
   maintain for a platform nobody on the team uses (the dev machine is
   aarch64-darwin); rejected.

## Fix and evidence

Option 1, a one-line list edit plus comment in `flake.nix`. Verified by
`nix eval --raw ...drvPath` over every `nixosConfigurations` entry
(`go-choir-b`, `go-choir-autoputer-vm`), every `packages.x86_64-linux`
attribute (including `guest-image`), and the x86_64-linux, aarch64-linux, and
aarch64-darwin dev shells: all 30 drvPaths are byte-identical before and after
(compared dirty tree to dirty tree, because `buildCommit` is `self.rev or
self.dirtyRev`, so a clean tree versus a dirty tree differs by commit identity
alone; `flake.nix` itself is not in the Go source filters). After the edit,
`nix flake show --all-systems --json --no-write-lock-file .` exits 0.

Not verified locally: the real publish (needs the GitHub OIDC token), so the
fix is confirmed only when the Publish Rolling Flake job goes green on `main`
after this lands.

## Heresy delta

- discovered: `devSystems` listed a system the pinned base cannot import, and
  nothing evaluated it until the publisher did; the weekly-bump build job
  (`nix build` of host and guest) cannot see this class of failure, so a bump
  can be green there and red at publish. A bump check that runs `nix flake
  show --all-systems --no-write-lock-file` would catch it on the PR.
- introduced: none.
- repaired: see the fix commit.

## Residual

Add `nix flake show --all-systems --json --no-write-lock-file .` to the
flake-bump PR checks (`.github/workflows/flake-bump.yml`) so the publisher's
precondition is tested before merge. Not done here; named for the next bump
slice.
