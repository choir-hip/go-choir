# NixOS Agent Platform Audit — Red Hat (bootc/Kagenti) & DeepSeek (DSec) Patterns

**Date:** 2026-10-01
**Mutation class:** green (report only; no runtime change)
**Scope:** `flake.nix`, `nix/*.nix`, `internal/{vmmanager,vmctl,updater,capsule,yaegikernel,platform,platformrelease,computerversion,computerevent,agentcore,actorruntime,autoputer,zot}`, `cmd/capsule-broker`.
**Evidence class:** source reading at `f26de8a`. Nothing here was run against
staging. Items marked **(verify on staging)** are hypotheses from source, not
observed behavior. The descriptions of bootc, Kagenti and DSec come from the
audit prompt and were not checked against the upstream papers.

---

## 0. How the platform is built

The platform is not "a NixOS fleet with per-tenant closures". The real shape is:

- **One host NixOS closure** (`nixosConfigurations.go-choir-b`, `nix/node-b.nix`),
  which includes **one guest image**, `guest-image` (`flake.nix:291-330`). That
  image holds the `vmlinux`, `initrd`, `rootfs.ext4` and **`storedisk.erofs`**
  (the whole guest Nix store closure as one EROFS image).
- **Every user computer boots the same guest image.** No per-tenant Nix module,
  per-tenant closure or per-tenant flake attribute exists anywhere.
- **Per-computer divergence** lives entirely on a per-VM `data.img` mounted at
  `/mnt/persistent` (`nix/autoputer-vm.nix:169-173`,
  `internal/vmmanager/manager.go` `buildFirecrackerConfig`). On top of that
  sits a **non-Nix release mechanism**: a content-addressed file manifest
  (`internal/updater/updater.go` `ReleaseManifest`) with a `current` symlink
  swap, a health check and prior-release restore. It is bound to the computer
  event chain (`AcceptedEventHead`, `CodeRef`, `ArtifactProgramRef`).
- **Isolation inside a VM** comes from guest-local **capsules**
  (`internal/capsule`, `cmd/capsule-broker`): user/mount/net/UTS/IPC/cgroup
  namespaces, an overlay root, a cgroup v2 budget, Landlock and seccomp, and
  signed per-run capabilities. The other path is **Yaegi in-cell workers**
  (`internal/yaegikernel`), which have a package allowlist and host-mediated
  egress.

So the comparison is closer to "DSec with 1 base image + N writable block
volumes" than to "bootc per tenant". Several patterns in the prompt that
assume per-tenant Nix closures (1.4, 2.1 toolkit layer, 3.2 per-VM stdenv)
**do not apply as written**. Below I say where adopting them would help and
where it would be the wrong move.

---

## Three-Planes Assessment

| Plane | Where it lives | Owner / authority |
|---|---|---|
| **Base** | `guest-image` (kernel, initrd, `storedisk.erofs`), bound into the host closure, swapped atomically by `system.activationScripts.go-choir-guest-image` (`nix/node-b.nix:842-956`) | host NixOS deploy (CI → Node B) |
| **Divergence** | `/mnt/persistent` on per-VM `data.img`: embedded Dolt (`/mnt/persistent/state`), Files (`/mnt/persistent/files`, incl. `Source/platform`), updater releases (`/mnt/persistent/choir-updater`), capsule artifacts, signer keys, privacy key. On the host: vmctl ownership registry (`DivergenceStatus`, `PlatformBaseRef`) and corpusd event heads | computer event chain + guest updater + vmctl |
| **Working** | Firecracker process, `go-choir-autoputer.service`, capsules (`/run/choir/capsules`), Yaegi session workers, zot/terminal shells | vmctl lifecycle, capsule `Executor` |

**Conflations found:**

1. **Toolchains sit in the base plane and are on the agent runtime's PATH.**
   `go`, `gcc`, `binutils`, `nodejs`, `libreoffice`, `pandoc`, `documentPython`,
   `perl` and `pkg-config` appear in both `environment.systemPackages` and the
   `go-choir-autoputer` `PATH` (`nix/autoputer-vm.nix:679-706, 779-801`).
   Bumping a toolchain therefore means a new guest image and refreshing every
   VM. That is acceptable at today's scale. The bigger problem is that there
   is no separate toolkit layer and no per-computer toolchain choice.
2. **Working-plane caches live in the divergence plane.** `GOPATH`,
   `GOMODCACHE` and `GOCACHE` are set to `/mnt/persistent/...`
   (`nix/autoputer-vm.nix:737-740`). These are rebuildable caches, but they
   share a volume with durable state (Dolt, signer keys, releases) and have no
   size bound. The ontology's restore set (`docs/computer-ontology.md`
   "Restore-set boundary") names nothing that excludes them, so they will be
   swept into any whole-volume snapshot or restore.
3. **Effective computer identity is split across planes.** The updater's
   per-computer release (`current/`) serves the computer surface, i.e. the
   frontend (`internal/autoputer/computer_surface.go`). But the systemd unit
   always runs the **image-baked** runtime binary,
   `exec ${goChoirPackages.autoputer}/bin/autoputer`
   (`nix/autoputer-vm.nix:138`). The code half of the computer's identity
   comes from the base plane, while the UI half and the manifest identity come
   from the divergence plane. CLAUDE.md already treats event-head +
   CodeRef + content witness + route as a required join, so this is a known
   residual. Name it explicitly here: no single artifact is "the computer's
   code".
