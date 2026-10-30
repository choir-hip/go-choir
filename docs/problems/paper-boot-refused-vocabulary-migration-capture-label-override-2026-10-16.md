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
written since then trips it. **Corrected 16:40:** not every boot runs the
full migration. The 12:49 boot replayed a one-event tail, which forces the
full rescan; boots without a replay take the fenced fast path. See
`live-writes-refused-by-deposit-upcast-prose-leaves-2026-10-16.md`. c8e1dc259 does not touch the migration, so
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

## Clustering assessment (two refusals of one class)

| Incident | Kind | Path | Effect |
|---|---|---|---|
| 2026-10-15 | `choir.commitment_record` | checkpointd replay (deposit upcast) | two QA computers' checkpoints can never rebuild (`checkpointd-replay-upcast-missing-identity-formula-2026-10-15.md`) |
| 2026-10-16 | `choir.capture_label_override` | boot-time `MigrateAndFenceServingVocabulary` | the paper refused to boot for 18 min |

**Shared cause.** The V1→V2 vocabulary rules (`migrateIDForward`) are
applied to every string leaf of an object, then `identityFieldMigrated`
treats any changed `-`- or `:`-bearing leaf as a possible identity
field. Free text (headlines, titles, reasons) and references both
qualify. Any frozen kind without an `ogIdentityFormulas` entry then
refuses. Adding formulas kind by kind (3d669e89b) repairs instances. It
does not change the class: the vocabulary families list many frozen
kinds and the formula table names about 40.

**Unexplained, a hypothesis to check before the substrate fix.** The
boot call has a fast path: once a store's report carries `FencedAt`,
a boot without replay returns at once
(`internal/store/vocab_migrate.go:210`). The paper ran the full pass at
12:49, so either its report (a file beside the store, `texturePath`)
lacked `FencedAt`, or this boot reached the migration as a replay after
the projection-recovery resume. If the paper re-migrates on every boot,
then every boot also rewrites "-super-" in post-cutover free text, and
explains part of the long store-lock holds after boot (`GetObject`
held up to 48 s at 13:07). Reading the report file needs the guest's
disk; it is not read while the guest runs.

**Substrate options (red: checkpoint projection and boot; owner or
panel decision):**
1. Restrict the vocabulary rules to identifier-shaped leaves (whole
   values matching an id grammar), never substrings of prose. Replay must
   then equal live for every existing tape. The f95311775 "replay matches
   live" class applies, and old checkpoints may need rebuilding.
2. Freeze the boot migration as done: a store fenced once is never
   migrated again, and the report moves into the store itself so it
   cannot be lost with a file.
3. Keep refusing, but make a refusal terminal for that object (record it,
   serve the rest), not for the computer's boot.

Recommended order: confirm the hypothesis (the report file and its
`FencedAt`) on the next stopped boot, then option 2, which is the
smallest change that ends boot-time re-migration. Option 1 is the true
repair and needs its own review.

**Resolved 16:40 (b8feaa147).** The hypothesis above is answered: the
report is fenced, and the 12:49 boot was a replay (tail=1). The
substrate fix is option 1, narrowed: a leaf with whitespace or "://"
is never rewritten. See
`live-writes-refused-by-deposit-upcast-prose-leaves-2026-10-16.md`.


## Merged from docs/problems/paper-boot-refused-vocabulary-migration-capture-label-override-2026-10-16.md (2026-10-16)

**The paper's live writes are refused (or rewritten) by the deposit upcast: prose leaves with "-super-" read as identities (2026-10-16).**
Status: fixed in b8feaa147 and deployed 16:30; outcome below. Third symptom of the class in the clustering assessment of `paper-boot-refused-vocabulary-migration-capture-label-override-2026-10-16.md`. **A boot with a replay tail can take the paper down again.**
- Status: fixed in b8feaa147 and deployed 16:30; outcome below. Third symptom of the class in

(Full text in git history at the path above.)
