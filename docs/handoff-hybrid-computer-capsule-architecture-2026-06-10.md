# Choir Hybrid Computer / Capsule Architecture Handoff

**Date:** 2026-06-10  
**Audience:** Choir coding agents, architecture agents, planning agents, and future implementation agents  
**Purpose:** Convert the recent architecture discussion into a sourced, implementation-ready handoff for docs, planning, and execution.

## Executive thesis

Choir should use a **hybrid persistence/sandbox architecture**:

- A **persistent computer** is the durable seat of agency.
- A **candidate computer** is a speculative future version of a persistent computer.
- A **capsule** is an ephemeral, effect-fenced sandbox used by a persistent computer or candidate computer to run risky, parallel, or disposable work.
- A **mutation transaction** is the durable commit protocol that converts selected effects into canonical state and preserves rollback.

Short formulation:

> **Agents live in computers. Experiments run in sandboxes. Futures run in candidates. State changes by promotion.**

This architecture is not merely a convenience. It follows from the fact that any agent with continuity must keep durable state somewhere. A supervisory agent therefore belongs in a persistent computer, not inside an ephemeral sandbox. The sandbox is where that agent sends bounded experiments.

## Recommended decision

Adopt **Nucleus-style capsules** as an execution primitive inside user, candidate, and platform computers, while preserving Firecracker/cloud-hypervisor-class microVMs as the durable computer boundary.

Do not treat Nucleus as a replacement for candidate VMs or persistent computers. Treat it as an optimization and hardening layer for:

- parallel experiments inside one candidate VM;
- disposable `curl | bash`-style previews inside a user VM;
- verifier capsules;
- source/media parsing and rendering capsules;
- bounded platform jobs with no direct semantic authority.

Keep the following hard boundaries:

| Thing | Durable? | Recommended substrate | Role |
|---|---:|---|---|
| User computer | Yes | microVM / VM-backed NixOS runtime | Durable private agency, app state, source state, local services, prompts, preferences |
| Candidate computer | Yes, speculative | microVM fork / VM-backed candidate runtime | Whole-computer experiment, possible future active computer |
| Platform computer | Yes | platform VM or several platform VMs | Platform-owned semantic state: source cycle, Wire, publication, shared artifacts |
| Capsule | No | Nucleus strict-agent / similar lightweight sandbox | Disposable bounded effect chamber |
| Qdrant | Yes as service, derived as data | host or platform-index VM service, likely Docker/Podman/systemd initially | Derived vector index, not canonical state |
| Host | Yes, infrastructure | NixOS/systemd | Lifecycle, routing, auth, gateway, vmctl, storage substrate |

## Core insight: the agent-inside-sandbox debate

The right answer for Choir is:

> **The durable supervisory agent lives inside a persistent computer and outside ephemeral sandboxes. Disposable worker agents may live inside ephemeral sandboxes.**

A durable agent needs continuity: memory, policy, authority, open tasks, evidence references, recovery state, tool state, and relationships to artifacts. If the agent is placed inside an ephemeral sandbox, either it dies with the sandbox or it must be rehydrated from an external durable state system. In the second case, the real durable agent is outside the sandbox anyway.

However, if the durable agent runs risky commands directly against its persistent computer, the computer becomes an unreviewable mutable accident. Commands like `curl | bash`, package installers, generated scripts, browser profile changes, and parser runs can write to arbitrary places, download binaries, create services, and change future behavior.

Therefore the architecture should distinguish:

1. **Where the agent lives:** persistent computer.
2. **Where risky effects execute:** ephemeral capsule or candidate computer.
3. **How effects become durable:** mutation transaction and promotion certificate.

## Persistent computer is not a sandbox

Choir’s persistent computer is allowed to be durable, personal, divergent, backed up, forked, merged, published from, and updated. It includes VM/runtime state, Dolt/app state, source/build state, content blobs, provenance, and route identity.[^choir-computer]

A sandbox is disposable. A persistent computer is not.

A useful internal slogan:

> **A Choir computer is the durable seat of agency. A Nucleus capsule is a disposable effect chamber. A candidate computer is a speculative future. Promotion is the transaction that turns selected speculative effects into canonical state.**

## Layer model

| Layer | Responsibility | Must not own |
|---|---|---|
| Host substrate | Hardware, storage, routing, lifecycle, auth edge, gateway, vmctl, observability | Private semantic state, user-level meaning, candidate mutations |
| Persistent computer VM | Durable agency, app state, source state, local services, Trace, VText, prompts, policy, preferences | Host-level lifecycle authority |
| Candidate computer VM | Speculative whole-computer mutation with rollback | Canonical route before promotion |
| Capsule runtime | Bounded computation, risky command preview, parser/renderer/verifier, parallel variant | Durable identity, canonical state, promotion authority |
| Mutation transaction | Coordinates base refs, staged effects, verification, commit, rollback | Running arbitrary effects itself |
| Derived index service | Search acceleration and retrieval projections | Canonical truth |

## Nucleus fit assessment

Nucleus is well suited to the **capsule runtime** role because it is designed for ephemeral agent sandboxes and Nix-native production services. It supports agent, strict-agent, and production modes; production mode uses Nix-built root filesystems, service declaration, egress enforcement, health checks, and systemd integration.[^nucleus-modes]

