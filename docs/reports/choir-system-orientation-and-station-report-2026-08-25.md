# Choir Master System Orientation & Station Report
**Date:** 2026-08-25  
**Context:** Engineering Orientation, Architectural Recalibration, Postmortem, and Strategic Roadmap Alignment  
**Target:** Engineering Leadership, Autonomous Self-Development Agents, and System Operators  
**Authority:** Derived from Apex Doctrine (`docs/choir-doctrine.md`), Product Vision (`docs/choir-vision.md`), Active Definitions, and Agentic Consensus Triad (2026-08-25)

---

## 1. Executive Orientation: The Automatic Computer

Choir is building the **Automatic Computer**: a persistent, personal computer for an individual user that continuously works, executes, and develops itself under supervised autonomy. The World Wire (an autonomous, verifiable, decentralized news and intelligence fabric) is strictly downstream of this single primitive. If the persistent self-developing computer does not function with verifiable safety, state monism, and deterministic rollback, the World Wire cannot exist.

```
+-----------------------------------------------------------------------------+
|                                CHOIR VISION                                 |
|                                                                             |
|   +---------------------------------------------------------------------+   |
|   |                       THE AUTOMATIC COMPUTER                        |   |
|   |  - Persistent ComputerID & Canonical Event Chain (Tape)             |   |
|   |  - Family A Computer Monism (All state reconstructed from Tape)     |   |
|   |  - Supervised Self-Development (CoSuper Capsule Authoring)          |   |
|   |  - Acceptance-Fenced Restores & Reversibility                       |   |
|   +---------------------------------------------------------------------+   |
|                                      |                                      |
|                                      v (Downstream)                         |
|   +---------------------------------------------------------------------+   |
|   |                           WORLD WIRE                                |   |
|   |  - Multi-Agent Continuous Synthesis & Verifiable Publishing         |   |
|   |  - Policy-Governed Public Projections & Consensus Settlement         |   |
|   +---------------------------------------------------------------------+   |
+-----------------------------------------------------------------------------+
```

### The Product Object vs. Implementation Machinery
- **The Product Object:** A persistent, durable computer identified by an immutable `ComputerID` and governed by a linear, append-only **Canonical Event Tape** (persisted in platform Dolt/Corpusd). The computer is the composite of its event chain, Dolt/object-graph state, file blobs, and cryptographic route identity.
- **The Implementation Machinery:** Firecracker MicroVMs, Autoputer guest daemons, VM controllers (`vmctl`), and Proxies. These are disposable realization engines. A MicroVM may crash, hang, or be garbage collected at any time; the computer's state is completely defined by its event tape and reconstructible via deterministic replay.

### Core Architectural Doctrines
1. **Family A Computer Monism:** Every behavior-bearing state (Texture documents, revisions, run memory, object graphs, desktops) belongs to an exact `ComputerID` and must be projectable from that computer's event tape alone. Tenancy by `OwnerID` is forbidden because it leaks state across computers.
2. **ReplayEventProjection Authority:** State projections must be derived from the event chain. Ad-hoc SQL tables, shadow registries, or direct database mutations that cannot be replayed from genesis are heresies.
3. **Problem-Documentation-First (Invariant I11):** When a new platform or substrate problem is discovered, the first commit must be a code-free evidence receipt recording the problem, symptoms, and causal hypotheses. Code fixes come second.
4. **Mutation Classes & Protected Boundaries:**
   - `Green`: Documentation and comments (no runtime behavior change).
   - `Yellow`: Tests, detector manifests, and prompt framing.
   - `Orange`: Runtime APIs, app state, database queries, and routing.
   - `Red`: Texture canonical writes, VM lifecycle (`vmctl`), auth/session renewal, route projection, checkpointing, and self-development promotion/restore.
   - `Black`: Production-destructive or irreversible actions.
5. **Reversibility as an Empirical Boundary:** Self-development modifications are authored in disposable guest capsules, frozen into `CapsuleEffectBundle`s, approved by qualified multi-agent consensus (`reversible-selfdev-v1`), promoted to active execution, and tested against pre-declared falsifications with verified rollback to an immutable pre-A checkpoint (`99949fe2`).

