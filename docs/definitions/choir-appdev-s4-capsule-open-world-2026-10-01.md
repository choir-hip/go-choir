---
definition_version: 4
definition_id: choir-appdev-s4-capsule-open-world-2026-10-01
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
        On staging, run a capsule build that fetches a nixpkgs dependency via
        curl | bash; inspect its trajectory and content store after the build.
      proves: >-
        Every fetch records URL, content hash, redirect chain, and connected
        address, and captured payloads support rebuilding when the origins are
        unavailable.
      evidence_class: deployed proof
    - action: >-
        On staging, dispose the capsule after an uncommitted build, then attempt
        host, private, loopback, link-local, and tap-range connections from a
        new capsule and on redirect targets.
      proves: >-
        Uncommitted capsule state is gone and connect-time policy rejects every
        prohibited address on the original request and every redirect.
      evidence_class: deployed proof
    - action: >-
        Inspect the capsule process environment and perform the same build using
        substitution through the egress proxy.
      proves: >-
        Provider credentials never enter the capsule and opaque or direct
        protocol bypasses are denied.
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
  protected_surfaces: [VM networking / tap forwarding / capsule egress]

now:
  status: checkpoint_incomplete
  slice: >-
    Pending S1 security floor: define and prove recording capsule egress and
    one S0b-selected private-store mechanism only after S1 closes the VM/tap
    floor.
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
      Deployed curl | bash/nixpkgs build records every URL, hash, redirect, and
      connected address; an origin-unavailable rebuild succeeds from captured
      payloads; disposal removes uncommitted state; network and credential
      negative proofs pass.
    edge: missing_oracle
    delta_o: >-
      S0b's disposable-computer mount, Nix DB, sandbox, and closure-resolution
      probes select one mechanism; staging trajectory, content-store, and
      connect/redirect negative-probe receipts observe the remaining claims.
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
      The S0b disposable-computer probe of those four falsifiers, followed by
      the S1 completion receipt that makes this egress work admissible.
  blocker_or_risk: >-
    S1-security-floor is incomplete. Current host tap setup explicitly enables
    forwarding and masquerading, including guest access to host services
    (internal/vmmanager/manager.go:2686-2695,2707-2753); enabling S4 first
    would widen an unclosed protected surface.
  next_action: promotes to working when S1-security-floor complete.

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