Nucleus is not a good persistent-computer substrate. It explicitly drops Docker’s image/distribution model, has no image layers or registry workflow, and treats persistent storage as explicit volume binds rather than as a default durable machine state.[^nucleus-not-docker]

Nucleus strengths that map directly to Choir capsules:

- programmatic launch config via JSON/TOML;[^nucleus-launch]
- workspace modes: `bind-rw`, `bind-ro`, and `copy-in-out`;[^nucleus-workspace]
- private tmpfs home and explicit provider config mounts;[^nucleus-home]
- pinned agent toolchain rootfs built through Nix helpers;[^nucleus-toolchain]
- all-caps-dropped/seccomp/Landlock/cgroups/namespaces/gVisor-oriented defense-in-depth;[^nucleus-architecture]
- NixOS module support for production-style services and systemd-managed containers;[^nucleus-nixos-module]
- egress allowlists and deny-all defaults in production bridge mode;[^nucleus-egress]
- explicit warning that default agent mode is not hardened; strict-agent or production mode should be used for serious isolation.[^nucleus-agent-warning]

### Required Nucleus policy for Choir

Use these defaults for any serious Choir capsule:

| Policy | Default |
|---|---|
| Service mode | `strict-agent`, not default `agent` |
| Network | `none` unless explicit egress policy exists |
| Workspace | `copy-in-out` or `bind-ro` by default; `bind-rw` only with explicit transaction authority |
| Home | private tmpfs home, not user home |
| Rootfs | pinned Nix-built toolchain rootfs |
| Secrets | no secrets by default; explicit secret mounts only |
| Provider config | read-only unless token refresh is specifically required |
| Logs | stdout/stderr, denied egress, policy hashes, rootfs hash, workspace base hash |
| Outputs | typed output directory/artifact refs only |
| Promotion | never direct; only through transaction/adoption path |

## Candidate VM plus N capsules

The fan-out optimization is sound:

> Instead of forking four candidate VMs for four source/build experiments, fork one candidate VM and run four Nucleus capsules inside it.

This is appropriate when the experiments share the same whole-computer base and differ only in command/workspace-level effects.

Good fit:

| Experiment | Why one candidate VM plus capsules works |
|---|---|
| Four alternative source patches | Shared base, independent worktrees, patch outputs |
| Four build/test strategies | Shared candidate environment, separate scratch/output |
| Parallel verifiers | Read-only access to candidate artifact, separate verifier records |
| Parser/rendering variants | Same source blob, independent outputs |
| Package installer previews | Observe effects without committing to candidate root |

Bad fit:

| Experiment | Why it still needs separate candidate VMs or explicit sub-snapshots |
|---|---|
| Conflicting package-manager mutations | Shared package DB becomes a contention/corruption point |
| Competing systemd service topologies | Port, unit, and runtime state conflicts |
| Browser/profile state experiments | Profile mutation is durable and path-sensitive |
| Kernel/module/device changes | Whole-machine behavior is the subject |
| Candidate route/cutover behavior | Needs actual candidate computer semantics |

Rule:

> **Capsules inside a candidate VM may share read-only base state. Each capsule gets its own write layer unless a mutation transaction explicitly grants shared mutation.**

## User VM disposable route

Inside a user VM, `super` should default to using capsules for risky command execution.

Do not let `super` itself live in the capsule. Let `super` live in the persistent user computer and delegate risky effects to capsules.

### Disposable `curl | bash` flow

1. User or `super` proposes a risky command.
2. Runtime creates a Nucleus strict-agent capsule.
3. Capsule receives no active home/root write access.
4. Capsule may receive a read-only workspace or a copy-in workspace.
5. Network is denied or allowlisted.
6. Output is an effect report, not a mutation of the active computer.
7. `super` classifies the effects.
8. The system chooses one of three outcomes:
   - discard;
   - convert to typed change;
   - escalate to candidate VM mutation.

Effect report fields:

```yaml
effect_report:
  command: string
  base_computer_id: string
  base_source_ref: string | null
  base_dolt_commit: string | null
  rootfs_hash: string
  workspace_base_hash: string | null
  service_mode: strict-agent
  network_policy: object
  secrets_granted: list
  downloads:
    - url: string
      sha256: string | null
      bytes: integer | null
  executed_files:
    - path: string
      sha256: string | null
      origin: downloaded | generated | rootfs | workspace
  filesystem_writes:
    - path: string
      kind: create | modify | delete | chmod | chown | symlink
      sha256_before: string | null
      sha256_after: string | null
  package_manager_attempts:
    - tool: apt | npm | pip | cargo | nix | other
      args: list
      result: string
  service_changes:
    - unit_or_daemon: string
      action: create | enable | start | stop | modify
  network_attempts:
    - dest: string
      port: integer
      allowed: boolean
  exit_code: integer
  stdout_ref: artifact_ref
  stderr_ref: artifact_ref
  output_refs: list
  risk_summary: string
```

## Candidate VM secure mutation route

When the goal is to mutate durable computer state, use a candidate computer.

Example secure route:

