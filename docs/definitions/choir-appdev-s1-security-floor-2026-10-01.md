---
definition_version: 4
definition_id: choir-appdev-s1-security-floor-2026-10-01
execution_mode: mission_orchestrator
readiness: drafted
member_of: choir-supervised-app-development-metamission-2026-10-01

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: '2026-10-01T00:00:00Z'
  source:
    canonical_ref: main@8aa1dce9
    deploy_identity: 'staging https://choir.news deployed_commit=a3cfaa00;
      owner guest computer-03335285269bdba4f94377e56879f9e6 on a3cfaa00'
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed_baseline:
    - >-
      Guest VM egress is open and tap->tap forwarding appears permitted
      (internal/vmmanager/manager.go setupHostNetworking; host firewall
      does not filter FORWARD). The guest runtime unit runs as unconfined
      root. Unverified on staging. See
      docs/reports/nixos-agent-platform-redhat-deepseek-audit-2026-10-01.md.
    - >-
      setupHostNetworking enables forwarding and appends unqualified FORWARD
      ACCEPT rules for traffic entering and leaving each tap
      (internal/vmmanager/manager.go:2697,2726-2732); it also MASQUERADEs
      outbound traffic not addressed to that guest subnet
      (internal/vmmanager/manager.go:2747-2753).
    - >-
      The runtime imports /run/go-choir-autoputer.env
      (nix/autoputer-vm.nix:751-764), whose producer writes
      RUNTIME_GATEWAY_TOKEN from choir.gateway_token and makes the file 0640
      (nix/autoputer-vm.nix:256-258,344-345,368). Its shown serviceConfig
      has no User or ProtectSystem setting (nix/autoputer-vm.nix:751-764).
    - >-
      The capsule broker starts a Yaegi session worker with the default safe
      package list (cmd/capsule-broker/main.go:113-137). That list permits a
      small stdlib set plus choir, while os, os/exec, net and net/http are
      explicitly banned (internal/yaegikernel/allowlist.go:8-42,76-95).

finish:
  deliver: >-
    Each deployed guest is isolated from every other guest and from unapproved
    egress, while the guest runtime and every capsule child operate without an
    inherited gateway token or ambient Yaegi escape path.
  artifact: >-
    A deployed guest security floor: per-tap host FORWARD policy, default-deny
    VM egress with explicit allowlist, a confined non-root
    go-choir-autoputer unit, token-scrubbed child execution, and the enforced
    restricted Yaegi worker import/kernel boundary.
  acceptance:
    - action: >-
        On staging, start two distinct guests and attempt a TCP connection from
        guest A to guest B's tap-addressed runtime port; the connection returns
        connection refused while each guest's own approved service path remains healthy.
      proves: Per-tap FORWARD policy prevents guest-to-guest reachability.
      evidence_class: deployed proof
    - action: >-
        From a staging guest, attempt TCP egress to a non-allowlisted public
        destination and to a host-private destination; neither connection succeeds,
        while each explicitly allowlisted guest dependency remains reachable.
      proves: VM egress is default-deny rather than merely masqueraded.
      evidence_class: deployed proof
    - action: >-
        Inspect the deployed go-choir-autoputer process identity and systemd
        confinement, then attempt the denied filesystem operation from that
        runtime identity; it is non-root and the confinement refuses the operation.
      proves: The runtime unit does not retain root-wide guest authority.
      evidence_class: deployed proof
    - action: >-
        Run a normal capsule child on staging and capture its environment names;
        RUNTIME_GATEWAY_TOKEN is absent while the child remains able to use only
        its intended broker-mediated capabilities.
      proves: The gateway token is not inherited across the runtime-to-child boundary.
      evidence_class: deployed proof
    - action: >-
        Submit a staging Yaegi worker cell importing os, os/exec, net, or net/http;
        each import is refused before use, and the worker remains inside the capsule
        execution boundary.
      proves: The Yaegi worker kernel floor rejects ambient OS and network authority.
      evidence_class: deployed proof
  rollback: >-
    Git-revert and redeploy the security-floor change to the prior guest image;
    preserve the before/after firewall and acceptance receipts, and use the
    existing pinned-head restore only for a computer state affected during proof.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the authority gap between an untrusted guest or capsule child and
    another tenant, host-private service, or gateway credential while preserving
    each guest's approved runtime and broker-mediated paths.
  goodharting_would_be: >-
    Adding an allowlist-looking rule while the broad tap FORWARD ACCEPT or
    outbound MASQUERADE path still carries traffic, proving only unit metadata,
    or hiding the token from a parent process while a capsule child inherits it.

homotopy:
  realism_axis: >-
    The same enforcement boundary at increasing resolution: static rule/unit and
    Yaegi-policy inspection, one guest's denied egress, two live guests' denied
    tap connection, then a capsule child and Yaegi worker exercised on staging.

