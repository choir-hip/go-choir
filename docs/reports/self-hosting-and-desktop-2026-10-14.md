# Self-Hosting Choir, and the Desktop App as a Server-less Deployment

Date: 2026-10-14. Research and design survey, read at `main` f54f05c42.
Mutation class of this note: **green** (docs only). Most of the work it
proposes is orange or red (§6). Nothing here was run against Node B or
staging. One public, read-only probe was made:
`GET https://choir.news/api/platform/publications/front` returned 200.

Owner, in session, 2026-10-14 (about 15:35 UTC): "Choir is open source and
you'll be able to self-host it, and we want to encourage that [...] We
definitely need to improve self-hosting options. We will make it work on
any kind of infra. Note that there's one other element I've been
deemphasizing: the Wails desktop app. We built it, then haven't touched it.
But it will become an essential component, and a basis for a sans-server
deployment, which runs VM(s) locally, perhaps consuming an Autopaper API
and LLM API(s) — but even those could be self-hosted, theoretically all on
the same machine as the Wails desktop app."

Related: [pricing-and-economics-notes-2026-10-14.md](pricing-and-economics-notes-2026-10-14.md)
argues that self-hosters are worth more than managed-hosting customers
because they join the network: federation, signed records, and
publication into and adoption from the Autopaper object graph. It leaves
the licence question open. The ontology already names the target:
a **Private Choir Cloud** is "a customer-controlled deployment with its own
NixOS host or host cluster, platform computer(s), many user computers"
([computer-ontology.md](../computer-ontology.md), "Cloud Boundary").

## Summary

1. **Self-hosting works today only in a narrow way.** You need an x86_64
   Intel bare-metal Linux box running NixOS, and you have to fork and
   hand-edit `nix/node-b.nix`. That file is one machine's configuration:
   disk UUIDs, root SSH keys, the `choir.news` domain, the owner's UUID, a
   remote builder at a fixed IP, and `kvm-intel`. It has no options. No
   self-hosting document exists.
2. **The documented local dev stack does not start without Firecracker.**
   vmctl exits with "Firecracker is mandatory" (`cmd/vmctl/main.go:155`),
   and `start-services.sh` then stops while waiting for it. The README's
   "Try it" section therefore does not work on macOS.
3. **There is a VM seam, with one implementation.** `vmctl.VMManager`
   (`internal/vmctl/ownership.go:249-290`) has 11 methods plus 7 optional
   capabilities found by type assertion. Only Firecracker implements it.
   Tap networking, source-IP caller identity, `/proc` scanning, the
   Firecracker snapshot API and an x86-only guest image all leak past it.
4. **btrfs is not a real dependency.** Only snapshot-budget accounting uses
   a btrfs ioctl, and it falls back to `st_blocks`. The real host tools
   are `mkfs.ext4 -d` and `resize2fs`.
5. **The Wails desktop app** (`cmd/desktop`) is a macOS thin client to
   `choir.news`. It has a sound native passkey bridge
   (ASWebAuthenticationSession plus a Go cookie jar). It builds today on
   darwin/arm64, but a fresh checkout needs the frontend built first. The
   last feature commit was 2026-07-10. Its "local mode" predates the
   persistent-computer cutover and, from reading the code, cannot work.
   Its entitlements file carries an invalid key, left there by a global
   rename.
6. **The server-less path has two shapes:**
   - **Linux appliance:** one Linux VM on the desktop runs the unchanged
     host stack, with Firecracker nested inside it. Needs an M3 or later
     Mac on macOS 15, or WSL2.
   - **Native backend:** host services run natively and the VM backend
     drives Virtualization.framework (or Hyper-V) directly.

   Both need an **aarch64 guest image** for Apple Silicon. The flake builds
   x86_64-linux only.
7. **Federation is a key and witness problem more than a hosting problem.**
   Each host has one platform key that witnesses every event head. A
   self-hosted computer's records would be witnessed by its own operator.
   For them to count across the network, the network needs a registry of
   platform keys and an external witness, such as publishing commitment
   hashes to the network.
8. **The repo has no LICENSE file.** GitHub reports `licenseInfo: null` on
   the public repo, so today the code is "all rights reserved" whatever
   the README implies. The licence decision should land before outside
   contributions do.
9. **Order of work:** a self-host guide and config extraction first; then
   making the VM backend selectable, plus an aarch64 guest; then the macOS
   desktop backend; then federation; then Windows and Kubernetes.
10. **A security gap found while reading code (documented separately).**
    The tap-to-tap deny rule covers only `10.200.0.0/16`, but the guest
    subnet pool spans `10.200`-`10.215`. See
    [docs/problems/tap-deny-rule-covers-one-of-sixteen-guest-subnets-2026-10-14.md](../problems/tap-deny-rule-covers-one-of-sixteen-guest-subnets-2026-10-14.md).

---

## 1. Self-host readiness today

### 1.1 What a stranger needs

From reading `flake.nix`, `nix/*.nix` and `.github/workflows/ci.yml`:

- **Hardware:** x86_64 Linux bare metal with hardware virtualization, i.e.
  `/dev/kvm`. `nix/hardware.nix:19` loads `kvm-intel`, so an AMD host must
  change it to `kvm-amd`. Node B today is a Xeon E-2236 (6 cores, 12
  threads) ([vm-density-limits-2026-10-14.md](vm-density-limits-2026-10-14.md) §4).
  The `hardware.nix` comment saying "E-2136 / 32GB" is stale.
- **Memory:** guests are 4 CPU / 8 GiB interactive and 2 CPU / 4 GiB
  default (`nix/node-b.nix` vmctl env). A 4 GiB guest thrashed on a
  compacted owner store. vmctl reserves `MemoryLow=14G` for guests. A
  realistic floor for one owner and the shared paper computer is about
  32 GB of RAM.
- **OS:** NixOS. There is no non-Nix install path: no systemd units,
  packages or containers outside the NixOS module.
- **Network:** a domain with DNS pointing at the host. Caddy gets TLS
  automatically through ACME. Ports 22, 80 and 443 must be open.
- **Accounts and keys:** at least one LLM provider (§1.4) and a Resend
  account for mail. Search is optional, because SearXNG runs on the host.
