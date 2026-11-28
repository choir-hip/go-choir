# The vault, storage and phone app design groups: which one stands

Standing: governing | kind: happened | settled-by: a reading of origin/main 23bd2bfe2 on 2026-11-28 | note: station T10 of docs/definitions/choir-docs-transform-2026-11-27.md; a reading, not a ruling; where a file's own Standing line differs, that line wins

This is a reading of main, done for station T10. It changed only the Standing
lines of the files below. A file is `governing` only where an owner statement
or a landed build says so. Where the evidence does not decide, the file is
`undecided`.

## Vault designs (3 files)

No vault is built. A search of `internal/`, `cmd/` and `nix/` finds no vault
service and no key release code. The only "vault" in Go is the snapshot store in
`internal/vmmanager`, which is a different thing. The owner said on 2026-11-27
"we need two vaults" (docs/inbox.md lines 167 to 169): one for the platform's
secrets, one for people's credentials. No design is written for the second.

| Doc | Was | Now | Evidence |
| --- | --- | --- | --- |
| docs/provider-vault-and-model-policy-design-2026-10-22.md | undecided | undecided, note names the other vault design | Proposal, not ratified (line 5). Owner rulings R1 to R5 of 2026-10-23 (section 9) settle five questions, not the design. No code. |
| docs/design/secrets-vault-design-2026-10-28.md | undecided | undecided, note names the other vault design | Draft; line 386 says "design accepted; execution held by the owner" but quotes no ruling. The acceptance in the log is a review seat (Devin). No code. |
| docs/design/mac-device-key-release-2026-11-01.md | undecided | undecided, note says it is a third thing | Accepted by a review seat, not the owner. Its goal is also undecided. No code. |

## Storage designs (19 files: 4 missions and 15 designs)

There is no "dozen" fixed list. These are all the storage and snapshot files
under `docs/design` and `docs/definitions`. Five of them had no Standing line.
The live work is the mission `choir-storage-efficiency-2026-11-22.md` (owner,
2026-11-22, live in docs/ACTIVE.md).

| Doc | Was | Now | Evidence |
| --- | --- | --- | --- |
| definitions/choir-storage-efficiency-2026-11-22.md | governing | governing (unchanged) | Owner quoted in its authority field. |
| design/snapshot-one-pass-2026-11-22.md | none | governing | Stations S2 and S3 of that mission are this design. Stage A1 landed as 9e3e2060f. The capture half is not built (docs/changes.md line 359 and the entry at 602). |
| design/one-tape-durability-2026-10-31.md | governing | governing (unchanged) | Owner 2026-10-31. |
| design/density-d2-snapshots-survive-deploys-2026-10-26.md | none | built, in part | docs/system-map.md section 2 (wake rules and the 168 h window); 727bb979e. |
| design/snapshots-cost-the-change-2026-11-13.md | none | superseded, in part | Its section 7 keeps the logical witness; the one-pass design drops it. The rest is not shown built or set aside. |
| design/storage-pressure-frame-2026-11-12.md | none | undecided | A frame, not a ruling. Beside the live mission, whether it still stands is open. |
| design/store-once-2026-11-14.md | none | undecided | Nothing built from it as a whole. Its mission is listed as superseded in evidence/mission-closing-verdicts-2026-11-27.md, which the owner has not read. |
| design/snapshot-without-scratch-copy-2026-11-20.md | superseded | superseded (unchanged) | By snapshot-one-pass. |
| design/restore-bases-store-once-2026-11-15.md | superseded | superseded (unchanged) | Set aside after three review rounds. |
| design/fp10-snapshot-levers-and-host-slab-2026-10-22.md | superseded in part | unchanged | L20 section replaced by density D2. |
| design/small-object-storage-2026-10-31.md | undecided | unchanged | Nothing built. It says it integrates with object-storage and does not replace it. |
| design/computer-history-restore-audit-2026-10-30.md | undecided | unchanged | Not ratified, nothing built. |
| definitions/choir-storage-solutions-2-2026-11-12.md, ...-3-store-once-2026-11-14.md, choir-store-header-body-split-2026-11-02.md; design/append-clustered-store, bounding-the-guest-store, object-storage, store-header-body-split | built (some in part) | unchanged | Already read in the first half of T10. |

## Phone app designs written in November 2026 (4 files)

The task described these as SwiftUI-era. The files do not say so. The lab note
carries the owner's "no swiftui please. lets do all exact" (2026-11-10, its
line 31), eight days before the Exact rule. Only app-ui puts page logic in
Swift, in its lane table (about lines 1297 to 1350), and the rule replaces that.

| Doc | Was | Now | Evidence |
| --- | --- | --- | --- |
| design/autoradio-app-ui-2026-11-10.md | built, in part | superseded, in part, replaced-by the Swift-not-Exact problem record | CLAUDE.md Product (owner 2026-11-18 and 2026-11-20). The page, bar and deck design is built as shared Exact (current-architecture rows 113 and 143) and is not replaced. |
| design/autoradio-lab-2026-11-10.md | built, in part | built, in part, note changed | Owner 2026-11-10 in the file. Swift kit modules stand only for voice. |
| design/autoradio-turns-2026-11-10.md | built, in part | built, in part, note changed | The turns and programme routes exist. Not a SwiftUI design; its Swift is native voice. |
| design/autoradio-voice-modes-2026-11-10.md | superseded | superseded, in part | Owner phone review 2026-11-15: hands-free only. Its audio-session findings still serve native voice, so only the three-modes choice is replaced. |

## Questions only the owner can settle

1. Which vault design, if any, is the platform's secrets vault, and does the
   people's credentials vault get a new design? Applied: all three undecided.
2. Does `design/store-once-2026-11-14.md` stand, or does the storage efficiency
   mission replace it? Applied: undecided.
3. Does `design/storage-pressure-frame-2026-11-12.md` still frame the storage
   work? Applied: undecided.
4. Is the line "design accepted; execution held by the owner" in the secrets
   vault design an owner ruling? No ruling is quoted. Applied: undecided.
5. Were the Swift parts of app-ui meant to be replaced whole? Applied:
   superseded in part, only the lane plan that puts logic in Swift.

## What T10 can carry

Carrier for station T10: this file and the commit that adds it. The station
status is not changed here.
