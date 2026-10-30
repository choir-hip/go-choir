# Signing precommitment records, and what blocks real computer keys

Research note, 2026-10-11. Answers the owner's question on
[precommitment-protocol-design-2026-10-11.md](../../precommitment-records.md)
§9 item 8: "Signing, I trust your judgment. But what blocks 'real computer
keys'?" Read-only investigation of code at `main@4b114e61` (staging at
`b172895b`). Mutation class of anything recommended here: red (signer
custody, the evidence ledger, the event chain receipts).

## Summary

**What exists.** Choir already has more key machinery than §9 implied, but
none of it is a computer identity key:

- **The platform key.** One host-held Ed25519 key in corpusd signs every
  event-head receipt, payload pin, checkpoint and credential envelope for
  every computer. Its public half is pinned in the repo.
- **Guest-generated keys on each computer.** Each computer generates two
  Ed25519 keys inside the guest (`guest-core` and `verifier-control`) and
  keeps them on its data disk. The state-homes inventory classes them as
  belonging to one realization, not to the computer. A fresh disk mints new
  ones without warning.
- **The verifier key is pinned on first use.** The host records it per
  computer, in a key-history table that already has columns for activation
  and revocation.
- **The guest-core key is never registered anywhere.** Verifiers accept
  whatever public key the guest presents.

**The finding that changes the question.** Every commitment record a desk
writes already passes through the computer event chain as a projection
batch, and the platform signs that batch's event-head receipt with a host
clock. So a record's commit time is already witnessed by a party the
computer does not control. A computer key would not improve that: a
signature proves who signed, not when. Anyone holding a key can sign a
backdated record at any time.

**What blocks real computer keys.** Six separate things, most of them
decisions and missing code rather than hard problems:

1. Nobody has decided whether the key belongs to the computer (it survives
   a lost disk and is delivered to the next realization, as the privacy key
   now is) or to the realization (certified by something longer-lived).
2. No registry binds a ComputerID to a guest-core public key, and nothing
   publishes such a binding.
3. No rotation or revocation ceremony exists. The schema is there; the
   code is not called.
4. Fork re-keying (S9) and snapshot one-shot rules (S3) are deferred, and
   both are needed before a disk or memory copy can exist.
5. No canonical frozen body of a commitment record exists to sign. The
   content hash that §10 reserves has not been added, and the record object
   is mutated after commit.
6. A limit no key design removes on this hosting model: the host operator
   can read every guest key. `data.img` is a plain image file on Node B, VM
   memory is readable by the host, and a host-side debugfs copier existed
   until the SH station deleted it.

**Recommendation.**

- **Do not sign with a host-held per-computer key.** It would be the
  platform key under a computer's name. It adds no protection, and it plants
  a mislabeled authority that later readers would have to discount.
- **Now:** make the existing platform witness explicit on every record (a
  content hash over a frozen commit body, plus a pointer to the event-head
  receipt that first carried it). Label records `platform-witnessed,
  unsigned`. Before records count as a public track record, add two things.
  First, publish all commitment hashes when they are made, so a computer
  cannot publish only its winners. Second, anchor the platform's heads
  externally, so the operator cannot backdate.
- **When real keys land:** records also carry a computer signature over the
  same commit body. Old records are never re-signed.

**Confidence.**

- High on what exists: code-cited below.
- Medium on the claim that the platform witness already covers every desk
  record. The projection-tape path is code-cited, but I did not trace a
  specific record to its receipt on staging.
- Hypotheses are labelled.

## 1. What exists today

### 1.1 The platform key (host-held, one for all computers)

- `internal/platform/signing.go:48` `LoadOrCreateSigningKey` generates an
  Ed25519 key on first start and writes it to disk. The default path is
  `$CORPUSD_ARTIFACTS_ROOT/signing-key` (`internal/platform/config.go:36`),
  which on Node B is `/var/lib/go-choir/platform-artifacts/signing-key`
  (`config.go:12`; `nix/node-b.nix` sets the root and does not set
  `PLATFORM_SIGNING_KEY_PATH`). The code comment says production should
  provision a backed-up key instead. I found no evidence that one has been
  provisioned or backed up (open question 1).
