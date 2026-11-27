---
date: 2026-11-27
title: the lander removes landed lanes and keeps selftests out of the timing log
class: yellow
---
What: after a successful push scripts/land prints, for each lane whose every commit went into the train, the line `land: landed: <lane>; remove it with: (cd <primary> && lane rm <lane> --force)`, and for a skipped or partly picked lane a line saying it was not fully landed and should not be removed. It removes nothing. Automatic removal (`--rm-lanes`, `LAND_RMLANE`) was built, sent back by review twice, and removed: it deletes branches and trees on a reading of git state that can be wrong (git errors read as clean, a failed worktree list, a tip not rechecked), so the lander prints the command and a person runs it. The timing changes stay: scripts/land hands the gates the log path its own line goes to, scripts/gates falls back to scripts/land-timing.sh in the checkout, a sourced (selftest) gates run writes the log only when LAND_TIMING_LOG is set, selftest-land-timings sets its own log, scripts/selftest-gates-runner parses under bash 3.2, and scripts/selftest-timing-log-guard runs the four timing-related selftests with HOME empty and a sentinel log preset and fails if either is written.

Why: the lane tool compares commits and the lander cherry-picks, so `lane rm` refused every landed lane ("commits main does not have") and 134 landed lanes were found unremoved. A selftest run left a `gates` line (steps `sleep 3` and `true`) at the end of the real timing log after a landing, and the landing's own gates line was not found there.

Proof: scripts/land.test.sh (the removal command is printed for a landed lane, the do-not-remove line for a skipped one, and the branch, tree and an untracked file are untouched), scripts/land-queue.test.sh, scripts/selftest-gates-runner, scripts/selftest-land-timings and scripts/selftest-timing-log-guard pass. The missing gates line of the real landing was not reproduced: the real gates run from a staged copy wrote their line in a throwaway repository, so that part is a hardening and not a proved cause. Not run against a real lane tool or a real landing.

Replaced: removing landed lanes by hand after checking each by eye; the command is now printed.

Undo: revert the commit; the lane tool and the landings are unchanged without it.
