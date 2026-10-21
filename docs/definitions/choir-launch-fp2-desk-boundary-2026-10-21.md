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
        go test ./internal/deskmanifest ./internal/yaegikernel ./internal/agentprofile ./internal/capsule
        (TestFrameRole*, TestForgedFrame*, TestSlotToken*, TestFrameRefuses*, TestHandleScopes*,
        TestVerbDispatchMatchesKernel, TestFrameBoundarySelfTestReceipt, the manifest tests), and
        scratchpad/bin/heavy go test ./internal/agentcore -run 'TestDeskRefusal|TestPool|TestManifest|TestRecordsDesk'
      proves: >-
        a frame naming an action outside the sending desk's table, carrying another
        desk's identity, a wrong slot token, a stale epoch, an unknown version, or an
        emit, is refused with a visible error and a trace event, never reaches the
        handler, and a wrong token ends the worker
      evidence_class: local test
    - action: >-
        the Linux tests in a privileged Linux container (go test -c for linux/arm64): TestDeskCell*,
        TestDeskCgroup*, TestCapsuleSeccomp*, TestSessionWorker*
      proves: >-
        an escaped worker cannot open sockets, fork, exec, signal, trace or write
        outside its own directory; a cell that allocates without end is killed by the
        kernel at its worker's budget while the host process and a sibling worker live;
        clone3 answers ENOSYS and CLONE_NEWUSER is refused
      evidence_class: local Linux test (Docker kernel 6.10, cgroup v2, no Landlock); Landlock and the guest's cgroup layout are staging proof
    - action: >-
        POST /api/runtime/isolation-selftest on a disposable test computer, judged by
        scripts/desk_cell_confinement_probe.py (exact steps in the FP2 report)
      proves: >-
        on the deployed guest kernel: Landlock V5 enforced, three distinct cell uids,
        CapEff 0, NoNewPrivs 1, Seccomp 2, own network namespace, every escape probe
        refused; the deployed binary's frame admission serves each desk exactly its table
        and refuses every forged case
      evidence_class: deployed proof
    - action: >-
        the out-of-memory probe and a real turn of each desk on the same computer
        (steps 4 and 5 of the FP2 report), then texture-acceptance-deployed
      proves: >-
        a runaway cell on the guest is killed at its worker's budget with the daemon's
        pid unchanged, and real management, research and Texture turns work confined
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
  status: checkpoint_incomplete
  selftest_oom_leg: >-
    2026-10-22, lane/fp2-selftest-oom (from d23dfaa2d), not landed. The
    isolation self-test has an OOM leg, POST
    /internal/runtime/isolation-selftest?leg=oom (host-internal since review
    bunch 22). A throwaway research worker
    under its real 768 MiB cgroup runs a fixed allocating cell. The leg
    reports oom_kill from memory.events, the worker's and the slice's
    limits, the daemon's pid, start ticks and boot id before and after, and
    whether a sibling answers. Callers: the guest's host peer only (the
    transport check, server.GuestHostCaller; see review_bunch_22 and
    review_bunch_27). FP2 step 4 runs
    it through desk_cell_confinement_probe.py --leg oom from Node B. The capsule leg (SEC-13,
    RES-03) is not built: a capsule is not reachable safely from the
    self-test (file:line in
    docs/problems/fp2-step4-deployed-oom-not-drivable-desk-refuses-runaway-2026-10-22.md).
    Not yet run: TestDeskOOMSelfTestKillsTheWorkerAtItsBudget needs root and
    cgroup v2 (FP0's Linux mirror run). Then the deployed leg on a test
    computer, which can tick RES-01 and RES-02.
  review_bunch_22: >-
    Sent back on bb50792d0, repaired on the same lane. The oom and seccomp
    legs moved to the host-internal POST
    /internal/runtime/isolation-selftest?leg=... The guest admits its host
    peer only (the credential check is inert on guests; see review_bunch_27).
    The proxy refuses a leg on
    /api; the probe and proof steps 4 and 6 run from Node B over ssh. The
    OOM leg gained: a MemAvailable guard (budget + 1 GiB), a sibling gate, a
    cgroup path saved before Eval (the timeout panic), a host-recorded start
    marker, and mode required in both judges. Seccomp children: group kill
    at the timeout. Proof steps renumbered: 6 is seccomp, 7 is rollback.
    Still FP0's: the root kill test, and both legs deployed.
  review_bunch_27: >-
    Round 2 accepted with edits (DeepSeek v4.1 flash). Doc accuracy (E1): the
    deployed guest runs hostauth in off mode (nix/autoputer-vm.nix sets
    neither CHOIR_INTERNAL_AUTH_MODE nor a credential; Authorize returns true
    in off mode), so the effective gate on the internal route is
    server.GuestHostCaller alone, the planned-restart rule. "Holding the
    internal service credential" was wrong for the deployed default and is
    removed here, in the handler comment and in the problem record. Named
    residual: guest internal routes rely on the transport gate; enforce mode
    on guests is a follow-up (red, not taken here).
  selftest_seccomp_leg: >-
    2026-10-22, same lane, on FP0's call. POST
    /internal/runtime/isolation-selftest?leg=seccomp re-execs the daemon's
    binary once per capsule filter. Each child loads the capsule's own
    filter (same constructor, BPF hash checked) and tries clone3 (ENOSYS)
    and a clone and an unshare into a new user namespace (EPERM); fork+exec
    and os/exec must still work. The receipt reports the kernel release.
    Proof step 6 runs it (renumbered in review bunch 22). It proves SEC-13's
    filters on the deployed kernel. It does not prove that a live capsule
    loads them, nor RES-03. Local:
    passes unprivileged on Linux 6.18, through cmd/autoputer.
  batch1_fix: >-
    Reviewer edits applied on lane/b3-desk-boundary-b1fix: (1) prlimit64 is
    read-only for a confined worker (pid 0 or self, null new_limit);
    (2) a bad wire version or reserved byte records a transport refusal
    (desk.egress, refused, verb "transport") before the worker ends;
    (3) memory.swap.max tolerates ENOENT. Not yet proved on a live guest: the
    sibling-rlimit denial is proved by confined-worker probes and a filter unit
    test only. The second batch is on lane/b3-desk-boundary-2.
  slice: >-
    hand-off to FP0: two candidates. The first batch is lane/b3-desk-boundary
    (under review, unchanged). The second batch is lane/b3-desk-boundary-2 on top
    of it: RES-04 (capsule memory per doctrine, observe), SEC-15's document-tool
    sandbox (observe) and the daemon's capability floor (observe). The non-root
    daemon itself (the privileged-helper split) is not built: a named residual
  source_ref: f399a2e8c
  deploy_identity: none of this batch is deployed
  candidate:
    id: fp2-desk-boundary (first batch); fp2-desk-boundary-2 (second batch, ref lane/b3-desk-boundary-2, base 5fcf59e21, adds internal/doctool, internal/capfloor, internal/content, cmd/autoputer and the capsule, agentcore, nix and probe-script changes listed in "Second batch")
    state: ready
    ref: lane/b3-desk-boundary
    base: f399a2e8c
    digest: the lane branch HEAD at hand-off (see the FP2 report)
    scope:
      - internal/deskmanifest
      - internal/yaegikernel
      - internal/agentcore
      - internal/agentprofile
      - internal/capsule
      - cmd/capsule-broker
      - internal/autoputer
      - internal/gomemlimit
      - nix/autoputer-vm.nix
      - scripts/desk_cell_confinement_probe.py
  conjecture:
    id: C-FP2-one-table
    claim: >-
      If confinement, frame role checks, handle scopes, capsule verbs and
      memory budgets read one desk manifest, then adding a desk (an app desk)
      cannot add authority by omission, and a reviewer can read a desk's whole
      authority in one place.
    test: >-
      Parity tests that fail when a desk's authority differs from the pinned
      table; a test that every consumer rejects a desk with no entry; the
      AST-reading TestVerbDispatchMatchesKernel; the staging probe's own table.
    edge: missing_oracle
    delta_o: >-
      The deployed probe (frame_boundary and desk_cells in the isolation
      self-test), which reads facts from /proc and from the daemon's own
      admission outside the cell.
    scope_if_supported: the five system desks and app desks that register through the manifest
    status: testing
    evidence_refs: [the local tests named under finish.acceptance; no deployed proof yet]
  decision:
    what: >-
      Frame role table lives in the manifest; the check sits in the yaegikernel
      transport (admitFrame), the one place every host-served action passes, with a
      second check in the agentcore Egress handler. Memory budget: cgroup v2 per
      worker under a desks cgroup the daemon makes, required in the guest image,
      best effort elsewhere; hard memory.max only, no memory.high (measured: it
      throttles a runaway for minutes instead of killing it). Cell identity is a
      numeric uid from the manifest. clone3 ENOSYS and no CLONE_NEWUSER in every
      capsule filter.
    kind: architecture
    status: proposal
    evidence_ref: the commits of lane/b3-desk-boundary; a panel on the frozen diff
    owner_ratification_ref: owner notes sections 36 and 37 ratify the goal (every item lands before launch), not these choices
  belief:
    believed_state: >-
      Frames, handles, capsule verbs and message targets read the manifest and the
      Linux behavior (identity, netns, seccomp, cgroup kill, clone rules) holds on a
      6.10 kernel. Landlock V5, the guest cgroup layout and the 8 GiB sizes are
      unproved.
    main_uncertainty: >-
      Whether the guest lets the runtime make /sys/fs/cgroup/choir-desks beside the
      existing /sys/fs/cgroup/capsule (the capsule code writes a root-level cgroup
      the same way) and whether 768 MiB / 384 MiB fit real research and management
      cells.
    next_observation: >-
      The staging probe's desk_cells receipt, then a research turn's worker memory.peak.
  blocker_or_risk: >-
    A required-mode guest image that cannot confine or cannot make its cgroup would
    stop every desk on the refreshed computers, the paper included: the first deploy
    must reach a test computer and pass the probe before the paper refreshes (FP0's call).
  next_action: >-
    FP0: panel the frozen diff, rebase, land with its own deploy, run the FP2 report's
    probe steps on a disposable test computer before the paper refreshes. The second
    batch lands after the first, in a deploy of its own; its proof is "Second batch,
    staging steps" below. Owner or FP0: whether and when the three observe switches
    move to doctrine, required and enforce (each after its probe).

receipts:
  - id: fp2-local
    boundary: implement
    identity: lane/b3-desk-boundary at hand-off
    proof_refs:
      - go test of internal/deskmanifest, yaegikernel, agentprofile, capsule on the Mac
      - the Linux container runs named in the FP2 report (yaegikernel as root and uid 1000, capsule seccomp, capsule-broker session limits)
      - heavy go test ./internal/agentcore -run TestDesk|TestPool|TestManifest|TestRecordsDesk|TestIsolation|TestRuntime|TestResearch|TestManagement
    rollback_ref: git revert of the lane's commits
    disposition: candidate ready for review; not landed
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
  - id: fp2-local-2
    boundary: implement
    identity: lane/b3-desk-boundary-2 at hand-off
    proof_refs:
      - go test ./internal/capsule ./internal/doctool ./internal/capfloor ./internal/content on the node-a agents VM (linux/amd64, kernel 6.18, Landlock V5, uid 1000, unprivileged user namespaces)
      - go test ./internal/agentcore -run 'TestIsolation|TestGuestEgress|TestEngineeringCapsule' (all pass except TestIsolationSelfTestDeskCellsThroughTheRealWorker, a first-batch test that fails on this VM; see "Second batch, residuals")
    rollback_ref: git revert of the second batch's commits
    disposition: candidate ready for review; not landed
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
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
guesses. Guest 8 GiB (`VM_INTERACTIVE_MEM_MIB=8192`). Daemon unit
`MemoryMin` 512 MiB (a guess, to be set from L1-02's measurement). Desks
cgroup (made by the daemon at start): `memory.max` half of guest memory,
`memory.high` three eighths. A worker: `memory.max` 768 MiB for management,
Texture and research; 384 MiB for a management task and a research child
(guesses; the first staging probe reads each worker's `memory.peak` and FP0
adjusts before the paper refreshes). A worker has no `memory.high`: measured in a
Linux container, `memory.high` at 320 MiB under a 384 MiB cap throttled a
runaway cell 5272 times in 90 s without reaching the kill, so the hard limit
alone is the budget. Capsule `memory.high` = requested, `memory.max` = 2x
requested, admission against requested (docs/choir-doctrine.md:406-408) is
RES-04, not built here: a `memory.high` below the link's anonymous peak
(1.50 GiB stripped, 1.80 GiB with DWARF) would throttle the build with no swap,
so the values need `memory.events` from real builds first.

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

## Data flow between taint classes (ARCH-05)

Research and its children read attacker-controlled content from the world
(pages, feeds, files). Management, Texture and engineering hold authority to
act. Untrusted content reaches a privileged desk only as a record or a report
that the host reduces onto the ledger: research stages `Report`/`ReportPacket`
acts, the reducer writes records addressed to `texture:<doc>`, and Texture
reads them with `choir.Records()` in its own worker. There is no call from a
research cell into a privileged desk, no shared process (the pool keys a worker
to one desk and `sameDeskSessionConfig` compares roles), no shared uid
(`TestCellUIDsAreDistinctPerWorkerClass`: an untrusted-input desk never shares a
uid with a privileged one), and no shared memory. Captures that management and
Texture read (`ReadCapture`) are content research or a source poll fetched; the
text is data in the cell, and what a cell may do with it is the table in the
manifest, not the content. Mail's input is labelled `UNTRUSTED_EXTERNAL_EMAIL` by
`maild` and the email desk is `untrusted_input` in the manifest; it has no worker
yet.

## Staging probe steps (for FP0; no secrets here)

All on a disposable test computer, after the deploy carrying this batch, never
first on the paper.

1. `GET /health` on the platform shows the deployed commit. On the computer:
   `GET /api/runtime/observability` shows the guest build commit. Cold boot
   the test computer onto the new guest image (an idle computer picks it up on
   its next refresh).
2. `python3 scripts/desk_cell_confinement_probe.py --cred <the test computer's
   0600 credential file> --out docs/evidence/fp2-desk-boundary-<date>.json --skip-egress`
   (drop `--skip-egress` after the host egress policy, lane FP3, is deployed).
   Pass is exit 0: mode `required`, Landlock ABI >= 5, three distinct cell
   uids (61001, 61002, 61003), CapEff 0, NoNewPrivs 1, Seccomp 2, own network
   and IPC namespaces, every escape probe EPERM or EACCES, the cross-desk canary
   EACCES, the check cell runs; and `frame_boundary`: each desk served exactly its
   table, every forged case refused, a wrong slot token ends the worker, no
   refused frame reached a handler. Exit 2 means the route is not deployed.
3. The trace shows refusals: for a run of the test computer after step 2 nothing
   is expected (the probe's handlers are counters); a real refusal appears as a
   `desk.egress` run event with `refused: true`.
4. Out of memory: `scripts/proofs/fp2-desk-boundary.sh --computer <test>
   --step 4 --mutate`, which runs `desk_cell_confinement_probe.py --leg oom`
   from Node B over ssh (POST the guest's host-internal
   `/internal/runtime/isolation-selftest?leg=oom`, once, never retried).
   No model is in the loop: the deployed desks refuse a runaway prompt
   (docs/problems/fp2-step4-deployed-oom-not-drivable-desk-refuses-runaway-2026-10-22.md).
   Pass is exit 0:
   - mode `required`, and MemAvailable at least 768 MiB + 1 GiB;
   - the cell's start marker reached the host;
   - the research worker's `memory.max` is 768 MiB and its `memory.events`
     `oom_kill` is at least 1;
   - its exit cause names the memory budget;
   - the desks slice has a hard `memory.max`;
   - a sibling research worker answers before and after;
   - the daemon's pid, `/proc/self/stat` start ticks and boot id are
     unchanged.
   No answer is exit 1, because the daemon may have died.
5. Real turns: management, research and Texture each complete a turn on the
   computer (`cd frontend && BASE_URL=https://choir.news npx playwright test
   tests/texture-acceptance-deployed.spec.js` with the test computer's session),
   and one engineering assignment completes (this proves the capsule seccomp,
   the session worker's new limits, and that nothing inside needed `clone3`).
6. Capsule seccomp: `scripts/proofs/fp2-desk-boundary.sh --computer <test>
   --step 6 --mutate`, which runs `desk_cell_confinement_probe.py --leg
   seccomp` from Node B over ssh on the host-internal route. Pass is exit 0:
   for each of the six capsule filters, the loaded BPF hash equals the
   daemon's, Seccomp is 2, clone3 is ENOSYS, clone and unshare into a new
   user namespace are EPERM, and fork+exec and os/exec work. This is SEC-13
   on the deployed kernel, not inside a live capsule.
7. Rollback: `git revert` the lane's commits and the Landing Loop; the guest
   image carries the unit environment, so a revert is a deploy.

## Residuals named now

- Guest internal routes rely on the transport gate; enforce mode on guests is
  a follow-up. The guest runs hostauth in off mode, so the self-test's
  internal route (like pprof and the planned-restart routes) is gated by
  `server.GuestHostCaller` alone: the request must come from the host's
  gateway address on the tap. A guest-root process with CAP_NET_ADMIN could
  assign that address and pass. A confined desk cell (non-root, private
  netns, seccomp without socket or connect) cannot. Setting enforce mode
  and loading a credential on the guest is a separate red decision (review
  bunch 27 E1); it is not taken here.
- The guest daemon still runs as root (SEC-15's main part). Design: split the
  runtime into an unprivileged API/orchestration process and a small
  privileged helper that creates capsules and cgroups, reached over a unix
  socket with peer credentials. It touches capsule creation, mounts and
  signers and is not a part of this batch.
- `pdftotext`, `pdfinfo` and `pandoc` still run on untrusted bytes as guest root
  with the guest's network (`internal/content/extract.go:215`). Design: run them
  through a re-exec helper that drops to a numeric uid in an empty network
  namespace, sets rlimits, no_new_privs and Landlock (read the input file and the
  tool's store paths, write nothing) and then execs the tool.
- RES-04 (capsule `memory.high` / `2x` / admission against the request, and a
  `memory.events` reader) and the capsules slice aggregate of RES-01 are not
  built; the mechanism needs measured values first.
- `OwnedKinds`, `Model`, `Restart` have no consumer; `recordsRoute` is checked
  against `Addresses`, not generated from it.
- The host's emit handler (`deskEmitSignal`, `DeskSessionWorkerConfig.Emit`) is
  unreachable (no desk's row holds `emit`) but still compiled: delete it.
- `CHOIR_DESK_CELL_CONFINEMENT` unset means `auto`: best effort on a host that
  cannot do more (dev, CI). The guest image sets `required`, pinned by a test. A
  stricter default (required unless a test says otherwise) would need every
  desk-spawning test to opt out explicitly; FP0 decides.
- Same-UID sub-roles (`research`/`research_child` on 61003, `management`/
  `management_task` on 61001) share read access to `/tmp` (the default
  `AllowedRoot`) and share a UID, so a sibling can read another's
  `choir-cell-*` directory. Fix later: distinct UIDs per sub-role and an empty
  read-only `AllowedRoot` for non-engineering desks (Argon).
- The aggregate desks `memory.high` near 3 GiB can throttle every desk's
  descendants before any reaches its own hard limit, the mechanism measured and
  removed at worker level. Remove it or justify it with concurrent-load
  measurements and aggregate-event visibility (sol).
- There is no one guest-wide memory budget: the desks aggregate allows about
  half of MemTotal while capsule admission independently allows three quarters
  (`internal/capsule/job_memory.go`); `MemoryMin=512M` is not OOM immunity
  (sol).

## Second batch (lane/b3-desk-boundary-2): RES-04 and SEC-15

Started 2026-10-21 on top of the first batch (5fcf59e21), which is under
review and is not changed here. Built in the Choir agents VM on node-a
(uid 1000, no root, Landlock and unprivileged user namespaces available, no
Nix builds): every nix change in this batch is unevaluated. Each item is a
separate red commit, tests first, off or in observe mode in the image until
FP0 has the probe on a test computer.

### RES-04: capsule memory per doctrine

Doctrine (`docs/choir-doctrine.md:406-408`): `memory.high` is the request,
`memory.max` twice that, admission counts the request, `memory.events` is
feedback. Code admits and caps at `MemoryMax`. The two cannot simply be
swapped: an engineering capsule's link peaks at 1.50 to 1.80 GiB anonymous
with no swap, so a `memory.high` below that throttles the build for minutes
(the 1 GiB drill thrashed for 14). So the mechanism lands with a policy
switch, `CHOIR_CAPSULE_MEMORY_POLICY`:

- `observe` (the default and the image's value): the cgroup and admission are
  exactly as before (`memory.max` = `MemoryMax`, admission against it). At
  destroy, the executor reads the capsule's `memory.events` (`high`, `max`,
  `oom`, `oom_kill`) and `memory.peak` and logs a memory receipt that names
  what doctrine would have done: the request, the `memory.high` it would set,
  and whether the peak would have crossed it. These receipts are the
  measurement the values need.
- `doctrine`: a spec that declares `MemoryRequest` gets `memory.high` =
  request, `memory.max` = 2 x request, admission against the request, and a
  job memory boost raises both. The capsules aggregate (`/sys/fs/cgroup/capsule`,
  the RES-01 half the first batch left) gets `memory.max` = the live capsule
  budget, so the overcommit doctrine allows is bounded by the same number
  admission uses. A spec with no request keeps the observe shape.
- Any other value is `observe`: the narrower shape.

The engineering assignment declares a request of 2 GiB (above the measured
1.80 GiB peak, so `memory.high` does not throttle the link): admission is
unchanged at 2 GiB and, under `doctrine`, the ceiling becomes 4 GiB.

Failure modes, written before code: Q1 a `memory.high` below a build's
anonymous peak throttles it with no swap (why the default is observe); Q2
admission against the request lets the sum of ceilings exceed the budget (the
aggregate bounds it, doctrine only); Q3 a spec with no request silently
doubles its ceiling under doctrine (it keeps the observe shape); Q4 a boost
raises `memory.max` but not `memory.high` and the job throttles under its
own boost (both are raised); Q5 a boost's end lowers a limit under usage
(`memory.max` comes down only when usage fits, as before; `memory.high`
follows it); Q6 accounting drift: spawn adds one number and destroy subtracts
another (the admitted bytes are stored on the capsule); Q7 the aggregate is
written below the subtree's usage and the kernel kills across capsules (it is
written only when the usage fits and never lowered under it); Q8 a policy typo
widens (unknown is observe); Q9 `memory.events` or `memory.peak` unreadable
(old kernel): the receipt says unknown and destroy never fails for it; Q10
request <= 0 or 2 x request overflowing under doctrine: the spawn is refused.

### SEC-15: the document-tool sandbox

`pdftotext`, `pdfinfo` and `pandoc` parse untrusted bytes as guest root with
the guest's network (`internal/content/extract.go:215`). The fix is the
design named in the first batch's residuals: the daemon runs each tool
through a re-exec helper (`autoputer doc-tool-sandbox`) that it starts as a
numeric sandbox user (uid and gid 61010, `choir-doctool`, declared in the
image for operators and pinned by a test), in an empty network and IPC
namespace, in a cgroup of its own under the desks cgroup (`w-doctool-*`, so
the boot sweep of the first batch removes a crashed run's leftover; hard
`memory.max` 1 GiB, a guess to be set from the probe, pids 64, no swap). The
helper sets no_new_privs, rlimits (no core, 64 open files, CPU 60 s), Landlock
V5 (read and execute the tool's roots, `/nix/store` on the guest; read the
one input directory and its own `/proc/<pid>`; write nothing; no TCP), a
seccomp deny list (ptrace, mount, namespaces, `CLONE_NEWUSER`, `clone3`
answering ENOSYS, bpf, keyrings, io_uring, kernel modules and the like, and
`socket`), then execs the tool. The input is a 0400 file in a 0700 directory
the sandbox user owns; output is the tool's stdout through a pipe, capped at
64 MiB.

`CHOIR_DOCUMENT_TOOL_SANDBOX`: unset or `off` runs the tools as before;
`observe` (the image's value) runs them sandboxed and, when the sandbox
cannot be set up, runs the tool as before and adds the warning
`document_tool_sandbox_fallback: <reason>` to the extraction (so the probe
can count them); `required` runs sandboxed or not at all (warning
`document_tool_sandbox_refused: <reason>`, and the extraction falls back to
its pure-Go projection as it does today when a tool is missing); any other
value is `required`.

Failure modes, written before code: D1 the tool runs as root or with network;
D2 it reads a secret (credential disk, store, another desk's directory, the
daemon's `/proc`); D3 it writes anywhere; D4 it exhausts memory or processes;
D5 it creates namespaces, traces, or loads kernel code; D6 a sandbox that
cannot be set up runs the tool unsandboxed in required mode; D7 a crashed run
leaks its directory or cgroup; D8 the helper entry is reachable without the
daemon's environment (argv alone does nothing); D9 a kernel without Landlock
V5 passes required mode; D10 the helper runs a tool as root (it refuses in
required mode); D11 unbounded output fills the daemon's memory.

### SEC-15: the daemon's capability floor (a first step, not non-root)

Making the daemon non-root is the split the first batch designed (an
unprivileged API and orchestration process, a small privileged helper for
capsules, cgroups and cell spawn, over a unix socket with peer credentials).
It is not a slice this lane can land safely: the capsule executor alone is
reached from 55 call sites in agentcore and autoputer, in process. Recorded,
not built; see the problem record. What lands is the floor under it: the
daemon keeps only the capabilities the inventory names, each with the code
that needs it, and drops the rest from its bounding set, so neither it nor any
child (git, go, node, libreoffice, a capsule's user namespace) can regain
them. Kept: CHOWN, DAC_OVERRIDE, DAC_READ_SEARCH, FOWNER, FSETID, KILL,
SETGID, SETUID, SETPCAP, NET_BIND_SERVICE, NET_ADMIN, SYS_CHROOT, SYS_PTRACE,
SYS_ADMIN, SYS_RESOURCE. Dropped: every other, among them SYS_MODULE,
SYS_RAWIO, SYS_BOOT, SYS_TIME, MKNOD, NET_RAW, BPF, PERFMON, SYSLOG,
MAC_ADMIN, MAC_OVERRIDE, LINUX_IMMUTABLE, AUDIT_*, CHECKPOINT_RESTORE.

The bounding set is per thread and a cgo binary cannot change every thread at
once, so the daemon drops on a locked thread at the top of `main` and
re-executes itself once, marked; the new image inherits the set on every
thread. `CHOIR_RUNTIME_CAPABILITY_FLOOR`: unset or `off` changes nothing;
`observe` (the image's value) logs what it would drop and reports the
daemon's uid and `CapBnd` in the isolation self-test; `enforce` drops,
re-executes, and after the re-exec refuses to start if any thread still holds
a capability outside the inventory; any other value is `enforce`.

Failure modes, written before code: K1 a needed capability is dropped and
capsules or desks fail (observe first, the inventory names a reason for each
kept capability, and a test computer runs a capsule and every desk under
`enforce` before the paper); K2 only the calling thread drops (the re-exec);
K3 a re-exec loop (the marker; a marked start never re-executes); K4 a
marked start that still holds extra capabilities (refused under enforce);
K5 a child that needed a dropped capability (none known: no Mknod, raw I/O,
module or time call in the guest code); K6 calling this non-root (it is not;
the receipt says root with a floor).

### Second batch: what was built

Commits on lane/b3-desk-boundary-2 (base 5fcf59e21):

- f7a1e17de docs: the designs and failure modes above (green).
- f414e3ca7 RES-04: `internal/capsule/capsule_memory.go`, the policy, the
  receipts (`capsule_memory` in `/api/runtime/observability`), the
  capsules aggregate under doctrine; the engineering request (red).
- e9de19687 SEC-15 document tools: `internal/doctool`, the helper and probe
  entries in `cmd/autoputer/main.go`, `internal/content/extract.go` through
  it, `document_tool` in the isolation self-test and the probe script, the
  `choir-doctool` user (red). One addition to the design above: observe
  never makes an extraction worse. A tool that fails only inside the
  sandbox runs again as before, and the warning is
  `document_tool_sandbox_divergence`.
- 4ab63a8f2 SEC-15 capability floor: `internal/capfloor`, `Apply` at the
  top of the daemon's `main`, `runtime_capabilities` in the isolation
  self-test and the probe script (red).

The three switches ship in observe (`CHOIR_CAPSULE_MEMORY_POLICY=observe`,
`CHOIR_DOCUMENT_TOOL_SANDBOX=observe`, `CHOIR_RUNTIME_CAPABILITY_FLOOR=observe`).
Under observe the capsule memory policy and the capability floor change no
behavior. The document-tool sandbox under observe changes how tools run
(sandboxed, as uid 61010) but never what an extraction returns. All nix is
unevaluated.

### Second batch, staging steps (FP0; no secrets here)

On a disposable test computer after the deploy that carries this batch,
never first on the paper. These steps come after the first batch's steps 1
to 5.

1. `python3 scripts/desk_cell_confinement_probe.py --cred <the test
   computer's 0600 credential file> --out
   docs/evidence/fp2-desk-boundary-2-<date>.json --skip-egress`. The
   receipt has two new sections. `document_tool` must show uid 61010,
   NoNewPrivs 1, Seccomp 2, CapEff 0, and every probe denied (never by
   ENOENT). `runtime_capabilities` is report-only under observe: record
   `cap_bnd`, `extra` and `missing`. `missing` should be empty.
2. Document tools under observe: read a PDF (a research turn that opens
   one, or a capture of one), then count `document_tool_sandbox_fallback`
   and `document_tool_sandbox_divergence` in the extractions'
   `extraction_warnings`. Both should be zero. Read the run's `w-doctool-*`
   memory with the desks cgroup log line, and set `toolMemoryMax` from it
   if 1 GiB is wrong.
3. Capsule memory under observe: run one engineering assignment to its end.
   `GET /api/runtime/observability` then has a `capsule_memory` receipt
   with policy observe, request 2048 MiB, its `peak_bytes`, `events_max`
   and `doctrine_would_throttle`. Doctrine is safe to try when builds show
   `events_max` 0 and a peak under the request.
4. Enforce trial (the capability floor), on the test computer only. Set
   `CHOIR_RUNTIME_CAPABILITY_FLOOR=enforce` there (an image built with it,
   or the env file `/run/go-choir-autoputer.env`, if FP0's tooling writes
   it without SSH; otherwise this step waits for an image). Then: the
   daemon starts (log `capfloor: enforced: bounding set 00000000012c15ff`),
   the probe passes with `runtime_capabilities` judged, one engineering
   assignment with egress completes, and a management, research and
   Texture turn each complete. If something fails, its log names the
   capability; add it back to `capfloor.Kept` with its reason.
5. Moving switches to doctrine, required or enforce on the paper is a
   separate decision, each after its own probe (FP0 or owner).

### Second batch, residuals

- **The non-root daemon (SEC-15's largest part) is not built.** The
  privileged-helper split is unchanged from the first batch's design. The
  floor is the step under it: it bounds what root can do, but the daemon is
  still root.
- The capability inventory is conservative. DAC_READ_SEARCH, FSETID,
  SYS_CHROOT and SYS_RESOURCE are kept "until the enforce probe shows them
  unused". Each later removal is a reviewed change.
- The document-tool sandbox runs the daemon's own binary as the helper,
  as uid 61010. That binary must be executable by that uid, the same need
  the desk cells have for `autoputer desk-session` as 61001 to 61003. If
  the cells start on staging, the helper will too.
- Seccomp for document tools is a deny list (default allow), because the
  three tools differ in what they need. An allow list needs syscall traces
  of real runs (strace inside the sandbox on a test computer).
- `toolMemoryMax` 1 GiB and the engineering request of 2 GiB are first
  values. The receipts (step 2 and step 3) set them.
- Observed on the node-a agents VM: `TestIsolationSelfTestDeskCellsThroughTheRealWorker`
  (first batch, `internal/agentcore`) fails as uid 1000 on a kernel with
  Landlock. Without root every desk's worker runs as the same uid, and the
  allowed root (the test's TMPDIR) contains the other desks' tmp dirs, so
  the cross-desk canary is readable. This batch changes no desk-cell code;
  whether the test needs a root or no-Landlock guard is FP0's call. It is
  not fixed here.
