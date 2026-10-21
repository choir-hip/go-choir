---
definition_version: 4

# Station FP2 of the launch first pass (batch B3, red): the guest desk
# boundary. Run as `/goal docs/definitions/choir-launch-fp2-desk-boundary-2026-10-21.md`.
# Spine: docs/definitions/choir-launch-first-pass-metamission-2026-10-21.md.
# Items: SDC ARCH-02 (the desk manifest, launch item L1-04), ARCH-05, ARCH-07,
# SEC-01, SEC-02, SEC-13, SEC-15, RES-01, RES-02, RES-03, RES-04
# (docs/security-density-architecture-checklist-2026-10-21.md).
# Owner 2026-10-21 (owner notes sections 36 and 37): every security and density
# item that can land before launch does; desk-cell confinement lands before
# launch; app agents get their own desks.
readiness: drafted

review:
  reviewer: none yet; a panel on the frozen diff runs before this lands (FP0)
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-21T18:30:00Z
  source:
    canonical_ref: origin/main f399a2e8c (lane branch lane/b3-desk-boundary starts here)
    deploy_identity: >-
      staging served f2c4a3ec3 when the metamission was written; c893a956a was
      deploying. Nothing of this batch is deployed.
  worktrees:
    - path: scratchpad/lanes/b3-desk-boundary (branch lane/b3-desk-boundary)
      status: clean
      class: goal_candidate
      owner: FP2 lane worker
      touch: goal_owned
      recovery: branch; FP0 rebases and lands
    - path: /Users/wiz/go-choir and every other worktree and stash
      status: unknown
      class: other_agent_wip
      owner: not this lane
      touch: forbidden
      recovery: leave in place
    - path: origin/land/desk-cell-confinement (9cee80a2d) and origin/fix/desk-cell-confinement
      status: clean
      class: other_agent_wip
      owner: owner's iMac session, recovered 2026-10-21 (REC-01)
      touch: read_only
      recovery: this lane ports from it with `git show`; it never edits it
  observed:
    - >-
      Desk workers run as guest root with `Setpgid` and `Pdeathsig` only
      (internal/yaegikernel/desk_worker_spawn_linux.go); the guest runtime unit
      has no `User=` (nix/autoputer-vm.nix, go-choir-autoputer).
    - >-
      `serviceEgress` and `serviceEmit` in internal/yaegikernel/desk_session_worker.go
      decode a worker's frame and call the host handler without reading
      `HandleRef`, `Epoch`, or the sending desk. `deskEgress`
      (internal/agentcore/tools_desk.go) checks the role only for System1 and Stories.
    - >-
      Handle scopes are built in `NewChoirScope` (internal/yaegikernel/choir.go):
      Texture and management carry `exec` and `write_file`; research and
      management carry `emit`, which no verb dispatches (`choir.Emit` is retired).
    - >-
      The same desk facts are written six times: agentprofile.PolicyFor (message
      targets), capsule.RoleVerbSets (broker verbs), NewChoirScope (handle
      actions), deskModuleSets (verbs), recordsRoute and recordsDesk (inbox
      routing), deskCarrierLive (carrier), and DeskCellUser on the unlanded branch.
    - >-
      The frame header is 8 bytes with three reserved bytes and no version
      (internal/yaegikernel/transport.go). Worker memory is RLIMIT_AS 8 GiB plus a
      soft GOMEMLIMIT; the unit has OOMPolicy=continue and no memory settings.
    - >-
      Capsule seccomp allows clone3 and every clone flag (internal/capsule/seccomp.go:28);
      the exec-go-session worker has no memory bound (cmd/capsule-broker/main.go:153-162);
      capsule admission reserves memory.max and no capsule sets memory.high
      (internal/capsule/job_memory_linux.go:37, namespace.go:38).
    - >-
      Desk-cell confinement exists, tested, on origin/land/desk-cell-confinement
      (831 commits behind main). Its host egress half (internal/vmmanager) is SEC-03,
      lane FP3, and is not ported here.

