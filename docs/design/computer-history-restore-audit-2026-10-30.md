# Computer history, restore and audit (2026-10-30)

Status: design, revision 1. Not ratified; nothing is built or deployed by
this note. This note is green. The work it proposes is **red**: tape and
evidence, checkpoint and route projection, key custody, operator identity
and auth, and the guest boot path. Review tier 2 (Devin and DeepSeek v4.1
flash), because a defect here would silently disclose owner content,
silently lose history, or let an operator act go unrecorded. Labels:
**[V]** verified by a code read at 67500fc81 or by a cited measurement in
this repo; **[I]** inferred, settled by the probes and predictions in
section 11.

Owner direction this answers (2026-10-30, the last three entries of
[the selection definition](../definitions/choir-data-architecture-selection-2026-10-28.md)):
"We need to be able to restore arbitrarily (within something like a 7, 14,
or 30 day window for users) so anything bad that happens to a user vm can
rewind. I think this requires host side encrypted tape and/or
snapshots/checkpoints ... where are we and where do we need to get to to
make it secure, stable, scalable, and simple? In the context of
self-development with psa, and in light of enterprise nonfunctional
requirements for a fully auditable system for compliance." Earlier the same
day: user computers keep their whole tape by default; the paper is windowed
and offloaded; precommitment records and PSA lead.

Inputs, not re-derived here:
[host plaintext problem](../problems/host-holds-user-state-in-plaintext-2026-10-30.md),
[data at rest problem](../problems/data-at-rest-snapshots-images-escrow-key-plaintext-2026-10-21.md),
[object storage, revision 12](object-storage-2026-10-30.md),
[secrets vault, revision 2](secrets-vault-design-2026-10-28.md),
[data rearchitecture](../archive/design/data-rearchitecture-2026-10-28.md),
[apply checkpoint without replay](../archive/design/apply-checkpoint-without-replay-2026-10-26.md),
[precommitment records](../precommitment-records.md) section 6,
[tape window sizing](../evidence/tape-window-sizing-2026-10-30.md),
[new engine storage](../evidence/new-engine-storage-measurement-2026-10-30.md),
[keep-set accounting](../evidence/keep-set-accounting-2026-10-29.md),
[computer ontology](../computer-ontology.md).

## Decisions in one place

1. **One source of truth per computer: its event chain** (envelopes,
   payloads, receipts). Snapshots are accelerators checked against the
   chain. The guest store, indexes, Texture views and any analytical copy
   are derived.
2. **The guest seals every payload before it leaves the guest**, except a
   closed control list of two content-free kinds. The host holds
   ciphertext, plus envelopes and receipts that carry digests, kinds,
   actors and times but no content. The plaintext pin path and the
   `owner`→`public` replay mapping are deleted.
3. **The guest makes snapshots; the host makes none.** checkpointd's
   rebuilder and verdict pass, projection bases, `PublishCapturedBase` and
   the routine host unwrap are deleted.
4. **The computer DEK never rests in plaintext on a host disk.** The guest
   data volume is encrypted under a key derived from it, and the DEK is
   delivered at every boot through the existing realization key delivery,
   as an audited unwrap.
5. **A restore is forward events**: `restore_requested` (exists),
   `restore_applied` (new, an epoch root: state := reconstruct(T) plus
   carried records), `restore_failed` (new). This also fixes the current
   restore, which does not persist (gap G1).
6. **Windows bound restore points, not history.** User computers keep
   their whole sealed tape; the tenant window (7, 14 or 30 days) is where
   any head can be restored. The paper keeps a 7-day hot window; older
   sealed segments move to an archive class; its records' witnesses are
   kept forever.
7. **One platform audit chain**, signed, hash-chained and append-only,
   records every operator, owner and service act on keys, restores,
   deletions, holds, retention and configuration. It replaces the escrow
   transparency log, owner-act receipts and the deletion logs.
8. **Hourly anchors**: a Merkle root over each computer's latest head,
   every record-carrying head and the audit head, timestamped by two RFC
   3161 authorities and committed to a public git repository.
9. **Legal hold, per-tenant policy, export with an offline verifier, and
   crypto-erase** are defined over those same objects; there is no second
   store.
10. **One cutover**: all of this lands inside object storage revision 12's
    single writer drain. Pre-cutover plaintext is retired under Q1 after
    the paper's record witnesses are re-sealed and the sealed-restore
    drills pass.

## 1. Where we are

### 1.1 The mechanisms today

