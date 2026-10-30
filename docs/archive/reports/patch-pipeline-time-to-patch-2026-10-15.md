# Choir's Patch Pipeline: Minimum Time to Patch

Date: 2026-10-15. Class: **green** (this document) plus a **yellow**
report-only CI workflow draft (`.github/workflows/patch-watch.yml`). No deploy,
no change to `flake.lock`, and no change to the main `nixpkgs` input. The
runtime work this design schedules is orange and red, and each slice carries
its own ceremony when it lands.

Owner request (2026-10-15): "we should turn nested virt off and update our
kernel and nixos and other systems. we need to be updating regularly! min
time to patch is essential. look into intelligent approaches here."

Inputs: [the host kernel problem](../problems/host-kernel-216-days-unpatched-2026-10-15.md),
[the boundary-strength ladder](boundary-strength-ladder-2026-10-15.md),
[the restart subsystem design](../restart-subsystem-design-2026-10-13.md),
`flake.nix`, `flake.lock`, `.github/workflows/ci.yml`, `nix/node-b.nix`,
`nix/autoputer-vm.nix`, `internal/vmctl/deploy_refresh.go`,
`internal/agentcore/planned_restart.go` and `internal/vmmanager/snapshot.go`.

**Builds on, does not duplicate:** branch `ops/kernel-6.18-security`, where
another agent is preparing the kernel-only pin, `kvm_intel nested=0` and a
host reboot script. That branch is the first emergency patch. This document
is the pipeline that keeps the next one from taking 216 days. Where this
design needs a reboot procedure, it extends that branch's script instead of
writing a second one. As of this writing the branch has no commits beyond
`main` (6e1039929), so its exact shape is **UNVERIFIED** here.

Labels: *measured* means read in this session (2026-10-14/15) from a live
source or the locked repo; *code* means read in this repo; **UNVERIFIED**
means a claim not checked against a primary source or the live host.

---

## 1. The short answer

Choir has no patch pipeline today. It has a deploy pipeline that patches as a
side effect, and the side effect stops at the kernel. Four facts set the
design:

1. **One nixpkgs pin moves everything at once.** The host kernel, the guest
   kernel, the Go compiler that builds every service, Firecracker, Dolt,
   Caddy, Qdrant, SearXNG and the guest's Python libraries all come from one
   `nixpkgs` revision, locked on 2026-07-01 (measured, `flake.lock`). So the
   riskiest component holds the cheapest fix hostage: a Go standard-library
   fix waits on a kernel-and-everything bump, and nobody does that bump
   because it might break everything. That revision is 3.5 months old.
2. **Deploys install kernels and never boot them.** `switch-to-configuration
   switch` (ci.yml, "Activating NixOS config") changes userspace and the
   boot-loader default; it never reboots. Node B has run 6.18.16 for 216
   days with 6.18.37 installed beside it. The deploy receipt records the
   NixOS closure as if it were live; it does not record that the kernel in
   that closure is not running.
3. **Nothing watches the feeds.** There is no Dependabot or Renovate
   configuration, no `govulncheck`, no `npm audit`, no `vulnix` in CI (code:
   grep of `.github`, `scripts`). `vulnix` is in `flake.lock` only as a
   transitive input of `sbomnix`.
4. **The guest side can already patch without anyone noticing.** A new guest
   kernel reaches each computer on its next cold boot. Snapshots refuse to
   resume when the kernel image file, the host kernel release or the
   Firecracker version changed (`internal/vmmanager/snapshot.go`,
   `validateSnapshot` and `snapshotBoundPaths`). Deploy refresh restarts idle
   computers under a planned-restart marker
   (`internal/vmctl/deploy_refresh.go`). The host reboot is the only step
   that has no machinery at all.

The design, in one paragraph: split the single pin into **lanes** with their
own cadence (kernel, VMM, Go toolchain, application dependencies, everything
else); run a **deterministic watcher** daily that compares what is declared
and what is *booted* against fixed versions from primary feeds (kernel.org,
the kernel CNA, CISA KEV, Go's vulnerability database, GitHub advisories);
make **installed-but-not-booted** a first-class health fact; make the host
reboot a **planned, drained, kexec-fast, self-reverting** operation that uses
the same planned-restart markers as deploy refresh; set **SLOs by severity**
measured from fix availability, recorded per advisory and, later, committed
as precommitment records. The research desk's agentic triage comes later and
is scored against the same feeds.

---

## 2. Where Choir stands (measured 2026-10-14/15)

The host kernel is the headline. kernel.org's `releases.json` lists 6.18.56
as the current 6.18 longterm release (released 2026-10-11). Node B boots
6.18.16 (problem doc, live read). The locked `nixpkgs` revision (ec84054698,
2026-07-01) carries 6.18.37 (measured from that revision's
`kernels-org.json`), which is what is installed but not booted. The
`nixos-26.05` branch carries 6.18.56 and `nixos-unstable` carries 6.18.55
(measured, same file on each branch). So stable nixpkgs had the newest 6.18
within three days of kernel.org, and newer than unstable.

The kernel is 40 stable releases behind. Running the draft watcher's join
of CISA KEV (catalog 2026.10.14) against the kernel CNA's per-CVE fixed
versions (section 4.1) over the last two years of KEV kernel entries gives
(measured):

- **booted 6.18.16 (host):** affected by three known-exploited bugs.
  CVE-2026-31431 "Copy Fail" (KEV 2026-05-01, fixed in 6.18.22),
  CVE-2026-53266 (KEV 2026-09-18, fixed in 6.18.36) and CVE-2026-53362
  (KEV 2026-08-27, fixed in 6.18.38). The first answers the ladder report's
  open question: Copy Fail's 6.18-stable fix is 6.18.22.
- **installed 6.18.37 (host) and running 6.18.37 (guests):** still affected
  by CVE-2026-53362. Rebooting into what is already installed is not enough.
- **6.18.56:** affected by none of them.

Measured against a 48-hour target for known-exploited kernel bugs, Node B is
more than five months late on Copy Fail (counted from its KEV date) and about 48 days late on
CVE-2026-53362. The watcher's other KEV kernel rows (CVE-2024-53104,
CVE-2025-38352, CVE-2025-39964 and others) were fixed in mainline before
6.18 and do not apply; pre-2024 CVEs have no kernel CNA record and are
reported for a human.

Firecracker is the second. Node B runs 1.15.1 from nixpkgs (ladder report).
Upstream released 1.16.2 and 1.17.0 on 2026-09-10 (measured, GitHub
releases API), and under Firecracker's release policy 1.15 no longer gets
fixes (ladder report). nixpkgs lags upstream here more than for the kernel:
`nixos-unstable` has 1.16.1 and `nixos-26.05` still has 1.15.1 (measured).
Following nixpkgs alone would never get Choir onto a supported Firecracker
quickly.

