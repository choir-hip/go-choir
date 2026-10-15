# Desk Cells Run as Guest Root, and Guest Egress Is Open

Date: 2026-10-14. Found by the computer isolation security review
([docs/reports/computer-isolation-security-review-2026-10-14.md](../reports/computer-isolation-security-review-2026-10-14.md),
findings F4 and F6). This record re-reads the code on `origin/main`
(980ffaf4e) and adds the guest egress inventory the fix needs. Status:
documented; repaired on branch `fix/desk-cell-confinement` (see
Repair), not deployed.
Mutation class of the fix: **red** (guest runtime child spawn boundary,
guest image users, host firewall for guest taps).

## Problem 1: three desks' cells have no OS boundary (F4)

Management, Texture and research run model-authored Go in a desk session
worker: the guest runtime re-executes its own binary as
`autoputer desk-session` (`internal/agentcore/tools_desk.go:34,751`,
`internal/autoputer/desk_session.go`). The spawn sets only `Setpgid` and
`Pdeathsig` (`internal/yaegikernel/desk_worker_spawn_linux.go:27-33`) and
an 8 GiB `RLIMIT_AS` (`desk_worker_memlimit_linux.go`). The runtime unit
has no `User=` (`nix/autoputer-vm.nix`, `go-choir-autoputer`), so every
worker is **guest root with full capabilities**: no uid change, no
`no_new_privs`, no seccomp, no Landlock, no namespaces.

The only thing between a cell and guest root is the Yaegi import
allowlist (`internal/yaegikernel/allowlist.go`) and the `choir` exports
(`choir.go`). Doctrine I18 says exactly the opposite: "Yaegi is not the
security boundary." Engineering's cells run in a capsule (user, mount,
net, pid namespaces; deny-by-default seccomp; mandatory Landlock V5;
cgroup; egress broker: `internal/capsule`), so this is three desks of
four.

What a Yaegi interpreter escape reaches today, as root: the gateway
token file (`/run/choir-bootstrap/gateway-token`), the privacy key
(`/mnt/persistent/choir-credentials/privacy-key`), the Dolt workspace,
the guest signer sockets, `ptrace` of the runtime, raw sockets, the
open guest egress below, and every host port the guest can reach.

### What the workers legitimately need (code reading)

- **Transport:** one inherited socketpair on fd 3. Every effect
  (emit, search, read, capture, product API, ledger acts) is a frame the
  host answers (`desk_session_worker.go` `serviceEmit`,
  `serviceEgress`; `sidecar.go` `ExecuteWorkerSessionConn`). The
  worker opens **no** sockets of its own.
- **Files:** `choir.ReadFile` / `choir.ListDir` read under
  `AllowedRoot` (`broker.go:269,333,456`), which is the run's
  `WorkingDir` or `os.TempDir()` (`tools_desk.go` `deskWorkerRoot`);
  in production nothing sets `tool_cwd`, so it is the guest's `/tmp`.
  `WriteFile` and `Exec` are exported only to engineering
  (`choir.go` `deskModuleSets`). Before serving, the worker reads
  `/proc/meminfo` for its Go memory limit (`gomemlimit`). Optional:
  `/etc/localtime` and `/etc/zoneinfo` for `time.LoadLocation`.
- **Processes:** none. No symbol for `os/exec`; threads only.
- **Network:** none.

## Problem 2: guest egress is open (F6)

`setupHostNetworking` (`internal/vmmanager/manager.go:3240-3323`) admits
the guest to six host service ports by INPUT ACCEPT plus DNAT to
127.0.0.1 (`tapReachableHostServicePorts`: 8082, 8083, 8084, 8086, 8087,
8088), then `FORWARD -i <tap> -j ACCEPT` and
`POSTROUTING -s <guest>/30 ! -d <guest>/30 -j MASQUERADE`: any
destination, any protocol. The host's NixOS firewall allows port 22 on
every interface, so guests reach host sshd (live, review §3.5); the
host's `nixos-fw` also admits 80/443. Guests get no routable IPv6, but
the taps carry IPv6 link-local and sshd listens on `::`.