1. Fork candidate computer from active computer.
2. Record base refs: VM snapshot, NixOS generation, source ref, Dolt commit, blob manifest, artifact graph ref, Qdrant alias/collection version if relevant.
3. Run one or more experiments inside the candidate, using Nucleus capsules for sub-effects where useful.
4. Apply selected effects inside the candidate VM.
5. Run parallel verifier capsules with read-only access to the candidate output.
6. Produce a promotion certificate.
7. Promote through route/pointer switch and keep rollback refs.

The important difference:

- **Capsule preview** answers: “What would this effect try to do?”
- **Candidate mutation** answers: “Does this possible future computer work?”
- **Promotion transaction** answers: “Should this future become canonical?”

## Mutation transaction coordinator

Choir should implement a durable `MutationTransaction` object. This is the cathedral that coordinates the existing Lego blocks.

It should not pretend to be one perfect ACID transaction over VM disks, NixOS generations, Dolt, Git, blobs, Qdrant, and route identity. It should be a saga-like protocol with strong local transactions, explicit base refs, idempotent apply steps, verifier evidence, and rollback pointers.

### Transaction phases

| Phase | Responsibility |
|---|---|
| Begin | Record owner, authority, base computer, base refs, risk class |
| Stage | Choose substrate: capsule, candidate VM, NixOS test generation, worktree, Dolt branch, Qdrant shadow collection |
| Execute | Run bounded experiments and/or candidate mutation |
| Capture | Collect diffs, logs, network attempts, filesystem writes, package changes, source patches, Dolt diffs, blobs, index deltas |
| Classify | Map effects into ledgers: VM/runtime, source/build, Dolt/app, blob/content, provenance, derived index, route |
| Verify | Run independent verifiers, preferably in read-only capsules |
| Commit | Apply typed deltas, switch route/generation/alias/pointer, record certificate |
| Rollback | Keep old route, old VM snapshot, old Dolt commit, old NixOS generation, old Qdrant collection alias, old artifact refs until TTL |

### Transaction schema sketch

```yaml
mutation_transaction:
  transaction_id: string
  kind: disposable_preview | candidate_mutation | personal_promotion | platform_promotion | derived_index_update
  owner_id: string
  authority_profile: string
  created_by_agent: string
  target_scope: user_computer | candidate_computer | platform_computer | host_substrate
  base_refs:
    computer_id: string
    vm_snapshot_ref: string | null
    nixos_generation: string | null
    source_ref: string | null
    dolt_commit: string | null
    blob_manifest_ref: string | null
    artifact_graph_ref: string | null
    qdrant_alias_ref: string | null
    route_ref: string | null
  stages:
    - stage_id: string
      substrate: nucleus_capsule | candidate_vm | nixos_generation | dolt_branch | git_worktree | qdrant_shadow_collection
      input_refs: list
      output_refs: list
      policy_hashes: list
      result: pending | succeeded | failed | discarded
  classified_effects:
    vm_runtime: list
    nixos_generation: list
    source_build: list
    dolt_app: list
    blob_content: list
    artifact_provenance: list
    derived_index: list
    route_identity: list
    external_side_effects: list
  verifier_results:
    - verifier_id: string
      contract_ref: string
      substrate: nucleus_capsule | candidate_vm | product_api
      evidence_ref: string
      result: pass | fail | inconclusive
  decision:
    status: discarded | accepted_for_candidate | promoted | rolled_back | blocked
    owner_acceptance_ref: string | null
    conflicts_ref: string | null
  rollback_refs:
    previous_route: string | null
    previous_vm_snapshot: string | null
    previous_nixos_generation: string | null
    previous_dolt_commit: string | null
    previous_qdrant_alias_target: string | null
    rollback_until: timestamp | null
```

## Ledger-specific commit laws

| Ledger | Commit law | Rollback law |
|---|---|---|
| VM/OS/runtime | snapshot/cutover, rebuild from typed inputs, or discard | previous VM snapshot / route pointer |
| NixOS system config | build generation, `test`, `switch`, or `boot` | previous system generation / `switch --rollback` |
| Source/build | git-like patch, worktree, package import, build/test | revert patch, previous source ref |
| Dolt/app state | Dolt branch/commit/merge plus app invariants | previous Dolt commit / merge rollback |
| Blob/content | content-addressed insert with metadata | remove/unreference by manifest policy |
| Provenance graph | provenance-preserving graph merge | previous graph ref / tombstone/supersession |
| Qdrant/derived index | build shadow collection, verify, atomically switch alias | switch alias back / keep old collection TTL |
| Route identity | atomic pointer switch only after certificate | previous route pointer |

## NixOS Lego blocks already available

NixOS provides important transactional building blocks, but they are not enough by themselves for Choir’s whole-computer state.

Relevant NixOS mechanics:

- `nixos-rebuild switch` builds a new configuration, makes it the boot default, and tries to realize it in the running system.[^nixos-switch]
- `nixos-rebuild test` switches the running system without making it the boot default, so a reboot can return to the previous default.[^nixos-test]
- `nixos-rebuild boot` makes the new configuration the next boot default without switching immediately.[^nixos-test]
- `nixos-rebuild build-vm` can build a QEMU VM containing a desired configuration for sandboxed testing.[^nixos-buildvm]
- Nix profiles use generation symlinks; the final symlink update is atomic, which enables atomic upgrades and rollbacks for profile-managed packages.[^nix-profiles]
- NixOS rollback can switch to a previous system generation, and system generations are represented under `/nix/var/nix/profiles/system-*-link`.[^nixos-rollback]
- NixOS system switch calculates systemd differences, runs activation, restarts/reloads units, and inspects failures; this is powerful but not an ACID transaction over arbitrary user state or databases.[^nixos-switch-internals]
- NixOS specialisations can build additional configurations and provide runtime switch paths, useful for predefined alternate modes, not arbitrary agent mutations.[^nixos-specialisations]