- **Build:** about 100 GB free for the Nix store
  (`nix.settings.min-free=80G`). The guest image builds with microvm.nix.
  Builds are x86_64-linux only (`flake.nix:29-30`).

### 1.2 Host services

All services bind to loopback on fixed ports. Guests reach a subset of
them through per-tap DNAT to the host's tap IP
(`internal/vmmanager/manager.go:3416-3428`; reachable ports 8082, 8083,
8084, 8086, 8087, 8088). State lives under `/var/lib/go-choir`.

| Service | Port | Purpose | Needs | Secrets / config | Minimal install? |
| --- | --- | --- | --- | --- | --- |
| Caddy (`services.caddy`) | 80/443 | TLS edge; routes `/auth/*`, `/api/*`, `/health*`; blocks `/internal/*` and `/provider/*` | Domain, ACME | `virtualHosts."choir.news"` hard-coded (`node-b.nix:228`) | Yes |
| `go-choir-auth` | 8081 | Passkey (WebAuthn) login, JWT, SQLite sessions | — | `AUTH_RP_ID` and `AUTH_RP_ORIGINS` = choir.news (`node-b.nix:385-386`); ed25519 signing key generated on first start (`:375`) | Yes |
| `go-choir-proxy` | 8082 | Auth-gated API and routing to computers through vmctl; serves the platform shell from `/var/www/go-choir/frontend-current` | Auth public key file, vmctl, corpusd, maild URLs (validated non-empty, `internal/proxy/config.go:139-163`) | `deploy.env` (optional) | Yes |
| `go-choir-vmctl` | 8083 | Persistent computer lifecycle, route CAS, hibernate/snapshot, pressure reclaim, retention | `/dev/kvm`, Firecracker, `CAP_NET_ADMIN`, `ip`/`iptables`, `ip_forward`, `mkfs.ext4`, `resize2fs`, platform Dolt (route DSN), guest image in `/var/lib/go-choir/guest` | `computer-tiers.json` embedded at build time (`cmd/vmctl/main.go:580-586`); an unused `VMCTL_PROMOTION_AUTHORITY_PUBLIC_KEY` (`node-b.nix:766`; nothing reads it) | Yes |
| `go-choir-gateway` | 8084 | Provider-neutral LLM and search gateway; issues guest tokens | Outbound HTTPS | `gateway-provider.env` (§1.4); identity store JSON | Yes, with ≥1 provider |
| `go-choir-corpusd` | 8086 | Platform service: event-head CAS, receipts (platform key), artifacts, the Autopaper object graph (Store B), publications | Platform Dolt; corpus Dolt optional through `corpus-dsn.env` | `CORPUSD_AUTOPAPER_OWNERS` = one owner UUID (`node-b.nix:594`); escrow operators (`secrets/corpusd-escrow-operators.env`, optional) | Yes |
| `go-choir-maild` | 8087 | Email in and out (Resend), drafts | Resend | `maild.env`: `RESEND_API_KEY`, `RESEND_WEBHOOK_SECRET` (`nix/deploy-mail-creds.sh`); `MAILD_PRIMARY_DOMAIN`. Sender and links hard-coded to choir.news (§1.3) | Optional in principle; the proxy requires its URL |
| `go-choir-goproxy` | 8088 | Go module proxy for engineering capsules | Outbound HTTPS | none (DynamicUser) | Needed for self-development |
| `go-choir-platform-dolt` | 13306 | `dolt sql-server`, Store A (platform) | Dolt | none | Yes |
| `go-choir-corpus-dolt` | 13307 | `dolt sql-server`, Store B (corpus) | Dolt | none | Optional (corpusd falls back to Store A) |
| `go-choir-checkpointd` (timer, 1 min) | — | Verified projection checkpoint maintenance | Platform Dolt, artifacts | none | Yes for restore |
| `go-choir-front-importance` (timer, 10 min) | — | Autopaper front ranking (Jev) | Gateway provider env (OpenRouter) | OpenRouter key | No |
| `services.qdrant` | 6333 | Vector search | — | none | No (guests cannot reach it, per the comment at `node-b.nix:274`) |
| `services.searx` (SearXNG) | 8888 | Free meta-search, first in the provider list | Python service | `SEARXNG_SECRET` generated on first deploy | No, but it is the free search path |
| `go-choir-disk-gc`, `-nix-gc`, `-platform-dolt-history-audit` | — | Housekeeping timers | — | — | Recommended |
| `choir-builder` | — | Layered app-layer release builder; a service pointer with no unit (`flake.nix:322-326`) | — | — | Self-development only |

Inside the guest image (`nix/autoputer-vm.nix`): the computer runtime
(`autoputer`), `choir-updater` (root, reached on `/run/choir/updater.sock`),
`capsule-broker`, and `obscura` (a Rust package fetched from
`yusefmosiah/obscura`, `flake.nix:139-146`). Per VAL-VM-011, the guest
carries no provider credentials and no signing keys.

### 1.3 Hard-coded assumptions