4. **Agent-generated state does not leak into base.** The guest store is
   read-only EROFS, there is no `writableStoreOverlay`, and `nix` is not on the
   runtime PATH. The base plane is clean in that direction.

---

## Part 1 — Red Hat Patterns

### 1.1 OS as a versioned image (bootc)

**Status:** Present (host and guest image), with a parallel Partial layer (guest release)

**Evidence**
- Guest OS = one Nix derivation `guest-image` (`flake.nix:322-330`), pinned by
  store path into the host closure (`flake.nix:417-420`, `guestImage = guest-image`).
- Atomic pointer cutover with conflict recovery:
  `system.activationScripts.go-choir-guest-image` (`nix/node-b.nix:842-956`).
  The rollback path is a prior host generation, via `nixos-rebuild --rollback` or
  boot menu. That generation pins the prior guest image.
- Read-only base: EROFS store disk attached `is_read_only: true` (`manager.go`
  `buildFirecrackerConfig`). `/etc` is generated by NixOS at guest boot from the
  closure. The persistent `/mnt/persistent` is the `/var` analog.
- Guest image identity is attested: `guestImageManifest`
  (`nix/autoputer-vm.nix:39-47`) is passed to the updater and runtime as
  `CHOIR_GUEST_IMAGE_MANIFEST`, and the kernel capability probe produces
  `kernel-capabilities.json`.
- `bootc switch` analog: `refreshConfigForCurrentDeploy` (`manager.go:1316`)
  clears the image paths so that a refresh adopts the current deployed image.
  vmctl's `DivergenceStatus` (`tracking|divergent|canary`) and
  `PlatformFollowPolicy` (`auto|proposal|pinned`)
  (`internal/platformrelease/types.go:15-23`) decide which computers follow.

**Gap**
- The **image is global**. Every computer tracks the one image the host
  closure points at. There is no "VM X tracks image ref R", so a canary is
  only "skip refresh", not "boot image R′". Pinning holds an old image only
  while the VM keeps running. Once it reboots it takes the current image.
- **Build tools are in base** (see conflation 1). bootc says exclude them. That
  advice does not fit this product: runtime compilation is a requirement
  (Pattern 3.3). The mismatch is not that the tools exist. It is that they are
  in the *base* rather than in a separately versioned toolkit layer.

**Refactor**
- Make the image reference per computer. Store `guest_image_ref` (the
  store path of a `guest-image` derivation) on `VMOwnership` next to
  `PlatformBaseRef`. Keep the last K images as GC roots under
  `/nix/var/nix/gcroots/go-choir-guest/<digest>` instead of one
  `/var/lib/go-choir/guest` pointer. `bootVM` resolves the image from the
  ownership record, and "switch" becomes a CAS on that field followed by a
  refresh. This is the bootc `switch` semantics.

**Priority:** **High.** Without it, `pinned`/`canary` cannot survive a reboot
and a bad image cannot be rolled back for a subset of computers. Size:
medium. **Red class**: touches vmctl/VM lifecycle.

### 1.2 Three-way merge for /etc

**Status:** Partial. A different but stronger mechanism exists for app
releases. Nothing exists for OS config, and nothing is needed there.

**Evidence**
- Guest `/etc` is fully declarative and regenerated on every boot. Tenants
  cannot diverge it in a durable way, so a three-way merge has nothing to
  merge.
- Tenant configuration divergence lives in Dolt or app state and in updater
  releases. The merge policy is per component: `ComponentLineage.DivergenceTrack`
  (`platform|user`) plus `DivergedComponents`, `Follow*` policy
  (`internal/platformrelease/types.go:31, 58-90`) and platform-update offers
  minted by corpusd (`internal/platform/platform_update.go`). A `tracking` +
  `auto` computer fast-forwards. A diverged component gets `proposal` (no
  overwrite). `pinned` ignores updates.
- Runtime parameters come from the kernel cmdline at boot
  (`go-choir-extract-cmdline`, `nix/autoputer-vm.nix:259-371`). This is
  imperative and per-boot, not persisted.

**Gap**
- Merge granularity is the **component**, not the file. A user-diverged
  component never takes upstream changes, even non-conflicting ones; it only
  gets a "proposal". bootc's rule (an unchanged local file takes the update)
  would correspond to a file-level three-way merge inside a component, using
  the `ReleaseManifest.Files` digests.
- The compat shims in `go-choir-extract-cmdline` (deriving `MAILD_URL` and
  `WIRE_PUBLISH_URL` from `vmctl_url` with `sed`, lines 350-365) are
  imperative config migration kept in base. They are a small form of drift.

**Refactor**
- Inside `platform_update.go`, compute a three-way diff on
  `ReleaseManifest.Files` (base = `PlatformBaseRef` manifest, ours = `current`,
  theirs = offer). Auto-apply files where ours equals base, and keep ours
  where ours equals theirs. Route only true conflicts to `proposal`. This uses
  digests the system already has and needs no new storage.
