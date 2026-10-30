# Portability: off-tape state inventory and the verification design (2026-10-28)

Status 2026-10-30: Store A escrow and Dolt references are stale (escrow is in the PostgreSQL control database); the ranked blockers remain valid. Design source for the new data system: [the data system frame](../design/choir-data-system-frame-2026-10-30.md).

Status: evidence for docs/definitions/choir-data-architecture-selection-2026-10-28.md.
Owner, 2026-10-28: "one of the major proof points is that we could easily
recover and restore a user's VM on a remote machine... from our NixOS host to
running on a Mac to running on a different NixOS host, to running on an
arbitrary Linux desktop or Windows eventually. If we can prove that
portability and then also make it fast, that is going to really be what makes
the architecture successful."

Delegated, read-only:

- Devin inventoried every piece of state a computer depends on that the tape
  does not hold.
- DeepSeek designed the verification and listed the triggers that would force
  a re-architecture.

Raw outputs, with all file:line citations, are in
`.agentic-consensus/data-arch/portability/out-{devin,deepseek}/` (gitignored).
FP0 has not re-verified every citation. The ranked blockers below cite the
code the inventory read.

## Devin: what is not on the tape (code-read)

Verdict: the restore model is "close but not yet honest". One item can cause
permanent data loss today.

### Ranked blockers to the portability proof

1. **The privacy key (DEK) has only one copy.** It lives on data.img at
   `/mnt/persistent/choir-credentials/privacy-key`. Its escrow upload to
   Store A is lazy and best-effort (internal/autoputer/key_escrow.go:20-23,61-79).
   If data.img is lost before one successful upload, every private tape
   artifact is unreadable forever. The owner already ratified making the
   upload guaranteed (O21,
   docs/problems/fresh-realization-missing-privacy-key-blind-boot-2026-10-08.md),
   but that has not landed. The host's escrow private key
   (`platform-artifacts/key-escrow-key`) is the second single point: lose it
   and no computer's escrowed key can be opened. This is the vault's first
   job.
2. **The files tree is a second home.**
   - `file_root_committed` syncs at most every 15 minutes, so file writes in
     between can be lost. `HydrateIfNeeded` fills only an empty tree, and no
     reducer restores the tree (internal/autoputer/file_sync.go:27,182).
   - The tree also carries the model policy (`System/model-policy.toml`,
     where the "owner-edited" bit is a comment line).
   - Prompt overrides (`/mnt/persistent/prompts`) and image blobs
     (`state.image-blobs`) are not synced at all.
   - A restore therefore silently changes desk behaviour.
3. **Nothing defines the portable unit.** Tape bodies, file CAS chunks,
   escrow wraps, signing keys and Store A rows sit in different host
   directories and trust domains, and nothing lists "what moves with
   computer C".
4. **`agent_evidence` and `co_super_slots`** are written by direct SQL, not
   through the tape.
5. **`computer_event_heads`, replay watermarks and projection bases** are
   restore authorities and accelerators in Store A. Without them a restore
   replays from genesis (about 36 hours for the paper).
6. **`state.apply-attempts` is not on the tape.** A restored computer
   forgets the crash-loop bound on self-development apply.
7. **actor.db (`state-actor.db`) has a gap.** It is mostly derivable (wakes
   re-mint from the outbox projection), but a recorded bug strands a wake
   when actor.db is wiped while a Dolt row is still marked projected. Its
   snapshots and poison counters are volatile.
8. **Mail lives on the host.** maild keeps per-owner `mail.db` and
   attachments: owner data outside the computer.
9. **Host assumptions limit where a restore can land:**
   - every service is reached by a loopback DSN, with a token valid only on
     loopback;
   - FICLONE/reflink and same-filesystem renames require btrfs;
   - ext4 and debugfs recovery tools;
   - the guest image is x86-only.
10. **The corpus (Store B)** is a dependency for any destination that lacks
    it.

Also found:

- Dead but still armed DDL: `events`, `wire_publish_debounce_*`, and the
  retired texture tables. Any stray row makes a computer checkpoint-ineligible,
  so delete the DDL.
- The computer's signer keys, gateway token and credential envelope are
  re-minted on restore, which is correct by design.
- Unverified: whether a re-mint is recorded in `control_key_history`.

## DeepSeek: the verification design

**Primary gate.** Restore a checkpoint taken on Node B onto a second NixOS
host **with the source host powered off**. Use only tape + CAS + authority
snapshot + release identity. Then repeat on a Mac, a Linux desktop and later
Windows. The strongest single kill: if that restore needs one byte from the
source host, the claim "the tape is the only durable truth" is not yet
implemented. That reopens the truth model, not the engine.