| Mechanism | Where | Encrypted | Retention | Note |
|---|---|---|---|---|
| Event envelope | `computerevent/event.go:78-109`; digest is sha256 of canonical JSON (`:131-138`); each event carries `previous_head` (`:83`), so the head is a hash chain [V] | no; pinned with the label `private` as canonical JSON (`platform/event_artifacts.go:33-47`) [V] | forever; the tape is outside artifact GC (`artifact_gc.go:106-110`, cited in object storage §1) [V] | content-free metadata: kinds, actors, times, digests |
| Sole sequencer | `ComputerEventAppender` (`computerevent/appender.go:115-119`) [V] | n/a | n/a | good substrate; keep |
| Non-private payload | `AppendNewPayload` (`appender.go:426-492`) pins as given and deletes the live cache after finalize (`:491`) [V] | **no** | forever | projection batches (`store/projection_tape.go:114-153`, class `owner`), platform update offers (`agentcore/platform_update.go:203`), materialization results (`self_development_materializer.go:1262`), file root manifests (`file_sync.go:722`), lifecycle receipts (`lifecycle_receipts.go:49`, `public`), verification decisions (`tools_capsule.go:717`, `public`), capsule effect bundles (`tools_capsule.go:860`, `public`) [V] |
| Private payload | `AppendNewPrivatePayload` (`appender.go:498-541`), `AppendNewPayloadSet` (`:566+`); envelope in `privacy.go:247-282` (canonical JSON, base64 ciphertext, AAD over computer, event, media type, length, key digest, nonce) [V] | yes, XChaCha20-Poly1305 under the computer DEK | forever | used only by Texture audit (`texture_audit.go:91`) [V] |
| Replay class mapping | `resolveProjectionBatch` maps `owner` or empty to `public` (`appender.go:1116-1118`) [V] | n/a | n/a | delete |
| Host segments | `PLATFORM_TAPE_SEGMENTS=write` (`nix/node-b.nix:1321`); only projection batches are segmented, everything else is legacy one-file pins (`event_artifacts.go:44-47`, `:63-65`, `:76-129`) [V] | no | forever | `tapestore/format.go:1-17` still says no production path writes segments (stale) [V] |
| Head CAS and receipts | `platform/computer_events.go:194-306`: serializable CAS, `Reduce`, `EventHeadReceipt` signed with host time under one platform key; lifecycle receipts joined for `lifecycle_observed` and `key_revoked` (`:288-299`) [V] | no (content-free) | forever, unbounded: ~0.77 GB/day raw for the paper (tape window sizing) [V] | no cross-computer chain, no anchor [V] |
| Guest event index | `store/computer_events.go:64` (event JSON), `:200` (receipt JSON); 4.4 KB/row (tape window sizing) [V] | no | whole life | holds envelopes and receipts, **no payloads** |
| Computer DEK | file JSON mode 0400 on the data image, created only before genesis (`privacy.go:98-132`); single-version keyring (`:229-241`); escrow required before genesis, best effort after (`autoputer/run.go:398-428`) [V] | plaintext file on a plaintext image | life of the computer | a copy of `data.img` carries the key beside the data |
| Key delivery | `HandleRealizationKeyDelivery` (`platform/key_delivery.go:45-144`): bound to the current issuance, requires an existing head, writes a transparency entry before `OpenDEK`, seals to the realization's X25519 key; used at boot when the key file is missing (`run.go:335-372`, O21) [V] | sealed in transit | per boot | a host-to-guest key path exists |
| Escrow unwrap | two-approval requests (`key_escrow.go:167-240`); hash-chained transparency log, `sha256(prev_hash ‖ payload)`, unsigned, in Store A (`:295-335`); head readable only internally (`key_escrow_http.go:277-296`); reveal returns the raw DEK base64 to the operator (`:221-275`); operators are shared tokens in an env file (`:298-336`, `nix/node-b.nix:1233-1236`) [V] | n/a | forever | not anchored, not signed, no owner notification |
| Routine host unwrap | checkpointd opens the escrow for every base rebuild (`cmd/checkpointd/worker.go:181-205`), logs `projection_checkpoint_key_use` (`platform/projection_jobs.go:590-599`) [V] | n/a | n/a | the host unwraps every computer's DEK as routine |
| Projection bases | cadence `checkpointDue` 2,500 events, or 1,000 and 24 h (`projection_jobs.go:122-128`); retention: watermark, pins, latest two, job seeds (`artifact_gc.go:402-404`); `PublishCapturedBase` reflinks the guest's `data.img` (`projectionbase/capture.go:18-30`); rebuilder uses the DEK (`rebuilder.go:381-395`) [V] | **no**; 61.6 GB (object storage §1) | latest two | host plaintext |
| ComputerVersion checkpoints | cadence compiled off, host-selected; on only for `computer-59c902c5…` (d@e) and `class:test` (`nix/node-b.nix:1785`) [V]; artifact JSON at `computer-checkpoints/<digest>.json` (`platform/checkpoints.go:182`) [V]; retention slice not built (apply-checkpoint F10) | no | unbounded | most computers have no restore point |
| Restore | `restoreComputer` (`agentcore/rematerialize.go:347-394`): refuse `not_derivable`, append `restore_requested` (`:410-442`, class `owner`, authority `platform-control:restore`), then `RematerializeFromTape` (`:64-321`): reconstruct to the target in a staged store, witness check, restage the pinned release, flip, reopen, `RebindProjection`; a changed exec takes a marker-backed planned restart (`self_development_restore_restart.go:128-170`) [V] | n/a | n/a | no `restore_applied` kind exists (`event.go:35-62`) [V]; see G1 |
| Cold recover | `vmctl/cold_recover.go`: `recover_current` only (`:23`, `:414`) [V] | n/a | two quarantines per VM | no older target |
| Guest volume, hibernate RAM | plaintext ext4 `data.img` (`vmmanager/manager.go:2997,3020`); Full memory snapshots kept up to 36 h (`VM_HIBERNATE_MODE=snapshot`, `nix/node-b.nix:1812`) [V, 10-21 problem record] | **no** | 36 h for RAM | the DEK is in both |
| Owner acts | `platform/owner_acts.go`, `owner_act_receipts`; `ownerActMode = "observe"` (`nix/node-b.nix:162`) [V] | n/a | forever | not enforced in production |
| Off-host backup | one restic secret opens keys and data (`scripts/off-host-backup.sh:133-153`, 10-21 record) [V] | restic | restic policy | carries all of the above plaintext |
| Host plaintext readers | `ledgerwitness/walk.go:119-144` reads projection batch payloads to find records; the checkpoint authority reads the verification decision payload (`platform/checkpoints.go:281-292`); `event_handlers.go:279` serves payloads back to guests for replay [V] | n/a | n/a | the first must move into the guest; the second is content-free; the third is a pass-through |
| Anchoring, legal hold, tenant retention, export | none: no RFC 3161, transparency-log or anchor code; no legal-hold or tenant retention code; no export path (grep at 67500fc81) [V] | n/a | n/a | |

### 1.2 Gaps beyond the known plaintext problem

- **G1. A restore today is neither a forward state change nor durable.**
  `restoreComputer` appends `restore_requested` at N+1, then rebuilds the
  store to the target head T and rebinds it (`rematerialize.go:383-390`,
  `appender.go:383-391`) [V]. The embedded head is then T while the
  canonical head is N+1, and `appendLocked` refuses every append with
  `ErrNeedsProjectionRepair` when the two differ (`appender.go:733-743`)
  [V]. At the next boot, recovery targets the canonical head
  (`autoputer/projection_base.go:115-140`) and the plan resumes or
  rebases toward it (`recoveryplan/recovery_plan.go:81`) [V]; the restore
  path has no restore awareness at boot (grep) [V]. Inference [I]: the next
  boot replays (T, N+1] and undoes the restore, including the planned
  restart a changed exec asks for. The test asserts only the instant after
  restore (`rematerialize_test.go:447-528`) [V]. Probe: on d@e, restore,
  make one owner edit, restart, read the projection head (R0k-01). Per the
  operating contract this needs its own problem record before any fix.
- **G2. The guest holds no payloads.** Its "whole tape" is envelopes and
  receipts (`appender.go:491`, `run.go:430-436`) [V]. Replay and history
  depend on the host copy.
- **G3. "The host holds only ciphertext" is false even for private
  payloads**, because the DEK sits in plaintext on the host-held
  `data.img`, and in hibernate RAM files [V, 10-21 record].
- **G4. The host routinely unwraps every computer's DEK** for base rebuilds
  (`worker.go:181-205`) [V].
- **G5. Four host plaintext dependencies would break under sealing**:
  ledger witness, checkpointd's rebuilder, base capture, and (content-free,
  kept) the checkpoint authority's decision read [V].
- **G6. Audit is fragmented and weak.** At least six logs (escrow
  transparency, owner-act receipts, mode receipts, lifecycle receipts, GC
  deletions, tape sweeps); the one hash chain is unsigned, unanchored,
  internal-only and rewritable by anyone with Store A write; the reveal
  hands a raw DEK to a human; operators are shared tokens; no owner is
  notified [V].
- **G7. Owner-act enforcement is off in production** (`observe`) [V].
- **G8. No legal hold, tenant retention or export; receipts grow without
  bound; the platform signing key has no rotation caller** (precommitment
  records §6) [V].
