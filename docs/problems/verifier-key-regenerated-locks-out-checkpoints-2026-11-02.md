# A lost data disk regenerates the verifier key and locks the computer out of every verifier checkpoint (2026-11-02)

Status: documented; fix in flight on branch `verifier-key-repin`. Class:
red (signing keys, checkpoint authority, self-development apply path).
Review tier 2.

Owner rule (2026-11-02): "upgrading things should never leave a computer in
a stuck state."

## Evidence

- On d@e (computer-59c902c5d46caecc0a180a8557164e72, vm-70070cf7…),
  owner-approved selfdev-534017c7… burned all 3 guest apply attempts in
  under a minute at 23:02Z. Every attempt failed with
  `guest credential: checkpoint refused with status 400: checkpoint
  authority: verifier key is not the pinned computer key`; the operation was
  then held "apply attempts spent". FP0 abandoned the held op at 23:1xZ; it
  settled forward to `applied` because the updater's current pointer equals
  the target.
- Postgres `control` on Node B: d@e has one active `verifier-control` row,
  `verifier-control-22033e8312994d59`, activated 2026-10-26 21:28:39Z, and a
  genesis receipt. The refusal is the mismatch branch of
  `internal/platform/checkpoints.go:141-142`, not the first-use branch.
- d@e's verifier-class checkpoints carry `verifier-control-22033e8312994d59`
  every 15 minutes up to 2026-10-30 16:21:46Z, and none after. Every
  checkpoint since is owner-recovery class, which skips the key check, so
  nothing noticed the change.
- d@e's `data.img` was born 2026-10-30 21:43:16Z (`stat %w` on Node B), the
  owner start of lost-disk drill H in the data-system ledger
  (docs/definitions/choir-data-system-build-2026-10-30.ledger.md, line 224:
  "disk killed 21:42:57; owner start 21:43:13"). Drills E-H (18:59, 19:48,
  20:26, 21:42) each renamed `data.img` aside and booted an empty one.
- b@c (computer-6c50d6e566143ae32cdb660faa60e24d) applied fine at 22:17Z;
  its verifier checkpoint carries its pinned key
  `verifier-control-fda688cd788d1aa7`, and its `data.img` (born 10-24
  03:53Z) predates its pin (10-24 04:27Z).

## Cause (confirmed by the record)

`/mnt/persistent/choir-signers/verifier/key.ed25519` lives on the data disk.
A lost-disk boot restores the store key from custodian escrow, the
snapshot, the tape tail and the file tree, and nothing restores the signer
directory (`internal/vmmanager/data_image_template.go:77` lists
`choir-signers` as per-computer material no image may carry; the lost-disk
path stages an empty image). At the next boot
`internal/receiptsigner/signing.go:27-31` finds no key file and silently
generates a new one. The host still pins the old key, so every later
verifier-class checkpoint is refused at `checkpoints.go:141`.

The same holds for any path that gives a computer an empty data disk:
the lost-disk boot, cold recover (`vmctl/cold_recover.go:554`,
`StageSparseImage`), and a hand rename. It is not the cutover data migration
(it migrated d@e's store in place at first boot) and not the store split
(it moves stores inside `/mnt/persistent`). The guest-core signer key
regenerates the same way; it is pinned only for standing-consensus ballots
(`internal/platform/standing_consensus.go:258`), and no running computer has
a guest-core pin today.

## Why it wedges

1. The guest-side apply attempt is spent at `markPlannedRestart`
   (`internal/agentcore/self_development_materializer.go:1461`), before the
   checkpoint step, so a refusal that will repeat every time burns the
   bound. The resume after the apply's restart re-enters `applyOrReadJournal`
   on every retry wake and spends again.
2. No owner act exists to re-pin a verifier key. `revokeControlKey` and
   `activateControlKey` (`internal/platform/control_keys.go:87-113`) have
   test callers only.
3. Nothing reports the mismatch before an apply: `/health` says nothing, and
   owner-recovery checkpoints succeed.

## Substrate question

A guest that silently regenerates a signing key can be locked out of every
later verifier checkpoint with no owner act to recover. Any computer that
went through a lost-disk boot or a cold recover after its pin is latent:
it will fail its next owner-approved apply. The fix is not to make the key
survive every disk loss (escrowing a verifier private key on the host would
give the host the verifier's authority); it is to make the regeneration
loud, to check the pin before an attempt is spent, and to give the owner an
act that re-pins the key the guest presents.

## Fleet (read-only check, 2026-11-02 ~23:40Z)

Running and hibernated computers on Node B, against `control_key_history`
(`verifier-control`, `status='active'`) and the `data.img` birth time:

| computer | state | verifier pin | data.img born | verdict |
| --- | --- | --- | --- | --- |
| computer-59c902c5 (d@e) | active | 22033e83, 10-26 | 10-30 21:43 | mismatch (refused today) |
| computer-6c50d6e5 (b@c) | active | fda688cd, 10-24 04:27 | 10-24 03:53 | match (verifier checkpoint 22:17Z today) |
| computer-1355b83b | active | none | 10-30 04:00 | unpinned: first-use pin |
| computer-03335285 | active | none | 10-07 | unpinned: first-use pin |
| computer-f1daccdb | hibernated | none | 10-31 22:12 | unpinned: first-use pin |
| computer-7cc96dc6 | stopped | none | 10-06 | unpinned: first-use pin |
| computer-bb0f4fa5 | hibernated | none | no image in the VM dir | unpinned |

Only d@e is locked out. An unpinned computer pins the key it presents at
its next verifier checkpoint (the first-use branch), so it is not latent.

## Fix shape

1. Signer: record a key-id sidecar at generation; a missing key with a
   sidecar present (partial loss) is a typed refusal; a generated key is
   reported (`generated_at`) on the signer's public-key route and logged.
2. Platform: a read-only verifier-key preflight (pin state for a presented
   key) used by the checkpoint authority and by the guest; an owner-act
   re-pin route that revokes the active pin and pins the presented key,
   authorized only by a proxy-signed owner-act receipt, recorded in
   `control_key_history` and `owner_act_receipts` with the reason.
3. Guest: `/health` carries `verifier_key` (pin state) and degrades on a
   mismatch; the materializer runs the preflight before an attempt is spent
   and holds a mismatch with a typed reason (no attempt, no retry wake); a
   checkpoint refused for the key is the same hold. The re-pin records a
   `key_rotated` event on the tape and wakes the reconciler once.
4. CLI: `choir computer repin-verifier --computer X --reason R` (owner act)
   and `choir computer verifier-key --computer X` (read).