**Three corrections:**

- Release identity is per system: flake ref, source revision, per-system
  output path, and guest image digest.
- "Restore" means re-realization plus re-derivation, not process migration.
- The checkpoint format needs its own version and conformance suite. Today's
  hibernate metadata binds host ports, inodes, the Firecracker version and the
  CPU fingerprint (internal/vmmanager/snapshot.go:174-218). That is a fine
  local cache and a fatal portable unit.

**Unit under test: a ComputerCheckpoint with five parts:**

- the tape prefix;
- the CAS closure;
- the authority snapshot with per-table roots;
- the release identity;
- a signed checkpoint receipt.

It excludes all hypervisor, host-path, inode, MAC and CPU state.

**Isolation is enforced, not asserted:**

- the source is powered off;
- the bytes the destination reads are attributed at the network edge, by
  class;
- the bundle is scanned for host taint (hostnames, IPs, paths, machine-id,
  inode pairs; Fly hit this class of bug);
- the destination has no route to host-local services.

**Equivalence checks E1-E9:**

| Check | Covers |
|---|---|
| E1 | tape |
| E2 | the witness root and per-table roots, with manifest classes enforced |
| E3 | Texture revision chains |
| E4 | files, against a non-empty tree |
| E5 | the CAS closure, including after GC |
| E6 | behaviour: replay recorded prompts with the network off; event bodies byte-identical |
| E7 | obligation fates: a restore neither closes live work as a crash nor resumes twice |
| E8 | identity: the lease epoch advances exactly one; a stale-epoch append is refused |
| E9 | negative controls that must fail: a missing CAS object, a flipped bit, the wrong key, the wrong computer, an old release meeting a new event kind, a tampered signature |

A witness root means something only together with (tape head, projector
version).

**Speed is measured on four clocks:**

- **T_boot:** to agent health;
- **T_usable:** to the first owner-visible read;
- **T_hydrated:** to the hot set being local;
- **T_witness:** to verification passing.

Bytes moved are recorded by class. Expected d@e-class ranges are hypotheses:

| Clock | Second NixOS host | Mac |
|---|---|---|
| T_usable p50 | at most 5 s | at most 15 s |
| T_hydrated | at most 60 s | at most 3 min |

Serving before replay finishes must be an explicit, tested choice.

**The artifact** is `portability-receipt.json` plus a signature, under
`docs/evidence/portability/`. Rerunning it reproduces the same digests.

**What becomes continuous:**

- the differential;
- kill -9 and torn-tail tests;
- ENOSPC;
- privacy;
- memory caps;
- ring compatibility;
- obligations;
- a nightly sampled restore of one random computer to a second host, with
  1/64 of the CAS verified (restic-style).

**One-time proofs:** per destination class, and per design (crypto-erase,
the source-dead attestation).

**What it builds on:**

- scripts/off-host-restore-drill.sh;
- checkpointd's verdicts (cmd/checkpointd/verdicts.go:47-70);
- the cold_recover fencing lease (internal/vmctl/cold_recover.go:59-69).

**Re-architecture triggers.** Tuning changes a parameter; re-architecture
changes the durable unit or the number of homes or writers. Each trigger
names the part of the architecture it would reopen.

| Trigger | Threshold | Reopens |
|---|---|---|
| A1 | authority state over 1 GiB on the paper, or over 20% of bytes | the authority model |
| A2 | tail over 5 s or over 1 GiB | the checkpoint model |
| A3 | checkpoint over 200 MiB or over 30 s p95 | the checkpoint mechanism |
| A4 | any byte from the source host | the seam and host locality |
| A5 | an arm64 vs x86 root divergence | the projector, or the truth model |
| A6 | replay non-determinism | the reducer or projector |
| A7 | a witness pitfall | the witness or the engine |
| A8 | a cross-owner leak | the CAS or encryption model |
| A9 | GC deletes a referenced object | the ref model |
| A10 | memory grows with input size | that operation's design |
| A11 | a dual writer | the lease and fencing |
| A12 | an old ring cannot replay | the schema-change policy |
| A13 | Mac T_usable over 45 s p95 | the portable unit |
| A14 | long-run slope over 10% per day | caches and GC |
| A15 | a paper restore over 10 minutes | the corpus, base and compaction |

**Inconsistency in FP0's ledger.** R0b-10 (a tail under 1 GiB replaying in
under 5 s) and R0-15 (about 2,500 events per second) cannot both hold
unless the checkpoint cadence keeps the tail to about 12,000 events, or
restore uses a bulk loader. That is a design constraint to precommit
(ledger record R0c-01).
