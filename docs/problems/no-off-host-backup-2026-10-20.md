# Nothing durable on Node B exists anywhere else (2026-10-20)

Mutation class: green for this record. The work it leads to is red for
`nix/node-b.nix` (a backup client that reads protected surfaces: the event
tape, Store A, signing and escrow keys, auth state) and orange for the host
module handed to node-a. Status: problem recorded; design below (added in
the next commit); client and target modules written but **disabled and not
applied anywhere**.

## The problem

Node B is a single physical host. Everything Choir would have to rebuild a
computer, prove its history, or sign anything again lives on it, and no byte
of it exists on any other machine. The root filesystem is btrfs on an md
RAID1 pair (`nix/disks.nix`), which survives one disk failing. It does not
survive losing the host (hardware, provider account, fire, a bad
`nixos-rebuild`, an operator `rm`, ransomware) or a software error that
damages both mirrors at once (the 2026-10-05 artifact GC deleting live
bodies was exactly that class: `docs/problems/s0-gc-og-wrong-store-deletion-2026-10-05.md`).

The only "backups" that have ever existed were on-node, temporary, and
partial:

- Dolt copies under `/var/tmp` taken before a risky step, kept until the
  step's soak ended, then deleted (the 48 GB pre-2.4.2 copy, the 2026-09-11
  and 2026-10-03 copies; `docs/problems/guest-dolt-journal-and-host-image-leak-2026-10-03.md`).
  Same disk, same failure domain.
- `s0-gc-og-wrong-store-deletion-2026-10-05.md`: "On-node backups cover
  dolt state, not `platform-artifacts`." The file-CAS bodies it lost were
  the sole copy; nothing could restore them.

## What is at stake (all of it on Node B only)

Sizes are from the cited records, not re-measured here.

| Durable thing | Where | Why it matters | Size (record) |
|---|---|---|---|
| Event tape (computer events, payloads, pin receipts) | `platform-artifacts/sha256/{computer-event,computer-event-payload,pin-receipts}` | The canonical, signed, immutable history of every computer. Owner rule: never edit it, never rewind heads. No replica anywhere. | 19.9 GB in 9.07M files (tape-volume report) |
| Store A, platform Dolt | `/var/lib/go-choir/platform-dolt` | Control store: ownership, heads, append receipts, event index. | 1.26 GiB logical; 23 GB on disk 2026-10-14 |
| Store B, corpus Dolt | `/var/lib/go-choir/corpus-dolt` | Autopaper corpus, shared published objects. | 22 GB on disk 2026-10-14 |
| File CAS chunks and roots | `platform-artifacts/sha256/file-cas-*` | Owner files; "the CAS file was the sole copy" (S0 record). | about 48 GB |
| `og` bodies, capture bodies, proposals, platform updates | `platform-artifacts/sha256/{og,platform-update,proposals}`, `capture-bodies/` | Externalised rows hold `body = NULL`; the file is the only copy. | about 6 GB |
| Platform signing key and key-escrow key | `platform-artifacts/signing-key`, `key-escrow-key` | Every head receipt and pin receipt is signed by the first; the second unwraps escrowed owner material. Without them the tape is unverifiable going forward and escrow is lost. | bytes |
| Auth state | `/var/lib/go-choir/auth/auth.db`, `auth-signing/` | Passkey credentials and sessions. Owner ruling #13 already says recovery is a launch blocker. | MB |
| Guest data disks | `/var/lib/go-choir/vm-state/*/data.img` | Each computer's persistent state and guest Dolt. Rebuildable in part from the tape, not entirely. | 110 GB with snapshots (2026-10-14) |
| Host secrets | `/var/lib/go-choir/secrets/*`, provider and mail env files | Rotation is possible but slow; the owner-act key and Dolt passwords are not. | KB |

Not at stake, because regenerable: `projection-base` (47 GB, rebuilt from the
tape), `checkpoint-scratch`, `host-build`, the Nix store, guest images.

## Why now

- **L3 is gated on it.** The ledger metamission's entry criterion is "a
  recoverable off-host copy of the ledger" (`choir-ledger-metamission-2026-10-13.md:137-138`);
  the cost-track report says SP0 therefore gates L3.
- **Tape surgery needs an archive before any delete.** The tape-volume
  report (section 6.1 step 5, 6.3, 6.4) makes the delete steps black unless a
  verified off-node archive exists, and asks the owner to "name the archive
  target". There is no target.
- **Growth.** The tape adds about 1.5M small files a day; the host is at
  about 80% of 476 GB. Disk pressure is the likeliest thing to force a
  hurried delete, and today a hurried delete has no undo.
- The 10-17 report puts it plainly: "Until then, a total loss of Node B
  would lose history that exists nowhere else."

## The ruling

Owner, 2026-10-14, `docs/reports/owner-notes-2026-10-14.md` section 27
item 17:

> **Off-host backup: node-a now, as a stopgap.** Encrypted (key only on
> Node B and the owner's offline copy), quota'd subvolume, node-a holds
> only ciphertext. "True durable offsite backup requires more infra which
> we will get later." Node-a is also the tape cleanup's archive target.

Constraints that follow, from `docs/node-a-shared-host.md` and the
roadmap's "Node-a stopgap" section:

- node-a runs another project and its administrators have root. Only
  ciphertext may reach it; the key never does.
- Choir does not touch node-a. Its own agent applies a NixOS module that we
  hand over.
- It is a stopgap: one extra host, shared, not a true offsite copy. A
  compromise or loss of Node B and node-a together, or of the owner's
  offline key copy plus Node B, still loses everything.

## Evidence this is worth doing the careful way

- Restic, rest-server and the Dolt backup procedure were checked as far as
  possible without touching any host (see the design's "What was verified").
- The app Dolt user cannot run `DOLT_BACKUP` (`command denied`); root can.
  Checked locally against Dolt 2.1.9. This shapes the client's credentials.

## Heresy delta

- discovered: backups exist only in the same failure domain as the data; the
  S0 record named the gap on 2026-10-05 and nothing closed it for 15 days.
- introduced: none (nothing is enabled).
- repaired: none yet. The repair is the enabled, drilled client and an
  applied target; this record closes only when a restore drill from node-a
  passes.
