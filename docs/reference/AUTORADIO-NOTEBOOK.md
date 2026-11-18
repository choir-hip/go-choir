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

**Bounds.** Target about 400 words; hard cap 1,000 words over the lines
Autoradio wrote (tagged lessons and trying lines, and its tap lines under steering),
counted by Texture at write time. A patch that adds a line and leaves the page over it is
refused, whole; a strike, a retire, an untag or a tap is never refused for words, and the
listener's own bullets are never counted. Never a line per segment or forecast. A confirmation changes
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
as `<ref>: <text>` (no counts: they change with every settlement and would break the
cache), grouped by section with the listener's first, then the budget sentence. Each
line is cut to 400 bytes and the section to 8 KB, in document order; when cut, a row says
how many lines are not shown, and the input receipt records lines shown and held.
Volatile part: a fenced `counts` table (`L1: heard 9 of 11`, by line id, from the ledger now), and the fenced
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

**Counts.** `heard h of m` is computed by the handler from the ledger on every extend: of the
settled forecasts (a no-verdict one is not settled) whose `caused_by` ref is the line, `h` were
heard through, `m` settled. The model sees it in the volatile `counts` table and never writes
it. The count printed in the document is a display copy: it rides along whenever a patch
carries another op, and is otherwise rewritten only when a count changed and at least one hour
(`nbCountCadence`) has passed since the desk's last revision. So **the document's counts can be
up to an hour old; the learning route is live** (`live_heard`, `live_of`). This is a write
cadence on a derived value, not a limit on model calls.

**What the counts do not prove.** The DJ writes `caused_by` on its own forecasts, so it chooses
which forecasts count toward a line: a line can be named only where the DJ expects a hit. "heard
9 of 9" is evidence about the nine cases the DJ chose, not about the listener. The route shows,
beside `heard` and `of`, `live_forecasts` (all the listener's forecasts made since the line was
written, settled or not) and `live_settled`, so a line cited on 9 of 200 forecasts reads as
that. The check the DJ cannot choose is the overall scorecard: the gap between the mean `p_heard`
on segments heard through and on segments skipped (ledger conjecture C120). No classifier of
"same kind or subject" is built.

**Retiring a tested line.** A `retire`, or an `add` with `replaces`, of a line with `m > 0` must
cite at least one settled forecast whose `caused_by` names that line; otherwise the op is dropped
with a reason (in `done.notebook.dropped` and the index). The retired line's record keeps its
text, its heard and of at that moment, its age and what replaced it; the route lists the last
50 as `retired`, newest first, and the replacing line's row says `replaces_text` ("replaces L3
(heard 2 of 9)"). The count of the new line restarts at 0 of 0: a reworded lesson is a new
conjecture. Lesson records are written before the patch is asked for; if one cannot be
appended, the patch is not applied.

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
Untagged lines anywhere are the listener's and are never touched. If their rewording of a
tagged line fails validation as a listener line (a tag-like string, say), the patch's `untag`
op leaves it where it stands as their own untagged bullet, and the strike record's `note` says
why: their words are never left tagged as a lesson.

## Steering