### Implication for Choir

NixOS should own declared OS/service state. Choir must own cross-ledger promotion.

`nixos-rebuild` can transact the Nix-managed system graph. It does not safely transact:

- `/var` database contents;
- user home state;
- browser profiles;
- Qdrant indexes;
- downloaded binaries;
- arbitrary shell-script side effects;
- app semantic state;
- route identity;
- artifact/provenance graph changes.

Therefore the NixOS generation should be one field in the Choir mutation transaction, not the whole transaction.

## Impermanence and Btrfs as design patterns

The NixOS Impermanence project is conceptually relevant because it makes persistence explicit: choose what files/directories survive; throw the rest away.[^impermanence]

Do not make user computers entirely impermanent. The product object is persistent. Instead, import the lesson:

> **Make persistence explicit by ledger. Everything else is disposable until promoted.**

Recommended VM layout direction:

| Path / ledger | Persistence policy |
|---|---|
| `/nix` | persistent store/cache, governed by image and GC policy |
| `/choir/state/dolt` | canonical app semantic state |
| `/choir/source` | source/build ledger, branchable |
| `/choir/blobs` | content-addressed blob store |
| `/choir/artifacts` | provenance, Trace, verifier, promotion records |
| `/var/lib/qdrant` | derived index state, rebuildable or alias-switchable |
| `/tmp`, capsule scratch, caches | disposable by default |
| `/etc` | NixOS-managed where possible; overlay/declared mutable exceptions only |

Btrfs snapshots can support local fast rollback and branching: a snapshot is a subvolume with initial content from the original, and modifications in the snapshot do not affect the original.[^btrfs-snapshot] But snapshots are not backups because original and snapshot initially share underlying data blocks.[^btrfs-not-backup]

Use Btrfs/local snapshots for quick local rollback and candidate staging, not as the only durability story.

## NixOS containers vs Nucleus capsules

NixOS containers are useful for trusted NixOS-shaped service compartments. They are not the default sandbox for arbitrary user/agent code.

The NixOS manual says NixOS containers share the host Nix store and are efficient, but also warns that they are not perfectly isolated from the host and that container root can affect the host.[^nixos-containers]

Use NixOS containers for trusted service boundaries. Use Nucleus strict-agent capsules for cheap effect-fenced agent/tool execution. Use microVMs for user/candidate/platform computer boundaries.

## Qdrant placement

Qdrant should not be run as a Nucleus capsule.

Reasons:

- Qdrant is a persistent database service.
- Its quickstart uses Docker and a mounted `/qdrant/storage` directory; default local configuration stores data in the mounted storage directory.[^qdrant-quickstart]
- Qdrant’s default local start has no encryption/authentication and must be secured before exposure.[^qdrant-quickstart-security]
- Qdrant requires block-level access to POSIX-compatible storage for persistent storage; it will not work with NFS or S3-style object storage.[^qdrant-storage]
- Qdrant’s docs say Docker/Compose can be used for production only if persistent storage, security, HA, load balancing, backups/disaster recovery, and monitoring/logging are handled by the operator.[^qdrant-docker-production]

Recommended placement options:

| Option | Recommendation |
|---|---|
| Host systemd-managed Docker/Podman Qdrant | Good initial pragmatic option if platform-owned and secured |
| Platform-index VM with systemd-managed Docker/Podman Qdrant | Better isolation and semantic alignment if index belongs to platform computer |
| Native NixOS package/service | Good later if packaging and upgrades are under control |
| Nucleus capsule | Bad fit for persistent DB service |

Critical semantic rule:

> **Qdrant is a derived index, not canonical state.**

Canonical state remains in Dolt/app state, source/build state, blob/content store, and provenance graph. Qdrant points should reference canonical artifact IDs, source spans, content hashes, document revision IDs, and embedding model/version records.

Qdrant’s collection aliases are useful for transactional index updates: build a second collection in the background, verify it, then atomically switch the alias; Qdrant states alias changes happen atomically and concurrent requests are not affected.[^qdrant-alias]

Derived-index update flow:

1. Select canonical corpus and embedding model/version.
2. Build shadow collection.
3. Verify counts, hashes, sample queries, metadata coverage, and latency.
4. Atomically switch alias to the new collection.
5. Keep old collection for rollback TTL.
6. Garbage-collect after confidence window.

## Kubernetes lesson

Kubernetes is not purely stateless. StatefulSets maintain sticky identity for Pods and are useful for applications needing persistent storage or stable network identity.[^k8s-statefulset] PersistentVolumes have a lifecycle independent of any individual Pod.[^k8s-pv]

But Kubernetes state primitives do not solve the Choir problem. They solve stable service identity and persistent storage for pods. Choir needs transactional personal computation: arbitrary agent-induced mutations across VM/runtime, NixOS, source, Dolt/app state, blobs, provenance, indexes, and routes.

