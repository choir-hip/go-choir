# The listener's notebook, as built

Status: stations L4 and L5 of
[the learning-loop goal](../definitions/choir-autoradio-learning-loop-2026-11-18.md),
built and tested on lane `learn-notebook` (on top of `learn-settle`),
**not deployed**; the proof `scripts/prove-autoradio-learning` (notebook phase)
has not been run on d@e. Design (normative):
[autoradio-learned-document-2026-11-17.md](../design/autoradio-learned-document-2026-11-17.md)
sections 2 to 4 and 6. Code: `internal/notebook` (grammar and arithmetic, no
store, no model), `internal/textureowner/notebook_owner.go` (Texture's side),
`internal/agentcore/api_autoradio_notebook.go` (Autoradio's side),
`internal/lease/grants.go` (the grants).

## The document

One Texture document per listener, titled `My notebook`. A document titled
`My edition` is adopted: found by title (`editiondoc.IsEditionTitle` accepts
both), retitled at the first desk patch, never duplicated. `GET
/api/texture/edition` keeps its path and answer; a document titled `My
notebook` beats a legacy one, then the most recently updated wins. Texture
creates it at the first patch when none exists, with the desk (not the owner)
as author of its first revision. Lines above the first `##` heading that are
bullets (an adopted edition's own) count as "What you asked for".

```markdown
# My notebook
## What you asked for        the listener's lines; never cut by a program
- Mornings: world news first.
## What Autoradio has learned  at most 12 lines
- <text> [L7 · heard 9 of 11]
## What Autoradio is trying    at most 3 lines
- <text> [T2 · heard 1 of 3]
## Recent steering             the last 10 taps
- More like this: "Title" (17 Nov 08:12)
```

**Line grammar.** One bullet per line. A tagged line is `- <text> [L<n>|T<n> ·
heard <h> of <m>]` (the dot is U+00B7). Text: one line, 1 to 200 characters,
no `[L`/`[T` plus digit, not starting with `-` or `#`. Refs (for `caused_by`
and cites): `lesson:L7`, `try:T2`, `mine:<first 8 hex of sha256 of the trimmed
text>` for any untagged bullet, `tap:<sheet entry id>` for a steering line
(the id is not in the text: the handler finds it from the steer note by the
line's exact text). A line the listener edits gets a new `mine:` ref; old
forecasts keep the old ref.

**Bounds.** Target about 400 words; hard cap 1,000 words over learned, trying
and steering together, counted by Texture at write time (a patch over it is
refused, whole). Never a line per segment or forecast. A confirmation changes
a count and adds no text. A full section takes a new line only if it names, in
`replaces`, live lines of that section; the prompt states the budget plainly
("holds 12 lines and has 12"). The listener's lines are never cut.

## Who writes: the patch, with no model

The DJ writes the words; Texture applies them. The Autoradio handler appends
`autoradio.notebook_patch` (status `asked`: base revision, ops, cites, minted
ids, counts) and calls `Handler.NotebookApply(ctx, ownerID, "desk:autoradio",
patch)` in process (agentcore cannot import textureowner, so the handler holds
the `notebook.Service` interface, bound in `apihandler.RegisterRoutes`).
Texture checks the grant, that the head is the patch's base (else `stale`; the
handler rebuilds on the new head, up to 3 times), applies the ops with
`internal/notebook` (strike, retire, add, ask, tap, then counts, then the
page budget), and commits one revision: `AuthorAppAgent`, `AuthorLabel`
`app:autoradio`, metadata `source: autoradio_notebook_patch` and the patch id,
through `CommitLifecycleArtifactHeadWithSourceGraph`. The revision id derives
from the document and patch ids, so a replay answers `applied` with the same
revision. The handler appends the answer as a second `notebook_patch` record
(`applied`, `stale`, `refused` with a reason, `noop`). No Texture lifecycle
wake: `reconcileAgentWakeLockedFor` returns for a notebook-titled document, so
no model reads or rewrites a revision of it.

**Grants** (`internal/lease/grants.go`, table `grantTable`): `notebook.patch`
(ask Texture to apply a patch to this listener's notebook) and `notebook.read`
(read the head and the chain back to the desk's last revision). Custodian
Texture; holder `desk:autoradio` alone, for both. They are static holders like
`texture.search`'s, not per-run leases: Texture's two functions refuse any
other caller (`lease.GrantHeldBy`). `commit.texture_revision` stays Texture's.

## What the DJ is shown and writes

Stable (cached) part of the input: the fenced `notebook` section, every line
as `<ref>: <text>` with `(heard h of m)` on tagged ones, grouped by section
with the listener's first, then the budget sentence. Volatile part: the fenced
`settled` section, the last 40 settled forecasts, one row each:
`<claim id> | <kind> | <title> | p_heard <p> | caused_by <ref> | heard
through | not heard through` (a no-verdict forecast is not a row). Not shown:
an overall rate, a streak, a band table.

After its entries (never in an `open` request) it may write, one JSON object
per line:

```json
{"kind":"notebook","op":"add","section":"learned","text":"...","cites":["<claim id>","<claim id>"],"replaces":["L3"]}
{"kind":"notebook","op":"retire","line":"T2","cites":["<claim id>"]}
{"kind":"notebook","op":"steer","turn":"<turn id>","quote":"<verbatim>"}
```

**Proposals** are read after the stream ends `completed`; at most 12 lines are
collected and at most 4 `add`s written per stretch (a bound on the model's
output, not on calls: there is no second call). Each is dropped, with its
reason in `done.notebook.dropped` and the index, unless: `section` is learned
or trying; text passes `notebook.CheckText`; the text does not name `p_heard`
or tell the DJ to raise, lower, bump or shade a probability, forecast,
confidence or number (regexp `moveNumberRe`); at least 2 cites, each a claim id
shown in the `settled` section; the cites are not all on struck lines; the
text is not already a line; and the op applies (the page budget and `replaces`
live in that section). `retire` needs a live line and at least one shown cite.
`steer` needs a turn this call lists (`instruction:turn/<id>` or the bare id)
and a quote that is a verbatim substring of its words (at most 200 characters);
the line written is `"<quote>" (you said, <d Mon>, during "<title on air>")`
in "What you asked for". A model-sent id is ignored: the handler mints `L<n>`
and `T<n>` above the notebook's and the lesson records' maxima, so a retired id
is never reused.

**Counts.** `heard h of m` is computed by the handler on every extend, from
the ledger: of the settled forecasts (a no-verdict one is not settled) whose
`caused_by` ref is the line, `h` were heard through, `m` settled. It rides the
patch; a patch is written when there are ops, strikes to move, or any count
differs. So there is at most one revision per extend, and a confirmation
costs a revision but no text. The model never writes a count.

**caused_by.** `lesson:L<n>` (cause `lesson`), `try:T<n>` (cause `trying`),
`mine:<8 hex>` and `tap:<id>` (cause `instruction`) resolve against the lines
of this call's notebook; a ref not shown drops the entry line. The claim gains
the tag `line:<ref>` when it fits in 64 bytes. `edition:P<n>` still resolves
only when the computer has no notebook and the phone sent `edition_text`.

## The listener's edits and strikes

The listener edits the document through the ordinary Texture routes
(`AuthorUser`). Before each patch the handler compares the desk's newest
revision with the head: a tagged line missing from the head is struck
(`autoradio.strike`, kind `deleted`, with the line's cites from its lesson
record); one whose text changed is struck (kind `corrected`) and the patch's
`strike` op moves their words verbatim to "What you asked for" with
`(you corrected L7)`. A proposal whose every cite is a cite of a struck line
is refused. Strike records are written even when nothing else changes.
Untagged lines anywhere are the listener's and are never touched.

## Steering

`POST /api/autoradio/steer`, owner-authenticated like the sheet. Body
`{"entry_id","direction":"more"|"less"}` (the entry must be on the server run
sheet with a title) or `{"direction":"mute","publisher":"voice-xxxxxx"}`. It
writes an `autoradio.note` (id prefix `autoradio-steer:`) and patches at once:
a tap appends `More like this: "<title>" (<d Mon HH:MM>)` (or `Less`) to
Recent steering, keeping 10; a mute appends `Never play takes by <handle>.`
to What you asked for. Answer `{status, reason, doc_id, revision_id, line,
note_id}`. A spoken instruction is the `steer` op above.

## Routes (rule 2)

`GET /api/autoradio/learning` gains `notebook`: `found`, `doc_id`,
`revision_id`, `written_words`, `lines` (ref, section, text, and for tagged
lines id, heard, of, cites, replaces, patch_id, revision_id), `struck`,
`patches` (the asked and answered `notebook_patch` records), `steering` (the
steer notes), `inputs` (per stretch: `notebook_revision_id`,
`notebook_sha256`, `notebook_bytes`, `marked_cases`). `?line=<ref>` (for
example `lesson:L7`) narrows `rows` to the forecasts that line caused: the
rows behind a line. Record kinds this adds: `autoradio.notebook_patch` (new in
the table), and first writers of `autoradio.lesson` (id `autoradio-lesson:L<n>`;
`...:retired:<patch>` for a retired or replaced line) and `autoradio.strike`.
All are owner-private, and all are on that route. The document is read through
Texture's own routes.

## Choices where the design was silent or the code differed

- One commit set holds L4 and L5 together: the validators need the input's
  `settled` fence, so they do not stand apart.
- Grants are static holders in the lease grant table, not per-run leases
  (nothing issues or ends them). The custodian functions are methods on
  `textureowner.Handler`, reached through `notebook.Service`.
- Counts refresh once per extend, not on a timer; a count-only patch is a
  revision.
- `Recent steering` taps are written immediately by the steer route, not
  queued for "within a minute".
- The listener's edit path for a notebook-titled document through the
  ordinary owner revision route (which needs an open Texture work item on a
  live trajectory) is **not verified**; the notebook is created with the
  initial open work item every Texture document has.
- The "DJ never tells a number which way to move" check is a regexp over the
  text, not a semantic check.
- Candidate order is unchanged (design 4: no re-ranking without another call).
- Promotion at `m >= 10`, the consolidation suggestion above 15 listener
  lines, and the trying-line retirement by rule after 20 stretches are not
  built.
