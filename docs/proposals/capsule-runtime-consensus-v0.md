# Capsule Runtime Design Consensus — v0

**Status:** Synthesis of three independent external agent review threads.
**Date:** 2026-07-05
**Mutation class:** green (design documentation, no runtime change)

## Methodology

Three independent review threads answered all 8 unresolved design questions,
each from a different perspective:
- **Thread 1:** Architecture & security principles
- **Thread 2:** Security-focused, opinionated, no hedging
- **Thread 3:** Pragmatic production engineering

## Consensus Matrix

| Q | Topic | Thread 1 | Thread 2 | Thread 3 | Result |
|---|-------|----------|----------|----------|--------|
| Q1 | Broker placement | B (bind-mount) | A (bake in EROFS) | B (bind-mount) | RESOLVED → A |
| Q2 | EROFS sharing | A (mount once) | A (mount once) | A (mount once) | CONSENSUS |
| Q3 | Network | D (air-gapped) | D (air-gapped) | D (air-gapped) | CONSENSUS |
| Q4 | User namespace | B (root in ns) | B (root in ns) | B (root in ns) | CONSENSUS |
| Q5 | File tools | A (through broker) | A (through broker) | A (through broker) | CONSENSUS |
| Q6 | Cosuper sharing | A (shared upperdir) | B (per-cosuper) | B (per-cosuper) | RESOLVED → Hybrid |
| Q7 | Network egress | C (allowlist) | A (deny all) | C (allowlist) | MOOT (Q3=D) |
| Q8 | Resource limits | C (tiered presets) | C (tiered presets) | C (tiered presets) | CONSENSUS |

All 8 questions resolved. 6 by unanimous consensus, 2 by user decision.

---

## Resolved Decisions (Unanimous Consensus)

### Q2: EROFS Base Sharing — Mount Once at Boot, Share Across Capsules

**All three threads agree.**

**Decision:** Mount the EROFS base image once at VM boot. All capsules share
this single mount point as their overlayfs lowerdir.

**Principles:**
- Page cache efficiency: single mount = shared page cache. Per-capsule mounts
  would duplicate page cache N times (2GB base × 10 capsules = 20GB wasted).
- Refcounting simplicity: kernel handles mount refcounting. No userspace
  refcounting bugs.
- Fail fast: mount failure detected at boot, not at capsule spawn time.
- Standard pattern: Docker, containerd, Podman all share read-only layers.

**Trade-off accepted:** Cannot update EROFS base without VM reboot. Acceptable
because base updates require VM restart anyway for full state consistency.

### Q3: Network Connectivity — Air-Gapped, No Network Namespace

**All three threads agree.**

**Decision:** Capsules have no network access. Either no network namespace at
all, or a network namespace with only loopback. All network I/O is mediated by
the host (VM agent) via files or explicit proxy tools.

**Principles:**
- Default-deny is the only secure default for AI agent execution.
- Network access is a privilege, not a right. If needed, it should be an
  explicit, audited tool (e.g., `http_fetch` tool that proxies through host).
- Eliminates veth/bridge/nftables complexity (hundreds of lines, many failure
  modes).
- File-based I/O is fully auditable via overlay diff. Network I/O is ephemeral.
- Forces all network traffic through the host where it can be logged,
  rate-limited, and audited.

**Trade-off accepted:** Cosupers cannot run `curl`, `git clone`, `npm install`
directly. Package dependencies must be baked into EROFS base or pre-fetched by
host. Git repos must be pre-fetched by host. API calls must go through
host-provided proxy tools.

**Implication for Q7:** Since capsules are air-gapped, the network egress
allowlist question becomes about the host-side proxy, not the capsule. The
host enforces its own policy for outbound requests. This is already handled by
the existing VM network configuration.

### Q4: User Namespace — Root Inside Namespace, No User Namespace

**All three threads agree.**

**Decision:** Capsules run as root inside their namespace. No user namespace
(uid/gid mapping).

**Principles:**
- The threat model is capsule escape, not privilege escalation within the
  capsule. The capsule is already inside a Firecracker VM.
- User namespaces add significant complexity (uid/gid maps, subordinate
  ranges, /etc/subuid, /etc/subgid, setuid binary handling) without addressing
  the actual threat.