Take the lesson but do not import the whole architecture unless needed:

> Kubernetes externalizes and orchestrates service state. Choir must make personal-computer mutation legible, staged, typed, verifiable, and reversible.

## Firecracker / microVM role

Firecracker-class microVMs remain the right mental model for user/candidate/platform computer isolation. Firecracker uses KVM to create microVMs, reduces device model and attack surface, and positions microVMs as stronger workload isolation with container-like speed and efficiency.[^firecracker]

Firecracker’s documented benefits include KVM-based virtualization, minimal device model, fast startup, and low memory overhead.[^firecracker-benefits]

Use microVMs for:

- user computer boundary;
- candidate computer boundary;
- platform computer boundary;
- cross-user/cross-tenant isolation;
- whole-computer state experiments.

Use capsules for:

- bounded commands;
- parser/renderer/verifier jobs;
- parallel patch/build/test variants;
- source-cycle transforms;
- disposable risky script previews.

## Platform service placement

Distinguish **substrate authority** from **semantic authority**.

### Host-level services

Keep host services narrow and infrastructure-oriented:

| Host service class | Why host-level is appropriate |
|---|---|
| `auth` | Edge identity/session lifecycle |
| `proxy` / routing | Authenticated request routing and route pointer resolution |
| `vmctl` | Computer lifecycle, warmness, snapshot, reclaim, host controls |
| `gateway` | Provider credential boundary and provider mediation |
| Storage plumbing | Volumes, images, snapshots, backups |
| Observability | Host logs, health, metrics |
| System deploy | NixOS host generations and systemd |

These services must not become private semantic owners.

### Platform computer services

Platform-level semantic work belongs in a platform computer:

| Platform semantic service | Why platform VM/computer is appropriate |
|---|---|
| Source cycle state | Owns source records, provenance, edition state |
| Wire / Universal Wire | Platform-level artifact synthesis and public/private source boundary |
| Publication state | Public artifact graph, route projections, review state |
| Shared index policy | Derived search policy and embedding provenance |
| Platform VTexts | Cloud-owned semantic artifacts |

### Source cycle split

Recommended split:

| Component | Placement |
|---|---|
| Host source adapter / scheduler | Host service; narrow queue/network/lifecycle role |
| Platform source-cycle semantic state | Platform computer |
| Source fetch/parse/render jobs | Nucleus capsules inside platform VM by default |
| No-user-secret batch jobs | Host-level Nucleus possible after audit |
| Platform Qdrant | Platform-index VM or host service, but derived index only |

### Multiple platform VMs

Start with one platform VM plus capsules unless pressure demands separation. Split platform VMs when resource, security, failure-domain, or lifecycle differences become real.

Potential split:

| Platform VM | Responsibility |
|---|---|
| Platform-control VM | Source-cycle coordination, publication state, platform semantic authority |
| Platform-index VM | Qdrant, embedding workers, derived indexes |
| Platform-render VM | Heavy media/source transforms and export rendering |
| Platform-candidate VMs | Speculative platform computer mutations |

## Capability and authority model

Every capsule should be launched from an explicit durable `CapsuleSpec`.

```yaml
capsule_spec:
  capsule_id: string
  parent_transaction_id: string
  owner_computer_id: string
  requesting_agent_id: string
  purpose: string
  substrate: nucleus
  service_mode: strict-agent
  runtime: native | gvisor
  rootfs_ref: string
  command: list
  workdir: string
  workspace:
    ref: string | null
    mode: bind-ro | bind-rw | copy-in-out | none
    exec_allowed: boolean
  network_policy:
    mode: none | bridge | gvisor-host
    dns: list
    egress_allow_cidrs: list
    egress_allow_domains: list
    egress_tcp_ports: list
  resource_limits:
    memory: string
    cpus: number
    pids: integer
    wall_time_seconds: integer
  secrets:
    - secret_ref: string
      mount_path: string
      mode: ro
  provider_configs:
    - source_ref: string
      dest: string
      mode: ro | rw
  allowed_outputs:
    - path: string
      schema: string
  logging:
    stdout: artifact
    stderr: artifact
    denied_egress: artifact
    lifecycle: trace
```

`CapsuleResult` should be equally durable.

```yaml
capsule_result:
  capsule_id: string
  parent_transaction_id: string
  status: succeeded | failed | killed | degraded_security_blocked
  exit_code: integer | null
  started_at: timestamp
  finished_at: timestamp
  policy_hashes:
    seccomp: string | null
    caps: string | null
    landlock: string | null
    launch_config: string
  rootfs_hash: string
  workspace_base_hash: string | null
  output_refs: list
  stdout_ref: string
  stderr_ref: string
  denied_egress_ref: string | null
  filesystem_diff_ref: string | null
  effect_report_ref: string | null
  verifier_record_ref: string | null
```

## Verifier capsules

Verifier capsules should be cheap, parallel, and mostly read-only.

Default verifier policy:

| Setting | Default |
|---|---|
| Workspace | read-only candidate artifacts |
| Network | none unless verifier contract needs product API |
| Secrets | none |
| Outputs | verifier result, logs, trace refs |
| Authority | cannot promote; can only report |
| Runtime | Nucleus strict-agent or candidate VM when full product behavior required |

