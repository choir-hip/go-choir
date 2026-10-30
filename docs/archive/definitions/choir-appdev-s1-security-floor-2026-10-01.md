---
definition_version: 4
definition_id: choir-appdev-s1-security-floor-2026-10-01
execution_mode: mission_orchestrator
readiness: reviewed
member_of: choir-supervised-app-development-metamission-2026-10-01

review:
  reviewer: 'agentic-consensus authoring panel (devin, codex, claude,
    omp-gpt6-sol, omp-gpt6-luna, omp-gemini38, omp-glm53-flash) - send-back
    round resolved'
  frozen_ref: 'main@653d975c'
  verdict: accept
  evidence_ref: '.agentic-consensus/agentic-consensus-20261001-135404/'

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
      does not filter FORWARD). The guest runtime defaults to root because
      its shown serviceConfig has no User, while its ReadWritePaths and
      InaccessiblePaths impose only listed path restrictions. Staging behavior
      is unverified. See
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
      (nix/autoputer-vm.nix:256-258,344-345,368). The shown serviceConfig
      lacks User and ProtectSystem but does set ReadWritePaths and
      InaccessiblePaths (nix/autoputer-vm.nix:751-764).
    - >-
      The capsule broker starts a Yaegi session worker with the default safe
      package list (cmd/capsule-broker/main.go:113-137). That list permits a
      small stdlib set plus choir, while os, os/exec, net and net/http are
      explicitly banned (internal/yaegikernel/allowlist.go:8-42,76-95).

finish:
  deliver: >-
    Each deployed guest is isolated from every other guest and from unapproved
    egress, while runtime children and capsule children inherit no gateway token
    or ambient Yaegi escape path; the trusted runtime retains only its own
    required credential.
  artifact: >-
    A deployed guest security floor: per-tap host FORWARD policy, default-deny
    VM egress with explicit allowlist, a confined non-root
    go-choir-autoputer unit, token-scrubbed child execution, and the enforced
    restricted Yaegi worker import/kernel boundary.
  acceptance:
    - action: >-
        On staging, start two distinct guests and verify guest B's
        tap-addressed runtime port has a healthy listening destination. Attempt
        a TCP connection from guest A: it is refused or times out with no
        established connection, receiver-side evidence records no A connection,
        and the host records the matching FORWARD deny while each guest's own
        approved service path remains healthy.
      proves: Per-tap FORWARD policy, rather than a closed destination port, prevents guest-to-guest reachability.
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
        Run a normal runtime child and capsule child on staging; capture their
        environment names and attempt reads of the gateway-token file path and
        relevant process surfaces. RUNTIME_GATEWAY_TOKEN is absent and neither
        child can recover the token, while each retains only its intended
        broker-mediated capabilities.
      proves: Runtime children and capsule children do not inherit or recover the gateway token.
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
    or hiding the token from an environment listing while a child can read it
    from a mounted file or process surface.

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
    - runtime confinement
    - gateway-token handoff
    - Yaegi worker boundary

heresy_delta:
  discovered:
    - >-
      Source inspection indicates broad tap forwarding and VM egress; the
      runtime defaults to root and receives the gateway token, while only
      path restrictions are presently visible. Staging confirmation is pending S0.
  introduced: []
  repaired:
    - none; this station remains checkpoint_incomplete pending S0.

now:
  status: working
  slice: >-
    S1a host-boundary hotfix COMPLETE on deployed evidence: network
    isolation + authority binding deployed (a3f0d48e) and the refusal
    matrix passed on two disposable accounts (d37408ee,
    docs/evidence/s1a-refusal-matrix-2026-10-04.json — 5/5 refusals,
    5/5 legitimate flows green). S0b resumes next; the rest of S1
    (runtime identity, token scrubbing, capsule/yaegi floor) remains.
    Landed 2026-10-04 (deploy pending): gateway token off the kernel
    cmdline onto the root-only credential disk with RUNTIME_GATEWAY_TOKEN_FILE
    resolution (619d6458); zot PATH-shadowing fallback removed (a80d2146);
    diag tcp-dial oracle restricted to the host peer (98cf3875).
  source_ref: main@98cf3875
  deploy_identity: 'staging https://choir.news deployed_commit=b15f012a (S1a network+authority + diag oracle live)'
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
      The deployed negative network, runtime-identity, token-recovery, and
      Yaegi-import proofs in finish.acceptance pass while approved paths remain healthy.
    edge: missing_oracle
    delta_o: >-
      Staging receipts that capture a verified listener, both sides of the
      two-guest connection, the observed FORWARD deny, actual guest egress
      results, systemd/process confinement, child token-read attempts, and
      Yaegi import refusals.
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
      gateway token into a runtime that defaults to root with only the shown
      ReadWritePaths and InaccessiblePaths restrictions, and has not demonstrated
      a confined non-root runtime or token-scrubbed child boundary on staging.
    main_uncertainty: >-
      The exact deployed firewall and service behavior, including whether the
      desired negative proofs can be achieved without breaking approved guest paths.
    next_observation: >-
      S0's staging tap reachability, gateway-token visibility, and runtime-layout
      receipts, followed by a scoped security-floor problem document if they confirm the hazards.
  blocker_or_risk: >-
    S4 and S9 must not execute against the current assumed guest boundary; an
    over-broad fix could sever the gateway or other approved guest services.
  next_action: S1a closed on the refusal-matrix receipt; the S1a→S0b transition is recorded in receipts below. The rest of S1 (runtime identity, token scrubbing, capsule/yaegi floor) continues as the S0b disposable-computer probe suite lands its findings.