- Replace the cmdline-derivation shims with one versioned bootstrap envelope
  (the credential disk already exists). Fold that into item 6 of the roadmap.

**Priority:** **Medium.** It reduces how often divergent computers become
stranded on old platform code. Size: medium.

### 1.3 Kagenti per-tool-call isolation

**Status:** Partial. Per-capsule kernel isolation is strong. Per-tool-call
isolation is absent. The agent runtime itself runs as unconfined root.

**Evidence: what is good**
- Capsules (`internal/capsule/executor.go` `Spawn`, ~`:415`):
  `CLONE_NEWUSER` plus unshared `NS|NET|UTS|IPC|CGROUP`, an overlay root
  (`CHOIR_CAPSULE_LOWER_ROOT=/`), a cgroup v2 budget and freeze/thaw/kill.
  The broker applies **Landlock** (`capsule.NewBrokerLandlock(mergedDir, ...)`,
  `cmd/capsule-broker/main.go:196`) and **seccomp** (`:204`). A capsule has
  **no network**: an empty netns. Every RPC is checked against an
  ed25519-signed, role-scoped, revocable `Capability`
  (`internal/capsule/capsule.go` `Exec`/`GoEval`, `roles.go`). The broker
  binary digest is pinned and rechecked before each spawn.
- The kernel capability probe (`internal/updater/kernel_probe_linux.go`) fails
  closed if namespaces, Landlock or seccomp are missing, and gates both
  `go-choir-updater` and `go-choir-autoputer` (`requiredBy`).
- Yaegi cells (`internal/yaegikernel`): `BannedPackages` blocks `os`,
  `os/exec`, `net`, `syscall`, `unsafe` and `reflect` (`allowlist.go:30-41`),
  and network verbs are host-mediated (`Egress` → `researchDeps.HostEgress`,
  `internal/agentcore/tools_desk.go:143-156`).
- Signer and updater units use strong systemd sandboxing:
  `ProtectSystem=strict`, `CapabilityBoundingSet=[]`, `PrivatePIDs`,
  `RestrictAddressFamilies=AF_UNIX`, `InaccessiblePaths`
  (`nix/autoputer-vm.nix:507-652`).

**Evidence: gaps**
- **Isolation is per capsule (session), not per call.** One broker with one
  Landlock ruleset (all of `mergedDir`) serves every `exec`, `read_file` and
  `write_file` in the capsule (`cmd/capsule-broker/main.go:394-690`). A
  `handleExecDirect` child inherits the broker's rules. No narrower,
  per-call write scope is applied.
- **Yaegi workers rely on language-level isolation only.** Desk session
  workers get `Setpgid` + `Pdeathsig` (`desk_worker_spawn_linux.go:27-32`),
  a sanitized env and a cwd. They get no Landlock, seccomp or netns. Escape
  safety depends entirely on the Yaegi symbol allowlist. Yaegi is an
  interpreter that is not designed as a security boundary.
- **The agent runtime is unconfined root.** `go-choir-autoputer.service` has
  no `User=`, no `ProtectSystem`, no `CapabilityBoundingSet`, no
  `NoNewPrivileges` and no `RestrictAddressFamilies`
  (`nix/autoputer-vm.nix:751-764`). Its `ReadWritePaths` and
  `InaccessiblePaths` only have effect next to `ProtectSystem` or a
  mount-namespaced unit. They give the signer directories partial protection
  but do not narrow anything else.
- **Shell paths without a capsule.** `internal/zot/session.go:97` runs
  `/bin/sh -lc <command>` with the full `os.Environ()` of the root runtime.
  `internal/autoputer/terminal.go:258` gives a root shell, and its zot path
  exports `RUNTIME_GATEWAY_TOKEN` as `OPENAI_API_KEY` (`:243-246`). Whether
  zot commands are model-originated in production needs confirmation, but if
  they are, they run with ambient root and the gateway credential.
- **VM-level egress is open (verify on staging).** `setupHostNetworking`
  (`internal/vmmanager/manager.go:2696-2771`) adds `FORWARD -i tap -j ACCEPT`
  and `-o tap -j ACCEPT`, and MASQUERADEs guest traffic out of every
  interface. The comment there reads "in case the guest needs internet access
  for any reason". The NixOS firewall (`nix/node-b.nix:217`) filters INPUT
  only, and `networking.firewall.filterForward` is not set. In practice:
  (a) any process in any guest has unrestricted internet egress, and
  (b) **tap→tap forwarding between tenants appears permitted**. Guest
  `10.200.X.2` could reach guest `10.200.Y.2:8085`, whose firewall opens 8085
  (`nix/autoputer-vm.nix:768`), and the guest API trusts the
  proxy-set `X-Authenticated-User` header (`internal/proxy/computer_lifecycle.go:203`).
  If that holds on staging, it is a **cross-tenant authority problem**, not
  just a missing hardening step. Per the Problem-Documentation-First rule, it
  should get its own problem record before any fix.

**Refactor (in order)**
1. **Egress default-deny at the tap** (red, small). Per tap: `FORWARD -i tapN
   -o tap+ -j DROP`, then allow only `tapReachableHostServicePorts()`, then a
   policy choice for internet egress. The recommendation is to deny it
   entirely and route research egress through the gateway or `HostEgress`,
   which already exists. Alternatively, allow it only to an egress proxy with
   a per-computer allowlist. Set `networking.firewall.filterForward = true`
   in `node-b.nix`.
