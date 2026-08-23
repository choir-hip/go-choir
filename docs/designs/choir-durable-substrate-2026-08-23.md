---
definition_id: choir-durable-substrate-2026-08-23
execution_mode: document
mutation_class: red
status: design-candidate-awaiting-review
---

# Durable, Secure, Recoverable Computer Substrate — Design

Date: 2026-08-23
Mutation class: red
Status: design candidate; owner-ratification and a CS review round are the
required next gates before implementation.

This document is the consolidation of four agentic-consensus rounds (divergent
and convergent) on the recovery/replay failure of
`computer-03335285269bdba4f94377e56879f9e6`, generalized into the durable
substrate design. It supersedes the per-incident `recover_current` fix loop and
is the substrate response to the documented Root Cause Clustering trigger.

A fifth round (a rigorous CS design review against the history of computing) was
then run and its corrections are folded in below. The review verdict: the
**primitives are historically sound and precedented** (Git/OSTree content-
addressed CAS + Merkle root, ZFS transaction groups, envelope encryption with a
per-owner KEK over per-computer DEKs, LUKS keyslots, passkey-PRF wrapping only
the root key, Postfix store-and-forward + Maildir, restore drills with measured
SLOs). But the design had **stated two contradictory durability clocks and
attempted to use the semantic event tape as the filesystem commit protocol**
("a bad Git/ZFS clone, not the clear form" — grok46). Those two contradictions
are corrected in §3.1 and §3.3.

---

## 1. The problem, stated precisely

Choir is a **multi-user persistent-computer platform**. The HOST is the shared
platform level (the durable, security, and recovery authority). Each OWNER is a
person (UUID). Each owner holds one or more COMPUTERS (VMs). The platform must
give an organization that trusts it with its workforce's VMs assurance that:

1. A computer can be lost or corrupted and the owner does **not** lose data.
2. The owner can regain access even if a device (and its on-disk key) is gone.
3. No user can read another user's mail, files, or keys.
4. Recovery time is bounded and published honestly.

### The four gaps found (verified, 2026-08-23)

1. **Files are not durably backed up.** The `files` root
   (`AUTOPUTER_FILES_ROOT=/mnt/persistent/files`) — Source checkouts, blobs,
   model-policy files, caches — is **not** in the event tape. There is no
   `ProjectionOpFiles`. `recover_current` copies only the privacy key then
   discards the disk. On VM corruption files are unrecoverable.
2. **Mail is host-side by accident, not by design.** It is per-**owner** SQLite
   (`mailboxes[ownerID]`, `mailbox_owner_id`), not per-computer-address, and not
   accessible from the computer. It survived this incident only because the
   host did.
3. **The data-encryption key is a random file on the guest disk.** It is a
   per-computer 32-byte XChaCha20 key, generated once, stored in a mode-0400
   JSON file, not derived from anything. On full-disk loss the key is gone and a
   perfect base is undecryptable. Passkey auth exists but there is **no
   passkey-based DATA-key recovery** (`auth/recovery.go` only re-provisions a
   WebAuthn credential after a lost device).
4. **Recovery time is bounded by O(history) at boot.** 132,436 events ×
   (per-event serializable transaction + per-op SQL) under a 30-minute
   `bootstrapCtx`/`VM_BOOT_READY_TIMEOUT`. This is the current failure.

---

## 2. Architecture model (corrected)

- **HOST = shared platform** — the trusted durable authority. Runs auth, proxy,
  vmctl, gateway, autoputer, maild, corpusd.
- **OWNER = person** (UUID). One owner → many computers.
- **COMPUTER = a VM** (`computer-<id>`), owned by an owner.

Three durable ledgers, three transition authorities, **never collapsed into one
"backup project,"** per the standing-questions authority-collapse warning and
the doctrine's Ledger Split. The three ledgers are:

