# Autoputer Self-Transition Architecture Proposal v0

## Problem

Today, transitioning an autoputer to a new audited state requires going
**outside** the VM:

1. Agent wants to try a risky mutation
2. Agent calls `vmctl.ForkDesktop` on the **host** → creates a candidate VM
3. Agent runs experiments in the candidate VM
4. Host promotes the candidate via route pointer flip
5. Host manages rollback via VM snapshot

This has three problems:

- **The VM is not self-authoring.** It cannot transition its own state
  without host participation. The host is the authority, not the autoputer.
- **Candidate VMs are expensive.** Each speculative fork is a full Firecracker
  microVM with its own kernel, memory, and disk. Resource cost scales with
  the number of concurrent experiments.
- **The promotion protocol is ceremonial.** The current
  `PromoteAppAdoption` is "a database pointer flip with no real git ref
  move, no route switch consumed by any proxy, no process restart, no
  binary swap" (promotion protocol conjecture doc). It doesn't actually
  promote audited state.

## Solution

With the EROFS + overlay + dm-verity architecture (see
`docs/proposals/round-trip-invertibility-design-v0.md`), state and runtime
are separated. The autoputer can self-transition by spawning capsules with
their own overlay layers, verifying results, and committing the winning
overlay diff to its own tape. The host does not participate in the
transition decision.

## Architecture

### Disk layout per autoputer VM

```
/dev/vd0  → EROFS store disk (Nix closure, read-only, shared across VMs)
/dev/vd1  → EROFS base disk (tape-derived, dm-verity sealed, read-only)
/dev/vd2  → ext4 data disk (mutable, per-VM)

Guest mounts:
  /nix/store           ← /dev/vd0 (EROFS, read-only)
  /autoputer/base      ← /dev/vd1 (EROFS via dm-verity, read-only)
  /mnt/persistent      ← /dev/vd2 (ext4, read-write)
    /mnt/persistent/dolt       ← Dolt database (direct ext4, NOT overlay)
    /mnt/persistent/overlays   ← overlay upper/work directories
      /mnt/persistent/overlays/active/upper
      /mnt/persistent/overlays/active/work
      /mnt/persistent/overlays/<capsule-id>/upper
      /mnt/persistent/overlays/<capsule-id>/work
  /autoputer/merged    ← overlayfs (lowerdir=/autoputer/base,
                                      upperdir=/mnt/persistent/overlays/active/upper,
                                      workdir=/mnt/persistent/overlays/active/work)
```

### Capsule layout (inside the VM)

Each capsule is a Nucleus container with:

```
  /capsule/base        ← bind-mount /autoputer/base (read-only)
  /capsule/merged      ← overlayfs (lowerdir=/capsule/base,
                                      upperdir=/mnt/persistent/overlays/<capsule-id>/upper,
                                      workdir=/mnt/persistent/overlays/<capsule-id>/work)
  /capsule/dolt        ← optional: Dolt checkout on a branch (see below)
```

The capsule's root filesystem is `/capsule/merged`. It sees the tape-derived
base state plus its own writes. It cannot see the active overlay or other
capsules' overlays.

### The self-transition loop

```
                    ┌─────────────────────────────────────┐
                    │       autoputer VM (one kernel)      │
                    │                                     │
                    │  active overlay                      │
                    │  /autoputer/merged                   │
                    │     │                               │
                    │     │ agent runs here                │
                    │     │                               │
                    │     │ 1. spawn capsule(s)            │
                    │     │    (own overlay, own dolt      │
                    │     │     branch)                    │
                    │     ▼                               │
                    │  capsule 1    capsule 2    capsule 3 │
                    │  /caps/1/     /caps/2/     /caps/3/  │
                    │  merged       merged       merged    │
                    │     │            │           │       │
                    │     │ 2. run experiments               │
                    │     ▼            ▼           ▼       │
                    │  results       results     results    │
                    │     │            │           │       │
                    │     │ 3. verify in read-only capsule   │
                    │     ▼                               │
                    │  verifier capsule (bind-ro all)       │
                    │     │                               │
                    │     │ 4. pick winner                   │
                    │     ▼                               │
                    │  5. commit overlay diff to tape       │
                    │     │   (journal entries + blobs)      │
                    │     ▼                               │
                    │  6. materialize new EROFS base        │
                    │     │   (from updated tape)            │
                    │     ▼                               │
                    │  7. swap base + reset active overlay  │
                    │     │   (drop old upper, mount new    │
                    │     │    base, new empty upper)        │
                    │     ▼                               │
                    │  /autoputer/merged (new base state)   │
                    │                                     │
                    └─────────────────────────────────────┘
```

