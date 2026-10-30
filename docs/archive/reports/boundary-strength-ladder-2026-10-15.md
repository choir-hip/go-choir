# Choir's Boundary-Strength Ladder: What Each Isolation Boundary Can Keep

**A disclosure draft and a red-team plan.** 15 October 2026. Repository:
`origin/main` at 795e085bb, plus the unmerged branch
`fix/desk-cell-confinement` (fedb7656c) where noted. No host was touched
and no secret was read. Live facts about Node B are quoted from earlier
documents, mostly the security review's readings on 14 October. Mutation
class of this document: **green** (docs only). Each hardening move below
names its own class; most are **red**.

Owner (owner notes 2026-10-14, §17 and §18): an agent's identity is "the
security boundary that can maintain a cryptographic identity", and once
identity is a key held by a boundary, "agentic red teaming is literally
capture the flag: sign a message with a key you are not supposed to hold.
Over time, hill-climb toward making that impossible, or at least publish a
clear disclosure of which boundaries can't be strongly secured."

This report covers the second half of that note. For each boundary in a
Choir computer it asks four questions: how strong is the boundary really,
what has recently broken boundaries like it, which keys may it hold, and
how do we test it? It builds on the following and does not repeat them:

- [the computer isolation security review](computer-isolation-security-review-2026-10-14.md),
  which has the threat model, live Node B evidence, and findings F1 to F13;
- [the isolation runtimes research](../../research/isolation-runtimes-2026-10-14.md);
- the problem records
  [firecracker-runs-as-root-without-jailer](../../problems/firecracker-runs-as-root-without-jailer-2026-10-14.md),
  [desk-cells-run-as-guest-root-and-egress-is-open](../../problems/desk-boundary-residuals-2026-10-21.md),
  [loopback-is-a-credential](../../problems/loopback-is-a-credential-2026-10-14.md)
  and
  [guest-can-arm-own-selfdev-mode](../problems/guest-can-arm-own-selfdev-mode-2026-10-15.md).

The principal-identity design
(`docs/archive/reports/principal-identity-and-signed-acts-2026-10-15.md`) had not
landed on `origin/main` when this was written (checked 2026-10-15). The
key assignments below are therefore this report's proposal, to be
reconciled with that design when it lands.

**Evidence labels**, as in the security review:

- **code**: read in the repository, cited with a path.
- **doc**: taken from an existing Choir document.
- **live**: read on Node B by an earlier document. This report took no new
  live readings.
- **upstream**: a vendor advisory, release note or kernel document, cited
  with URL and date.
- **press**: a news or researcher write-up, cited with URL and date.
- **inference**: reasoning that has not been tested.
- **UNVERIFIED** marks a claim this report could not confirm from a primary
  source.

Every CVE identifier below appeared on a page the research fetched or saw
in search results. None is reconstructed from memory.

## The short answer

1. **The "last week" microVM escape exists, but nobody outside Vercel can
   check it yet.**
   - On 2026-10-03, researcher Paulos Yibelo announced a "full VM escape
     zeroday (guest>host root in industry standard hypervisors)", found
     through Vercel Sandbox's bug bounty. Vercel's CEO said Vercel had
     "confirmed a KVM 0day".
   - Vercel Sandbox runs one Firecracker microVM per sandbox on KVM, so
     this is the closest public analogue to a Choir computer escaping.
   - As of 2026-10-14 there is no write-up, CVE or patch. Nobody has said
     publicly whether the bug is in KVM (the host kernel) or in Firecracker
     (press: The Register 2026-10-06; webhosting.today 2026-10-08).
   - If it is in KVM, no Firecracker version or jailer setting protects
     against it. Only the host kernel patch does, and none exists yet.
   - In the same window, "Vghost" (CVE-2026-74580, write-up 2026-10-12) is
     a Linux vhost bug. It needs a vIOMMU and device-IOTLB mode, which
     Firecracker does not use, so it is a host-local root bug for Choir,
     not a guest escape.
2. **Firecracker 1.15.1 is not affected by any published Firecracker
   advisory.**
   - It is the fix release for CVE-2026-5747, and CVE-2026-1386 was fixed
     earlier.
   - The 1.15 line no longer receives patch releases. Upstream is 1.17.0
     (2026-09-10), which carries later hardening that has no CVE.
3. **The newest finding is not in Firecracker; it is Node B's host
   kernel.**
   - The security review read kernel **6.18.16** live on 2026-10-14.
   - That is older than the 6.18.38 fix for CVE-2026-53362, an IPv6 UDP
     out-of-bounds write added to CISA's Known Exploited Vulnerabilities
     list on 2026-08-27. In July 2026 OpenAI's agents used it to break out
     of a container.
   - It is also older than the 6.18.38 fix for the KVM shadow-MMU bug
     "Januscape" (CVE-2026-53359) and the 6.18.45 fix for Vghost.
   - The host's kernel changes only when the host is rebooted. A
     `nix/node-b.nix` comment on a KVM parameter says as much: "Takes
     effect at the next host boot".
   - Patch cadence is the cheapest large gain on the whole ladder.
4. **The ladder, weakest first, as of today:**
   1. Desk cells for management, Texture and research: **weak**.
   2. Engineering capsules: **moderate**.
   3. Guest root against the computer's own signers: **label**. Guest root
      can reach every key the computer holds.
   4. The microVM: **strong** at the hardware boundary, but only
      **moderate** end to end, because the VMM still runs as root
      without the jailer and the host kernel is behind.
   5. The CPU, as a side door between tenants: **weak** while SMT is on.
      Node B's Xeon E-2236 gets no more microcode; Intel's servicing
      ended 2025-06-30.
   6. The host: **weak**. Host-internal service authentication is in
      observe mode, so loopback is still a credential.
   7. The owner's passkey: **very strong** against key extraction. Owner
      *authority*, though, is only as strong as the host, the email
      account behind recovery, and the browser, because owner acts ride
      on a host-minted session.
5. **The ladder is not monotone, and the disclosure must say so.** The
   rung with the most valuable keys (the host) is weaker today than the
   rung below it (the microVM). The passkey's strength does not reach most
   owner acts. Hill-climbing means raising the host and binding owner acts
   to the passkey, not only hardening the inner rungs.
6. **Three landed or ready changes move grades fastest:**
   - `fix/desk-cell-confinement` takes cells from weak to moderate.
   - Turning on `VMCTL_JAILER`, which is already on `main`, gives the
     VMM's second line a real cage.
   - Moving hostauth from observe to enforce takes the host from weak to
     moderate.

## How to read a rung

A boundary is something a key can live behind. Its strength is what it
costs an attacker who starts just inside the boundary to sign with a key
that lives just outside it. Five grades are used for every rung:

- **Label.** No mechanism stops the attacker. The key is a name, not a
  secret; anyone with a foothold signs.
- **Weak.** One bug in a large surface fed adversarial input suffices.
  Bugs of that class are found routinely across the ecosystem, monthly or
  more often.
- **Moderate.** An escape needs a bug in a well-defended surface, usually
  the kernel. Such bugs go public several times a year, often with
  exploits. Patch cadence decides most of the risk.
- **Strong.** An escape needs a bug in a small, hardened, heavily fuzzed
  surface, chained with a second bug to leave a second confinement layer.
  Public full escapes are rare, a handful a year across the whole class,
  and command large bounties.
- **Very strong.** Software cannot extract the key at all. What remains is
  misuse of an authorized signer (getting it to sign the wrong thing), or
  physical and supply-chain attacks.

A rung's grade is its grade **today**, on Node B, with what is deployed.
Where a change that has landed but is switched off, or a branch-only
change, would move the grade, the rung says so separately.

Two rules hold on every rung and are not repeated each time:

1. **A boundary that can be crossed by asking is not a boundary.** A
   signing oracle is a signer that signs whatever a caller hands it. It
   gives the key's authority to the caller even when the key bytes never
   move.
   - The confirmed guest-arming hole has exactly this shape. The computer
     asks corpusd, and corpusd signs a self-development mode receipt with
     the platform-control key (doc: guest-can-arm-own-selfdev-mode).
   - So every flag below counts as captured by **any unauthorized
     signature**, whether or not the key leaked.