The Go toolchain is the third. The locked nixpkgs builds every service with
Go 1.26.4 (measured, that revision's `go/1.26.nix`); CI tests use 1.26.4
too (ci.yml `go-version`). A `govulncheck ./...` run in this session
reported 26 vulnerabilities that Choir's code reaches. 22 are in the
standard library (`net/http`, `crypto/tls`, `html/template`, `os`,
`net/url`, `encoding/xml`, `encoding/asn1`, `mime/multipart`), all fixed by
Go 1.26.9; five of those also appear through `golang.org/x/net` v0.55.0
(fixed in v0.60.0). The other four are modules: two in
`google.golang.org/grpc` v1.79.3 (fixed by v1.83.1), one in
`go.opentelemetry.io/otel/sdk` v1.43.0 (fixed in v1.45.0) and one in
`golang.org/x/text` v0.37.0 (fixed in v0.39.0) (measured, local run with Go
1.26.4; output not committed). Two of the standard-library findings are root
escapes in `os.Root` (GO-2026-4970, GO-2026-6604), which
`internal/actorruntime` uses. All 22 are one Go point release away and are not
shipped because the Go version is tied to the main nixpkgs pin.

The frontend is the fourth. `npm audit --package-lock-only --omit=dev`
reports one high-severity runtime advisory: **pdfjs-dist** below 6.2.108,
"arbitrary JavaScript execution upon opening a malicious PDF"
(GHSA-hq66-cqwq-w95j). Choir opens untrusted PDFs as part of its reading
surfaces, so this one deserves its own triage (the fix is a major-version
bump). Including dev dependencies, 11 advisories (5 high, 6 moderate)
(measured).

Other observations from the same pass, each a smaller lag:

- **Obscura** is the guest's headless browser, and it executes untrusted
  web JavaScript. It is built against `rusty_v8` v137.3.0, released
  2025-08-14; the latest is v152.2.0 (2026-08-20) (measured, GitHub releases
  API). V8 is the most exploited component in browsers. Inside the guest it
  sits behind rung 3 and 4 of the ladder, so this is a guest-level exposure,
  not a host one. It is pinned in `flake.nix`, outside every nixpkgs lane.
- **Dolt** 2.1.9 in the lock against upstream v2.4.2 (2026-10-06);
  **Qdrant** 1.18.2 against v1.19.2 (2026-10-05); **Caddy** 2.11.4 against
  v2.11.7 (2026-10-03); **SearXNG** at a 2026-06-20 snapshot (measured,
  locked nixpkgs `package.nix` files and GitHub latest releases). Whether
  any of these deltas contain security fixes is **UNVERIFIED**; Caddy is
  the public edge and SearXNG parses untrusted pages on the host.
- **CPU microcode** is final. The Xeon E-2236 (CPUID 906EA) reached 0xfa,
  and Intel's servicing ended 2025-06-30 (ladder report, Intel's servicing
  page). There will be no more microcode fixes. `nix/hardware.nix` loads
  microcode only when `hardware.enableRedistributableFirmware` is set; that
  the running value is 0xfa is from the ladder's live read.
