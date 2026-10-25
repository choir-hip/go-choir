# CI0 C4: what the Node B build spends, and an incremental proposal (2026-10-24)

Station: CI0, slice C4 (docs/definitions/ci-test-cooptimization-2026-10-24.md).
Mutation class of this note: green. The build itself is red (prebuild-staging,
deploy-staging, flake.nix); this is findings and a proposal for FP0 to build or
adjudicate. Nothing here was changed.

## Findings (CI logs of runs 40138087758, 40136125633, 40132893674, 40136421316)

1. **The prebuild is the deploy floor and is always the whole host closure.**
   All four runs prebuilt `Frontend bundle` (16-17 s) and `Host NixOS closure`
   in parallel; the closure took 403, 418, 434 and 434 s. The
   service-pointer path in `node-b-ci-prebuild` (build only
   `.#<service>` for `HOST_SERVICES`) was not taken in any of them:
   `deploy_host_os=true` in the baseline run. Whether code-only pushes ever
   take it is a classifier question (`deploy-impact-classify`), not measured
   here: it is the first thing to check, because a service-only prebuild skips
   the guest image and the systemd units.
2. **Deploy after the build is small.** The deploy job's `nix build` phase was
   11 s because the prebuilt result was reused; the switch 2 s; service
   activation 2 s; the job 85-92 s in total. Push to deployed is therefore
   test-end or prebuild-end (whichever is later) plus about 1.5 min.
3. **What the closure builds (baseline log):** 88 derivations and 91 fetched
   paths (561 MiB from cache.nixos.org). The 88 are: `go-choir-services`
   (`-go-modules` and the one shared Go compile of 22 `cmd/` packages,
   `goServicesAll` in flake.nix), about 20 thin per-service wrappers, the
   guest image chain (`libguestfs-appliance`, `microvm-bootdisk.img`,
   `erofs-utils`, `microvm-store-disk.erofs`, `go-choir-guest-image`), and
   dozens of systemd unit and `system-path` derivations. Some build on
   `ssh-ng://root@node-a-builder`, some on Node B.
4. **No per-derivation timing exists.** The prebuild writes `nix build` output
   to a temp file and `cat`s it at the end, so every line of the log carries
   the finish time of the whole build (22:28:53 for 540 lines). The 418 s
   cannot be divided from the log. This is the main uncertainty.
5. **Every commit changes the Go compile's inputs.** `commonGoArgs.ldflags`
   embeds `buildCommit` and `buildDate`, so `goServicesAll` (and everything
   after it, including the guest image) is a new derivation on every commit,
   even a commit that changes no Go file. Nix has no incremental Go build:
   each rebuild compiles the whole dependency graph, embedded Dolt included
   (about 94 s alone in 2026-10-09's measurement,
   docs/problems/ci-deploy-latency-per-service-go-builds-2026-10-09.md).
6. **The prebuild runs `nix store gc` first** ("move GC off the deploy
   critical path"). It runs on the prebuild's own critical path, and it
   deletes unrooted build-time dependencies, which is one candidate reason
   the 561 MiB of qemu, libvirt, xen and similar are fetched again. At
   ordinary link speed 561 MiB is seconds to a minute, not minutes, so this is
   a hypothesis about a small term, not the main cost.

## Proposals, in the order I would do them

P0 (measure, red because it edits `node-b-ci-prebuild`, 5 lines): stream
`nix build` through a timestamp filter instead of buffering it:
`nix build ... -L 2>&1 | while IFS= read -r l; do printf '%(%T)T %s\n' -1 "$l"; done`
(or `--log-format internal-json` and a summary of `start`/`stop` events per
derivation). One run then shows the per-derivation times and settles findings
3, 4 and 6. Also print `nix store gc` duration. Without this every later
proposal is a guess.

P1 (incremental Go, the structural fix, red): split `goServicesAll` into
two derivations.
- `goDepsCache`: `go build` of every third-party package the services import
  (everything outside `internal/` and `cmd/`), with `GOCACHE=$out`. Its inputs
  are `go.mod`, `go.sum` and the list of `cmd/` imports of third-party
  packages, not the commit. It rebuilds only when dependencies change.
- `goServicesAll`: copy `goDepsCache` into a writable `GOCACHE`, then build
  the services. Only the repo's own packages (`internal/`, `cmd/`) compile,
  plus the link.
Expected: the dependency compile (Dolt, ICU cgo, protobuf and the rest) leaves
the per-commit path. The saving is the dependency share of the Go compile,
which P0 will state; the 2026-10-09 figure suggests most of it. Risk: the Go
build cache keys depend on the toolchain and flags (`-trimpath`, cgo env,
`-ldflags` are applied at link only, so `-X` stamping does not invalidate the
compile cache); the two derivations must use identical flags and
`CGO_*` environment (flake.nix lines 268-270 and the `buildInputs`). Check
byte-identical output against the current build (the SBOM differential
already fingerprints binaries) before cutting over.

P2 (cheap, red): take the commit and date stamp out of the compile
derivation's inputs. `build.json` already carries them in `mkGoService`;
`ldflags -X ...buildinfo.Commit/BuiltAt` is the only reason the Go compile
changes for a commit that touches no Go source (a test-only or `.github`-only
push that still deploys). With P1 in place this matters little, because the
repo packages are the only recompile; without P1 it saves a whole compile on
those pushes. Needs the runtime to read the stamp from `build.json` or from a
tiny second link step; a product decision, so FP0's.

P3 (start earlier, red): the prebuild starts at +60-110 s because it waits for
`deploy-impact` and a runner slot (docs/problems/ci-critical-path-race-
shards-2026-10-22.md, finding 3). It could start from `plan` for pushes that
are not docs-only, and cancel itself if deploy-impact says no deploy is
needed. That changes the prebuild's concurrency contract, so it is FP0's.

Not proposed: building the Go binaries on the GitHub runner with the Actions
Go cache and pushing store paths to Node B. It moves trust (an artifact built
off Node B enters its store) for a saving P1 should mostly deliver.

## What this does to the track's target

With the tests under about 6.5 min (C2), push to deployed is
`prebuild_start (about +1.5 min) + 7 min + 1.5 min deploy = about 10 min`.
Ten is the target; below it the prebuild has to shrink (P1) or start earlier
(P3). The 5-minute stretch needs P1 to take the closure under 3 min, or P3 and
P1 together.
