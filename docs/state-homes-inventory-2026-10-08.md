# State Homes Inventory

Date: 2026-10-08. Status: **draft inventory** (read-only survey, green).
Enforces the proposed operational invariant **O21 — a realization holds no
unique state** in the
[operational invariants register](operational-invariants-register-2026-10-08.md).
Restates, operationally, what [computer-ontology.md](computer-ontology.md)
already says: "a realization may be replaced without changing ComputerID" and
"realization is replaceable VM/OS/runtime machine state."

## The rule

Every piece of a computer's persistent state has exactly one durable home
outside any realization. A realization (hosted microVM, or a local VM under
the desktop app) is a cache built from those homes.

| Home | Holds | Authority |
|---|---|---|
| Tape (Store A event chain via corpusd head CAS) | what happened | canonical |
| Content-addressed store (platform CAS) | bytes: projection bases, file chunks, artifacts, releases | content digest named by the tape |
| Escrow (platform key escrow) | keys, wrapped per protector | escrow record bound to ComputerID |
| Projections (VM-local embedded Dolt, derived views) | derived state | rebuildable from tape + CAS |
| Realization | caches of all of the above, plus realization-scoped identity | disposable |

Recovery, resume, host move, hosted↔desktop placement, and forks become one
operation: construct a cache from durable homes. Anything that exists only on
a realization is a bug against this rule.

## Inventory: guest persistent volume (`/mnt/persistent`)

Sources: `nix/autoputer-vm.nix` tmpfiles and service units, guest code paths
named per row. "Verified" means read in code on 2026-10-08; "unverified" means
the row's claim needs a disposable lose-the-disk test.

| Path | Contents | Durable home | Verdict |
|---|---|---|---|
| `choir-credentials/privacy-key` | per-computer DEK (`computerevent/privacy.go`) | custodian escrow wrap exists (lazy per-boot upload, `autoputer/key_escrow.go`); **no path delivers it to a new realization** | **violation** — the 2026-10-08 fresh-realization crash loop. Only restore path is debugfs copy from a quarantined old image (`vmctl/trusted_guest_copier.go`). Also: a computer whose escrow upload never succeeded has no second copy at all. |
| `capsule-artifacts/` (subjects/, receipts/) | capsule subject artifacts and receipts, named by durable refs committed to the store (`capsule/executor.go`) | refs on tape/store; **bytes only on the realization** | **violation** — after realization loss, committed refs dangle (`ErrSubjectArtifactUnavailable`). Unverified whether any path uploads them to CAS. |
| `files/` | owner files | encrypted chunks + manifest root in platform CAS, root cited on tape (`autoputer/file_sync.go`); hydrated on boot | **bounded violation** — periodic sync every 15 min (`defaultFileSyncInterval`); edits since last sync exist only on the realization. A sync barrier exists (`POST /api/files/sync`). Hydration needs the DEK, so it inherits the privacy-key violation. |
| `state/`, `state.texture/` | embedded Dolt projections, store schema | tape + projection base in CAS (checkpointd, 2026-10-08) | **OK** — derived; bounded rebuild (O6). |
| `choir-updater/` (`releases/`, `current`, `operations/`, `store/`) | installed runtime releases, current symlink, apply journals | releases content-addressed; applied release named by the applied event (ontology: "updater projects an authorized event") | **unverified** — need proof that a fresh realization re-derives `current` from the effective event head rather than booting the image baseline. In-flight `operations/` journals are realization-scoped (acceptable only if a lost apply resolves to a typed outcome, O9). |
| `choir-signers/guest-core`, `choir-signers/verifier` | ed25519 receipt-signing keys + receipt state (`receiptsigner.LoadOrCreateSigningKey`) | none — created per realization | **realization-scoped identity, probably correct** — receipts carry `signer_public_key`. Unverified: that trust in a new realization's signer is re-established by platform attestation, and that `receipts/` state holds nothing a later verification needs. |
| `gateway-token`, `computer-event-envelope` (credential disk) | per-boot credentials issued by vmctl (`vmmanager/manager.go`) | platform issues fresh per realization | **OK** — realization-scoped by design. |
| `go/pkg/mod`, `go-build-cache` | toolchain caches | none needed | **OK** — cache. |

## Host-side realization state (vmctl)

| State | Home | Verdict |
|---|---|---|
| recovery conditions (`vm-state/recovery-conditions.json`) | vmctl state dir | **OK** — re-derivable from inputs (O6 design). |
| data images, quarantined images | host disk | caches/forensics; must never be a restore source of last resort once every row above has a home. |

## Findings

1. **Privacy key** — no delivery from escrow to a new realization. Decision
   (owner, 2026-10-08): deliver the escrowed key to the same computer's new
   realization automatically, bound like the credential envelope (ComputerID
   + realization epoch, consume-once, recorded in key-escrow transparency).
   Human reveal keeps two-approval. Then delete the debugfs copier path.
   Prerequisite: escrow upload must be **guaranteed before the key becomes
   the computer's only copy**, not lazy — a computer whose escrow never
   succeeded is one disk away from permanent loss.
1a. **Escrow is overwritable** — `UpsertKeyEscrow` replaces the key digest;
   any realization of the computer can replace the only off-realization copy
   ([problem doc](problems/key-escrow-overwritable-by-guest-2026-10-08.md)).
   Staging: all 113 chained computers are escrowed, so the backfill is done;
   the guarantee left is write-once escrow plus escrow-before-genesis.
2. **Capsule artifacts** — committed refs, realization-only bytes.
   Problem doc owed.
3. **Files** — 15-minute loss window. Any planned realization change
   (handoff, restart for update, desktop move) must run the sync barrier
   first; unplanned loss is bounded at the interval.
4. **Updater `current`** and **signer trust** — unverified; settle with the
   disposable lose-the-disk proof.

## Placement: hosted ↔ desktop

Owner requirement (2026-10-08): a computer should move between its hosted
realization on choir.news and a local realization under the desktop app
(`cmd/desktop`, Wails v3; out of current scope) and back. Under O21 that is
the same operation as recovery:

- **Moving** = construct a local realization from the homes (tape, CAS,
  escrow), then switch the route. Nothing is copied VM-to-VM.
- **Single writer.** Today corpusd head CAS is the only append authority, so
  one realization appends at a time. Moving is a fenced handoff of the
  writer (realization epoch), not a merge. Two realizations writing
  divergent heads is a fork (new ComputerID, publication/adoption), never a
  sync.
- **Key custody on the desktop** is the open question: delivering the DEK to a
  machine the platform does not attest is a different trust act from hosted
  re-realization. The owner's device holding an owner-derived protector
  (passkey-bound) fits this better than custodian delivery, and is the same
  seam as the future end-to-end option.
- **Offline use** needs either a delegated head lease or a local
  pending-append queue reconciled on return — an open design question,
  recorded here, not decided.

## Proof

The O21 proof is a disposable on staging: create, use (files, capsule run,
self-development apply), destroy its data disk entirely, realize again, and
show identical effective head, content witness, files, artifacts, and
release — with no debugfs or operator step.