- **CI supply chain:** `DeterminateSystems/flakehub-push@main` runs on a
  floating branch ref with `id-token: write` (ci.yml, "Publish Rolling
  Flake"). Every other action uses a version tag. A floating ref on a job
  that can mint OIDC tokens is a patching question in reverse: it updates
  without review.

---

## 3. Inventory: what needs patching and how each moves today

The inventory groups components by **how they reach a running process**,
because that is what decides time to patch. For each: where it is pinned, how
an update reaches production today, and what restart it needs.

### 3.1 Host kernel and KVM

The host kernel is `boot.kernelPackages` left at the nixpkgs default, so it
moves only with the main `nixpkgs` input. KVM is a module of the same kernel
(`kvm-intel` in `nix/hardware.nix`); its parameters (`nested`, `halt_poll_ns`
via `boot.kernelParams` in `nix/node-b.nix`) take effect at module load, so at
boot. An update reaches production in two steps that are never joined today:
a deploy that changes `flake.lock` installs it, and a host reboot (which no
procedure performs) runs it. Restart needed: the whole host, which stops
every computer.

The ladder ranks this as the weakest dated link under every boundary. KVM
bugs (Januscape, the unexplained Vercel "KVM 0day" of 2026-10-03) are fixed
only here; no Firecracker version or jailer setting helps.

### 3.2 NixOS host userspace and host services

Everything else in the host closure: systemd, OpenSSH, glibc, OpenSSL,
Caddy, Qdrant, SearXNG, the Dolt server, the `firecracker` package itself.
Pinned by the same main `nixpkgs` input. An update reaches production through
the normal deploy (`switch-to-configuration switch` restarts changed units;
Choir's own units use `restartIfChanged = false` and are restarted by the
deploy script). Restart needed: per unit, seconds. glibc and systemd updates
are live for new processes only; long-running daemons keep the old library
mapped until restarted (standard Linux behaviour). This lane is cheap to
apply once a bump is merged, and the bump is the bottleneck.

### 3.3 CPU microcode

No patch stream (section 2). Choir's only levers are kernel configuration
(SMT off or core scheduling, `vmentry_l1d_flush`, the mitigations the kernel
picks) and hardware replacement. The planned EPYC 7371 is no better: Naples
microcode in linux-firmware has been frozen since 2024-11-21 and EntrySign
needs a BIOS update (ladder report). The pipeline still tracks CPU
advisories, because each one needs a decision: a config mitigation, a
documented acceptance, or a hardware move.

### 3.4 Guest kernel

`mkGuestImage` takes `vmConfig.boot.kernelPackages.kernel.dev` from the
guest NixOS configuration (`flake.nix`), so the guest kernel also follows
the main `nixpkgs` input; guests were last seen on 6.18.37 (ladder report,
`docs/evidence/s0a-guest-layout-2026-10-01.json`). It reaches production as a
file under `/var/lib/go-choir/guest/` that node-b activation advances
atomically, and each computer picks it up on its next cold boot. This is the
one kernel Choir can patch without a host outage. Capsules share the guest
kernel, so the container-escape class of kernel bugs (CVE-2026-53362) lives
here as well as on the host.

### 3.5 Guest image userspace

The guest closure: the autoputer, updater and capsule broker binaries, the
Python libraries used to parse untrusted documents (`lxml`, `pdfplumber`,
`pypdf`, `python-docx`), Obscura (V8), zot. All but Obscura and zot follow
main nixpkgs; those two are pinned by hand in `flake.nix`. They reach a
computer the same way as the guest kernel: new image, next cold boot.

### 3.6 Firecracker

`pkgs.firecracker` from main nixpkgs (`nix/node-b.nix`,
`VM_FIRECRACKER_BIN` and `VM_JAILER_BIN`). Each running VM holds its own
VMM process launched from a fixed store path, so a new Firecracker reaches a
computer only when that computer's VMM restarts. Snapshots taken under
another Firecracker version are refused (`snapshotReasonFirecracker`), so the
first wake after a Firecracker update is a cold boot. Deploy refresh already
restarts idle computers; busy ones wait (section 6.6).

### 3.7 Go toolchain and Go modules

Two separate things. The **Go toolchain** builds every service through
`buildGoModule` with `pkgs.go`, so standard-library fixes follow main
nixpkgs. **Go modules** are pinned by `go.mod` and `go.sum`, vendored into
the build through `vendorHash`; an update is a normal source change (edit
`go.mod`, update `vendorHash`, deploy). Restart: the deploy's service-pointer
restart for host services, and a guest refresh for the autoputer.

### 3.8 Frontend npm dependencies

Pinned twice: `pnpm-lock.yaml` for development and CI, and
`package-lock.json` plus `npmDepsHash` for the Nix build (`flake.nix`). They
can drift. An update is a source change plus a hash refresh. The built SPA is
served by the host and embedded in the autoputer package, so it reaches users
on the next deploy (host) and the next guest refresh (computers).

### 3.9 Dolt, SearXNG, Qdrant, Caddy

All four are nixpkgs packages run as NixOS services on the host (`nix/node-b.nix`).
Two of them face untrusted input: Caddy is the public TLS edge, and SearXNG
parses arbitrary web pages from inside the host network namespace (rung 5 of
the ladder). Dolt is also linked into the Go services as a library through
`go.mod`, which is a separate pin. Restart: unit restart through the deploy.

### 3.10 Third-party services Choir calls

The gateway calls model providers through an OpenAI-compatible path and
search providers (Exa, Keenable, Parallel, Brave, Tavily, ChatGPT search);
maild calls Resend (code: `internal/gateway`, `internal/maild`). Choir
cannot patch these. What the pipeline can do is track their advisories and
deprecations, keep the client libraries current (Go modules lane), and treat
a provider breach as a credential-rotation event rather than a patch. This
lane is listed so it is not forgotten; it has no SLO beyond rotation.

### 3.11 The CI and build chain

GitHub Actions (versions by tag, one by floating branch), the Determinate
Nix installer, FlakeHub publishing, the root SSH deploy key. These update by
editing workflow files. They belong in the pipeline because a compromised
action reaches Node B as root through the deploy job.

---

## 4. Detection

### 4.1 What is deterministic now

Every check in this section is a comparison between two versions. No model is
involved and none is needed to start.

**Feeds and what each answers.** The pipeline reads primary sources first
and aggregators only for discovery:

- **kernel.org `releases.json`** gives the current release of each longterm
  branch with its date. It answers "how many 6.18 releases behind are we".
- **The kernel CNA's vulns repository**
  (`git.kernel.org/pub/scm/linux/security/vulns.git`) has one JSON record per
  kernel CVE with the first unaffected release on each stable branch (for
  example `{"version": "6.18.38", "lessThanOrEqual": "6.18.*", "status":
  "unaffected"}` for CVE-2026-53362; measured). This makes "is our kernel
  affected by CVE X" a deterministic lookup. The kernel CNA has issued kernel
  CVEs since 2024, so older CVEs in KEV need a fallback.
- **CISA KEV** (`known_exploited_vulnerabilities.json`) answers "is anyone
  exploiting it". Joined with the kernel CNA records it gives the single most
  important detector: *a KEV kernel entry whose fixed 6.18 release is newer
  than our booted or declared kernel.*
- **The Go vulnerability database through `govulncheck`** answers which
  reachable symbols in Choir's code are vulnerable, with the fixing Go or
  module version. It is reachability-aware, so its findings are already
  triaged better than a version scan.
- **`npm audit`** over `package-lock.json` (the lock the Nix build uses).
- **GitHub security advisories** for repositories whose binaries Choir runs
  from outside nixpkgs or ahead of it: `firecracker-microvm/firecracker`
  (two advisories in 2026, measured), `denoland/rusty_v8` and the Obscura
  fork, `patriceckhart/zot`. The REST endpoint
  `/repos/{owner}/{repo}/security-advisories` is public for published
  advisories (measured for Firecracker).
- **The NixOS security tracker** (`tracker.security.nixos.org`, reachable,
  measured) and **`vulnix`** for the nixpkgs closure. vulnix matches package
  names and versions against NVD and is known to be noisy (it cannot see
  backported patches without whitelists). The useful mode is a *diff*: new
  findings in the host or guest closure since the last accepted scan, not
  the full list. `sbomnix` (already a flake input) ships `vulnxscan`, which
  combines vulnix with other scanners over the same derivation; whether it
  scans an unbuilt derivation well enough for CI is **UNVERIFIED**.
- **oss-security** (Openwall) and **linux-cve-announce** for discovery of
  embargo lifts and write-ups before they reach structured feeds. These are
  human-read today; they are the natural input for the research desk later.
- **Upstream release feeds** (GitHub releases) for Firecracker, Dolt, Qdrant,
  Caddy, SearXNG. A new release with no advisory is a lag signal, not a
  vulnerability.

**The two comparisons.** Each feed is compared against two versions, and
they are different on purpose:

- the **declared** version: what the current `main` would deploy (evaluated
  from the flake, no build needed);
- the **running** version: what is booted or executing on Node B and in each
  guest.

The declared check runs on a GitHub runner with no access to Node B. The
running check runs on Node B. The difference between them is its own alarm.

**Cadence.** Daily for the kernel, KEV and Firecracker checks (cheap: a
flake evaluation and a few JSON fetches); daily for `govulncheck` and `npm
audit`; weekly for the full closure diff scan. The draft workflow on this
branch (`.github/workflows/patch-watch.yml`) implements the declared side:
the KEV-by-kernel-CNA join for both host and guest kernels, the 6.18 lag,
the Firecracker lag and advisories, `govulncheck` and `npm audit`, and opens
or updates one GitHub issue per finding class. It reads no secrets and does
not touch Node B. It is report-only.

### 4.2 The installed-but-not-booted detector

This is the gap that produced 216 days, and it is the cheapest to close.

On NixOS the facts are files. `/run/booted-system` is the system the host
booted; `/run/current-system` is the system last activated. The detector
compares, at minimum:

- `readlink /run/booted-system/kernel` against
  `readlink /run/current-system/kernel` (the kernel image);
- `/run/booted-system/kernel-modules` against the current one (a module
  update without a kernel change still needs a reboot to load);
- `/proc/cmdline` against `/run/current-system/kernel-params` (a
  `boot.kernelParams` change such as `kvm.halt_poll_ns=0` waits for boot);
- the live module parameters that Choir declares, read from
  `/sys/module/kvm_intel/parameters/nested` and
  `/sys/module/kvm/parameters/halt_poll_ns`, against the declared values
  (`boot.extraModprobeConfig` changes are invisible to the kernel-path
  comparison);
- `/sys/devices/system/cpu/smt/active` against the declared SMT policy, once
  one exists.

It produces one fact, `host_reboot_pending`, with the reasons and the age:
how long the host has been running a kernel older than the installed one
(the activation time of the first generation whose kernel differs from the
booted one, or the deploy receipt's `activated_at`).

Where it is recorded:

1. **The deploy receipt.** `host_identity` already records `nixos_closure`
   (ci.yml, deploy receipt). Add `booted_system`, `booted_kernel`,
   `current_kernel` and `reboot_pending`. The receipt then stops claiming a
   closure is live when its kernel is not. The deploy log prints a warning
   line on every deploy where `reboot_pending` is true.
2. **A host-internal health fact.** A small systemd timer (or vmctl, which
   already reads `uname -r` for snapshot metadata in
   `internal/vmmanager/boot_timeline.go`) writes
   `/var/lib/go-choir/patch-state.json` hourly. vmctl's loopback `/health`
   (or an internal route behind hostauth) serves it.
3. **Not the public `/health`.** The public proxy health is reachable by
   anyone. Publishing "kernel 6.18.16, reboot pending" there tells an
   attacker which exploits will work. The public surface gets nothing; the
   owner surface (Texture, later) and CI over its existing SSH channel get
   the detail.
4. **A threshold.** `reboot_pending` older than 7 days is a finding in its
   own right, with the same SLO machinery as an advisory (section 7).

The same comparison applies to computers. The guest updater already records
`kernel_release` and `boot_id` in its kernel capability probe
(`internal/updater/kernel_capabilities.go`). The per-computer fact is "this
computer runs guest kernel X, the image on disk has Y". vmctl can join it
with the guest image manifest (`guest-image-manifest`, which names the
kernel store path) to list computers still on an old guest kernel or an old
VMM binary. That list is the guest-side reboot-pending report.

### 4.3 What is agentic later

The deterministic layer answers "are we on a fixed version". It cannot
answer the questions that decide how fast to move when the answer is no:

- Is the vulnerable code reachable in Choir's configuration? (Is the
  subsystem compiled in? Is the device node openable by a DynamicUser
  service? The ladder lists Vghost's `/dev/vhost-*` reachability as
  **UNVERIFIED**, which is exactly this question.)
- Will this be exploited? (Most kernel CVEs never are; the kernel CNA issues
  thousands a year.)
- Is a write-up about to drop for a bug with no CVE yet (the Vercel case)?

The research desk is built for this: read-only world-evidence authority
(CLAUDE.md, desk ontology). The dogfooding slice is a **triage record per
advisory** written as a precommitment record
([docs/precommitment-records.md](../precommitment-records.md)): a typed
prediction committed before the evidence that resolves it. The predictions
should be ones the deterministic feeds can resolve, or scoring becomes an
argument:

- "CVE X is added to CISA KEV within 30 days: p = 0.12" resolves against
  the KEV feed.
- "A public proof of concept for CVE X appears within 30 days: p = 0.3"
  resolves against the advisory's references (weaker, needs a rule).
- "nixpkgs `nixos-26.05` ships the fix within 3 days: p = 0.8" resolves
  against `kernels-org.json` on that branch.
- "Choir's configuration reaches the vulnerable code: p = 0.6" resolves only
  when a red-team flag or a test settles it, so it is scored rarely. It is
  still worth recording because it drives the severity choice.

Brier scores over these records tell the owner whether the desk's urgency
is calibrated, which is the point of precommitment records. The desk's
output changes **ordering and severity class** inside the SLO, never whether
a known-exploited fix is applied. The deterministic floor (section 7) holds
no matter what the desk predicts. This is a later slice: it needs research
desk scheduling, feed access through the gateway, and a records store the
watcher can write to. It does not block anything before it.

---

## 5. Update automation

### 5.1 Lanes instead of one pin

The main structural change is to stop updating everything through one
input. Each lane has its own input or pin, its own cadence, its own build
cost, and its own restart. A lane can move without the others.

**Kernel lane.** A separate flake input, `nixpkgs-kernel`, used only for
`boot.kernelPackages` on the host and in the guest configuration. It follows
the current NixOS stable branch (`nixos-26.05` today, `nixos-26.11` after
that release), because on the evidence of this session stable carried the
newest 6.18 before unstable, and a stable branch takes fewer unrelated
changes. A kernel from another nixpkgs revision works beside the main
userspace in general: the kernel and its in-tree modules are self-contained;
out-of-tree modules must come from the same `linuxPackages` set; firmware and
microcode stay on the main input. Binaries come from cache.nixos.org once the
branch's channel has advanced, so the lane costs no kernel compile. Cadence:
a daily automated PR when the branch's 6.18 version changes. The
`ops/kernel-6.18-security` branch's kernel-only pin is the first instance of
this lane; this design asks that it be expressed as that input (or moved to
it in the next slice), so the daily bump is a lock update rather than a
hand edit.

**Emergency kernel path.** When a known-exploited fix is on kernel.org but
not yet in a nixpkgs channel, the lane overrides `linux_6_18` with the
kernel.org tarball version and hash (`buildLinux` or an
`overrideAttrs`-style version bump of the nixpkgs kernel derivation). That
means compiling the kernel ourselves; on Node B's six cores a full NixOS
kernel build is roughly an hour (**UNVERIFIED**, not measured). It is used
only when the channel is behind and the bug is known-exploited; on this
session's evidence the channel lag was about three days, so this path is
expected to be rare.

**VMM lane (Firecracker).** nixpkgs is behind Firecracker upstream on both
branches (section 2), and Firecracker drops support for old minor versions.
Package Firecracker in `flake.nix` from the upstream release (either the
signed release tarball with a pinned hash, or `pkgs.firecracker.overrideAttrs`
with the upstream tag and a new `cargoHash`), tracked by a watcher that opens
a PR on each upstream release. A Firecracker change invalidates every
snapshot, so its rollout is a deploy plus guest restarts (section 6.6). The
first instance is the 1.15.1 to 1.17.0 move the ladder already ranks fifth.

**Go toolchain lane.** Build services with an explicit Go: either
`buildGoModule.override { go = <newer go>; }` from the kernel lane's stable
input or `nixpkgs-unstable`, or a `go` override to the latest 1.26.x point
release. Go point releases carry only security and critical bug fixes by
policy, and they are the cheapest fix in the whole inventory: 22 of the 26
reachable findings close with one (Go 1.26.9).
CI's `go-version` moves in the same commit so tests and production use one
compiler. Cadence: on each Go point release (monthly, plus out-of-band
security releases).

**Application dependency lane (Go modules, npm).** Renovate or Dependabot,
grouped weekly, with security updates opened immediately. Renovate handles
`vendorHash` and `npmDepsHash` poorly without custom post-upgrade tasks
(**UNVERIFIED** for the current Renovate version); a scheduled workflow that
runs `go get -u=patch`, recomputes `vendorHash` from the Nix build error, and
opens a PR is simpler and fits the repo's existing pattern. The
`pnpm-lock.yaml` / `package-lock.json` pair should be reconciled to one
source of truth, or the PR must update both.

**Everything else (main nixpkgs).** A weekly
`DeterminateSystems/update-flake-lock` PR for the remaining inputs (main
`nixpkgs`, `microvm`, `sbomnix`). The action (v29, 2026-09-09, measured)
takes an `inputs` list, so each lane can run its own instance against its own
input. CI builds the host closure and the guest image for the PR. The owner
merges. Cadence: weekly PR, and a merge target of at least monthly.

**Hand-pinned sources (Obscura, rusty_v8, zot).** A watcher on upstream
releases, with a monthly review. Obscura's V8 is 15 major versions behind
and runs untrusted JavaScript; it deserves a scheduled bump now, not a
watcher later.

One GitHub constraint matters for all of these: pull requests opened with
the default `GITHUB_TOKEN` do not trigger other workflows, so the CI that
builds host and guest would not run on them. The bump workflows need a
GitHub App token or a fine-grained token with `contents` and
`pull-requests` scope (secret name in the draft: `PATCH_BOT_TOKEN`).

### 5.2 Merge policy: who presses the button

Choir's landing loop treats a push to `main` as a deploy. So the merge
policy is the patch policy.

- **Owner merges by default.** Every lane opens a PR; CI builds the host
  closure and guest image (the deploy-impact classifier will mark
  `flake.lock` as a host change); the owner merges.
- **Auto-merge for a narrow class, later.** A kernel-lane PR that changes
  only the 6.18 patch version, or a Go point release, may auto-merge once CI
  passes and the watcher has tagged it as fixing a KEV entry or a
  reachable `govulncheck` finding. That is a deployment-routing change
  (red), so it is an owner decision with a ratified policy, not something a
  workflow turns on by default. Until then, the watcher's issue pages the
  owner.
- **Never auto-merge** main-nixpkgs bumps, Firecracker minor versions, or
  anything that changes the guest image's userspace. These change too much
  surface for CI to judge alone.

### 5.3 Pinning versus following channels

Choir pins (it has to: the deploy needs a reproducible closure and the
receipt binds to it). The design question is only *what* each pin follows
and *how stale* it may get. The rule: every pin follows a channel or
upstream release stream, an automated PR keeps it within one cadence period
of that stream, and the watcher alarms when a pin falls more than its
lane's cadence behind. A pin that nobody updates is a decision to stop
patching that component, and it should be recorded as one.

---

## 6. Reboot strategy: minimum time to patch for the kernel

The kernel is where minimum time to patch is hardest, because applying the
patch costs an outage of every computer. This section is about making that
outage short, safe and rare, and about moving as much patching as possible
off the host reboot.

### 6.1 Live patching: not realistic for Choir

kpatch and the kernel's livepatch interface exist, and nixpkgs kernels may
build `CONFIG_LIVEPATCH` (**UNVERIFIED** for the locked configuration).
What does not exist is a patch stream: NixOS publishes no livepatch modules,
and building them with `kpatch-build` against a Nix-built kernel is an
unsupported, per-build engineering effort. The bugs that matter most here
(KVM shadow MMU, vhost, netfilter) often change data structures or
locking, which livepatch cannot safely handle, and module parameter changes
like `nested=0` need a module reload or boot regardless. Commercial live
patch services (Canonical Livepatch, KernelCare) target distribution
kernels. Verdict: no. The effort belongs in making reboots cheap.

### 6.2 A planned host reboot that resumes work

Today a host reboot is a crash for every computer: the VMs die with the
host, nobody writes a planned-restart marker, so every computer's open work
is closed as "interrupted by a restart" at its next boot (CLAUDE.md,
"Restarts End Work (Crash) Or Resume It (Planned Update)"). The machinery to
do better exists, because deploy refresh already does it for one computer at
a time. The host reboot procedure uses the same pieces:

1. **Pick the window** (section 6.4) and announce it to computers as a
   host transition record (restart design §3.3: the initiator writes a
   record with a nonce before asking the guest to write its marker).
2. **Drain.** For each running computer, vmctl calls the guest's
   `POST /internal/runtime/planned-restart` with reason `host-reboot` and
   the target kernel. The guest refuses with 409 while busy; vmctl retries
   at turn boundaries for a bounded period (restart design §3.3, admission
   fence). Each computer that accepts is stopped cleanly. Hibernation is not
   an option here: a snapshot taken under the old host kernel is refused
   after the reboot (`snapshotReasonHostKernel`).
3. **Deadline by severity.** At the window's deadline, computers still busy
   are handled by severity: for a routine patch, the reboot is postponed to
   the next window; for a known-exploited host kernel or KVM bug, the reboot
   proceeds and the still-busy computers take a crash boot, visibly. Choosing
   that trade is an owner policy decision. The default this design proposes:
   S0 (section 7) proceeds after one deferred window or 24 hours, whichever
   comes first.
4. **Reboot** (section 6.3, kexec).
5. **Boot.** vmctl's warmness policy brings back always-on computers and the
   platform (paper) computer. Each consumes its marker and resumes. Every
   other computer boots on demand.

**Gap found: the marker expires too early for a host reboot.** A
planned-restart marker is honoured only for 15 minutes
(`plannedRestartMaxAge` in `internal/agentcore/planned_restart.go`). A host
reboot is POST (skipped by kexec), host boot, service start, vmctl reattach,
then a guest boot per computer, serially for the warm ones. For anything
beyond the first few computers, or any boot that hits a slow path after 216
days, 15 minutes is too short, and an expired marker turns a planned restart
back into a crash. The fix belongs to the restart subsystem's host-attested
records (§3.3 already says hibernation markers are bounded by the host
record, not a short TTL): a `host-reboot` transition record that bounds the
marker's validity to the window (for example 2 hours), consumed once. This
is a red change in the restart subsystem, and it must land before the first
drained reboot is attempted, or the procedure should accept that warm
computers past 15 minutes boot as crashes.

**Standing intents need nothing extra.** The paper's source schedules
survive crash boots by design (`recoverSourceSchedulesAtBoot`); a reboot
closes at most the poll in flight. Timing the reboot right after a poll
completes (default interval 15 minutes, `defaultSourcePollInterval`) means
none is in flight.

### 6.3 Kexec, a self-reverting boot and a watchdog

Three mechanisms make the reboot short and safe to attempt:

- **kexec** loads the new kernel from the running one and jumps to it,
  skipping firmware POST, which on server boards often takes minutes
  (Node B's POST time is **UNVERIFIED**; measure it on the first reboot).
  NixOS ships `kexec-tools`, and `systemctl kexec` loads the system
  profile's kernel through a `prepare-kexec` unit (**UNVERIFIED** on Node B's
  GRUB configuration; the manual form is `kexec -l
  /nix/var/nix/profiles/system/kernel --initrd=.../initrd
  --append="$(cat .../kernel-params) init=.../init"` then `systemctl
  kexec`). The deploy already sets that profile (ci.yml, `nix-env -p
  /nix/var/nix/profiles/system --set`). Microcode is unaffected: there is
  no newer microcode to load (section 3.3).
- **A boot loader default that only moves after verification.** Every
  deploy currently makes the newest generation GRUB's default. If the new
  kernel hangs, a hard reset boots the same broken kernel. Instead, keep
  GRUB's default at the last *verified* generation (GRUB's saved default,
  `boot.loader.grub.default = "saved"` plus `grub-set-default` after
  verification; **UNVERIFIED** that NixOS's GRUB module honours this with
  `devices = [ "nodev" ]` and EFI), kexec into the candidate, run the
  verification (section 8), and only then promote it. A failed candidate is
  undone by any cold reset, including one from the provider's API.
