# Screens-as-files reviews (2026-11-03)

Tier 2 (red surfaces: capsule Freeze, run-acceptance evidence, Texture
evidence write, bundle schema). Branch selfdev-screens-as-files.

- Round 1 (e41f378b8): Devin accept with non-blocking findings (staging
  namespace collision, partial-save embeds, crash mid-save, fsync, secret
  scan does not decompress, pixel bound). DeepSeek v4.1 flash send_back,
  two blocking-real: the capsule (uid 65534) could not create
  /var/lib/artifact/screens, so every Freeze staged zero screens; and
  scripts/test-node-b-guest-release-retention.sh still asserted the old
  browser grant profile.
- Fix (4d305c7d9): a capsule-owned screens leaf beside release; the
  retention script matches browser=*; Freeze replay re-saves missing
  screens; only resolvable screens embed; the Files writer refuses symlinks;
  screens stage where no release file can be; exact blank check, 4096x8192
  pixel bound; directory fsyncs.
- Round 2: Devin accept ("All round-1 blockers verified fixed and complete;
  no new regressions found"), non-blocking residuals: a crash window between
  recordFreezeScreens and the embed request; the same clobber class at
  bundle.draft.json and source.patch; nits. DeepSeek accept.