- **G9. Restore points mostly do not exist**: checkpoint cadence is on for
  one computer and the test class; bases keep the latest two [V].
- **G10. Cold recover reaches only the current head** [V].
- **G11. Mislabels**: event pins are labelled `private` but stored as
  plaintext (`event_artifacts.go:46`); capsule effect bundles are pinned
  `public` and carry the owner's workspace file paths, modes and refs
  (`capsule/transaction/builder.go:26-66`) [V]; no host reader of that
  payload was found (grep for the media type) [V].
- **G12. Stale statements**: the tapestore package comment; object storage
  §3 "No such channel exists" (key delivery exists, `run.go:335-372`); object
  storage §1 and §7 step 6 "the paper leaves the tape", superseded by the
  2026-10-30 direction [V].

## 2. The model

**Source of truth.** Each computer's event chain: the sequence of
envelopes (hash-linked by `previous_head`), the payloads they commit to by
digest, and the host's signed head receipts. Nothing else is
authoritative. The guest store is a projection of the chain; a snapshot is
a projection at one head; the audit chain is the platform's own chain.

**What the host holds per computer** (all in blobd, object storage's one
store, plus receipt rows in the control database):

- envelopes and receipts in plaintext. They are control records: the
  CAS reduces them, anchors hash them, and a third party verifies them
  without keys. They carry kinds, actor profiles, times and digests, which
  is metadata, not content (named residual, section 13);
- sealed payloads, sealed by the guest (section 3);
- sealed snapshots made by the guest, with a small plaintext manifest
  header (head, sequence, witness digest, code identity, chunk refs,
  closure refs);
- nothing else for history. No base, no plaintext image capture, no
  rebuilt store.

**What the computer holds.** Its working SQLite store (current state, the
event index of envelope digests and decisions), its file tree, its
release, and its DEK in memory only. It reads its own history through
blobd's internal scope, which only it can read (object storage §3). I read
the owner's "the full tape stays on the computer" as: the tape is in the
computer's own scope, never windowed and never offloaded to a platform
analytical store. A second payload copy on the guest disk would be a
second authority, and the guest disk is itself a file on the same host, so
it adds no independence. That reading is open question 1.

**Restore point** = a head T of this computer whose receipt time is
inside the tenant window and for which a snapshot at or before T exists
with tape from it to T. Every head inside the window is a restore point
by construction (section 4).

**Restore address** = (computer ID, sequence, canonical head, content
witness, code identity). The content witness is the per-table hash set of the store at T (today
the Dolt-hash v2 witness, `selfdevprotocol/witness_v2.go:63`; the engine
design redefines it over SQLite). Code identity is the
ComputerVersion in force at T: CodeRef, ArtifactProgramRef, release
digest and frontend identity (C15/I25 put the frontend in the restore
set). An owner picks a head or a time; a time maps to the last head whose
receipt host time is at or before it. The witness and code identity are
filled in by reconstruction and recorded in `restore_applied`; if the
request carried a witness (from a snapshot manifest), a mismatch fails the
restore.

**Restore is forward.**

1. `restore_requested` (existing kind) at N+1 opens a pending transition
   naming the target address. The reducer refuses it while another
   transition is pending (a self-development apply or rollback); the guest
   does not request it while a capsule is live or an apply is
   mid-transaction (runtime state the reducer does not see).
2. `restore_applied` (new) at R closes it. It is an **epoch root**: its
   defined effect is state := reconstruct(T) ⊕ carried, where carried is
   the ordered ops on carried tables in batches (T, N] (section 6). Its
   payload records the target address, the carried-batch digest and the
   resulting witness. Replay across R is deterministic from the tape
   alone; the post-restore snapshot at R only saves time.
3. `restore_failed` (new) closes a pending restore with a reason:
   refused, a snapshot or replay failure, or "interrupted by a restart".

Nothing is deleted. Undo is another restore, to N.

**Restarts.** A crash restart never resumes work. At boot, a pending
restore with no committed `restore_applied` is closed by
`restore_failed` "interrupted by a restart". A committed `restore_applied`
whose flip did not finish is not resumed work: the canonical head is
already R, and boot recovery replays R like any other committed event
(through `RecoverPrepared`, `appender.go:798-851`). A restore that changes
the exec writes the existing planned-restart marker after
`restore_applied` commits (`self_development_restore_restart.go:128-170`);
the next boot consumes it and finds local head = canonical head = R, so
nothing replays. A planned refresh waits while a restore is pending, the
way it waits for a self-development apply. A pending restore is closed at
any boot, planned or not: restores take seconds and the owner can retry,
so resuming one buys nothing.

**Deploy identity.** A restore can move a computer to an older release
than the platform's current one. That is the divergence the operating
contract names ("product restore vs platform deploy"), not a failed
deploy. Health and identity reporting join event head, CodeRef, witness
and route.

## 3. Encryption and keys

### 3.1 Which key seals which byte

All history keys derive from the computer DEK with HKDF-SHA-256, one
`info` string per purpose and keyring version, never reused (the vault's
rule, vault §2 decision 3).

| Host-held bytes | Sealed by | Key | Readable by the host |
|---|---|---|---|
| Envelopes, head and pin receipts | not sealed | none | yes; content-free |
| Control payloads: lifecycle receipts and verification decisions only (closed list; schemas limited to IDs, digests and verdicts, as `checkpoints.go:288-292` reads) | not sealed | none | yes |
| Every other payload | guest | per-object key = HKDF(`choir/history/v1/payload-kek`ᵥ, salt = object nonce) | only after an audited unwrap |
| Snapshot chunks | guest | per-chunk key from `choir/history/v1/snapshot-kek`ᵥ; chunk ID = HMAC(`choir/history/v1/id`, plaintext), so unchanged chunks dedup through blobd `IfAbsent` | only after an audited unwrap |
| Snapshot manifest | guest | sealed body; plaintext header (head, sequence, witness digest, code identity, chunk and closure refs as `hmac:` IDs) | header only |
| Guest data volume | guest dm-crypt (LUKS2) | volume key = HKDF(`choir/history/v1/volume`) at the volume's creation version | only after an audited unwrap |
| Hibernate RAM | host | the snapshot vault: a per-host-boot random key never written down (10-21 record) | live root only |
| Computer DEK | n/a | protectors: custodian escrow wrap (exists), owner-KEK wrap (vault slice 5, later); never in plaintext on disk | through key delivery, audited |
| Escrow private key | host | sealed under the host root (vault §2 decision 5) | live root |
| Audit chain, anchors | not sealed | signed | yes; content-free |

