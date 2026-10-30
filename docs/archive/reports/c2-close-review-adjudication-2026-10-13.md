# C2 close review: adjudication (2026-10-13)

Panel: convergent, prompt `.agentic-consensus/c2-close-prompt-2026-10-13.md`,
seven members all `ok` (claude opus, devin, opencode, codex, gpt-6.1-sol,
gemini-3.8-flash, deepseek-v4.1-flash). Raw outputs are session diagnostics
under `.agentic-consensus/c2-close-2026-10-13/` (not durable evidence).

## Verdicts

| member | verdict |
| --- | --- |
| claude | holds with residuals, conditional on (a) and (b) below |
| devin | holds with residuals |
| opencode | holds with residuals |
| gemini | holds with residuals |
| deepseek | holds with residuals; one record-integrity blocker (immutability) |
| gpt-6.1-sol | reopen the acceptance gate (keep the paper running) |
| codex | reopen the acceptance gate (keep the paper running) |

No member asks to roll back or to change frozen identities. The split is
about whether missing receipts block the close or travel as residuals. The
two "reopen" votes and claude's two conditions name the same items, and all
of them are read-only derivations from data the live paper already holds.

## Adjudication

**C2 acceptance stays open on four evidence items; autopaper_live stands as
a live state.** The orchestrator adopts the stricter reading: the missing
items test C2's central claim (evidence drives revision and correction), so
they cannot be transferred as ordinary residuals. None needs a code change
first; each is a deployed, read-only receipt over the paper computer's
records and Store B.

Blocking for the close (owed receipts):

- **E1, criterion (3) freshness.** For every v5+ story, each version cites
  at least one capture whose *arrival* time is after the previous version's
  publish time; reconcile committed revisions against published versions
  and explain gaps. (all seven)
- **E2, immutability.** Hash the fixed contents (text, manifest hash,
  signatures, resolved blocks) of every version of one live v5+ story,
  re-hash after its next publish, diff. (claude, deepseek, gpt-6.1-sol,
  codex)
- **E3, criterion (2) answers.** Join each capture-producing poll's arrival
  record to the management turn that answered it (story, Cast, Note or
  reasoned decline) and give the per-interval table of captures, stories,
  revisions and arrival-to-answer latency. Cursor=head shows consumption,
  not answers. (claude, gpt-6.1-sol, codex)
- **E4, support-failure correction loop.** deepseek reports it is already in
  the ledger: Ukraine story `texture:<doc>` inbox, Jev `unsupported`
  verdicts at 15:38:53, 15:39:37, 15:46:40, 15:49:49, each followed by a
  committed "Repair the … support failure" revision (15:39:29, 15:45:48,
  15:49:39, 15:50:40). Verify and promote into the live receipt; split the
  14 failed badges into quote-not-found versus unsupported verdicts (a
  quote-not-found on a published citation would fail criterion (4)).

Conformance gap (fix as a compatible change, outside the freeze):

- **A1 not implemented.** `quoteState` maps `quotematch.NotCaptured` to
  `pending` (`internal/platform/publication_checks.go:240-251`); the binding
  pre-flight amendment required a distinct "unread source" badge, and A2
  required the not-captured share in receipts. (claude, devin, deepseek,
  gpt-6.1-sol, codex)

Residuals (C2 follow-up unless named):

- Pending badges do not drain: 84 pending at 16:22 and still 84 later while
  exact grew 194→208 (deepseek); verdict and publication pushes are
  process-local queues with no durable retry
  (`internal/agentcore/story_publish.go`); an arrival-append failure after a
  capture persist is only logged (`internal/agentcore/source_schedule.go:161-175`).
  Report pending-badge age, then design a durable delivery obligation that
  does not add a markerless boot-time resumer.
- Front-list paging orders by second-resolution `updated_at` but re-sorts
  pages by nanosecond publish time (`publication_front.go`), so same-second
  boundaries can misorder or duplicate.
- Golden test pinning normalized text and block ids against the
  `golang.org/x/text` pin (claude).
- Remaining tracer receipts: changed/unchanged re-capture, PDF page-block
  quote, `[12]` links, gateway "M of N" line, late provider inside a running
  cell, `@all` preservation, per-turn metrics against C0, phone-width reader
  artifact.
- Model comparison: transferred by explicit amendment, not silently done.
- Prose defects (`**`, process narration): fixed at d75cb193 for new
  revisions; old versions keep their bytes.
- Paper update path: the paper takes a deploy refresh as a crash boot the
  first time (no guest-written marker on the old build), which closes every
  schedule; the owner restarts them. Later boots refresh through the
  planned-restart marker (bcc556de). An owner-visible "paper stopped"
  signal belongs to the restart-policy station.
- Management load at 211 sources (about 1,000 records an hour on one desk)
  and Store B growth from pages that change every poll: gate tiers 2 and 3
  on the E3 latency table and a 24-hour captures-per-URL count.
- B11 (cell compiler panic on a slice literal over `choir.Records()`) is
  still open; the now card's "B10-B13 fixed" shorthand is wrong and is
  corrected to "B10, B12, B12b, B13 fixed; B11 open".
- C1e: engineering-backed translation scenario (already transferred).

## Next actions

1. Derive E1-E4 read-only from the paper computer and Store B; write
   `docs/evidence/c2-close-receipts-2026-10-13.json`.
2. A1 badge as a compatible orange change with its own Landing Loop.
3. Close C2 when E1-E4 pass; reopen 2b if E1 or E2 fails.
