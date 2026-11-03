# A lost data disk regenerates the verifier key and locks the computer out of every verifier checkpoint (2026-11-02)

Status: fix built on branch `verifier-key-repin` (not merged, not deployed;
tier 2 review pending). Class: red (signing keys, checkpoint authority,
self-development apply path). No re-pin has been run on any computer.

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

1. Signer: a generated key is recorded beside it
   (`key.ed25519.provenance.json`, written through a unique O_EXCL
   temporary) and reported (`generated_at`, `generated_this_boot`,
   `prior_key_id`) on the public-key route and in the log. The signer still
   generates and starts: the autoputer requires its unit, so a refusal would
   brick the computer. On d@e's shape the record is lost with the disk, so
   `prior_key_id` is empty; the signal there is `generated_at` and the
   host's mismatch.
2. Platform: a read-only verifier-key preflight (the pin state of a
   presented key, the same lookup the checkpoint authority makes); an
   owner-act re-pin route that revokes the active pin and pins the presented
   key, authorized only by a proxy-signed owner-act receipt, recorded in
   `control_key_history`, `owner_act_receipts` and a new activation record
   (`control_key_activations`) with the reason. A key revoked by an earlier
   re-pin (a disk restored from an older image) is re-activated by the same
   owner act under a new window: its row's window starts at the
   re-activation sequence and time, its closed earlier window moves to the
   activation record (`prior_window_*`), and `ControlKeyResolver` honors
   that earlier window, so a receipt the key signed before its revocation
   still resolves and one it signed while revoked never does. Without an
   owner act a revoked key stays refused. Resolvers of verifier-control
   keys: `ControlKeyResolver` (`platform/control_keys.go`, behind the
   platform's bootstrap resolver) is the only one that reads
   `control_key_history` windows; the checkpoint authority
   (`checkpoints.go:117`) and the preflight compare the active pin only;
   `verifierprotocol`'s certificate resolver uses the certificate's own
   key; the bundle trust set refuses any domain but its own.
3. Guest: `/health` carries `verifier_key` (pin state) and degrades on a
   mismatch; the materializer runs the preflight before an attempt is spent
   and holds a mismatch with a typed reason (no attempt, no marker, no retry
   wake). When the preflight cannot answer (host unreachable) or the pin
   changes after it, the checkpoint refusal arrives after `markPlannedRestart`
   spent that attempt: one attempt is spent, then the same hold stops every
   later one. The re-pin records a `key_rotated` event on the tape and wakes
   the reconciler once.
4. CLI: `choir computer repin-verifier --computer X --reason R` (owner act)
   and `choir computer verifier-key --computer X` (read).

## Computers on an older guest release

The re-pin reads the presented key through a guest route this change adds.
A computer still on an older release answers 404, and the proxy answers 409
with the way out; there is no compatibility path. The owner abandons the
held operation (`choir self-dev abandon`), which leaves the computer idle so
the deploy refresh (or `choir computer refresh`) boots the current release,
then runs `choir computer repin-verifier` and retries the change. d@e's held
operation was already abandoned, so d@e needs only the refresh and the
re-pin.

## Where the fix lives

- Signer provenance: `internal/receiptsigner/signing.go:98`
  (`LoadOrCreateSigningKeyWithProvenance`), reported on `/v1/public-key`.
- Preflight and re-pin: `internal/platform/verifier_key_pin.go:53`
  (`VerifierKeyPreflight`), `:98` (`RepinVerifierKey`, re-activation included), `:259` (route),
  `:302` (owner-act check, every rollout mode).
- Proxy: `internal/proxy/verifier_repin.go:60`.
- Guest hold and health: `internal/agentcore/selfdev_verifier_key.go:151`
  (`verifierKeyHold`), `:205` (`/health` `verifier_key`); call sites
  `internal/agentcore/self_development_materializer.go:1468` (before the
  attempt), `:1145` (checkpoint refusal), `:446` (no retry wake).
- CLI: `cmd/choir/commands.go:457` (`computer repin-verifier`) and
  `computer verifier-key`.

Fleet check once deployed: `choir computer verifier-key --computer X`, or
the `verifier_key.pin` field of each guest's `/health`.
