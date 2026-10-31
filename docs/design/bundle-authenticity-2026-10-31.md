# Bundle authenticity (2026-10-31)

Status: built on `lane/bundle-authenticity`, not on main. Class red
(evidence and import trust). Part of moving a computer from Node B to the
owner's Mac and back (owner approved 2026-10-31; one-tape-durability §4a, §6a2
B2).

A computer bundle (internal/bundle, cmd/choir-bundle) has a snapshot root,
its chunks, and the tape from the snapshot's head to the current head:
events, payloads, pin receipts and event head receipts. Before this change,
verify checked that digests are self-consistent and that the chain links. It
did not check who signed the receipts. A bundle that an attacker had put
together with consistent digests verified and imported. Import also accepted
any snapshot root the manifest declared. An imported root with a high
`head_sequence` would then be the one `RecoveryBase` prefers.

## 1. What was checked first

| Claimed gap | Found |
|---|---|
| Verify does not check receipt signatures | True. `Verify` parsed head and pin receipts but never called `Receipt.Verify`. |
| Import accepts a snapshot root the tape does not name | True. Only its digests and chunk files were checked. |
| Genesis and identity binding | Partial. A bundle from genesis was not required to start with `genesis_imported`, and the manifest carried no genesis. A same-id computer with another genesis was refused only as a fork. |
| Key ids, rotation, `key_revoked` | `key_revoked` events revoke a guest credential capability, not a receipt signer (`internal/selfdev/credentials.go`, reducer epoch). Receipt signers rotate through `control_key_history` (platform domain), and Node B holds no `platform-control` rows there (27 `verifier-control` rows only, read 2026-10-31). Node B signs every receipt sampled with the repo-pinned key `868f96cca8726f99` (`computerevent.PlatformControlTrustKeyID`). |
| Trust roots | Not defined for bundles. Nothing said which keys an importing host trusts. |
| Incremental overlap | Only event digests in the overlap were compared, through an in-memory map of the whole overlap (unbounded). |
| Compact receipt codec (`lane/receipt-trim`, d68fb3d87) | Landed on main during this work. Its bundle change is in `readReceipts`, which is the only read of the receipt column in this package, so export and the overlap compare both see expanded bytes (drill B6). Separately, **Node B's stored `event_head_receipt_json` is not the canonical bytes**: the rows read back as `{"issuer": "corpusd", ...}`, with spaces and in another key order. Anything that compares stored receipt bytes byte for byte fails on real data. Verification and overlap comparison must work over the canonical re-encoding. |
| Memory | The read path loaded the pack index into string-keyed maps (a few hundred bytes per object), and the seen sets were maps of digest strings. On the paper's 6.6M events (about 20M objects) that is several GB. |

## 2. Failure list (each attack or accident, and what refuses it)

Verify-side failures (A) are tested in `internal/bundle/authenticity_test.go`
on a synthetic, production-shaped chain (`synth_test.go`). Import-side
failures (B, and I/D from the earlier drill) are tested in
`internal/platform/bundle_authenticity_drill_test.go` and
`bundle_drill_test.go` against PostgreSQL.

