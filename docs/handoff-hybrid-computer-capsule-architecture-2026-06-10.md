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

[Showing lines 1-300 of 831. Use :301 to continue]