- **A hardware watchdog.** systemd's `RuntimeWatchdogSec` with the board's
  watchdog (`iTCO_wdt` on Intel chipsets; presence on Node B
  **UNVERIFIED**) resets a hung kernel, which, with the saved default above,
  falls back to the last verified kernel without a human.

Together: a candidate kernel can be tried with an expected outage of a
couple of minutes, and the worst case is an automatic return to the old
kernel rather than a dead host. The out-of-band console (provider rescue or
IPMI) is the last resort and should be tested once before it is needed
(**UNVERIFIED** that it works for Node B today).

The 216-day boot path is itself a risk: the GRUB default already points at
6.18.37, so an unplanned power loss today would boot an untested kernel.
Regular reboots are what keep the boot path a tested path.

### 6.4 Windows chosen by measured activity

A window is a time with the fewest busy computers and no paper poll in
flight. Choir already knows when computers are busy: the guest answers the
planned-restart request with 409 while it has open runs or a
self-development operation (`internal/vmctl/deploy_refresh.go`), and the
idle sweeper runs the same busy test every interval
(`internal/vmctl/ownership.go`, `StartIdleSweeper`). The window scheduler
needs only a history:

- record, every 5 minutes, the count of busy computers (from the sweeper's
  busy test) into a small host-side series;