**Envelope v2 for sealed payloads** replaces the canonical-JSON envelope
of `privacy.go:247-282`: binary, compress (zstd) then seal; header =
version, computer ID, media type, plaintext length, key version, nonce,
and a **key commitment** = SHA-256(`choir/history/v1/key-commit` ‖
per-object key). The event commits to the digest of the whole sealed
object, as private pins do today. The key commitment matters for PSA: a
disclosed per-object key must not be swappable for another key that opens
the same ciphertext to a different record (XChaCha20-Poly1305 is not
key-committing). Per-object keys allow **selective disclosure**: one batch
can be opened for a reviewer without opening anything else.

**Deleted:** `AppendNewPayload`'s plaintext path for anything outside the
control list, the `owner` and `public` payload classes as storage classes,
the `owner`→`public` replay mapping, and the `private` label on plaintext
event pins (they become `control`).

### 3.2 Unwrap, delivery and the broken VM

The guest DEK leaves the data image. Every boot of an existing computer
gets it by realization key delivery, which already binds to the current
issuance and seals to the realization's key (`key_delivery.go:45-144`),
and is already used when the key file is missing (`run.go:335-372`) [V].
Each delivery is an audit entry (purpose `realization_boot`). Genesis is
unchanged: the guest generates the DEK and escrows it before the first
event (`run.go:398-428`) [V]. The guest opens its volume with the derived
key and keeps the DEK in memory only. Build risk [I]: the credential
bootstrap must not depend on the encrypted volume (today's handoff files
live on it, `run.go:280-313`).

A broken VM restores the same way: vmctl boots a fresh realization on a
new empty volume with a signed restore directive; delivery gives it the
DEK (purpose `restore`); it builds the restored store (section 7).

**Escrow unwrap purposes are a closed list**: `realization_boot`,
`restore`, `export`, `drill`. Each is an audit entry with actor, subject,
reason and the request digest. Automated purposes need no human approval;
they are bound to an issuance the host minted for that computer. An
operator-initiated `restore` or `export` needs two named approvals and
notifies the owner. The raw reveal (`key_escrow_http.go:221-275`) is
deleted: no path returns a DEK to a person. Legal compulsion goes through
`export`, wrapped to a named recipient key. checkpointd's routine unwrap
is deleted with the rebuilder.

**Sovereign owners** (vault §2, not built) decline the custodian wrap. Their
computers boot and restore only when the owner's client delivers the DEK,
so an operator cannot restore them. That is the definition, stated as a
product fact.

**Honest residual.** Live root on Node B sees guest memory, and corpusd
sees a DEK while it seals a delivery. Without confidential-computing
hardware no software design removes that. What this design removes is
plaintext at rest: on disks, in backups and in copies.

### 3.3 Rotation and crypto-erase

- **Computer DEK rotation** uses the vault's keyring digest map (vault
  §2 decision 4; today `resolve` returns only the current key,
  `privacy.go:229-241`). New objects use the new version; old ones stay
  readable through the map. History is immutable and committed by
  ciphertext digest, so it is **never re-sealed**; after a suspected
  compromise of an old version the choices are to accept it or erase
  that version. This is the conservative option: re-sealing would mean a
  second, mapped history.
- **Platform signing key**: versioned in a published key history, with an
  overlap period; receipts and audit entries name the key ID. Rotation
  gets a caller and a drill (G8).
- **Crypto-erase of a computer** (owner act, or tenant deletion): refused
  under legal hold; otherwise stop the computer, destroy every DEK
  protector (custodian wrap, owner-KEK wrap, vault slice), purge its
  internal scope in blobd (packs are per scope, object storage §3), and
  record the act in the deletion journal so restic copies are erased when
  that journal runs (vault §2 decision 8; never claim backup erase before
  that). Envelopes are deleted on full erase; receipts, anchors and audit
  entries stay, being digests and IDs.

### 3.4 With the vault and object storage

The computer DEK is the vault's tier-4 key. Its owner-KEK protector (vault
slice 5) makes history portable to another deployment through the vault
slice. It is not a cutover prerequisite: the vault says not to make
itself a hard restore dependency before it has an HA story (vault §4), so
custodian escrow stays the boot path. Owner-scope objects (images, files)
stay under object storage's owner content key. History objects are
internal-scope objects under the computer DEK, as object storage §3
already says "Internal objects use the computer DEK"; this design fixes
the derivations.

**The plaintext problem, resolved.** After the cut, nothing a guest writes
reaches the host in plaintext except envelopes, receipts and two
content-free control kinds. Snapshots and volumes are sealed. The DEK is
not on any disk. The pre-cutover plaintext (segments, legacy pins, bases,
images, restic) is retired under section 9.

## 4. Retention

**User computers (default).**

- Tape: whole, forever, sealed, in the computer's internal scope.
  Receipts: forever.
- Window (per tenant: 7, 14 or 30 days; default 30): every head whose
  receipt time is inside it can be restored.
- Snapshot cadence: when the head has advanced and either 10,000 events
  have passed (today's `MaxRecoveryTailEvents`, `recoveryplan`; apply-checkpoint §4.1) or 7
  days have; never more often than hourly.
- Snapshots kept: every one younger than 48 h; the last of each day inside
  the window; the newest at or before the window start; and pins (the
  pre-restore and post-restore snapshots of any restore inside the window,
  self-development rollback targets, legal holds). An inactive computer
  therefore keeps one snapshot.
- Bounded replay: at most 10,000 events inside the last 48 h; at most one
  day of events (or 10,000, whichever is larger) inside the window. At the
  measured rates (active median 62 events/day, the owner's computer about
  282/day, tape window sizing) almost every restore replays under 2,000.
- Older than the window: the tape stays readable and verifiable; replay is
  not guaranteed (the snapshot it needs may be gone).
- Optional tenant maximum history retention (unset by default): deleting
  a tape prefix is an audited deletion, preceded by a snapshot at the new
  first head; later chain links stay verifiable against anchors.
- Disk fill on the guest is the guest-storage design's (an owner-visible
  warning and an owner act, never silent truncation). The tape no longer
  counts toward the guest disk.

**The paper.**

- Hot tape window 7 days (14 once the slimmed layout lands). Older sealed
  segments move to an archive class in blobd, unchanged (they are already
  sealed), and stay until the analytical database loads them. Their
  retention after that is the analytical design's question.
- Snapshot cadence daily (T = 24 h, no event trigger), kept for the
  window plus the one before it. Daily rather than 6-hourly because
  resource use leads short-term uptime (owner, 2026-10-22) and paper
  restores are rare; the cost is a longer replay (R0k-08).
- Receipt rows in the control database: windowed with the hot tape. The
  sealed segment carries each event's pin receipts and its head receipt,
  so the archive keeps them.
- **Record-witness set, kept forever**, independent of the archive: every
  event whose batch writes a commitment record, resolution or score, with
  its sealed payload, pin receipts, head receipt and anchor inclusion
  proof. The guest tags such batches at write time (authority
  `authority:vm-local-projection-record`, beside the existing
  `-projection` and `-telemetry` values, `projection_tape.go:114-153`).
  Size: about 10,800 records/day at about 3.2 KB plus receipts (tape window
  sizing), roughly 35 MB/day before receipts.