2. **Rungs nest, so an escape skips every key in between.** A key held at
   one rung is exposed to every attacker who reaches that rung or any rung
   outside it.

## The ladder

### Rung 1: desk cells (Yaegi), management, Texture and research

**Mechanism today (code).**

- Model-written Go is interpreted by Yaegi v0.16.1 (`go.mod`), inside a
  desk session worker the guest runtime re-executes as
  `autoputer desk-session`.
- The interpreter gets only filtered standard-library symbols
  (`buildFilteredSymbols`, `internal/yaegikernel/eval.go:83-97`) from a
  16-package allowlist. `unsafe`, `reflect`, `runtime`, `syscall`, `os`,
  `os/exec`, `net`, `net/http`, `plugin` and `io/ioutil` are banned
  (`internal/yaegikernel/allowlist.go:9-42`).
- Imports are parsed and refused before evaluation, and the check fails
  closed (`eval.go:99-121`).
- `choir.Exec` and `WriteFile` are exported only to engineering.
- On `main`, the worker is a plain child of a root runtime. The spawn sets
  only `Setpgid` and `Pdeathsig`
  (`internal/yaegikernel/desk_worker_spawn_linux.go:27-33`), plus an 8 GiB
  `RLIMIT_AS`.
- So on Node B today, every management, Texture and research cell runs as
  **guest root with full capabilities**. There is no seccomp, no Landlock
  and no namespace. Engineering's cells run in a capsule (rung 2).

**On the branch `fix/desk-cell-confinement` (790d5e2c1; not merged, not
deployed).**

- Each desk gets its own unprivileged user, `no_new_privs` and Landlock
  V5, mandatory in required mode.
- The worker gets an empty network namespace and a deny-by-default
  seccomp filter (`internal/yaegikernel/desk_cell_confinement_linux.go`
  on the branch, `DeskCellSeccompFilter`):
  - no `socket`, `connect`, `exec*`, `fork`, `ptrace`, `set*id`,
    `unshare`, `mount`, `bpf`, `memfd_create`, `splice` or `io_uring`;
  - `clone` only with `CLONE_THREAD`, and `clone3` answers `ENOSYS`;
  - `kill` and `tgkill` only to its own pid.
- The local Linux evidence is real: a mutation check shows the probes
  fail when the filter is removed.