receipts:
  - id: s1a-to-s0b-transition-2026-10-05
    kind: station_transition
    status: closed
    landed: S1a-host-boundary-hotfix
    next: S0b (resumes its disposable-computer probe suite)
    closed_at: '2026-10-05T00:40:00Z'
    boundary_evidence: >-
      S1a closed on the deployed refusal matrix
      (docs/evidence/s1a-refusal-matrix-2026-10-04.json, deploy b15f012a):
      5/5 refusals (tap->tap FORWARD drop, vmctl internal 403, maild
      forged-owner 403, corpusd mint 405, proxy publish 405) + 5/5
      legitimate flows green (gateway dial, bound-owner CV resolve,
      bound-owner maild read, egress dial, product page). Token moved off
      the kernel cmdline to the root-only credential disk
      (RUNTIME_GATEWAY_TOKEN_FILE, 619d6458); zot PATH-shadowing removed
      (a80d2146); diag oracle restricted to the host peer (98cf3875).
    panel: >-
      Reused the S0b boundary panel
      (.agentic-consensus/s0b-boundary-panel-20261004) rather than a fresh
      panel: the transition is a closed-receipt hand-off, not a new
      candidate gate — S0b resumes an in-flight probe suite, it does not
      land a new candidate.
    residual: >-
      S1 remainder stays open: confined non-root runtime identity, child
      token scrubbing, and the restricted Yaegi/capsule worker floor
      (docs/problems/s1-non-root-runtime-child-uid-boundary-2026-10-04.md).
      The owner computer's stale-active mark (candidate-fleet-e15cb89f,
      last_active bumped by a deploy refresh on a dead VM) is filed under
      docs/problems/s0-storage-lifecycle-gaps-2026-10-05.md, not a reaper
      target.
  - id: s1a-refusal-matrix-2026-10-04
    kind: deployed_acceptance
    status: closed
    closed_at: '2026-10-04T05:35:00Z'
    deploy_identity: 'b15f012acab6e2e9d947c90d22739de947dfcee6 (S1a network+authority + diag mode=http oracle live on staging)'
    evidence: docs/evidence/s1a-refusal-matrix-2026-10-04.json
    note: >-
      Two disposable accounts (A=computer-6450a253/10.200.220.2,
      B=computer-cbcce1fa/10.200.221.2). All guest-originated through the
      host-sourced diag oracle: R1 tap->tap FORWARD drop (connect timeout),
      R2 vmctl internal 403, R3 maild forged owner 403, R4 corpusd mint
      405 (loopback-only bypass), R5 proxy publish 405. Legitimate:
      L1 gateway dial, L2 bound-owner CV resolve 200, L3 bound-owner
      maild read 200, L4 egress dial, L5 product page 200.
    heresy: >-
      discovered: guest 'zot' resolves to the third-party TUI agent —
      management-console spawn PATH-shadowed, no in-guest exec surface
      (problem-doc addendum). introduced: none beyond the oracle itself
      (host-sourced-only, GET, header-allowlisted).
---

# S1 Security Floor

## Mechanism sketch

The host policy must make each tap a tenant boundary, not merely an address.
A guest may reach only explicitly declared host services and egress targets;
all guest-to-guest forwarding and every unlisted route must deny before it
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
## S1a slice — host-boundary hotfix (pulled ahead of S0b, 2026-10-04)

Scoped by `docs/problems/s0-guest-reaches-host-internal-authority-2026-10-04.md`
and the deployed correction
`docs/evidence/s1a-guest-to-host-addr-preservation-2026-10-04.json`:
guest-originated packets arrive at host INPUT post-DNAT with the **real
guest tap IP** (`SRC=10.200.207.2 DST=127.0.0.1` observed); the `lo`
MASQUERADE rules provably never fire. RemoteAddr is therefore a usable
tap-sourced discriminator.

### Design (mutation class: red — VM networking, vmctl, corpusd mint, maild, guest /internal surface)

