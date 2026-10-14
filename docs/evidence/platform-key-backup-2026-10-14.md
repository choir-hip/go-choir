# Platform Key Backup, 2026-10-14

Owner request (2026-10-14, about 10:40 UTC): "maybe you could SCP them to
node A or save them somewhere locally on my iMac." The owner's iMac was
chosen over node-a, which is shared with another project.

- **What:** the escrow custodian key (`key-escrow-key`, 44 bytes) and the
  platform signing seed (`signing-key`, 64 bytes), from
  `/var/lib/go-choir/platform-artifacts/` on Node B (root, 0600). No
  environment override moves them (checked the corpusd unit and its env
  files for `PLATFORM_SIGNING_KEY_PATH` and `CHOIR_KEY_ESCROW_KEY_PATH`).
- **Where:** the owner's iMac, `~/Choir Key Backup/2026-10-14/`, directory
  0700, files 0600. The disk's FileVault status is recorded in the session.
- **Check:** SHA-256 of each copy equals the source on Node B. Contents and
  hashes were never printed or written to the repo.
- **Not yet done (owner, this evening):** move the copies to offline,
  encrypted storage (and delete the laptop copy if that is the policy);
  name SP0's host and the recovery targets. O22 is still open until a
  restore from the copy is drilled.