**Step 5 is the key:** the agent commits the capsule's overlay diff to the
tape as typed journal entries + blobs. This is not a host-level promotion —
it's a tape-level commit. The tape is the source of truth.

**Step 6-7:** the new EROFS base is materialized from the updated tape. The
old base remains as a rollback point. The active overlay is dropped (reset
to the new base). The VM is now on the new audited state — without going
outside itself.

### What the host does (and doesn't do)

**The host does:**
- Attaches the EROFS base disk + store disk + ext4 data disk at VM boot
- Provisions the overlay backing (ext4 file on the data disk)
- Enforces dm-verity at the block layer (kernel-level)
- Manages VM lifecycle (boot, stop, hibernate, resume)

**The host does NOT:**
- Decide when to transition state (that's the autoputer's decision)
- Create candidate VMs (capsules replace them)
- Flip route pointers for promotion (the tape commit IS the promotion)
- Manage rollback snapshots (the EROFS base history IS the rollback)

### Choir-in-choir as nonlocal API

The agent inside the autoputer uses the **same HTTP API** as a remote user.
No in-process shortcuts. No shared memory. The agent is a regular API client
with elevated permissions.

```
Agent in autoputer VM
  │
  │  choir CLI → HTTP → vsock/tap → host proxy → Choir API
  │
  ├── POST /api/compute/spawn-capsule    → creates overlay upper
  ├── POST /api/compute/run-in-capsule   → runs command in capsule
  ├── POST /api/compute/verify           → read-only verifier capsule
  ├── POST /api/compute/commit-to-tape   → commits overlay diff to journal
  ├── POST /api/compute/materialize      → builds new EROFS base from tape
  ├── POST /api/compute/swap-base        → swaps base + resets overlay
  └── GET  /api/compute/state            → reads current tape state
```

The API is already HTTP-based (`http.ServeMux`). The new endpoints are
**compute-local** — they operate on the autoputer's own state, not on host
state. The host proxy forwards them to the VM's local service (or the VM
handles them directly via a vsock listener).

**Why nonlocal:**
- One API surface for local and remote
- Clear boundary between agent and host
- No corruption risk from in-process access
- Substrate independence: agent doesn't know if Choir is local or remote
- The "super" agent is just a regular API client with elevated permissions

### Dolt branching for state speculation

Dolt is mounted directly on ext4 (not through overlay) at
`/mnt/persistent/dolt`. For state-level speculation, capsules use Dolt
branches:

```
Active Dolt state:     main branch
Capsule 1 Dolt state:  candidate-1 branch (checkout in /capsule/1/dolt)
Capsule 2 Dolt state:  candidate-2 branch (checkout in /capsule/2/dolt)
```

Each capsule checks out a Dolt branch. Writes go to the branch. Promotion
merges the winning branch to main. Rollback drops the branch.

This covers the Dolt/app state ledger. The EROFS base covers the
VM/runtime state ledger. Together they cover the full computer ontology:

| Ledger | Speculation mechanism | Promotion |
|---|---|---|
| VM/runtime state | EROFS base + overlay | commit overlay diff → new EROFS base |
| Dolt/app state | Dolt branch | merge branch to main |
| Source/build | git branch (in overlay) | commit to tape |
| Blobs | content-addressed (immutable) | union by hash |
| Artifact graph | provenance-preserving | graph merge |
| Route identity | host-level pointer | host swaps route after tape commit |