boundaries:
  mutation_class: red
  authority_sources:
    - owner-ratified metamission 2026-10-01
    - docs/choir-doctrine.md
    - docs/computer-ontology.md
    - AGENTS.md (red-surface ceremony and problem-documentation-first)
  must_preserve:
    - Capsules own no semantic state; only an accepted event changes desired code.
    - Owner approval (or a stronger declared consensus policy) gates every commit.
    - Every committed change is restorable through the pinned-head path.
    - No guest-side writable Nix store or daemon outside a capsule boundary.
    - The non-forkable base (kernel, capsule broker, updater, signers, network policy) stays platform-owned and identical across computers on the same image.
    - Provider credentials never enter a capsule.
    - A fork never holds its parent's identity material.
  excluded:
    - S0 reality and boot-timeline measurement and disposable experiments.
    - S2 guest base/app-layer materialization and release activation.
    - S3 snapshot lifecycle and resume policy.
    - S4 recording proxy, open-world capsule egress, and capsule-private Nix store.
    - S9 fork construction, re-keying, and fleet admission.
  protected_surfaces:
    - VM networking / tap forwarding

now:
  status: checkpoint_incomplete
  slice: >-
    Pending S1 security floor after S0 establishes the staging networking,
    token-visibility, and runtime reality receipts.
  source_ref: main@8aa1dce9
  deploy_identity: 'staging https://choir.news deployed_commit=a3cfaa00; owner guest computer-03335285269bdba4f94377e56879f9e6 on a3cfaa00'
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: security-floor-enables-open-world-and-forks
    claim: >-
      Per-tap deny-by-default forwarding, VM egress allowlisting, a confined
      non-root runtime, child token scrubbing, and the restricted Yaegi worker
      kernel jointly remove the ambient authority that would otherwise make S4
      capsule egress and S9 sibling computers unsafe to operate.
    test: >-
      The deployed negative network, runtime-identity, child-environment, and
      Yaegi-import proofs in finish.acceptance all pass while approved paths remain healthy.
    edge: missing_oracle
    delta_o: >-
      Staging receipts that capture both sides of the two-guest connection,
      actual guest egress results, systemd/process confinement, child environment
      names, and Yaegi import refusals.
    scope_if_supported: >-
      Staging guests and their capsule children on the current single-host
      Firecracker deployment; not a claim about a future multi-host topology.
    status: proposed
    evidence_refs:
      - internal/vmmanager/manager.go:2697-2753
      - nix/autoputer-vm.nix:256-258,344-345,368,658-764
      - cmd/capsule-broker/main.go:113-137
      - internal/yaegikernel/allowlist.go:8-95
  decision:
    what: >-
      Establish the security floor before S4 capsule open-world work or S9 fork
      and fleet work, using deployed negative proofs rather than source inspection.
    kind: architecture
    status: settled
    evidence_ref: docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:519-522
    owner_ratification_ref: docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:375-379
  belief:
    believed_state: >-
      The checked source exposes broad tap forwarding and egress, imports the
      gateway token into the runtime, and has not yet demonstrated a confined
      non-root runtime or token-scrubbed child boundary on staging.
    main_uncertainty: >-
      The exact deployed firewall and service behavior, including whether the
      desired negative proofs can be achieved without breaking approved guest paths.
    next_observation: >-
      S0's staging tap reachability, gateway-token visibility, and runtime-layout
      receipts, followed by a scoped security-floor problem document if they confirm the hazards.
  blocker_or_risk: >-
    S4 and S9 must not execute against the current assumed guest boundary; an
    over-broad fix could sever the gateway or other approved guest services.
  next_action: promotes to working when S0-reality-and-boot-timeline complete.

receipts: []
---

# S1 Security Floor

## Mechanism sketch

The host policy must make each tap a tenant boundary, not merely an address.
A guest may reach only explicitly declared host services and egress targets;
all guest-to-guest forwarding and every unlisted route must reject before it
becomes a usable path. The broad per-interface ACCEPT rules currently appear at
`internal/vmmanager/manager.go:2726-2732`; the general outbound MASQUERADE
appears at `internal/vmmanager/manager.go:2747-2753`.

The guest runtime must run as a dedicated non-root identity with an explicit
systemd confinement envelope. Its gateway credential is needed only at the
trusted runtime boundary: child-launch construction must remove it rather than
relying on child convention. The present extraction and import chain is
`nix/autoputer-vm.nix:256-258,344-345,368,751-764`.

Yaegi is an orchestration surface, not a sandbox substitute. The worker keeps
its existing restricted import floor — the broker selects the default safe list
at `cmd/capsule-broker/main.go:113-137`, and the kernel refuses ambient OS,
process, and network imports at `internal/yaegikernel/allowlist.go:29-42,86-95`.
The runtime and capsule containment must make that floor meaningful even when
model-authored code is adversarial.

## Risk and handoff

S0 supplies the factual staging receipt; if it confirms a hazard, document the
problem before the repair as required by `AGENTS.md:170-189`. The implementation
then lands through commit, CI, staging deployment, identity verification, and
the negative product proofs. A failure of an approved path is a security-policy
design defect to repair, not evidence that broad forwarding is acceptable.

S4 receives a closed guest boundary on which it can add recorded capsule egress.
S9 receives the same boundary before a sibling computer can exist. Neither
station inherits an unproved exception to this station's policy.