2. **Confine the runtime unit** (red, medium). Run `go-choir-autoputer` as a
   dedicated user. Grant only the capabilities that capsule spawn really
   needs (it uses `CLONE_NEWUSER`, so possibly none, or `CAP_SYS_ADMIN` for
   the cgroup delegate). Add `ProtectSystem=strict`, `NoNewPrivileges`
   (check compatibility with capsule setuid/fcaps first) and `Delegate=yes`
   for the capsule cgroup subtree. Strip `RUNTIME_GATEWAY_TOKEN` from child
   environments by default.
3. **Per-call Landlock in the broker** (orange, medium). In
   `handleExecDirect`, fork a helper that applies a *second* Landlock ruleset
   before `execve`. Landlock rulesets stack and can only narrow. The ruleset
   is derived from the call's capability: write scope =
   `/workspace/<run_id>/<call_id>` or a declared path set, read = the merged
   root. Cost is one extra fork per call, in the same range as Kagenti's
   ~7 ms.
4. **Kernel floor under Yaegi workers** (orange, small). Apply the same
   broker Landlock + seccomp + empty netns to `SpawnDeskSessionWorker`, so
   an interpreter escape lands in a sandbox. All egress is already
   host-mediated, so an empty netns costs nothing.
5. **Route zot/terminal agent commands through a capsule** or delete the
   non-capsule path (Convergence-Before-Patching: prefer the existing
   substrate).

**Priority:** **Critical** for 1 (if confirmed) and 2. High for 3 and 4.

### 1.4 Chunked images and layer granularity

**Status:** Absent. The prompt's framing (per-tenant closures) also does not
apply.

**Evidence:** `storedisk.erofs` is one monolithic EROFS image of the whole
guest closure (`flake.nix:316, 323`), built with `-zlz4`
(`nix/autoputer-vm.nix:187`). Any change produces a fresh multi-GB image:
an autoputer Go change, a frontend change, or a LibreOffice bump. Because
the image is built and consumed on Node B, nothing is re-downloaded, but the
whole image is **rebuilt** (mkfs.erofs over the full closure) on every
deploy. The comment at `:180-186` shows this cost is already felt.

**Gap:** There is no separation between slow-moving layers (kernel, systemd,
LibreOffice, Python, toolchains) and fast-moving ones (`autoputer`,
`capsuleBroker`, `updater`, skills, frontend).

**Refactor:** Split into **two EROFS disks**: `base.erofs` (the closure of
everything except the `goChoirPackages.*` and frontend paths) and
`app.erofs` (the Choir packages). Mount both as Firecracker drives and
compose them into `/nix/store` with overlayfs (two lowers) in the initrd.
microvm.nix does not do this out of the box: the build needs a custom
`pkgs.runCommand` that computes `closureInfo` for both sets and takes the
set difference. Expected effect: most deploys rebuild only the small app
layer. A cheaper first step is to measure. Log mkfs.erofs time and image
size per deploy, and check whether this is a real bottleneck before building
it.

**Priority:** **Low to Medium.** This is a deploy-latency optimization at
single-host scale. It becomes High only if a second image class or a remote
fleet appears. Size: medium.

---

## Part 2 — DeepSeek DSec Patterns

### 2.1 Three-layer environment composition

**Status:** Partial. Base and workspace exist. The toolkit layer is absent.

**Evidence**
- **Base:** the shared read-only `storedisk.erofs`, one image for all VMs
  (the DSec analog is its 2 shared base images).
- **Workspace / writable upper:** a per-VM `data.img` (ext4 block device,
  not an overlay upper). Inside the VM, a capsule *is* the DSec composition:
  lower = guest `/`, upper and work dirs per capsule (`Capsule.UpperDir`,
  `WorkDir`, `MergedDir`), plus a content-addressed source snapshot
  (`PreflightSourceSnapshot`, `capsule-subject:sha256:` refs).
- **Toolkit:** none. Harness components (skills under
  `share/go-choir/skills`, `obscura`, `zot`, toolchains) are baked into base.

**Gap:** Adding a package to one computer is impossible without a new global
image. Because the store is read-only and `nix` is absent, there is no
per-computer package path at all. Any package a tenant "adds" goes through
language package managers into `/mnt/persistent` (`GOPATH`, npm), with no
Nix provenance.

**Refactor:** Add a **third drive, `toolkit.erofs`, selected per computer**
(a store-path ref on `VMOwnership`) and overlaid into `/nix/store` like the
app layer in 1.4. The platform publishes a few named toolkits, for example
`toolkit-docs` (LibreOffice, pandoc, Python docs) and `toolkit-build` (go,
gcc, node). This moves the heavy toolchains out of base and lets computers
opt in. Pair this with 3.3 for user-authored packages.

**Priority:** **Medium.** It is the enabling step for 3.2, 3.3 and the
toolchain conflation. Size: medium to large.

### 2.2 On-demand loading from distributed storage

**Status:** Absent. Not needed at current scale.

