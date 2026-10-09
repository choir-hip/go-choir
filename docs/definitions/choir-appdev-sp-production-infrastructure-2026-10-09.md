---
definition_version: 4

# SP — Production infrastructure. Station of the supervised app-development
# metamission. DEFERRED. Owner 2026-10-09, in reply to the proposed
# architecture: "I like the architecture, but it's definitely to be
# deferred, along with the other deferred optimizations. Gate 1 doesn't
# require it. It's important that we prove the system for human qa before I
# invest more money into it, and before we make the infrastructure more
# complex (and thus harder to manage)." Then: "But do document sp."
# This file records the design so it is not lost. It is not executable.
readiness: intent

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-09T02:00:00Z
  source:
    canonical_ref: main 02beedb7
    deploy_identity: >-
      staging choir.news: guest autoputer c72c38c4 (partial app-layer push);
      vmctl, corpusd, proxy eedd7b22
  observed:
    - >-
      Node B is the only platform host: OVH bare metal, 12 cores, 31 GiB
      RAM, 2 x 512 GB NVMe in software RAID 1 (/dev/md127), root 81% used
      (93 GB free) on 2026-10-09. It runs proxy, auth, gateway, maild,
      corpusd, platform-dolt (Store A), corpus-dolt (Store B) and vmctl with
      every guest VM.
    - >-
      Every durable home (tape, content store, key escrow, control store)
      and every log lives on that one disk pair. Operational invariant O22
      (docs/operational-invariants-register-2026-10-08.md) names the gap.
    - >-
      Deploys restart host services in place; guest updates go through the
      S2 app-layer push. Neither is zero-downtime today.
    - >-
      Owner 2026-10-09: a nearly identical second machine is available;
      more OVH bare metal can be rented; logging and monitoring must be
      self-hosted on machines the owner controls, not SaaS.

finish:
  deliver: >-
    Losing a platform host loses nothing and stops nothing for long:
    every durable home has an off-host copy, every log and metric reaches a
    second machine, deploys drop no requests, and the audit record is kept
    on cold storage.
  artifact: >-
    NixOS host roles from the one flake, a WireGuard mesh, off-host
    replicas of Store A, the content store and the escrow material, a
    self-hosted log/metric/alert stack, a zero-downtime deploy path, and
    three recorded drills (below).
  acceptance:
    - >-
      Restore drill: a platform rebuilt on the second host from off-host
      copies only reports the same event heads, append receipts and escrow
      transparency head as the source, and realizes a disposable computer
      that reads its own files (O21 + O22).
    - >-
      Deploy under load: a full platform deploy while a synthetic client
      drives the owner-facing API completes with zero failed requests and
      no guest VM restarted except by the app-layer push.
    - >-
      Host loss: Node B powered off; the platform serves again from the
      second host within the declared recovery time with zero lost
      committed events.
    - >-
      Log completeness: a guest killed mid-write has its last console lines
      in the remote log store; an alert reached the owner.
  rollback: >-
    Each slice is additive (replicas, sinks, mesh) until failover; the
    single-host deploy path stays working until the drills pass.
  landing: >-
    Standard Landing Loop per slice; drills recorded under docs/evidence/.

value:
  better_means: >-
    The computer the owner trusts is not one disk failure, one bad deploy
    or one OVH incident away from loss.
  goodharting_would_be: >-
    Replicas nobody has restored from; dashboards nobody reads; "zero
    downtime" measured only on stateless services while corpusd and guests
    still drop requests.

homotopy:
  realism_axis: >-
    single host with local logs (now) -> second host as off-host copies and
    log sink -> zero-downtime deploys -> failover -> multi-host guest
    placement -> cold audit archive.

boundaries:
  mutation_class: red
  authority_sources:
    - docs/operational-invariants-register-2026-10-08.md (O21, O22, O10)
    - docs/state-homes-inventory-2026-10-08.md
    - docs/computer-ontology.md
  must_preserve:
    - one writer per durable home; a replica is never a second writer
    - custodian key custody rules (two-approval reveal, audited delivery)
    - the tape is the audit authority; Dolt history is secondary
  excluded:
    - SaaS logging, metrics or hosting control planes
    - any work before Gates 1 and 2 pass human QA
  protected_surfaces:
    - deployment routing, vmctl, Store A, key escrow, gateway credentials

now:
  status: deferred
  slice: none
  decision: >-
    Deferred by the owner until Gates 1 and 2 pass human QA. Gate 1 runs on
    the single host with local logs. Re-open when the owner decides to
    invest; nothing below is approved spend.
  next_action: >-
    None until re-opened. When re-opened: refresh the observed facts, settle
    the open decisions, then author slices.

receipts: []
---

# SP — Production infrastructure (deferred)

## Why it is deferred

Gate 1 (stable computer) and Gate 2 (self-development with live Texture
supervision) are proved on one host. The owner wants the system proved
with human QA before spending more or adding infrastructure that is harder
to manage. SP is the first thing after that proof, ahead of scale-out.

What Gate 1 work must not foreclose (cheap now, expensive later):

- every durable home keeps exactly one writer and is reached through
  configuration (address, path), not a hard-coded host assumption;
- no new realization-local or host-local state outside the
  [state homes inventory](../state-homes-inventory-2026-10-08.md);