---

## 2. Current Station: Operational State on Staging (2026-08-25)

The platform is operating on staging (`https://choir.news`) on host `choiros-b`. Staging computer `computer-03335285269bdba4f94377e56879f9e6` is permanently sealed under host hold as an immutable historical evidence artifact; active overhaul and self-development execution transitions to fresh, snapshot-backed computers.

```
+-----------------------------------------------------------------------------+
|                          STAGING OPERATIONAL STATUS                         |
|                                                                             |
|  Host: choiros-b (Node B)                                                   |
|  Target Computer: computer-03335285269bdba4f94377e56879f9e6                 |
|  Active MicroVM: candidate-fleet-e15cb89f25d963c220319b7b (Epoch 794)       |
|                                                                             |
|  +---------------------------+       +-----------------------------------+  |
|  |       GUEST RUNTIME       |       |          PLATFORM TAPE            |  |
|  | - Status: LIVE & SERVING  |       | - Canonical Head: 133,319+        |  |
|  | - Port: 10.200.164.2:8085 | <---> | - Restored Ancestor Base: 132,539 |  |
|  | - Health: HTTP 200        |       | - Replay Delta: 132,540 -> 133,319|  |
|  | - Admission: FENCED       |       | - Pre-A Checkpoint: 99949fe2      |  |
|  |   (RUNTIME_MAINTENANCE_   |       | - Propose Only Mode: ACTIVE       |  |
|  |    HOLD=1 Active)         |       | - Effects Execution: OFF          |  |
|  +---------------------------+       +-----------------------------------+  |
|               |                                       |                     |
|               v                                       v                     |
|  +---------------------------+       +-----------------------------------+  |
|  |     VMCTL HOST STATE      |       |       PUBLIC ROUTE STATUS         |  |
|  | - Ownership: Active (794) |       | - Route: UNREGISTERED (404)       |  |
|  | - Host Hold: HELD (True)  |       | - Error: "computer route not      |  |
|  | - Safety: Deploy-Refresh  |       |   found" (Refresh blocked by      |  |
|  |   Skips This Computer     |       |   Heresy 3 Hang)                  |  |
|  +---------------------------+       +-----------------------------------+  |
+-----------------------------------------------------------------------------+
```

### Verified State Matrix
- **Guest Runtime:** Epoch-794 realization. Autoputer daemon is running on guest port `:8085` (HTTP 200). `RUNTIME_MAINTENANCE_HOLD=1` is active in the guest environment, safely refusing new run admissions and agent rewakes.
- **Platform Tape:** Canonical event chain has advanced past sequence **133,319** (`6e7424f0...`). The guest has re-driven pending worker updates and is actively appending new lifecycle and run-terminal events as the valid event authority.
- **Host State:** `vmctl` ownership is `state=active, epoch=794`. The host maintenance hold is set to `held=true` (`reason: protect-live-guest-during-hang-diagnosis`), ensuring that background deploys or health loops will not kill or recreate the live VM.
- **Public Route:** The platform route slot is currently **unregistered** (`GET /api/computers/computer-0333528...` returns HTTP 404: `computer route not found`). The route can only be registered through a completed owner refresh, which is currently blocked by Heresy 3.
- **Safety Gate:** Effects remain strictly **OFF**; self-development mode is `propose_only`; pre-A checkpoint `99949fe2` remains the unviolated rollback fence.

---

## 3. Incident Postmortem: The Substrate Cascades (Aug 19 – Aug 25)

Understanding how the system arrived at its current station requires tracing five sequential phases of substrate discovery, storm cascades, and recovery.