finish:
  deliver: >-
    A desk that is compromised, forged or runaway cannot take more than its
    manifest gives it: it cannot name another desk's authority, cannot reach
    outside its confined worker, and cannot exhaust the guest's memory, while
    the real desk turns (management, Texture, research, engineering) work as
    before.
  artifact: >-
    1. internal/deskmanifest, the one table every desk check reads, with parity
    tests that pin it to the authority the code granted before. 2. The daemon's
    per-frame role check and the versioned, slot-bound frame protocol. 3. Desk
    cell confinement ported from the recovered branch, its profile read from the
    manifest. 4. In-guest cgroup tree and a hard per-worker memory budget, the
    capsule worker bound, capsule memory per doctrine. 5. Capsule seccomp with
    clone3 answering ENOSYS and no new user namespace. 6. Guest sysctls and a
    sandboxed document-tool child. 7. A guest isolation self-test and a staging
    probe script that read the facts from outside the cell.
  acceptance:
    - action: >-
        go test -run 'TestDeskManifest|TestFrameRole|TestForgedFrame|TestFrameVersion|TestSlotToken'
        on internal/deskmanifest and internal/yaegikernel; go test -run 'TestDeskEgressRole|TestDeskRefusal'
        on internal/agentcore
      proves: >-
        a frame naming an action outside the sending desk's table, carrying another
        desk's identity, a wrong slot token, an unknown version, or an emit, is
        refused with a visible error and a trace event, and the handler is never called
      evidence_class: local test
    - action: >-
        the Linux confinement and cgroup tests (go test -c for linux, run in a
        privileged container): TestDeskCell*, TestDeskCgroup*, TestCapsuleSeccomp*
      proves: >-
        an escaped worker cannot open sockets, fork, exec, signal, trace or write
        outside its own directory; a worker over its budget is killed by the kernel
        and not the daemon; clone3 answers ENOSYS and CLONE_NEWUSER is refused
      evidence_class: local Linux test (Docker kernel, no Landlock); Landlock and cgroup delegation are staging proof
    - action: >-
        POST /api/runtime/isolation-selftest on a disposable test computer, then
        scripts/desk_cell_confinement_probe.py (exact steps in the FP2 report)
      proves: >-
        on the deployed guest kernel: Landlock V5 enforced, three distinct cell
        uids, CapEff 0, NoNewPrivs 1, Seccomp 2, own network namespace, every
        escape probe refused, the cgroup budget present and an out-of-memory probe
        killing the worker while the daemon lives
      evidence_class: deployed proof
    - action: texture-acceptance-deployed and a research desk turn on the same computer
      proves: real desk turns still work confined
      evidence_class: deployed proof
  rollback: >-
    One rollback unit: git revert of the lane's commits and the Landing Loop.
    Faster, on a live guest: the runtime reads CHOIR_DESK_CELL_CONFINEMENT only
    at spawn and the unit file is in the image, so a bad image is reverted by
    the deploy, not by an edit on the guest. There is no opt-out flag by
    design (a typo strengthens the policy).
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize what a compromised, forged or runaway desk can do beyond its
    manifest (authority named, confinement enforced, memory bounded), while
    preserving every desk's real turn and the paper's availability.
  goodharting_would_be: >-
    A manifest nobody reads; a role check that trusts a role carried in the
    frame; a refusal only the model sees and the trace does not; confinement
    that passes on a kernel without Landlock; an out-of-memory test that kills
    a test process but not the daemon-versus-worker choice; ticking an item on
    merged code with no probe from outside the cell.

homotopy:
  realism_axis: >-
    Pure table tests (manifest parity), then a real socketpair and worker
    process with a forged frame on a Linux kernel in a container, then the
    deployed guest kernel with Landlock V5 and cgroup delegation, then a day of
    the paper's real desk load on it.