**Evidence:** Single host. The EROFS image is a local file shared by all
Firecracker processes, so it already behaves as "one shared base,
host-page-cache backed, loaded on demand". Host `nix.settings`
(`nix/node-b.nix:1026-1031`) configure no `substituters` beyond defaults, no
cache server (no harmonia, nix-serve or attic) and no `nix.buildMachines`.
Guests have no Nix daemon. Node A (`nix/node-a.nix`) is a mirror that builds
its own closure.

The comments at `nix/autoputer-vm.nix:177, 194` and `flake.nix:315, 394`
claim **KSM deduplication**, but nothing enables it: no `hardware.ksm.enable`.
Firecracker also does not `madvise(MADV_MERGEABLE)` guest memory, so KSM
would not apply even if enabled. What actually dedups is the **host page
cache** for the shared EROFS file. Guest page caches are still per-VM
copies, so N guests means N copies of hot store pages in guest RAM.

**Refactor:** (a) Fix the comments, or enable KSM with an explicit
Firecracker-compatible mechanism. (b) Before considering a second host, add
`services.harmonia` (or attic) on Node B, signed with a host key, and add it
as a substituter and trusted key on Node A, so the mirror stops rebuilding.
(c) Consider virtio-pmem / DAX for the EROFS store so guests map host page
cache directly with no per-guest copy. This is DSec's "only the writable
delta is local" in memory terms. It needs Firecracker pmem support; check
before relying on it.

**Priority:** **Low.** (a) is a trivial doc fix. (b) becomes Medium when
Node A becomes a real failover.

### 2.3 Trajectory log for preemption-safe resumption

**Status:** Partial, and explicitly flagged in-code as incomplete.

**Evidence**
- Durable, ordered semantic events: one guest `ComputerEventAppender`
  (`internal/computerevent/appender.go`) with corpusd head CAS. The ontology
  states reconstruction "never reruns a model, tool, or network observation"
  (`docs/computer-ontology.md`).
- Actor durability: an SQLite WAL actor recovery log
  (`internal/actorruntime/adapter.go:122-126`), used for replay or recovery
  only.
- Replay tooling: `ReplayCompletenessReport` (`agentcore/replay_completeness.go`)
  builds replay state in a disposable Dolt workspace and diffs it against
  live state. `replayAirworthinessEntries`
  (`agentcore/replay_eligibility.go:44-60`) classifies tables:
  **`runs`, `events`, `trajectories`, `run_continuations`, `channel_messages`
  and `work_items` are `ReplayEmptyUntilSupported`**. Only the event index,
  effective state and `run_memory_entries` are real event projections.
- Recovery mode: `choir.runtime_recovery_replay_only` cmdline flag
  (`nix/autoputer-vm.nix:341`) and `VM_REPLAY_STALL_TIMEOUT=1500s`
  (`node-b.nix`).
- Hot/warm/cold tiering exists for **VMs** (`warmness_policy.go`,
  `pressure_reclaim.go`) but not for **session state**.

**Gap:** Tool-call results are not yet a cached, replayable tape. After a
crash mid-run, the system recovers actors and semantic events, but in-flight
tool calls either re-execute or are abandoned. DSec's fast-forward by
cached result is the missing piece. It matters most for non-idempotent
calls: gateway LLM spend, email or wire publish, capsule exec with effects.

**Refactor:** Promote `trajectories` and `runs` from
`ReplayEmptyUntilSupported` to `ReplayEventProjection` by appending a
`tool_call.requested` / `tool_call.resolved` pair through the existing
appender. Put the result body in the content store by digest, which matches
the privacy/payload-resolver pattern already in `computerevent`. On
resume, the runtime matches `(trajectory_id, step, request_digest)` and
returns the cached result rather than re-executing. No new log location is
needed. `/var/lib/agent-trajectory` would create a **second state
authority** that conflicts with the "single state authority" standing
question. Use the event tape.

**Priority:** **High.** This is the substrate under preemption, hibernate
(2.3 × 4.2) and restore. Size: large. **Red class**: the evidence/trace
surface.

### 2.4 pack_diff: incremental snapshot of runtime changes

**Status:** Present in design and source. Effect proof pending.

**Evidence:** `CapsuleEffectBundle`
(`internal/capsule/transaction/builder.go:25-40`) is a content-addressed
pack_diff and is richer than DSec's: `BaseEventHead`,
`SourceTreeRef`, `OrderedFileEffects` (classified upper-dir diff),
`GeneratedArtifactRefs`, **`BuildRecipeRef`**, `RuntimeArtifactRef`,
`DependencyToolchainRefs`, `TestReceipts`, `VerifierReceipts` and
`CapabilityPolicyDigest`. Accepted bundles become desired code via an event.
The updater materializes them as a `ReleaseManifest` with a health check and
prior restore (`updater.go` `Apply`, `restorePrior`). The ontology table
marks capsules "Implemented source candidate; effect proof pending".

**Gap:** Artifacts are **file-manifest blobs, not Nix store paths**. A
runtime-compiled binary in a bundle carries no closure: its dynamic
libraries and interpreter path point into the *current* base image's
`/nix/store`. If the base image is GC'd or changes, the artifact can break
silently. `DependencyToolchainRefs` records this but nothing enforces it at
materialization.