```
  Aug 19: Super Continuation Storms
  [9bc99f90] Persistent Super wakes on CoSuper cancel -> [3654d925] Claim producer_report_ids ->
  [5e01ac3a] Assign prompt & replacement flags -> [9a55b756] Omit claimed reports
         |
         v
  Aug 24: Replay Ceilings & Large-Disk Maintenance
  133k event replay hits 3-6s/ev ceiling + 4m TTL -> [b5cf67fd] Disable Dolt GC on guest ->
  [f7b3ccd2] B14 host-side replay-only boundary
         |
         v
  Aug 25 (06:53Z): Cold-Recover Failure & Pre-Genesis Loop
  Deploy [a29f52cc] -> Cold-Recover stages blank disk -> Guest loops on "pre-genesis refusal"
         |
         v
  Aug 25 (13:17Z): Manual B14 Ancestor Restore
  Swap ancestor image (132,539) -> Replay drive -> Serve under hold -> Appending past 133,318
  [d8c98d39] Record 3 Heresies: Blank Stage, Empty Credential, Normal-Boot Hang
         |
         v
  Aug 25 (16:00Z): Consensus Discovery of the refresh_runtime Confound
  [20701098] Panel proves hang is binary selection (store vs dynamic) via choir.refresh_runtime=1
```

### Phase 1: The Super Continuation Storms (Aug 19)
1. **The Defect:** When a CoSuper subagent exceeded its 200-iteration budget or timed out, the system persisted a cancellation `producer_report`. Commit `9bc99f90` wired persistent Super to wake on this report. However, because the continuation Super failed without claiming the report, `maybeContinuePersistentSuperInbox` repeatedly found the undelivered report and spawned a new Super every second, creating a storm loop (Runs `b57705fd` -> `999bd208` -> `483eec47`).
2. **The Fixes:** 
   - Commit `3654d925` claimed `producer_report_ids` in Super metadata so terminal continuation Supers would not restorm.
   - Commit `5e01ac3a` gave replacement Supers explicit `assign_co_super` prompts.
   - Commit `9a55b756` omitted voluminous claimed cancel-report bodies from prompts, avoiding `max_tokens` context exhaustion.

### Phase 2: Replay Ceilings & Disk Substrate Bottlenecks (Aug 24)
1. **Replay Latency:** As the staging computer's canonical tape grew past 133,000 events, guest-side replay throughput hit a ceiling of 27–82 events/sec (3–6 seconds per event batch). A full replay required 30–90 minutes.
2. **Credential TTL Race:** Guest credential envelopes had a short TTL (~4 minutes). When a boot/replay took longer than 4 minutes, the guest could not complete credential exchange, crashing the autoputer.
3. **Dolt GC Out-Of-Memory:** Dolt garbage collection on 4GiB guest RAM caused OOM kills mid-write, corrupting database headers. Commit `b5cf67fd` disabled Dolt GC inside the guest. Commit `f7b3ccd2` established the B14 host-side replay-only boundary to materialize projections offline.

### Phase 3: The Cold-Recover Blank-Disk Failure (Aug 25, ~06:53Z)
1. **The Action:** Commit `a29f52cc` deployed `RecoverVMForDesktopMaintenance` to allow authorized cold recovery of held computers. An operator triggered cold recovery via `POST /api/computers/{id}/lifecycle/cold-recover` (`rec-1-e32d7c7f4c5fc7e8`).
2. **The Failure:** `StageSparseImage` allocated a brand-new, completely **blank** 32GiB sparse `data.img`. When the guest booted, the local tape had no genesis record. The guest's appender refused to pull from the platform because the pre-genesis gate required a bootstrap chain, throwing the computer into a continuous pre-genesis refusal loop (`"computer is pre-genesis: run admission refused"`).

### Phase 4: The Manual B14 Ancestor Restore (Aug 25, ~13:17Z)
1. **The Rescue:** To recover from the pre-genesis loop without losing 133k events of history, the operator:
   - Restored the ancestor disk snapshot `data.img.pre-upgrade-20260824T074931Z` (which contained local history up to sequence 132,539, valid genesis, and DEKs).
   - Executed Boot #1 (Replay Drive): Booted with `choir.runtime_recovery_replay_only=1` and `choir.runtime_maintenance_hold=1`, with a hand-minted credential envelope and manual tap/NAT setup. The drive materialized local state and cleanly exited (`seq=0 committed=0`).
   - Executed Boot #2 (Serve Boot): Booted with `choir.runtime_maintenance_hold=1` (replay-only removed). The guest runtime started on `:8085`, caught up the 670-event delta (132,540 -> 133,209), loaded worker updates, and resumed appending new events up to 133,319+.