Verifiers should be able to create temporary tests/scripts in their own scratch area, but not mutate canonical product state.

## Risk classes and substrate selection

| Risk class | Example | Default substrate |
|---|---|---|
| Pure computation | format conversion, static analysis | Nucleus capsule |
| Risky command preview | `curl | bash`, unknown installer | Nucleus capsule with no active writes |
| Source-only experiment | patch/build/test | Nucleus capsule inside user/candidate VM |
| Parallel implementation variants | four competing patches | one candidate VM plus N capsules |
| Persistent app/runtime change | new Go/Svelte build, app bundle, prompt update | candidate VM, with capsules for substeps |
| OS/package/service mutation | install package, add systemd service | candidate VM plus NixOS generation transaction |
| Platform semantic mutation | source cycle, publication state | platform candidate VM / platform computer transaction |
| Host substrate change | auth/proxy/vmctl/gateway/NixOS host config | host NixOS deploy path, staging proof |
| Derived index update | Qdrant rebuild | shadow collection + alias switch |

## Implementation plan

### Milestone 0 — Documentation and vocabulary

Deliverables:

- Add architecture doc: persistent computers, candidate computers, capsules, transactions.
- Add glossary entries for `Capsule`, `CapsuleSpec`, `CapsuleResult`, and `MutationTransaction`.
- State rule: durable supervisory agents live in persistent computers; disposable workers may run inside capsules.

Acceptance:

- Docs distinguish sandbox/capsule from computer.
- No doc suggests Nucleus replaces candidate VMs.

### Milestone 1 — CapsuleRunner interface

Deliverables:

- Define Go interface for `CapsuleRunner`.
- Define `CapsuleSpec` and `CapsuleResult` structs.
- Add persistence for launch config, result, logs, and policy hashes.
- First backend may be a no-op/fake runner for tests.

Acceptance:

- Unit tests show specs/results round-trip.
- Trace records capsule lifecycle events.

### Milestone 2 — Nucleus strict-agent backend

Deliverables:

- Add Nucleus backend behind `CapsuleRunner`.
- Launch via JSON/TOML config file or fd, not ad hoc argv string construction.
- Enforce strict-agent; reject degraded security.
- Capture stdout/stderr, exit code, denied egress logs, and output refs.

Acceptance:

- Deterministic local test runs a harmless command in strict-agent.
- Test verifies network-denied behavior where supported.
- Test verifies active home is not mounted by default.

### Milestone 3 — Disposable shell route for `super`

Deliverables:

- Default risky shell/script execution path delegates to a capsule.
- `curl | bash`-style commands produce effect reports, not active mutation.
- Add UI/product evidence path for “discard / convert / escalate.”

Acceptance:

- Demonstration command writes inside capsule scratch only.
- Active computer state remains unchanged.
- Effect report appears in Trace/artifacts.

### Milestone 4 — Candidate fan-out

Deliverables:

- Candidate VM can spawn multiple independent capsules.
- Each capsule gets independent worktree/scratch.
- Results can be compared by `super`/`vsuper`.

Acceptance:

- One candidate VM runs at least two parallel source/build variants.
- Outputs are separate patch/artifact refs.
- No shared writable checkout contention.

### Milestone 5 — Verifier capsules

Deliverables:

- Verifier contract can request read-only capsule execution.
- Verifier result references input artifact hashes and capsule policy.
- Parallel verifier scheduling supported.

Acceptance:

- Candidate artifact is verified by at least two independent capsule runs.
- Verifier cannot promote or mutate canonical state.

### Milestone 6 — MutationTransaction model

Deliverables:

- Define durable transaction object.
- Record base refs across VM/NixOS/source/Dolt/blob/artifact/Qdrant/route where applicable.
- Implement state machine: begin, stage, execute, capture, classify, verify, commit, rollback.

Acceptance:

- A simple source patch transaction records base refs, capsule output, verifier evidence, owner acceptance, and rollback refs.

### Milestone 7 — Qdrant derived-index transaction

Deliverables:

- Deploy Qdrant as host or platform-index VM service, not as capsule.
- Define canonical-to-index manifest.
- Build shadow collection and switch alias after verification.

Acceptance:

- Qdrant collection can be rebuilt from canonical refs.
- Alias switch is recorded as transaction step.
- Old collection remains available for rollback TTL.

### Milestone 8 — Platform source cycle capsules

Deliverables:

- Source-cycle jobs run parser/renderer/extractor tasks in capsules.
- Platform computer owns semantic source-cycle state.
- Host service remains narrow scheduler/adapter where needed.

Acceptance:

- One source fetch/parse/render path produces typed artifacts and provenance through a capsule.
- Capsule has no direct write to platform semantic state except through output adoption.

## Critical constructive disputes to preserve