- logs stay structured (journald fields, guest console lines with computer
  and VM ids) so a later shipper needs no code change;
- the Store A rebuild keeps append receipts and escrow transparency intact
  (they are the future audit record).

## Topology

| Host | Role now | Role in SP |
| --- | --- | --- |
| Node B (OVH, 12c / 31 GiB / 2x512 GB NVMe RAID 1) | everything | primary: edge, control (corpusd, Store A, gateway, auth), guest host |
| Node C (near-identical, owner-held) | unused | off-host durable homes, log/metric/alert sink, backup target; standby for failover |
| Cold archive (rented OVH storage server, spinning disks) | none | Store A history remote, tape and escrow transparency archives (O22: "our audit log is gold") |
| More OVH bare metal | none | guest hosts when placement goes multi-host |

- **Mesh.** WireGuard between all hosts. Internal services bind to mesh
  addresses only; the only public listener is the edge proxy.
- **Roles from one flake.** NixOS roles `edge`, `control`, `guest-host`,
  `observability`, `archive`; a host is a list of roles. Deploy pushes the
  same closure everywhere and each host activates its roles.

## Non-functional requirements

| Area | Requirement | Candidate mechanism |
| --- | --- | --- |
| Durability (O22) | Every durable home has an off-host copy with a declared lag | Store A: Dolt replication to a standby `sql-server` on Node C (cluster/standby mode or per-commit remote push; verify against our Dolt version). Content store: content-addressed, so copy missing objects (rsync of new objects, or Garage as an S3 store — open decision). Escrow rows ride Store A; the custodian private key and the platform signing seed are backed up separately, encrypted, offline — losing the custodian key makes every escrow unreadable. |
| Recovery | Declared RPO and RTO per home; restore drill on a schedule | Restore drill (acceptance 1), run monthly once live. |
| Audit retention | Full history kept cold, bounded history kept hot | Scheduled push of Store A history to a Dolt remote on the cold archive; tape (heads, receipts, content) and escrow transparency archived there too. Hot Store A history squashed on a declared bound (O10). |
| Logs | Self-hosted, every host and every guest console, declared retention | Vector on each host (journald + guest console files) shipping over the mesh to VictoriaLogs on Node C (Loki as the alternative). |
| Metrics and alerts | Host, service and guest health; alerts reach the owner | VictoriaMetrics (or Prometheus) scraping node and service metrics; Alertmanager to email via maild or a self-hosted push channel. Alerts start with the operational invariant alarms the register lists as missing. |
| Zero-downtime deploys | No failed owner request during a deploy | Stateless services (proxy, auth, gateway): systemd socket activation or a second instance behind the proxy, drained before stop. vmctl restarts must not touch running VMs (verify). corpusd: fenced handoff — the new instance takes a higher-epoch lease and the old one drains — or a short write pause covered by client retry with Retry-After (open decision). Guests: S2 app-layer push, per computer. |
| Placement and moves | A computer can be realized on any guest host, or the owner's desktop | O21 construct-from-homes; single-writer fenced handoff between realizations, never merge. Same operation as the desktop hosted <-> local move. |
| Edge and DNS | Edge survives a host loss | Low-TTL DNS with a scripted switch to Node C first; floating IP (OVH additional IP) later. |
| Secrets | No secret in the repo or the Nix store in plain text | sops-nix or agenix, keys per host. |
| Capacity | Guest hosts admit by declared budgets | vmctl recovery admission extended with per-host capacity; disk and memory budgets per O10/O11. |
| Hardening | Minimal public surface | Only the edge port public; SSH over the mesh only; unattended security updates through the flake. |
| Time | Ordered timestamps across hosts | chrony on every host; the tape orders by sequence, not wall clock. |

## Open decisions (settle when re-opened)

1. **Control-store scale-out.** One primary plus a standby replica, or
   shard computers' event heads across control hosts. The first is enough
   until one control host saturates; sharding changes routing and is a
   larger design.
2. **Content store.** Replicated directory (simple, our code) or Garage
   (S3 API, multi-node, more to run).
3. **corpusd handoff.** Fenced lease handoff (no write pause, more code) or
   a short pause with retries (simple, a visible blip).
4. **Failover trigger.** Manual, scripted switch first; automatic only
   after drills show the fencing holds.
5. **Recovery targets.** RPO and RTO per home, set by the owner.

## Slices (sketch, in order)

- **SP0 — off-host copies and log sink (no new spend).** Node C joins the
  mesh; Store A standby, content-store copy, escrow-material backup, log
  and metric sink, alerts. Exit: restore drill and log completeness.
- **SP1 — zero-downtime deploys.** Exit: deploy under load.
- **SP2 — failover.** Exit: host-loss drill.
- **SP3 — cold audit archive (first new spend).** Exit: a past event head
  and its receipts read back from the archive.
- **SP4 — multi-host guest placement.** After S9 fork re-keying and the
  desktop move share the O21 handoff.

## Relation to other work

- O21 (station SH) makes a realization disposable; O22 (this station) makes
  the platform host disposable. SH is a prerequisite.
- Store A rebuild (problem doc
  `store-a-dead-wire-data-and-unbounded-history-2026-10-09`) shrinks what
  SP0 must replicate.
- The Wails desktop move uses the same placement operation.