| # | Attack or accident | Refused by | Test |
|---|---|---|---|
| A1 | Forged signature (signature bytes altered) | `Receipt.Verify` over the canonical payload, on the signature workers | `TestBundleAuthenticity/A1` |
| A2 | Well-formed bundle signed by a key the importer does not trust (an attacker's host, an unenrolled host) | `TrustSet.ResolveReceiptKey`: unknown key id, `ErrUntrustedKey` | `A2`, and `TestBundleImportAuthenticity` B1 (nothing written) |
| A3 | Receipts name a trusted key id but are signed by another key; a trust entry that rebinds a key id to another key | Signature check; at load, the trust entry's `key_id` must be `sha256(public)[:16]` | `A3` |
| A4 | Retired key used at or after its cutoff (sequence or time); pin receipts by the old key after the cutoff | Trust entry cutoffs. Pin receipts are checked at their event's sequence. A sequence cutoff needs a computer id (sequences are per computer). | `A4` (planned rotation old to new still verifies) |
| A5 | Snapshot root the tape does not name; a root swapped into a named bundle; a root claimed at a head after its naming event | `snapshot_recorded` event in the range with idempotency key `snapshot:<root>` and sequence above the root's `head_sequence`. Export picks only named roots and refuses an unnamed `--root`. | `A5`, B2 |
| A6 | Truncated chain: receipts end early, or one is missing in the middle | Sequence continuity, and tape end equal to the manifest head | `A6`, E5 |
| A7 | Spliced foreign chain: another computer's receipts after a prefix, or a fork of the same computer spliced in | `computer_id` on every receipt, event and head receipt; `previous_head` chaining | `A7` |
| A8 | Genesis mismatch: manifest genesis is not event 1; event 1 is not `genesis_imported`; a second genesis; a target with the same id and another genesis | Verify (range from genesis); import `checkGenesis` | `A8`, B3 |
| A9 | Tampered payload with every digest recomputed (payload, artifact ref, event, pins, head receipts), re-signed by an untrusted key or under a trusted key id | Signature and trust | `A9` |
| A10 | Head receipt field rewritten and its canonical payload digest recomputed (for example a head projection) | Signature over the recomputed payload fails | `A10` |
| A11 | Empty or nil trust set | Refused: nothing can be authenticated | `A11` |
| A12 | (accident) Head receipt stored in another JSON spelling (Node B) | Must **pass**: signatures are checked over the canonical bytes | `A12` |
| A13 | Head row fields the receipts do not sign (reducer version, credential revocation epoch) | Reducer version equals the last event's. For a range from genesis the epoch equals the number of `key_revoked` events, otherwise it is at least that number. | `TestBundleVerifyCounts` |
| B1 | Import of an unauthentic bundle writes files or rows before refusing | Import runs `VerifyBundle` first and returns `ErrRefused` wrapping `ErrUnauthentic` | B1 (file count and head unchanged) |
| B4 | Incremental overlap mismatch: the target forked after the shared head and the source's next bundle overlaps the fork | Streaming overlap compare in `(start, min(target head, bundle head)]` | B4 |
| B5 | Overlap receipt differs byte for byte (another receipt id for the same event); and the accident of another spelling | `sameReceipt`: all columns plus the canonical head receipt bytes. A respelled receipt imports. | B5 |
| B6 | receipts.jsonl swapped between verify and import | Import hashes the file on its file pass and its row pass. Rows commit only if the hash equals the hash verify saw. | Mechanism only (see residual R5) |
| B6c | (accident) Source or target stores compact receipts | Must **pass**: export expands them, and the overlap compare expands the target's | `TestBundleCompactReceipts` |
| B7 | Pack objects that no receipt names, written into the target's tape store | Import writes only the objects the verified receipts reference | Drill counts (I1/I3) |
| B8 | Empty range onto an empty target: nothing signed authenticates the head row | Refused | Code path, `planImport` |
| I4/D3 | Fork at the start, gap, delta onto an empty target, delta onto a fork | Existing refusals, kept | `TestBundleRestoreDrill`, `TestBundleDelta` |

**Principle.** Nothing in `manifest.json` is signed. A manifest field
counts only once the signed tape confirms it: the head row against the last
head receipt, the start head against the first receipt's `previous_head`,
the snapshot root against its naming event. An unconfirmed field (`genesis`
on a range that does not start at genesis, `owner_id`, `producer`) may only
cause a refusal, never an acceptance.

**Authenticity is transitive.** A signed head receipt binds the event
digest and the pin receipt digests. The event binds its payloads by artifact
ref digest. Sha256 then authenticates every event, payload and pin receipt.
Pin receipt signatures are checked as well, as defense in depth: a trusted
head receipt that names a pin receipt from an untrusted signer means
something is wrong upstream.

## 3. Trust roots

An importing host trusts a set of `platform-control` public keys. **The set
never comes from the bundle.** A bundle that carried its own keys would
authenticate itself. The set is the union of:

1. the repository-pinned staging key (`computerevent.PlatformControlTrustKeyID`
   and `PlatformControlTrustPublicKey`, `868f96cca8726f99`), which is Node B's
   signer and is already the reviewed staging trust root. It is on by
   default; `--no-pinned-trust` drops it.
2. each `--trust <file>` the operator names: JSON `{"keys": [{"signer_domain",
   "key_id", "public_key", "label", "computer_id"?, "first_invalid_sequence"?,
   "first_invalid_time"?}]}`. Unknown fields are refused.

Provisioning. Run `choir-bundle trust-key` on the exporting host. It reads
the platform signing key file (never creates it) and prints a trust file
with only the public key. Copy that file to the importing host over an
authenticated channel (ssh, or the repo). For Node B to the Mac, the pinned
key already covers Node B. For the Mac back to Node B, the Mac's `trust-key`
output goes into Node B's trust file. A computer that moved and appended on
the Mac has a chain signed by Node B's key up to the move and by the Mac's
key after it. That bundle verifies on a host that trusts both keys, and on
no host that trusts only one.

Key id binding. `key_id = hex(sha256(public_key))[:16]`, as `newSigningKey`
derives it. An entry whose id does not match its key is refused at load, so
an id cannot be rebound.

Rotation and revocation. A **retired** key (planned rotation) keeps its
entry with a cutoff: `first_invalid_sequence` (with `computer_id`) or
`first_invalid_time`. Receipts at or after the cutoff are refused, and
receipts before it still verify, so history signed by the old key stays
importable. A **compromised** key is removed from the set entirely. Whoever
holds it can sign a forged chain with any sequence and any `issued_at`, so
no cutoff contains it. The time cutoff is only as good as `issued_at`, which
the signer chooses. `key_revoked` tape events are guest credential
revocations and do not affect receipt trust. They are counted only to bind
the head row's epoch.

## 4. What changed

- `internal/bundle/trust.go`: `TrustSet`, `TrustedKey`, `TrustFile`,
  `PinnedTrust`, `TrustEntry`, `KeyIDFor`; a resolver that checks domain,
  scope and cutoffs.
- `verify.go`: `VerifyBundle(dir, trust)` returns `Verified` (the manifest,
  the receipts.jsonl digest, the signature count, and the sequence of the
  event that names the snapshot). Every failure wraps `ErrUnauthentic`.
  The scan is a pipeline (`scan.go`). A reader cuts receipts.jsonl into
  batches of 256 rows. `GOMAXPROCS` workers check each row on its own:
  event decode and binding, head and pin receipt parse and binding, payload
  hashes, and every signature. One consumer, in sequence order, checks the
  rest: continuity, the previous-head chain, genesis, the snapshot's naming
  event, and the unique-object counts. At most 2 x workers batches are in
  flight, and the failure reported is the first in tape order. The strict
  field set of the head receipt is checked (12 kind fields). The head row's
  reducer version and epoch are checked too.
- `packindex.go`, `mmap_*.go`: the read path's pack index is a sorted slice
  of 48-byte slots with binary search, and the pack is mapped read-only (a
  pread per object costs a syscall per event, payload and pin). The seen
  sets are one bit per pack slot.
- `import.go`: `Import(ctx, dir, db, root, trust)` runs `VerifyBundle` first.
  `checkGenesis`. A streaming overlap compare (one page of target rows at a
  time), byte for byte over the canonical receipt. It tolerates a target
  that was itself imported from a later snapshot: the overlap is what the
  target holds. Files are written only for referenced objects. The
  receipts.jsonl hash is rechecked before the rows commit.
- `export.go`, `store.go`: the default root is the newest **named** root. An
  unnamed `--root` is refused. The manifest carries `genesis`.
- `cmd/choir-bundle`: `--trust` (repeatable) and `--no-pinned-trust` on
  `verify` and `import`, and a new `trust-key` subcommand.

## 5. Cost of verify (per million receipts)

Measured by `TestBundleVerifyCost` (`CHOIR_BUNDLE_MEASURE_EVENTS=N go test
./internal/bundle -run TestBundleVerifyCost -v`). The chain is synthetic:
every third event carries a payload, one in a thousand is a key revocation,
and each receipt has about 3.3 objects and 2.7 signatures. The measured
numbers are in §5a. Per receipt, verify does a fixed amount of work:

- read and hash the event;
- decode it and re-encode it canonically;
- parse the head receipt and its pin receipts, and hash them;
- check 1 + (pins first named by the event) signatures, each with its
  canonical re-encode and an ed25519 verify.

Memory is the pack index, 48 bytes an object, plus a seen bit per object and
a bounded job queue. It does not grow with anything else: the scan holds one
receipt line at a time.

### 5a. Measured

Agents VM (`choir-agents`: 8 vCPU, 9.7 GB, Linux, go1.26.10), 2026-10-31.
The synthetic chain has 1,000,000 receipts, 3,331,998 objects and 2,665,999
signatures; the bundle is 4,989 MiB:

| Workers | Verify wall | Per million receipts | Signatures/s | Peak heap in use above baseline |
|---|---|---|---|---|
| 1 | 6 min 43 s | 403 s | 6,617 | 679 MiB |
| 8 | 1 min 21 s | 81 s | 32,899 | 679 MiB |

At 20,000 receipts the numbers are the same per receipt: 433 s per million on
1 worker, 94 s on 8. Cost is linear in receipts.

CPU. About 400 CPU-seconds per million receipts, roughly 0.4 ms per
receipt. A little under half of it is ed25519 verify (2.67 signatures per
receipt). The rest is canonical JSON re-encoding (events, head receipts, pin
receipts) and sha256. The scan allocates about 156 KB per receipt, almost
all of it in `computerevent.CanonicalJSON` and `Receipt.UnmarshalJSON`, and
all of it is short-lived. Wall time scales with cores: 5x on 8 workers.

Memory. Live memory is the pack index plus the seen bitsets:
48 B x 3.33 objects, about 160 MB per million receipts. Peak heap in use is
679 MiB because of GC headroom over the allocation churn (GOGC=100). The
index does not depend on payload size, only on object count.

The paper (6.6M events, about 22M objects): about 9 min on 8 cores, about
44 min on one, with about 1.05 GB of live index. Set `GOMEMLIMIT` (for
example 2GiB) when verifying the paper on Node B, which keeps the peak near
the live set. Heavy runs belong on the agents VM or the Mac, not Node B.

The earlier map-based read path (string-keyed index entries and seen sets of
digest strings) would have held several GB live on the paper.

On the measuring Mac (inside the agent sandbox), each pread and pwrite cost
about 1 ms. Hence the read-only mmap of the pack (§4). Numbers taken in that
sandbox are not representative.

## 6. Residual risks and open items

- R1 Chunk list. The snapshot root is bound to the tape, but its chunk list
  is not: the root is the guest's filecas manifest root, and the manifest
  sits inside the encrypted `snapshot_recorded` payload. A tampered chunk
  list can only drop or add content-addressed chunks. The guest's restore
  authenticates every chunk it reads against the decrypted record. A missing
  chunk is a restore failure, not a forgery. Binding the list host-side
  needs the record's plaintext root (or a host-signed root receipt).
- R2 Snapshot head. A manifest may lower a root's `head_sequence`, which
  only lengthens the replay. It cannot raise it past the naming event.
- R3 Freshness. Verify proves a bundle is an authentic prefix. It does not
  prove the bundle is the latest. A stale but authentic bundle imports onto
  an empty target. Onto a target that is ahead, it is a no-op. Freshness
  needs the writer epoch (one-tape-durability §4).
- R4 Time cutoffs trust `issued_at`. Use sequence cutoffs, or removal for
  compromise.
- R5 Closed (2026-10-31). Every pass of import (the overlap plan, the files,
  the rows) reads receipts.jsonl to EOF and refuses unless its sha256 is the
  digest verify saw. `bundle.ImportHooks` swaps the file between verify and
  the plan in `TestBundleImportRepairs`. `created_at` is written from the
  signed `issued_at`, never from the bundle.
- R6 The pack is memory-mapped. A pack truncated by another process while
  it is mapped raises SIGBUS. Bundles are read-only once their manifest is
  written. Do not import from a directory an export is still writing.
- R7 Linear memory. Memory is 48 bytes an object, not constant: about
  1.05 GB live on the paper's roughly 22M objects, and twice that at peak
  without GOMEMLIMIT. A constant-memory verify needs a pack in tape
  order (a format 3 that export rewrites in first-reference order), so verify
  can stream it beside receipts.jsonl.
- R8 Export memory. The pack writer still keeps string maps, and export
  issues one pwrite per object. This was not touched (out of scope); for the
  paper it is the next memory item.
- R9 Import transaction size (one-tape-durability §6a2 B4) is unchanged.
- R10 Closed. `lane/receipt-trim` landed on main first, and this lane is
  rebased onto it. `TestBundleCompactReceipts` compacts the source's and the
  target's receipts, then exports, verifies, imports and runs an
  incremental overlap. Everything works over the expanded canonical bytes.
- R11 `control_key_history` rows of the target host are not consulted. Node B
  has no `platform-control` rows, so trust is only the pinned key plus the
  trust files.