1. **Do not run durable `super` inside Nucleus.** Run `super` in the persistent computer. Let it operate capsules.
2. **Do not replace candidate VMs with Nucleus.** Use Nucleus to optimize sub-experiments inside candidates.
3. **Do not run Qdrant as a Nucleus capsule.** It is persistent database infrastructure; capsules may build inputs for it.
4. **Do not use host-level Nucleus for arbitrary user code by default.** User-originated code should first be contained by the user/candidate microVM boundary.
5. **Do not let capsules mutate active state directly.** Typed outputs and transaction adoption only.
6. **Do not confuse snapshots with backups.** Snapshots are rollback tools, not durability guarantees.
7. **Do not confuse NixOS generations with full computer transactions.** NixOS generations cover Nix-managed system state, not arbitrary mutable ledgers.
8. **Do not import Kubernetes as the answer.** Kubernetes teaches stable identity and volumes, but Choir needs transactional personal-computer mutation.

## Research backlog

Further research before deep implementation:

- Nucleus maturity audit: kernel requirements, rootless behavior, gVisor behavior, failure modes, degraded-security rejection paths, maintenance status.
- Compare Nucleus with `bubblewrap`, `systemd-run`/`DynamicUser`/`PrivateTmp`, `firejail`, `nsjail`, `gVisor runsc`, and `microvm.nix` for capsule workloads.
- Determine whether effect capture should use filesystem overlay diffs, fanotify/inotify, eBPF, ptrace/seccomp trace, package-manager wrappers, or a combination.
- Decide VM filesystem strategy: Btrfs subvolumes, block snapshots, qcow2 overlays, or Firecracker snapshot/restore path.
- Decide Qdrant deployment shape: host Docker/Podman, platform-index VM Docker/Podman, native Nix package, or Qdrant Cloud during development.
- Define exact secret/capability delegation model for provider CLIs inside capsules.
- Define how capsule output refs enter Trace, RunAcceptance, AppChangePackages, and promotion certificates.
- Validate performance of one candidate VM plus many capsules under realistic Go/Svelte/Dolt workloads.

## Source footnotes

[^choir-computer]: **Choir computer ontology.** URL: https://github.com/choir-hip/go-choir/blob/main/docs/computer-ontology.md. Relevant text: Choir gives each user a persistent computer, not merely a sandbox; the computer includes VM/runtime state, Dolt/app state, source/build state, blobs, provenance graph, and route identity.

[^nucleus-modes]: **Nucleus README — modes.** URL: https://github.com/sig-id/nucleus/blob/main/README.md. Relevant text: Nucleus supports agent mode, strict-agent mode, and production mode; production mode combines Nix-built root filesystems, egress policy enforcement, health checks, and systemd integration.

[^nucleus-not-docker]: **Nucleus README — relationship to Docker.** URL: https://github.com/sig-id/nucleus/blob/main/README.md. Relevant text: Nucleus says it is not a Docker replacement, has no images/layers/registry/pull/push flow, and defaults to ephemeral tmpfs or Nix closures with persistence through explicit volume binds.

[^nucleus-launch]: **Nucleus README — programmatic launch config.** URL: https://github.com/sig-id/nucleus/blob/main/README.md. Relevant text: `nucleus run` can accept JSON/TOML launch config via file or fd; config owns the launch request.

[^nucleus-workspace]: **Nucleus README — workspace modes.** URL: https://github.com/sig-id/nucleus/blob/main/README.md. Relevant text: workspace modes include `bind-rw`, `bind-ro`, and `copy-in-out`; mounts default to restrictive flags and executable workspace requires explicit handling.

[^nucleus-home]: **Nucleus README — sandbox home and provider config.** URL: https://github.com/sig-id/nucleus/blob/main/README.md. Relevant text: Nucleus creates a private tmpfs home and recommends explicit provider config mounts rather than broad host bind mounts.

[^nucleus-toolchain]: **Nucleus README — agent toolchain rootfs.** URL: https://github.com/sig-id/nucleus/blob/main/README.md. Relevant text: Nucleus supports a pinned agent toolchain rootfs built with `mkAgentToolchainRootfs`, avoiding dependence on mutable host `/bin`, `/usr`, `/lib`, or `/nix` binds.

[^nucleus-architecture]: **Nucleus README — architecture.** URL: https://github.com/sig-id/nucleus/blob/main/README.md. Relevant text: Nucleus uses namespaces, cgroups v2, pivot_root, capabilities, seccomp, Landlock, gVisor, OCI bundle generation, secrets tmpfs, and mount audit.

[^nucleus-nixos-module]: **Nucleus README — NixOS module.** URL: https://github.com/sig-id/nucleus/blob/main/README.md. Relevant text: Nucleus provides a declarative NixOS module that creates systemd services, journald logging, sd_notify readiness, automatic restart, workload identity, rootfs attestation, egress policy, secrets, credentials, and volumes.

[^nucleus-egress]: **Nucleus README — egress policy.** URL: https://github.com/sig-id/nucleus/blob/main/README.md. Relevant text: in production bridge mode without allow rules, Nucleus installs deny-all outbound policy including DNS; with allow rules it permits loopback, established flows, configured DNS, resolved domain/CIDR/port rules, and drops the rest.

[^nucleus-agent-warning]: **Nucleus README — security notes.** URL: https://github.com/sig-id/nucleus/blob/main/README.md. Relevant text: default agent mode is explicitly not hardened and may warn-and-continue on security mechanisms; strict-agent and production modes forbid degraded security.

[^nixos-switch]: **NixOS Manual — changing configuration.** URL: https://nixos.org/manual/nixos/stable/#sec-changing-config. Relevant text: `nixos-rebuild switch` builds the new configuration, makes it the boot default, and tries to realize it in the running system.