- after two weeks, compute the busy count per hour-of-week and pick the
  quietest one-hour slot as the standing weekly window;
- at the window, wait for the platform computer's poll to complete and the
  busy count to reach zero (or the deadline), then drain.

With the owner as the main user today, the quietest slot is the owner's
night. The point of measuring is that this stays true as users arrive in
other timezones. A routine kernel waits for the weekly window; an S0 kernel
uses the next window that comes within its SLO, or an ad hoc one.

### 6.5 Rebootless where possible: the guest path

The guest kernel, guest userspace and Firecracker all patch without a host
reboot, and Choir's existing machinery already does most of it:

- A deploy that changes the guest image refreshes idle computers under a
  planned-restart marker (deploy refresh). They boot the new guest kernel
  immediately.
- Hibernated computers cannot resume across the change: the snapshot is
  bound to the kernel image file's identity (`snapshotBoundPaths`), and to
  the Firecracker version. Their next wake is a cold boot onto the new
  kernel and VMM. Snapshots also expire after 12 hours
  (`DefaultSnapshotMaxAge`), which bounds how long any hibernated computer
  can carry an old guest kernel.
- Busy computers are deferred.

### 6.6 Gap found: deferred computers wait for the next deploy, not the next idle moment

CLAUDE.md says deploys "leave busy ones to their next idle moment". The code
leaves them to the next *deploy*: a deferred computer is reported and
nothing retries it (`deploy_refresh.go`: "Deferred ... to the next deploy";
no update-pending retry was found in `internal/vmctl` or
`internal/vmmanager`). An always-on computer that happens to be busy at each
deploy (the platform computer, polling every 15 minutes, is the likely one)
can run an old guest kernel and an old Firecracker indefinitely. The fix is
small and orange: vmctl keeps an update-pending set from the deploy refresh
plan, and the idle sweeper retries the planned refresh for those computers
when they turn idle, with a bound on attempts. The guest-side reboot-pending
report (section 4.2) is its acceptance signal: the list empties.