2. **The Problem Receipt (`d8c98d39`):** The operator recorded three new substrate heresies:
   - *Heresy 1:* `cold-recover` stages a blank sparse image that B14 replay cannot bootstrap.
   - *Heresy 2:* `cold-recover` formats `credential.img` empty (only `lost+found`), causing taped images to crash-loop on missing bootstrap envelopes.
   - *Heresy 3:* `vmctl`-managed normal boots of the ancestor image hang silently 3/3 before emitting any autoputer logs, while manual boots with `choir.runtime_maintenance_hold=1` booted 4/4.

### Phase 5: The Consensus Discovery (`20701098`)
An agentic consensus panel (10 models) analyzed Heresy 3 and uncovered a critical confound:
- `RUNTIME_MAINTENANCE_HOLD` is only checked deep inside `agentcore.Runtime.Start` (`runtime.go:583`), *after* database open and initialization logs. It cannot physically cause a pre-log boot hang.
- In `nix/autoputer-vm.nix:135-158`, when `choir.refresh_runtime=1` is present (which `vmctl` always injects on owner refreshes), the guest wrapper **deletes `/mnt/persistent/choir-updater/current`** and executes the immutable **store binary** (`a29f52cc`).
- On manual boots without `refresh_runtime=1`, the wrapper preserved `current` and executed the **old dynamic binary** from the persistent disk.
- Therefore, the true failure is that the **new store binary hangs at startup when opening the ancestor persistent store** (suspected silent migration, lock contention, or early initialization stall), while the old dynamic binary boots cleanly.

---

## 4. Agentic Consensus Triad Synthesis

The system was evaluated through three consecutive independent consensus passes:

```
+-----------------------------------------------------------------------------+
|                         AGENTIC CONSENSUS TRIAD                             |
|                                                                             |
|  [Pass 1: DIVERGENT]                                                        |
|  12 Models · 7 Lenses (newcomer, architect, skeptic, historian, operator...) |
|  Key Insight: "Identity Pile-Up" & the mathematical incompatibility of      |
|  linear event replay (133k events @ 3-6s/ev) vs. 4-min credential TTLs.     |
|                                                                             |
|  [Pass 2: LATERAL]                                                          |
|  12 Models · Frame Inversions & Systems Analogies                           |
|  Key Insight: Replay is audit/reconstruction, NOT a boot sequence.          |
|  Self-development is a software supply chain, NOT live surgery on patient. |
|  Split-plane architecture: immutable data kernel + ephemeral compute heads. |
|                                                                             |
|  [Pass 3: CONVERGENT]                                                       |
|  12 Models · Authoritative Action Plan                                      |
|  Key Insight: Unanimous agreement on the clean strategic sequence:          |
|  Substrate Cleanup -> Overhauls (K/F/M/Assurance) -> Yaegi -> Self-Dev.    |
+-----------------------------------------------------------------------------+
```

### Divergent Insights (Pass 1)
- **Identity Pile-Up:** A Choir computer is a composite of six distinct identities: ComputerID, realization epoch, public route slot, CodeRef, updater binary digest, and canonical tape head. Cascades happen when disagreement among these is treated as a single bug.
- **The Replay vs. TTL Mathematical Ceiling:** At 133k events, linear replay exceeds the 4-minute credential TTL. Every successful append pushes the computer further past the recovery horizon until $O(\Delta)$ differential replay is deployed.
- **The Break-Glass Dilemma:** SSH-based manual recovery procedures and hand-minted credentials saved the computer, but bypassed the product route registration, creating a split-brain where the computer is live internally but 404 to the world.