**Refactor:** At `updater.Apply`, verify that every ELF `PT_INTERP` and
`DT_NEEDED` path in the manifest resolves inside the currently booted
image's closure (or the bundle) and refuse otherwise. Record the
`guest_image_ref` from 1.1 in the `ReleaseManifest` so a base change
triggers a rebuild-from-recipe rather than a broken exec.

**Priority:** **Medium.** It becomes High as soon as effects turn on. Size:
small to medium.

### 2.5 Unified backend abstraction

**Status:** Partial. There are three isolation tiers but no unified API.

**Evidence:** The tiers are (1) Yaegi in-process or worker cell
(`yaegikernel`, ≈ FnCall), (2) capsule (≈ container, `capsule.Executor`
with `Exec`/`GoEval`/`ReadFile`/`WriteFile`/`ListDir`/`Stat`/`Mkdir`/`Remove`),
and (3) the Firecracker VM (`vmmanager`). The capsule broker RPC surface is
the closest thing to a unified SDK. Desk tools already pick
`desk_go_eval` or `capsule_go_eval` by role
(`internal/agentcore/tools_desk.go:23-28`). The legacy
`VMCTL_ALLOW_HOST_PROCESS=false` toggle hints at a retired host-process
backend. There is no TTY verb on the capsule surface; the terminal goes
around it (`autoputer/terminal.go`).

**Refactor:** Define one Go interface, roughly `Sandbox{Exec, GoEval,
ReadFile, WriteFile, ListDir, Stat, PTY}`, implemented by
`yaegikernel` (no PTY), `capsule` and a future `vmSandbox` that proxies
over the vmctl autoputer socket. Selection is by role and policy, not by
tool name. Do **not** add systemd-nspawn as a fourth backend. Capsules
already are the guest-local container tier, and nspawn would duplicate them
(Convergence-Before-Patching).

**Priority:** **Medium.** It is what makes 1.3 item 5 (moving the terminal
into a capsule) clean. Size: medium.

---

## Part 3 — Differential Update Patterns

### 3.1 Selective propagation (VM 1 → VM 2, not VM 3)

**Status:** Partial. A registry and push channel exist, but only for the
platform → computer direction.

**Evidence:** The vmctl `OwnershipRegistry` (`internal/vmctl/ownership.go:138-146`)
maps computer → `DivergenceStatus`, `PlatformBaseRef` and hold status. Corpusd
mints signed **platform-update offers** per computer
(`HandlePlatformUpdateOfferMint`, `internal/platform/platform_update.go`).
vmctl transports them over the autoputer proxy and the guest verifies and
applies them. That is push-based, per-computer, signed and policy-gated.
The route ledger (`internal/routeledger`, `vmctl/route_authority.go`)
projects an accepted checkpoint into serving with generation CAS.

**Gap:** No **computer → computer** channel exists. An artifact accepted on
computer A cannot be offered to computer B. Offers are minted only from
platform code commits (`CodeCommit`). Shared world-wire Dolt is explicitly
outside the restore set, so it cannot serve as the channel either.

**Refactor:** Generalize the offer mint so the payload source is either
`code_commit` or `accepted_bundle_ref`, which is a `CapsuleEffectBundle`
digest already pinned in the content store. Target selection is a list of
`computer_id`s chosen by the owner or a management desk. B receives an
offer and runs it through its own acceptance (its own event chain and
verifiers). C is simply never targeted. Delivery stays push-based through
the existing vmctl → guest transport. This fits the "promote typed
artifacts, not opaque machine accidents" rule.

**Priority:** **Medium.** It is the World-Wire-adjacent capability, but it
is gated on effects (2.4) being live. Size: medium.

### 3.2 Per-VM custom builds (source-level propagation)

**Status:** Partial in design (`BuildRecipeRef`). Absent at the Nix level.
The premise is mostly false today.

**Evidence:** All VMs share one base image, so ABI divergence between VMs
currently comes **only** from what tenants compile into `/mnt/persistent`
against the same base closure. Binaries are portable across today's
computers as long as they are on the same image generation. They break
across image generations (see 2.4). Each VM has no flake.lock: the single
repo `flake.lock` pins everything. No remote builders are configured.
Capsules build with a local `go` and `GOTOOLCHAIN=local`, against
`GOMODCACHE` on the data volume (offline-ish).

**Refactor:** Treat `BuildRecipeRef` as the portable unit, as the prompt
suggests. Make the recipe a Nix expression (a `default.nix` or flake
fragment in the bundle) so the receiving computer gets reproducible
rebuilds and `DependencyToolchainRefs` become store paths. Builds run
**host-side** in a sandboxed Nix builder service. The guest asks vmctl for a
build and gets back a store path, delivered as a toolkit-style EROFS
fragment or via the updater. This avoids a writable Nix store in every guest
and gives a single remote builder for free. Do not put `nix-daemon` in
guests: that would reopen the base-plane leak that the read-only store
currently prevents.

**Priority:** **Low** now. It becomes Medium after 1.1, 2.1 and 3.1 land.
Size: large.

### 3.3 Exposing code for runtime compilation

**Status:** Present (non-Nix). Capture is Partial.