`POST /api/autoradio/steer`, owner-authenticated like the sheet. Body
`{"entry_id","direction":"more"|"less"}` (the entry must be on the server run
sheet with a title) or `{"direction":"mute","publisher":"voice-xxxxxx"}`. It
writes an `autoradio.note` (id prefix `autoradio-steer:`) and patches at once:
a tap appends `More like this: "<title>" (<d Mon HH:MM>)` (or `Less`) to
Recent steering, keeping 10; a mute appends `Never play takes by <handle>.`
to What you asked for. A tap's identity is the optional client `id`, else the entry and the
direction (a mute: the publisher); a re-send is answered `duplicate`, and the `tap` op is itself
a no-op when the notebook already holds a tap line with the same verb and title. The op is
validated before anything is written (a title that fails validation is `422` and leaves no
note), and the note is written once the line is in. A tap that loses three head races is
recorded as a `deferred` note and answered `202 {"status":"deferred"}`, never `200`; the next
extend's patch adds a line for every deferred note of the last 24 hours (at most 10) and writes a
done marker (`autoradio-steer-done:`). The title is the phone's when the server's sheet lacks the
entry: an unverified, one-lined, bounded title becomes a steering line in a canonical document and
reaches the model's prompt, fenced as data. Only Autoradio's own tap lines are cut at 10; a bullet the
listener put under Recent steering is never cut. Answer `{status, reason, doc_id, revision_id, line,
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
All are owner-private, and all are on that route: the lines (with lineage and live numbers),
the last 50 retired lines with their numbers, the strikes, the last 40 patches asked and answered
with their reasons (a refused patch included), the steering notes and the input receipts. The
rows behind a line (`?line=L1`) include the forecasts it caused that ended with no verdict. The document is read through
Texture's own routes.

## Finding the document

One function, `Handler.notebookDoc`, serves `GET /api/texture/edition`, `NotebookRead` and
`NotebookApply`, so the phone and the DJ cannot be given two documents. It looks first for the
document the latest applied patch wrote (its result record's `doc_id`: the identity is
persisted, so a notebook adopted by title stays that document), then the deterministic id
(`notebookDocID`, from the owner and the computer), then among the owner's own non-archived documents titled `My notebook`
or `My edition` whose revisions no model agent wrote (any revision by an agent whose label is
not `app:autoradio` makes a document not adoptable: that is how a document a Texture model made
in the same turn is told from the listener's). A failure to list is an error, never "not found".
If the notebook is archived the desk reads and writes nothing; the route says `archived` and the
patch answers `refused: your notebook is archived` (un-archiving is the owner's act). A patch
writes the document's text from the notebook grammar: **rich-text formatting in the document is
flattened to the grammar by a patch.** Lesson text may not hold a link or a URL.

## Whose a tag is, and what is a tap

A tag is the desk's only when a lesson record with that id exists and was not retired. A
bullet that parses as a tagged line without one (the listener retyped a tag) is their words:
the patch writes it back as a plain bullet (the `untag` op), a strike record of kind
`foreign_tag` says why, and it is never counted, cited or retired. A *tap line* is told by the
whole grammar the desk writes, `More like this: "<title>" (<d Mon HH:MM>)` (or `Less`); a
listener's own line that merely begins `More like this: ` is theirs, is not counted toward the
page, and is never cut by the steering cap. If the lesson records cannot be read, the patch is
not written (an id could be minted twice).

## Limits and residuals, named

- The desk compares the listener's edits with its own last revision by walking back at most 60
  revisions. Past that without reaching a desk revision, the state says `chain_cut`; the patch
  is not applied (status `not_applied`, reason recorded on a patch record), and the learning
  route shows `chain_cut`. Strikes are never skipped silently.
- A ledger read that hit its record cap (`card.Truncated`) writes no counts and the route says
  `counts_truncated`.
- A notebook read that fails is an `error` on the learning route, never "no notebook".
- **The grants are declared in the lease grant table** (`internal/lease/grants.go`), not in the
  Autoradio package manifest (`internal/appdesk/firstparty.json`), which has no grant field.
  Install review therefore does not see them. This is a named residual: the manifest should
  name them when the package manifest can carry a grant.
- Cost per extend (named, not fixed): the lesson records are read once, but the tap map, the
  deferred-steering scan and two ledger scorecard reads (the input's and the patch's) each read
  up to the store's record cap. They are not yet shared.
- Residuals named, not fixed: the prompt's display of tags (`loadNotebook`) reads the lesson index
  again instead of sharing the patch's; and a listener who types the exact tap grammar
  (`More like this: "<title>" (<d Mon HH:MM>)`) writes a line the desk reads as its own tap.
- A read of the lesson or patch records that returns the cap (5,000) refuses the patch instead of
  drafting from an incomplete list; the done event says why and the log carries the owner and
  stretch ids. A retire record counts as gone only once its patch landed (applied or noop).
- No wake of a model for a notebook-titled document, whether published through the Texture
  root or not (`reconcileAgentWakeLockedFor`).

## Choices where the design was silent or the code differed

- One commit set holds L4 and L5 together: the validators need the input's
  `settled` fence, so they do not stand apart.
- Grants are static holders in the lease grant table, not per-run leases
  (nothing issues or ends them). The custodian functions are methods on
  `textureowner.Handler`, reached through `notebook.Service`.
- Counts are computed on every extend but written to the document at most once an hour when
  nothing else is written (see Counts).
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

## Residual: the notebook is read without a privacy label (2026-11-18, review of the impact rows)

`NotebookRead` returns the document as plain strings and the programme
handler sets no `owner_private` label on the run that reads it. The DJ
therefore consumes the listener's own text with no label, and a later
outbound act in that run would not see it as owner-private. No such act
exists on the programme route today. To be closed when the route gains any
outbound act: label the read as `texture.search` labels its snippets.