### Lateral Inversions (Pass 2)
- **Replay is Audit/Reconstruction, Not Boot:** Linear event replay at boot is an $O(N)$ anti-pattern. Boot must be $O(1)$ point-in-time snapshot loading + local delta projection.
- **Self-Development as a Supply Chain:** Authoring source code directly inside the user's primary MicroVM mixes container lifecycle with agentic code modification. Candidates belong in isolated ephemeral process capsules (or Yaegi actor capsules); only verified, signed bundles should be deployed to the persistent computer.
- **Split-Plane Architecture:** Decouple the durable data plane (the event tape, Dolt store, encryption keys) from the disposable compute heads (the autoputer runtime).

### Convergent Verdict (Pass 3)
- Unanimous consensus to **permanently seal `computer-0333528...` under host hold**, eliminate mutable dynamic binary selection, and cut over to the clean substrate overhauls on fresh snapshot-backed computers.
- Implement fail-closed gates for cold-recovery (refuse blank sparse disks; require regular credential envelope).
- Execute the clean strategic roadmap: Substrate Cleanup $\to$ Overhauls (Tracks K, F, M, Assurance) $\to$ Private-Go / Yaegi Actor Kernel $\to$ Self-Development resumption.
---

## 5. Strategic Roadmap & Upcoming Missions

The Choir development trajectory is structured into four distinct horizons:

```
  [COMPLETED] Audited Construction & Fleet Materialization (2026-07-17)
  [COMPLETED] Coherent Durable Computer Convergence (2026-07-24)
  [COMPLETED] Tape Recovery Substrate (2026-08-15)
       |
       v
  [ACTIVE MISSION] Substrate Cleanup & Cutover (2026-08-25)
  - Delete mutable dynamic binary fallback (/mnt/persistent/choir-updater/current)
  - Restore standard embedded Dolt GC in guest MicroVM environment
  - Delete choir.refresh_runtime=1 parameter and wrapper deletion logic
  - Seal computer-0333528 under permanent hold as an immutable historical artifact
  - Formally suspend self-development effects pending clean substrate overhauls
       |
       v
  [QUEUED OVERHAUL] Durable Substrate Overhauls (2026-08-23)
  - Track K (Kernel/Replay): Passkey PRF / Key Escrow / Dual-Approval Recovery
  - Track F (Files/Frontend): Content-Addressed File CAS / Merkle FileRootCommitted / O(Delta) Replay
  - Track M (Mail/Memory): Per-Computer MTA Spool -> Guest Maildir Integration
  - Track Assurance: Portable Capsule Bundles / Daily Restore Drills / Recovery Cells
       |
       v
  [NEXT ARCHITECTURE] Private-Go / Yaegi Actor Kernel (2026-08-12 Definition)
  - Yaegi Interpreter inside isolated Guest Capsules (Strict Memory & Tool Sandboxing)
  - Typed Obligation Projections replacing sprawling tool loops
  - Exact one Super per ComputerID + Structured Consensus Seats
       |
       v
  [RESUMED MISSION] Supervised Self-Development Effects (Candidate A/B Solitaire Proof)
  - Execute on fresh, snapshot-backed, Yaegi-powered computer
  - Pre-A Checkpoint: 99949fe2 (Replaced by clean initial checkpoint)
  - CoSuper Authors Candidate A (pre-declared foundation defect) -> Consensus -> Promotion -> Restore
       |
       v
  [LONG-TERM HORIZON] The World Wire
  - Multi-Agent Continuous Synthesis & Verifiable Publishing
  - Downstream of the Supervised Automatic Computer
```

