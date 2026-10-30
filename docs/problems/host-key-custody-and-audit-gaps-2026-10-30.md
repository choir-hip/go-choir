# Host key custody and audit gaps (2026-10-30)

Class: green (problem record). Surfaces: key custody, auth, Trace/evidence
(red). Found by the restore and audit design pass
(`docs/design/computer-history-restore-audit-2026-10-30.md` §1.2 G3-G9, G11);
companion to `host-holds-user-state-in-plaintext-2026-10-30.md`.

## Evidence (as cited by the design pass; [V] there unless marked)

- G3: each computer's privacy DEK is a plaintext file on its data image
  (`/mnt/persistent/choir-credentials/privacy-key`, recorded in
  `docs/design/secrets-vault-design-2026-10-28.md` table row 1), and the data
  image and hibernate memory files live on the host, so even payloads sealed
  "private" are readable from host disks.
- G4: checkpointd unwraps every computer's DEK as a routine step of base
  rebuilds (`worker.go:181-205`).
- G5: four host readers depend on plaintext payloads (ledger witness,
  checkpointd's rebuilder, base capture, the checkpoint authority's decision
  read).
- G6: audit is spread across at least six logs; the one hash chain is
  unsigned, unanchored, internal-only and rewritable with Store A write; the
  escrow reveal returns a raw DEK to a human; operators are shared tokens; no
  owner is notified of an unwrap.
- G7: owner-act enforcement runs in `observe` mode in production.
- G8: no legal hold, tenant retention or export; receipts grow without bound;
  the platform signing key has no rotation caller.
- G9: checkpoint cadence is enabled for one computer and the test class;
  most computers have no restore points beyond the latest two bases.
- G11: event pins are labelled `private` but stored as plaintext
  (`internal/platform/event_artifacts.go:46`); capsule effect bundles are
  pinned `public` and carry workspace file paths, modes and refs
  (`capsule/transaction/builder.go:26-66`).

## Belief state

- Taken together, the host operator (and anyone with host disk or backup
  access) can read every user computer's data today, and most such reads
  leave no owner-visible record. This is a pre-launch posture, not a
  launchable one.
- G7 is the cheapest to change and is flagged for the owner separately.

## Next

- The restore and audit design addresses each item; the build follows its
  review. No fix lands before this record.