**Evidence:** go, gcc, node and binutils are in the base closure and on the
runtime PATH. `CGO_*` flags point at ICU. Caches live on the persistent
volume (`nix/autoputer-vm.nix:719-740`). Capsules compile inside the overlay
upper. Artifacts are captured by `CapsuleEffectBundle` → `ReleaseManifest`
(2.4). Uncaptured compilation (zot, terminal, any direct process in
`/mnt/persistent`) survives VM recycle because it is on `data.img`, but it is
**unversioned and unattributed**. That is "opaque machine accidents" in
ontology terms.

**Gap / refactor:** The build-time (declarative) and runtime-artifact
(imperative) planes are already separated by mechanism: the Nix image versus
the updater release. They become entangled through the **non-capsule shell
paths**, which write into the same volume without capture. Closing 1.3 item 5
(route all agent execution through capsules) is the refactor that separates
them. After that, every runtime artifact either arrives as a bundle or is
scratch in a capsule upper that is discarded.

**Priority:** **High.** It falls out of 1.3. No extra size beyond that work.

---

## Part 4 — Platform Engineering & Operations

### 4.1 Fleet-wide drift prevention

**Status:** Present for base, Partial for divergence.

**Evidence:** Every VM boots the same declarative image, and a refresh
adopts the current image. The guest image manifest, kernel capability
receipt, `build.json` and `/run/go-choir-guest-deploy-receipt.json` give
attested identity. `updater.VerifyCurrentRelease` re-hashes the release
tree. `DivergenceStatus` names whether a computer is meant to drift. Host
drift is bounded by Nix plus the CLAUDE.md "no Node B edits" rule.

**Gap:** (a) No periodic drift **detector** compares a running VM's booted
image digest with what its ownership record says it should run. Refresh is
best-effort per deploy. (b) Divergence-plane drift outside the updater
(Go caches, zot-written files, anything under `/mnt/persistent/files`) is
not detected. That is by design for user files, but not for executables.
(c) `vmctl-priority.env` is a mutable host file (`node-b.nix` vmctlExec
comment), which is a small amount of acknowledged host drift.

**Refactor:** Add a vmctl sweep that reads each guest's
`/health` → `guest_image_manifest` digest and release digest and compares
them with `VMOwnership.{guest_image_ref, PlatformBaseRef}`. Report the
result in the warmness health summary. Make it read-only first.

**Priority:** **Medium.** Size: small.

### 4.2 Scheduler co-design (DSec Watcher)

**Status:** Partial.

**Evidence:** vmctl has an idle sweep (`VMCTL_IDLE_TIMEOUT=30m`), warmness
classes (`public_platform|primary|premium_always_on|critical_protected`),
**PSI-driven pressure reclaim** (memory, CPU and IO avg10 thresholds, up to
5 candidates; `vmctl/pressure_reclaim.go`) and retention pruning for
ephemeral accounts. A maintenance hold blocks automatic lifecycle. All of
this is configured in `nix/node-b.nix:640-718`.