- Overlayfs copy-up with user namespaces has edge cases and kernel bugs.
- Docker and containerd default to root-in-namespace for most workloads.
- Rootless is for multi-tenant hosts, not single-tenant VMs.
- The elevated capabilities (CAP_DAC_OVERRIDE, CAP_FOWNER) are already
  required; user namespaces wouldn't improve the security posture.

**Trade-off accepted:** A compromised cosuper has root within the capsule.
This is already true due to CAP_DAC_OVERRIDE + CAP_FOWNER. The VM boundary is
the real security boundary.

### Q5: File Tools Routing — Through Exec-Broker

**All three threads agree.**

**Decision:** All file operations (write_file, edit_file, read_file) route
through the exec-broker via JSON-RPC, same as bash commands.

**Principles:**
- Consistency: all file operations go through the same path, ensuring
  consistent overlayfs semantics, attribution, and auditability.
- Overlayfs semantics: direct upperdir access from host bypasses whiteout
  handling, opaque directory xattrs, and copy-up semantics. The broker
  mediates through the merged view.
- Attribution: file operations routed through broker can be logged with
  cosuperRunID, command context, timestamps.
- Namespace isolation: direct upperdir access bypasses the capsule's namespace
  isolation, weakening the architecture.

**Trade-off accepted:** ~0.5ms latency per file operation (Unix socket
round-trip). Acceptable for agent workloads (tens to hundreds of file ops).

**Build cost:** ~200 lines of Go in the broker for file tool handlers.

### Q8: Resource Limits — Tiered Presets

**All three threads agree.**

**Decision:** Capsule resource limits use tiered presets (small/medium/large),
not fixed defaults or arbitrary per-capsule configuration.

**Principles:**
- Fixed defaults are too rigid for diverse workloads.
- Per-capsule configuration adds too many knobs for the super to reason about.
- Tiered presets provide bounded configuration with known trade-offs.
- Matches cloud computing instance type patterns.

**Presets:**
| Tier | Memory | CPU | PIDs | Disk (tmpfs) |
|------|--------|-----|------|--------------|
| small | 512MB | 0.5 | 100 | 1GB |
| medium | 1GB | 1 | 200 | 2GB |
| large | 2GB | 2 | 500 | 4GB |

**Trade-off accepted:** Workloads that don't fit any tier will fail. The tier
system is extensible — new tiers can be added.

**Build cost:** ~50 lines of Go (preset definitions, mapping to cgroup values).

---

## Split Decisions (Require Resolution)

### Q1: Exec-Broker Binary Placement

**Thread 1 (arch):** B (bind-mount from host)
**Thread 2 (security):** A (bake into EROFS)
**Thread 3 (production):** B (bind-mount from host)

**The tension:** Is the broker part of the tape-derived state or runtime
infrastructure?

**Argument for A (bake into EROFS):**
- The broker is trusted code — part of the trusted computing base.
- Baking it into the EROFS base means it's content-addressed and dm-verity
  sealed. Any tampering is detectable.
- Bind-mount creates a supply chain attack vector: a compromised host could
  swap the broker binary.
- Broker changes should go through the same materialization and verification
  pipeline as the base — they're substrate-level changes.
- "Production-only, no fallbacks" means the broker should be sealed, not
  injectable.

**Argument for B (bind-mount from host):**
- The broker is runtime infrastructure, not tape-derived state. Conflating
  them means broker bug fixes require full EROFS rebuild + re-seal.
- The EROFS base represents immutable tape state. The broker is mutable
  runtime code. They have different lifecycles.
- Bind-mount allows debugging: swap broker for instrumented version without
  rebuilding the sealed base.
- Bind-mount allows fast recovery: if broker crashes, restart with patched
  version immediately. With A, you'd rebuild EROFS + reboot VM.
- The host already controls the VM agent, capsule spawning, and cgroup
  configuration. If the host is compromised, the game is already over. The
  broker binary is not a new trust boundary.

**Key question that resolves this:** Is the broker version part of the
reproducibility contract? If the tape says "this state was produced with
broker v1.2.3," then the broker must be content-addressed (A). If the broker
is just a transport mechanism (like a network driver) whose version doesn't
affect the audited state, then B is correct.

### Q6: Cosuper Sharing Model

**Thread 1 (arch):** A (shared upperdir, per-cosuper shells)
**Thread 2 (security):** B (per-cosuper capsules, separate upperdirs)
**Thread 3 (production):** B (per-cosuper capsules, separate upperdirs)

**The tension:** Collaboration vs isolation.

