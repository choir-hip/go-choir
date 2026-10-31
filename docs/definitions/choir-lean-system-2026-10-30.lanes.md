# Lean system: open lanes

The register of lane branches this mission has opened that are not on main.
Owner, 2026-10-31: "we should be careful to not accumulate lane branches
that don't land. We should keep track of them and understand when we can
land them." A lane leaves this table when it lands on main (name the commit)
or is closed (name why). Update it every time a lane opens, lands or closes.

| lane | what it does | state (2026-10-31) | lands when | blocker |
|---|---|---|---|---|
| lane/backup-memory | weekly restic prune unit, prune-keygen | built, 1 commit | next deploy | none; missed the 2b2bc2c7d merge |
| lane/snapshot-sweep | sweeps VM memory snapshots that can never resume (8 GiB each) | built, tests pass | next deploy | none |
| lane/tape-seg-compact | rewrites sealed tape segments once; ~21 GB on Node B | built, tests pass | next deploy, then a d@e dry run and apply with --compsize | reclaim unmeasured on btrfs |
| lane/chaos-harness | d@e-only crash drills, scenarios 1-3 | built, selftest 111 pass | with or after tape-group-commit | needs group commit on d@e to be useful |
| lane/tape-group-commit | groups acknowledged tape writes; telemetry rides the next write | 3 commits, tier-2 review running | after review accepts | guest security epoch 23 conflicts with tool-layers |
| lane/tool-layers | erofs layer per tool (office, browser, toolchain, delegate) | 2 commits, unbooted | its own deploy, after group commit | epoch 23 conflict; needs a boot proof |
| lane/mac-arm-guest | aarch64 guest image in Nix | 1 commit; image build running | after a vfkit boot proof | moves the x86 floor digest: one cold boot of hibernated snapshots; ship with snapshot-sweep |
| lane/bundle-authenticity | receipt signature checks; refuse a snapshot root the tape does not name | WIP | when its builder reports | in progress |
| lane/store-churn | paper SQLite churn (clustered keys) | 1 commit, benchmark running | after benchmark and a d@e proof | in progress |
| lane/layered-image, lane/layered-image-r0, lane/release-storage | earlier layered-image and release-storage work | 1 commit each not in main by patch id | triage: likely landed under another id | triage running |
| lane/mac-follow | Mac app follows a computer through bundles (sync slice 1) | builder running | after host API deploy and the d@e follow E2E | in progress |

Older lanes (1 to 9 days, from earlier missions): 66 lane branches carry a
commit whose patch is not on main; 247 more are fully landed and only left
over. A read-only triage sorts the 54 older ones into landed, superseded,
docs-only and live; its result replaces this paragraph.
