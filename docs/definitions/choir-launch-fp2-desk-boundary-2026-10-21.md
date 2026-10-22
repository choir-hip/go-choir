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
  batch2_round2: >-
    2026-10-22: the second batch replayed onto origin/main b7d79a082 as
    lane/fp2-b2-on-main, and DeepSeek's round 2 (bunch 39) edits E1-E8 made
    there: CAP_SYS_TIME kept for the hibernate resume (E1, unblocks enforce in
    code; the enforce trial now includes a hibernate and resume), the observe
    shadow in its own cgroup (E2), E3-E6 and E8 fixed, the station's text and
    hashes re-pointed (E7). Switches still ship observe. See "Second batch,
    review round 2: outcome". Rebased onto a84b39bac (861e765a3). Round 3
    (DeepSeek, bunch 43) F1-F5 fixed or recorded ("Second batch, review round
    3: outcome"); Devin's review is still running.
  batch2_deploy_p: >-
    2026-10-22: batch 2 live in deploy P (966d3f519). Staging step 1 failed: the
    self-test's document-tool probe ran the desk-worker binary
    (docs/problems/doctool-selftest-runs-desk-worker-2026-10-22.md); fixed on
    lane/fp2-selftest-binary with the seccomp leg, not deployed. FP0: land it,
    then re-run step 1 and the seccomp leg before reading step 2.
  slice: >-
    hand-off to FP0: two candidates. The first batch is lane/b3-desk-boundary
    (under review, unchanged). The second batch is lane/b3-desk-boundary-2 on top
    of it: RES-04 (capsule memory per doctrine, observe), SEC-15's document-tool
    sandbox (observe) and the daemon's capability floor (observe). The non-root
    daemon itself (the privileged-helper split) is not built: a named residual
  source_ref: f399a2e8c
  deploy_identity: none of this batch is deployed
  candidate:
    id: fp2-desk-boundary (first batch); fp2-desk-boundary-2 (second batch, ref lane/fp2-b2-on-main, replayed onto origin/main b7d79a082 from lane/b3-desk-boundary-2, adds internal/doctool, internal/capfloor, internal/content, cmd/autoputer and the capsule, agentcore, nix and probe-script changes listed in "Second batch")
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
    staging steps" below. Review round (bunch 4b, frozen e2a88e4ee) repaired on the
    lane: five blocking findings fixed, none refuted ("Second batch, review round:
    outcome"); FP0 re-freezes and re-reviews the new head. Review round 2 (DeepSeek,
    bunch 39, head 5c7615904) accepted it for observe with edits E1-E8; the lane was
    replayed onto origin/main as lane/fp2-b2-on-main and every edit is made there
    ("Second batch, review round 2: outcome"). FP0: land it in a deploy of its own
    and run "Second batch, staging steps", including step 4's hibernate and resume
    before any enforce. Owner or FP0: whether and when the three observe switches
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
      - review round 2 on lane/fp2-b2-on-main (node-a agents VM, linux, uid 1000, Landlock V5): GOMAXPROCS=2 GOFLAGS=-p=2 go test -race of internal/capfloor, doctool, capsule, content, agentprofile, and internal/agentcore -run 'Hibernate|TestIsolation|DocTool|Doctool|DocumentTool|RuntimeCapabilit'; go vet of the changed packages; go build ./cmd/autoputer
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
review and is not changed here. Replayed 2026-10-22 onto origin/main
b7d79a082 as lane/fp2-b2-on-main (cherry-pick, no conflicts), then
rebased onto a84b39bac; every commit hash below is on the lane head. Built in the Choir agents VM on node-a
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

Sizing evidence (2026-10-22, FP4 lane/fp4-capsule-memory): a release build
ran at the 2 GiB ceiling with 2 OOM kills on 71afbf14. With Go bounded to
the capsule's quota (fd8337941, c68c49a99), a capsule-shaped cold build's
anonymous peak is 846 MiB in a local measurement; the deployed proof (the
FP4 drill re-run on 71afbf14) is pending. The 2 GiB request stands; `memory.peak` counts page cache,
so `doctrine_would_throttle` reads true for every cold build
(docs/evidence/res04-capsule-memory-sizing-2026-10-22.md).

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
namespace, in a cgroup of its own (`t-*`) under the document tools' own root
`CHOIR_DOCTOOL_CGROUP_ROOT` (`/sys/fs/cgroup/choir-doctools`, a 2 GiB
aggregate, never the desks' root: review B2; swept of `t-*` leftovers at
daemon start; per run a hard `memory.max` 1 GiB, a guess to be set from the
probe, pids 64, `memory.oom.group`, no swap). The
helper sets no_new_privs, rlimits (no core, 64 open files, CPU 60 s), Landlock
V5 (read and execute the tool's roots, `/nix/store` on the guest; read the
one input directory and its own `/proc/<pid>`; write nothing; no TCP), a
seccomp deny list (ptrace, mount, namespaces, `CLONE_NEWUSER`, `clone3`
answering ENOSYS, bpf, keyrings, io_uring, kernel modules and the like, and
`socket`), marks every fd from 3 up close-on-exec (review E8), then execs
the tool. The input is a 0400 file in a 0700 directory
the sandbox user owns; output is the tool's stdout through a pipe, capped at
64 MiB.

`CHOIR_DOCUMENT_TOOL_SANDBOX`: unset or `off` runs the tools as before;
`observe` (the image's value) runs each tool as before, and that run is the
extraction, byte for byte, with no warning added (review B3); a sandboxed
shadow of the same input then runs, one at a time, in its own `t-*` cgroup
(review E2; a shadow that cannot get one does not run), and its outcome is
only counted: equal, diverged (outputs differ, or only the sandbox
succeeded: review E3), sandbox failed, not set up, skipped, over budget
(`ShadowSnapshot`, reported in the isolation self-test as
`document_tool.observe`); `required` runs sandboxed or not at all (warning
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
them. Kept (16, floor `00000000032c15ff`): CHOWN, DAC_OVERRIDE,
DAC_READ_SEARCH, FOWNER, FSETID, KILL, SETGID, SETUID, SETPCAP,
NET_BIND_SERVICE, NET_ADMIN, SYS_CHROOT, SYS_PTRACE, SYS_ADMIN,
SYS_RESOURCE, and SYS_TIME (the hibernate resume steps the clock with
settimeofday; review E1). Dropped: every other, among them SYS_MODULE,
SYS_RAWIO, SYS_BOOT, MKNOD, NET_RAW, BPF, PERFMON, SYSLOG, MAC_ADMIN,
MAC_OVERRIDE, LINUX_IMMUTABLE, AUDIT_*, CHECKPOINT_RESTORE.

The bounding set is per thread and a cgo binary cannot change every thread at
once, so the daemon drops on a locked thread at the top of `main` and
re-executes itself once, marked; the new image inherits the set on every
thread. `CHOIR_RUNTIME_CAPABILITY_FLOOR`: unset or `off` changes nothing;
`observe` (the image's value) logs what it would drop and reports the
daemon's uid and `CapBnd` in the isolation self-test; `enforce` drops,
re-executes, and after the re-exec refuses to start if any thread still holds
a capability outside the inventory; any other value is `enforce`
(`Apply` itself refuses a mode it does not know: review E8).

Failure modes, written before code: K1 a needed capability is dropped and
capsules or desks fail (observe first, the inventory names a reason for each
kept capability, and a test computer runs a capsule and every desk under
`enforce` before the paper); K2 only the calling thread drops (the re-exec);
K3 a re-exec loop (the marker; a marked start never re-executes); K4 a
marked start that still holds extra capabilities (refused under enforce);
K5 a child or a daemon path that needed a dropped capability. The first
version said "none known: no Mknod, raw I/O, module or time call in the
guest code", and that was false: the hibernate resume calls settimeofday
(`internal/agentcore/hibernate_clock_linux.go`) on every resume after a
snapshot (review E1). CAP_SYS_TIME is kept for it, and
`TestKeptCoversTheDaemonsPrivilegedSyscalls` now parses the daemon's source
(internal, cmd/autoputer, cmd/capsule-broker; the two denial probes
excepted) for the named capability-needing calls into syscall and
x/sys/unix (a list in the test, raw SYS_* numbers included; review F4) and
fails if a call's capability is dropped. It is a list, not every call:
calls it does not name, syscalls reached through cgo, and binaries the
daemon execs (git, go, node) are the enforce trial's to find. K6 calling this non-root (it is not;
the receipt says root with a floor).

### Second batch: what was built

Commits on lane/fp2-b2-on-main (replayed from lane/b3-desk-boundary-2,
whose base was the first batch, 5fcf59e21; every commit listed below is
an ancestor of the lane head):

- 9c637ae8b docs: the designs and failure modes above (green).
- d36a1b78d RES-04: `internal/capsule/capsule_memory.go`, the policy, the
  receipts (`capsule_memory` in `/api/runtime/observability`), the
  capsules aggregate under doctrine; the engineering request (red).
- 911da25ad SEC-15 document tools: `internal/doctool`, the helper and probe
  entries in `cmd/autoputer/main.go`, `internal/content/extract.go` through
  it, `document_tool` in the isolation self-test and the probe script, the
  `choir-doctool` user (red). Its first observe (sandboxed result, with
  fallback and divergence warnings) was replaced in the review round by
  the direct result and a counted shadow (c97a577e1); no
  `document_tool_sandbox_fallback` or `document_tool_sandbox_divergence`
  warning exists.
- e2acf6025 SEC-15 capability floor: `internal/capfloor`, `Apply` at the
  top of the daemon's `main`, `runtime_capabilities` in the isolation
  self-test and the probe script (red).

The three switches ship in observe (`CHOIR_CAPSULE_MEMORY_POLICY=observe`,
`CHOIR_DOCUMENT_TOOL_SANDBOX=observe`, `CHOIR_RUNTIME_CAPABILITY_FLOOR=observe`).
Under observe the capsule memory policy and the capability floor change no
behavior. The document-tool sandbox under observe adds a counted,
cgroup-bounded shadow run but never changes what an extraction returns. All nix is
unevaluated.

### Second batch, staging steps (FP0; no secrets here)

On a disposable test computer after the deploy that carries this batch,
never first on the paper. These steps come after the first batch's steps 1
to 5.

**2026-10-22, deploy P (966d3f519): step 1 failed on computer-9648031f.**
The self-test's document-tool probe ran the dedicated desk-worker binary
(RES-17), which serves only desk-session, as the sandbox helper and probe
tool, so `document_tool` had no report; the rest of the self-test
answered. The capsule seccomp leg has the same shape by code reading.
Neither affects production extraction. Record:
`docs/problems/doctool-selftest-runs-desk-worker-2026-10-22.md`; fix on
lane/fp2-selftest-binary (both legs run the daemon's own executable).
Until that fix is deployed, step 1's `document_tool` judgement cannot
pass and "required is ready" (step 2) cannot be read. Re-run step 1, and
the seccomp leg, after the fix lands.

1. `python3 scripts/desk_cell_confinement_probe.py --cred <the test
   computer's 0600 credential file> --out
   docs/evidence/fp2-desk-boundary-2-<date>.json --skip-egress`. The
   receipt has two new sections. `document_tool` must show uid 61010,
   NoNewPrivs 1, Seccomp 2, CapEff 0, and every probe denied (never by
   ENOENT). `runtime_capabilities` is report-only under observe: record
   `cap_bnd`, `extra` and `missing`. `missing` should be empty.
2. Document tools under observe (revised in the review round): read PDFs
   (a research turn that opens one, or captures). Then run the probe again
   and read `document_tool.observe`: `runs`, `equal`, `diverged`,
   `sandbox_failed`, `setup_failed`, `skipped`, `max_rss_bytes`,
   `over_budget`. Required is ready when `diverged`, `sandbox_failed`,
   `setup_failed` and `over_budget` are all zero over real reads. The
   extractions themselves carry no sandbox warning under observe.
3. Capsule memory under observe: run one engineering assignment to its end.
   `GET /api/runtime/observability` then has a `capsule_memory` receipt
   with policy observe, request 2048 MiB, its `peak_bytes`, `events_max`
   and `doctrine_would_throttle`. Doctrine is safe to try when builds show
   `events_max` 0 and a peak under the request.
4. Enforce trial (the capability floor), on the test computer only. Set
   `CHOIR_RUNTIME_CAPABILITY_FLOOR=enforce` there (an image built with it,
   or the env file `/run/go-choir-autoputer.env`, if FP0's tooling writes
   it without SSH; otherwise this step waits for an image). Then: the
   daemon starts (log `capfloor: enforced: bounding set 00000000032c15ff`),
   the probe passes with `runtime_capabilities` judged, one engineering
   assignment with egress completes, and a management, research and
   Texture turn each complete. Then hibernate and resume the test computer
   (review E1): the host's snapshot and `POST
   /internal/runtime/hibernate/resumed` must answer 200 with a
   `clock_step_ms`, the resume must be a fast resume (no
   `guest_resumed_failed` snapshot fallback, no cold boot in the vmctl log),
   and a turn after it completes. If something fails, its log names the
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

### Second batch, review round (bunch 4b, frozen e2a88e4ee): findings

GPT-6.1-sol sent the candidate back. Gemini 4 Argon sent it back too, or
would accept it with the non-Linux fix. The findings file is FP0's
(`~/runs/fp2b2-findings.md`). The rule for this round: observe mode must
not change today's kill domain or today's results. Every blocking finding
is accepted. None is refuted.

- **B1 (sol, Argon), high: non-Linux builds break.** Accepted.
  `internal/agentcore/isolation_selftest.go` has no build tag, but it uses
  `doctool.SelfTestReceipt` and `doctool.SelfTest`, which exist only in
  `probe_linux.go`. Evidence: both reviewers got `undefined:
  doctool.SelfTestReceipt` / `doctool.SelfTest` building `internal/agentcore`
  for darwin; reproduced here with `GOOS=darwin GOARCH=arm64 go vet` (round
  receipt below). Fix: the report types and `JudgeProbe` move to portable
  code, and the non-Linux `SelfTest` fails closed ("needs Linux").
- **B2 (sol), high: observe charges the converter to the desks aggregate.**
  Accepted. `runSandboxed` puts every sandboxed run in a `w-doctool-*`
  child of `CHOIR_DESK_CGROUP_ROOT` (`doctool_linux.go:65-74`). Under
  observe, a converter's memory now counts against the desks' half-guest
  aggregate. Before, it ran in the daemon's cgroup, outside that aggregate.
  A desk worker (`oom_score_adj` 1000) can therefore be killed by memory
  the desks never used. Fix: the sandbox never runs in the desks cgroup.
  Under observe it runs with no cgroup budget, in the daemon's cgroup as
  before, and its peak RSS is recorded against the budget. Under required
  it gets a cgroup of its own under a separate document-tools root
  (`CHOIR_DOCTOOL_CGROUP_ROOT`, its own aggregate and boot sweep), never
  the desks root.
- **B3 (sol), medium: observe can change stored results (TZ).** Accepted.
  The helper's environment drops `TZ`, and a sandboxed run that succeeds is
  returned as the result. sol's check: the same PDF date prints as October
  20 under `TZ=America/New_York` and October 21 under `TZ=UTC`, and the
  printed date is stored as metadata. More generally, observe returned the
  sandbox's output whenever the sandbox succeeded, and the
  `document_tool_sandbox_*` warnings went into stored `extraction_warnings`.
  Fix: `TZ` is passed through. Under observe the direct run is the
  authoritative result and the extraction is byte-for-byte what it is today.
  The sandboxed run is a shadow that runs after it. Its outcome (equal,
  diverged, failed, setup failed) is counted in memory, logged, and shown
  in the isolation self-test, never written into the extraction.
- **B4 (sol), high: the aggregate does not follow a hotplug-backed boost.**
  Accepted. Under doctrine, `boostJobMemory` admits against the live budget
  and raises the leaf, but the capsules aggregate keeps the value written at
  the last spawn (`job_memory_linux.go:67-108`). Fix: under doctrine a boost
  first raises the aggregate to the live budget. If that fails, the boost is
  refused, and nothing is raised or counted.
- **B5 (sol), high: doctrine admission ignores aggregate failures.**
  Accepted. `setCapsuleAggregateLocked` only logs when it cannot read usage,
  when usage is above the budget, or when the write fails
  (`job_memory_linux.go:207-228`), and `Spawn` goes on. Fix: it returns an
  error, and under doctrine `Spawn` refuses the capsule (the existing error
  path deletes its cgroup) unless the aggregate is set to the budget. Under
  observe it is never written, as before.
- **Non-blocking (Argon), capfloor, cheap and in scope: fixed.** Under
  observe, an unreadable `/proc/self/status` now logs instead of stopping
  the daemon from starting. Under enforce, a start whose reading thread is
  already within the floor still verifies every thread.
- **Non-blocking (sol): "sandbox fallback does not establish observe
  equivalence."** Covered by B3's fix (shadow comparison).

### Second batch, review round: outcome

| finding | outcome | commit |
|---|---|---|
| B1 non-Linux build | fixed | 1dc74153f (orange) |
| B2 converter charged to the desks aggregate | fixed | c97a577e1 (red) |
| B3 observe changes stored results (TZ) | fixed | c97a577e1 (red) |
| B4 aggregate does not follow a hotplug boost | fixed | 62f39cc5f (red) |
| B5 doctrine admission ignores aggregate failures | fixed | 62f39cc5f (red) |
| Argon: observe status read, enforce thread check | fixed | f3fac0bc4 (red) |

Observe semantics after the round. Capsule memory under observe never
reads or writes the aggregate. The capability floor under observe never
fails the start. Document tools under observe return the direct run byte
for byte, add no warning, and (after this round) created no cgroup; round
2 (E2) gave the shadow a cgroup of its own, below. The shadow is counted
only.
One deliberate change remains under observe: one extra sandboxed process
per extraction (one at a time; a second is skipped), after the result is
returned. This round put it in the daemon's own cgroup and called that
"bounded load"; it was not (review round 2, E2: no memory or process
bound). Since round 2 it runs in a `t-*` cgroup of its own under the
document tools' root (1 GiB, 64 pids, oom.group, no swap) or not at all.
It changes no result; its kill domain is its own cgroup.

The design text above, in "SEC-15: the document-tool sandbox", was
rewritten in round 2 to describe the code (review E7). Required mode's and
the shadow's cgroups are `t-*` children of `CHOIR_DOCTOOL_CGROUP_ROOT`
(`/sys/fs/cgroup/choir-doctools`, a 2 GiB aggregate), made at daemon start
only when the sandbox is on.

Residuals added in this round:
- A darwin build of the whole `internal/agentcore` package could not run
  here: this VM has no darwin cgo toolchain, and Dolt's cgo dependencies
  stop the build first. The gap B1 named was reproduced and closed with a
  darwin-typechecked probe. `internal/doctool`, `capfloor` and `capsule`
  pass `GOOS=darwin GOARCH=arm64 go vet`. FP0 should run the darwin build
  on the Mac.
- The shadow doubles converter CPU work under observe, after the request
  returns, and holds up to 1 GiB of memory (its cgroup's `memory.max`)
  while it runs, inside the 2 GiB document tools aggregate, which is not
  carved out of the desks' or capsules' budgets. Turning observe off
  removes both.
- `toolAggregateMax` (2 GiB) is a first value, like `toolMemoryMax`.

### Second batch, review round 2 (DeepSeek, bunch 39, head 5c7615904): findings

Flash and Devin accepted the repaired candidate. DeepSeek accepts it for its
shipped configuration (all three switches observe) with edits, and finds one
that blocks enforce. The full review is FP0's (`~/runs/fp2b2-deepseek.md`).
The lane was replayed onto origin/main as lane/fp2-b2-on-main before any
fix. Every finding is accepted; none is refuted. Fixes come after this
record.

- **E1, high, blocks enforce: the floor drops CAP_SYS_TIME, which the
  daemon uses on every hibernate resume.** Accepted. `capfloor.Kept` has no
  `capSysTime`. `POST /internal/runtime/hibernate/resumed`
  (`internal/agentcore/hibernate.go`, `resumeFromHibernate`) steps the clock
  with `settimeofday` (`hibernate_clock_linux.go`) whenever host and guest
  differ by 1 s or more, which is always after a snapshot. Under enforce
  that is EPERM, the handler answers 500, and the host kills the guest and
  cold-boots it (`internal/vmmanager/snapshot.go`, resume fallback). K5's
  premise above ("no ... time call in the guest code") is false, and the
  enforce trial (staging step 4) runs no hibernate, so it would not catch
  this. Fix direction: keep CAP_SYS_TIME with the hibernate reason (fast
  resume stays); a test ties the inventory to every capability-needing
  syscall in the daemon's source; the probe constant, K5 and the enforce
  trial (a hibernate and resume) follow.
- **E2, medium: the observe shadow has no memory or process bound.**
  Accepted. The shadow runs with no cgroup (review B2's fix) and the helper
  sets only core, open-file and CPU rlimits. A crafted PDF can grow the
  shadow until the daemon's own cgroup runs out, after the result was
  returned. The direct run has the same exposure today, so this is not a
  new class, but "That adds bounded load" (outcome above) is false and the
  residual names CPU only. Fix direction: the shadow runs in a `t-*` cgroup
  under the document tools' own root (1 GiB, 64 pids, oom.group, no swap),
  which B2 allows (it forbade the desks aggregate); a shadow that cannot get
  one is counted as not set up and does not run.
- **E3, low/medium: "direct failed, sandbox succeeded" counts as equal.**
  Accepted. `observeShadow` falls through to Equal when the direct run
  failed and the sandbox succeeded. Under required that case changes the
  stored extraction, and the staging gate (step 2) reads only `diverged`,
  `sandbox_failed`, `setup_failed` and `over_budget`. Fix: count it as
  diverged.
- **E4, low: a directory read probe can report a false denial.** Accepted.
  `probeRead` on an empty directory returns EISDIR, and `JudgeProbe`
  accepts any errno but ENOENT as a denial, so the store and signers
  directory probes prove nothing without Landlock. Fix: a directory is read
  with ReadDir (an empty one reads as success), and the judge accepts only
  EACCES or EPERM as a read denial.
- **E5, low: the boot sweep of `t-*` uses os.RemoveAll.** Accepted. On
  cgroupfs a recursive unlink is meaningless and the final rmdir can fail
  while the killed cgroup drains, with no retry. Fix: cgroup.kill, then
  rmdir with the per-run removal's short retry.
- **E6, low: a failed memory.high rise discards the memory.max restore's
  error.** Accepted (`job_memory_linux.go`, `boostJobMemory`). The boost is
  refused, but if `SetMax(old)` also fails, the leaf's ceiling stays raised
  and nothing counts it. Fix: such a raise is retained and counted until the
  capsule ends (as `endJobMemory` already does for a boost that cannot come
  down), and the error says so.
- **E7, docs: the station contradicts the repaired code and cites
  pre-rebase hashes.** Accepted. The design text still describes
  `document_tool_sandbox_fallback` and promises
  `document_tool_sandbox_divergence`, which the code does not emit; "what
  was built" and the outcome table cite commits that are not ancestors of
  the head. Fix: re-point every hash to the replayed SHAs.
- **E8, info.** Accepted. `capfloor.Apply` no-ops on a Mode it does not
  know (closed today: `ModeFromEnv` maps unknown to enforce); it should
  refuse. The helper inherits any fd without close-on-exec; marking every
  fd from 3 up close-on-exec before the tool's exec is cheap defense in
  depth.

### Second batch, review round 2: outcome

Replayed onto origin/main as lane/fp2-b2-on-main (first onto b7d79a082,
then rebased onto a84b39bac with conflicts in imports and the probe
script only). The batch's commits on this head (the superseded copies on
origin/lane/b3-desk-boundary-2 are mapped by `git cherry`):

| commit | subject |
|---|---|
| 9c637ae8b | docs: the designs and failure modes (green) |
| d36a1b78d | feat(capsule): capsule memory per doctrine (red) |
| 911da25ad | feat(doctool): the document-tool sandbox (red) |
| e2acf6025 | feat(capfloor): the capability floor (red) |
| 8cbc30692 | docs: second batch built (green) |
| 78c7b2e4a | docs: review round (bunch 4b) findings (green) |
| 1dc74153f | fix(doctool): portable probe report and judge (orange) |
| c97a577e1 | fix(doctool): observe returns today's run; own cgroup root (red) |
| 62f39cc5f | fix(capsule): the aggregate before any capsule or boost (red) |
| f3fac0bc4 | fix(capfloor): observe never fails the start (red) |
| ac7a10956 | docs: review round repaired (green) |
| 320f85820 | test(capfloor): LD_LIBRARY_PATH for the stand-in daemon (yellow) |

Round 2: problem record d0d49cd7c (green), failing tests 099e5e0b4
(yellow), then the fixes.

| finding | disposition | commit |
|---|---|---|
| E1 CAP_SYS_TIME dropped; hibernate resume needs it (high, blocked enforce) | fixed: kept with the hibernate reason (fast resume stays); floor `00000000032c15ff`; probe constant, image comment, K5 and enforce trial step 4 (a hibernate and resume) updated; source scan test | b1cee0a10 (red) |
| E2 observe shadow has no memory or process bound (medium) | fixed: the shadow runs in a `t-*` cgroup under the document tools' root or not at all; "bounded load" corrected; memory named in the residual. RLIMIT_AS not set (pandoc's GHC runtime reserves a large address space at start; the cgroup bounds memory); RLIMIT_NPROC not set (per uid; pids.max bounds the tool) | a40b6db09 (red) |
| E3 direct failed, sandbox succeeded counted equal (low/medium) | fixed: counted as diverged, so staging step 2's gate sees it | a40b6db09 (red) |
| E4 directory read probe false denial (low) | fixed: a directory is listed, never read; the judge and the probe script take only EACCES or EPERM as a read denial | a40b6db09 (red) |
| E5 boot sweep uses os.RemoveAll (low) | fixed: cgroup.kill (written, never created), then rmdir with the per-run removal's retry; a leftover is logged | a40b6db09 (red) |
| E6 SetMax(old) error discarded (low) | fixed: a raise that cannot be put back is counted until the capsule ends (`stuck:<job>`) and the error and a log line name it | 9d8f1d5c9 (red) |
| E7 docs contradict the code; pre-rebase hashes (docs) | fixed: design text rewritten to the code; every hash re-pointed (tables above; completed in round 3, F2) | 861e765a3 (green) |
| E8 Apply no-ops on an unknown Mode; inherited fds (info) | fixed: Apply refuses an unknown Mode (Linux and elsewhere); the helper marks fds from 3 up close-on-exec before the exec | b1cee0a10, a40b6db09 (red) |

Shipped defaults are unchanged: all three switches observe. Enforce for
the capability floor is no longer blocked by E1 in the code; it still
waits for staging step 4 on a test computer, now including the hibernate
and resume.

Residuals added in round 2:
- The source scan (E1's test) reads the daemon's own Go source for the
  named calls (round 3, F4). It does not see capabilities used by binaries the daemon
  execs (git, go, node, libreoffice) or by syscalls reached through cgo
  libraries; the enforce trial is the evidence for those.
- The shadow's `t-*` cgroup could not be exercised as uid 1000 here (a
  temp dir is not cgroupfs; the test proves the shadow asks for it and
  does not run without it). Staging step 2 is its first real run: a
  shadow whose cgroup cannot be made shows as `setup_failed`, and
  `setup_failed` must be zero before required.

### Second batch, review round 3 (DeepSeek, bunch 43, reviewed b5b33cf43; lane head 861e765a3): findings

Gemini flash accepted. DeepSeek accepts with edits; Devin's review is still
running. The references were checked against 861e765a3 (the rebase moved
only conflict context). Every finding is accepted; fixes follow this
record.

- **F1, medium-low: the tool cgroup fails without a swap controller.**
  Accepted. `newToolCgroup` (`internal/doctool/doctool_linux.go`) writes
  `memory.swap.max` = 0 and treats any write error as a setup failure. On a
  kernel without the swap controller every observe shadow is then
  `setup_failed`, and staging step 2's gate (setup_failed zero) can never
  pass. The desk worker cgroup already tolerates exactly this
  (`internal/yaegikernel/desk_cgroup.go`, "No swap controller means no
  swap to bound"). Fix: ignore ENOENT for `memory.swap.max` only.
- **F2, low: E7 is partly applied.** Accepted. The round-1 outcome still
  says document tools under observe "create no cgroup; the shadow is
  counted only", which E2 contradicts; the problem record
  `docs/problems/desk-boundary-residuals-2026-10-21.md` cites three
  pre-rebase hashes (not ancestors), names the old lane, and one line
  says 15 capabilities. And the rebase onto a84b39bac moved every
  batch-2 SHA again, so the station's tables cite pre-rebase hashes too.
  Fix: re-point every hash to the head's ancestors and check each with
  `git merge-base --is-ancestor`.
- **F3, low: a second stuck boost for one job id leaks the first.**
  Accepted. E6's fix keys the stuck raise `stuck:<job>` and overwrites it,
  while `vmMemoryUsed += bytes` always runs, so a second stuck attempt
  for the same job id leaves the first attempt's bytes counted after
  `returnMemoryBoostsLocked` has returned only the last. Fix: accumulate
  into the existing entry.
- **F4, low: the E1 source scan is a list of spellings.** Accepted. It
  matches `unix.<Name>(` and `syscall.<Name>(` in text: a raw
  `unix.Syscall(unix.SYS_*)`, an aliased import, or a comment evades it,
  and a comment could satisfy its sentinel. Two privileged calls it does
  not name: `cmd/capsule-broker/egress.go` (IoctlIfreq SIOCSIFFLAGS,
  CAP_NET_ADMIN) and `internal/capsule/capsule_release_dir.go` (Fsetxattr).
  Fix: read the source with go/parser (comments are not code; import
  aliases resolve), add IoctlIfreq, the setxattr family, the set*id
  family and raw SYS_* numbers as the first argument of a Syscall, and
  narrow K5's wording to the named calls, the enforce trial being the
  backstop.
- **F5, info: close_range needs no fallback under the Landlock V5 gate.**
  Accepted; a residual line (Landlock V5 needs kernel 6.7, close_range
  with CLOSE_RANGE_CLOEXEC 5.11).

### Second batch, review round 3: outcome

Problem record d5a2eacc9 (green), failing tests eb5671925 (yellow), then
the fixes.

| finding | disposition | commit |
|---|---|---|
| F1 tool cgroup fails without a swap controller (medium-low) | fixed: ENOENT on `memory.swap.max` alone is skipped, as the desk worker cgroup does; any other error fails closed | 40b58db59 (red) |
| F2 E7 partly applied (low) | fixed: the round-1 observe sentence, the problem record's capability count, lane name and hashes; every batch-2 hash in this file and the problem record re-pointed to the head, each checked with `git merge-base --is-ancestor` (the only non-ancestors left are identities, not batch-2 commits: the reviewed heads e2a88e4ee, 5c7615904 and b5b33cf43, the first batch's 5fcf59e21 and bb50792d0, and origin/land/desk-cell-confinement's 9cee80a2d) | this round's outcome commit (green) |
| F3 second stuck boost for one job leaks the first (low) | fixed: the stuck entry accumulates | 9624975f9 (red) |
| F4 the E1 scan is a list of spellings (low) | fixed in the test: go/parser, aliases resolved, comments ignored, dot imports refused, IoctlIfreq, setxattr, set*id and raw SYS_* added, call sentinels; K5 narrowed to "the named calls" | eb5671925 (yellow) |
| F5 close_range needs no fallback (info) | recorded below | this round's outcome commit (green) |

F4, each call the parsed scan finds that the text scan did not. Every
process that makes them runs under the floor: the daemon, and the capsule
broker, which the daemon starts and which inherits its bounding set. None
needs a dropped capability, so none blocks enforce.
- `cmd/capsule-broker/egress.go:32` and `:36`, `unix.IoctlIfreq` on
  "lo" (SIOCGIFFLAGS, then SIOCSIFFLAGS to bring loopback up in a
  capsule's network namespace). The set needs CAP_NET_ADMIN, which is
  kept and is in the broker's ambient set with egress.
- `internal/capsule/capsule_release_dir.go:57`, `unix.Fsetxattr` of
  `user.overlay.opaque` (the const at `:16`; the review said
  trusted.overlay.opaque, but the code writes the user.* form, which
  MountOverlayFS mounts with). A user.* xattr needs only ownership of the
  file or CAP_FOWNER; the test still counts the setxattr family under
  CAP_SYS_ADMIN, the strongest need (for trusted.*), and both are kept.
  Run by the daemon's capsule executor, in process.
The text scan already saw the rest (Settimeofday, Mount, Unmount); no raw
SYS_* call with a privileged number and no set*id wrapper call exists in
the scanned source.

Residual added in round 3:
- F5: `close_range(3, ~0, CLOSE_RANGE_CLOEXEC)` in the helper has no
  fallback; it needs Linux 5.11, and the sandbox already refuses a kernel
  without Landlock V5 (6.7), so no kernel that runs the sandbox lacks it.