### 6.7 A second host: what it buys and what it costs

A second host enables **rolling host reboots** only if computers can move
between hosts. Today they cannot: each computer's `data.img`, credential
disk and snapshots live on Node B's local RAID, and there is no shared
storage or migration path. A second host therefore buys, for patching:

- a **kernel canary**: boot the candidate host kernel on the second host
  first, with test computers, soak for a day, then reboot Node B;
- **platform failover** for the stateless public services, if the routing
  work is done.

It does not shorten the outage of the computers on the host being rebooted.
It costs a second machine's rent (the EPYC 7371 in
[vm-density-limits](vm-density-limits-2026-10-14.md); price **UNVERIFIED**
here), a second patch surface on the same pipeline, and frozen microcode on
that CPU too (section 3.3). The recommendation is to not buy it for patching
alone. When the EPYC arrives for density, the pipeline adds the canary
order (new host first, then the other); real rolling reboots wait for
computer migration, which is its own project.

A cheaper canary is available sooner: a GitHub-hosted Linux runner with
`/dev/kvm` can boot the candidate guest image under Firecracker directly and
check guest health, and can boot the candidate host kernel in a QEMU NixOS
test to prove it starts and loads `kvm_intel` with the declared parameters
(runner KVM availability for this repository is **UNVERIFIED**). It cannot
test Firecracker on the candidate host kernel; that needs real hardware.

---

## 7. Targets: time-to-patch SLOs

### 7.1 Two clocks

Time to patch is measured with two clocks, and both are recorded:

- **Exposure time** runs from public disclosure (advisory publication or the
  KEV `dateAdded`, whichever is earlier) to the moment the fix is *running*
  everywhere it applies. This is the risk the owner carries.