**Kept forever for every computer:** head receipts of record-carrying
events, the audit chain, anchors and their leaves, the key history.

**Legal hold** (operator act with two approvals, or owner act): a scope
(tenant, computer, or a head range). While held, blobd refuses to purge
objects in the scope, retention GC skips it, crypto-erase and tenant
deletion are refused, and no paper archive segment it covers is pruned. A hold does not block a restore, because a restore
deletes nothing. Set and release are audit entries; release needs two
approvals.

**Deletion with audit.** Every deletion run (retention GC, tape sweep,
erase) writes one audit entry naming the run, its policy, counts and the
digest of a deletion manifest stored in the platform internal scope. The
tape sweep's archive-before-delete (`tape_sweep.go:28-41`) and tombstones
(`tape_tombstone.go`) stay [V].

## 5. Audit

**Two kinds of chain, anchored together.** Each computer's tape is its own
chain (hash-linked envelopes, signed head receipts). The **platform audit
chain** records acts on computers and tenants from outside them. Operator
acts are not mirrored into computers' tapes: anchors make the audit chain
tamper-evident, and owners read their entries through an API with
inclusion proofs.

**Audit entry**: `seq`, `prev_hash`, `host_time`; `actor {kind:
owner|operator|service|computer, id, auth_method, credential_ref,
step_up}`; `act`; `subject {tenant, owner, computer, refs}`; `reason`;
`approvals[]` (each an entry ref); `policy_ref`; `request_digest`;
`result`; `key_id`; `sig`. Acts, a closed list: `key_unwrap`,
`key_delivery`, `restore_requested`, `restore_applied`, `restore_failed`,
`export`, `legal_hold_set`, `legal_hold_released`, `retention_changed`,
`deletion`, `crypto_erase`, `override`, `config_change`, `deploy`,
`operator_session`, `anchor`, `access_digest`, `secret_*` (the vault's
acts). Entries carry digests and IDs, never content or secret names.

**Storage.** One append-only table in the control database: the writing
role has no UPDATE or DELETE. Each entry is signed with the platform
signing key under its own domain tag, so the platform has one signing key
to custody and rotate. Hourly segments are also written to the platform
internal scope of blobd and go to the off-host backup.

**Replaced, not kept beside it.** The escrow transparency log is frozen;
the audit genesis entry commits to its head (`key_escrow.go:337-349`) and
row count. New owner-act receipts, mode receipts, GC deletion logs and
tape sweep logs become audit entries; their old tables are frozen with
their heads in the genesis entry. The vault's `AuditSink` (vault §5) is
this chain, not a second hash chain. checkpointd's key-use entries are
deleted with checkpointd. Lifecycle receipts stay: they are computer
facts that the CAS joins (`computer_events.go:288-299`), not operator
acts.

**Operator identity.** Named principals in the control database (ID, role,
public keys), each with an individual credential (passkey or hardware key,
step-up for approvals), acting through an operator API or CLI. The shared
token file is deleted. SSH is break-glass: a PAM hook writes
`operator_session` start and end entries; full session recording is a
follow-up. Owner acts need a fresh passkey assertion bound to the act
(vault §3); owner-act mode moves to `enforce` at the cut (open question
12).

**Anchoring.** Every hour, a Merkle tree (RFC 6962 hashing) over sorted
leaves:

- for each computer whose head moved this hour: (computer ID, sequence,
  head, receipt digest) of its latest head;
- every record-carrying head of the hour, so a record has a direct
  inclusion proof with no chain walk;
- the audit chain head;
- once a day, the previous day's access-log digest.

The root is timestamped by two independent RFC 3161 authorities and
committed with both tokens to a public git repository. The anchor is
itself an audit entry; leaves are stored for inclusion proofs. This is
the conservative standard choice. RFC 3161 is widely accepted by auditors,
two authorities cover one's failure or compromise, and public git is a
witness no single authority controls. Hourly bounds backdating to an hour
for 48 requests a day. A missing anchor is retried; the next one covers the
gap; a gap over 6 h pages. The first anchor is at the cut and covers every
H0 (section 9).

**Access logging.** Every key use is an audit entry, and with sealing a key
use is the only path to plaintext. blobd logs every read by caller,
scope, ref and bytes into a daily file in the platform internal scope;
its digest is anchored daily. Owners can read both for their computers.

**Export** (owner act, or operator act with two approvals): a directory
or tar with

- `manifest.json`: computer, range, counts, digests, the export's audit
  entry;
- `events/`: envelopes;
- `receipts/`: head and pin receipts;
- `payloads/`: sealed objects; and `keys/`: the keyring wrapped to a
  recipient key the requester supplies. Or plaintext payloads, if the
  owner chooses;
- `anchors/`: anchor entries, leaves, inclusion proofs, authority tokens,
  git commit IDs;
- `audit/`: entries naming this computer, with the chain links needed to
  verify them;
- `keyhistory/`: platform public keys and validity.

`choir verify-export` checks it offline.

**Reviewer verification, end to end.** For a record:

1. recompute `commit_hash` from the disclosed row (precommitment records
   §6 step 4);
2. check the disclosed per-object key against the header's key commitment;
   decrypt; find the row in the batch;
3. check the batch object's digest against the event's payload
   commitment;
4. hash the envelope and check it equals the head in the receipt;
5. check the receipt signature against the key history for its key ID;
6. check the receipt's head is a leaf under an anchored root; check both
   timestamp tokens and the git commit.

The record existed no later than the anchor and no later than the
receipt's host time, backdating is bounded by an hour, and the host could
not have changed it without breaking the anchor. For a whole history, the
reviewer walks the envelope chain from the first exported head and checks
the latest head against the latest anchor.

| Enterprise requirement | Mechanism |
|---|---|
| Tamper-evident append-only history | envelope hash chain plus signed receipts per computer; signed, hash-chained audit chain; append-only role |
| External anchoring of chain heads | hourly Merkle root, two RFC 3161 authorities, public git |
| Every operator act audited | closed act list; named operators; two approvals for unwrap-to-human, restore, export, hold release; SSH sessions logged |
| Key unwrap audited and approved | closed purpose list; automated purposes bound to issuance; human purposes approved and owner-notified; raw reveal deleted |
| Per-tenant retention | tenant policy: window 7/14/30, optional maximum history, sovereign flag; changes audited |
| Legal hold over GC | scope hold refusing purge, GC, erase and deletion |
| Export | self-verifying bundle plus `choir verify-export` |
| Crypto-erase | protector destruction, scope purge, deletion journal for backups; honest levels |
| Access logs | key uses in the audit chain; blobd reads in an anchored daily log |
| Restore without history loss | forward restore events; nothing deleted |

## 6. Self-development and PSA

