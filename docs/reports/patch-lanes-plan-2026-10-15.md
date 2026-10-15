# Patch Lanes: the Next Six Slices

Date: 2026-10-15. Branch `ops/patch-lanes` (from `origin/main` 133de45fa),
not pushed. This note comes before any code in the branch (Problem
Documentation First). It names each problem, the evidence, the change, its
mutation class, its rollback, and what it needs to take effect.

Owner direction (2026-10-15): "we need to be updating regularly! min time to
patch is essential."

Source design: [the patch pipeline report](patch-pipeline-time-to-patch-2026-10-15.md),
rollout steps 2, 3 and 7. Already landed on `main`: the kernel lane
(`nixpkgs-kernel` on nixos-unstable, host and guest 6.18.55, `kvm_intel
nested=0`; 9cf6baf75), the draft report-only watcher
(`.github/workflows/patch-watch.yml`, 40d8aa5ee) and the reboot runbook and
script (`scripts/node_b_host_reboot.py`, ba104bd5b). The main `nixpkgs`
input is still the frozen NixOS 26.11 pre-release snapshot of 2026-07-01
(ec84054).

Measured for this note (2026-10-15):

- `nixpkgs` (ec84054): Go 1.26.4, Firecracker 1.15.1, rustc 1.95.0,
  govulncheck 1.5.0, nodejs 24.16.0.
- `nixpkgs-kernel` (nixos-unstable 7fd4004): Go 1.26.8, Firecracker
  1.16.1, rustc 1.98.1.
- go.dev: Go 1.26.9 is released (source tarball sha256
  `9735d7dc…fd4ca`). No nixpkgs ships it yet.
- Firecracker upstream: v1.17.0 and v1.16.2, both 2026-09-10. 1.17.0's
  `rust-toolchain.toml` pins rustc 1.97.0, so the base's rustc 1.95.0 is
  too old to build it; unstable's 1.98.1 is new enough.
- The Go 1.26.x recipe at ec84054 and at 7fd4004 differ only in `version`
  and `hash`: a version override of the base recipe is what unstable does.
- GHSA-hq66-cqwq-w95j (pdfjs-dist, high, published 2026-08-06): affected
  `>= 5.6.83, < 6.2.108`. Choir locks 5.7.284. Choir calls only
  `getDocument({ data })` and `GlobalWorkerOptions.workerSrc`; the 6.0
  `[api-major]` changes (no argument-less `getDocument`, no
  `PDFDocumentProxy.destroy`) do not touch either.
- `.github/workflows/`: 26 third-party `uses:` lines, all by tag; one by
  floating branch (`DeterminateSystems/flakehub-push@main`, with
  `id-token: write`).

---

## 1. The installed-but-not-booted detector (orange)

**Problem.** Deploys install kernels and never boot them. Node B ran
6.18.16 for 216 days with 6.18.37 installed, and nothing recorded it. The
deploy receipt's `host_identity.nixos_closure` names the activated system
as if it were live. The same gap now holds for `nested=0` and
`kvm.halt_poll_ns=0`: both are in the activated configuration and neither
is in effect until the host reboots.

**Change.** One fact, `reboot_pending`, computed from files the host
already has, with the reasons:

- `kernel`: `/run/booted-system/kernel` and `/run/current-system/kernel`
  resolve to different files;
- `kernel_modules`: the same for `kernel-modules`;
- `kernel_params`: a parameter in `/run/current-system/kernel-params` that
  is not on `/proc/cmdline`;
- `module_param`: a KVM module parameter declared in the current system
  (a `kvm*.param=value` kernel parameter, or an `options kvm* param=value`
  line in `/run/current-system/etc/modprobe.d/*.conf`) whose live value in
  `/sys/module/<module>/parameters/<param>` differs. `0`/`1` and `N`/`Y`
  compare as booleans when the live value is `N` or `Y`. A module that is
  not loaded is reported and does not count: its parameter applies when it
  loads.

