# MA6.4: every desk worker under gVisor (2026-11-06)

Class red, tier 2. Rollback: revert to the process-only profile, by an owner
act only (it lowers isolation). Plan row: docs/design/modular-architecture-plan-2026-11-04.md,
MA6.4; rulings R39, R40. Phase 0 evidence: docs/evidence/gvisor-desk-capsule-x86-2026-11-04.md,
docs/evidence/gvisor-arm64-mac-guest-2026-11-04.md.

## Design

- **runsc outer.** `internal/yaegikernel/desk_gvisor*.go`. One runsc container
  per trust class key (`desk_trust_class.go`): `a` first-party custodians
  (management, its tasks, Texture, first-party app desks such as Mail), `b`
  first-party desks with taint `untrusted_input` (research and its children),
  `c-<slug>-<hash>` one per non-first-party app desk. Flags on every command:
  `--platform=systrap --network=none --ignore-cgroups --overlay2=none
  --gvisor-marker-file` (`--rootless` only for a non-root spawner: dev and
  tests).
- **Sandbox.** `runsc run` in the foreground with a parent-death signal,
  started through `childthread.Start` (the signal fires when the forking
  thread ends, so the fork runs on a thread held for the child's life). The
  signal ends the launcher with the daemon, not the sandbox: its workers
  exit at their session socket's EOF, and the next daemon's sweep ends the
  sentry (`runsc delete --force`, then `cgroup.kill` on its `s-*` cgroup)
  and removes the state root (`/run/choir-desk-gvisor`), its specs and every
  `s-*` cgroup at first use after start (and `EnsureDeskCgroupRoot` sweeps
  `s-*` beside `w-*`). Nothing resumes across a restart. A sandbox
  starts outside the pool lock: concurrent spawns of its class wait on its
  start, other classes do not. The container's init is the worker binary
  in its pause role (`CHOIR_DESK_SANDBOX_ROLE=init`, uid 65534, no
  capabilities). The sandbox stops when its last worker is reaped.
- **Filesystem view (replaces Landlock and the MA6.2 mount namespace).** An
  empty read-only root; the worker binary's ELF closure bound read-only at
  the same paths (whole Nix store directories, found by walking PT_INTERP and
  DT_NEEDED with each object's RUNPATH); the time zone database read-only at
  `/zoneinfo` (`ZONEINFO`); an empty read-only `/work` (the cell's allowed
  root: desk cells read files only through broker verbs); a memory `/tmp`;
  an empty read-only `/sys`; gVisor's own `/proc` and `/dev`. No host path is
  writable from inside: each worker makes its own 0700 tmp dir in the
  sandbox's `/tmp` (random name; it refuses a dir that already exists).
- **Worker.** `runsc exec --process <spec> --pass-fd 3:3 --internal-pid-file
  <f> <id>`. The fd 3 socketpair and the slot token cross unchanged (the
  token rides the process spec's environment, a 0600 file in a root-only
  directory, never argv). The worker runs as its activation's own uid (the
  MA6.2 pool) with no capabilities and no_new_privs; inside it checks the
  gVisor marker and its uid, then loads the rlimits and the same cell seccomp
  filter. The host process the spawner holds is runsc's exec client; a kill
  ends the worker inside the sandbox first (`runsc kill --pid`).
- **Memory (review r1, calibrated r2).** A worker in a shared sandbox has
  no cgroup of its own; its runaway stop is RLIMIT_AS = its **heap share** +
  the runtime's reservations (1440 MiB amd64, 1408 MiB arm64; gVisor
  enforces RLIMIT_AS, not RLIMIT_DATA), with GOMEMLIMIT at 3/4 of the share.
  The process spec sets RLIMIT_STACK 2 MiB and the environment
  MALLOC_ARENA_MAX=1: thread stacks and glibc's 64 MiB malloc arenas are
  address space, and without them what a worker could hold at a given limit
  swung with its thread count (72 to 288 MiB at one limit). The memory bound
  proper is the admission ledger and the sandbox cgroups. Every live
  sandbox claims 96 MiB (sentry and gofer, about 40 MiB measured, plus its
  `/tmp`, a tmpfs capped at 32 MiB); every live worker claims its share + a
  quarter + 64 MiB (one heap arena: at a given limit a worker holds up to
  about one arena past its share) + 16 MiB margin + 32 MiB (an idle worker,
  26 to 29 MiB measured). The share is the desk's budget, at most what lets
  six workers in three sandboxes fit the desks cgroup, at least 96 MiB.
  **Admission** keeps the sum of claims within the desks cgroup's
  memory.max. A spawn that does not fit now waits (review r3) in one queue,
  class a (management, Texture) first and then by arrival, strict
  head-of-line, and is granted at a reap. While a spawn heads the queue and
  its deadline is live, the least recently used idle desk session (no cell
  running) is ended to make room, and again each time a session goes idle
  (a cell's end pokes the queue; r4), so a waiter gets idle memory before
  its deadline rather than at retention. A waiter behind the head, or past
  its deadline, ends nothing. The session is detached under its pool's
  lock and closed outside it. The wait is bounded by the caller's own context (the cell's or the
  research child's deadline; none means two minutes); when it expires the
  spawn is refused by name: `desks memory committed: a <role> worker waited
  D for N MiB beside M MiB claimed of the desks cgroup's T MiB (K others
  waiting)`. A worker whose claim plus a sandbox can never fit the desks
  cgroup is refused at once (`desks memory too small`). Neither refusal
  falls back to process confinement. A class whose sandbox fails to start
  refuses its queued waiters with that reason at the next grant, rather
  than each retrying a broken runsc (they then run under process
  confinement, first-party only). The wait is in the worker's receipt
  (`queued_ms` in the self-test and OOM-leg workers, both bounded by the
  handler's deadline, and in the desk cell event beside `confinement`,
  gvisor or process) and the pool's status (`committed_bytes`, `waiting`).
  **Claims are returned only with memory (r4).** A reaped worker whose end
  inside the sandbox is not confirmed keeps its claim, like its uid, until
  the sandbox stops; the last worker's sandbox overhead and every held
  claim go back to the waiters only after the sandbox's stop returns.
  **Fallback workers are in the ledger (r4).** A first-party desk under
  process confinement (gVisor unusable for its class) waits in the same
  queue for a claim of what a sandboxed worker of the desk would claim, at
  most its manifest budget, and its own cgroup's memory.max is capped
  there, so the claim bounds what it holds; it is returned at its reap.
  **No memory.high on the desks cgroup (r4).** Admission fills to
  memory.max, so the old memory.high (three eighths of the guest, 75% of
  memory.max) would throttle admitted workers before their own limits bind.
  Admitting against memory.high instead was the other option; it would cut
  a 2 GiB guest to two concurrent desk workers (96 + 2 x 232 + 96 + 232 =
  888 MiB exceeds 768). The soft bound memory.high served (keep the desks
  below a total) is what admission now does per worker, without
  throttling, so `autoputer` sets no memory.high and `EnsureDeskCgroupRoot`
  writes `max` to clear one an earlier daemon set. memory.max (half the
  guest) still bounds the desks against the daemon. A sandbox runs in its own child of the desks cgroup
  (`s-<key>-<rand>`: oom.group, no swap, pids.max 4096) whose memory.max is
  its claim: 96 MiB plus its live workers' claims. It rises before a worker
  is admitted (a failed write refuses the worker) and is lowered at each
  reap to the live claims, never below memory.current + 32 MiB (memory the
  sentry has not returned would meet the limit as an OOM of the class). A
  reaped worker's tmp dir is removed inside the sandbox as its uid. So the
  worker's own limit binds before its sandbox's cgroup and the desks cgroup
  on every guest size, and a runaway ends its own worker ("fatal error: out
  of memory") while its class-mates keep serving.
- **Required mode = gVisor-confined.** Where gVisor is unavailable (no runsc,
  cached for 2 minutes; or a class whose sandbox will not start, that class
  alone for 2 minutes), first-party desks run under the MA6.2 process
  confinement, logged at each spawn, and every other desk is refused. A correctness refusal, not a switch: there is no setting that turns
  gVisor off. `CHOIR_RUNSC` names the binary (the guest image sets the pinned
  store path); an unset or wrong path makes gVisor unavailable (a set `CHOIR_RUNSC` is
  the only candidate: no fall-back to the profile or `PATH`), which the
  isolation self-test fails in required mode.
- **Image.** `nix/gvisor.nix`: the release-20261102.0 tarball (the one Phase 0
  measured), x86_64 and aarch64, fixed-output with the published sha512;
  `runsc` and `gvisor-bin/` in `bin/`. `nix/autoputer-vm.nix` sets
  `CHOIR_RUNSC` for the guest runtime.
- **Self-test.** `POST /api/runtime/isolation-selftest` now carries
  `gvisor` (availability, runsc version, every live sandbox with its class,
  workers, host processes, RSS and PSS, cgroup memory.max/current) and each
  `desk_cells` worker's `profile`, `sandbox_key`, `sandbox_id`,
  `gvisor_marker` and `cell_p50_us`. A gVisor worker's status is read inside
  its sandbox by the binary's inspect role (the worker cannot forge it); its
  report and the cross-desk canary go through the same role as the worker's
  uid. The judge (Go and `scripts/desk_cell_confinement_probe.py`) requires
  gVisor in required mode, one sandbox per class (management and Texture
  share `a`, research alone in `b`), resident memory read back, and the escape
  probes. The OOM leg judges a sandboxed worker by its address-space end.

## Measured (2026-11-06)

agents VM (x86_64, Xeon E-2236, rootless runsc, non-root spawner), 300 warm
cells each, research worker:

| Cell | native p50 | gVisor p50 | ratio |
|---|---|---|---|
| trivial | 222 us | 451 us | 2.03x |
| verb round trip | 364 us | 777 us | 2.14x |
| allocation loop | 5.61 ms | 5.48 ms | 0.98x |
| fib(25), 30 cells | 283 ms | 311 ms | 1.10x |

Spawn to ready: native 17 ms, gVisor 84 ms (first worker, sandbox start
included). Phase 0 measured 1.0 to 1.7 ms warm cell (1.7x) and 2.8x a verb on
the same host: the ratio holds.

Privileged Docker on the Mac (aarch64 linuxkit 6.10, root spawner, the
guest's shape): trivial 1.83x, verb 1.71x, allocation 0.80x, fib 0.88x;
required mode with a desks cgroup passes the self-test (before r1, grow-only
budgets: sandbox `a` memory.max 1,879,048,192, `b` 1,073,741,824; after r1
see the fit below).

Sandboxes read back (x86, rootless, after one cell each): class `a` with 2
workers PSS 163 MiB; `b` with 2 workers PSS 159 MiB; each `c` with 1 worker
PSS 104 to 115 MiB; one sandbox with 4 research workers PSS 323 MiB (aarch64
root). RSS sums run 3x PSS because systrap's stub processes share the
sentry's memory; PSS is the figure to budget.

Escape probes inside a sandbox: sockets, exec, fork, signals to others,
setuid, unshare, io_uring, keyctl, bpf, perf, userfaultfd, ptrace, mount and
namespace clones all EPERM from the cell filter; a world-readable host file,
/etc/passwd and the test's build dir are not in view (ENOENT); a class-mate's
canary in its 0700 tmp dir is EACCES; another class's is ENOENT; /work is
read-only; /sys/kernel and /dev/mem absent.

## Calibration (review r2)

Method (`TestGvisorAddressCalibration`, which re-runs it where runsc is): a
worker under the production spawn path (its share as RLIMIT_AS less the
overhead, GOMEMLIMIT, MALLOC_ARENA_MAX, RLIMIT_STACK) runs a cell that
holds live 8 MiB slabs, touching every page, and reports each 8 MiB
through a broker verb until its address limit ends it. "Held" is the last
report. With a desks cgroup the sandbox's memory.current is sampled every
millisecond: its rise tracks held within 2 to 6 MiB, so held is physical.
Agents VM (x86_64, 8 vCPU, rootless, delegated user cgroup) and privileged
Docker (aarch64, 8 CPU, root); Go 1.26, gVisor 20261102.0; 5 trials each.

| Arch | Share | AS limit | Held (min to max) | Min / share | Heap allowance | Claim |
|---|---|---|---|---|---|---|
| amd64 | 96 MiB | 1536 MiB | 120 to 176 MiB | 1.25 | 200 MiB | 232 MiB |
| amd64 | 124 MiB | 1564 MiB | 128 to 168 MiB | 1.03 | 235 MiB | 267 MiB |
| amd64 | 256 MiB | 1696 MiB | 248 to 296 MiB | 0.97 | 400 MiB | 432 MiB |
| amd64 | 512 MiB | 1952 MiB | 504 to 560 MiB | 0.98 | 720 MiB | 752 MiB |
| arm64 | 96 MiB | 1504 MiB | 136 to 184 MiB | 1.42 | 200 MiB | 232 MiB |
| arm64 | 124 MiB | 1532 MiB | 120 to 176 MiB | 0.97 | 235 MiB | 267 MiB |
| arm64 | 256 MiB | 1664 MiB | 280 to 352 MiB | 1.09 | 400 MiB | 432 MiB |
| arm64 | 512 MiB | 1920 MiB | 512 to 632 MiB | 1.00 | 720 MiB | 752 MiB |

The overhead is constant across shares (the Go heap grows in 64 MiB arenas,
so held moves in steps of about 64 MiB around share + constant); it is the
smallest that keeps the minimum held at or above 90% of the share at every
share on its architecture (amd64 at 1408 held 56 at share 96; arm64 at 1376
held 80). The test fails if the minimum falls below 90% of the share (a Go
or gVisor upgrade that shrinks every heap) or the maximum passes the heap
allowance (the claim no longer bounds a runaway).

Before r2, at the r1 constants and without the stack and arena limits, the
same worker held 32 MiB at 1660 MiB (amd64) and arm64 did not reliably
start below 1680 MiB: r1's shares were mostly unusable.

**Real cells at the 2 GiB share (96 MiB, AS 1536 / 1504 MiB)**, 3 workers
each, sandbox memory.current sampled: the benchmark cells (trivial, a verb
round trip, an allocation loop, fib(25)) and a research cell that reads a
capture of 1 or 3 MiB of JSON records, decodes it and builds word, bigram
and tag maps, then sorts the bigrams, all pass on both architectures. Peak
rise over the idle worker: fib(25) 4 to 8 MiB; the 1 MiB document 5 to 7
MiB; the 3 MiB document (near the 4 MiB frame limit) 13 to 17 MiB. A 96 MiB
share holds at least 120 MiB: the room for a cell is about 7 times the
largest real cell measured. Cell latency is unchanged by the stack and arena
limits (x86, research worker, 300 cells, gVisor/native p50: trivial 2.13x,
verb 2.42x, alloc 1.26x, fib(25) 1.09x; r1 2.03 / 2.14 / 0.98 / 1.10).

## Fit per guest size (review r2)

The desks cgroup is MemTotal/2 (`internal/autoputer/desk_cgroup.go`); the
interactive default guest is 2048 MiB (vmctl). From
`TestWorkerShareFitsEveryGuest`:

| Guest | Desks max | Share (budget 384 / 512 / 768 MiB) | Claim | Workers admitted beside 2 / 3 sandboxes |
|---|---|---|---|---|
| 2048 MiB | 1024 MiB | 96 / 96 / 96 MiB | 232 MiB | 3 / 3 (4 in one sandbox) |
| 3072 MiB | 1536 MiB | 96 / 96 / 96 MiB | 232 MiB | 5 / 5 |
| 4096 MiB | 2048 MiB | 145 / 145 / 145 MiB | 293 MiB | 6 / 6 |
| 8192 MiB | 4096 MiB | 384 / 418 / 418 MiB | 592 to 634 MiB | 6 / 6 |
| 16384 MiB | 8192 MiB | 384 / 512 / 768 MiB (the budget) | 592 to 1072 MiB | 13 / 10 / 7 |

**Share rule (r2, admission r3).** Small guests get the minimum share and
fewer concurrent workers, never smaller shares: on a 2 GiB guest three desk
workers run at once (management, Texture and research across classes a, b
and one c); a fourth waits for a reap (r3; r2 refused it), class a ahead of
research's children so a burst of children cannot starve management or
Texture, and idle sessions are ended to make room. Before MA6.4 a
2 GiB guest admitted any number of process-confined workers, each with a
768 MiB cgroup under the 1 GiB desks cgroup, so concurrent heavy cells met
the desks cgroup's OOM; in a shared sandbox that OOM would end the whole
class, which admission rules out. The alternative, budgeting each sandbox
for its idle workers plus one runaway at a time, admits more workers on a
2 GiB guest but lets two concurrent runaways in one class end it; it is not
taken.

**OOM leg on a 2 GiB-guest desks cgroup** (`TestGvisorOOMLegOn2GiBGuest`,
privileged Docker, aarch64, root, required mode; desks memory.max 1 GiB,
high 768 MiB): the research runaway's heap share 96 MiB, address limit
1504 MiB; its sandbox `s-b-*` memory.max 560 MiB (96 + 2 x 232); the
runaway ended on its own with the runtime's out of memory, its sibling
answered after, and oom_kill was 0 in the sandbox cgroup and in the desks
cgroup. `TestGvisorAdmissionRefusesBeyondDesksMemory`: four research
workers admitted in one sandbox (96 + 4 x 232 = 1024 MiB), the fifth
waited its 300 ms deadline and was refused by name; one reap lowered the sandbox memory.max from 1024 to 792 MiB and
admitted again; the last reap removed the cgroup and the claims returned to
0.

**Admission wait on a 2 GiB-guest desks cgroup (review r3)**
(`TestGvisorAdmissionWaitsOn2GiBGuest`, privileged Docker, aarch64, root,
required mode): management, Texture and research ran (888 MiB claimed);
research_web and research_texture, each with a 60 s deadline, both waited
(`waiting` 2); research's reap admitted one child, which ran its cell, and
its reap admitted the other; both reported `queued_ms` > 0, and management
and Texture served throughout. Unit tests without runsc
(`desk_gvisor_admission_linux_test.go`, also under -race): the wake order
(class a before earlier class b children), the refusal at the deadline, the
idle-session eviction, and the immediate refusal of a claim that can never
fit (desks memory.max 256 MiB). agentcore: the least recently used idle
session is the one ended, and concurrent first cells of one session share
one worker (the spawn runs outside the session lock).

## Residuals

1. Root mode was proved on aarch64 in a privileged container, not in the
   Firecracker x86 guest kernel: the deploy proof on d@e is the first run
   there. If runsc cannot start in the guest, first-party desks keep working
   under process confinement and the self-test fails, naming the reason.
2. `nix/gvisor.nix` was evaluated for both systems but not built here (no
   Linux builder reachable from the Mac; the agents VM has no nix). CI's
   image build is its first build.
3. Class C is keyed per app desk, not per publisher: the manifest row has no
   publisher field. Never shares across publishers; may split one
   publisher's desks.
4. The capability floor (`internal/capfloor`) is in observe; under enforce,
   runsc as root may need capabilities outside the floor. Check before
   enforce.
5. RSS per sandbox includes the sentry's retained heap (the arm64 325 MiB
   after heavy cells, MA0.8): memory.max is lowered at a reap only to
   memory.current + 32 MiB, so a sandbox that retained memory keeps a higher
   limit than its claim until it stops. The admission ledger counts claims,
   not that retention.
6. RLIMIT_AS is a calibrated, not exact, bound: what a worker holds at its
   limit varies by about one 64 MiB heap arena, so each claim carries that
   step. The calibration is per Go and gVisor version and per architecture;
   `TestGvisorAddressCalibration` re-measures it but runs only where runsc
   is (not in CI), so a Go or gVisor upgrade must run it (agents VM and a
   privileged arm64 container) before it ships. A tighter per-worker bound
   inside gVisor (sentry-side memory accounting per process) does not exist
   in 20261102.0.
7. Capsules are unchanged (MA6.7 pilots gVisor there).
8. Class `c` sandboxes stop with their last worker (no linger); a desk that
   spawns and reaps in a loop pays the sandbox start (30 to 73 ms) each time.
   Deferred (r1 item 6, optional).
9. A 2 GiB guest runs at most three gVisor desk workers at once (the r2
   share rule); research's children (research_web, research_texture) beside
   management, Texture and research wait there for a reap (r3), so they run
   serially and a child whose deadline passes first is refused. Head-of-line
   order means a large waiter at the head holds back smaller ones behind it
   (all claims are equal at one share, so this is moot today). A 3 GiB guest
   runs five; 4 GiB and up six or more.
10. Starvation by design (review r4, D8): class a always goes first, so a
    steady stream of management and Texture spawns can hold research and
    its children at the back until their deadlines pass; and head-of-line
    order holds every waiter behind a head that does not fit. Both are
    bounded by each waiter's own deadline (a named refusal, never a hang).
    No aging is added.
11. Exec-client memory is outside the ledger (review r4, D7): each sandboxed
    worker's host-side `runsc exec` client (a Go process, tens of MiB
    resident) lives in the daemon's cgroup, not the sandbox's, and is not
    in any claim. Moving it into the sandbox cgroup would count it against
    the class's memory.max, which the claims do not cover; it stays a
    measured residual for the d@e proof (host RSS of the clients).
12. Teardown on daemon death is not immediate (review r4, D6): the launcher
    dies with the daemon, the sandbox's sentry lives until the next daemon's
    sweep. Its workers exit at their socket's EOF, so it holds memory but
    serves nothing. Putting launchers and exec clients in a cgroup the
    daemon's unit kills would need a cgroup outside the desks tree; not
    done.

## What the d@e deploy proof must check

- `POST /api/runtime/isolation-selftest`: `ok`, `gvisor.available`, runsc
  `release-20261102.0`, every `desk_cells` worker `profile: gvisor` with the
  marker, management and Texture in one `a` sandbox, research in `b`,
  `rss_bytes`/`pss_bytes` per sandbox, cgroup `memory_max` set, `cell_p50_us`,
  `committed_bytes` and `waiting` (0 at rest), each worker's `queued_ms`.
- The desks cgroup reads `memory.high max`; desk cell events carry
  `confinement: gvisor`; the host RSS of the `runsc exec` clients is
  recorded (residual 11).
- `scripts/desk_cell_confinement_probe.py` (all legs), including
  `leg=oom`: the runaway ends at its address limit (heap share at most the
  budget), the sibling answers, oom_kill is 0 in the sandbox cgroup, the
  daemon is unchanged.
- On the d@e guest (x86, its vCPU count), `TestGvisorAddressCalibration`'s
  numbers are not re-measured by the deploy; the OOM leg's runaway ending
  at its limit with no cgroup kill is the in-guest check.
- A real desk run (management, research) still serves cells; the daemon's
  journal shows no `gVisor sandbox did not start`.