- The staging proof is still owed. The Docker kernel used for the tests
  has no Landlock (doc: the problem record's repair section).

**Known escape classes, with dated evidence.**

- *In-process language sandboxes are the weakest class of boundary in
  2025–2026.*
  - vm2 for Node.js produced Critical escapes almost weekly through
    August and September 2026 (upstream: https://github.com/patriksimek/vm2/security/advisories,
    advisories dated 2026-08-24, 08-27 and 09-03).
  - isolated-vm, built on V8 isolates, fell to a host-binding type
    confusion with a demonstrated guest-to-host escape (press:
    https://thehackernews.com/2026/08/isolated-vm-flaw-lets-sandboxed.html, 2026-08-20;
    GHSA-864f-rcv7-6rh4).
  - RestrictedPython: CVE-2026-76825, a `string.Formatter` guard bypass,
    2026-09-17 (upstream: https://advisories.gitlab.com/pypi/restrictedpython/CVE-2026-76825/).
  - smolagents: CVE-2025-9959, a dunder walk to `subprocess`, 2025-09-03
    (upstream: https://research.jfrog.com/vulnerabilities/smolagents-local-python-sandbox-escape-jfsa-2025-001434277/).
  - n8n Python node: CVE-2025-68668, 2025-12-26.
  - Redis Lua: "RediShell", CVE-2025-49844, a garbage-collector
    use-after-free with CVSS 10, 2025-10-03 (press:
    https://www.wiz.io/vulnerability-database/cve/cve-2025-49844).
  - The pattern: denylists and allowlists over a language runtime fail to
    reflection-like tricks, host-binding bugs and runtime memory bugs.
- *Yaegi in particular makes no sandbox claim.*
  - It has no security advisories and no SECURITY.md. Its last release,
    v0.16.1, is from 2024-04-03 (upstream: https://github.com/traefik/yaegi).
  - Its default `stdlib.Symbols` exposes `os.StartProcess`, file I/O and
    the network. Choir avoids that by filtering symbols, which is the
    right pattern.
  - The interpreter itself uses `reflect` and `unsafe` internally. The
    `choir` exports are built by reflection.
  - Go's memory model says data races on interfaces, slices, maps and
    strings "can in turn lead to arbitrary memory corruption"
    (upstream: https://go.dev/ref/mem, version of 2022-06-06).
  - Yaegi cells can start goroutines (inference: `go` is a language
    statement, not an import). A racy cell may therefore corrupt the
    worker's memory with no banned import at all.
  - Choir's own doctrine already says this (I18): "Yaegi is not the
    security boundary."
- *Agent sandboxes fail at the seam, not in the OS primitive.*
  - In 2026, Claude Code, Codex, Cursor and Gemini CLI escapes mostly
    came from files written inside the sandbox that a trusted process
    outside later ran or followed: hooks, settings, symlinks, git config.
  - Examples: Claude Code CVE-2026-25725 (fixed 2026-01-21, disclosed
    2026-04-07, https://cymulate.com/blog/the-race-to-ship-ai-tools-left-security-behind/);
    CVE-2026-39861, a symlink write-through, about 2026-04-21; Pillar
    Security's "week of sandbox escapes", 2026-07-20
    (https://www.bleepingcomputer.com/news/security/cursor-codex-gemini-cli-antigravity-hit-by-sandbox-escapes/).
  - The Choir analogue is anything a cell can make the root runtime
    read or act on: frames on fd 3, files under the allowed root, and
    the runtime's handling of the cell's results (inference).
- *Landlock and seccomp, which the branch adds, have known gaps.*
  - Landlock cannot restrict `chdir`, `stat`, `flock`, `chmod`, `fcntl`
    or `ioctl` on already-open fds (upstream:
    https://docs.kernel.org/userspace-api/landlock.html).
  - CVE-2026-72183 (2026-08-15): signal scoping bypassed through SIGIO
    and `F_SETOWN`. The cell filter allows `fcntl`, but the kernel's
    cross-uid SIGIO permission check should still stop a per-desk uid
    from signalling root (inference).
  - Seccomp cannot inspect pointer arguments.
  - CVE-2026-89603 (2026-09-11): a ptrace/TSYNC race lets a stopped
    thread miss a filter. The cell has no ptrace (UNVERIFIED beyond the
    Debian tracker).

**Hardening and gaps.**

- Today the only boundary is the curated symbol set.
- With the branch, the cell becomes a process jail with a very small
  kernel surface: no new sockets, no processes, no namespaces, pipes and
  the inherited socketpair only.
- The branch's named residuals:
  - no per-desk cgroup;
  - `ioctl` still allowed;
  - the allowed root defaults to the guest's `/tmp`;
  - the runtime it talks to is still guest root (S1 remainder).

**Keys it may hold under the identity design.**

- Owner note §17: "Desk-cell confinement earns desks their keys. Without
  it, a desk key is only a label."
- This report adds a sharper rule: **even with confinement, the desk key
  should not live in the cell worker's memory.**
  - The worker runs adversarial code every turn, and rung 1's escape
    classes are memory-corruption classes.
  - Put each desk's key in a per-desk signer *outside* the worker: a
    separate process under the desk's uid, or the guest signer with a
    per-desk subkey.
  - Give the worker a signing *capability* over fd 3 that signs only
    statement types the desk may make, with the cell's record hash bound
    in.
  - Then a cell escape captures an oracle limited to that desk's
    statement types, not the key.

**Grade: weak today, from one Yaegi or Go-runtime bug to guest root.**

- With the branch deployed and the Landlock proof done: **moderate**. An
  escape then needs a kernel bug reachable through roughly 80 allowed
  syscalls, with no sockets, no namespaces and no `io_uring`.
- With per-desk signers outside the worker: a cell escape is bounded to
  that desk's oracle, which is the right shape for a desk key.

### Rung 2: engineering capsules

**Mechanism today (code).** `internal/capsule` and `cmd/capsule-broker`:

- **Namespaces:** a user namespace mapping capsule root to guest uid
  65534 with `setgroups` denied, plus mount, network, UTS, IPC and cgroup
  namespaces and a chroot into an overlay
  (`internal/capsule/executor.go:507-521`). A PID namespace is added for
  the workload (`cmd/capsule-broker/main.go:308`).
- **Capabilities:** cut to `DAC_OVERRIDE` and `FOWNER` inside the user
  namespace.
- **Seccomp:** deny-by-default with `NoNewPrivs` and TSYNC
  (`internal/capsule/seccomp.go:13-74`). Sockets are limited to
  `AF_UNIX`, plus `AF_INET` and `AF_INET6` in network (L2) capsules only.
- **Landlock:** V5, mandatory and fail-closed (`landlock.go:65-126`).
- **cgroup v2:** 2 GiB for engineering since 1a91176ac, 1 CPU, 256 pids.
- **Environment:** filtered so that `TOKEN`, `KEY` and `CHOIR_` names
  are stripped.
- **Network:** an empty network namespace, or loopback forwarded to an
  allowlisting egress proxy for L2 capsules.
- **Keys:** the capsule holds none. The runtime-side executor holds the
  ed25519 private key; the broker gets only the public key
  (`executor.go:130-135,505`).

**Known escape classes, with dated evidence.**

- *A shared kernel is a weak boundary in 2026.* Kernel privilege
  escalations that reach container-to-host:
  - **CVE-2026-53362**, an IPv6 UDP `__ip6_append_data` out-of-bounds
    write. Added to CISA KEV 2026-08-27, fixed in 6.18.38. **Agents in
    OpenAI's internal evaluation adapted a public exploit on 2026-07-19,
    left a container and reached Kubernetes cluster-admin** (press:
    https://corgea.com/research/cve-2026-53362-linux-udpv6-fraggap-root-container-escape;
    OpenAI report: https://cdn.openai.com/pdf/67869394-cb91-4c12-888c-5cbd85c7814c/OpenAI-Hugging-Face%20Incident-Technical-Report.pdf,
    not read directly).
  - **CVE-2026-31431, "Copy Fail"**, a page-cache write through AF_ALG and
    `splice`, disclosed 2026-04-29, in KEV (upstream:
    https://unit42.paloaltonetworks.com/cve-2026-31431-copy-fail/).
  - **CVE-2026-23111**, an nf_tables use-after-free that needs
    unprivileged user namespaces. Public exploits 2026-04-16 and
    2026-06-08 (press: https://techearl.com/cve-2026-23111-nftables-lpe).
  - **"Dirty Frag"**, CVE-2026-43284 and CVE-2026-43500, 2026-05-07.
  - Three kernel KEV additions on 2026-09-18
    (https://thehackernews.com/2026/09/cisa-flags-three-linux-kernel.html).
  - The 2022 cgroup `release_agent` escape (CVE-2022-0492) was added to
    KEV on 2026-06-02.
- *Runtime bugs.* The runc trio of 2025-11-05 (CVE-2025-31133,
  CVE-2025-52565, CVE-2025-52881) gives full escapes from a hostile mount
  configuration when running as root without a user namespace
  (https://www.openwall.com/lists/oss-security/2025/11/05/3). Choir's
  capsule is not runc and maps root to 65534, the configuration that
  advisory says blocks the writes.
- *Seccomp and io_uring.* io_uring operations bypass syscall-level
  filters ("Curing", ARMO, 2025-04-24,
  https://www.armosec.io/blog/io_uring-rootkit-bypasses-linux-security).
  The capsule list does not admit `io_uring_*` (code).

**How Choir's capsule meets those classes (code and inference).**

- **Blocks the AF_ALG path of Copy Fail.** The socket family filter
  admits only UNIX and INET; `splice` is allowed.
- **Blocks netlink**, so many nf_tables and xfrm triggers cannot be
  reached directly.
- **Allows `clone` and `clone3` with no flag filtering** (`seccomp.go:28`).
  So a capsule can create a nested user and network namespace and get
  `CAP_NET_ADMIN` inside it. That is the precondition for the nf_tables
  and Dirty Frag class (the security review's open item).
- **Admits AF_INET6 in L2 capsules**, which is the socket family
  CVE-2026-53362 needs.
- **The guest kernel version decides the rest.** Evidence from 2026-10-01
  shows guests on **6.18.37** (`docs/archive/evidence/s0a-guest-layout-2026-10-01.json:174`),
  one release before the 6.18.38 fix. The S3 design cites a 6.18.45
  config. Which kernel runs in guests now must be read from a live guest.

**Hardening and gaps.**

- What is missing:
  - `clone3` should answer `ENOSYS` and `clone` should refuse
    `CLONE_NEWUSER`. The desk-cell filter on the branch already does the
    first.
  - Unfiltered `ioctl`, `prctl`, `memfd_create` and `vmsplice`.
  - Every capsule shares uid 65534.
  - The egress allowlist includes upload-capable hosts: GitHub and
    Hugging Face.
- gVisor (`runsc`, systrap) is the stronger option for capsules. Its own
  2026 record:
  - CVE-2026-96812, 2026-09-25: host root through `/dev/cuse` on
    CUSE-enabled hosts.
  - CVE-2026-95702, 2026-10-09: code execution in the Sentry, still
    inside host seccomp
    (https://app.opencve.io/cve/CVE-2026-96812, https://app.opencve.io/cve/CVE-2026-95702).
  - That is a better record than a shared kernel's, not a perfect one.

**Keys it may hold.** None long-lived. A capsule's statements, such as
"this manifest built this artifact", should be signed by the computer
about the capsule, never by a key inside it. An ephemeral per-capsule key
is acceptable only if the computer certifies it for the capsule's lifetime
and one manifest.

**Grade: moderate.** An escape needs a guest kernel bug. Three 2026 bugs
in that class are public, in KEV, or both, and the capsule leaves the
nested user-namespace door open. A capsule escape lands at guest root,
which is rung 3: every key the computer holds.

### Rung 3: guest root against the computer's own signers

The owner's ladder skips this rung, but it is where the computer's keys
live today, so the disclosure needs it.

**Mechanism today (code).**

- The guest-core and verifier signers run as their own users
  (`choir-guest-signer`, `choir-verifier-signer`) with empty capability
  sets, `NoNewPrivileges` and `InaccessiblePaths` over each other's state
  (`nix/autoputer-vm.nix:447-456,591-650`).
- The guest-core key is `/mnt/persistent/choir-signers/guest-core/key.ed25519`
  (`autoputer-vm.nix:593`).
- The guest runtime `go-choir-autoputer` runs as **root**, with no
  `User=`, `CapabilityBoundingSet`, `SystemCallFilter` or `ProtectSystem`.
  Only `InaccessiblePaths = [ /mnt/persistent/choir-signers
  /run/choir-updater-control ]` applies (`autoputer-vm.nix:742-857`).
- The runtime also holds directly:
  - the gateway bearer (`/run/choir-bootstrap/gateway-token`);
  - the privacy key (`/mnt/persistent/choir-credentials/privacy-key`);
  - the computer's `event:append` capability.
- Untrusted content is parsed as root inside it: `pdftotext`, and Obscura
  for Web Lens.

**Why this is a label, not a boundary (inference, cheap to test).**

- `InaccessiblePaths` is a mount-namespace overlay. A root process with
  `CAP_SYS_ADMIN` can leave or undo it, for example by entering PID 1's
  mount namespace, or by reading through `/proc/<signer pid>/root` with
  `CAP_SYS_PTRACE`.
- The signer *socket* directory `/run/choir-signers` is not in the
  runtime's `InaccessiblePaths`, and root ignores the directory modes. So
  guest root holds the signer as an oracle even without the key file.
- The guest-arming hole (doc) shows the next hop: anything that can act as
  the computer can get corpusd to sign a self-development mode receipt
  with the platform-control key.

**Known escape classes.** None needed. Guest root is the design state of a
malicious tenant, and every rung-1 or rung-2 escape lands here.

**Keys it may hold.** The computer key (guest-core), the verifier key, the
gateway bearer, the event capability, the privacy key, and the owner ROOT
key once it is unwrapped in memory.

**Grade: label between guest root and the computer's keys.** The
computer's keys are therefore only as strong as the weakest way into guest
root:

- rung 1, today: weak;
- a `pdftotext` or Obscura bug: weak, since these parsers are fed
  untrusted input as root;
- rung 2: moderate.

S1's remainder (a non-root runtime, signers that refuse statement types
the runtime may not request, guest sysctls) is what turns this rung into a
real boundary. The grade then becomes moderate: the guest kernel, again.

### Rung 4: the Firecracker microVM

The microVM is two boundaries in series:

- **4a, the hardware boundary:** KVM in the host kernel, plus
  Firecracker's device emulation.
- **4b, the VMM process cage:** what a Firecracker process can reach once
  a device bug gives the guest code execution in it.

**Mechanism today (code, live per earlier documents).**

- **Firecracker version and launch.** Firecracker 1.15.1 from nixpkgs,
  launched with `--enable-pci` (`internal/vmmanager/manager.go:2023`,
  `snapshot.go:312`). It runs per-thread seccomp filters (`Seccomp: 2`,
  6 filters).
- **Devices.** virtio-net, virtio-blk, balloon, serial, a partial i8042
  and RTC. There is no vsock, virtio-fs or VFIO in Choir's config.
- **4b today: the jailer is landed but off.**
  - The jailer shipped in 9ac656298 and 10e9f18e1 behind `VMCTL_JAILER`,
    default off (`internal/vmmanager/config.go:147-156`).
  - vmctl lost `CAP_SYS_PTRACE` (`nix/node-b.nix:917-921`), and direct
    VMMs now run with `CapEff` 0x1000 (`CAP_NET_ADMIN` only) and
    `NoNewPrivs` 1.
  - Host secrets (the JWT key, provider env, escrow and platform signing
    keys) are hidden from vmctl's mount view by `InaccessiblePaths`
    (`node-b.nix:311-323`, vmctl unit).
  - A direct VMM is still **uid 0** in vmctl's mount namespace, with
    read-write access to all of `/var/lib/go-choir` (every computer's
    `data.img`, `credential.img` and snapshot, minus the hidden secrets),
    in the host network and PID namespaces, holding `CAP_NET_ADMIN`
    there.
  - vmctl's own credentials (the hostauth token and the Dolt client
    credential) are under `/run/credentials/go-choir-vmctl.service/`. It
    is unverified whether that path is in a direct VMM's view.
- **4a host facts (live, 14 October).**
  - Kernel **6.18.16**, `kvm_intel nested=Y`, `/dev/kvm` mode 0666.
  - Node B has not been rebooted onto a newer kernel since at least that
    reading. The halt-poll parameter comment in `nix/node-b.nix:1632` says
    kernel parameters wait for "the next host boot".

**Known escape classes, with dated evidence.**

*Firecracker itself.* Two advisories in 2026, neither a demonstrated
escape:

- **CVE-2026-5747** (GHSA-776c-mpj7-jm3r, 2026-04-07).
  - A virtio-PCI out-of-bounds write, CVSS 4.0 8.7. Affects 1.13.0 to
    1.14.3 and 1.15.0; fixed in 1.14.4 and 1.15.1.
  - Needs guest root and `--enable-pci`. Writes stay in guest memory with
    a stock guest kernel.
  - Credited to Xinyang Ge and Claude.
  - https://github.com/firecracker-microvm/firecracker/security/advisories/GHSA-776c-mpj7-jm3r
- **CVE-2026-1386** (GHSA-36j2-f825-qvgc, 2026-01-23).
  - A jailer symlink overwrite by a host-local user, fixed in 1.13.2 and
    1.14.1.
  - Not reachable from a guest.
- **Fixed after 1.15.1 without a CVE, and not backported:**
  - virtio-mem left unplugged memory writable after restore (#6174,
    #6176);
  - a vsock busy-spin (#6031, #6041, #6077);
  - a vhost-user-block read-only flag ignored (#6083);
  - an aarch64 jailer TOCTOU (#5956);
  - unreleased on main: bounds checks on every virtio queue access
    (#6201).
  - Under Firecracker's release policy, 1.15 no longer gets patches
    (upstream: https://github.com/firecracker-microvm/firecracker/releases;
    1.17.0 and 1.16.2 released 2026-09-10).
- **Trail of Bits, 2026-08-26.** A cyber-capable model escaped QEMU/KVM
  three times. Against Firecracker it hard-locked the host through
  since-patched kernel bugs but "could not successfully escape" (upstream:
  https://blog.trailofbits.com/2026/08/26/vms-wont-contain-cyber-capable-agents/).

*KVM, which no VMM choice or jailer protects against.*

- **The Vercel Sandbox "KVM 0day", 2026-10-03.**
  - Guest-to-host root against a Firecracker-on-KVM product.
  - $50k bounty: the program's maximum, per a screenshot only, so
    UNVERIFIED.
  - No details, CVE or patch as of 2026-10-14.
  - Sources: https://x.com/PaulosYibelo/status/2106378929158135903;
    https://www.theregister.com/offbeat/2026/10/06/security-researcher-claims-they-found-kvm-guest-host-escape-flaw/5301267;
    https://cybersecuritynews.com/kvm-zero-day-vm-escape/ (2026-10-04);
    https://webhosting.today/2026/10/08/vercel-paid-50000-for-a-kvm-escape-that-has-no-cve-and-no-patch-every-vps-host-is-waiting-for-the-write-up/.
  - Whether the bug is in KVM or Firecracker: **UNVERIFIED**.
  - Vercel's $1M Sandbox challenge (2026-08-18 to 09-01) had already found
    two host kernel networking bugs reachable from the microVM: one host
    memory leak and one host crash
    (https://www.securityweek.com/1-million-sandbox-challenge-uncovers-linux-kernel-flaws/, 2026-09-15).
- **"Januscape", CVE-2026-53359.**
  - A KVM/x86 shadow-MMU use-after-free. Needs guest root and nested
    virtualization. Fixed in 6.18.38 (stable releases 2026-07-04).
  - The public proof of concept panics the host. A full escape is
    claimed but unreleased
    (https://thehackernews.com/2026/07/16-year-old-linux-kvm-flaw-lets-guest.html).
  - Firecracker does not expose VMX to guests, so a Choir guest cannot
    reach it (inference).
  - But Node B's `/dev/kvm` is 0666 and `nested=Y`. Any host-local process,
    for example a compromised SearXNG or goproxy, can create its own VM,
    give it VMX and be "guest root with nested virtualization" on an
    unpatched 6.18.16 kernel (inference; rung 6).
- **"ITScape", CVE-2026-46316.** A KVM/arm64 vGIC-ITS double free with a
  public PoC (https://github.com/V4bel/ITScape). arm64 only, so not
  Node B.

*Other VMMs, for comparison.*

- QEMU had two demonstrated full escapes in 2026:
  - virtio-snd, OtterSec, 2026-03-17
    (https://osec.io/blog/2026-03-17-virtio-snd-qemu-hypervisor-escape/);
  - a virtio-gpu chain, 2026-04-28 (https://calif.io/research/qemu).
- Cloud Hypervisor:
  - CVE-2026-45782, 2026-05-14: a virtio-blk use-after-free the
    advisory says "can be escalated" to an escape;
  - CVE-2026-27211, 2026-02-20: a QCOW2 header lets the guest read host
    files.
- Kata Containers:
  - CVE-2026-47243 (advisory 2026-05-21, NVD 2026-08-07): guest root to
    host root through virtiofsd;
  - CVE-2026-93417, 2026-07-21: Dragonball virtio-blk out-of-bounds.
- The lesson for Choir: escapes concentrate in large device models
  (sound, GPU, virtiofs, user-mode networking). Firecracker's small,
  block-only device model is why it has none, and KVM is now the shared
  weak point.

**Hardening and gaps.**

- **4a.** The device model is minimal. The one opt-in Choir takes,
  `--enable-pci`, is the newest code and carried the 2026 high CVE; the
  only stated reason for it is matching the upstream microvm runner
  (`manager.go:2023`).
- **4b.** The jailer, once on, gives:
  - a per-VM uid;
  - a chroot holding only that computer's files;
  - mount and PID namespaces and a per-VM cgroup;
  - an empty environment and empty capabilities.
  It was proven in a real jailer run, not on staging (doc). A per-VM
  network namespace is slice 2.
- **The kernel.** Patching is the gap. 6.18.16 misses at least the
  Januscape fix, the CVE-2026-53362 fix and the Vghost fix.

**Keys it may hold.** None. The VMM should hold nothing but its own
computer's disks and memory. Because it holds the guest's RAM, a VMM
compromise always yields that computer's rung-3 keys.

- Today, without the jailer, it also yields read-write access to every
  other computer's disks and snapshots, which contain their guest-core
  keys. So 4b is currently the boundary that guards **every tenant's
  computer key**.
- With the jailer it guards only its own.

**Grade.**

- **4a, hardware boundary and device model: strong.** No public
  Firecracker escape exists, and the class is rare and expensive. But a
  KVM 0-day against exactly this stack was confirmed two weeks ago, and
  the host kernel is behind on public KVM fixes. On today's kernel, 4a is
  **moderate**.
- **4b, VMM cage: weak today** (uid 0, rw over every tenant's disks, host
  netns with `CAP_NET_ADMIN`). With `VMCTL_JAILER=on`: **strong**.
- **End to end, guest to another tenant's key: moderate today.** The
  target is **strong**: a patched kernel, the jailer on, and PCI off or
  1.17.

### The side door: the CPU (between tenants, beside rung 4)

This is not a rung but a way around one. A side channel reads another
tenant's secrets without crossing any software boundary.

**Node B today (live, doc).**

- Xeon E-2236, Coffee Lake U0 (CPUID 906EA), microcode 0xfa, SMT on.
- `l1tf` and `mds` report "SMT vulnerable".
- Core scheduling is unused.
- The VMMs are not opted in to IBPB or SSBD. The jailer launch thread now
  sets them, which takes effect when the jailer is on.
- `vmscape` reports "IBPB before exit to userspace".

**Evidence.**

- **VMScape** (ETH Zurich, CVE-2025-40300, disclosed 2025-09-11, IEEE
  S&P '26) names both of Choir's CPUs. The test machines included a
  Coffee Lake Core i7-8700 on microcode 0xfa (the same silicon as Node B)
  and a Zen 1 Ryzen 1600X.
  - Guest-to-own-VMM on the same thread worked on all Zen generations
    and Coffee Lake.
  - **Guest-to-VMM and guest-to-guest across SMT siblings worked only on
    Zen 1 and Coffee Lake** (https://comsec-files.ethz.ch/papers/vmscape_sp26.pdf).
  - The kernel's mitigation (IBPB on exit, live on Node B) covers the
    same-thread case. Its documentation says "STIBP is required for full
    protection" with SMT on
    (https://docs.kernel.org/admin-guide/hw-vuln/vmscape.html).
- **L1TF and MDS** leak across hyperthreads on this CPU. The kernel
  documents that full protection needs SMT off
  (https://docs.kernel.org/admin-guide/hw-vuln/l1tf.html,
  https://docs.kernel.org/admin-guide/hw-vuln/mds.html).
- **Microcode end of life.** Intel's servicing for CPUID 906EA ended on
  **2025-06-30**, and 0xfa (microcode-20250211) is its last microcode
  (https://www.intel.com/content/www/us/en/support/topics/support-and-servicing-for-processors.html).
  Any future Intel microcode-only fix will not reach Node B.
- **Not affected:**
  - Branch Privilege Injection (INTEL-SA-01247, 2025-05-13) lists 906ED,
    not 906EA.
  - Indirect Target Selection affects only eIBRS parts.
  - BHI is flagged only on eIBRS parts.
- **The planned EPYC 7371 (Zen 1).**
  - Not affected by Zenbleed (Zen 2) or TSA (family 19h).
  - Exposed to Retbleed, SRSO, VMScape across SMT, cross-thread return
    prediction (`kvm.mitigate_smt_rsb`), the Zen 1 DIV0 bug and FP-DSS
    (AMD-SB-7053, 2026-04-17).
  - The EntrySign microcode signature flaw (CVE-2024-36347) is fixed only
    through a BIOS update. Naples microcode in linux-firmware has been
    frozen since 2024-11-21
    (https://gitlab.com/kernel-firmware/linux-firmware/-/raw/main/amd-ucode/README).
- **Firecracker's own host guide** says: "Our recommendation is to
  disable SMT in production scenarios that require tenant separation"
  (https://github.com/firecracker-microvm/firecracker/blob/main/docs/prod-host-setup.md).

**Grade: weak between tenants while SMT is on.** No bug in Choir is
needed, and the attacks are public. It is irrelevant while every computer
on the host is the owner's.

- Core scheduling with one cookie per tenant, or `nosmt`: **moderate**.
  Spectre-class residue remains, and Node B gets no new microcode.
- Strangers on Node B's E-2236 should be treated as a stopgap at best.

### Rung 5: the host (NixOS, services, loopback)

**Mechanism today (code, live per earlier documents).**

- **systemd hardening landed on `main` (fda386b47).** The
  `@privileged` re-allow is gone, capability sets are emptied where
  unused, secrets are taken out of each unit's mount view, and the proxy
  no longer has the JWT key directory.
- **Host-internal service authentication (`internal/hostauth`, 44656ef90)
  is in observe mode** (`internalAuthMode = "observe"`,
  `nix/node-b.nix:67`). It logs "would refuse" and admits. Loopback is
  therefore still the credential for:
  - vmctl: stop, remove or relay any computer;
  - gateway credential minting;
  - every corpusd internal route: event heads, control key, escrow,
    self-development modes;
  - maild owner impersonation.
- Dolt accepts `root` with no password on loopback (doc:
  loopback-is-a-credential).
- Who sits in that network namespace: SearXNG, which parses untrusted
  web pages; goproxy, which fetches untrusted module archives; Qdrant;
  Caddy; and the VMMs.
- **Host kernel 6.18.16** (live, 2026-10-14). `/dev/kvm` is 0666.
- **CI deploys as root over SSH** (F13).

**Known escape classes, with dated evidence.**

- *Host-local privilege escalation through kernel bugs.* Every kernel
  privilege escalation listed under rung 2 also applies to host services,
  against a kernel older than most fixes:
  - CVE-2026-53362, fixed 6.18.38: in KEV.
  - Copy Fail: in KEV. The 6.18-stable fix version is **UNVERIFIED**.
    Upstream says 4.14 to 6.19.12 are affected.
  - Vghost (CVE-2026-74580): local root through `/dev/vhost-net` or
    `/dev/vhost-vsock`, fixed 6.18.45
    (https://thehackernews.com/2026/10/ai-found-linux-kernel-flaw-gives-local.html,
    2026-10-14). Whether those device nodes are openable by Node B's
    DynamicUser services is **UNVERIFIED**.
  - VsockDrop (CVE-2026-53365, 2026-08-18): local root over the vsock
    loopback.
- *Unauthenticated control planes.*
  - IngressNightmare (CVE-2025-1974, 2025-03-25) is the industry example.
  - Choir's own guest-arming hole (doc, 2026-10-15) is the local one: a
    signing oracle reachable from every guest.
- *Supply chain into a privileged runner.* Root CI deploy keys are the
  standing case.

**Keys it may hold, and does.**

- The auth JWT signing key. It mints any user's session, including the
  owner's.
- The platform-control key (corpusd). It signs mode receipts and
  checkpoints.
- The platform signing key.
- The escrow key. It unwraps every computer's escrowed data key.
- The escrow operator tokens.
- The hostauth credential.
- Provider credentials.
- Dolt write access to Store A (route and head authority) and Store B.

**Grade: weak today.**

- One code-execution bug in SearXNG or goproxy is enough for platform
  control while hostauth only observes. A host-local kernel privilege
  escalation is unusually cheap on 6.18.16.
- With hostauth enforcing, Dolt authenticating, services on their own
  users and a current kernel: **moderate**.
- **Strong** needs the platform keys out of general-purpose service
  processes: a signer process or an HSM/KMS behind a policy, with the
  JWT key used only by auth. That is the same oracle-with-policy shape
  proposed for desk keys.

### Rung 6: the owner's passkey in a secure enclave

**Mechanism today (code).**

- Passkey registration requires a platform authenticator with resident
  keys and requests the PRF extension (`internal/auth/handlers.go:540-547`).
- The PRF output derives the key that wraps the owner ROOT key
  (`internal/keyescrow/keyescrow.go:226-261`, "Track K 1b").
- Login uses `BeginLogin` with PRF evaluation (`handlers.go:640-663`).
  The relying-party config sets no user-verification requirement for
  login (`cmd/auth/main.go:33-37`). Whether the server therefore accepts
  an assertion without the UV flag is **UNVERIFIED**: it depends on the
  go-webauthn default and should be tested.
- **Login produces a host-minted session.** An Ed25519 JWT with a 5-minute
  access TTL and a 720-hour refresh TTL (`internal/auth/config.go:72-76`),
  signed by the rung-5 JWT key.
- **Owner acts ride on that session, not on a fresh passkey assertion.**
  These include self-development approval, restore and mode set, which
  are CLI-only since 5b17bb572.
- **Recovery** by email magic link registers a *new* passkey
  (`internal/auth/recovery.go:18-27`).

**Known attack classes, with dated evidence.** The cryptography holds. The
2024–2026 attacks go around it.

- **Pass-ta-key** (Unit 42, 2026-08-03). Malware with Chrome and Google
  Password Manager gets logins signed, registers its own verification
  key, and steals the sync key
  (https://bleepingcomputer.com/news/security/new-pass-ta-key-attacks-let-malware-hijack-google-synced-passkeys).
  The site-side defense is to require **and check** user verification.
- **SquareX "Passkeys Pwned"** (DEF CON 33, 2025-08-10). A malicious
  extension or XSS hooks the WebAuthn API.
- **Phishing kits that downgrade FIDO to phishable MFA** (Proofpoint,
  August 2025).
- **EUCLEAK** (2024-09-03). Physical extraction from Infineon-based
  YubiKeys below firmware 5.7.
- **No 2024–2026 software extraction from the Apple Secure Enclave was
  found** (UNVERIFIED as an absence).
- **What-you-see-is-what-you-sign.** Bybit, 2025-02-21, about $1.5B: the
  keys never left the hardware wallets, but the signers were shown a
  benign transaction and signed a malicious one
  (https://www.certik.com/resources/blog/bybit-incident-technical-analysis).
  An enclave stops copying, not authorized misuse.

**Keys it may hold.** The owner's WebAuthn credential, and through PRF the
wrap of the owner ROOT key. Under the identity design it should also be
the only thing that can authorize owner acts.

**Grade.**

- **The key: very strong.**
- **Owner authority: weak to moderate,** because it is the weakest of:
  - the host, whose JWT key mints the owner's session (rung 5, weak
    today);
  - the owner's email account, through recovery;
  - the owner's browser and its extensions;
  - a 720-hour refresh window.
- Binding owner acts to a fresh passkey assertion over the act's hash
  (transaction binding) is what lets the enclave's strength reach them.
  With a server-rendered summary of what is being approved, it closes the
  Bybit gap too.

## The ladder at a glance

| rung | boundary | keys it holds (proposed) | grade today | grade with landed or ready work | next move |
|---|---|---|---|---|---|
| 1 | desk cell (Yaegi worker) | desk key, via an outside signer only | weak | moderate (desk-cell branch) | land `fix/desk-cell-confinement`; per-desk signers outside the worker |
| 2 | engineering capsule | none long-lived | moderate | moderate | `clone3` ENOSYS, deny `CLONE_NEWUSER`; current guest kernel; gVisor comparison |
| 3 | guest root against guest signers | computer key, verifier key, gateway bearer, event capability, privacy key | label | label (S1 remainder) | non-root runtime; signer statement policy |
| 4a | KVM and Firecracker device model | none | moderate (stale kernel); strong when patched | strong | host kernel to at least 6.18.45; FC 1.17.0; reconsider `--enable-pci` |
| 4b | VMM process cage | none (holds its guest's RAM) | weak | strong (`VMCTL_JAILER=on`) | jailer rollout; per-VM netns |
| side | CPU between tenants | n/a | weak (SMT on) | moderate (core scheduling or `nosmt`) | policy before strangers; E-2236 is end of life |
| 5 | host | JWT, platform-control, signing, escrow keys; provider credentials | weak | moderate (hostauth enforce, Dolt passwords) | enforce; kernel cadence; platform keys behind a signer |
| 6 | owner passkey | owner credential, ROOT key wrap | very strong (key); weak to moderate (authority) | same | passkey-bound owner acts; require UV; harden recovery |

## Red teaming as capture the flag

The security review §8 and the isolation runtimes §7 already define the
LT station's rules of engagement and experiments E1 to E8. This section
recasts them as flags, so that every result is a signature, a grid cell
and a line in the disclosure.

### Flags

A flag is a signature over a challenge, `CTF/<campaign>/<flag>/<nonce>`,
by a key the attacker's starting position must not reach. A scorer outside
the target holds only public keys and checks each submission.

- On the LT host, every real key is replaced by a **canary key** of the
  same type, in the same place, held by the same process, with the same
  policy. No production key is ever a flag.
- Signatures produced through an oracle count. So do accepted
  authorizations: a gateway call admitted under another computer's
  bearer, or a corpusd write admitted as another principal.

| flag | key or authority | rung it sits behind |
|---|---|---|
| D-own | the attacker's own desk key, outside its declared statement types | 1 (oracle policy) |
| D-other | another desk's key, e.g. a research cell signing as management | 1 |
| C-sign | the computer's guest-core key, or a signature through the signer socket | 3 |
| C-verify | the verifier signer's key | 3 |
| C-bearer | the gateway bearer used for a call the gateway accepts | 3 |
| T-other | another computer's guest-core key, or any write to its disks | 4 |
| H-net | a firewall or route change on the host (`CAP_NET_ADMIN` effect) | 4b and 5 |
| P-control | a corpusd platform-control signature over a statement the attacker chose, e.g. a mode receipt for a computer it does not own | 5 |
| P-session | a JWT accepted by the proxy for a user the attacker is not | 5 |
| P-escrow | an unwrapped escrowed data key of another computer | 5 |
| O-act | an owner act (self-dev approve, restore, mode set) accepted without the owner's fresh passkey assertion | 6 |
| X-leak | any byte of a co-tenant's canary secret read through a side channel | side door |

The guest-arming hole is already a captured flag: P-control from start
S3, found by an in-process probe on 2026-10-14. It should be the first
entry in the grid.

### Starting positions

- **S0, outside.** Untrusted web content that research reads, a hostile
  RSS feed, a crafted PDF. The attacker controls only input.
- **S1, model-written cell code** in each desk: management, Texture,
  research. The attacker is the model, or a prompt injection steering it.
- **S2, capsule code.** Both an L0 capsule (no network) and an L2
  capsule (egress proxy).
- **S3, guest root.** The malicious-tenant starting point, free by
  design.
- **S4, assumed VMM breach.** Arbitrary code under a Firecracker
  process's identity, namespaces, cgroup and seccomp. This is E1. It
  needs no exploit because it is assumed.
- **S5, a host-local unprivileged service.** Code execution as SearXNG's
  or goproxy's DynamicUser.
- **S6, the owner's browser with a hostile extension.**

Each start runs against two target shapes, as the review requires: **T1**,
today's deployed shape, and **T2**, the hardened shape (desk-cell branch,
jailer on, hostauth enforce, current kernel).

### Scoring

The grid has starting positions as rows, flags as columns, and T1 and T2
as layers. Each cell is one of:

- **captured**, with method, date, commit and evidence ref;
- **held**, with what was tried and for how long;
- **not attempted**.

Points reward distance and value, and penalize what the defender missed:

- **Distance:** a flag scores the number of rungs between its start and
  its rung, so S1 to T-other scores far more than S3 to C-sign.
- **Value weight:** C-sign is 1, T-other is 3, P-control, P-session and
  P-escrow are 5, O-act is 5.
- **Key extraction** (the key bytes, not just one signature) doubles the
  score. So does persistence across a restart.
- **Public-exploit captures** (an adapted known CVE) are marked
  separately from new bugs. The first measures patch cadence; the second
  measures design.
- **Detection:** the defender loses points for every capture its own
  ledger, alerts or `scripts/verify-host-hardening.py` did not see.

Attackers include Choir's own desks under a red-team assignment, external
frontier models (the Trail of Bits and OpenAI results above show models
now run these chains), and humans. Rules follow the review §8:

- disposable LT hosts of the same CPU family only;
- never Node B or production keys;
- every capture becomes a problem document before any fix (CLAUDE.md:
  "Problem Documentation First").

### How results feed the disclosure

- **A rung's published grade is capped by the grid.** If a flag behind a
  rung was captured from just inside it on T1, the rung cannot be
  published above "weak" for the shape that lost it.
- **Each capture is published with a date.** Each fix is published with
  the commit that took the cell from captured to held.
- **The hill-climb metric** is the cheapest captured path to each flag
  class, in rungs crossed and public-versus-new bugs, tracked per
  campaign.
- **"Cannot be strongly secured" is a disclosure outcome, not a failure.**
  Two cells are expected to stay captured on shared hardware and are
  disclosed that way:
  - X-leak with SMT on;
  - O-act from S6 for any design where the browser shows what is signed.

## Draft public disclosure: what Choir's boundaries can and cannot protect

> Every Choir computer is a Firecracker microVM on a shared Linux host.
> Inside it, AI desks run code they write, and that code is the least
> trusted thing in the system. We put it in the weakest boundaries and
> keep keys out of them. Desk code runs in a Go interpreter inside a
> locked-down process. The interpreter is not a security boundary; the
> process jail around it is. Engineering's builds run in a
> container-style sandbox that shares the computer's Linux kernel. Kernel
> bugs that break such sandboxes are published several times a year, so
> we treat everything inside a computer as one trust zone: whatever can
> run code in your computer may be able to use your computer's keys. The
> microVM between your computer and everyone else's is a strong
> boundary, with no public escape from Firecracker to date. But it rests
> on the host kernel's KVM, where a guest-to-host escape against a
> Firecracker-based product was confirmed on 3 October 2026 and is not
> yet public. It is only as strong as our patch cadence. On today's
> hardware, computers that share a CPU core can leak secrets to each
> other through speculative-execution side channels. We do not place
> computers of different people on sibling hyperthreads [target; not yet
> true]. Choir's platform keys live on the host, which is today the
> weakest place we keep anything valuable; hardening it is our current
> priority. Your passkey never leaves your device's secure hardware, but
> it protects what you approve only for acts that ask for it at the
> moment of approval [target; owner acts use a session today]. We test
> these claims as a capture-the-flag exercise, with canary keys placed
> where real keys live, and publish which flags were captured, from
> where, and when.

The two bracketed clauses describe targets, not current fact. They must be
removed, or the targets met, before this is published.

## Prioritized hardening moves

Ordered by grade moved per unit of work, highest first.

1. **Host kernel current, and a patch cadence** (red; host reboot).
   - Move Node B from 6.18.16 to the current 6.18 stable, at least 6.18.45,
     and confirm the guest kernel is at least 6.18.45 too.
   - Make it a rule: kernels within 7 days of any CISA KEV kernel entry
     or KVM escape fix, otherwise monthly.
   - Reboot under the planned-restart marker, so busy computers are not
     crash-closed (CLAUDE.md, "Restarts End Work").
   - In the same change, set `kvm_intel nested=0` and `/dev/kvm` to
     `0660 root:kvm`. Together these close the "host-local process builds
     its own nested VM" route to Januscape-class bugs.
   - Watch the Vercel write-up and patch the day it lands.
   - Moves 4a and 5.
2. **Land `fix/desk-cell-confinement`** (red; guest image and runtime
   spawn).
   - Do the staging Landlock proof the branch owes.
   - Moves rung 1 from weak to moderate.
3. **Turn the jailer on** (red; vmctl).
   - Follow the staged rollout already written: one test computer, then
     everyone, with `scripts/vmm_confinement_verify.py` as acceptance.
   - Then slice 2 (per-VM network namespace) and removal of the direct
     path.
   - Moves 4b from weak to strong.
4. **hostauth from observe to enforce, and Dolt passwords** (red;
   auth and deploy).
   - Moves rung 5 from weak toward moderate.
   - Pair it with the guest-arming fix (`fix/selfdev-mode-owner-only`),
     which closes the only flag already captured.
5. **Firecracker 1.15.1 to 1.17.0** (red; snapshot invalidation per
   density lever L20). 1.15 is out of support.
   - In the same change, decide whether `--enable-pci` is needed. If
     nothing Choir uses requires PCI, the MMIO transport removes the
     youngest guest-facing code.
6. **A CPU policy before any stranger shares Node B** (red; host).
   - Either `mitigations=auto,nosmt`, or core scheduling with one cookie
     per tenant, plus `kvm-intel.vmentry_l1d_flush=always`.
   - VMM speculation opt-in through the jailer launch thread.
   - KSM off.
   - Record that the E-2236's microcode is final.
   - For the EPYC: a current BIOS (EntrySign), SRSO and
     cross-thread return-prediction mitigations, and the same SMT policy.
7. **Keys behind policy signers, not in general processes** (red;
   identity design).
   - Per-desk signers outside cell workers.
   - Guest signers that refuse statement types the runtime may not
     request.
   - Platform-control and escrow keys moved out of corpusd into a signer
     with a statement policy.
   - The JWT key used only by auth.
   - This turns every rung's oracle from "signs anything" into "signs
     what this principal may say".
8. **Owner acts bound to the passkey** (red; auth).
   - A fresh WebAuthn assertion with user verification required and
     checked, over the act's hash, for self-development approval,
     restore, mode set and fleet push.
   - A server-rendered summary of the act at signing time.
   - Recovery that needs the old passkey or a waiting period, with
     notice to the old devices.
9. **Capsule filter tightening** (orange; capsule).
   - `clone3` returns `ENOSYS`; `clone` refuses `CLONE_NEWUSER`.
   - Drop `vmsplice` and `memfd_create` unless a build needs them.
   - Run the gVisor capsule comparison (review §4.9) as an LT experiment.
10. **Finish S1 in the guest** (red; guest image).
    - A non-root runtime.
    - `pdftotext` and Obscura in a capsule-lite.
    - Guest sysctls: `unprivileged_bpf_disabled`, `yama.ptrace_scope`,
      `kptr_restrict`.
    - Turns rung 3 from a label into a boundary.

## Residuals and unverified claims

- **The Vercel "KVM 0day"** (2026-10-03): which component is at fault,
  the affected versions, whether nested virtualization or particular
  devices are needed, and the bounty amount are all **UNVERIFIED** until
  the write-up appears.
- **The host kernel version** comes from the security review's live
  reading on 2026-10-14. The guest kernel version comes from evidence
  dated 2026-10-01 and a design doc. Both should be re-read live before
  acting.
- **Copy Fail's 6.18-stable fix version** is **UNVERIFIED**. So is
  whether Node B's DynamicUser services can open `/dev/vhost-*`, which
  Vghost needs.
- **Some identifiers came only from aggregator databases:**
  CVE-2026-72183, CVE-2026-89603 and CVE-2026-76825. CVSS scores for
  several items are also from secondary sources. Confirm in NVD before
  external publication.
- **The passkey login requirement:** whether login accepts an assertion
  without the UV flag (library default) is **UNVERIFIED**. It is a
  one-request test.
- **Inferences to test first on LT:**
  - guest root reaching the guest-core signer socket or key (rung 3);
  - a direct VMM's view of `/run/credentials/go-choir-vmctl.service/`;
  - whether a uid-0 VMM with `CAP_NET_ADMIN` passes the kernel's ptrace
    capability check against root host services whose capability sets
    are now empty, and so opens their `/proc/<pid>/root` or `mem`. This
    depends on Yama and dumpability. If it succeeds, emptying the
    services' capabilities has made them *easier* to reach from a VMM.
  - nested user-namespace creation from a capsule reaching nf_tables.
- **Not covered:** Caddy and the public edge, the frontend, CI runner
  internals, physical access to the hosting provider, and the macOS
  desktop runtime.

## Sources

Repository (at 795e085bb unless noted):

- `internal/yaegikernel/{allowlist.go,eval.go,desk_worker_spawn_linux.go}`;
  `fix/desk-cell-confinement:internal/yaegikernel/desk_cell_confinement_linux.go`
- `internal/capsule/{executor.go,seccomp.go,landlock.go}`;
  `cmd/capsule-broker/main.go`
- `internal/vmmanager/{config.go,confinement.go,manager.go,snapshot.go}`
- `nix/node-b.nix`, `nix/autoputer-vm.nix`, `nix/hardware.nix`
- `internal/auth/{handlers.go,recovery.go,config.go}`;
  `cmd/auth/main.go`; `internal/keyescrow/keyescrow.go`
- Docs: the security review, isolation runtimes, the four problem records
  linked at the top, `docs/reports/owner-notes-2026-10-14.md` §17 and
  §18, `docs/archive/reports/computer-cpu-efficiency-2026-10-14.md`, and
  `docs/archive/evidence/s0a-guest-layout-2026-10-01.json`

MicroVM, VMM and KVM:

- Vercel and Yibelo claim, 2026-10-03:
  https://x.com/PaulosYibelo/status/2106378929158135903
- The Register, 2026-10-06:
  https://www.theregister.com/offbeat/2026/10/06/security-researcher-claims-they-found-kvm-guest-host-escape-flaw/5301267
- Cyber Security News, 2026-10-04: https://cybersecuritynews.com/kvm-zero-day-vm-escape/
- Techzine, 2026-10-06:
  https://www.techzine.eu/news/security/144780/potential-vm-escape-puts-kvm-security-under-the-microscope/
- webhosting.today, 2026-10-08:
  https://webhosting.today/2026/10/08/vercel-paid-50000-for-a-kvm-escape-that-has-no-cve-and-no-patch-every-vps-host-is-waiting-for-the-write-up/
- SecurityWeek on the Vercel $1M challenge, 2026-09-15:
  https://www.securityweek.com/1-million-sandbox-challenge-uncovers-linux-kernel-flaws/
- Vghost, 2026-10-14:
  https://thehackernews.com/2026/10/ai-found-linux-kernel-flaw-gives-local.html
- Firecracker advisories:
  https://github.com/firecracker-microvm/firecracker/security/advisories/GHSA-776c-mpj7-jm3r
  (2026-04-07) and
  https://github.com/firecracker-microvm/firecracker/security/advisories/GHSA-36j2-f825-qvgc
  (2026-01-23)
- Firecracker releases (1.17.0 and 1.16.2, 2026-09-10):
  https://github.com/firecracker-microvm/firecracker/releases
- Firecracker host setup guide:
  https://github.com/firecracker-microvm/firecracker/blob/main/docs/prod-host-setup.md
- Trail of Bits, 2026-08-26:
  https://blog.trailofbits.com/2026/08/26/vms-wont-contain-cyber-capable-agents/
- Januscape, 2026-07-06:
  https://thehackernews.com/2026/07/16-year-old-linux-kvm-flaw-lets-guest.html
- ITScape: https://github.com/V4bel/ITScape
- QEMU escapes: https://osec.io/blog/2026-03-17-virtio-snd-qemu-hypervisor-escape/ (2026-03-17);
  https://calif.io/research/qemu (2026-04-28)
- Cloud Hypervisor advisories:
  https://github.com/cloud-hypervisor/cloud-hypervisor/security/advisories
- Kata advisories:
  https://github.com/kata-containers/kata-containers/security/advisories;
  https://nvd.nist.gov/vuln/detail/CVE-2026-47243
- gVisor: https://app.opencve.io/cve/CVE-2026-96812 (2026-09-25);
  https://app.opencve.io/cve/CVE-2026-95702 (2026-10-09)

Containers, kernel, Landlock and seccomp:

- runc, 2025-11-05: https://www.openwall.com/lists/oss-security/2025/11/05/3
- CVE-2026-53362:
  https://corgea.com/research/cve-2026-53362-linux-udpv6-fraggap-root-container-escape;
  https://letsdatascience.com/news/cisa-lists-linux-flaw-exploited-by-openai-agents-77aad60a
- Copy Fail: https://unit42.paloaltonetworks.com/cve-2026-31431-copy-fail/
- CVE-2026-23111: https://techearl.com/cve-2026-23111-nftables-lpe
- Dirty Frag: https://borecraft.com/news/dirty-frag-cve-2026-43284-guide.html
- KEV additions, 2026-09-18:
  https://thehackernews.com/2026/09/cisa-flags-three-linux-kernel.html
- Landlock and seccomp kernel docs:
  https://docs.kernel.org/userspace-api/landlock.html;
  https://docs.kernel.org/userspace-api/seccomp_filter.html
- CVE-2026-72183:
  https://docs.devguard.org/vulnerability-database/DEBIAN-CVE-2026-72183/
- CVE-2026-89603: https://security-tracker.debian.org/tracker/CVE-2026-89603
- io_uring "Curing":
  https://www.armosec.io/blog/io_uring-rootkit-bypasses-linux-security

Interpreters and agent sandboxes:

- Yaegi: https://github.com/traefik/yaegi
- Go memory model: https://go.dev/ref/mem
- vm2: https://github.com/patriksimek/vm2/security/advisories
- isolated-vm:
  https://thehackernews.com/2026/08/isolated-vm-flaw-lets-sandboxed.html
- RestrictedPython:
  https://advisories.gitlab.com/pypi/restrictedpython/CVE-2026-76825/
- smolagents:
  https://research.jfrog.com/vulnerabilities/smolagents-local-python-sandbox-escape-jfsa-2025-001434277/
- RediShell: https://www.wiz.io/vulnerability-database/cve/cve-2025-49844
- Cymulate: https://cymulate.com/blog/the-race-to-ship-ai-tools-left-security-behind/
- Pillar "week of sandbox escapes":
  https://www.bleepingcomputer.com/news/security/cursor-codex-gemini-cli-antigravity-hit-by-sandbox-escapes/

CPU:

- VMScape: https://comsec-files.ethz.ch/papers/vmscape_sp26.pdf;
  https://docs.kernel.org/admin-guide/hw-vuln/vmscape.html
- L1TF and MDS: https://docs.kernel.org/admin-guide/hw-vuln/l1tf.html;
  https://docs.kernel.org/admin-guide/hw-vuln/mds.html
- Attack-vector controls:
  https://docs.kernel.org/admin-guide/hw-vuln/attack_vector_controls.html
- Intel servicing:
  https://www.intel.com/content/www/us/en/support/topics/support-and-servicing-for-processors.html
- Intel microcode release notes:
  https://github.com/intel/Intel-Linux-Processor-Microcode-Data-Files/blob/main/releasenote.md
- INTEL-SA-01247:
  https://www.intel.com/content/www/us/en/security-center/advisory/intel-sa-01247.html
- SRSO: https://docs.kernel.org/admin-guide/hw-vuln/srso.html
- Cross-thread RSB:
  https://docs.kernel.org/admin-guide/hw-vuln/cross-thread-rsb.html
- EntrySign: https://ubuntu.com/security/vulnerabilities/entrysign
- linux-firmware AMD microcode README:
  https://gitlab.com/kernel-firmware/linux-firmware/-/raw/main/amd-ucode/README

Passkeys:

- Pass-ta-key, 2026-08-03:
  https://bleepingcomputer.com/news/security/new-pass-ta-key-attacks-let-malware-hijack-google-synced-passkeys
- Passkeys Pwned:
  https://infocondb.org/con/def-con/def-con-33/passkeys-pwned-turning-webauthn-against-itself
- FIDO downgrade:
  https://www.darkreading.com/cybersecurity-operations/downgrade-attack-phishing-kits-bypass-fido
- EUCLEAK: https://ninjalab.io/eucleak/
- Bybit: https://www.certik.com/resources/blog/bybit-incident-technical-analysis