### What this eliminates

| Old concept | New model |
|---|---|
| Candidate computer (full VM fork) | Capsule with own overlay (same kernel) |
| `vmctl.ForkDesktop` (host creates VM) | `spawn-capsule` (VM creates overlay) |
| VM snapshot for rollback | EROFS base history (drop overlay = reset) |
| Host-level promotion (route pointer flip) | Tape commit (autoputer self-promotes) |
| In-process choir-in-choir | Nonlocal HTTP API |
| Separate kernel per candidate | One kernel per autoputer, shared by capsules |

### What this keeps

- **One VM per autoputer** (Firecracker microVM) — a user can have many
- **Kernel/module changes** — still need a new VM, but this is a rare
  "kernel upgrade" mission, not a speculative candidate
- **Dolt direct mount** — still on ext4, not through overlay
- **dm-verity enforcement** — kernel still enforces base integrity
- **Host VM lifecycle** — host still boots/stops/resumes VMs

### When you still need a separate VM

- **Kernel upgrade**: changing the kernel image requires a new VM boot.
  This is a deliberate mission, not a speculative experiment.
- **OS-level migration**: changing the Nix closure (rootfs, initrd) requires
  a new VM. Also a deliberate mission.
- **Cross-substrate proof**: materializing the same tape on a different
  substrate (e.g., container vs Firecracker) requires a separate substrate
  instance. This is a verification step, not a speculative candidate.

These are all **deliberate, rare operations** — not the speculative
exploration that candidate computers were designed for.

## Concrete API

### New endpoints (handled inside the VM)

```go
// SpawnCapsule creates a new capsule with its own overlay upper layer.
// The capsule shares the EROFS base (read-only) and gets a fresh
// overlay upper backed by /mnt/persistent/overlays/<capsule-id>/.
POST /api/compute/spawn-capsule
{
  "capsule_id": "exp-1",
  "dolt_branch": "candidate-1",  // optional: checkout a Dolt branch
  "workspace_mode": "copy-in-out",  // Nucleus workspace mode
  "network": "deny-all"  // Nucleus network policy
}
→ 200 { "capsule_id": "exp-1", "root": "/capsule/exp-1/merged" }

// RunInCapsule executes a command inside a capsule.
POST /api/compute/run-in-capsule
{
  "capsule_id": "exp-1",
  "command": ["nix-build", "-A", "foo"],
  "timeout_ms": 30000
}
→ 200 { "exit_code": 0, "stdout": "...", "stderr": "..." }

// CommitToTape commits a capsule's overlay diff to the tape.
// This walks the overlay upper directory, classifies changes into
// typed journal entries + blobs, and appends them to the journal.
POST /api/compute/commit-to-tape
{
  "capsule_id": "exp-1",
  "commit_message": "install foo package",
  "dolt_merge": true  // merge Dolt branch to main
}
→ 200 { "journal_entries": 42, "blob_count": 17, "tape_head": "sha256:..." }

// MaterializeBase builds a new EROFS base from the current tape.
POST /api/compute/materialize
{
  "tape_head": "sha256:..."  // optional: materialize at a specific head
}
→ 200 { "base_image": "/tmp/base-new.erofs", "verity_root": "sha256:..." }

// SwapBase swaps the active EROFS base and resets the active overlay.
// The old base is retained as a rollback point.
POST /api/compute/swap-base
{
  "base_image": "/tmp/base-new.erofs",
  "verity_root": "sha256:..."
}
→ 200 { "previous_base": "sha256:...", "active_base": "sha256:..." }

// RollbackBase reverts to a previous EROFS base.
POST /api/compute/rollback-base
{
  "target_base": "sha256:..."
}
→ 200 { "active_base": "sha256:..." }
```

### Modified endpoints (host-level)

```go
// ForkDesktop is DEPRECATED. Replaced by spawn-capsule inside the VM.
// Kept for backward compatibility but logs a deprecation warning.
POST /internal/vmctl/fork-desktop
→ 410 Gone { "error": "use spawn-capsule inside the VM instead" }
```