### The Candidate A / B Solitaire Rehearsal Specification
The self-development effects mission executes an end-to-end rehearsal using a standalone Solitaire game engine (`/api/solitaire`):
- **Candidate A:** CoSuper authors additive `/api/solitaire` routes and SQL tables with a **pre-declared foundation defect** (the card validator correctly checks ascending rank from Ace, but omits the matching suit check, allowing e.g. 2♠ on A♥). Candidate A's unit tests cover standard legal moves and intentionally pass.
- **Promotion & Play:** Candidate A is frozen into a 5-ref `CapsuleEffectBundle`, approved by qualified consensus, and promoted. Adversarial play submits an off-suit move, which is accepted by the buggy engine, generating an admissible falsification receipt.
- **Candidate B:** CoSuper authors Candidate B citing the falsification receipt, repairing the suit check (`card.Suit == foundation.Suit`). Promotion is approved, and the same off-suit move now returns HTTP 400.
- **Total Restore:** The operator issues a restore to pre-A checkpoint `99949fe2`. Verification proves `/api/solitaire` returns 404, database tables are completely absent, content root matches `c302f6d9...`, while the canonical event tape durably retains the entire self-development excursion.

---

## 6. The Actionable Master Execution Plan

```
+-----------------------------------------------------------------------------+
|                         CLEAN STRATEGIC PIPELINE                            |
|                                                                             |
|  [Step 1] SUBSTRATE CLEANUP & CUTOVER                                       |
|           - Delete choir-updater/current dynamic fallback in autoputer-vm.  |
|           - Re-enable standard embedded Dolt GC in guest environment.       |
|           - Delete choir.refresh_runtime=1 argument formatting.             |
|           - Seal computer-0333528 under permanent host hold.                |
|                                                                             |
|  [Step 2] DURABLE SUBSTRATE OVERHAULS (Tracks K -> F -> M -> Assurance)     |
|           - Track K: Passkey PRF / Key Escrow on fresh computer creation.   |
|           - Track F: File CAS, Merkle FileRootCommitted, ProjectionBase.    |
|             Enables O(Delta) instant sub-second boot and recovery.         |
|           - Track M: Per-computer mail spool to guest Maildir.              |
|           - Track Assurance: Daily restore drills and integrity scrubbing.  |
|                                                                             |
|  [Step 3] PRIVATE-GO / YAEGI ACTOR KERNEL                                   |
|           - Replace 200-iteration shell loops with structured Yaegi         |
|             interpreted Go capsules and typed obligations.                  |
|           - Eliminates orchestration storms and cancellation leaks.         |
|                                                                             |
|  [Step 4] RESUME SUPERVISED SELF-DEVELOPMENT                                 |
|           - Resume Candidate A/B Solitaire self-development rehearsal on a  |
|             rock-solid, snapshot-backed, Yaegi-powered computer.            |
|           - Clean proof: Capsule build -> Consensus -> Promotion -> Restore.|
+-----------------------------------------------------------------------------+
```

### Detailed Operational Steps

#### Step 1: Substrate Cleanup & Cutover (Active Mission)
- **Action:** In `nix/autoputer-vm.nix`, simplify `autoputerRuntimeExec` to directly execute the store binary `${goChoirPackages.autoputer}/bin/autoputer`. Remove `RUNTIME_DOLT_GC_DISABLED = "1";`. In `internal/vmmanager/manager.go`, remove `choir.refresh_runtime=1`.
- **Safety:** Eliminates dual-binary split-brain and restores normal GC for all future computers.

#### Step 2: Durable Substrate Overhauls (Track K & Track F)
- **Action:** Execute `docs/definitions/choir-durable-substrate-overhauls-2026-08-23.md` on a freshly provisioned test computer. Implement key escrow (Track K) and $O(\Delta)$ File CAS with Merkle snapshots (Track F).
- **Safety:** Computers boot in <2 seconds from snapshot bases, completely eliminating 133k-event linear replay.

#### Step 3: Private-Go / Yaegi Actor Kernel
- **Action:** Execute `docs/definitions/choir-private-go-actor-kernel-2026-08-12.md`. Replace brittle subagent bash loops with structured in-process Yaegi Go activations.

#### Step 4: Resume Supervised Self-Development
- **Action:** Re-open candidate proof on the clean foundation: CoSuper authors Candidate A (with pre-declared foundation defect) in an isolated Yaegi capsule -> qualified consensus -> promotion -> falsification with Candidate B -> acceptance-fenced total restore.
*Report approved by convergent agentic consensus panel (12 models). Ready for executive review and operational execution.*