- **Capsules and accept/reject** are already events with host receipts:
  verification, effect proposals, apply, rollback (`event.go:35-62`) [V].
  Verification decisions stay in the control class because the host's
  checkpoint authority reads them to accept a checkpoint
  (`checkpoints.go:260-292`) [V]. Capsule effect bundles become sealed:
  they carry workspace paths and refs and no host reader of the tape
  payload was found [V]. The host builder's inputs reach it outside the
  tape [I], to be confirmed before the seal lands.
- **Restore and self-development.** A restore is refused while an apply or
  rollback is pending, during a live capsule, or mid-apply (the same
  guards as a busy refresh, `self_development_restore_restart.go:115-126`).
  `rollback_*` stays the release-scope undo for self-development; folding
  it into restore with scope `release` is a follow-up (open question 11).
- **Records, resolutions and scores are carried across a restore.**
  `restore_applied` re-applies, in order, every op on the record tables
  in batches (T, N]. Without this, a restore would hide losing
  predictions from the live ledger (the "hide" attack, precommitment
  records §6). The tape would still hold them, but the owner-facing
  ledger would not. Record tables are append-only, so replaying their ops
  onto state(T) is well defined. A record whose subject the restore
  removed shows its subject as absent. This changes the restore set
  (records join external sends as outside the rewind) and needs an owner
  ruling (open question 3).
- **Trusted time** is the first head receipt that carried the record;
  the guest's `committed_at` is advisory (precommitment records §6 step
  1). The anchor bounds backdating to an hour, by Choir too (step 2).
- **Kept beyond any window:** for users, everything; for the paper, the
  record-witness set (section 4).
- **Publication** of a record discloses one per-object key: the batch
  plaintext, the envelope, the receipts and the anchor proof. A batch
  opened this way shows every row in it, so record commits should be their
  own store transaction [I: to check in the record writer].
- **Ledger witness** (`ledgerwitness/walk.go`, a host plaintext reader)
  moves into the guest and into `choir verify-export`.

## 7. Restore operations

**Owner rewind of a running computer.**

1. The owner picks a time or a head in Texture's history view. Admission:
   inside the window; no pending transition, live capsule or mid-apply;
   free space for staging. The owner act gets an audit entry with step-up.
2. The guest appends `restore_requested` through a narrow appender path
   that checks only the platform head (the reducer needs no store state for
   it). Today's `appendLocked` embedded-head check would otherwise be
   inapplicable.
3. In a staging directory: fetch the newest kept snapshot at or before
   T, verify its chunks, replay the tape to T, compute the witness.
   Snapshot the live store at N+1 as the pre-restore pin.
