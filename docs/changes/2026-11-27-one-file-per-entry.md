---
date: 2026-11-27
title: a change-log entry and a calibration claim are each one new file
class: yellow
---
What: a new change-log entry is one file in `docs/changes/` (`YYYY-MM-DD-<slug>.md`, front matter plus the five labelled parts) and a new calibration claim is one file in `docs/calibration/claims/` (`<YYYYMMDDTHHMMZ>-<slug>.md`, the file name is the id). `scripts/changes` prints the log newest first from both the directory and the old `docs/changes.md`; `scripts/ledger` lists open claims past their settle-by time and scores the directory by band; `scripts/check-entries` (with `scripts/selftest-check-entries`, both in `scripts/gates`) validates every file. `docs/changes.md` and `docs/calibration/claude-ledger.md` stay as the record to now, each with a note at the top of its intro; C1 to C257 keep their ids. CLAUDE.md's two sentences point at the directories.

Why: in 14 days 120 commits touched the change log and 255 the ledger, out of 1,613, and every lane appending to the same two files conflicted with the others (`union_docs` in `scripts/land` joins them by hand); two lanes also chose the same next claim id. One file per entry and an id made from the time and a slug leave nothing shared to conflict on.

Proof: not proved on a landing yet. Run in this lane: `python3 -I scripts/selftest-check-entries` (every rule has a failing and a passing case, and the two readers run on a fixture tree), `scripts/check-entries` on the real tree. The mission's acceptance (two lanes landed in one batch, each with an entry and a claim, no hand join) is not run.

Replaced: appending to the top of `docs/changes.md` and to the Claims table of the ledger. `union_docs` stays for lanes in flight that still edit those files; once none does it can go. `choir precommit import-ledger`, `scripts/e1`'s ledger gate and `scripts/system_briefing.py` still read only the old ledger file and so do not see new claims.

Undo: revert the commit; the directories' files then have no reader, and their content can be pasted into the old files.
