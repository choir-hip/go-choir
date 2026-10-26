# Provider keys ride in the off-host backup in plaintext form

Status: open. This record documents the problem first; this commit has no
fix. Mutation class of a fix: red (backup contents).

Found 2026-10-22 by reading code. No backup was opened.

## What the code does

- `scripts/off-host-backup.sh` adds `/var/lib/go-choir/secrets` as a
  required path (`:233`). It adds `gateway-provider.env` and
  `codex-auth.json` as optional paths (`:235-236`).
- Those two files hold every provider API key and the ChatGPT OAuth token
  set in plaintext (`nix/node-b.nix:2252-2278`).
- Restic encrypts the repository. But one backup secret (the restic
  password and the REST credentials under `secrets/offhost-backup`) then
  opens the provider keys together with everything else. The FP9 record
  names that class: "one backup secret opens keys and data together"
  (`docs/definitions/choir-launch-fp9-keys-recovery-2026-10-21.md`, heresy
  delta).
- The escrow key's KEK is already excluded by name, `key-escrow-kek*`
  (`off-host-backup.sh:188-228`). Nothing comparable protects the provider
  keys.

## Owner ruling

R5, 2026-10-23: the vault key "for now just kept out of backups". The
design keeps sealed provider records in the backup and leaves out their
KEK, named `provider-vault-kek*`
(`docs/provider-vault-and-model-policy-design-2026-10-22.md` §2.2).

## Open

Whether existing snapshots should be pruned of these files after the
vault lands. That is a retention decision, recorded here and not taken.

## Interim fix (2026-10-26)

Owner, 2026-10-26 02:4x, accepting FP0's default: exclude the provider keys
from future backups by name; keep the existing snapshots untouched (pruning
them is a destructive retention decision, still open). `scripts/off-host-backup.sh`
no longer adds `gateway-provider.env` or `codex-auth.json` as paths and
excludes both names at any depth, the way it excludes `key-escrow-kek*`.
Test: `scripts/off-host-backup-provider-keys-exclude-test.sh`.

Restore consequence: after an off-host restore, the provider keys and the
ChatGPT OAuth token set are written back by hand to
`/var/lib/go-choir/gateway-provider.env` and `codex-auth.json` (the files the
gateway units read, `nix/node-b.nix` about lines 506 and 2452), as the vault
design already assumes for its KEK. Rollback: `git revert` of the fix commit; the next backup carries
the files again. The vault (sealed records in the backup, KEK kept out)
remains the durable fix.