**Gap:** **Hibernate is stop** (`manager.go:845-866`: "Full memory
snapshot/restore is a future Firecracker feature"). Resume is a cold boot
plus recovery replay of up to 30 minutes, with a 1500 s stall gate for
large Dolt workspaces. So reclaiming an idle computer costs a full replay,
which pushes policy toward keeping VMs resident. That is exactly the
pinned-resident-state problem DSec's Watcher addresses. Reclaim is not
coordinated with run state: it does not check for in-flight trajectories.

**Refactor:** (1) Use Firecracker's existing snapshot API
(`PUT /snapshot/create`, `/snapshot/load`; diff snapshots supported) for
real hibernate. Write memory and vmstate under the VM's state dir, and fall
back to cold boot on load failure. (2) Expose a guest
`/internal/quiescence` signal (no open run, no capsule active, actor
mailboxes drained) and have pressure reclaim prefer quiescent computers.
(3) Use `balloon` to return guest page cache under pressure before
hibernating. Caveat: memory snapshots capture in-memory secrets and the
ed25519 capsule key (process-local by design). Snapshot files need the same
protection as `data.img`, and the "authority is lost on restart" assumption
in `capsule/executor.go:35-37` must be re-examined. This is **red** for that
reason.

**Priority:** **High.** It is the main lever on host density at 32 GiB.
Size: medium to large.

### 4.3 Cross-plane state ownership

| State | Plane | Owner today | Recovery today | Gap |
|---|---|---|---|---|
| Kernel, initrd, systemd, core services | Base | `guest-image` in host closure | rebuild from flake; prior host generation | per-computer image ref (1.1) |
| Choir runtime binary | Base (!) | image-baked `goChoirPackages.autoputer` | as above | identity split from updater release (conflation 3) |
| Frontend / computer surface | Divergence | updater `current/` | `restorePrior`; baseline import from `/nix/store` | — |
| Tenant packages / toolchains | Base (no divergence path) | — | — | toolkit layer (2.1) |
| Dolt app state, textures, run memory | Divergence | embedded Dolt + event chain | replay from event tape (partial; see 2.3 table classes) | `runs`/`trajectories` not replayable |
| Semantic events | Host + guest | `ComputerEventAppender` + corpusd CAS | immutable, pinned content | — |
| Actor mailbox state | Working→Divergence | SQLite actor log | actor replay | — |
| Tool-call results | Working | none durable | **re-execute or lost** | 2.3 |
| Capsule scratch | Working | capsule upper on `/run/choir/capsules` | discarded (correct) | — |
| Capsule effect bundles / subjects | Divergence | `/mnt/persistent/capsule-artifacts` | content-addressed; `ErrSubjectArtifactUnavailable` if lost | lives only on one VM's `data.img` → not in shared content store until pinned |
| Go/npm caches | Working (stored as Divergence) | — | rebuildable | move to separate disposable volume (conflation 2) |
| Uncaptured shell-built binaries | **unowned** | — | survive on `data.img`, unversioned | 1.3/3.3 |
| Signer keys, privacy key | Divergence (secret) | signer units | on `data.img` only | if `data.img` is lost, guest-core signing identity is lost; confirm escrow (`internal/keyescrow`) covers it |
| Host VM registry, route slots | Host | vmctl, routeledger | host disk | — |
| Historical logs, archived workspaces | Cold | retention prune deletes; no archive tier | — | no cold tier; ephemeral accounts are simply deleted |

**If a VM's `data.img` is lost:** the event chain and pinned content blobs
survive host-side (corpusd), so the computer can be reconstructed by replay
up to the `ReplayEmptyUntilSupported` boundary. Runs, trajectories,
continuations, messages and work items would not be recovered, and neither
would unpinned capsule artifacts or the Files tree (unless Choir Base or
filecas covers it; confirm). Signer keys and the privacy key are recoverable
only if escrowed.

---

## Prioritized Refactor Roadmap

| # | Change | Patterns | Class | Size | Why this order |
|---|---|---|---|---|---|
| 1 | **Document, then close, VM egress and tap→tap forwarding.** Problem record first (CLAUDE.md rule), then a per-tap `DROP` to other taps, an allowlist of host service ports, internet egress denied or sent through a proxy, and `firewall.filterForward = true`. | 1.3 | red | **small** | Possible cross-tenant reach to guest `:8085` with a trusted identity header. Cheapest high-severity item. Verify on staging first. |
| 2 | **Confine `go-choir-autoputer.service`**: non-root user, `ProtectSystem=strict`, minimal capabilities, delegated cgroup for capsules, scrub `RUNTIME_GATEWAY_TOKEN` from child env. | 1.3 | red | medium | Today the in-VM blast radius of any non-capsule execution is root plus the gateway token. |
| 3 | **Route zot/terminal/shell agent execution through capsules** (or delete the non-capsule path); add a `PTY` verb to the broker. | 1.3, 3.3, 2.5 | orange | medium | Removes the unowned "uncaptured binaries" state class and the entanglement between the runtime and declarative planes. |
| 4 | **Tool-call request/result events on the tape**; promote `runs`/`trajectories` to `ReplayEventProjection`; resume = cached-result fast-forward. | 2.3, 4.3 | red | large | Prerequisite for safe hibernate, preemption and restore. Prevents duplicate non-idempotent effects. |
| 5 | **Real hibernate via Firecracker snapshots + quiescence-aware reclaim.** | 4.2 | red | medium–large | Main density lever. Depends on #4 so that snapshot failure degrades to replay safely. |
| 6 | **Per-computer `guest_image_ref` on `VMOwnership` + GC-rooted image retention** (bootc `switch`). | 1.1, 3.1, 4.1 | red | medium | Makes `pinned`/`canary` survive reboot and allows partial rollback. Enables #8–#9. |
| 7 | **Kernel floor under Yaegi workers + per-call Landlock in the broker.** | 1.3 | orange | medium | Defense in depth where the language-level allowlist is the only barrier today. |
| 8 | **Drift sweep**: compare booted image and release digests with the ownership record. Read-only. | 4.1 | green→orange | small | Cheap observability once #6 exists. |
| 9 | **Split base / app / toolkit EROFS layers**; move toolchains to opt-in toolkits. | 1.4, 2.1 | orange | large | Faster deploys and per-computer package sets. Measure mkfs cost first. |
| 10 | **Computer→computer offers** (`accepted_bundle_ref` source) + ELF closure check at `updater.Apply`; Nix-expression `BuildRecipeRef` built host-side. | 3.1, 3.2, 2.4 | red | large | The selective-propagation endgame. Gated on effects going live. |

Small cleanups that can go in any time (green): correct the unbacked KSM
comments (`flake.nix:315, 394`; `nix/autoputer-vm.nix:177, 194`); move
`GOCACHE`/`GOMODCACHE` to a separate disposable volume or `/var/cache`
tmpfs, and add them to the restore-set exclusions in
`docs/computer-ontology.md`.

## What not to adopt

- **bootc's "no build tools in production"**: this product requires runtime
  compilation. Put the tools in an opt-in layer (#9), and do not remove
  them.
- **Writable Nix store / nix-daemon in each guest**: this breaks the
  read-only base plane, which is currently the strongest invariant. Do Nix
  builds host-side (#10).
- **A separate `/var/lib/agent-trajectory` SQLite**: it would be a second
  state authority. Use the existing event tape (#4).
- **systemd-nspawn as another backend**: capsules already fill this tier.
- **3FS-style distributed on-demand loading**: there is no fleet to justify
  it. Revisit with a second production host.