**Argument for A (shared upperdir):**
- Parallelism: cosupers work simultaneously on the same filesystem.
- Simplicity: single diff at commit time, no merge logic.
- Coagent model alignment: Choir's cosupers are independent goroutines working
  toward a common goal. Shared filesystem matches this.
- Cosupers can collaborate: one cosuper's changes are visible to another.
- Quiescence is simpler: freeze all shells, capture one diff.

**Argument for B (per-cosuper capsules):**
- Attribution: each cosuper's changes are isolated. Trivial to attribute.
  With A, attribution requires correlating file timestamps with shell logs
  (fragile).
- Isolation: one cosuper cannot overwrite another's work. With A, last writer
  wins silently — dangerous for auditable state.
- No merge conflicts: each cosuper's diff is independent. With A, two cosupers
  writing the same file create conflicts with no defined resolution.
- The documents don't define a merge strategy for shared upperdir — this is a
  gap.
- "Production-only" means the dangerous silent-overwrite behavior of A is
  unacceptable.

**Key question that resolves this:** Do cosupers need to see each other's
changes in real-time? If yes, A. If they work independently and coordinate
through the super, B.

---

## Moot Decision

### Q7: Network Egress Allowlist

**Thread 1:** C (explicit allowlist)
**Thread 2:** A (deny all)
**Thread 3:** C (explicit allowlist)

**Moot because Q3=D (air-gapped).** Since capsules have no network, the
egress allowlist is about the host-side proxy, not the capsule. The host
enforces its own policy for outbound requests. This is already handled by the
existing VM network configuration.

If a host-side proxy tool is built later (e.g., `http_fetch`), it should use
an explicit allowlist (C) per threads 1 and 3. Thread 2's deny-all stance
applies to the capsule, not the host proxy.

---

## Summary of All Resolved Design

| Component | Decision | Principle |
|-----------|----------|-----------|
| Broker placement | Bake into EROFS | Broker is Go code, not special infra |
| EROFS sharing | Mount once at boot, share | Page cache efficiency |
| Network | Air-gapped, no network ns | Default-deny, auditability |
| User namespace | Root in namespace | VM boundary is real isolation |
| File tools | Through exec-broker | Consistency, overlay semantics |
| Cosuper sharing | N cosupers per M capsules (hybrid) | Super-controlled topology |
| Resource limits | Tiered presets (S/M/L) | Bounded configuration |
| Network egress | Moot (air-gapped) | Host proxy handles if needed |

## Resolved Splits

### Q1: Exec-Broker Binary Placement — Bake Into EROFS

**Decision: A (bake into EROFS).**

The broker is just Go code we write — no different from any other code in the
system. It gets parameterized/templated and goes through the same
materialization pipeline as everything else. It is not special runtime
infrastructure. Broker updates require EROFS rebuild, which is the same
pipeline as any other code change to the base.

This means:
- The broker is content-addressed and dm-verity sealed with the base.
- No supply chain attack vector via broker binary substitution.
- Broker version is part of the reproducibility contract.
- Broker updates go through the full tape → EROFS materialization pipeline.
- Parameterization/templating handles any per-capsule configuration needs.

### Q6: Cosuper Sharing Model — N Cosupers per M Capsules

**Decision: Hybrid (super-controlled topology).**

Neither pure shared-upperdir nor pure per-cosuper capsules. The model is:

- M capsules exist, each with its own overlay upperdir.
- N cosupers are distributed across M capsules by the super.
- Multiple cosupers can share a capsule (shared upperdir within that capsule).
- Other cosupers can have their own capsule (full isolation).
- The super decides the cosuper-to-capsule mapping at spawn time.

This gives the super full control over the collaboration topology:
- 1 capsule, 1 cosuper = full isolation (per-cosuper capsule)
- 1 capsule, N cosupers = full collaboration (shared upperdir)
- M capsules, N cosupers = workgroups (some cosupers collaborate, others isolated)

**Attribution:** At the capsule level, attribution is trivial (which capsule
made this change). Within a shared capsule, attribution is via timestamp
correlation with shell logs. The super knows which cosupers are in which
capsule, so capsule-level attribution is always clean.

**Implementation:** The `SpawnSpec` gets a `CosuperIDs []string` field. The
`spawn_cosuper` tool gets a `capsule_id` parameter. The super assigns cosupers
to capsules explicitly.