### Existing endpoints (unchanged)

All existing `/api/*` endpoints continue to work. The agent uses them
exactly as a remote user would. The only difference is that the agent's
requests originate from inside the VM and reach the host via vsock/tap.

## Implementation phases

### Phase 1: EROFS base + overlay (this mission)

- `ErofsImageBuilder` using `mkfs.erofs` (reference builder)
- `ErofsObservationExtractor` using `go-erofs` (read-only, for audit)
- `VeritySealer` using `veritysetup` (build-time sealing)
- `AutoputerMaterializer` (tape → EROFS base + verity seal)
- `AutoputerAuditor` (go-erofs read → manifest comparison)
- vmmanager config: add `AutoputerBaseDiskPath` + `BaseVerityRoot`
- Guest init: mount EROFS base at `/autoputer/base` with dm-verity
- Guest init: mount overlayfs at `/autoputer/merged`
- Keep Dolt on direct ext4 at `/mnt/persistent/dolt`

### Phase 2: Capsule overlays (next mission)

- Nucleus integration: capsules get overlay upper layers
- `spawn-capsule` endpoint (inside VM)
- `run-in-capsule` endpoint (inside VM)
- Per-capsule Dolt branch checkout
- Capsule network isolation (Nucleus deny-all + allowlist)

### Phase 3: Self-transition (next mission)

- `commit-to-tape` endpoint: walk overlay diff → journal entries + blobs
- `materialize` endpoint: tape → new EROFS base (inside VM)
- `swap-base` endpoint: swap base + reset overlay (inside VM)
- `rollback-base` endpoint: revert to previous base
- Deprecate `fork-desktop` host endpoint

### Phase 4: Nonlocal API (next mission)

- Expose all `/api/compute/*` endpoints over vsock/tap
- Agent CLI uses HTTP exclusively (no in-process path)
- Authentication: VM-provisioned credentials
- The "super" agent is a regular API client with elevated permissions

## Mutation class

Orange — adds new materializer, new capsule system, new API endpoints, new
guest init. Does not change existing StateGenerator or TreeToFS. Does not
change vmmanager VM lifecycle (boot/stop/resume still work). Rollback path:
don't use the new endpoints; existing `fork-desktop` flow still works until
deprecation.

## SIAC gate advancement

- **Gate 2** (substrate boundary): ✅ EROFS base + capsules behind materializer boundary
- **Gate 3** (typed durable state): ✅ EROFS base is tape-derived, verity-sealed, typed
- **Gate 4** (cross-substrate proof): ✅ same tape → EROFS on Firecracker vs directory on host
- **Gate 5** (failure proof): ✅ seeded mismatch → different verity root → audit fails
- **Gate 6** (promotion/rollback): ✅ tape commit IS the promotion; EROFS base history IS rollback
- **Gate 7** (staging proof): ❌ blocked until vmmanager boots from materialized base on staging

## Open questions

1. Can `materialize` run inside the VM (build EROFS from tape in-guest), or
   does it need to call out to the host? `mkfs.erofs` needs to be available
   in the guest. Alternative: the VM sends the tape head to the host, the
   host materializes, and attaches the new base disk via PATCH /drives.
2. How does `swap-base` work while the VM is running? Options:
   a. Stop the VM, swap the base disk, resume (downtime)
   b. PATCH /drives to hot-swap the base disk (Firecracker supports this)
   c. Mount the new base at a new path, atomically switch the overlay lowerdir
3. How are capsule overlay uppers cleaned up? On capsule exit? On tape commit?
   Periodically?
4. How does the agent authenticate to its own VM's API? VM-provisioned token?
   vsock-implicit trust?
5. Can the host still force a rollback (e.g., for security reasons)? Or is
   the autoputer fully autonomous?
6. How does this interact with the existing `super`/`vsuper`/`cosuper` agent
   hierarchy? Does `vsuper` now mean "capsule owner" instead of "candidate
   VM owner"?
7. What happens to existing candidate VMs during migration? Do they get
   converted to capsules?