1. **Network (vmmanager `setupHostNetworking`).**
   - Drop tap->tap forwarding: per-tap FORWARD DROP for traffic whose
     destination is any other tap's /30 or the 10.200.0.0/16 guest space,
     placed before the per-tap ACCEPTs.
   - Anti-spoofing: drop packets entering a tap whose source is not the
     guest's own /30 address. A guest is root; without this it can emit
     `SRC=10.200.x.1` (host peer IP) or another guest's IP and defeat every
     IP-bound authority check below.
   - Remove `:8085` from `tapReachableHostServicePorts()` — no production
     host autoputer exists; the rule only lets guests reach a dead port.
   - Delete the dead `-o lo -j MASQUERADE` rules (0 packets ever; misleading).
   - Guest->internet egress stays open in S1a (DNS/NTP/resolution paths not
     fully resolved; S1's recording-proxy default-deny owns egress policy).
   - Reconcile: the drop/anti-spoof set applies to **existing** taps at
     service start (reconcile pass per tap device), not only on new-tap
     create — otherwise already-running guests stay open until reboot.
   - RemoteAddr evidence leg (cheap, lands first): log RemoteAddr on one
     host service for one guest-originated request to confirm the
     loopback-leg claim (S0a `x-choir-remote-addr` trace already recorded
     `SRC=10.200.207.2` post-DNAT; this verifies it at the socket layer).
   - The NixOS `networking.firewall.filterForward` flag is NOT used — it
     would kill guest->internet too; vmmanager emits the drops directly.
   - Cleanup: stale `vm-vm-*-tap` iptables rules for deleted devices are
     left alone (inert); new rules land on current taps at boot/refresh.
2. **Authority (bind identity to transport, delete header trust for
   tap-sourced callers).** Legitimate flows identified by inventory:
   - **:8084 gateway** — bearer per-VM token + peer check; already sound.
     No change beyond what token scrubbing does in S1.
   - **:8086 corpusd** — event/file CAS already uses per-computer Bearer
     capabilities (keep). The `X-Internal-Caller` host-read bypass and the
     platform-update mint become **loopback-only** (RemoteAddr must be
     `127.0.0.1`); header alone can no longer satisfy them.
     The guest self-dev corpusd mint (platform-control calls a guest's own
     self-development run may issue) stays a credentialed guest operation,
     not the bypass: bound to the calling computer's capability, never to
     `X-Internal-Caller`.
   - **:8083 vmctl** — `isInternalCaller` tightens to loopback RemoteAddr
     **except** a guest-scoped route family: the four ComputerVersion
     endpoints (`computer-version-inputs/resolve`,
     `computer-version-routes/{resolve,apply-self-development,
     apply-platform-follow}`) accept tap-sourced requests bound by source
     IP to the owning computer (vmctl owns tap IP -> computer/owner
     mapping; anti-spoofing rule makes the binding sound). Every other
     vmctl internal endpoint refuses non-loopback callers outright.
   - **:8087 maild** — tap-sourced requests bound by source IP to the
     owning computer; `X-Authenticated-User` must equal that computer's
     owner. `X-Internal-Caller` grants nothing from a guest source.
   - **:8082 proxy** — guest wire/platform publish paths accept tap-sourced
     requests bound by source IP to computer identity (the publish's
     computer must be the caller's); other proxy internal surfaces refuse
     non-loopback.
   - **:8787 source service** — gains a caller floor: tap-sourced requests
     bound to a known computer's guest IP (or loopback); unknown sources
     refused.
   - **Guest `/internal/runtime/*` and `/internal/diag/*`** — accept only
     host-sourced requests (RemoteAddr = the tap's host peer IP; anti-
     spoofing rules make that unforgeable from the guest side). `X-
     Internal-Caller` stops being the credential; it may stay as a marker.
   - **Guest user routes** (`X-Authenticated-User`) — honored only when
     RemoteAddr is the host peer IP (proxy-authenticated transport); a
     guest-asserted owner header is ignored.
3. **Verification (deployed, two disposable computers).**
   - Pre/post `iptables-save` per tap recorded.
   - From guest A: tap->tap to guest B :8085 refused/times out; spoofed-
     source packets dropped; :8083 vmctl non-CV endpoints refused;
     corpusd mint refused; maild with a forged other-owner
     `X-Authenticated-User` refused; proxy publish naming another
     computer refused; source-service from bound IP works.
   - Legitimate flows verified green post-deploy: gateway inference
     (bearer), corpusd event CAS (capability), maild drafts (bound owner),
     guest->vmctl CV-route resolve (bound computer), wire publish (bound),
     source-service search, and a product user route through the proxy.

### Rollback

`git revert` + redeploy; rules are per-tap at boot/refresh so refresh
restores the open set. Authority checks revert with the binary.

### Heresy delta for this slice

- discovered: none expected beyond the problem doc (loopback leg already
  corrected).
- introduced: none intended; the fix removes authority paths, adds no
  credential surface (reuses IP binding + existing capability tokens).
- repaired: the entire cross-tenant chain in the problem doc.


## Risk and handoff

S0 supplies the factual staging receipt; if it confirms a hazard, document the
problem before the repair as required by `AGENTS.md:170-189`. The implementation
then lands through commit, CI, staging deployment, identity verification, and
the negative product proofs. A failure of an approved path is a security-policy
design defect to repair, not evidence that broad forwarding is acceptable.

S4 receives a closed guest boundary on which it can add recorded capsule egress.
S9 receives the same boundary before a sibling computer can exist. Neither
station inherits an unproved exception to this station's policy.