- That key signs as domain `platform-control`. It signs the event-head
  receipt for every appended computer event
  (`internal/platform/computer_events.go:232`,
  `event_artifacts.go:187-192`). It also signs payload pin receipts,
  checkpoint receipts, the `VerifierKeyPinned` receipt
  (`checkpoints.go:172`), credential envelopes
  (`credential_envelope.go`), the `ExecutionIdentityJoin` attestation
  (`checkpoints.go:434-470`), and per-revision Texture attestations
  inside the publication manifest (`signing.go:15-22`,
  `version_history.go:23`).
- Its public key is pinned in the repo as the staging trust root, and
  "rotation requires a reviewed overlap policy"
  (`internal/computerevent/appender.go:1091-1097`). The comment at
  `signing.go:36` says rotation is out of scope.
- Guests verify event-head receipts against the platform key delivered in
  their credential envelope (`internal/selfdev/credentials.go:561-572`).

### 1.2 The guest-core key (generated in the guest, belongs to a realization)

- `nix/autoputer-vm.nix:629-660` runs `choir-receipt-signer --mode
  guest-core` as its own user, with the key at
  `/mnt/persistent/choir-signers/guest-core/key.ed25519`. The key is
  generated in the guest by `internal/receiptsigner/signing.go:17`
  (create-if-absent, mode 0600, owner check).
- It signs only an allowlist: `HealthReceipt`, `MaterializationReceipt`,
  `UpdaterRecoveryReceipt`, `KernelCapabilityReceipt`, `ExecutionIdentity`
  (`internal/receiptsigner/receiptsigner.go:102-109`). It refuses a
  `computer_id` that is not its own and an `issued_at` outside the window
  from ten minutes in the past to one minute in the future
  (`receiptsigner.go:80-90`). It cannot sign commitment records.
- **No host registry.** No `control_key_history` row is ever written for
  `guest-core`. Verifiers trust the key the guest presents:
  - `internal/proxy/execution_identity.go:148-156,188` resolves to "the key
    in this envelope".
  - The guest's own materializer asks the local signer for its key
    (`internal/agentcore/self_development_materializer.go:327`).
  - The host keeps only a digest of the key inside per-request
    `ExecutionIdentityJoin` attestations (`checkpoints.go:422,465`).
- The state-homes inventory calls these keys "realization-scoped identity,
  probably correct … created per realization", with "trust in a new
  realization's signer" unverified
  (`docs/operational-invariants-register-2026-10-08.md:42,73`). A fresh disk silently
  mints a new key, because the loader creates a key whenever none exists.

### 1.3 The verifier-control key (generated in the guest, pinned on first use)

- Same signer binary, `--mode verifier-control` (`autoputer-vm.nix:675`).
- On a computer's genesis checkpoint, or on first use for imported chains,
  the host pins the key into `control_key_history` with a platform-signed
  `VerifierKeyPinned` receipt (`internal/platform/checkpoints.go:105-160,
  172, 196`). Afterwards a different key is refused: "verifier trust
  substitution refused" (`checkpoints.go:146`).
- The code comment says the "verifier key lives on the guest data volume
  and cannot be observed host-side" (`checkpoints.go:133`). That is false
  against the host operator (§1.5).
- **Hypothesis, unexercised:** after a lose-the-disk re-realization, the
  new realization mints a new verifier key, and verifier-evidence
  checkpoints for that computer would then be refused. The SH
  lose-the-disk proof
  (`docs/evidence/sh-lose-the-disk-2026-10-09T14-44-09-081Z.json`) did not
  publish a checkpoint afterwards. A read-only search of Node B journals
  since 2026-10-01 found no "substitution refused" lines. This is the same
  key-scope question as §2 item 1, already latent in production code.

