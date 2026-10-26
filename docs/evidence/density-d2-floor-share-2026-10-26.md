# D2 floor share: does a deploy keep the guest floor? (from 2026-10-26)

The S6 gate of [D2](../design/density-d2-snapshots-survive-deploys-2026-10-26.md)
(§7 S6, §10 R9): over at least 7 days and at least 20 deploys that run a NixOS
switch, at least 50% keep the previous switch deploy's `floor_digest`,
`security_epoch` and `guest_abi`. Below 50%, the built-binary floor digest is
designed before S6.

Method: after each staging deploy that ran a NixOS switch, read the deployed
guest's release facts on Node B (read only):
`ssh node-b 'cat /var/lib/go-choir/guest/release.json'`. A missed deploy can
be read later from its commit's `.#packages.x86_64-linux.guest-release-json`
on a Linux builder. Append one row per switch deploy; compare with the row
above.

| Deployed (UTC) | Commit | Train | floor_digest | epoch | abi | Kept the floor? |
|---|---|---|---|---|---|---|
| 2026-10-26 20:35 | 013e6bce0 | 21 (S1 to S5 land) | 61492fcb… | 1 | 1 | first row |
| 2026-10-26 20:50 | 9a9273fef | 22 (D6, D5 step 2; guest code changed) | 61492fcb… | 1 | 1 | yes |

Running share: 1 of 1. The gate opens no earlier than 2026-11-02 and after
20 rows.