### Guest egress inventory (code reading, this record)

| path | guest process | destination |
|---|---|---|
| LLM, judgments, search | runtime | host :8084 (gateway) |
| credentials, events, file sync, captures, keys, checkpoints | runtime | host :8086 (corpusd) |
| story publish | runtime | host :8082 (proxy) |
| computer-version routes (self-development) | runtime | host :8083 (vmctl) |
| capsule Go modules | runtime (capsule egress proxy) | host :8088 (goproxy) |
| maild | none found | host :8087 (admitted, unused) |
| research `choir.Read`, Autopaper `PollSource` (RSS, GDELT, Telegram), content import, podcast feeds | runtime, `sourcefetch` (address guard: no private, loopback, link-local, CGNAT, metadata) | **internet, direct, any public host and port** |
| capsule L2 egress (pip, npm, git, Hugging Face, arXiv) | runtime's `EgressProxy` | **internet**, CONNECT :443 / GET :80, host allowlist |
| YouTube transcripts, podcast search (itunes.apple.com) | runtime | **internet** :443 |
| Obscura web lens, browser follow fetch | Obscura / runtime | **internet**, no address guard |
| DNS | systemd-resolved | **public resolvers, UDP/TCP 53** (no DNS in `ip=`, no host resolver) |
| NTP | systemd-timesyncd | `*.nixos.pool.ntp.org`, UDP 123 |
| updater, signers | updater, signers | loopback and unix sockets only |

So the guest legitimately needs: the six (five used) host service
ports; the **public** internet on TCP for research, sources and
packages; DNS and NTP over UDP. It does not need: any other host port
(sshd 22, Dolt 13306/13307, caddy admin 2019, qdrant, searxng, the stray
:19090), any private, link-local, metadata or CGNAT range, other
guests, SMTP, arbitrary UDP, or IPv6.

A default-deny allowlist of *internet hosts* (the S1 acceptance item)
is not possible at the packet layer while research reads any public
URL directly from the guest; it needs the host-side recording proxy
(S4). Until then the packet layer can close everything above that is
not needed.

### Where the egress policy belongs

Guest-side rules (nftables in the guest image) bind only non-root guest
processes: a compromised guest root, the case F6 is about, flushes them.
The policy therefore goes on the host, keyed on the guest interfaces
(`vm-+`). The open branch `fix/tap-deny-guest-range` rewrites the
per-tap filter rules into `CHOIR-GUEST-FWD` / `CHOIR-GUEST-TAPS`
(forward default-deny over the whole allocator pool, INPUT source
checks, teardown and sweep). To compose with it without fighting over
filter-table rule order (both insert at position 1), the egress policy
lives in its own chain in the **mangle** table, which every guest
packet traverses before nat and filter, so it holds whatever order the
filter chains end up in.

## Failure modes the fix must survive (written before code)

Cells:
1. The cell user does not exist in the image (new runtime binary on an
   old image): the worker must not fall back to root. Refuse the spawn.
2. The runtime is root but the identity switch silently does not happen
   (attr dropped): the worker itself refuses to serve as uid 0 when the
   sandbox is required.
3. Landlock unsupported or disabled at boot: required mode fails
   closed (as capsules do); non-root dev/CI hosts run best-effort.
4. Seccomp list misses a syscall the Go runtime or glibc (the binary is
   cgo: Dolt/ICU) needs after confinement: the worker dies mid-cell.
   `clone3` must return `ENOSYS` (glibc falls back to `clone`), not
   `EPERM`. The filter must assemble on both amd64 (Node B) and arm64.