[^nixos-test]: **NixOS Manual — test and boot actions.** URL: https://nixos.org/manual/nixos/stable/#sec-changing-config. Relevant text: `nixos-rebuild test` switches the running system without making it the boot default; `boot` makes the configuration boot default without switching immediately.

[^nixos-buildvm]: **NixOS Manual — build-vm.** URL: https://nixos.org/manual/nixos/stable/#sec-changing-config. Relevant text: `nixos-rebuild build-vm` builds and runs a QEMU VM containing the desired configuration for sandboxed testing.

[^nix-profiles]: **Nix Reference Manual — profiles.** URL: https://nix.dev/manual/nix/2.28/package-management/profiles. Relevant text: profiles implement different configurations and atomic upgrades/rollbacks using generation symlinks; the final profile symlink update is atomic.

[^nixos-rollback]: **NixOS Manual — rollback.** URL: https://nixos.org/manual/nixos/stable/#sec-rollback. Relevant text: a previous system configuration can be selected via bootloader or `nixos-rebuild switch --rollback`; system generations appear under `/nix/var/nix/profiles/system-*-link`.

[^nixos-switch-internals]: **NixOS Manual — what happens during a system switch.** URL: https://nixos.org/manual/nixos/stable/#ch-system-switch. Relevant text: `switch-to-configuration` computes systemd/mount differences, runs activation, restarts/reloads/starts/stops units, and inspects failures.

[^nixos-specialisations]: **NixOS specialisation module source.** URL: https://raw.githubusercontent.com/NixOS/nixpkgs/master/nixos/modules/system/activation/specialisation.nix. Relevant text: NixOS specialisations build additional configurations and expose runtime switch paths such as `/run/current-system/specialisation/<name>/bin/switch-to-configuration test`.

[^impermanence]: **nix-community/impermanence.** URL: https://github.com/nix-community/impermanence. Relevant text: Impermanence lets users choose which files/directories persist between reboots while the rest are thrown away; it encourages declaring what should be kept and supports tmpfs or Btrfs-root patterns.

[^btrfs-snapshot]: **Btrfs subvolumes documentation.** URL: https://btrfs.readthedocs.io/en/latest/Subvolumes.html. Relevant text: a Btrfs snapshot is a subvolume with initial content from the original; modifications in the snapshot do not affect the original subvolume.

[^btrfs-not-backup]: **Btrfs subvolumes documentation.** URL: https://btrfs.readthedocs.io/en/latest/Subvolumes.html. Relevant text: Btrfs snapshots are not backups because snapshot and original initially share the same data blocks; damage to shared blocks affects both.

[^nixos-containers]: **NixOS Manual — Container Management.** URL: https://nixos.org/manual/nixos/stable/#ch-containers. Relevant text: NixOS containers share the host Nix store and are efficient, but the manual warns they are not perfectly isolated and container root can affect the host.

[^qdrant-quickstart]: **Qdrant Local Quickstart.** URL: https://qdrant.tech/documentation/quickstart/. Relevant text: Qdrant’s local quickstart pulls the Docker image and runs it with `-v ./qdrant_storage:/qdrant/storage`; default configuration stores data in that local directory.

[^qdrant-quickstart-security]: **Qdrant Local Quickstart — security note.** URL: https://qdrant.tech/documentation/quickstart/. Relevant text: Qdrant starts by default with no encryption or authentication, so anyone with network access to the instance can access it unless secured.

[^qdrant-storage]: **Qdrant Installation Requirements — storage.** URL: https://qdrant.tech/documentation/operations/installation/. Relevant text: persistent Qdrant storage requires block-level access with a POSIX-compatible filesystem; NFS and S3-style object storage are not supported for this purpose.

[^qdrant-docker-production]: **Qdrant Installation — Docker/Compose production.** URL: https://qdrant.tech/documentation/operations/installation/. Relevant text: Docker/Compose can be used for production only if the operator handles persistent storage, security, HA/distributed deployment, load balancing, backup/disaster recovery, and monitoring/logging.

[^qdrant-alias]: **Qdrant Collections — collection aliases.** URL: https://qdrant.tech/documentation/manage-data/collections/#collection-aliases. Relevant text: Qdrant supports aliases for collections; one can build a second collection in the background and atomically switch the alias without affecting concurrent requests.

[^k8s-statefulset]: **Kubernetes StatefulSets.** URL: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/. Relevant text: StatefulSets maintain sticky identity for Pods and are useful for applications that need persistent storage or stable unique network identity.

[^k8s-pv]: **Kubernetes Persistent Volumes.** URL: https://kubernetes.io/docs/concepts/storage/persistent-volumes/. Relevant text: PersistentVolumes are cluster storage resources with lifecycles independent of any individual Pod that uses them.

[^firecracker]: **Firecracker homepage.** URL: https://firecracker-microvm.github.io/. Relevant text: Firecracker is a KVM-based VMM for lightweight microVMs, designed for secure multi-tenant container/function services.

[^firecracker-benefits]: **Firecracker benefits.** URL: https://firecracker-microvm.github.io/. Relevant text: Firecracker reduces device model and attack surface, supports fast startup, low memory overhead, and uses a jailer as a second line of defense.
