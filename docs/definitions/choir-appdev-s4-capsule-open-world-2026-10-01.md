---
definition_version: 4
definition_id: choir-appdev-s4-capsule-open-world-2026-10-01
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
      "Capsules are air-gapped (empty netns, internal/capsule/namespace.go);
      the guest Nix store is read-only EROFS with no nix on the runtime PATH.
      curl | bash and nixpkgs are impossible inside a capsule today."
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:112-115)
    - >-
      "Guest VM egress is open and tap->tap forwarding appears permitted
      (internal/vmmanager/manager.go setupHostNetworking; host firewall does
      not filter FORWARD). The guest runtime unit runs as unconfined root."
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:124-128)
    - >-
      Capsule creation currently builds an overlay root and launches a broker
      in new user, mount, network, UTS, IPC, and cgroup namespaces
      (internal/capsule/executor.go:191-193,335-375,401-424).

finish:
  deliver: >-
    A capsule can build an app using declared public network protocols,
    including curl | bash and a nixpkgs package, while every fetched byte is
    recorded and replayable without its origin and no capsule gains a path to
    the host, private networks, taps, or provider credentials.
  artifact: >-
    A deployed capsule egress broker bridge with trajectory fetch records and
    content-addressed payload capture, plus one S0b-proven capsule-private Nix
    store mechanism: either a writable capsule-private upper or a Nix
    local-overlay store. The broker owns the capsule netns and drops
    capabilities before capsule execution; substitutions use the same proxy.
  acceptance:
    - action: >-
        On staging, run a capsule workload whose declared public source is
        fetched by curl | bash through the proxy; inspect its fetch trajectory.
      proves: >-
        The curl | bash fetch records its URL, content hash, redirect chain,
        and connected address through the declared protocol path.
      evidence_class: deployed proof
    - action: >-
        On staging, run a separate capsule workload that builds one nixpkgs
        dependency through the proxy and its selected private-store mechanism.
      proves: >-
        A nixpkgs dependency can build in the selected capsule-private store
        without a guest-global writable Nix store or direct substitution path.
      evidence_class: deployed proof
    - action: >-
        Stage a workload after its curl | bash and nixpkgs inputs are captured,
        make the original fetch origins unavailable, and rebuild that workload
        from the captured content-addressed payloads.
      proves: >-
        The captured payloads, rather than origin availability, reproduce the
        staged workload's declared fetch inputs.
      evidence_class: deployed proof
    - action: >-
        On staging, from the workload attempt direct-IP and DNS-rebinding
        connections to host, private, loopback, link-local, and tap ranges;
        attempt redirect-target, opaque-protocol, and direct egress bypasses.
      proves: >-
        Connect-time policy rejects every prohibited address on the original
        request and every redirect, while direct and opaque bypasses fail.
      evidence_class: deployed proof
    - action: >-
        Seed a unique uncommitted workspace and private-store marker, dispose
        the capsule, then search the disposed capsule state; also attempt to
        read gateway-token and provider-credential material from the workload's
        environment, mounts, and process-visible state.
      proves: >-
        Disposal retains no uncommitted capsule state, provider credentials and
        the gateway token are unreadable to the workload, and the selected
        private-store boundary leaves no reusable capsule authority.
      evidence_class: deployed proof
    - action: >-
        Inspect the workload's effective capability state and mount table after
        broker launch, then attempt a privileged operation from the workload.
      proves: >-
        The broker drops capabilities before capsule execution and the selected
        writable store is isolated to that capsule.
      evidence_class: deployed proof
  rollback: >-
    Git revert and redeploy the broker, namespace, store, and policy change;
    dispose affected capsules and retain their trajectory/content receipts.
    Refuse rather than enable egress if the selected S0b mechanism or its
    isolation proof fails.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the gap between a capsule that must be offline and one that can
    reproducibly consume declared public build inputs, while preserving
    non-reachability of host, private, loopback, link-local, and tap networks
    and keeping all provider credentials outside the capsule.
  goodharting_would_be: >-
    Opening guest egress, logging only request URLs, trusting DNS before
    connect, or retaining a payload without its redirect and connected-address
    evidence; each makes curl appear to work without proving mediated,
    replayable, non-private access.

homotopy:
  realism_axis: >-
    One declared fetch through the proxy, then redirect and address-policy
    challenges, then curl | bash plus one nixpkgs build with captured inputs,
    then an origin-unavailable rebuild and disposal proof using the same broker
    and store boundary.

boundaries:
  mutation_class: red
  authority_sources:
    - owner-ratified metamission S4 intent (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:560-573)
    - owner approval of the metamission after consensus review (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:375-379)
    - docs/computer-ontology.md:39-50,85-105
  must_preserve:
    - Capsules own no semantic state; only an accepted event changes desired code.
    - Owner approval (or a stronger declared consensus policy) gates every commit.
    - No guest-side writable Nix store or daemon outside a capsule boundary.
    - Provider credentials never enter a capsule.
    - The non-forkable base (kernel, capsule broker, updater, signers, network policy) stays platform-owned and identical across computers on the same image.
    - Publication is source plus pinned inputs; no cross-tenant binary execution.
  excluded:
    - S1-security-floor's per-tap FORWARD isolation, default-deny VM egress, runtime confinement, and gateway-token scrubbing.
    - S2-layering-runtime-from-release's guest base/app-layer closure and updater materialization.
    - S5-live-preview-supervision's desktop preview bridge and Texture streaming.
    - S6-commit-gate-full-release's test and owner-approval materialization gate.
    - S8-source-publication's host registry and publication/adoption flow.
  protected_surfaces:
    - VM networking / tap forwarding / capsule egress
    - broker capability dropping before capsule execution
    - capsule-private Nix store isolation
  heresy_delta:
    discovered:
      - >-
        Existing host networking permits tap forwarding and masquerading, while
        capsules currently have no recorded egress or usable Nix store
        (internal/vmmanager/manager.go:2686-2753;
        docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:112-115).
    introduced: []
    repaired:
      - none; this station remains checkpoint_incomplete pending S1 and S0b.