| Assumption | Where | Effect on a self-hoster | Fix shape |
| --- | --- | --- | --- |
| The NixOS host is one specific machine | `flake.nix:457-472` (`nixosConfigurations.go-choir-b`), `nix/disks.nix` (UUIDs, btrfs subvolumes), `nix/hardware.nix` (`kvm-intel`, md RAID1) | Must fork and edit | A `nixosModules.choir` with options; the host config imports it |
| Domain `choir.news` | Caddy `node-b.nix:228,267`; auth RP `:385-386`; `MAILD_PRIMARY_DOMAIN` `:479`; Dolt commit email `:39,50` | Passkeys fail on another origin; mail is misaddressed | One `domain` option |
| Mail sender and links | `internal/maild/drafts.go:267` (`https://choir.news/?app=email...`), `drafts.go:271` and `notifications.go:64` (`updates@choir.news`) | Ignores `MAILD_PRIMARY_DOMAIN`; Resend rejects an unverified sender domain | Build from config (orange) |
| Clients default to choir.news | `cmd/desktop/main.go:346`, `cmd/choir/main.go:38` | Overridable (`CHOIR_BACKEND`, `CHOIR_HOST`) | Fine; document it |
| Owner and paper-computer identities | `CORPUSD_AUTOPAPER_OWNERS` (`node-b.nix:594`); `internal/computertiers/computer-tiers.json` (one paid account, the owner; one platform computer, the paper) | The front page filters to Choir's owner; tiers need a rebuild | Tiers are deliberately *not* host config (owner ruling 2026-10-13, `node-b.nix` vmctl comment). For self-hosters, a flake input or override is a decision for the owner |
| Root SSH keys | `node-b.nix:207-209` (`wiz@choiros-ovh`, `github-actions-deploy`) | A stranger's host would trust Choir's keys | Must be an option, with no default |
| Remote builder node-a | `node-b.nix:1296-1320` (51.81.93.94, pinned host key, `/root/.ssh/nix-builder-node-a`) | Falls back to local builds if unreachable, but pins a foreign host key | Optional option |
| Fixed public IP in CI | `.github/workflows/ci.yml:1644,1660` (`--resolve choir.news:443:147.135.70.196`) | Only Choir's CI | CI stays Choir's own; document a self-host deploy instead |
| Deploy over root SSH to `/opt/go-choir` | `ci.yml:810-818, 1074-1167`; `scripts/node-b-sync-service-pointers`; `nix/deploy-provider-creds.sh` (default host `node-b`) | The self-host update path is undocumented | A `choir-host` update command or `nixos-rebuild --flake` guidance |
| Linux paths as Go defaults | `/var/lib/go-choir` in `internal/platform/config.go:12,15`, `internal/vmmanager/manager.go:325`, `internal/vmctl/{pulse,retention_prune,pressure_reclaim,cold_recover}.go`, `internal/gateway/jev.go:226`, `cmd/goproxy/main.go:24` and others | Fine on Linux; wrong on macOS and Windows | One `CHOIR_STATE_DIR` root |
| One host, fixed loopback ports | 8081-8088, 13306/13307, 6333, 8888, 11434 | No multi-host | Acceptable for single-host and desktop; Kubernetes needs a multi-host design (§4) |
| Guest network space | `10.200-10.215.x.0/30` (`manager.go:370-412`) | Can collide with a LAN or VPN on desktops and clouds | Configurable base |
| Repo URL `choir-hip/go-choir`; Go module path `github.com/yusefmosiah/go-choir` | `flake.nix:37`, `nix/node-b.nix:16`, `nix/autoputer-vm.nix:24`, `go.mod:1` | Cosmetic, but a fork's guest records the upstream as its source | An option |
| Search and model defaults | `start-services.sh:45` assumes ChatGPT/Codex auth | — | Document it |

### 1.4 Secrets inventory

- **LLM** (gateway; `internal/provider/provider.go`). All are optional
  individually. With none, the gateway starts, logs "no real provider
  configured", and inference fails (`cmd/gateway/main.go:71`).
  - `OPENROUTER_API_KEY`
  - `AWS_BEARER_TOKEN_BEDROCK` + `AWS_REGION`
  - `ZAI_API_KEY` (+`ZAI_BASE_URL`)
  - `FIREWORKS_API_KEY` (+`FIREWORKS_BASE_URL`)
  - `OPENCODE_API_KEY`/`OPENCODE_GO_API_KEY` (+base URLs)
  - `CHATGPT_AUTH_PATH`, a copied `~/.codex/auth.json`, read from
    `~/.codex` by default
- **Search:** `TAVILY_`, `BRAVE_`, `EXA_`, `SERPER_`, `PARALLEL_`,
  `SERPAPI_` and `KEENABLE_API_KEY`, plus `SEARXNG_URL`. All optional.
- **Mail:** `RESEND_API_KEY` and `RESEND_WEBHOOK_SECRET`.
- **Generated on the host:**
  - auth ed25519 key
  - SearXNG secret
  - the corpusd platform signing key, which signs every receipt (see
    [precommitment-signing-and-computer-keys-2026-10-11.md](../precommitment-signing-and-computer-keys-2026-10-11.md))
  - the receipt signer's key (`receiptsigner.LoadOrCreateSigningKey`)
- **Operator credentials:** `CHOIR_KEY_ESCROW_OPERATORS`. If absent, the
  unwrap gate stays closed with a 503.
- **Delivery:** `nix/deploy-provider-creds.sh` and `deploy-mail-creds.sh`
  write env files over SSH. This works for a remote server, not for a
  desktop.

### 1.5 Documentation gaps

- No self-hosting document. "Private Choir Cloud" exists only as a concept
  (`docs/current-architecture.md:199`, `docs/computer-ontology.md`).
- README "Try it" (`README.md:113-118`) runs `start-services.sh`, which
  needs Firecracker and therefore Linux with KVM. The README doesn't say
  so.
- `docs/runbooks/` holds three Node B operator runbooks. None covers
  install, first boot, backup and restore of `/var/lib/go-choir`, or
  upgrading.
- No hardware sizing guide. The density report covers Choir's hosts only.
- No account of what the platform key means for a self-hoster: the host
  operator is the witness.
- Stale comments: the tap IP in a gateway comment says `172.X.0.1`, but the
  real range is `10.2xx.x.1` (`node-b.nix:1006`); host-process mode comments
  in `internal/vmctl/ownership.go:507,3405` and `cmd/vmctl/main.go:129`;
  the `hardware.nix` CPU.

**Verdict.** A determined Linux and Nix operator with an Intel bare-metal
box could run Choir in about a day by forking `node-b.nix`. Anyone else
cannot. The blocking work is packaging and configuration. The code itself
is mostly portable on Linux.

---

## 2. The Wails desktop app

### 2.1 What exists

`cmd/desktop/` is its own Go module (`go.mod`: `wails/v3
v3.0.0-alpha2.104`, `replace github.com/yusefmosiah/go-choir => ../..`).
It is about 880 lines of Go without tests. A Nix dev shell is available
(`nix develop .#desktop`, `flake.nix:387-401`).