### 1.4 Rotation scaffolding that nothing calls

- `control_key_history` has `activation_sequence`, `activation_time`,
  `first_invalid_sequence`, `first_invalid_time` and `replacement_key_id`.
  The resolver enforces them (`internal/platform/control_keys.go:33-61`).
- `activateControlKey` and `revokeControlKey` (`control_keys.go:88-117`)
  have no callers outside tests.
- The event kind `key_rotated` is declared (`internal/computerevent/event.go:56`)
  and never appended. `key_revoked` is used, but for bearer-credential
  epochs, not signing keys (`internal/platform/computer_events.go:264`).
- The schema is ready for a rotation-aware per-computer key; the ceremony
  is not.

### 1.5 Where private keys live, and who can read them

| Key | Generated | Stored | Readable by |
|---|---|---|---|
| platform-control | corpusd, on host | host file | host root, corpusd |
| guest-core, verifier-control | guest | `/mnt/persistent/choir-signers/*` on `data.img` | the signer unit; the guest runtime as root, by hypothesis (below); host root (disk image and VM memory) |
| privacy DEK | guest | `/mnt/persistent/choir-credentials/privacy-key` | guest runtime; host custodian escrow under two approvals |
| owner passkey PRF root | owner's authenticator | wrapped per credential | owner (Track K 1b, unit-proven) |

Details behind the table:

- `data.img` is created as a plain file under
  `/var/lib/go-choir/vm-state/<vm>/` (`internal/vmmanager/manager.go:774-777`).
  I found no LUKS, dm-crypt or fscrypt anywhere in the repo.
- The SH definition names "deletion of the debugfs copier" as an
  artifact (`docs/archive/definitions/choir-appdev-sh-state-homes-2026-10-08.md:63`).
  Host-side reading of guest files was routine until then.
- The metamission defers "encryption at rest for snapshots and exports"
  and a "signer and credential lifecycle review"
  (`docs/archive/definitions/choir-supervised-app-development-metamission-2026-10-01.md:1259-1264`).
- Only the privacy DEK is escrowed (`internal/autoputer/run.go:400-407`).
  The signer keys are not, so a lost disk loses them. The 10-01 audit
  flagged exactly this
  (`docs/reports/nixos-agent-platform-redhat-deepseek-audit-2026-10-01.md:643,652`).
- **Hypothesis:** the autoputer runtime runs as root with no capability
  bounding set (`nix/autoputer-vm.nix:879-891`). `InaccessiblePaths` hides
  the key directory but does not stop a root process with
  `CAP_SYS_ADMIN` from reaching the block device. Runtime confinement is
  S1's job. In practice, any code execution inside the runtime can already
  call the guest-core socket for its allowlisted kinds
  (`internal/agentcore/api_execution_identity.go:116`).

### 1.6 The witness every desk record already has

Every object-graph mutation in a production guest is routed through the
projection tape:

- `internal/store/projection_tape.go:21-37,96-131`, bound at boot in
  `internal/autoputer/run.go:427-433`.
- Commitment records are object-graph objects of kind
  `choir.commitment_record` (`internal/types/commitment.go:11-12`).
- Each batch is pinned on the host with its sha256 as the event's
  `PayloadCommitment` (`internal/computerevent/appender.go:332-390`).
- corpusd checks the guest's projection and signs an `EventHeadReceipt`.
  The receipt binds `computer_id`, `sequence`, `previous_head`,
  `event_digest` and the pin receipt digests, under host time
  (`internal/platform/computer_events.go:195-245`,
  `internal/computerevent/event_receipt.go:8-61`).

The receipt is a platform-signed, hash-chained, host-timestamped statement:
"computer C submitted content D as event N after head H, no later than T."
That is the "store-assigned commit time and content hash" §9 asked for,
and it already exists for every record the desks write.

**Hypothesis:** I did not trace one specific P1 record to its receipt on
staging, and batches may carry other objects alongside the record.

What is missing around it:

- The record's own `provenance.committed_at` is stamped from the guest
  clock (`internal/agentcore/rlm_reduce.go:1091`). It is a claim, not a
  witness.
- The record has no content-hash field. The fields reserved for §10 are
  `emitter_kind`, `computer_id` and `subject_refs`
  (`internal/types/commitment.go:244-252`).
- The record is mutated after commit (observation, discrepancy, scores on
  the same object), so "the record's hash" is not yet well defined.
- The retention of `computer_event_append_receipts` is undeclared
  (`docs/problems/store-a-dead-wire-data-and-unbounded-history-2026-10-09.md`,
  item 4). A history rebuild must not drop the witnesses a track record
  depends on.

## 2. What "real per-computer keys" would require

| Requirement | Exists? | What blocks it |
|---|---|---|
| 1. A key whose scope is the computer, not the realization | No. Guest keys are per-realization by inventory; a fresh disk silently re-mints | **Design decision**: (a) one computer key, escrowed and delivered to each new realization like the privacy key (SH slice 3 pattern, landed and lose-the-disk proven); or (b) a long-lived computer key that certifies short-lived realization keys. Plus **missing code**: escrow and delivery for signer keys, and an O21-style rule that an existing computer never mints a replacement silently |
| 2. Generated where only the computer can use it | Partly. Generated in the guest and isolated from the runtime by systemd | **Security limit**: the host operator can read it from `data.img` or memory; no encryption at rest; runtime confinement (S1) not landed |
| 3. Bound to the ComputerID at birth | Only for verifier-control (first-use pin) | **Missing code**: a `ComputerKeyPinned` binding at genesis (key digest in `genesis_imported`, or a platform-signed pin receipt in `control_key_history`). The verifier pattern can be reused |
| 4. Published for outside verifiers | No public key-history endpoint; `/internal/platform/control-key` serves only the platform key, internally | **Missing code**: a public, Store-B-readable key history per computer with activation and invalidation points. **Decision**: whether the platform countersigns it |
| 5. Rotation and revocation | Schema and resolver yes; ceremony no | **Decision**: who may rotate (old key plus platform countersign?) and who may revoke on compromise (owner passkey plus platform?). **Missing code**: callers, `key_rotated` events |
| 6. Survives restart, update, product restore | Yes for restart and update (key on the persistent disk; product restore is a forward event and keeps the disk) | — |
| 7. Survives lost disk and host move | No | Same as row 1 |
| 8. Never duplicated by forks or snapshots | Not applicable yet (no forks; Firecracker snapshots unused) | **Deferred stations**: S9 fork re-keying (blocked on S1, S3, S8; deferred in `docs/ACTIVE.md:27`) and S3 snapshot secrecy / one-shot. Until they land, the only copies are host backups of `data.img` |
| 9. Signs the right bytes | No | **Missing code / decision**: a canonical frozen "commit body" (prediction, emitter, subject refs, kind, addressee, parent), the content hash §10 reserves, and a new allowlisted signer kind that the reducer calls at mint, never exposed to desk modules |
| 10. Verifiers know what to trust | Platform key pinned; nothing for computers | Follows from rows 3–5 |

None of these is blocked by an unknown technique. Rows 1, 4, 5 and 9 need
owner-level decisions. Rows 3, 4, 5, 7 and 9 need ordinary code reusing
patterns that already landed (verifier pin, key-history resolver, SH escrow
delivery). Row 8 waits on deferred stations, but only matters once forks or
snapshots exist. Row 2 has no software-only fix on this hosting model.
Excluding the host operator would take confidential-VM hardware (SEV-SNP or
TDX) with remote attestation, or owner-held keys. **Hypothesis:** upstream
Firecracker does not support confidential guests; I did not check Node B's
CPU.

## 3. Threat model in plain words

Three questions matter for a track record:

- **Forge:** make a record look like this computer said it.
- **Backdate:** make a prediction look older than the evidence.
- **Hide:** keep the losing predictions out of the record.