- **Pipeline time** runs from **fix availability** (the fixed release is
  published upstream: kernel.org, Go, Firecracker, the package's maintainer)
  to running. This is what Choir controls and what the SLO binds.

"Running" means booted on Node B for the host kernel and KVM parameters,
active after a unit restart for host services, and booted in every computer
that is not deferred for the guest kernel, guest userspace and Firecracker.
Installed is not running.

### 7.2 Severity classes and targets

- **S0, known-exploited or escape-class, isolation components.** A CISA KEV
  entry, a public proof of concept, or a demonstrated guest-to-host or
  container escape, in the host kernel, KVM, the guest kernel or
  Firecracker. Target: **48 hours** pipeline time to running on the host
  and on every idle computer; busy computers within 72 hours (one deferred
  window, then the owner's crash-boot policy from section 6.2).
- **S1, critical.** Remote unauthenticated code execution in anything on the
  public edge (Caddy, auth, proxy) or in a component that parses untrusted
  input on the host (SearXNG, goproxy), or a local root in the host kernel
  without known exploitation. Target: **7 days**.
- **S2, high and routine.** Everything else with a fix, including reachable
  `govulncheck` findings and guest-side parser bugs. Target: **30 days**,
  meaning the next monthly cycle.
- **Staleness floor, regardless of advisories.** No lane more than one
  cadence period behind its stream: the host and guest kernel at most one
  monthly window behind the 6.18 longterm head; Firecracker on a supported
  minor version; Go on the latest point release of its minor within 30
  days; `reboot_pending` never older than 7 days.

The ladder's existing rule ("kernels within 7 days of any CISA KEV kernel
entry or KVM escape fix, otherwise monthly") becomes S0 at 48 hours. The
owner's "min time to patch is essential" argues for the tighter target, and
the kernel lane plus a planned reboot makes it reachable: nixpkgs stable lag
was about 3 days for 6.18.56, the emergency path removes even that, and the
reboot itself is minutes.

The ladder's open question about the Vercel "KVM 0day" is an S0 event that
has not started its pipeline clock yet: the clock starts when the write-up
and fix land. The watcher should flag the write-up the day it appears.

### 7.3 Recording

Every finding becomes one **patch record** with: the advisory id, component
and lane, severity, `disclosed_at`, `fix_available_at`, `detected_at`,
`merged_at` (commit), `deployed_at` (deploy receipt), `running_at` (the
verification receipt from section 8), the evidence references, and any
deferral with its reason.

- **Now:** one GitHub issue per finding, labelled `patch-sla` and the
  severity, opened by the watcher (`detected_at` is the issue's creation
  time) and closed by a comment that links the verification output. The
  draft workflow opens these issues. Issues are cheap, visible, and need no
  new storage.
- **Next:** the same fields in a host-side JSONL ledger written by the
  verification step, so `running_at` is a machine timestamp and not a human
  comment.
- **Metric:** the pipeline-time distribution per severity, and the count of
  open records past their target. That is a weekly line in the owner's
  report and an alert when anything S0 passes 24 hours.

### 7.4 The SLO as a precommitment

Yes, and it fits the mechanism well. At detection the watcher (later the
research desk) commits a record: "advisory X will be running on Node B by
`fix_available_at` + 48 hours: p = 0.9", with the plan as its assumptions
(lane, window, expected deferrals). The verification receipt resolves it.
Two things follow:

- The pipeline's own promises get a calibration score. A pipeline that says
  0.9 and lands 60% of the time is miscalibrated, and that is a finding
  about the pipeline, not about any one patch.
- The records are honest by construction: the prediction is committed before
  the outcome, so a missed SLO cannot be re-explained after the fact as
  "always expected to slip".

This needs the precommitment record store to accept records from a host-side
or CI writer, which is part of the later agentic slice. The GitHub issue is
the stand-in until then.

---

## 8. Verification after each update

Each lane has a verification step that produces the `running_at` timestamp.
All of them are read-only.

**Host kernel and KVM.**

- `uname -r` equals the declared kernel version, and
  `readlink /run/booted-system/kernel` equals `readlink
  /run/current-system/kernel` (the detector of section 4.2 reports
  `reboot_pending = false`).
- `/sys/module/kvm_intel/parameters/nested` is `N`;
  `/sys/module/kvm/parameters/halt_poll_ns` is `0`; SMT state matches the
  declared policy.
- `scripts/verify-host-hardening.py` passes at the expected rollout stage. It
  already checks unit sandboxing, capability sets, Dolt authentication and
  secret file modes; it should gain the boot-fact checks above so one
  command is the host acceptance.
- The KEV join from the watcher, re-run against the *booted* version, reports
  nothing.

**Guest kernel, guest image and Firecracker.**

- Each refreshed computer's guest health reports the expected build commit
  (the deploy already checks this), and the updater's kernel capability
  probe reports the expected `kernel_release` with a fresh `boot_id`
  (`internal/updater/kernel_capabilities.go`).
- vmctl's per-computer list shows the expected Firecracker version (the
  snapshot metadata already records it as `FirecrackerVersion`) and the
  current guest image manifest.
- The guest-side reboot-pending list (section 4.2) is empty, or contains
  only deferred computers, each with a recorded reason.

**End to end.**

- A tracer smoke on staging: log in as a test user, wake a computer from
  cold, run one desk turn that performs a web read and a model call, and
  check that the paper's next scheduled poll completes. This is the
  existing deployed acceptance path, run after the reboot rather than after
  a deploy.
- For the host reboot specifically: every always-on computer returned with
  its marker consumed (planned) rather than as a crash boot; the count of
  `interrupted_by_restart` closures from the reboot is recorded in the patch
  record. Zero is the target for a routine window.

**Promotion.** Only after all of the above does the boot loader default move
to the new generation (section 6.3).

---

## 9. Rollout plan, in order

Sizes: S is under a day, M is a few days, L is a week or more.

0. **Land `ops/kernel-6.18-security` and reboot (in flight, other agent;
   red).** The kernel pin to at least 6.18.45 (6.18.56 is now available in
   `nixos-26.05`), `nested=0`, and the first planned reboot. The floor is
  6.18.45 for Vghost, and the KEV join (section 2) shows the installed
  6.18.37 is still exposed to CVE-2026-53362, so a reboot into what is
  already installed does not suffice. Before it runs:
   decide whether to accept crash boots for warm computers past the 15-minute
   marker TTL (section 6.2) or wait for step 4. This design does not block
   that branch; it suggests expressing the pin as the `nixpkgs-kernel` input
   so step 3 can automate it.
1. **Merge the report-only watcher (S, yellow).**
   `.github/workflows/patch-watch.yml` on this branch: daily KEV-by-kernel-CNA
   join for the declared host and guest kernels, the 6.18 lag, the
   Firecracker lag and advisories, `govulncheck`, `npm audit`, and one
   issue per finding class. No secrets, no Node B access. It immediately
   reports today's backlog (the Go point release, pdfjs-dist, Firecracker
   1.15.1). Triage pdfjs-dist in the same week: it is a high-severity bug in
   the component that opens untrusted PDFs.
2. **Installed-but-not-booted detector (S, orange).** Add `booted_system`,
   `booted_kernel`, `current_kernel` and `reboot_pending` to the deploy
   receipt's `host_identity`, print a warning on deploys where it is true,
   write `/var/lib/go-choir/patch-state.json` from a host timer, and add the
   boot-fact checks to `scripts/verify-host-hardening.py`. Rollback: revert;
   the facts are read-only.
3. **Split the lanes (M, orange to red).** The `nixpkgs-kernel` input for
   host and guest kernels with a daily `update-flake-lock` PR scoped to that
   input; Firecracker packaged from upstream at 1.17.0 (red: VMM, snapshot
   invalidation, per ladder move 5); the Go toolchain override to the latest
   1.26.x point release with CI's `go-version` moved in the same commit.
   Each is its own commit and its own landing loop. Add `PATCH_BOT_TOKEN`
   so bump PRs trigger CI.