5. `Pdeathsig` is cleared by a credential change: it must be set after
   the uid switch (Go's `forkAndExecInChild` does this).
6. The cell user cannot exec the binary or enter the working directory:
   the worker's cwd becomes its own tmp dir; the Nix store is world
   readable.
7. Per-worker tmp dirs leak on crash: removed at reap; they live under
   the guest's `/tmp`.
8. Cells of one desk signalling or reading another desk's worker: one
   uid per desk, and seccomp admits `kill`/`tgkill` only on the worker's
   own pid; no `ptrace`, no `process_vm_readv`.
9. The session-config equality check (`sameDeskSessionConfig`) must not
   see a per-spawn value, or every cell respawns its worker.

Egress:
10. Replies to host-initiated flows (vmctl to guest :8085 via OUTPUT DNAT)
    enter from the tap with a random destination port: admit
    `ESTABLISHED,RELATED` first.
11. The guest's own host services arrive pre-DNAT with the tap IP as
    destination: match `addrtype LOCAL` plus the service port list, the
    same list `tapReachableHostServicePorts` uses (one source).
12. Rule order against the branch's filter chains: separate table.
13. Concurrent tap setup: one mutex; idempotent ensure; a changed policy
    is rebuilt, an equal one untouched.
14. vmctl restart with live guests: the policy is ensured on reattach too.
15. Research reading `choir.news` resolves to Node B's public address
    (LOCAL): admit TCP 80/443 to local addresses (the public front door).
16. `ip6tables` missing: log, do not fail VM boot.

## Repair (branch `fix/desk-cell-confinement`, not deployed)

**Cells.** `internal/yaegikernel/desk_cell_confinement*.go`:
- Spawner: a root runtime (always) or required mode runs the worker as
  `choir-cell-management` / `-texture` / `-research` (management tasks
  share management's, research children research's; an unknown role is
  refused), no supplementary groups, in new network and IPC namespaces,
  with a private 0700 tmp dir as cwd and `TMPDIR`, removed at reap. A
  missing user, an unknown role, or required mode on a non-root spawner
  refuses the spawn.
- Worker (`autoputer desk-session`, after memory limits, before the
  first cell): refuses uid 0 in required mode or when its uid differs
  from the planned one; Landlock (read on the allowed root if the cell
  user can open it, read-write on its tmp dir, `/dev/null`,
  `/dev/urandom`, time zones; no TCP bind/connect; V6 scoping best
  effort); a deny-by-default seccomp filter: no `socket`, `connect`,
  `bind`, `exec*`, `fork`, `ptrace`, `set*id`, `unshare`, `mount`,
  `bpf`, `memfd_create`; `clone` only with `CLONE_THREAD`; `clone3`
  answers `ENOSYS`; `kill`/`tgkill` only to its own pid. Names the
  architecture lacks are dropped, so it assembles on amd64 and arm64.
- Guest image: the three users and `CHOIR_DESK_CELL_CONFINEMENT=required`
  (mandatory Landlock V5, as capsules have).
- Non-root dev and CI spawners load no_new_privs and seccomp, Landlock
  best effort: every existing desk test now runs through the filter.

**Egress.** `internal/vmmanager/guest_egress.go`: mangle
`PREROUTING -i vm-+ -j CHOIR-GUEST-EGRESS`, ensured at vmctl start, on
every tap setup and every reattach; IPv6 from `vm-+` dropped. The chain: established
first; host addresses only on the tap service ports (one list with
`tapReachableHostServicePorts`) and 80/443; the ranges above dropped;
SMTP dropped; TCP, UDP 53/123 and ICMP returned; everything else
dropped.

**Evidence (local Linux, Docker Desktop kernel 6.10 arm64, no Landlock
in that kernel).**
- `internal/yaegikernel` `TestDeskCell*` as root with the cell users: worker
  Uid = choir-cell-research on all four ids, no groups, CapInh/Prm/Eff/Amb
  0, NoNewPrivs 1, Seccomp 2, own network namespace; plain, ReadFile,
  ListDir, 64 busy goroutines, time/json/rand/regexp cells and an egress
  frame all work; an escaped worker cannot write outside its tmp dir,
  read `/proc/1/environ`, open packet, raw, inet, inet6, unix or netlink
  sockets (EPERM), dial a host listener, fork or exec (EPERM), signal or
  `ptrace` its parent, `setresuid(0)` or `unshare(CLONE_NEWUSER)`; the tmp
  dir is 0700 cell-owned and gone after close. Required mode without
  Landlock refuses to start the worker. As non-root: NoNewPrivs 1,
  Seccomp 2, same probes refused. Mutation check: with the filter and
  netns removed the same tests fail (Seccomp 0, sockets open, dial
  succeeds, exec reaches ENOENT).
- `internal/agentcore` desk tests with the real `autoputer desk-session`
  binary (cgo, ICU), root and non-root: management, Texture, research
  (including research children, searches, deadlines, kill and respawn)
  pass; `TestDeskGoEvalWorkerIsConfined` reads the production worker's
  status.
- `internal/vmmanager` `TestGuestEgressPolicyOnRealNetfilter` (iptables
  1.8.11 nf_tables, netns guest and "world"): before the policy every
  probe connects; after it, gateway and corpusd on the tap peer, the
  host's 443, internet 443 and 8443, and DNS pass; host sshd, host Dolt,
  internet SMTP, a private address, 169.254.169.254 and arbitrary UDP
  are dropped; a host-initiated connection to the guest still gets its
  reply.

Staging proof still owed: Landlock V5 enforcement on the guest kernel
(the Docker kernel has none), and a deployed guest's desk worker status
plus a guest-side dial to host :22 and a private address.

**Overlap with `fix/tap-deny-guest-range`.** That fix landed on `main`
(efbd69e27) while this branch was in progress; the branch is rebased on
it. It moved the tap rules into `hostfirewall.go` (`hostNet`,
`CHOIR-GUEST-FWD`/`CHOIR-GUEST-TAPS`, filter table). The egress
installer hangs off `hostNet` (`egress`, nil in the fake-backed tests so
they never touch the host's netfilter) and is ensured in
`reconcileAtStart` (vmctl start, before any reattach), in
`setupHostNetworking` and on reattach. The two compose: mangle runs
first and only drops; the filter chains then decide forwarding between
guests and taps. Guest-to-guest is dropped by both (here as
`10.0.0.0/8`), on purpose. The review's other network item, inbound
forwarding to a tap only for established flows, is not in either change.

## Remaining after the fix (named residuals)

- Internet egress is still any public TCP destination: the recording
  proxy (S4) owns host allowlisting.
- The guest runtime itself is still root (S1 remainder), so pdftotext,
  Obscura and terminal children are not confined by this change.
- Obscura's navigate and the browser follow fetch have no address
  guard; the host policy now blocks the private and host targets they
  could reach, but not guest loopback.
- maild :8087 is admitted with no guest caller.
- Cell workers have no cgroup of their own (RLIMIT_AS bounds memory;
  threads are not counted per desk). The capsule's cgroup code could be
  reused.
- The default allowed root for desks without a working dir is the
  guest's `/tmp`; it should become the worker's own scratch dir.
- `ioctl` stays in the cell filter (the capsule list has it); with no
  device files reachable under Landlock its surface is sockets, pipes
  and regular files.
- `ip6tables` absent on the host logs a warning and leaves guest IPv6
  open (Node B has it).

## Landing (2026-10-15): rebase onto `origin/main` 133de45fa

Branch `land/desk-cell-confinement` is `fix/desk-cell-confinement`
rebased onto 133de45fa, which had moved 142 commits (capsule jobs, the
Claude delegate, the capsule broker re-dial, the Freeze changes, egress
for the claude network mode, hostauth, the auth boundary, VMM jailer
slice 1). Not pushed. Mutation class: **red**, as above.

### Failure modes of the rebase (written before resolving)

1. A textual conflict resolved by dropping one side: the egress ensure
   on reattach, or main's jailer change to console adoption.
2. A guest-to-host route added on `main` after the egress inventory
   above, whose port is not in `tapReachableHostServicePorts`, so the
   policy cuts it the moment vmctl restarts (every running guest at once:
   the mangle jump covers `vm-+`).
3. The claude network mode's model API (`api.anthropic.com:443`) or the
   capsule goproxy route (`goproxy.choir.internal` mapped to the gateway
   host on 8088) refused by the host policy.
4. Jailed taps (slice 1) set up on a path that skips
   `setupHostNetworking`, so a jailed guest has no egress policy; or the
   VMM no-IP rule and the egress chain fighting over a table.
5. A `main` change that makes desk session workers (not capsules) exec,
   open sockets or read outside their grants after confinement: the new
   `jobs.go`, `build_toolchain.go`, `inspect_bundle.go` and broker env in
   `internal/yaegikernel`.
6. A secret added to the guest after the branch that a cell could now
   read: the Claude delegate token on the credential disk.
7. `fix/tap-deny-guest-range` half-landed, leaving two filter-table
   schemes.
8. Tests that pass only as non-root (CI) while the guest runtime is root.

### What the rebase found

- `11d257d53` (the problem record) was dropped by git as already applied
  (`e2e595629` on `main`). One conflict, in `ReattachVMWithConfig`
  (`internal/vmmanager/manager.go`): `main` changed
  `adoptConsole(vmID, pid)` to `adoptConsole(vmID, pid,
  inst.confinement.jailed())` (jailer slice 1); the branch had added
  `m.ensureGuestEgressPolicy()` before it. Kept both: the ensure, then
  `main`'s call. (1)
- Guest-to-host routes, re-inventoried on 133de45fa (2): the guest's
  kernel command line names `choir.gateway_url` :8084,
  `choir.vmctl_url` :8083, `choir.maild_url` :8087,
  `choir.platform_url` :8086 (corpusd) and `choir.proxy_url` :8082
  (`buildFirecrackerConfig`); the capsule Go module proxy is :8088 on
  the gateway host (`capsule.GoModuleProxyAddr`). All six are in
  `tapReachableHostServicePorts`, the one list the chain uses. vmctl
  :8083 also carries `feat/memory-hotplug`'s guest memory route (tap
  source only), so that branch needs no policy change. hostauth and the
  auth-boundary fixes add credentials on these routes, not ports. New
  test `TestGuestEgressAdmitsEveryGuestToHostRoute` reads the ports from
  the kernel command line itself, so a future `choir.*_url` without its
  port fails CI instead of cutting guests off.
- The claude network mode (3): the capsule egress proxy runs in the guest
  runtime and dials `api.anthropic.com:443` directly (public TCP,
  returned by the chain); goproxy is :8088 on the host address
  (returned). DNS goes to public resolvers (systemd-resolved fallback;
  `ip=` names none), UDP/TCP 53, returned.
- Jailer slice 1 (4): `prepareVMNetwork` creates jailed taps as `vm-*`
  in the host namespace through the same `setupHostNetworking`, which
  ensures the policy. The no-IP rule is filter `OUTPUT` (VMM uids'
  own sockets); the egress chain is mangle `PREROUTING` on `vm-+`. They
  do not touch.
- Desk workers (5): capsule jobs (`choir.Start/Poll/Stop`), the build
  toolchain env and `Exec` are engineering's, which runs in a capsule;
  the desk module sets for management, Texture and research gained none
  of them. The whole `internal/yaegikernel` suite passes with every
  desk worker spawned confined as root (below).
- The Claude delegate token (6) is on the credential disk, mounted at
  `/run/choir-bootstrap` (`CHOIR_CLAUDE_DELEGATE_CREDENTIAL_DIR`). The
  self-test below probes that mount and the gateway token file from a
  confined cell.
- `fix/tap-deny-guest-range` (7): both of its commits are on `main`
  (`git cherry` marks 816077e83 and 8653a48a2 as applied, as efbd69e27
  and f38752ab6). Nothing of that branch is left to merge; the
  composition described above holds.
- Running as root (8): the existing yaegikernel desk tests spawn the
  `go test` binary from its 0700 build dir, which a cell user cannot
  exec. Compiled with `go test -c` into a 0755 dir, the whole suite
  passes as root with every worker confined.

### Staging proof (owed; a script now exists)

Neither Landlock nor the host policy can be proven off staging, and a
guest is never reached by SSH. The branch adds an owner-only guest route,
`POST /api/runtime/isolation-selftest` (not on the ProductAPI allowlist,
so no desk can call it):

- desk cells: one worker per desk through the production spawner and
  entrypoint, which confines itself and then acts as an escaped cell
  (sockets of seven kinds, fork, exec, signals, set*id, unshare, writes
  outside its tmp dir, and reads of `/etc/passwd`, the owner's files,
  the store, the credential disk and a world-readable canary in another
  desk's live tmp dir), then runs one plain cell. The runtime reads each
  worker's `/proc` status and namespaces from outside;
- egress: the guest runtime itself dials host :22, :13306, :2019,
  169.254.169.254:80 and another guest's address (must be dropped) and
  the gateway, corpusd and 1.1.1.1:443 (must stay open).

`scripts/desk_cell_confinement_probe.py` calls it and judges the raw facts
itself. Against a real receipt from a kernel without Landlock it fails on
exactly the Landlock-only checks (mode, ABI, `/etc/passwd`, a write to the
allowed root), so it can pass only where Landlock is enforced.

Plan, after the Landing Loop deploys this branch:

1. CI green; `/health` `deployed_commit` is the pushed SHA. vmctl's
   restart installs the chain for every tap at once. Read it:
   `ssh node-b sudo iptables -t mangle -S CHOIR-GUEST-EGRESS`, and
   `sudo iptables -t mangle -S PREROUTING` / `sudo ip6tables -t mangle -S
   PREROUTING` each show one `go-choir-guest-egress` jump first.
2. Cold boot a disposable test computer onto the new guest image (an
   idle computer; deploys do not restart busy ones), then:
   `scripts/desk_cell_confinement_probe.py --cred <its 0600 credential>
   --out docs/evidence/desk-cell-confinement-staging-<date>.json`.
   Pass: exit 0 (mode `required`, Landlock ABI >= 5, three distinct cell
   uids, CapEff 0, NoNewPrivs 1, Seccomp 2, own net and IPC namespaces,
   every escape probe EPERM or EACCES, the cross-desk canary EACCES,
   the check cell runs; host :22 and the metadata address dropped, the
   gateway and the internet open).
3. Desks still work confined, on the product path:
   `cd frontend && BASE_URL=https://choir.news npx playwright test
   tests/texture-acceptance-deployed.spec.js` (management, research and
   Texture cells all run in confined workers).
4. Rollback: `git revert` the branch commits and deploy. Live host,
   before that: `iptables -t mangle -D PREROUTING -i vm-+ -m comment
   --comment go-choir-guest-egress -j CHOIR-GUEST-EGRESS`; `iptables -t
   mangle -F CHOIR-GUEST-EGRESS`; `iptables -t mangle -X
   CHOIR-GUEST-EGRESS`; `ip6tables -t mangle -D PREROUTING -i vm-+ -m
   comment --comment go-choir-guest-egress -j DROP`.

Residuals added by the landing: the cell's capability bounding set is
not dropped (CapBnd full, CapEff/CapPrm 0 under no_new_privs; nothing
can raise them, but dropping it is cheap defense in depth); workers that
exit while their session object is never closed stay zombies until the
session is reaped (seen in tests, not yet checked in the guest); UDP 443
(QUIC) is dropped, which nothing in the guest uses today.