now:
  status: checkpoint_incomplete
  slice: >-
    Pending S1 security floor and S0b mechanism-decision receipt: define and
    prove recording capsule egress and exactly one S0b-selected private-store
    mechanism only after both gates close.
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
    id: recorded-egress-private-store
    claim: >-
      A broker-owned netns bridge can permit only declared public protocols and
      capture their exact fetch inputs while a single capsule-private Nix store
      mechanism supports nixpkgs builds without widening guest or host access.
    test: >-
      Deployed curl | bash and nixpkgs builds separately record every URL, hash,
      redirect, and connected address; an origin-unavailable rebuild succeeds
      from captured payloads; disposal, capability, network, and credential
      negative proofs pass.
    edge: missing_oracle
    delta_o: >-
      S0b's disposable-computer mount, Nix DB, sandbox, and closure-resolution
      probes select one mechanism; staging trajectory, content-store, capability,
      and connect/redirect negative-probe receipts observe the remaining claims.
    scope_if_supported: >-
      Capsule builds on a staging Choir computer through the declared egress
      broker, not general guest egress or cross-computer publication.
    status: proposed
    evidence_refs:
      - docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:512-518
      - docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:560-573
  decision:
    what: >-
      Select exactly one capsule-private Nix store mechanism only from the S0b
      probe result; do not ship both a writable upper and a Nix local-overlay
      store.
    kind: architecture
    status: proposal
    evidence_ref: docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:566-573
    owner_ratification_ref: docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:375-379
  belief:
    believed_state: >-
      Existing capsules are offline namespaces with an overlay root; their
      broker currently starts with a deliberately minimal environment
      (internal/capsule/executor.go:191-193,335-375,401-424).
    main_uncertainty: >-
      Whether the EROFS lower has a usable Nix DB, a user namespace can mount
      an overlay at /nix/store, Nix sandbox nesting works, and closure paths
      still resolve after capsule disposal.
    next_observation: >-
      The S0b disposable-computer probe of the four falsifiers and its
      mechanism-decision receipt, followed by the S1 completion receipt that
      makes this egress work admissible.
  blocker_or_risk: >-
    S1-security-floor is incomplete. Current host tap setup explicitly enables
    forwarding and masquerading, including guest access to host services
    (internal/vmmanager/manager.go:2686-2695,2707-2753); enabling S4 first
    would widen an unclosed protected surface.
  next_action: >-
    Promotes to working when S1-security-floor completes and S0b records the
    selected capsule-private Nix store mechanism.

receipts: []
---

# Capsule Open World

## Mechanism sketch

- The egress proxy is the sole network bridge into each capsule netns. It
  declares the protocols it carries; direct and opaque bypasses fail closed.
- A fetch receipt binds the requested URL, every redirect, the connected
  address, and the content hash to the capsule trajectory. The response body
  is retained in content-addressed storage for S8's fixed-input rebuild.
- Address policy is evaluated after resolution at every connection, then again
  for every redirect target. Private, loopback, link-local, host, and tap
  ranges never become proxy destinations.
- The broker, rather than capsule code, retains netns authority. Its current
  launcher creates a new network namespace and currently carries ambient setup
  capabilities, so capability dropping before capsule execution is a required
  proof obligation, not an assumed property (internal/capsule/executor.go:401-424).
- Existing workload seccomp admits AF_UNIX sockets only
  (internal/capsule/seccomp.go:38-56); the bridge must preserve an equivalent
  no-direct-networking boundary rather than weaken the capsule filter.

## Store decision and falsifiers

- S0b selects exactly one: a capsule-private writable upper or the Nix
  local-overlay store. No dual implementation is a fallback or completion
  path (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:566-573).
- The selection probe must falsify or support: absent Nix DB on the EROFS
  lower; inability to mount an overlay over `/nix/store` in the capsule user
  namespace; nested Nix sandbox failure; and unresolved closure paths after
  disposal (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:570-572).
- Capsule destruction already terminates the namespace leader, unmounts the
  overlay, removes the private tree on successful cleanup, and revokes capsule
  capabilities (internal/capsule/executor.go:484-550). S4 must prove that this
  lifecycle also removes every uncommitted store artifact.

## What changes hands

The proxy hands S8 content-addressed fetch inputs and their trajectory evidence,
not an origin-dependent build or a binary. The private store hands S6 only a
frozen candidate's build outputs through the existing acceptance boundary; it
never becomes guest-global Nix state. These limits preserve the ontology that a
capsule effect bundle is a frozen speculative candidate and an accepted event is
the only desired-code transition (docs/computer-ontology.md:85-105).