4. **Planned host reboot procedure (M, red).** Extend the reboot script from
   `ops/kernel-6.18-security` with: the host transition record and a marker
   validity bound for `host-reboot` (restart subsystem §3.3), drain through
   the existing planned-restart call, the severity deadline, kexec, GRUB's
   saved default promoted only after verification, and the hardware
   watchdog. Rehearse it once on a quiet window with only test computers
   warm, and measure POST time, kexec time and warm-computer boot time.
5. **Update-pending retry at idle (M, orange).** vmctl keeps the deferred set
   from deploy refresh and the idle sweeper retries it (section 6.6).
6. **Activity-measured windows (M, orange).** The 5-minute busy-count series
   and the weekly window selection (section 6.4), then a scheduled reboot
   job that runs step 4's procedure when `reboot_pending` is true and the
   window opens. Until then, the owner triggers reboots by hand from the
   watcher's issue.
7. **Weekly main-nixpkgs PR and the application dependency lane (M,
   yellow to orange).** `update-flake-lock` for the remaining inputs; a
   Go-modules and npm bump workflow that refreshes `vendorHash` and
   `npmDepsHash`; reconcile the two npm lock files. Pin
   `flakehub-push` to a release tag or SHA in the same pass.
8. **Patch ledger and SLO reporting (M, orange).** Host-side JSONL patch
   records written by the verification step, and the weekly SLO line.
9. **CI canaries (M, yellow).** Guest image boot under Firecracker on a KVM
   runner; host kernel boot in a NixOS VM test. Gates on the kernel and VMM
   lane PRs.
10. **Research desk triage as precommitment records (L, orange).** The
    agentic slice of section 4.3 and the SLO precommitments of section 7.4.
11. **Second host canary order (deferred).** When the EPYC arrives for
    density, not before.

---

## 10. Residuals and unverified claims

- **The other branch's shape.** `ops/kernel-6.18-security` had no commits
  when this was written. Whether its pin is an input, an override or a
  `linuxPackages_6_18` selection is **UNVERIFIED**; step 0 and step 3 adapt
  to whatever lands.
- **Kexec on Node B** (the `prepare-kexec` path with GRUB-on-EFI), **GRUB
  saved default** with NixOS's module, **hardware watchdog** presence, POST
  time and provider console access are all **UNVERIFIED** and are what the
  step 4 rehearsal measures.
- **Kernel compile time** on Node B for the emergency path is
  **UNVERIFIED** (estimated about an hour).
- **vulnix and vulnxscan** on unbuilt derivations in CI: **UNVERIFIED**; the
  draft workflow does not depend on them.
- **GitHub runner KVM** for the CI canaries: **UNVERIFIED** for this
  repository's runners.
- **Security content of the Dolt, Qdrant, Caddy and SearXNG deltas**:
  **UNVERIFIED**; only the version lag is measured.
- **The `govulncheck` baseline** was run locally on macOS with Go 1.26.4,
  matching the production toolchain version, but Linux-only files were not
  analysed; a Linux run in CI is the reference.
- **Livepatch config** in the nixpkgs kernel: **UNVERIFIED**, and moot for
  the verdict.
- **KEV entries older than the kernel CNA (pre-2024 CVEs)** have no vulns
  repository record; the watcher reports them as "no CNA record" for a
  human, and treats a CVE year before 2024 as fixed before 6.18 by
  assumption.
- **The watcher's kernel join was run locally** (the Python embedded in
  `patch-watch.yml`, against the live feeds, for 6.18.16, 6.18.37 and
  6.18.56). The workflow as a whole, including the Nix evaluation of the
  declared versions on a GitHub runner, has not run; it is a draft.
- **Copy Fail's 6.18 fix (6.18.22)** comes from the kernel CNA record, which
  closes the ladder report's UNVERIFIED item; the release date of 6.18.22 is
  not checked here.

---

## Sources

Measured in this session (2026-10-14/15):

- kernel.org releases: https://www.kernel.org/releases.json (6.18.56,
  2026-10-11).
- nixpkgs kernel versions: `pkgs/os-specific/linux/kernel/kernels-org.json`
  at `nixos-unstable` (6.18.55), `nixos-26.05` (6.18.56) and the locked
  revision ec84054698 (6.18.37), via raw.githubusercontent.com.
- nixpkgs package versions at the locked revision and branches:
  `pkgs/by-name/fi/firecracker/package.nix`, `do/dolt`, `qd/qdrant`,
  `ca/caddy`, `se/searxng`, `pkgs/development/compilers/go/1.26.nix`.
- Firecracker releases and advisories: GitHub REST API,
  `repos/firecracker-microvm/firecracker/releases` and
  `/security-advisories` (GHSA-776c-mpj7-jm3r, GHSA-36j2-f825-qvgc).
- Upstream latest releases: dolthub/dolt v2.4.2, qdrant/qdrant v1.19.2,
  caddyserver/caddy v2.11.7, denoland/rusty_v8 v137.3.0 (2025-08-14) and
  v152.2.0 (2026-08-20).
- CISA KEV: https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json
  (catalog 2026.10.14).
- Kernel CNA record: https://git.kernel.org/pub/scm/linux/security/vulns.git/plain/cve/published/2026/CVE-2026-53362.json
- update-flake-lock: https://github.com/DeterminateSystems/update-flake-lock
  (v29, 2026-09-09; `inputs` input in `action.yml`).
- NixOS security tracker: https://tracker.security.nixos.org/ (reachable).
- `govulncheck` (golang.org/x/vuln) over `./...` and `npm audit
  --package-lock-only` in `frontend/`.

Repository (code and documents): `flake.nix`, `flake.lock`,
`.github/workflows/ci.yml`, `nix/node-b.nix`, `nix/hardware.nix`,
`nix/autoputer-vm.nix`, `internal/vmctl/deploy_refresh.go`,
`internal/vmctl/ownership.go`, `internal/agentcore/planned_restart.go`,
`internal/agentcore/hibernate.go`, `internal/agentcore/source_schedule.go`,
`internal/vmmanager/snapshot.go`, `internal/vmmanager/boot_timeline.go`,
`internal/updater/kernel_capabilities.go`,
`scripts/verify-host-hardening.py`,
[host kernel problem](../problems/host-kernel-216-days-unpatched-2026-10-15.md),
[boundary-strength ladder](boundary-strength-ladder-2026-10-15.md),
[restart subsystem design](../restart-subsystem-design-2026-10-13.md),
[S3 snapshot hibernate design](s3-snapshot-hibernate-design-2026-10-14.md),
[VM density limits](vm-density-limits-2026-10-14.md),
[precommitment records](../precommitment-records.md).

Cited through the ladder report (not re-fetched here): Intel servicing for
CPUID 906EA, Firecracker's production host guide on SMT, the Vercel Sandbox
"KVM 0day" coverage, Januscape, Vghost and VMScape write-ups.
