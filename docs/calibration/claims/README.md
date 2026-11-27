# Calibration claims

One file per claim, named `<YYYYMMDDTHHMMZ>-<slug>.md`: the UTC minute it is
recorded, then a lowercase slug. The file name without `.md` is the claim's id,
so no shared counter exists and two lanes cannot pick the same id. Cite a claim
by that id (`claim 20261127T1432Z-batch-lands-green`) in commit messages, goal
files and settlements. Claims C1 to C257 stay in
[the older ledger](../claude-ledger.md) with their ids.

```
---
id: 20261127T1432Z-batch-lands-green
recorded: 2026-11-27T14:32Z
who: lead
kind: landing
p: 0.8
settle_by: 2026-11-28T14:00Z
evidence_source: a CI run on main, read with gh run list --commit <sha>
status: open
---
The claim and its test, written before the evidence exists.
```

`evidence_source` names, at record time, the outcome the author cannot write (a CI
run, a deployed proof, a count, the owner's verdict; learning-engine rule 1,
[docs/learning-engine.md](../../learning-engine.md)). Settling edits this file only:
set `status: settled` and add

```
settled: 2026-11-28T09:00Z
outcome: true | false | partial | lapsed
evidence: the run, count or verdict that settled it, cited
brier: 0.04
```

`brier` is `(p - outcome)^2` with outcome 1 for true and 0 for false; partial and
lapsed carry none. `scripts/ledger due` lists open claims past their settle-by
time; `scripts/ledger score` gives the scorecard by band; `scripts/check-entries`
validates every file (run by `scripts/gates`). `choir precommit import-ledger`
reads only the older file.

A claim is fixed once it is on main; a lane may correct its own claim before it lands, and a claim must be on main before its evidence exists. Against the base, `scripts/check-entries --base`
refuses, for a claim that exists at the base, any change to `recorded`, `who`, `kind`, `p`, `settle_by`, `evidence_source`
or the claim text, any change to a claim already settled, any deleted or renamed claim file, and any claim it cannot read on either side;
only `status`, `settled`, `outcome`, `evidence` and `brier` may be added. Whether
`evidence` is the outcome `evidence_source` promised cannot be checked mechanically:
the outcome is cited, and a seat or the owner reads it.

Front matter is single-line `key: value` scalars only (no `|` or `>` blocks, no
indented lines), and every time is a full zero-padded UTC time. Name the slug for
the thing specifically: two lanes that choose the same slug in the same minute meet
in an add/add conflict, which stops the landing, and that is the intended outcome.