boundaries:
  mutation_class: red (guest desk confinement, the frame protocol, run authority); green for the docs commits
  authority_sources:
    - CLAUDE.md "Desk Ontology" and docs/agent-product-doctrine.md "Authority Boundaries" (Texture sole writer of canonical revisions; management admits engineering through delegated casts; engineering capsule-bound mutation; research read-only world evidence)
    - docs/security-density-architecture-checklist-2026-10-21.md items named above, and owner notes sections 36 and 37
    - docs/problems/desk-frame-actions-not-role-checked-by-host-2026-10-21.md and docs/problems/desk-cells-run-as-guest-root-and-egress-is-open-2026-10-14.md
  must_preserve:
    - No desk's authority widens. The manifest reproduces the old tables and every difference narrows (exec, write_file and emit leave the Texture, management and research handles; emit leaves all frame tables). A parity test fails if a change widens a desk.
    - A desk or role the manifest does not know fails closed everywhere (frames, handles, cell identity, sandbox profile).
    - The daemon decides a frame's authority from the worker it spawned (role and slot token the host holds), never from a field in the frame.
    - No opt-out flag for confinement; no unconfined fallback when a cell user, Landlock V5 (required mode) or the cgroup budget (required mode) is missing: the spawn is refused and the refusal reaches the desk and the trace.
    - Restarts end work (CLAUDE.md): the cgroup tree and cell directories are made at spawn, removed at reap, and swept at boot only by name pattern under the desk cgroup, never by scanning the object store; no boot-time resumer.
    - The Landing Loop runs for this batch; it shares no deploy with another red batch.
  excluded:
    - SEC-03 host egress and the vmmanager half of the recovered branch (lane FP3).
    - Making the guest daemon itself non-root (SEC-15's largest part): named residual with a design below.
    - Obscura (lane FP6 deletes it); the host-side jailer (SEC-04).
    - ARCH-03 (app package carries its desk), ARCH-04, ARCH-06, ARCH-08, ARCH-09, ARCH-10. The manifest gives them their seam; it does not build them.
  protected_surfaces: [guest desk confinement, desk worker spawn, run authority at the daemon, gateway-bound egress verbs the frames reach, the guest image (users, units, sysctls), capsule seccomp]

now:
  status: working
  slice: station goal file and problem record (first commit), then the manifest, the frame boundary, confinement, memory, seccomp
  source_ref: f399a2e8c
  deploy_identity: none of this batch is deployed
  candidate:
    id: none
    state: none
    ref: lane/b3-desk-boundary
    base: f399a2e8c
    digest: none
    scope: []
  conjecture:
    id: C-FP2-one-table
    claim: >-
      If confinement, frame role checks, handle scopes, capsule verbs and
      memory budgets read one desk manifest, then adding a desk (an app desk)
      cannot add authority by omission, and a reviewer can read a desk's whole
      authority in one place.
    test: >-
      Parity tests that fail when a desk's authority differs from the pinned
      table; a test that every consumer rejects a desk with no entry; a diff
      review that no other file spells a desk's frame actions.
    edge: missing_oracle
    delta_o: >-
      The guest self-test reading the worker's facts from /proc outside the
      cell, and a deployed forged-frame probe (a test-only worker is not
      possible in production; the probe is the isolation self-test's escaped-cell
      worker sending raw frames).
    scope_if_supported: the five system desks and app desks that register through the manifest
    status: active
    evidence_refs: []
  decision:
    what: >-
      Frame role table lives in the manifest; the check sits in the yaegikernel
      transport (serviceEgress, serviceEmit), the one place every host-served
      action passes, with a second check in agentcore deskEgress for defense in
      depth. Memory budget: cgroup v2 per worker under a desks slice, required in
      the guest image, best effort elsewhere.
    kind: architecture
    status: proposal
    evidence_ref: this file; panel on the frozen diff
    owner_ratification_ref: owner notes sections 36 and 37 ratify the goal (every item lands before launch), not these choices
  belief:
    believed_state: >-
      The recovered confinement branch ports with conflicts in five files; the
      role table is derivable from NewChoirScope and the verbs; emit has no
      caller; the Landlock and cgroup behavior cannot be proved on this Mac.
    main_uncertainty: >-
      Whether the guest's systemd lets the runtime unit own a delegated cgroup
      subtree beside the existing /sys/fs/cgroup/capsule tree (the capsule code
      writes a root-level cgroup directly).
    next_observation: >-
      The staging probe reading /sys/fs/cgroup layout and memory.events from the
      self-test route.
  blocker_or_risk: >-
    A required-mode guest image that cannot confine would stop every desk on the
    refreshed computers, the paper included: the first deploy must reach a test
    computer and pass the probe before the paper refreshes (FP0's call).
  next_action: commit this file, then the failing manifest and frame tests.

receipts: []
---

# FP2: the guest desk boundary

## The problem in one paragraph

A desk is a worker process that runs model-written Go. What that worker may do
is decided in six places that nobody reads together, and the daemon that
answers its frames does not check which desk sent them. The confinement that
would make the worker a real boundary exists on a branch 831 commits behind.
The memory limit is a virtual address cap in an 8 GiB guest. Records:
[desk-frame-actions-not-role-checked-by-host](../problems/desk-frame-actions-not-role-checked-by-host-2026-10-21.md),
[desk-cells-run-as-guest-root-and-egress-is-open](../problems/desk-cells-run-as-guest-root-and-egress-is-open-2026-10-14.md),
[desk-cell-init-only-for-loop-never-ends-guest-oom](../problems/desk-cell-init-only-for-loop-never-ends-guest-oom-2026-10-16.md),
and the new
[desk-boundary-residuals-2026-10-21](../problems/desk-boundary-residuals-2026-10-21.md)
(RES-03, RES-04, SEC-13, SEC-15, ARCH-05, ARCH-07, each with a code reference
verified on f399a2e8c).

## The manifest (ARCH-02)

`internal/deskmanifest` imports only the standard library, so the daemon
(`agentcore`), the kernel (`yaegikernel`), the capsule (`capsule`) and
`agentprofile` can all read it. One entry per desk, keyed by the strings the
code already uses (`management`, `texture`, `research`, `engineering`,
`email`, `management_task`, `research_child`; app desks `app:<name>`).

| field | meaning | read by (this batch) |
|---|---|---|
| `Carrier` | desk_worker, capsule, or none | spawn (profile selection) |
| `Verbs` | the choir verbs the desk's cells may call (module set) | `yaegikernel.deskModuleSets` |
| `LocalActions` | actions its worker serves itself (`read_file`, `list_dir`; `exec`, `write_file` only where the manifest says) | `NewChoirScope` handle scopes |
| `FrameActions` | actions the daemon will serve from this desk's worker (the role table, SEC-02) | `serviceEgress`, `serviceEmit`, `deskEgress` |
| `CapsuleVerbs` | broker verbs for a desk that runs in a capsule | `capsule.RoleVerbSets` |
| `MessageTargets` | desks it may message | `agentprofile.PolicyFor` |
| `Addresses` | inbox routes it may use | parity test against `recordsRoute` (read, not yet replaced) |
| `Sandbox` | cell user, namespaces, filesystem reads, spawn and network allowed | `planCellConfinement`, the Landlock and seccomp builders |
| `Taint` | `untrusted_input` or `privileged` (ARCH-05) | worker pool key, cell user |
| `Memory` | `HardLimitBytes` per worker (RES-02), aggregate class (RES-01), capsule request and ceiling (RES-04) | cgroup builder, capsule admission |
| `OwnedKinds`, `Model`, `Restart` | object kinds it may write, model policy role, restart contract | declared and parity-pinned only: no consumer yet (named residual) |

Rules the package enforces, each with a test: `Lookup` of an unknown name
returns not-found and every consumer treats that as deny; only engineering holds
`exec` and `write_file`; no desk holds `emit` or `message` frame actions; an
app desk is constructed through `NewAppDesk`, which refuses `exec`,
`write_file`, `product_api`, any system desk's name, a missing memory class, and
widens nothing a system desk lacks; entries are immutable after init.

## Frame boundary (SEC-02, ARCH-07)

1. The daemon identifies a worker by the process it spawned: its `Role`, its
   `Epoch`, and a 32-byte slot token it generated at spawn (env, never argv).
   Nothing in a frame names the sender.
2. Header byte 5 is the wire version (1). A frame with another version, or
   non-zero reserved bytes, ends the connection before its payload is parsed.
3. Every worker-to-daemon request carries the slot token in `BrokerRequest`;
   the ready handshake carries it too. A wrong or missing token is a forged
   frame: refused, recorded, and the worker is killed.
4. The daemon checks `FrameActions[role]` for every `StreamBrokerEgress` and
   `StreamBrokerEmit` frame before any handler runs, and checks `req.Epoch`.
   A refusal returns `refused: desk <role> may not call <action>` on the result
   stream (the cell reads it as a choir error) and is reported through
   `DeskSessionWorkerConfig.Refused`, which agentcore records as a
   `desk.egress` event with `refused: true`.
5. Honest limit: the token proves which connection a frame arrived on and makes
   the identity portable to vsock or TLS later. A worker that is fully
   compromised can read its own token. The role table, not the token, is what
   bounds it.

## Failure modes, written before code

Manifest: M1 a desk present in one consumer and absent in another (parity
tests over every consumer); M2 an unknown desk falls open; M3 a manifest edit
widens a desk silently (the parity table is the reviewed artifact); M4 an app
desk claims exec, write_file, product_api, a system name; M5 an import cycle.
Frames: F1 a frame names an action outside the role; F2 a payload carries a
role or desk name; F3 wrong, missing or replayed slot token; F4 unknown
version or reserved bytes; F5 emit from any desk; F6 a research child served
research's wider set; F7 an empty or unknown role serving frames; F8 a real
verb refused because the table forgot an action it dispatches (a test derives
the dispatched actions from the module sets and requires each to be in the
table); F9 a legitimate test harness frame without a token (the harness sets
one; production never omits it).
Confinement: the nine cell and sixteen egress failure modes in the 2026-10-14
record apply to the cell half; this lane adds C1 manifest and cell-user drift
(a desk with a sandbox profile and no user in the image), C2 a cell user that
shares a uid with another desk, C3 the new `jobs`, `build_toolchain` and
`inspect_bundle` code in yaegikernel becoming reachable by a confined worker
(engineering only, in a capsule).
Memory: R1 the cgroup is unavailable (non-Linux, CI, no delegation): required
mode refuses the spawn, otherwise the worker runs with RLIMIT_AS and
GOMEMLIMIT and the event says `budget: rlimit_only`; R2 a worker killed by the
budget is reported as such to the desk (cause: memory budget), not as a bare
EOF; R3 a cgroup directory leaks after a crash (reap removes it, boot removes
any `desk-*` leftover under the desks slice, one directory listing); R4 the
budget is smaller than a legitimate research cell's working set (the value is
set from measured RSS, see Memory below); R5 capsule `memory.high` throttles a
Go build that previously ran to its 2 GiB ceiling.

## Memory

The values come from code and the record, and are marked where they are
guesses. Guest 8 GiB (`VM_INTERACTIVE_MEM_MIB=8192`). Daemon slice
`memory.min` 512 MiB (a guess, to be set from L1-02's measurement). Desks slice
`memory.high` 3 GiB and `memory.max` 4 GiB. A worker: `memory.max` 768 MiB for
management, Texture, research; 256 MiB for a management task and a research
child (guesses; the first staging probe reads each worker's `memory.peak` and
FP0 adjusts before the paper refreshes). Capsules slice: aggregate
`memory.max` 4.5 GiB, which is the existing admission total, now enforced by
the kernel. A capsule requested R: `memory.high` R, `memory.max` 2R,
admission against R (doctrine, docs/choir-doctrine.md and
docs/runtime-invariants.md).

## What the probes must show

Forged frame: from an escaped-cell worker started by the self-test (the
production spawner and entrypoint), raw frames for an action outside its table,
another desk's name in the payload, a wrong token, a bad version byte, an emit.
Each gets a refusal (or the connection closes, for version), and the host
trace has a refused `desk.egress` event for each. Escape: sockets of seven
kinds, fork, exec, signals, set*id, unshare, writes outside its directory,
reads of the credential disk, another desk's directory, and `/proc/1`. Out of
memory: a cell that allocates until it dies is killed by the cgroup, the
daemon's pid is unchanged, and the desk's result names the memory budget.
Real turns: a management, a research and a Texture turn on the same computer.

## Residuals named now

- The guest daemon still runs as root (SEC-15's main part). Design: split the
  runtime into an unprivileged API/orchestration process and a small
  privileged helper that creates capsules and cgroups, reached over a unix
  socket with peer credentials. It touches capsule creation, mounts and
  signers and is not a part of this batch.
- `OwnedKinds`, `Model`, `Restart` have no consumer; `recordsRoute` is checked
  against `Addresses`, not generated from it.
- ARCH-05's data-flow record for how research output reaches Texture and
  management is a doc; the enforcement here is the pool, the user and the
  profile.