1. **Canonical event tape** (guest `ComputerEventAppender` → corpusd `HeadCAS`).
2. **Blob / file-CAS** — the content-addressed artifact store (extend the existing
   `platform-artifacts` sha256 store, or adopt a proven repo format; one of the
   design's open decisions #4). This is the recovery/restore substrate.
3. **Key wrap table** — per-owner ROOT key wrapping each computer DEK, held as a
   table under KMS/HSM quorum. It is a key table, never an event store and never
   on the tape.

**Mail is NOT a fourth ledger.** Mail is a data class under the file-CAS authority
(`§3.3`): the guest mailbox is a Maildir in `/mnt/persistent` under the file-CAS
durability, and the host MTA spool is a transient in-flight delivery queue, not a
recoverable ledger. Mail restores with the computer; there is no independent mail
recovery path and no third transition authority for mail (this is the correction
the owner directed and the review confirmed).

---

## 3. Design decisions

### 3.1 Files — content-addressed file-CAS as primary; guest is a write-back cache

**Decision: a Git/ZFS/restic-shaped commit protocol, with the semantic tape
citing the root but NOT being the filesystem WAL.** Encrypted file chunks are
pinned to a platform content-addressed file-CAS; a small **snapshot-ref** (the
Merkle root of the current file tree) is published via `FileRootCommitted` into
the event tape. The tape owns the root *reference*; the CAS owns the bytes; the
guest disk is a **write-back cache**, not write-through.

**The commit protocol (one coherent durability boundary — fixes the
WAL-vs-checkpoint contradiction):**

- The guest data disk is a **write-back cache with epoch-based asynchronous
  checkpointing** (the ZFS transaction-group / SQLite-WAL-checkpoint model, per
  LFS/WAFL precedent). Local writes ack immediately to guest processes.
- A checkpoint is: pin all changed chunks to CAS (content-addressed, encrypted,
  fsync'd) → fsync-ordered **snapshot-ref** publish (the root) → append
  `FileRootCommitted`. **The snapshot-ref is the durability boundary.** Root
  commit is always causally after the bytes it names (Git object-before-ref).
- **Explicit barrier:** a guest `sync_computer_files()` RPC/syscall forces a
  checkpoint; the guest appender's durable-ack path uses it.
- **Honest RPO:** the uncommitted-file RPO is bounded by the `FileRootCommitted`
  flush interval ($\Delta t$), NOT zero. The design does NOT claim pin-before-ack
  for ordinary writes. This is the correction to the earlier contradictory
  "pin-before-every-write-ack + root-at-idle" — those two cannot both hold.

**An alternative the review also accepted (choose one coherent point):**
synchronous pin per write PLUS a small per-write journal record (path + chunk
hashes, ~few hundred bytes — nowhere near the 1 MiB/8 MiB payload ceiling),
replayed from the last snapshot-ref. This gives write-through RPO=0 at the cost
of a per-write host RTT. The default is async-pin + root-commit-as-boundary
($\Delta t$ RPO); the journal variant is the write-through option.

- **Write path:** `O(changed chunks)`, never `O(file size)`.
- **Recovery:** cache hydrate from CAS `O(metadata + requested live bytes +
  post-root events)`, never `O(full tape + empty disk)`.
- **Payload-size bound respected:** only the root goes on the tape, never
  per-write bodies (the tape already failed at 1 MiB then 8 MiB payload ceilings).
- **Snapshot consistency (no torn copies):** each checkpoint is taken at an
  **application-consistent freeze** (guest reflink/CoW snapshot, or SQLite
  backup API, or a crash-safe per-message store) — never page-by-page chunking
  of a live filesystem or live SQLite. For mail specifically this is the
  Maildir choice (§3.3).
- **GC (referential-integrity-anchored, defined up front):** never collect a
  chunk reachable from any live `FileRootCommitted` root or any recovery capsule
  manifest (including customer media). Specify a grace window and walk all roots
  a recovery path can name. Retrofitted GC is the classic corruption vector
  (Git `gc.pruneExpire`, IPFS pinning) — it must be in the design, not added later.
- **Rejected:** files-in-tape bytes (too large, bloats tape); host ext4
  surgery (the stale-manager-warmness-race class); overlay/FUSE as a form
  (disproportionate lifecycle surface); sealed-file overlays; treating the
  semantic event tape as the filesystem WAL.
- **`FileRootCommitted` cadence:** a policy (e.g. commit at session/idle
  boundaries) — document as its own conjecture; too frequent bloats tape, too
  sparse widens the uncommitted-file RPO. This cadence IS the RPO.

### 3.2 Keys — keep the random DEK; wrap it under a recoverable hierarchy

**Decision: recoverable-by-default, sovereign-by-election.**

- **Keep** the existing per-computer 32-byte XChaCha20 DEK exactly as-is
  (`internal/computerevent/privacy.go`). Do not change the crypto.
- **Wrap each DEK under a per-owner ROOT key**, held in **redundant independent
  wraps** (envelope encryption; LUKS-keyslot-style multiple protectors). The
  three wrap protectors are ordered by what exists today:
  1. **User-device passkey (owner-held, WebAuthn PRF extension)** — derives a
     wrapping key for the owner root key from the owner's platform authenticator.
     **Never wraps bulk data** (1Password/Bitwarden model). Device rotation
     rewraps, never re-encrypts. **This is the real existing user-held primitive
     — the owner's passkey.** The PRF extension that turns it into a wrapping key
     does **not exist yet** and is a Track K build item, not current code.
  2. **Owner offline capsule** (paper/device share) — a separate out-of-band
     channel. Current: none built; it is a Track K build item.
  3. **Platform-side escrow (host-held custodian wrap)** — the platform holds a
     wrap of the DEK under a host-held escrow key so it can recover under
     ceremony. **There is NO KMS, NO HSM, NO vault in the codebase today.** So
     this escrow is a **host-held key under an application-level two-approval
     authorization gate** (two named operator accounts approve an unwrap; a
     product-layer audit record), NOT a cryptographic HSM quorum. "HSM
     dual-control" / "non-exportable HSM key destruction" are **deferred target
     primitives** (a HSM/KMS/transparency-service build), to be built or
     explicitly assumed in Track K — they are NOT current infrastructure and
     must not be described as if they are.

  Optional 4th protector: org custodian (enterprises needing legal hold /
  offboarding) — deferred to Track K.

- **The privacy-vs-recoverability tension, explicitly resolved:** the platform
  holds a custodian escrow and therefore *can* decrypt under the approval gate —
  the price of a workforce-contract SLO. It is **not zero-knowledge by default.**
  Mitigations (all deferred to Track K, none current):
  - Per-DEK scope (never fleet-wide; each unwrap is per-computer).
  - **Transparency logging, not per-unwrap receipts** (which are theater
    against a platform operator who controls the ledger): unwrap receipts go
    into an append-only log whose head is externally witnessed (Certificate
    Transparency / Key Transparency model), so *absence* of a receipt is
    detectable. The two-approval gate is logged; if it later becomes
    cryptographic, it is Shamir k-of-n on the custodian wrap (not current).
  - **Sovereign election via crypto-erasure:** "delete the custodian wrap"
    understates crypto-shredding — platform backups and already-shipped capsules
    retain it. The precedented mechanism is destruction of a **non-exportable
    HSM key** in the wrap chain, plus re-minting pre-election capsules. **This is
    black-class irreversible and belongs in the Track K successor with its own
    ceremony, NOT in the Rail A red Definition.**
- **Passkey PRF is 1b, not primary** (PRF availability across owner
  authenticators is uneven; ship the non-PRF wrap first).
- **Migration clamp:** existing computers have unwrapped DEKs on disk. The
  wrap-upgrade is **lazy per-boot**, not a big-bang re-key. Computers already
  past full-disk loss cannot recover files that were never CAS-pinned.

### 3.3 Mail — host MTA relay + guest Maildir SoR (CORRECTED)

**Decision: mail is a data class under the existing file-CAS + tape authority,
NOT an independent recovery ledger.** Host maild is a **Postfix-shaped MTA**;
the guest mailbox is a **Maildir** on the file-CAS. This is the original Unix
mail model (qmail Maildir, 1990s) and is crash-safe AND CAS-friendly by
construction.

- **Host maild = MTA + delivery semantics + durable spool queue.** It handles
  ingress (Resend/provider), abuse/quarantine, address resolution
  (`mailbox_id → computer_id`), rate limits, and a **durable host spool queue**
  (Postfix `qmgr` model: fsync'd on write) for in-flight mail. The spool — NOT
  the guest — is the SoR for in-flight mail.
- **Inbound SMTP contract (RFC 5321 store-and-forward — fixes the contradiction):**
  the host sends SMTP `250` only after the message is durably written to the
  **host spool** (fsync'd), NOT after the guest CAS-pins. Delivery to the guest
  is **asynchronous** (LMTP/local-MDA style) with retries and backoff. The guest
  pins the message to CAS + commits the root, then the host deletes it from the
  spool **only after durable+idempotent guest acceptance** (Message-ID dedup).
  This respects 40 years of MTA law: a down VM means queue-and-250 (not chronic
  4xx tempfail, which gets senders junked), and the spool is the durable accept
  point.
- **Guest = mailbox SoR.** The mailbox is a **Maildir tree** (one immutable file
  per message — crash-safe, and each message is an ideal CAS object) in
  `/mnt/persistent`, DEK-encrypted, covered by the same file-CAS durability.
  Read/draft/send-record all live in the guest.
- **Recovery is free:** mail restores automatically with the computer (file-CAS
  hydrate + tape replay). No mail ledger, no mail recovery path, no third
  transition authority.
- **Per-computer-address:** each computer has its own email address. The address
  resolves to `(cloud_id, owner_id, computer_id, mailbox_id)`.
- **Isolation is cryptographic:** mail lives inside the computer's own
  `data.img`, encrypted with the per-computer DEK. Cross-user visibility is
  **structurally impossible** — the host relay never stores message bodies
  beyond the transient spool, and the guest volume is per-computer-encrypted.
- **Outbound:** a send has irreversibly left the building. The **send-record**
  is durable (CAS-pinned) at send time. Duplicate-delivery on unknown outcome is
  modeled explicitly (SMTP's fundamental exactly-once ambiguity); never promise
  exactly-once.
- **Rejected alternatives:** SQLite mailbox under chunk-CAS (torn page /
  write-amplification — Maildir is the crash-safe choice); a third independent
  mail recovery ledger (overfit; the owner directed and the review confirmed it
  as unnecessary).
- **Authorization:** a computer-scoped capability; one computer's relay
  capability cannot read another's mail. Replace the current owner-scoped
  `X-Authenticated-User`/`X-Internal-Caller` mail auth (`maild/api.go:152-161`).
- **Current code to change:** `maild/store.go` is per-owner
  (`mailboxes[ownerID]`, `mailbox_owner_id`, `target_type` without a computer
  dimension). `maild/api.go` auth is owner/header-scoped and needs a
  computer-scoped capability. `maild/ingest.go` selects the store via
  `alias.TargetID` (owner).

### 3.4 Assurance — portable recovery capsule + continuously-proven restores

- **Portable Recovery Capsule:** a self-describing encrypted artifact binding
  the file-root manifest, reachable blobs, event head, projector/schema/reducer
  versions, mailbox cursor/generation (Maildir), key-wrap set, release identity,
  and a verification program. Stored on **platform + customer + owner media**
  (customer-owned S3/private-cloud for independent custody). **Custody copies
  must be pull-based or object-locked (WORM)** — if the platform pushes and can
  also delete, a ransomware-class compromise takes data and backups together.
- **Continuously-proven restores:** an automated daily restore drill in an
  isolated realization; **only publish SLO numbers the drill actually produced.**
  No numeric RTO/RPO claimed until measured.
- **Scrub:** daily drills validate one path; rare-blob bit-rot needs a
  continuous background **integrity scrub with repair** (ZFS scrub / par2 /
  erasure coding). Publish scrub coverage as part of the SLO. Silent corruption
  "finds the blob you never restored" — 40 years of backup lore.
- **Ceremony in drills:** drills must exercise the **key ceremony** (the
  application-level two-approval unwrap + passkey-PRF, when those exist in Track
  K), not just data restore — DR tests that rehearse data but not key escrow
  famously fail on the day. **Deferred to Track K / Assurance; no KMS/HSM exists
  today.**
- **Blast-radius contract:** a single recovery event cannot exhaust the
  workforce (recovery cells).

### 3.5 Scale — recovery cells, per-owner fairness, no cross-owner dedup

- **Recovery Cell Architecture:** partition the workforce into independently
  encrypted storage/recovery cells with per-cell restore budgets. A cell failure
  cannot exhaust or expose the rest of the workforce. A cell with no replica in
  another cell is a durability domain, not just a performance domain (correlated
  restore after cell loss is the thundering herd the cell was meant to prevent).
- **Cross-owner dedup is FORBIDDEN** (it leaks across the DEK boundary), even if
  storage-costly. Dedup is per-owner/per-cell only.
- **Scheduling controls (more than bandwidth):** admission control, weighted
  fair queue, max-inflight, shuffle sharding — restore load is decrypt CPU and
  metadata IOPS, not just bytes.

---

## 4. The irreducible tension, owned

**Recoverability requires a platform-side decryption path; privacy requires the
platform to be unable to decrypt.** The design chooses
**recoverable-by-default (escrow custodian)** because it is the only path to a
workforce SLO. The cost: a malicious platform operator could decrypt under the
custodian wrap. Mitigations are per-DEK scope, transparency-logged unwraps with
externally-witnessed head, cryptographically-enforced dual control, and the
voluntary Sovereign election (via non-exportable HSM key destruction). This is
a **product-values decision requiring owner ratification**, not a code default.
True zero-knowledge is opt-in and must be clearly documented as downgrading the
published SLO.

---

## 5. Execution plan (Rail A first; then the substrate tracks, off the boot-critical path)

**Ordering override (2026-08-23 — live stopped computer):** the general
substrate ordering below says "Track K first," but there is a LIVE stopped
computer (0333528, epoch 361) with no healthy differential base. Track K does
not boot a stopped VM and does not manufacture a projection base. **Rail A — the
offline full-tape rebuild + first `ProjectionBase` + O(delta-path) recovery of
0333528 to head 132,436 — executes FIRST** as the singular finish of the
`choir-durable-substrate-2026-08-23` Definition. Only after the computer is
active does the substrate program proceed to Track K, then Track F, then
Track M. The O(delta) *proof* (a later kill-and-recover against a published
base) is a Track F successor, not Rail A.

- **Rail A (this Definition):** isolated, resumable offline full-tape rebuild →
  publish the first `ProjectionBase` as a content-addressed blob in a NEW
  NAMESPACE of the existing `platform-artifacts` sha256 store (no third root,
  no new store; durability is tmp+rename, NOT restic-class) → recover 0333528
  via base-verify → hydrate → replay H+1..head → final head+witness → route CAS
  → active, with no-rewind refusal + host-cannot-HeadCAS structural proof +
  quarantine preserved. **The offline rebuild decrypts batch bodies with the
  existing guest 32-byte XChaCha20 DEK** (via the trusted-guest privacy-key
  copy); there is no KMS/HSM/wrap — those are Track K.
- **Track K (keys, zero guest change; SECOND, after Rail A):** host-side wrap
  table; escrow the DEK on computer creation; passkey-PRF (WebAuthn PRF
  extension, not built) as 1b; host-held escrow under an application-level
  two-approval authorization gate. Lazy per-boot wrap-upgrade for existing
  computers. **There is NO KMS/HSM/vault in the codebase; the wrap table is
  declared for Track K, not current, and Rail A must not create it.**
- **Track F (files; THIRD):** the Git/ZFS-shaped commit protocol — CAS pin →
  fsync-ordered snapshot-ref → `FileRootCommitted`, with the explicit
  `sync_computer_files()` barrier; guest-as-write-back-cache; application-
  consistent checkpoints; referential-integrity-anchored GC. The RED restore
  step (materialize root pre-replay) lands alone with its own staging proof. Do
  NOT touch `recover_current`'s boot path until this ships (Rail A is an
  offline job, not a `recover_current` change).
- **Track M (mail; FOURTH):** per-computer-address; Postfix-shaped host MTA +
  durable spool; guest Maildir SoR; computer-scoped relay capability.
- **Then (Assurance & Scale, FIFTH):** recovery capsule (pull-based or WORM
  custody) + daily restore drills (including key ceremony) + recovery cells.
  Publish SLOs only from measured drills. **None of these exist today on the
  single staging host; they are deferred successor build items, not current
  mechanisms.**

The Root Cause Clustering assessment must be written first and must state that
this design **is** the substrate repair direction (it feeds the assessment, not
bypasses it).

---

## 6. Hard invariants (do not violate)

- No rewind of canonical events; the guest `ComputerEventAppender` remains the
  only semantic event writer; recovery is forward-to-current-head.
- No host ext4 parsing/writing as the recovery mechanism (read-only diagnostic
  read is permitted).
- No disproportionate new lifecycle/overlay/mount bug surface (the
  stale-manager-warmness race was that class).
- Strict per-user isolation at the platform layer: a user is never able to read
  another user's mail, files, or keys.
- Problem-documentation-first; effects remain OFF; mutation class RED.

---

## 7. Open decisions requiring owner ratification

1. **Recoverability-by-default (escrow custodian) vs. sovereignty-by-default
   (zero-knowledge).** The design defaults to recoverability; the owner must
   ratify this values choice, transparency-logged unwraps, dual-control, and the
   Sovereign-election (crypto-erasure) mechanism.
2. **`FileRootCommitted` cadence policy** (RPO for uncommitted files) — this IS
   the honest RPO, so it must be a named, ratified number.
3. **Filesystem commit authority:** adopt a COW block/store substrate (replicated
   COW volume + object CAS), or a write-through journal, or async root-commit as
   the default. This is the single most important unresolved engineering choice.
4. **CAS substrate reuse (split by scope):** (a) Rail A resolves this NOW — use a
   NEW NAMESPACE under the existing `platform-artifacts` sha256 root for the
   first `ProjectionBase`; its `writeBlob` is tmp+rename with no fsync, so it
   is content-addressed files but NOT restic-class durability. (b) Track F
   re-opens extend-vs-adopt (restic/borg) for the full file-CAS commit protocol.
   Either way: no third store, no new root, and Choir Base stays non-canonical.