Plus `pending_since` (the activation time of the first system generation
after the booted one whose boot inputs differ) and `pending_overdue` when
that is more than 7 days ago (the report's staleness floor). The running
kernel release and boot time are recorded with it.

Where it appears:

1. **Deploy receipt:** `host_identity.boot_state`, written by
   `python3 scripts/verify-host-hardening.py --boot-state-json` after the
   NixOS switch. The deploy log prints a `WARNING: host reboot pending`
   line with the reasons. A failure to compute it records an error object
   and never fails the deploy.
2. **Internal route:** `GET /internal/vmctl/host/boot-state` on vmctl
   (loopback plus the host-internal credential, like every
   `/internal/vmctl/*` route). It is computed per request, so it is never
   stale. Not on public `/health`: a public "kernel X, reboot pending"
   tells an attacker which exploits work.
3. **`scripts/verify-host-hardening.py`:** a `boot` section. Each reason is
   a FAIL unless `--allow-reboot-pending` (for the baseline before a
   planned reboot). Independently of what is declared, `kvm_intel nested`
   must read `N` (the security floor).

Two implementations of one contract: Go (`internal/hostboot`, for vmctl)
and Python (the verify script, which runs as `python3 -` over ssh and
cannot import). A Go test runs the Python on the same fixture trees and
requires the same answer, so the two cannot drift silently.

**Ways it could fail (written before the code).**

- The links are relative or chained (`/run/booted-system` to a store path
  whose `kernel` is itself a link): compare fully resolved paths, not
  `readlink` text.
- `/proc/cmdline` carries extra tokens (`BOOT_IMAGE=`, `init=`, the
  bootloader's own) and a different order: compare the declared set
  against the live set, never string equality.
- `kernel-params` ends with a newline or has repeated spaces: split on
  whitespace.
- Module names use `-` in modprobe (`kvm-intel`) and `_` in sysfs: normalize
  to `_`.
- Booleans: declared `0`, live `N`. Numbers: `halt_poll_ns=0` live `0`.
  Compare as booleans only when the live value is `Y`/`N`.
- A module not loaded (`/sys/module/kvm_amd` on an Intel host): not a
  pending reboot.
- `/run/booted-system` missing (a container, a test machine): report
  `available: false` with the error; never claim `reboot_pending: false`.
- Profile generations garbage-collected so the booted one is absent:
  `pending_since` falls back to the first generation that differs from
  the booted inputs, or is omitted. It never blocks the main answer.
- vmctl's sandbox (`ProtectSystem=strict`, `ProtectKernelTunables` off for
  vmctl) must still read `/run`, `/proc`, `/sys/module` and
  `/nix/var/nix/profiles`. All are read-only reads.
- The deploy receipt must not fail a deploy because the detector failed.

**Rollback.** Revert the commit. All reads are read-only; the receipt gains
one field that nothing parses yet.

**Takes effect.** The receipt field on the next host deploy (workflow
change). The route on the next vmctl deploy. The verify script at once.
On Node B today the expected answer is `reboot_pending: true` with reasons
`kernel`, `kernel_modules`, `kernel_params` (`kvm.halt_poll_ns=0`) and
`module_param` (`kvm_intel.nested`, `kvm.halt_poll_ns`), until the reboot.

## 2. Firecracker 1.17.0 lane (red: VMM, snapshot validity)

**Problem.** Node B runs Firecracker 1.15.1, which no longer receives
fixes. No nixpkgs has 1.17.0. 1.16.0 added official support for 6.18 host
kernels, and 1.17.0 adds official support for 6.18 guest kernels and
fixes MSR reads under 6.18 hosts (#6120; Choir uses no CPU template, so
that fix is not load-bearing here). Choir moved host and guest to 6.18.55.

**Change.** A `nixpkgs-vmm` input (nixos-unstable, locked to the same
revision as `nixpkgs-kernel`, so no extra download) whose `firecracker`
recipe (1.16.1, rustc 1.98.1) is overridden to the upstream `v1.17.0` tag
with a pinned source hash and a pinned cargo vendor hash. Host and the
`VM_FIRECRACKER_BIN`/`VM_JAILER_BIN` paths use it. An assertion keeps the
version at or above 1.17.0. When a channel ships 1.17.0 or later, the
override retires itself (it applies only when the channel is older).

**Cost.** A VMM change invalidates every Firecracker snapshot
(`snapshotReasonFirecracker`): every hibernated computer's next wake is a
cold (crash) boot, and running computers move to the new VMM only when
their VMM restarts (deploy refresh for idle ones; busy ones defer). It is
also a Rust build on Node B.

**Prepared on branch `ops/firecracker-1.17`**, stacked on this branch. Not
in `ops/patch-lanes`.

**Ship with the morning reboot? Yes, in the deploy just before it**, on
these grounds:

- The host reboot already invalidates every snapshot (the host kernel
  release is part of snapshot validity, `snapshotReasonHostKernel`) and
  restarts every VMM. Shipping 1.17.0 in the same window costs zero extra
  cold boots. Shipping it later costs a second round of cold boots for
  every computer that hibernated in between.
- A VMM is a deploy-revertible change; the host kernel is not. Deploying
  1.17.0 first, before the reboot, lets the deploy refresh boot idle
  computers on 1.17.0 under the old host kernel as a canary. If they fail
  health, revert that deploy before rebooting.
- Against: two changes in one window make a failure harder to attribute.
  The canary step above separates them: 1.17.0 on the old kernel first,
  then the new kernel with 1.17.0.

If there is no time for the canary step, reboot first and ship 1.17.0 the
same day.

**Rollback.** Revert the commit and deploy (snapshots taken under 1.17.0
are then refused, and those computers cold boot once more).

## 3. Go toolchain lane (orange: every binary's compiler)

**Problem.** Every service is built with Go 1.26.4 from the frozen base. A
`govulncheck ./...` run found 22 reachable standard-library findings
(`net/http`, `crypto/tls`, `html/template`, `os.Root` escapes and others),
all fixed by Go 1.26.9. CI tests run on Go 1.26.2 (the `go` directive in
`go.mod`, read by `setup-go`) and 1.26.4 (pinned in three jobs); production
uses 1.26.4. Three compilers.

**Change.** `choirGo` in `flake.nix`: the base `go_1_26` recipe with
`version = "1.26.9"` and its source hash, applied only while the base is
older (it retires itself when the weekly base bump brings 1.26.9 or newer).
An assertion keeps the floor. All Choir Go builds (`goServicesAll`, `zot`)
use `buildGoModule.override { go = choirGo; }`; the dev shells, the host's
`go` and the guest's `go` (engineering capsules build Choir code with it)
use `choirGo`. `go.mod` gains `toolchain go1.26.9`, so every `setup-go`
job that reads `go.mod` installs 1.26.9; the pinned `1.26.4` jobs move to
1.26.9. `GOTOOLCHAIN=local` (nixpkgs default, and set in the guest) keeps
the toolchain line from triggering a download anywhere.

`govulncheck` in CI as a **reporting** job: runs when the Go lane is
selected, never gates `check`, writes its findings to the job summary.

**Not in this slice.** Go modules (grpc, otel sdk, x/net, x/text; 4
findings) are the application dependency lane. Caddy and Dolt are built by
nixpkgs with the base Go; they get 1.26.9 with the base bump.

**Rollback.** Revert; the build returns to the base Go.

**Takes effect.** The next deploy rebuilds every service and the guest
image (a Go compile on Node B first, then the usual builds).

## 4. Frontend: pdfjs-dist to 6.4.299 (orange)

**Problem.** GHSA-hq66-cqwq-w95j, "arbitrary JavaScript execution upon
opening a malicious PDF", affects the locked pdfjs-dist 5.7.284. Choir's
PDF and Slides apps open untrusted PDFs.

**Change.** pdfjs-dist `^6.4.299` (latest; the fix is 6.2.108) in
`package.json`, both lock files (`pnpm-lock.yaml` for CI and development,
`package-lock.json` for the Nix build) and `npmDepsHash`. `npm audit` as a
reporting CI job when the frontend lane is selected.

**Rollback.** Revert.

**Takes effect.** Next deploy (frontend and the autoputer's embedded SPA).

## 5. Weekly full nixpkgs bump workflow (yellow)

**Problem.** The base pin is 3.5 months old and moves only by hand.

**Change.** `.github/workflows/flake-bump.yml`: weekly (and on dispatch),
`DeterminateSystems/update-flake-lock` pinned by commit SHA, scoped to the
`nixpkgs` input, opens or updates one PR. The PR's CI evaluates and builds
the host closure and guest image. That needs `PATCH_BOT_TOKEN` (a PR
opened with the default token triggers no workflows); without it the
workflow still opens the PR and says CI must be triggered by hand. Not run
here; `flake.lock` is not bumped.

## 6. Open-source hygiene (green and yellow)

- `SECURITY.md`: report through GitHub private vulnerability reporting.
  The repository setting itself is an owner action.
- Every third-party action in `.github/workflows/*.yml` and
  `.github/actions/*` pinned by commit SHA with the tag in a comment; the
  CI contract test enforces it.
- The patch-watch workflow linted with actionlint.

## Classes and order

| # | Slice | Class | Needs |
|---|---|---|---|
| 1 | boot-state detector | orange | deploy (route); receipt on next host deploy |
| 2 | Firecracker 1.17.0 | red | deploy; VMM restarts; separate branch |
| 3 | Go 1.26.9 lane | orange | deploy (Go compile on Node B) |
| 4 | pdfjs-dist 6.4.299 | orange | deploy |
| 5 | weekly base bump | yellow | `PATCH_BOT_TOKEN` secret |
| 6 | SECURITY.md, SHA pins | green/yellow | private reporting setting |

No slice needs a reboot to land. The detector exists to report what the
reboot will change.

## Status (branch `ops/patch-lanes`, not pushed)

- Landed on the branch: slices 1, 3, 4, 5 and 6. Commit messages record
  what each verified.
- Not verified locally: a Nix build of Go 1.26.9 (the remote
  aarch64-linux builder was unreachable, and the local build was
  stopped to free the Mac's CPUs), a browser render of a PDF with pdfjs
  6, and a run of the bump workflow on a hosted runner.
- **Next: slice 2 (Firecracker 1.17.0)** on `ops/firecracker-1.17`,
  stacked on this branch, per the design above. 1.17.0 needs rustc 1.97;
  the base has 1.95.0 and `nixos-unstable` (7fd4004) has 1.98.1, so the
  override starts from the unstable recipe through a `nixpkgs-vmm` input.
- Owner actions: the `PATCH_BOT_TOKEN` secret (slice 5) and enabling
  private vulnerability reporting in the repository settings (slice 6).