4. Apply the carried ops. Append `restore_applied` with the staged store
   as the projection being finalized (the same narrow path: the epoch
   root sets the staged store's head to R).
5. Flip, reopen and rebind (today's quarantine-and-flip,
   `rematerialize.go:282`). Restage the pinned release if the code identity
   differs; if the exec differs, take the marker-backed planned restart.
   Snapshot at R (pinned).

**Operator restore** (owner absent, support or incident): same path,
started through the operator API; two approvals; the owner is notified
before the flip. Refused for sovereign owners unless their client delivers
the key.

**Broken VM onto a new realization.** vmctl's `recover_current`
(`cold_recover.go`) generalizes to `restore_to` with a target address. It
quarantines the old volume (existing), creates a new encrypted one, and
boots a realization with a signed restore directive. Delivery gives the
DEK. The guest runs steps 2-5 with no live store: the carried ops come
from the tape, not from the old store. The old volume stays in quarantine
under its existing retention.

**Partial restore** is not built. Documents already revert through Texture
revision history (doctrine I1); other kinds take a forward write through
their owning verb. A table-scoped restore would need cross-table
consistency rules that nobody has asked for.

**What the user sees.** A history entry: "Restored to <time> (event n) by
<actor>. Later changes are kept in history. Undo restore." Undo is a
restore to N. While a restore runs, the computer shows a restoring state.
On failure it says which step failed and that nothing changed.

**Failures.**

- Snapshot chunk missing or bad: fall back to an older kept snapshot and
  replay longer. With none, `restore_failed` "restore point unavailable".
- Witness mismatch against a requested witness, or a replay error:
  `restore_failed`; a problem record; the drill set is flagged.
- Delivery refused or the escrow wrap missing: `restore_failed`, or a wait
  for the owner client (sovereign).
- Crash before `restore_applied`: closed at boot. Crash after it: boot
  replays R.

**Idempotency.** The owner act ID is the request's idempotency key:
`restore_requested` idempotency = digest(act ID); `restore_applied` =
digest(`restore-applied` ‖ request event ID). A resent request returns the
first result.

**Drills.**

- Weekly, in each active guest, admitted against free space: restore
  snapshot k-1, replay to snapshot k's head, compare with k's witness.
  This checks snapshots and replay determinism. The outcome is a
  management record (prediction: equal) and its resolution, so drill
  history is itself a track record.
- Monthly on d@e: a broken-VM restore in a `drill` realization (read-only,
  no appends, delivery purpose `drill`).
- Quarterly on the paper, the same.
- Each drill is an audit entry. A failed drill pages.

## 8. Scalability and cost

From the measurements cited (tape window sizing; new engine storage;
apply-checkpoint):

- **User tape** [I]. Payloads average 4,657 B and compress about 5x
  before sealing. Envelope plus pin receipts are about 3.3 KB raw and
  compress in the segment. Estimate 2.0 KB/event in segments plus 3.1 KB of
  receipt row in PostgreSQL. Median active computer (62/day): about 4 MB of
  segments and 6 MB of receipts per 30 days. The owner's computer
  (282/day): about 17 MB and 26 MB. Whole non-paper fleet (about 10,500
  events/day): about 21 MB/day of segments and 33 MB/day of receipts,
  about 20 GB/year together.
- **User snapshots** [I]. A small store is 50 MB at 16 KiB pages (layout
  C), about 17-20 MB compressed. About 5 kept per 30 days, with chunk
  reuse. Estimate 45 MB per active computer per window, about 9 GB across
  the non-paper fleet (R0k-04).
- **Paper** [I]. Store 11.9-20.8 GB, about 4-7 GB compressed. Eight daily
  snapshots kept, with partial chunk reuse: estimate 20 GB (R0k-05). Hot
  segments about 3.4 GB at today's 0.49 GB/day compressed. Archive growth
  about 0.45 GB/day until the analytical database loads it (R0k-13).
  Control database growth drops from about 1.07 GB/day to about 0.1 GB/day
  once paper receipts are windowed (R0k-12).
- **Replay** [I]. In-guest SQLite at 500-3,000 events/s, against about 50
  events/s for today's host Dolt rebuild (2,500 events in 43 to 56 s, apply-checkpoint §3 option D). Typical
  user restore about 12 s; a broken VM adds boot and delivery; the paper
  about 12 min (R0k-07, R0k-08).
- **Audit and anchors** [I]. About 300 entries/day, under 1 MB/day; 48
  timestamp requests/day; one git commit/hour.
- **Deleted cost.** Projection bases (61.6 GB) and the host rebuild CPU
  (checkpointd at about 50 events/s per rebuild). Legacy pins (47.4 GB,
  7.4 M inodes) go per object storage. The guest event index drops its
  receipt copy (4.4 → about 1.2 KB/row, engine design).
- **Scale path.** Everything is per computer except the audit chain and
  anchors. Snapshot and replay work runs in the guest that owns it, so it
  scales with VMs, not with the host. The host does CAS, storage and
  signing.

## 9. Cutover

This joins object storage revision 12's single writer drain (§7); there is
no separate cut.

**Before the cut** (each on staging, d@e first, then widened by ring):

1. Envelope v2 sealing in the appender; the control list.
2. The guest snapshot producer. It streams pages inside one read
   transaction (`sqlite_dbpage`), or uses the backup API into a temporary
   file when space allows [I: whether `modernc.org/sqlite` enables the
   dbpage table].
3. `restore_applied` and `restore_failed`, the narrow append path, and boot
   closure.
4. Delivery at every boot; LUKS volumes.
5. The audit chain with named operators; anchors; the export tool and
   verifier.
6. The record-batch tag.

Each lands with its proof; the sealed-restore drills pass on d@e.

**At the cut** (computers stopped by the drain):

1. Record each computer's drained head H0. The audit genesis entry commits
   to the frozen transparency, owner-act, mode and deletion log heads.
   The first anchor covers every H0. Pre-cutover history keeps a witness
   of its existence even after its bytes go.
2. Each computer boots the cutover release. In the guest: the engine
   migration writes the new SQLite store onto a new encrypted volume;
   the DEK leaves the old image; the guest takes snapshot S0 at H0, the
   first post-cutover restore point (in place of object storage's
   `PublishCapturedBase` base); the tape continues from H0 sealed. Hibernate
   snapshots are discarded; the snapshot vault starts.
3. The paper's guest extracts its pre-cutover record-witness bundles from
   the plaintext tape (≤ 127,901 records, tape window sizing) and re-seals
   them as record-witness objects. Pre-cutover envelopes commit to
   plaintext digests, so they still verify after decryption. Open
   question 4.
4. A new restic repository starts, with keys and data split
   (10-21 record).

**Deletion order.** Object storage §7 "Deleted, not migrated", with these
added before anything is deleted:

- (5) the sealed-restore drills pass with the old plaintext moved out of
  reach: a user computer restored to an arbitrary head inside a fresh
  window, a broken-VM restore on d@e, a paper restore from S0;
- (6) the paper's record-witness bundles are verified against their
  receipts.

Then delete, under Q1: pre-cutover segments and legacy pins, bases,
checkpoints, plaintext `data.img` volumes and quarantines, the old restic
repository (after its retention, honestly stated), the escrow operator
token file and the reveal endpoint. Pre-cutover user tapes are retired
under Q1 (open question 5): their H0 receipts and anchor leaves stay.

**Rollback.** Inside the drain: object storage's one-act rollback; the old
images and tapes are still present. After writes resume: post-cutover
events exist only sealed, and the old release cannot read envelope v2. The
preferred path is a forward fix. The emergency path is an in-guest
decrypt into the old format, per computer, each an audited `export`. It
must be rehearsed once on staging before the production cut.

## 10. Changes required in other designs

**Object storage (revision 12; not edited here):**

1. §3 "No such channel exists ... Delivery is new work": wrong; key
   delivery exists and runs at boot (`key_delivery.go:45-144`,
   `run.go:335-372`).
2. §3 "host replay workers for projection bases ... under a computer-DEK
   lease": delete; bases are retired.
3. §1 table and §7 step 6 "the paper leaves the tape (owner 21:10)":
   superseded; the paper keeps a 7-day hot tape and archives older sealed
   segments. R0g-11 and R0g-12 assume step 6 writes only user tape and
   need correcting records.
4. §7 step 6, first post-cutover restore point: the guest snapshot S0,
   not `PublishCapturedBase`; delete that function.
5. §4 Checkpoint closures: closure refs come from the guest's snapshot
   manifest header (`hmac:` refs from the guest ref table at that head),
   not from a host verification rebuild, which can no longer read the
   store. A guest that under-reports harms only its own objects. A
   retained snapshot without closure refs still makes the provider unable
   to run.
6. §2: `tape-segment` stays internal but is host-named (the host writes
   segments; their payload records are already guest-sealed, and the host
   has no ID key). `base` is retired. Add `snapshot-chunk` (internal,
   `hmac:`, guest-computed) and `snapshot-manifest`.
7. Record header: a key-commitment field for every internal and private
   object, so a disclosed key is bound.
8. §4 providers: a snapshots provider (window, pins, holds); user
   segments never collected; paper segments move to archive, not purge;
   a legal-hold provider that refuses purge in a held scope.
9. §7 deletion order: add steps (5) and (6) above.

**Secrets vault (revision 2):** the computer DEK leaves the data image and
is delivered every boot (§2 decision 4, §4 restore ordering); its
`AuditSink` is the platform audit chain (§5); break-glass reveal of a DEK
is deleted, since export replaces it (§3); a sovereign owner's computer
boots only with the owner's client.

**Apply checkpoint without replay:** the host rebuild, verdict pass and
projection bases are deleted. A checkpoint is a guest snapshot manifest,
and derivability is checked by restore (which already re-verifies) and by
drills. The automatic cadence becomes this design's, on for every
computer (it is off today, G9). Retention slice 4 is this section 4.

**Data rearchitecture:** the checkpoint definition stands (tape head plus
current-state snapshot plus closure plus identities plus recoverable
keys). Add `restore_applied` as an epoch root.

**Precommitment records §6:** step 2's anchor is defined here (hourly; two
RFC 3161 authorities plus public git; record heads as leaves). Receipt
retention: users forever, the paper's record-witness set forever.

**Computer ontology:** a restore is forward events. If open question 3 is
accepted, records are carried across a restore, like external sends
outside the rewind.

**Guest engine design:** stream pages under a read transaction; per-table
witness hashes, including the guest ref table; drop the event index's
receipt copy; the record-batch tag.

**Problem records to write before any fix:** G1, and G4-G7 together.

## 11. Precommitted expectations

Round 0k is appended in full to
`docs/definitions/choir-data-architecture-selection-2026-10-28.precommit.md`;
this list is its index. Each record there states its estimate with an 80% interval or a probability, a
falsifier, the resolving artifact, and what is hard to measure.

- **R0k-01** G1 reproduces: restore, one owner edit, restart. p = 0.8.
- **R0k-02** No plaintext non-control payload after the cut. p = 0.95.
- **R0k-03** User segment bytes per event: est 2.0 KB [1.3, 3.2].
- **R0k-04** User snapshot bytes per 30-day window: est 45 MB per active
  computer [15, 150]; fleet est 9 GB [4, 30].