| File | What it does |
| --- | --- |
| `main.go` (506 lines) | Wails app; embeds `frontend/dist` (`//go:embed all:frontend/dist`, `:24`). **Cloud mode** (default) serves the embedded SPA and reverse-proxies `/auth/*` and `/api/*` to `CHOIR_BACKEND` (default `https://choir.news`). **Local mode** (`CHOIR_MODE=local`) launches `gateway`, `autoputer` and `proxy` from `bin/` as child processes with state in `~/.choir`, and serves the SPA on `localhost:3000` |
| `desktop_auth.go` (246 lines) | Native session broker: one-time code redeem through `/auth/desktop/redeem`, a process-local cookie jar, `Set-Cookie` stripped before the renderer sees it. Tokens never reach JavaScript |
| `webauthn_session_darwin.go` (123 lines) | `ASWebAuthenticationSession`, because WKWebView has no platform authenticator; `choir://auth-complete` callback |
| `webauthn_session_other.go` | Stub: no auth bridge outside macOS |
| `Taskfile.yml` | `build:frontend` (npm), `copy:frontend`, `build:services` (auth, gateway, autoputer, proxy), `dev`, `cloud`, `build`, `package`, `sign`, `icon` |
| `build/darwin/` | `Info.plist`, entitlements, icons |

The server side of the bridge is in `internal/auth/handlers.go` (desktop
start and redeem).

### 2.2 State and history

`git log -- cmd/desktop` shows 19 commits:

- **2026-06-22:** created ("Phase 1 — Wails v3 desktop shell"), with local
  service orchestration and the passkey bridge.
- **2026-06-28:** Base sync (M5) and a macOS File Provider extension (M6)
  added.
- **2026-07-10:** both removed as superseded (02ff06a64, 7b59d33e8). The
  auth handoff was made credential-free (738341bef). These are the last
  feature commits.
- **Since then, mechanical changes only:**
  - 74c4c7833 (2026-08-11): global "sandbox → autoputer" rename
  - 49b002f79 (2026-08-24): Go toolchain bump
  - 2fbecebc5 (2026-09-25): unit-test purge

**Build check (today, darwin/arm64, go1.26.4):**
- A fresh checkout fails with `pattern all:frontend/dist: no matching
  files found`. The dist directory is gitignored, and the `.gitkeep` the
  `.gitignore` expects is not tracked.
- With a placeholder `frontend/dist/index.html`, it builds a 15.7 MB
  binary. The only output is linker warnings (objects built for macOS 14,
  linked for 11).
- The Taskfile path (`task build`) would build the real frontend first.
- I did not launch the app.

**Defects found by reading:**
- **The entitlements key is invalid.** `com.apple.security.app-autoputer`
  (`build/darwin/entitlements.plist:5`) came from the 08-11 global rename
  and was meant to be `com.apple.security.app-sandbox`. Also, a VM-running
  build will need `com.apple.security.virtualization`.
- **The desktop serves its own frozen UI.** The embedded SPA answers every
  path except `/auth/*` and `/api/*` (`assetHandler`, `main.go:451-475`).
  So the desktop shows the UI compiled into the app, not the
  computer-surface frontend that the server selects per computer after
  vmctl resolve (`node-b.nix` Caddy comment; doctrine C15/I25 in
  [computer-ontology.md](../computer-ontology.md): computer-surface
  frontend is inside the restore set). After a restore or self-development
  change to a computer's UI, the desktop would diverge from the browser.
  This is a hypothesis from code; no trace confirms it.
- **Local mode cannot work as written (code reading, not run):**
  - It starts no auth service. The proxy is fatal without an auth public
    key (`cmd/proxy/main.go:20-23`).
  - It starts no corpusd, maild, vmctl or Dolt. The proxy requires
    `PROXY_CORPUSD_URL` and `PROXY_MAILD_URL`; defaults exist but nothing
    listens.
  - It runs the computer runtime as a host process. That mode is now
    forbidden in deployment; vmctl is fail-closed on Firecracker.
  - It predates the persistent-computer cutover (2026-08-11).
- The Taskfile uses `npm`; the repo also carries a `pnpm-lock.yaml`.
- Wails v3 is still an alpha line.

### 2.3 What it does today

It is a thin native client to `choir.news`. Its useful assets are a native
window and the passkey bridge: Touch ID through Safari, with no token
exposed to JavaScript. Nothing runs locally in the supported (cloud) mode.

### 2.4 Gap to a server-less deployment

1. **A local VM backend on macOS** (§3.2). Firecracker does not run on
   macOS.
2. **An aarch64-linux guest image.**
   - The guest and host packages are x86_64-only (`flake.nix:29-30, 444`).
   - Other x86 assumptions: an x86 `rusty_v8` archive (`flake.nix:135`),
     `i8042.*` boot args (`manager.go:1640-1643`), `vmlinux` ELF kernel
     boot.
   - Building aarch64-linux needs a Linux builder: the nix-darwin
     `linux-builder`, or a remote builder.
3. **A local supervisor for the host services that still matter** (§3.4),
   replacing the stale local mode.
4. **Single-owner auth.** Device possession is the owner credential, and
   WebAuthn on `localhost` works without a public domain. Local mode
   already assumes "device access is ownership" (`main.go:229-231`).
5. **UI from the computer:** load the computer-surface frontend from the
   local proxy, not the embedded build. This also fixes the cloud-mode
   divergence in §2.2.
6. **Settings UI for provider keys, stored in Keychain.** `go-keyring`
   is already an indirect dependency.
7. **Lifecycle:** app quit or laptop sleep must map onto planned
   hibernate, not crash (§3.9).
8. **Distribution:** Developer ID signing, notarization and an updater.
   The `sign` task is ad hoc only.
9. **Windows and Linux auth bridges.** Only the darwin implementation
   exists.

---

## 3. Server-less architecture options

### 3.1 Target shape

```text
Choir.app (Wails, Go)
  ├─ window: computer-surface UI served by the local proxy
  ├─ supervisor: host services (sidecar processes or one multi-call binary)
  │    auth(single-owner) · proxy · vmctl · gateway · corpusd + Dolt
  │    · checkpointd · goproxy · [maild] · [qdrant] · [search]
  ├─ VM backend: Firecracker (Linux) | Virtualization.framework (macOS)
  │              | WSL2/Hyper-V (Windows)
  ├─ storage: ~/Library/Application Support/Choir (macOS) — state dir root
  └─ outbound: LLM APIs (BYO keys) or local LLM; search APIs;
               Autopaper public API; network federation endpoints
```

### 3.2 VM backend per OS

The facts on other vendors' products come from public sources; links are
at the end. Items marked *(check)* were not confirmed against a primary
source.