| Scheme | Who can forge | Who can backdate | Who can hide losers |
|---|---|---|---|
| **A. Today: unsigned, platform-witnessed** (record in a projection batch, platform-signed receipt with host time) | The host operator (holds the platform key; can edit Store A/B). Code inside the computer can write records as its desks, which is the computer speaking, not forgery | Only the host operator. The computer can lie in `committed_at`, but not in the receipt time | The computer, if it publishes selectively. The operator, by deleting |
| **B. Host-held key per computer** (in corpusd, signs on request from an authenticated computer) | The host operator; any corpusd compromise or authentication bug, for every computer at once | Same as A | Same as A |
| **C. Guest-held computer key** (rows 1–5 and 9 built) plus the platform witness | The computer itself (anything with root in the guest, until S1 confines it); the host operator, by reading the disk or memory deliberately; anyone holding a copy of `data.img` (backups, an un-re-keyed fork) | Only the host operator, as in A. The key adds nothing on time | Same as A |
| **C + external anchoring** (platform heads hashed into a public, third-party timestamp on a schedule) | Same as C | Nobody, for anything before the last anchor. The operator could only fabricate records newer than the last anchor | Same as A, unless commitments are public (next row) |
| **C + anchoring + public commitments** (each published prediction's hash is public when made: commit-reveal) | Same as C | Nobody, before the last anchor | Nobody. Unrevealed commitments still count, as unrevealed |

What this table shows:

- **B is A with a different name on the key.** Today's platform signature
  already says "corpusd attests that computer C submitted this". A
  host-held per-computer key says the same thing, but looks like the
  computer's own signature. Later readers would have to know to discount it,
  which is the mislabeled-authority hazard. It also concentrates every
  computer's forging power in one process.
- **C protects authorship, not time.** Its real value is that a corpusd
  compromise alone can no longer forge a computer's records, and that
  records stay attributable when read away from Choir. It does not stop
  the operator, and it does not prove time.
- **For a track record, the two levers that matter most are not keys.**
  External anchoring stops backdating, including by us. Public
  commitments stop selective publication. Both are cheap compared with
  computer keys.

## 4. Recommended signing plan

### Now (inside P1–P4; no new key)

1. **Define the commit body and hash it.** Fix a canonical "commit body"
   for `CommitmentRecord`: the fields frozen at commit (schema, kind,
   prediction, directive, addressee, parent, emitter kind, computer id,
   subject refs, agent and model). Add `commit_hash` = sha256 of its
   canonical JSON (`computerevent.CanonicalJSON` already exists). Later
   mutations (observation, discrepancy, scores) never change it. This is
   the content hash §10 already reserves.
2. **Record the witness.** Store on the record, or in an index, the
   `event_digest`, `sequence` and receipt id of the first event-head
   receipt whose batch carried the record. Treat `committed_at` as
   advisory. The trusted commit time is the receipt's `issued_at`.
3. **Label records honestly.** Set `integrity: "platform-witnessed/v0"` and
   `signature: none` on every record. Do not mint any per-computer key on
   the host.
4. **Keep the witnesses.** Declare retention for the receipts and pinned
   batches of records that may become public. This is input to the open
   Store A history decision.

### Before records count as a public track record (Autopaper W3)

5. **Publish commitments when they are made.** When an edition publishes,
   corpusd writes each record's `commit_hash` to Store B with a
   platform-signed publication receipt that names the first-witness event.
   Private editions publish only the hash and the receipt at commit time,
   and reveal later (commit and reveal, §10).
6. **Anchor heads externally.** On a schedule, publish a digest of the
   platform's event heads and Store B heads to a third-party timestamp,
   such as a public transparency log, a public git repository, or an
   RFC 3161 or blockchain-anchored timestamp. Which service is a decision
   for the owner.
7. **Give the platform key real custody.** Provision and back up the
   platform key. Write the "reviewed overlap policy" for rotating it that
   `appender.go:1091-1094` refers to. Under this plan the platform key is
   the time witness for every track record, so its custody matters more
   than a computer key's.

### When real computer keys land (a station of its own, after S1)

8. **K1 (key scope).** Decide the scope. Recommendation: option (a), one
   computer identity key distinct from the realization's receipt signers.
   Generate it in the guest at genesis. Bind its digest into
   `genesis_imported`, or pin it with a platform-signed receipt in
   `control_key_history` (reuse the verifier pin). Escrow it and deliver it
   to new realizations through the SH path. Never re-mint it for an
   existing computer. The same decision should settle the verifier-key
   re-realization hypothesis in §1.3.
9. **K2 (publication).** Serve a public key history per computer from
   Store B, with activation and invalidation sequence and time.
10. **K3 (signing).** Add a new signer kind that the reducer calls at mint
    to sign `commit_hash`. The signature lives beside the record. The
    platform witness continues unchanged. Integrity becomes
    `computer-signed+platform-witnessed/v1`.
11. **K4 (rotation and revocation).** Use the existing columns.
    Recommendation: rotation is signed by the old key and countersigned by
    the platform; revocation after compromise is authorized by the owner's
    passkey and the platform.
12. **K5 (forks and snapshots).** S9 fork re-keying and S3 one-shot
    snapshots must land before any fork or memory snapshot of a keyed
    computer exists.

### How records under each scheme stay verifiable

- **A v0 record** (platform-witnessed). Verify it in five steps:
  1. Recompute `commit_hash` from the commit body.
  2. Find the body in the pinned batch.
  3. Check the batch sha256 against the event's `PayloadCommitment`.
  4. Check the event digest against the `EventHeadReceipt`.
  5. Verify the receipt with the platform key in force at that sequence
     and time, using the repo-pinned key and, later, the platform key
     history.

  With anchoring, also check that the receipt's head is covered by an
  anchor older than the evidence.
- **A v1 record** verifies the same way, and additionally checks the
  computer signature against the computer key active at that sequence,
  from the published key history.
- **Never re-sign v0 records with a new key.** Re-signing an old record
  would create a backdated signature by construction. If continuity is
  wanted, the computer can sign one "key adoption" statement at activation
  that names its current event head. That statement, dated at activation,
  vouches for the chain up to that point. It does not turn older records
  into signed ones.

## 5. What this means for §9 item 8

The recommendation stands with one sharpening. "Unsigned" should read
"platform-witnessed, not computer-signed". The witness is real today. A
host-held per-computer key should not be used, because it would add a name
and no protection. Real computer keys are blocked mainly by an unmade
key-scope decision and by ordinary missing code: a registry, publication,
a rotation ceremony and a frozen commit body. Fork and snapshot safety are
deferred to S9 and S3. The host operator's ability to read guest keys is
not removed by any of these.

## Open questions for the owner

1. **Platform key custody.** Is the Node B platform key backed up anywhere?
   It was auto-generated, and the code says production should provision
   one. Under this plan it becomes the time witness for every track record.
2. **Key scope.** Should a computer key be one identity that follows the
   computer to every realization (escrowed like the privacy key), or should
   each realization have its own key certified by something longer-lived?
   Recommendation: one computer identity key.
3. **The trust boundary against the operator.** Is it acceptable that
   Choir's operator can technically forge or backdate until external
   anchoring exists? If not, external anchoring moves ahead of computer
   keys, and confidential-VM hardware or owner-held keys become a
   long-horizon item.
4. **Which external timestamp to anchor to.** Options include a public
   transparency log, a public git repository, an RFC 3161 service or a
   blockchain-based timestamp. It also needs a cadence; daily would bound
   the operator's backdating window to a day.
5. **Who may revoke a computer key after compromise.** The owner alone (by
   passkey), the platform alone, or both?
6. **Retention of event-head receipts.** Should the receipts and pinned
   batches behind any record that may become public be kept indefinitely,
   given the open Store A history-rebuild decision?