- **R0k-05** Paper snapshot bytes per 7-day window at daily cadence: est
  20 GB [8, 60].
- **R0k-06** Chunk reuse between consecutive snapshots: user est 0.6
  [0.25, 0.9]; paper est 0.5 [0.15, 0.85].
- **R0k-07** In-guest replay rate: user est 1,200 events/s [400, 4,000];
  paper est 600 [150, 2,500].
- **R0k-08** Restore wall time: user est 12 s [4, 45]; broken VM est
  100 s [45, 300]; paper est 12 min [4, 40].
- **R0k-09** Drill pass fraction in the first 4 weeks: est 0.98
  [0.92, 1.0].
- **R0k-10** Audit entries per day: est 300 [80, 1,500].
- **R0k-11** Anchor completeness: est 0.97 [0.90, 0.995]; no gap over
  6 h, p = 0.9.
- **R0k-12** Control database growth: est 0.1 GB/day [0.04, 0.3].
- **R0k-13** Paper archive growth: est 0.45 GB/day [0.25, 0.8].
- **R0k-14** Added boot latency from delivery: est 0.3 s [0.05, 2];
  delivery-caused boot failures in 30 days: est 1 [0, 8].
- **R0k-15** Carried records survive a d@e restore drill. p = 0.85.
- **R0k-16** Paper pre-cutover record-witness extraction: est 120 K
  batches [60 K, 200 K]; est 0.15 GB [0.06, 0.4] sealed.
- **R0k-17** `choir verify-export` passes on the first run: p = 0.8;
  est 20 s [5, 120].
- **R0k-18** Observe-mode owner-act receipts show no would-refuse verdict
  on intended acts in the 7 days before the cut. p = 0.6.

## 12. Open questions for the owner

Each has a default this design proceeds on (No Blocking Asks).

1. **Does "the full tape stays on the computer" mean the computer's own
   sealed scope on the host, with the guest disk holding working state
   only?** Default yes: one authority, no second copy, and the guest disk
   is on the same host anyway.
2. **Encrypt guest data volumes in the cutover?** Default yes: without it,
   "the host holds ciphertext" is false (G3).
3. **Carry records, resolutions and scores across a restore?** Default
   yes, so a restore cannot hide losing predictions.
4. **Keep the paper's pre-cutover record witnesses despite Q1?** Default
   yes; about 0.15 GB sealed.
5. **Retire the other pre-cutover user tapes under Q1, given the later
   "keep the whole tape" direction?** Default yes, Q1 stands (a settled
   decision, pre-launch data). Their H0 receipts and anchors stay. Any
   owner computer the owner names is re-sealed instead.
6. **Anchor cadence and target?** Default hourly; two RFC 3161 authorities
   plus a public git repository.
7. **Default tenant window?** Default 30 days for users; 7 for the paper
   (14 after slimming).
8. **Operator restore policy?** Default two named approvals plus owner
   notification before the flip; impossible for sovereign owners.
9. **Snapshot cadence?** Default for users: 10,000 events or 7 days,
   hourly at most, with 48 h, daily and window thinning. Default for the
   paper: daily.
10. **Enterprise maximum history retention?** Default unset (keep
    forever); a tenant may set one, enforced as audited deletion.
11. **Fold `rollback_*` into restore with scope `release`?** Default
    later, as a named follow-up.
12. **Switch owner-act mode to `enforce` at the cut?** Default yes, after
    R0k-18's observe-mode check.
13. **Delete the raw DEK reveal?** Default yes; export to a named
    recipient key replaces it.
14. **Paper archive retention before the analytical database exists?**
    Default keep every sealed segment (about 0.45 GB/day), reviewed when
    the database lands.

## 13. Ceremony

**Conjecture delta.** Before: restore, audit and privacy were separate
problems, each needing new machinery (host-sealed segments, host-built
bases, more logs). After: one encrypted event log plus guest-made
encrypted snapshots carries restore, PSA witness and audit. The host's
role shrinks to CAS, storage, signing and anchoring. Net mechanisms
removed: checkpointd's rebuilder and verdict pass, projection bases, base
capture, the routine unwrap, the raw reveal, the operator token file, five
of six logs, the plaintext pin path and its class mapping. Added:
envelope v2, the guest snapshot producer, two event kinds and a narrow
append path, the audit chain, anchors, the export verifier, volume
encryption.

**Protected surfaces.** Tape and evidence (appender, envelope, pin
service, segments); checkpoint and route projection (bases, checkpoints,
restore); key custody (escrow, delivery, DEK placement); auth (operator
identity, owner-act enforcement); boot path (delivery before volume
mount); GC (holds, legal hold); run acceptance (verification decisions
stay readable).

**Admissible evidence.** Staging (`choir.news`) runs on d@e first, then by
ring:

- R0k probes with artifacts (segment samples, `du`, catalog sums, timing
  logs);
- sealed-restore drills with the old plaintext out of reach;
- `choir verify-export` on a real export;
- anchor tokens checked against the authorities;
- a review of the audit chain over a run that includes an operator
  restore.

Local tests are not admissible for delivery, vmctl restore, anchors or
restore durability.

**Rollback path.** This note: delete the file and the round 0k block.
The build: per section 9. Before writes resume, object storage's one-act
rollback. After, a forward fix, with an audited in-guest decrypt export as
the emergency path, rehearsed on staging.

**Heresy delta.**

- *Discovered:*
  - G1, restore neither forward nor durable;
  - G2-G5, the guest holds no payloads, the DEK on the image, the routine
    host unwrap, host plaintext readers;
  - G6, fragmented, unsigned, unanchored, internal-head audit, the raw DEK
    reveal, shared operator tokens;
  - G7, owner acts not enforced;
  - G8, no hold, retention or export, unbounded receipts, unrotated
    platform key;
  - G9-G10, no restore points for most computers, current-head-only cold
    recover;
  - G11, mislabelled pins and public effect bundles with workspace
    paths;
  - G12, stale claims in object storage and tapestore.
- *Introduced (named, accepted residuals):*
  - envelope and receipt metadata stays visible to the host;
  - two plaintext control kinds;
  - non-sovereign computers are opened by an automated, audited unwrap
    at every boot (the platform can read them by design, visibly);
  - records survive a restore (if open question 3 is accepted), so a
    restore is not a pure rewind;
  - compress-then-seal exposes compressed sizes;
  - forward-only DEK rotation leaves history under a compromised
    version exposed unless erased.
- *Repaired:* none. This is a design.

**Human-learning digest.** The restore that exists today looks complete
in its test and is undone by the next boot. The host already holds the
key path the object-storage design said was missing. And the cheapest
route to "the host cannot read it" is not a better host-side seal but
moving every seal, snapshot and rebuild into the guest that already holds
the key.
