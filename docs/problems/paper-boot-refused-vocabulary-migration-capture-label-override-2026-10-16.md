# The paper refused to boot: the vocabulary migration read "-super-" in a capture label override's text as an identity change (2026-10-16)

Status: documented; fix in the next commit. **The paper is down.**

## Evidence

The deploy of c8e1dc259 (12:47 UTC) gave the paper its planned refresh.
The guest booted and then refused to start (paper console, 12:49:18):

```
autoputer: vocabulary migration refused: og migrate: choir.capture_label_override
obj:choir.capture_label_override:ZTU5…:key-a619… has migrated identity fields
but no verified identity formula
autoputer: startup refused (startup_failed)
```

vmctl answered the refresh with 409 `computer recovery blocked
(guest_startup_failed)`. The paper's lifecycle state is `failed`, and
its API returns 502. The deploy job failed on the paper's refresh. The
other two computers it refreshed came up.

The 10:50 boot (bc67aee10) passed the same migration. An override
written since then trips it. c8e1dc259 does not touch the migration, so
reverting it would not help: every boot runs the migration
(`internal/autoputer/run.go:690`).

## Mechanism

- The boot-time vocabulary migration rewrites V1 desk tokens in every
  object-graph row. That includes the infix `-super-` → `-management-`
  anywhere in a string (`migrateIDForward`).
- A `choir.capture_label_override` holds free text: the capture title,
  the story headline and the reason. A headline such as "Ex-super-…"
  matches.
- `identityFieldMigrated` treats any changed `-`-bearing leaf as a
  possible identity field. `choir.capture_label_override` (and
  `choir.capture_label`) had no entry in `ogIdentityFormulas`, so the
  migration refused instead of verifying that the id is stable.

Reproduced by `TestVocabMigrationKeepsCaptureLabelOverridesWithDeskWordsInText`.

## Fix direction

Add the two kinds' identity formulas. Labels are keyed by (computer,
capture, version), and overrides by (computer, capture, label version,
record), as in `captureLabelKey`. The migration then verifies the id
from those fields and keeps it.

## Not fixed here

- **The migration still rewrites free text.** A headline or title
  containing `-super-` becomes `-management-` in the object body. A
  V1→V2 cutover tool running over post-cutover content at every boot is
  the substrate problem: it should touch only rows written before the
  cutover, or only identity and desk-token fields.
- Any other frozen kind without a formula whose free text holds
  `-super-`, `-researcher-` or `-co-super-` can brick a boot the same
  way.
