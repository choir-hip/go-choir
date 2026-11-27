# Change-log entries

One file per thing built, named `YYYY-MM-DD-<slug>.md` (lowercase letters,
digits and hyphens; the date is the day you write it). Two lanes never write the
same file, so they never conflict. Read them newest first with
`scripts/changes [--since YYYY-MM-DD] [--full]`; it also lists the older entries
of [docs/changes.md](../changes.md), the record up to 2026-11-27, which is not
split or migrated. Updating an entry edits its own file. `scripts/check-entries`
(run by `scripts/gates`) validates every file here.

```
---
date: 2026-11-27
title: the short name of the thing, as a reader would look it up
class: green | yellow | orange | red | black
---
What: what changed.
Why: why.
Proof: the CI run, the deployed check, the count; or "not proved" and what is missing.
Replaced: what it replaced.
Undo: how to undo it.
```

Each of the five parts is a paragraph starting with its label (it may run over
several lines). The rule that every behavior-changing landing has an entry is in
[CLAUDE.md](../../CLAUDE.md) (owner, 2026-11-22: "its hard to catch up on what
claude has done since i last looked").

Front matter is single-line `key: value` scalars only. Name the slug for the thing
specifically: two lanes that choose the same slug on the same day meet in an add/add
conflict, which stops the landing, and that is the intended outcome.