| OS | Option | Notes for Choir |
| --- | --- | --- |
| Linux | **Firecracker + KVM** (as today) | Works now. Desktop Linux needs `/dev/kvm` access (`kvm` group), `CAP_NET_ADMIN` for taps, iptables. Cloud Hypervisor is an alternative KVM VMM if Firecracker's device model limits bite |
| macOS | **Virtualization.framework (VZ) directly**, via Go bindings (e.g. `Code-Hex/vz`) | High-level Linux guests with virtio net, block, vsock, balloon and virtiofs. Save and restore of machine state exists in recent macOS versions *(check the minimum version)*. Apple Silicon requires **arm64 guests**. Networking is NAT through the framework, not host taps and iptables. Needs the `com.apple.security.virtualization` entitlement |
| macOS | **vfkit** (CLI over VZ; used by Podman machine and CRC) | Fastest to prototype: a subprocess per guest, much like Firecracker today. Adds a binary to ship |
| macOS | **libkrun / krunkit** (Hypervisor.framework microVMs; used by Podman's krun provider) | Closest in spirit to Firecracker (a library microVM). krunkit exposes nested virtualization on M3+ |
| macOS | **Lima** (VZ or QEMU), **Tart** | Lima is mature for "a Linux VM on a Mac" and suits the appliance shape. Tart is aimed at macOS and Linux CI images. Both are heavier than needed for per-computer guests |
| macOS | **Apple Containerization / `container`** (2025, Swift; one lightweight VM per Linux container) | The architectural analog of Firecracker on macOS. Swift-only; requires a recent macOS *(check)* |
| macOS | **Nested: a Linux VM running the unchanged stack, Firecracker inside it** | Requires nested virtualization, which Hypervisor.framework supports on **M3 or later with macOS 15+** (community and libkrun reports; Apple's `VZGenericPlatformConfiguration.isNestedVirtualizationEnabled`). Excludes M1 and M2 Macs |
| Windows | **WSL2 with nested KVM** | Run the Linux stack, with Firecracker, unchanged inside WSL2. Windows 11 exposes `/dev/kvm` to WSL2 when nested virtualization is enabled *(check per build)* |
| Windows | **Hyper-V / HCS** (`hcsshim`), or WHP | The native route; a large new backend. Hyper-V requires Pro or Enterprise editions; WHP is available on Home |

**Two strategies.**

**A. Linux appliance.**
- **Shape:** one Linux VM per desktop runs the NixOS host closure. It is
  the Node B module minus Caddy and ACME, with the same vmctl and
  Firecracker nested inside. The Wails app starts that VM and talks to the
  proxy inside it.
- **Code divergence:** almost none. Restart semantics, tap isolation,
  snapshots and the guest image stay identical to the server, which keeps
  "staging is the acceptance environment" meaningful.
- **Costs:**
  - Nested-virtualization overhead.
  - M3 or later with macOS 15 only; WSL2 only on Windows.
  - A large appliance image to ship and update.
  - Two layers of memory budgeting on an already memory-tight guest
    shape.

**B. Native backend.**
- **Shape:** host services run as native macOS (or Windows) processes.
  vmctl gains a second `VMManager` implementation backed by VZ.
- **Benefits:** works on every Apple Silicon Mac, and gives better
  battery life and memory.
- **Costs:** a real port. Section 3.3 lists what leaks out of the current
  seam.

These are not exclusive. A is the shortest way to prove a server-less
computer on hardware that supports it. B is the product path for the
installed base of Macs. Both need the aarch64 guest on Apple Silicon:
under A the appliance is arm64, so its nested guests must be arm64 too.

### 3.3 The VM backend abstraction today

**The interface:** `vmctl.VMManager` (`internal/vmctl/ownership.go:249-290`)
has these methods:
- `ReserveBootEpoch`, `BootVM(VMManagerConfig)`, `StopVM`, `HibernateVM`
- `ResumeVM`, `ReattachVM`, `RecoverVM`, `RefreshVM`
- `DestroyVMState`, `GetVM`, `CheckHealth`

**Optional capabilities**, found by type assertion:
- `snapshotResumer` (`:297`) and `snapshotDiscarder` (`:305`)
- `vmGatewayTokenReader` (`:358`)
- boot and hibernate timeline readers (`handlers.go:962,969`)
- `vmProcessController` (`LiveFirecrackerVMs`, `ReapUnmanagedVM`,
  `SetVMOOMScoreAdj`; `host_capacity.go:39`)
- `coldRecoveryStorage` (`cold_recover.go:132`)
- `guestBootRefusal` (`recovery_admission.go:1006`)

**Implementations:** one, `vmmanager.Manager`, wrapped by
`vmManagerAdapter` (`cmd/vmctl/main.go:224+`), plus test mocks.

**Selection:** none. `vmmanager.IsFirecrackerAvailable()`
(`internal/vmmanager/config.go:159`) either succeeds or vmctl exits
(`cmd/vmctl/main.go:134-155`).

**What leaks past the seam and must move behind it before a second backend:**

1. **Guest networking.**
   - Taps, `/30` addressing, iptables DNAT, MASQUERADE, anti-spoof and
     tap-to-tap deny (`manager.go:3089-3428`).
   - Kernel command line URLs pointing at the host peer IP
     (`manager.go:1646-1653`).
   - VZ NAT and WSL networking have no per-guest host-visible tap.
2. **Caller identity by source IP.** vmctl, maild and the gateway identify
   the calling guest by its tap address:
   - `internal/vmctl/handlers.go:1189-1212`
   - `internal/maild/api.go:123-136`
   - `internal/gateway/handlers.go:702-731`

   This is sound only because of the anti-spoof rules. A backend-neutral
   design uses a per-guest credential. A gateway token per guest already
   exists (`vmGatewayTokenReader`); alternatively, use a vsock channel per
   guest. This change touches auth: **red**.
3. **Process control.** PID files, `/proc` cmdline scanning, OOM score
   (`manager.go:2159-2250`). These map to "handle owned by the backend" on
   VZ.
4. **Snapshot and hibernate.** These use the Firecracker API socket
   (`internal/vmmanager/fcapi.go`, `snapshot.go:290-305`). They map to VZ
   save and restore where available, and the existing kill-hibernate
   fallback already covers a backend without snapshots.
5. **Memory return.** Firecracker's free page reporting balloon
   (`balloon.go`) maps to VZ's memory balloon device, whose semantics
   differ *(check)*.
6. **Disk images.** `mkfs.ext4 -d` and `resize2fs` on the host
   (`manager.go:2648,2755,2781`) need e2fsprogs on macOS (Nix provides it),
   or images created inside the guest.
7. **Boot artifacts.** `VMManagerConfig` carries kernel, initrd, rootfs and
   store-disk paths, which suits VZ's Linux boot loader. The kernel must be
   an arm64 `Image`, and console arguments are per architecture.
8. **The startup gate** should become "a real VM backend is mandatory".
   This keeps the fail-closed refusal of host-process mode and lets
   configuration choose the backend: `VM_BACKEND=firecracker|vz|...`.

btrfs is not on this list. The only btrfs-specific code is
`measureFileDiskBytes` (`snapshot_disk_linux.go:13-50`, with a non-Linux
stub), used for snapshot budget accounting. On APFS, `clonefile(2)` could
make guest disk copies cheap, but nothing needs it.

### 3.4 Host services: in-process or sidecars

**Which services a desktop needs:**
- **Required:**
  - proxy
  - vmctl
  - gateway
  - corpusd with platform Dolt. It holds the event-head CAS and route CAS,
    and the platform key that witnesses the computer's event chain.
  - checkpointd (restore)
- **Self-development:** goproxy.
- **Optional:** maild (a desktop rarely has inbound mail), Qdrant,
  SearXNG (a Python service and awkward to bundle; use a search API or
  connect to a self-hosted instance), front-importance (not needed).

**Single-owner auth** would replace WebAuthn ceremonies with the native
session. The auth service still mints the JWTs the proxy verifies.

**Sidecars** (separate processes supervised by the app, as Node B runs
them under systemd):
- Keeps crash isolation and the restart-loop protections (systemd
  `startLimitBurst` maps to the supervisor's own limits).
- Keeps one code path with the server.
- Answers the "fate-sharing" standing question
  ([standing-questions.md](../standing-questions.md)).
- Shipping cost: one multi-call binary (`choir-host <service>`) avoids
  carrying 8 binaries in the bundle.

**In-process** (goroutines inside the Wails binary):
- Fewer moving parts.
- A gateway or corpusd panic kills the window and every service together,
  and the restart rules would then treat every app crash as a computer
  crash.

**Dolt:** corpusd and vmctl speak MySQL to `dolt sql-server`, which runs
on macOS. The embedded driver (`dolthub/driver/v2`, already used inside
guests) is an option for a single-process host, but it changes the
concurrency model of the platform store. Treat it as a later step.

### 3.5 Storage

- **State:**
  - `/var/lib/go-choir` becomes an app-support directory, so all Go
    defaults must derive from one root (§1.3).
  - Per computer: `data.img` (ext4) and a credential disk, plus the
    read-only erofs store disk shared by all guests.
- **Size:**
  - Guest workspaces reach 5-8 GiB on the owner store. Snapshots are as
    large as guest memory.
  - The Node B budgets (`VM_SNAPSHOT_BUDGET_GIB=48`, 16 GiB free-space
    floor) are server values. A desktop needs budgets scaled to free disk.
- **Backup:** the platform Dolt store, artifacts and per-computer disks
  are the whole state. A desktop backup story (Time Machine exclusion of
  live images, a consistent snapshot) is design work nobody has done.

### 3.6 Model and search providers

- **Bring-your-own keys** already works server-side through the gateway
  env file. A desktop needs a settings UI writing to Keychain, and the
  gateway reading from the app's secret store instead of an env file.
- **ChatGPT/Codex sign-in:** the provider reads `~/.codex/auth.json` by
  default (`provider.go:847`). That suits a desktop where the user has
  Codex installed, subject to the platform's terms
  ([pricing notes](pricing-and-economics-notes-2026-10-14.md), Risks).
- **Local LLMs:**
  - There is **no generic OpenAI-compatible "local" provider.**
  - `FIREWORKS_BASE_URL`, `ZAI_BASE_URL` and `OPENCODE_*_BASE_URL` point
    OpenAI-style clients at another endpoint. Aiming one at llama.cpp's
    server or Ollama's OpenAI-compatible endpoint is a workaround, not a
    supported path.
  - Ollama is already wired for embeddings
    (`internal/qdrant/ollama_embedder.go`, default `localhost:11434`) and
    for small-model PII redaction (`internal/pii/slm_redactor.go`). The
    gateway health probe knows Ollama (`internal/gateway/config.go:77`).
  - Model choice per desk is policy (`RUNTIME_MODEL_POLICY_PATH`).
  - *Hypothesis:* desks are tuned on frontier models and write Go cells.
    [cross-model-cell-errors-2026-10-13.md](cross-model-cell-errors-2026-10-13.md)
    shows cell error rates vary by model, so a small local model is likely
    to fail desk work long before it fails chat. Local models fit
    embeddings, redaction and summarization first.
- **Search:** remote search APIs, or SearXNG self-hosted beside the app.

### 3.7 Autopaper: public API or a local corpus

- **Reading works today without an account.**
  `GET https://choir.news/api/platform/publications/front` returned 200
  with stories (probe today). The proxy routes `publications/resolve`,
  `/export` and `/front` (`internal/proxy/handlers.go:832-847`).
- **Adoption across hosts does not exist.** Publication and adoption run
  inside one host's Store B today. Adoption is how a published object
  enters a computer's own tape. A self-hosted or desktop computer adopting
  from choir.news needs three things:
  - a signed export format;
  - verification of the publishing host's platform key;
  - the adoption act recording the foreign receipt.
- **A local corpus** means running source schedules (polls, synthesis) on
  the desktop. The cost case is in the pricing notes: one 14-source paper
  costs about $109 a day at list prices. A shared paper amortizes that and
  a personal one does not. That favors consuming the network's paper and
  running local *additions*. This is an inference from the cost figure.

### 3.8 Identity and federation

From [precommitment-signing-and-computer-keys-2026-10-11.md](../precommitment-signing-and-computer-keys-2026-10-11.md):

- One host-held Ed25519 **platform key** in corpusd signs every
  event-head receipt, checkpoint and credential envelope. Its public half
  is pinned in the repo.
- Real **computer keys** are blocked on six named items, including
  ownership semantics, a registry, rotation and fork re-keying.

What changes for a self-hoster:
- **The self-host operator is the witness.** On Choir's cloud, the
  platform witnesses a computer's commit times independently of the
  computer. On a self-hosted host or a desktop, the owner runs the host,
  so their records are self-witnessed. They are worth as much as the
  owner's word.
- **Federation therefore needs:**
  1. A **registry of platform keys**, one per host or cloud, which a
     network participant can verify.
  2. An **external witness** for records that should count as a public
     track record. The earlier note recommends "publish all commitment
     hashes when they are made" and "anchor the platform's heads
     externally". For a self-hoster, the network itself can be that
     witness: commitment hashes submitted to the network at commit time.
  3. **Computer keys** that survive a lost disk, if records should outlive
     a host move.
  4. **Signed publication and adoption** across hosts (§3.7).
- **One thing gets easier on a desktop.** The doc's sixth blocker, "the
  host operator can read every guest key", stops being a privacy problem,
  because the operator is the owner. It still matters for anything that
  must be trusted by others.

### 3.9 Updates

Three layers, each with its own path:

1. **The desktop app binary**, which carries the host services and the
   guest base image. It needs Developer ID signing, notarization and an
   updater, either Sparkle-style or Wails' own.
2. **The guest base image.** On Node B, host activation swaps
   `/var/lib/go-choir/guest` (`node-b.nix:1093`) and vmctl's deploy-refresh
   cold-boots idle guests after asking for a planned-restart marker
   (`internal/vmctl/deploy_refresh.go:15-26`). Busy guests are deferred.
   The same flow works locally.
3. **App-layer releases** inside the guest go through `choir-updater`
   (`internal/updater/updater.go`), which stages, swaps, health-checks and
   rolls back. Station LU
   ([lu-guest-updates-design-2026-10-14.md](lu-guest-updates-design-2026-10-14.md))
   plans a pull channel with rings and drain. A desktop is a natural ring
   member: it pulls signed releases from Choir's release feed (and, for
   forks, its own).

**Restart rules** (CLAUDE.md, "Restarts End Work"):
- Quitting the app should be a **planned hibernate**, which writes the
  marker and lets work resume.
- A power loss or app crash is a crash and closes open work.
- Laptop sleep should pause VMs, not restart them. VZ supports pause and
  resume.
- Without this mapping, every lid close becomes a crash restart and ends
  work, which is wrong for a desktop.

---

## 4. "Any kind of infra": targets and order

Nested virtualization facts are from public sources (end of this note) and
dated where known. Items marked *(check)* were not confirmed against a
primary source.

| # | Target | Virtualization | What it needs from Choir | Abstraction |
| --- | --- | --- | --- | --- |
| 1 | **Single Linux server, bare metal** (Intel or AMD; OVH, Hetzner dedicated, colo) | Native KVM | NixOS module with options (§1.3), `kvm-amd`, self-host guide, backup/restore runbook | `nixosModules.choir` |
| 2 | **Cloud VMs with nested virtualization** | AWS: nested virtualization on *virtual* instances since 2026-02-16 for C8i, M8i and R8i (KVM and Hyper-V as L1), and on `.metal` before that. GCP: supported on Intel machine series, opt-in *(check per series)*. Azure: supported on many Intel v3+ sizes *(check)*. Hetzner Cloud: no nested KVM per user reports (dedicated servers work). OVH Public Cloud *(check)*; OVH bare metal works (Node B) | Same as 1. Also a non-NixOS install path (a generated systemd bundle or a host container plus host Firecracker), because most cloud images are Ubuntu or Debian | Host bundle independent of NixOS |
| 3 | **Desktop Linux** (Wails) | Native KVM | Supervisor, single-owner auth, the app as the window | Supervisor plus the existing Firecracker backend |
| 4 | **macOS desktop** (Wails) | VZ / Hypervisor.framework; nested on M3+ with macOS 15+ | aarch64 guest; VZ backend (B) or Linux appliance (A); entitlements; notarization | `VMManager` with backend selection; guest network and identity off source IPs |
| 5 | **Windows desktop** | WSL2 (nested KVM) or Hyper-V | Appliance in WSL2 first (A); Wails on WebView2; an auth bridge for Windows Hello | Same as 4 |
| 6 | **Kubernetes** | KVM through a device plugin (e.g. KubeVirt's), or Kata Containers (QEMU, Cloud Hypervisor or Firecracker) through `RuntimeClass` | Multi-host first. vmctl per node with a central route CAS; Dolt as a service; per-computer disks that stay put while pods reschedule (local PVs and node affinity, or a VM-per-pod model that hands lifecycle to Kata, which conflicts with vmctl's snapshot and hibernate ownership) | A multi-host Private Choir Cloud design |

**Why this order.**
- (1) is nearly working.
- (2) unlocks most VPS users and is mostly packaging.
- (3) is cheap once (1) and the supervisor exist.
- (4) is the owner's "essential component" and carries the largest new
  backend.
- (5) can reuse the appliance.
- (6) needs the multi-host design that Private Choir Clouds need anyway.
  An org that wants Kubernetes today can run (1) on a node pool of bare
  metal or nested-virt VMs.

---

## 5. Licence: facts for the owner's decision

**Current state:**
- The public repo `choir-hip/go-choir` has **no LICENSE file**. GitHub
  reports `licenseInfo: null`.
- Under default copyright, others may view and fork on GitHub (GitHub's
  terms) but have no licence to use, modify or redistribute. "Choir is
  open source" is an intent, not yet a legal fact.
- Contributors so far appear to be the owner and agents working for the
  owner (the git author is the owner). Relicensing is easiest **now**,
  before outside contributions arrive.

| Option | Self-hosting | Hosted-service case (someone sells hosted Choir) | Contributors | Examples |
| --- | --- | --- | --- | --- |
| **AGPL-3.0** | Free | Allowed, but section 13 requires offering modified source to network users | OSI open source. Some companies ban AGPL internally. With a CLA, the owner can also sell commercial licences (dual licensing); with only a DCO, relicensing later needs every contributor | Grafana (Apache to AGPL, 2021); Mastodon; Synapse (Apache to AGPL plus CLA, 2023); Redis (added AGPL in 2025 after the 2024 SSPL/RSAL change); Elastic (added AGPL in 2024) |
| **Apache-2.0 / MIT** | Free | Allowed with no obligations; a cloud can resell without contributing back | Lowest friction. Apache adds an explicit patent grant. Widest adoption | Dolt and Firecracker (Apache-2.0); Wails (MIT); AT Protocol reference code (MIT/Apache) |
| **Source-available** (BSL 1.1, FSL, Elastic License v2, SSPL) | Usually free for own use | Restricted or forbidden for competing hosted offerings; BSL and FSL convert to open source after a delay (FSL: 2 years) | **Not open source** (OSI). Contributors and distributions treat it as proprietary. Past switches triggered forks (Terraform to OpenTofu, 2023; Redis to Valkey, 2024) | HashiCorp (BSL, 2023); Sentry (FSL, 2023); MongoDB (SSPL, 2018) |

**Facts that bear on "encourage self-hosting" and "the network is the business":**
- **No OSI licence forbids a cloud from reselling.** AGPL only requires
  that their modifications be published. Preventing resale needs a
  source-available licence, which contradicts "open source".
- **The network is a separate lever from the code licence.** It is gated
  by keys, registries and terms of participation (§3.8), and by trademark
  ("Choir", "Autopaper", "Autoradio"). A permissive or AGPL codebase can
  sit behind a network that requires registered platform keys.
- **Split licensing is a common pattern:** the server under AGPL, and
  protocol specs, client libraries and SDKs under Apache or MIT, so that
  third-party implementations can join a federation freely.
- **Third-party components are compatible with any of these options.**
  SearXNG (AGPL-3.0) runs as a separate service, not linked. Dolt, Qdrant,
  zot and Firecracker are Apache-2.0; microvm.nix is MIT. The `obscura`
  fork's licence should be checked before choosing.

**Clear without a recommendation on the licence itself:** add a LICENSE,
and pick a CLA or a DCO at the same time, before outside contributions
are accepted.

---

## 6. Phased plan

| Phase | Work | Mutation class | Notes |
| --- | --- | --- | --- |
| **P0** | Problem docs first: tap deny gap (filed with this note); desktop entitlements key; desktop local mode stale; README "Try it" needs Firecracker. Draft `docs/self-hosting.md` describing **today's** reality: prerequisites, the service table, secrets, a fork-the-module path, backup. Owner decides licence plus CLA/DCO | green | No runtime change. The licence file itself is green but is an owner decision |
| **P1** | **Config extraction.** `nixosModules.choir` with options: domain, admin SSH keys, CPU vendor, builders, paper owners, Resend, state dir; `node-b` becomes one instance. Fix maild's hard-coded sender and links (orange). Parametrize the deploy scripts. A tiers override for forks needs an owner ruling (tiers are deliberately not host config) | orange (Nix and maild); red where the host module touches vmctl or deploy routing | Rollback: `node-b` builds the same closure (diff the system derivation before and after). Acceptance: staging deploy identical, plus a fresh VM install from the guide |
| **P2** | **VM backend seam.** `VM_BACKEND` selection with the fail-closed gate kept; move guest networking behind a `GuestNetwork` interface; replace source-IP caller identity with per-guest credentials; one `CHOIR_STATE_DIR` root; **aarch64 guest image** (flake `aarch64-linux` outputs, rusty_v8 per architecture) | red (vmctl, gateway auth, guest image) | Full ceremony. Firecracker stays the only staging backend; acceptance stays on staging |
| **P3** | **macOS desktop.** Prototype A (appliance on M3+) to prove a server-less computer end to end; then B (VZ backend through Go bindings or vfkit). Supervisor (sidecars, multi-call binary); single-owner auth; computer-surface UI from the local proxy; Keychain provider keys; quit and sleep mapped to planned hibernate; notarized build | red (VM lifecycle, auth, restart semantics) | Acceptance needs a new admissible evidence class: a desktop proof run with an artifact, since staging cannot prove a Mac |
| **P4** | **Federation.** Platform-key registry; commitment-hash publication to the network; signed cross-host publication export and adoption; then computer keys (the six blockers) | red (evidence ledger, signer custody) | Builds on the precommitment signing note |
| **P5** | Windows (WSL2 appliance, then Hyper-V); non-NixOS server bundle; Kubernetes after a multi-host Private Choir Cloud design | red | |

**What to do first:**
1. Write `docs/self-hosting.md` against today's code (P0).
2. Prepare the licence decision (P0).
3. Extract the Node B assumptions into a NixOS module, checking that
   Node B's system closure is unchanged (P1).

These make "you'll be able to self-host it" true for Linux operators
within one station, and they create the configuration surface that the
VM backend and desktop work need.

**Heresy delta (this note):**
- *Discovered:*
  - tap-to-tap deny covers 1 of 16 guest /16s
  - desktop entitlements key corrupted by the global rename
  - desktop local mode incompatible with fail-closed vmctl
  - desktop serves an embedded UI instead of the computer surface
    (hypothesis)
  - README local stack requires Firecracker without saying so
  - no LICENSE
- *Introduced:* none.
- *Repaired:* none.

## Sources (external facts)

- AWS, "Amazon EC2 supports nested virtualization on virtual Amazon EC2 instances" (2026-02-16): https://aws.amazon.com/about-aws/whats-new/2026/02/amazon-ec2-nested-virtualization-on-virtual
- AWS EC2 user guide, nested virtualization: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/amazon-ec2-nested-virtualization.html
- The Register on AWS nested virtualization: https://theregister.com/2026/02/17/nested_virtualization_aws_ec2
- UTM issue on nested virtualization (macOS 15, M3+): https://github.com/utmapp/UTM/issues/6700
- libkrun issue "Support nested virtualization (macOS 15+)" #375 (krunkit; `VZGenericPlatformConfiguration.isNestedVirtualizationEnabled`)
- Hetzner Cloud nested KVM (user reports, 2022 and 2025): https://blog.wirelessmoves.com/2022/05/nested-virtualization.html
- GCP, Azure, Windows WSL2, Apple Containerization minimum OS, VZ save/restore minimum OS, and licence history dates are from background knowledge and marked *(check)* where they affect a decision.
