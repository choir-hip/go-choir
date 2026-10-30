# Outboard capsules: leased leaf machines for agent trees (ARCH-11, ARCH-12), design (2026-10-22)

Station: [FP10](../definitions/choir-launch-fp10-arch-11-12-2026-10-21.md).
Problem: [agent-tree-leaves-share-the-computer-guest-2026-10-22.md](../problems/agent-tree-leaves-share-the-computer-guest-2026-10-22.md).
Checklist: ARCH-11, ARCH-12, DEC-05 in
[security-density-architecture-checklist-2026-10-21.md](../security-density-architecture-checklist-2026-10-21.md).
Status: design only, P2, not launch work. Mutation class of this file:
green. Every slice below is orange or red and names its ceremony.

"Outboard capsule" is a **proposed** name (owner question OQ-1). Until
the owner rules, read it as a placeholder.

## 1. What ARCH-11 asks for

The checklist line: "Ephemeral, one per (tenant, taint class), cloned from
a snapshot, no persistent state or authority, a lease per tree; they run
untrusted content leaves and heavy compute and return proposed acts."
ARCH-12: the same machines on other hosts (SP4, after S9; needs ARCH-07).

The owner's direction behind it (owner notes §36): a dense multi-agent
system per user where "it would be very hard to escape every layer", and a
research tree of "100 or 1,000 leaf agents ... across processes, whether
they be capsules or across VMs, and then eventually across hosts".

## 2. The ontology gap and the proposed name

Today there are two kinds of disposable or sibling execution:

| | Capsule (today) | Forked computer (S9) | **Proposed: outboard capsule** |
|---|---|---|---|
| Where it runs | inside the computer's guest | its own microVM | its own microVM, on this or another host |
| ComputerID, event chain | none (the parent's) | its own | **none** |
| Identity material | none | freshly minted | **none**; a per-lease token only |
| Store, tape, route | none (writes go through the parent's apply) | its own | **none** |
| Output | a CapsuleEffectBundle or artifacts the parent applies | publications another computer adopts | **proposed acts** the leasing desk validates and commits |
| Lifetime | one assignment | ephemeral or persistent | **one lease, one tree** |

The new machine is a capsule in every respect but location: disposable,
authority-less, its effects enter the computer only through the computer's
own writer. It is not a fork: it has no identity and never becomes a
computer. Calling it a capsule keeps one concept for "disposable, returns
proposals" and the word *outboard* says the boundary is a microVM outside
the guest rather than a namespace inside it.

Names considered and rejected:

- **executor VM** (the checklist's working title): "executor" already names
  the per-computer vmctl executor (`docs/vmctl-360-review-2026-10-10.md:140`)
  and the guest capsule executor (`internal/capsule/executor.go`).
- **worker VM**: retired (`docs/computer-ontology.md`, Naming Rules).
- **ephemeral fork**: an ephemeral fork is a real computer with identity
  (ontology); this machine must never have one.
- **satellite VM**: workable, but adds a second concept beside capsule.

Ontology edit owed once the owner rules (green, not made here because
the name is open): a Naming Rules line, "Use **outboard capsule** for an
authority-less microVM a computer leases for one agent tree; it has no
ComputerID, identity material, store, tape or route, and its output enters
the computer only as proposed acts the leasing desk commits. It is never a
fork and never a self-development candidate."

## 3. The design

### 3.1 The lease

A lease is the only authority an outboard capsule ever has. Fields:

- `lessor_computer_id`, `tree_id` (the root run of the tree), `desk`
  (the leasing desk, e.g. research);
- `taint_class` (ARCH-05; for the first slice only `untrusted_web`);
- `budget` (a slice of the tree's budget: tokens, wall time, egress
  bytes, memory; ARCH-08, reconciled with DEC-07 "track spending, never
  cap");
- `expires_at`; `lease_token` (random, 32 bytes, held by the host relay
  and injected into the machine after boot or restore, never written to a
  snapshot);
- `host` (ARCH-12: which host realizes it).

The lease is a host record (vmctl), not a tape object: it carries no
product meaning beyond the tree. The tree's own run records, on the
lessor's tape, name the lease id so the ledger shows where each leaf ran.

### 3.2 One machine per lease, pooled templates per (tenant, taint class)

The checklist says "one per (tenant, taint class) ... a lease per tree".
Read literally, one machine serves many trees of one tenant and class in
turn. That lets tree A leave residue (files, page cache, a poisoned
process) for tree B. Proposed default (OQ-3): **a fresh machine per
lease**, and the (tenant, taint class) key names the **warm template**
the machine is restored from. Snapshot restore makes a fresh machine cheap
enough that reuse buys little.

Leaves of one tree share their lease's machine (one microVM, many leaf
processes, each in its own capsule namespace inside it). A tree that
outgrows one machine takes a second lease; the tree's budget bounds how
many.

### 3.3 Templates and DEC-05

A template is a booted, idle outboard image with **no tenant data** in it,
captured with the existing snapshot path
(`internal/vmmanager/snapshot_lifecycle.go:110`, `fcapi.go:124,143`).
Restoring many machines from one template shares its memory pages
copy-on-write. Shared pages between tenants are the side channel DEC-05
names (same class as KSM; Flush+Reload on shared pages,
`docs/archive/reports/computer-isolation-security-review-2026-10-14.md:1125`).
**DEC-05's default stands:** a template is per tenant (captured for that
tenant, restored only for that tenant's leases), or the machine cold boots.
A cross-tenant template is a later option only if the owner rules DEC-05
otherwise after the security panel.

Every restore must also make the machine unique before it does anything:
fresh entropy (VM generation id or an injected seed) so clones do not
share RNG state; the lease token injected after restore; the clock reset.
A restore that cannot confirm all three is refused and the lease cold
boots. The current resume path validates launch identity and mints a fresh
credential envelope after validation
(`snapshot_lifecycle.go:330-338`); outboard restore is the same shape with
a lease token instead of a computer credential.

### 3.4 What the machine can reach

Nothing by default. All traffic goes through a host relay over vsock
(the host already assigns vsock ports per VM, `internal/vmmanager/manager.go:116,275`):

- **To the lessor:** only frames on the relay, authenticated with the
  lease token (ARCH-07's versioned, authenticated frames are a hard
  prerequisite; the 8-byte unauthenticated header,
  `internal/yaegikernel/transport.go:14-23`, is not used across a VM
  boundary). The machine never gets a tap route to the lessor's guest, to
  other guests or to host-private services. S9 already records that tap
  forwarding is broad today (`internal/vmmanager/manager.go:2686-2733`);
  outboard machines get no tap device at all.
- **Models:** the relay forwards model calls to the host gateway billed to
  the lessor, admitted against the lease's budget. The machine holds no
  gateway bearer (OQ-2). Requires SEC-05 (the bearer bound to its
  computer) so the relay can act for exactly one lessor.
- **The web:** through NET-01's host fetch service, under the lessor's
  policy and the taint class ("Networking must enable, not starve": research
  reads stay unrestricted by allowlist).
- **Inputs:** only what the leasing desk puts in the leaf request, by
  value or content hash. No store, no tape, no Dolt, no credential disk.

### 3.5 What comes back: proposed acts

A leaf ends by returning a **proposed act**: a typed record (findings with
sources for research; an artifact by content hash for heavy compute) with
a size cap. The relay delivers it to the leasing desk; the desk validates
it and, if it accepts, commits it as its own act. The leasing desk stays
the sole writer of its kinds (ARCH-04; Texture stays the sole writer of
canonical revisions; research keeps read-only world authority). Nothing
an outboard capsule returns is ever applied without a desk's commit, and
nothing is merged into a store: that is the same rule as forks
(publication and adoption), applied within one computer.

Taint travels with the proposal: an act built from an `untrusted_web`
leaf carries that class into the desk's context (ARCH-05).

### 3.6 Lifecycle and the restart rule

- A lease ends when its tree ends, its budget or `expires_at` runs out, or
  the lessor cancels; the machine is destroyed and its disk discarded.
- **Crash:** a host or vmctl crash closes every open lease with the fate
  "interrupted by a restart" and destroys the machines at boot. There is
  nothing to resume; outboard capsules hold no state.
- **Planned restart:** leases are closed the same way. The lessor's tree
  sees its leaf as interrupted; whether the tree retries is the desk's
  choice after its own planned-restart resume, not a host resumer.
- **Lessor restart:** a lessor that restarts (crash or planned) has its
  leases closed by the host.
- **Admission:** a lease is admitted by vmctl's host admission like any
  start (`internal/vmctl/host_admission.go`), as the lowest class:
  reclaimed first under pressure (before any computer, including ephemeral
  forks), never protected.

### 3.7 Other hosts (ARCH-12)

An outboard capsule has no state and no identity, so placing it on another
host is far simpler than moving a computer (SP4): only the lease and the
relay cross. Requirements:

1. ARCH-07 frames carried over TLS between host relays, with mutual host
   authentication (host identity from SP0's mesh);
2. the remote host's gateway forwarding still bills the lessor and checks
   the lease budget at the lessor's host (one budget authority);
3. NET-01 fetch on the remote host under the lessor's policy;
4. template rules unchanged: per tenant, now also per host (a snapshot is
   valid only for the same guest image and hypervisor version,
   ontology "machine snapshot");
5. SP4 and S9 ordering stands: ARCH-12 comes after S9, as the checklist
   says, because SP4's mesh, host identity and placement operation are
   shared with fork placement.

## 4. Failure modes (written before any code, per CLAUDE.md Testing)

| # | Failure | Guard |
|---|---|---|
| F1 | Machine reaches the lessor guest, another guest or a host service | no tap device; vsock relay only; probe in O2 |
| F2 | Machine forges frames as another lease or desk | ARCH-07 per-lease token; relay binds token to (lessor, tree, desk) |
| F3 | A clone shares RNG state or a token with its template or siblings | entropy reseed and post-restore token injection, or refuse and cold boot |
| F4 | Cross-tenant page sharing via template | per-tenant templates (DEC-05 default) |
| F5 | Tree B reads tree A's residue | fresh machine per lease (OQ-3 default) |
| F6 | Proposed act applied without a desk commit | relay delivers only to the leasing desk's inbox; no write path exists |
| F7 | Leaf exhausts the lessor's model budget (cf. 2026-10-11 fan-out record) | lease budget slice admitted at the gateway; tree budget bounds lease count |
| F8 | Leases resume after a crash | boot closes every lease; no outboard resumer exists |
| F9 | Outboard machines starve computers of memory | lowest admission class, reclaimed first |
| F10 | Oversized or malformed return | size cap and schema check at the relay; desk validates again |
| F11 | Lease outlives its lessor or tree | host closes leases on lessor stop and tree end; expiry as backstop |
| F12 | Remote host impersonated (ARCH-12) | mutual host TLS; lease token never sent to an unauthenticated host |

## 5. Slices

| Slice | What | Class | Touches | Prerequisites |
|---|---|---|---|---|
| O0 | Ontology line and doctrine note once OQ-1 is ruled | green | docs | owner |
| O1 | A `LeafRunner` seam in agentcore: research children go through it; the only implementation is today's in-guest activation. Behavior unchanged; proposed-act shape defined. | orange | `internal/agentcore` only | none; buildable now without FP8's files |
| O2 | vmctl outboard class: cold boot, one host, `untrusted_web` only, test computers only, off by default (`VMCTL_OUTBOARD_MODE=off`); relay over vsock; probes F1, F2, F6, F8 | red | vmctl, vmmanager, image, relay | FP8 landed; ARCH-05, ARCH-07, SEC-05, NET-01 |
| O3 | Per-tenant templates with reseed and post-restore token (F3, F4) | red | vmmanager snapshot path | O2; RES-05 jailer sharing |
| O4 | Lease budgets from the tree (F7) | red | gateway, agentcore | ARCH-08, DEC-07 |
| O5 | Heavy-compute leaves (ARCH-09 kinds) | red | capsule kinds | O2, ARCH-09 |
| O6 | Other hosts (ARCH-12) | red | relay, placement | S9, SP4, O2-O4 |

The smallest useful first code slice is **O1**: it does not touch FP8's
files, it fixes the shape (lease, proposed act) the rest builds on, and
it changes no behavior. It is left for a later pass because this station
is design only.

Red ceremony for O2 onward (to be restated in each slice's commits):
conjecture delta as in the station file; protected surfaces vmctl,
guest boundary and egress, gateway and provider calls; admissible evidence
is a deployed proof on a test computer (the F1, F2, F6, F8 probes from
inside a leased machine, run by FP0) plus local relay tests; rollback is
the mode flag off, then git revert; heresy delta recorded per slice.

## 6. Standing questions, answered for this design

- **Decision provenance:** owner notes §36; SDC ARCH-11/12; DEC-05.
- **Settled-decision conformance:** ontology (no worker VM; forks keep
  identity); CLAUDE.md Restarts (no resumer); desk ontology (sole
  writers); DEC-05 default.
- **Single state authority:** the lessor's tape for the tree; vmctl for
  the lease; the machine holds no state.
- **Artifact-verified success:** a proposed act committed by the desk,
  read back from the tape, plus the refusal probes.
- **Fate-sharing:** a lease shares its lessor's and its host's fate,
  never the reverse; an outboard crash closes one leaf.
- **Restart durability:** leases are deliberately not durable.
- **No-SSH operability:** leases visible in vmctl's admission and health
  reporting (owed in O2).

## 7. Open owner questions

- **OQ-1 (name):** "outboard capsule" for the authority-less leased
  microVM? Alternatives: "satellite VM"; keep "executor" (not
  recommended: collides with two existing executors).
- **OQ-2 (model access):** model calls relayed through the host gateway
  billed to the lessor, with no bearer in the machine (recommended), versus
  a lease-scoped bearer inside the machine.
- **OQ-3 (reuse):** a fresh machine per lease, with (tenant, taint class)
  naming the template (recommended), versus one long-lived machine per
  (tenant, taint class) serving many trees, as the checklist line reads.
- **OQ-4 (DEC-05):** unchanged; this design assumes the default (no
  cross-tenant templates) and needs no ruling to proceed.

## 8. Residuals

- How many leaves fit in one outboard machine, and its memory shape, is
  unmeasured; O2 measures it (density report lever list).
- Whether heavy compute (O5) wants a larger machine class or a GPU host
  (Cloud Hypervisor VFIO, RES-20) is open.
- PD4 (one computer drives another) is a different relation: computer to
  computer, both with identity, by publication and adoption. Outboard
  capsules do not replace it.
